import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import { createCanvas, loadImage } from "@napi-rs/canvas";
import type { ImageData } from "@napi-rs/canvas";
import { TennParseError, renderTenn } from "../src/node.ts";
import { canvasQuirks } from "../src/drawable.ts";
import { renderElement } from "../src/render.ts";
import { newCanvas, parseDiagram } from "./helpers.ts";

const cli = fileURLToPath(new URL("../src/cli.ts", import.meta.url));
const dir = mkdtempSync(join(tmpdir(), "tenn-render-"));
const pages = (pdf: Buffer): number => (pdf.toString("latin1").match(/\/Type\s*\/Page[^s]/g) ?? []).length;
const two = `element A { item X { pos 0 0 }\n item Y { pos 100 0 }\n link X Y }\nelement B { item Z }\nelement Empty { }`;
const PNG1 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==";
const withImage = (b64: string): string => `element A { item X {\n image "pic.png" @(${b64})\n title %{\n A@(pic.png|32)\n}\n } }`;

function run(args: string[], text: string, name = "in.tenn") {
  const file = join(dir, name);
  writeFileSync(file, text);
  return spawnSync(process.execPath, [cli, file, ...args], { encoding: "utf8" });
}

test("pdf: one page per element with items, svg first element, png scaled", async () => {
  const pdf = await renderTenn(two, { format: "pdf" });
  assert.equal(pdf.subarray(0, 5).toString(), "%PDF-");
  assert.equal(pages(pdf), 2);
  assert.equal(pages(await renderTenn(two, { format: "pdf", element: "B" })), 1);

  const svg = await renderTenn(two, { format: "svg" });
  assert.match(svg, /^<svg /);
  const img = await loadImage(Buffer.from(svg));
  assert.ok(img.width > 60 && img.height > 60);

  const p1 = await loadImage(await renderTenn(two, { format: "png", scale: 1 }));
  const p2 = await loadImage(await renderTenn(two, { format: "png" }));
  assert.equal(p2.width, p1.width * 2);
  assert.equal(p1.width, img.width);
});

test("errors: parse errors carry line:col, unknown element", async () => {
  await assert.rejects(renderTenn("element A {", { format: "svg" }), TennParseError);
  await assert.rejects(renderTenn(two, { format: "svg", element: "Nope" }), /element not found/);
});

test("valid image reaches the output, truncated one renders as missing", async () => {
  assert.match(await renderTenn(withImage(PNG1), { format: "svg" }), /data:image\/png/);
  const bad = Buffer.from(PNG1, "base64").subarray(0, 40).toString("base64");
  assert.doesNotMatch(await renderTenn(withImage(bad), { format: "svg" }), /<image/);
});

test("cli: truncated and garbage images do not crash the process", () => {
  const png = Buffer.from(PNG1, "base64");
  for (const b64 of [png.subarray(0, 40).toString("base64"), png.subarray(0, 60).toString("base64"), "AAAA", "/9j/4AAQSkZJRg=="]) {
    const out = join(dir, "bad.png");
    const r = run(["-o", out], withImage(b64), "bad.tenn");
    assert.equal(r.status, 0, r.stderr);
    assert.ok(readFileSync(out).length > 0);
  }
});

test("cli exit codes", () => {
  const out = join(dir, "o.pdf");
  assert.equal(run(["-o", out], two).status, 0);
  assert.equal(pages(readFileSync(out)), 2);
  assert.equal(run(["-o", join(dir, "o.png"), "--scale", "1", "--element", "B"], two).status, 0);

  const usage = run([], two);
  assert.equal(usage.status, 1);
  assert.match(usage.stderr, /usage:/);
  assert.equal(run(["-o", join(dir, "o.txt")], two).status, 1);
  assert.equal(run(["-o", out, "--scale", "x"], two).status, 1);
  assert.equal(spawnSync(process.execPath, [cli, join(dir, "missing.tenn"), "-o", out], { encoding: "utf8" }).status, 1);

  const parse = run(["-o", out], "element A {\n  item X {", "broken.tenn");
  assert.equal(parse.status, 2);
  assert.match(parse.stderr, /broken\.tenn:\d+:\d+: /);

  const render = run(["-o", out, "--element", "Nope"], two);
  assert.equal(render.status, 3);
  assert.match(render.stderr, /element not found/);
});

test("png: background fills the whole export, drop shadow is drawn", async () => {
  const src = (style: string): string => `element A { styles { item { ${style} } }\n item X { pos 0 0 } }`;
  const decode = async (s: string) => {
    const img = await loadImage(await renderTenn(s, { format: "png", background: "#e7e9eb" }));
    const g = createCanvas(img.width, img.height).getContext("2d");
    g.drawImage(img, 0, 0);
    return g.getImageData(0, 0, img.width, img.height);
  };
  const plain = await decode(src(""));
  const shadowed = await decode(src("shadow -5 -5 5"));
  const px = (d: ImageData, x: number, y: number): number => d.data[(y * d.width + x) * 4]!;
  const bg = 0xe7;
  for (const [x, y] of [[0, 0], [plain.width - 1, 0], [0, plain.height - 1], [plain.width - 1, plain.height - 1]] as const) {
    assert.equal(px(plain, x, y), bg);
  }
  let deep = 0;
  for (let i = 0; i < shadowed.data.length; i += 4) if (plain.data[i]! - shadowed.data[i]! > 15) deep++;
  assert.ok(deep > 20, `pixels darkened by the shadow: ${deep}`);
});

test("shadowColor alpha is raised to canvasQuirks.shadowAlphaPower", async () => {
  const { canvas2d } = newCanvas(200, 200);
  const seen: string[] = [];
  const spy = new Proxy(canvas2d, {
    get(target, key) {
      const v = Reflect.get(target, key) as unknown;
      return typeof v === "function" ? v.bind(target) : v;
    },
    set(target, key, value) {
      if (key === "shadowColor") seen.push(value as string);
      return Reflect.set(target, key, value);
    },
  });
  const el = parseDiagram(`element A { styles { item { shadow 2 2 4 } }\n item X { pos 0 0 } }`);
  try {
    canvasQuirks.shadowAlphaPower = 0.5;
    renderElement(spy, el, { measureContext: undefined });
  } finally {
    canvasQuirks.shadowAlphaPower = 1;
  }
  assert.ok(seen.length > 0);
  assert.ok(seen.every((c) => Math.abs(Number(/,([\d.]+)\)$/.exec(c)![1]) - Math.sqrt(1 / 3)) < 1e-6), seen.join(" "));
});

// Guards the quirk itself: if @napi-rs/canvas stops squaring blurred shadow alpha, this fails and the quirk must go.
test("@napi-rs/canvas squares shadow alpha only when blurred", () => {
  const shadowAlpha = (blur: number) => {
    const c = createCanvas(200, 200);
    const x = c.getContext("2d");
    x.fillStyle = "#fff";
    x.fillRect(0, 0, 200, 200);
    x.shadowColor = "rgba(0,0,0,0.5)";
    x.shadowBlur = blur;
    x.shadowOffsetX = 60;
    x.shadowOffsetY = 60;
    x.fillStyle = "#f00";
    x.fillRect(10, 10, 80, 80);
    return (255 - x.getImageData(110, 110, 1, 1).data[0]) / 255;
  };
  assert.ok(Math.abs(shadowAlpha(0) - 0.5) < 0.02, `blur 0: ${shadowAlpha(0)}`);
  assert.ok(Math.abs(shadowAlpha(6) - 0.25) < 0.02, `blur 6: ${shadowAlpha(6)}`);
});
