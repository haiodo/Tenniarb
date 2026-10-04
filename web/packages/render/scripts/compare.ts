// Renders every fixture element at scale 2 (like the Swift export) and builds Swift|TS side-by-side images + index.html.
// usage: node web/packages/render/scripts/compare.ts [file.tenn [outdir]]
// default: all web/fixtures into .work/web-stage5 (needs swift-png/ there from `make swift-png`);
// with a file: outdir needs swift-png/ from `make swift-png F=file O=outdir/swift-png`.
import { resolve, basename } from "node:path";
import { existsSync, mkdirSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import { createCanvas, loadImage } from "@napi-rs/canvas";
import { TennParser, parseTenn } from "@tenniarb/core";
import type { Element } from "@tenniarb/core";
import { genericToSRGB } from "../src/color.ts";
import { renderTenn } from "../src/render-tenn.ts";
import { fileURLToPath } from "node:url";

// The Swift export fills #e7e9eb as a Generic RGB CGColor, which lands lighter in the sRGB bitmap.
const background = `rgb(${genericToSRGB({ r: 0xe7 / 255, g: 0xe9 / 255, b: 0xeb / 255, a: 1 }).map((v) => Math.round(v * 255)).join(",")})`;

const root = fileURLToPath(new URL("../../../../", import.meta.url));
const fixtures = `${root}web/fixtures`;
const [inFile, outArg] = process.argv.slice(2);
const out = outArg ? resolve(outArg) : `${root}.work/web-stage5`;
for (const d of ["ts-png", "compare"]) mkdirSync(`${out}/${d}`, { recursive: true });

const rows: string[] = [];
const walk = async (file: string, text: string, e: Element, path: string[]): Promise<void> => {
  const name = `${file}.${path.join(".").replaceAll(" ", "_").replaceAll("/", "_")}`;
  const tsFile = `${out}/ts-png/${name}.png`;
  const swFile = `${out}/swift-png/${name}.png`;
  let status = "";
  try {
    writeFileSync(tsFile, await renderTenn(text, { format: "png", element: e.name.includes("/") ? e.name : path.join("/"), scale: 2, background }));
  } catch (err) {
    status = `TS fail: ${(err as Error).message}`;
  }
  if (status === "" && existsSync(swFile)) {
    const [a, b] = await Promise.all([loadImage(swFile), loadImage(tsFile)]);
    const c = createCanvas(a.width + b.width + 10, Math.max(a.height, b.height));
    const g = c.getContext("2d");
    g.fillStyle = "#fff";
    g.fillRect(0, 0, c.width, c.height);
    g.drawImage(a, 0, 0);
    g.drawImage(b, a.width + 10, 0);
    writeFileSync(`${out}/compare/${name}.png`, c.encodeSync("png"));
    status = `swift ${a.width}x${a.height}, ts ${b.width}x${b.height}`;
  } else if (status === "") status = "no swift png";
  rows.push(`<h3>${name} <small>${status}</small></h3>${status.startsWith("swift") ? `<img src="${name}.png">` : ""}`);
  for (const c of e.elements) await walk(file, text, c, [...path, c.name]);
};

const inputs = inFile
  ? [resolve(inFile)]
  : readdirSync(fixtures).filter((n) => n.endsWith(".tenn") && !n.endsWith(".saved.tenn")).sort().map((n) => `${fixtures}/${n}`);
for (const path of inputs) {
  const f = basename(path);
  const text = readFileSync(path, "utf8");
  const p = new TennParser();
  const tree = p.parse(text.charCodeAt(0) === 0xfeff ? text.slice(1) : text);
  if (p.errors.hasErrors()) continue;
  const model = parseTenn(tree);
  for (const c of model.elements) await walk(f.slice(0, -5), text, c, [c.name]);
}
writeFileSync(`${out}/compare/index.html`, `<!doctype html><meta charset="utf-8"><title>compare</title><body style="font:14px sans-serif">${rows.join("\n")}`);
console.log(`${rows.length} elements -> ${out}/compare/index.html`);
