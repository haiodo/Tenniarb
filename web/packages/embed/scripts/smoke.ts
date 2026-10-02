// Playwright smoke: node scripts/smoke.ts [outDir]. Not part of CI; Playwright comes from web/bench/node_modules.
// Needs a built dist/ (npm run build -w @tenniarb/embed).
import { createServer } from "node:http";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { extname, join, normalize } from "node:path";

const out = process.argv[2] ?? ".work/embed";
mkdirSync(out, { recursive: true });
const pkg = new URL("../", import.meta.url).pathname;
const { chromium } = createRequire(import.meta.url)("../../../bench/node_modules/playwright");

const types: Record<string, string> = { ".html": "text/html", ".js": "text/javascript", ".woff2": "font/woff2" };
const server = createServer((req, res) => {
  const file = normalize(join(pkg, decodeURIComponent(new URL(req.url ?? "/", "http://x").pathname)));
  try {
    if (!file.startsWith(pkg)) throw new Error("outside");
    res.setHeader("content-type", types[extname(file)] ?? "application/octet-stream");
    res.end(readFileSync(file));
  } catch {
    res.statusCode = 404;
    res.end("not found");
  }
}).listen(0, "127.0.0.1");
await new Promise((r) => server.once("listening", r));
const base = `http://127.0.0.1:${(server.address() as { port: number }).port}`;

// Single-file page as the Swift export produces it: standalone bundle inline, documents as text/x-tenn.
const example = readFileSync(new URL("../../../../docs/Example.tenn", import.meta.url));
const bundle = readFileSync(`${pkg}dist/tenniarb-embed.standalone.min.js`, "utf8");
const page = `<!doctype html><meta charset="utf-8"><title>single file</title>
<style>body{font:15px system-ui;margin:16px}.tenn-diagram{border:1px solid #ccc;margin:12px 0}</style>
<script>${bundle}</script>
<script type="text/x-tenn" data-encoding="base64" data-element="Documentation/Basic steps">${example.toString("base64")}</script>
<script type="text/x-tenn">element "Привет" { item "plain UTF-8 text" { pos 0 0 } }</script>`;
const single = `${out}/example.html`;
writeFileSync(single, page);

const browser = await chromium.launch();
const failures: string[] = [];
const check = (ok: boolean, what: string): void => {
  console.log(`${ok ? "ok  " : "FAIL"} ${what}`);
  if (!ok) failures.push(what);
};

async function open(url: string, shot: string) {
  const p = await browser.newPage({ viewport: { width: 900, height: 900 }, deviceScaleFactor: 2 });
  const problems: string[] = [];
  p.on("console", (m: { type(): string; text(): string }) => ["error", "warning"].includes(m.type()) && problems.push(m.text()));
  p.on("pageerror", (e: Error) => problems.push(String(e)));
  await p.goto(url);
  await p.waitForFunction(() => document.querySelectorAll("canvas").length > 0 && [...document.querySelectorAll("canvas")].every((c) => c.width > 0));
  await p.waitForTimeout(500);
  await p.screenshot({ path: `${out}/${shot}.png`, fullPage: true });
  return { p, problems };
}

// non-transparent pixel count per canvas
const blank = (p: any): Promise<boolean[]> =>
  p.$$eval("canvas", (cs: HTMLCanvasElement[]) =>
    cs.map((c) => !c.getContext("2d")!.getImageData(0, 0, c.width, c.height).data.some((v, i) => i % 4 === 3 && v !== 0)),
  );
const sig = (p: any, i: number): Promise<string> => p.$$eval("canvas", (cs: HTMLCanvasElement[], n: number) => cs[n].toDataURL(), i);

for (const [name, url] of [["demo", `${base}/demo.html`], ["single-file", `file://${process.cwd()}/${single}`]] as const) {
  const { p, problems } = await open(url, name);
  check(problems.length === 0, `${name}: no console errors/warnings ${problems.slice(0, 3).join(" | ")}`);
  const b = await blank(p);
  const errors = await p.$$eval(".tenn-error", (e: HTMLElement[]) => e.map((x) => x.textContent));
  const expectedErrors = name === "demo" ? 1 : 0;
  check(errors.length === expectedErrors, `${name}: inline errors = ${expectedErrors} (${JSON.stringify(errors)})`);
  if (name === "demo") check(/\d+:\d+/.test(errors[0] ?? ""), "demo: parse error has line:col");
  check(b.length >= 2 && b.every((x) => !x), `${name}: ${b.length} canvases, none blank`);
  check((await p.evaluate(() => [...document.fonts].filter((f) => f.family === "Inter" && f.status === "loaded").length)) === 4, `${name}: 4 Inter faces, loaded once`);

  // pan: drag on the first interactive canvas; zoom: ctrl+wheel
  const box = await p.locator("canvas").first().boundingBox();
  const cx = box.x + box.width / 2, cy = box.y + box.height / 2;
  const before = await sig(p, 0);
  await p.mouse.move(cx, cy);
  await p.mouse.down();
  await p.mouse.move(cx + 40, cy + 20, { steps: 4 });
  await p.mouse.up();
  await p.waitForTimeout(100);
  const panned = await sig(p, 0);
  check(panned !== before, `${name}: drag pans`);
  await p.keyboard.down("Control");
  await p.mouse.wheel(0, -200);
  await p.keyboard.up("Control");
  await p.waitForTimeout(100);
  check((await sig(p, 0)) !== panned, `${name}: ctrl+wheel zooms`);
  await p.mouse.dblclick(cx, cy);
  await p.waitForTimeout(100);
  check((await sig(p, 0)) === before, `${name}: double-click fits back`);

  // ResizeObserver re-fit: canvas follows the container width
  const w0 = await p.locator("canvas").first().evaluate((c: HTMLCanvasElement) => c.clientWidth);
  await p.setViewportSize({ width: 200, height: 900 });
  await p.waitForTimeout(300);
  const w1 = await p.locator("canvas").first().evaluate((c: HTMLCanvasElement) => c.clientWidth);
  check(w1 < w0 && !(await blank(p))[0], `${name}: resize ${w0} -> ${w1}px, still drawn`);
  await p.screenshot({ path: `${out}/${name}-narrow.png`, fullPage: true });
  await p.close();
}

// evaluate: false must not run expressions; the default must.
const { p } = await open(`${base}/demo.html`, "eval");
const probe = await p.evaluate(async () => {
  const T = (window as any).Tenniarb;
  const text = `element "E" { item "v" { pos 0 0 title %{${"${(window.__probe = (window.__probe || 0) + 1, 'x')}"}} } }`;
  const el = () => document.body.appendChild(document.createElement("div"));
  await T.render(el(), text, { evaluate: false });
  const off = (window as any).__probe ?? 0;
  const h = await T.render(el(), text, { evaluate: true });
  const on = (window as any).__probe ?? 0;
  const bad = el();
  await T.render(bad, 'element "Broken" {', {});
  const missing = el();
  const h2 = await T.render(missing, text, { element: "nope" });
  // markup attributes: data-evaluate="false" skips, the default runs
  const node = (attr: string) => {
    const d = el();
    d.className = "tenn";
    if (attr) d.setAttribute("data-evaluate", attr);
    d.textContent = text;
    return d;
  };
  await T.run({ nodes: [node("false")] });
  const attrOff = (window as any).__probe;
  await T.run({ nodes: [node("")] });
  return { attrOff, attrOn: (window as any).__probe, off, on, tree: JSON.stringify(h.elements()), path: h.path, bad: bad.textContent, missing: missing.textContent, h2: h2.path };
});
check(probe.off === 0, "evaluate:false does not run expressions");
check(probe.on > 0, "evaluate:true runs expressions");
check(probe.attrOff === probe.on && probe.attrOn > probe.on, "data-evaluate attribute honoured by run()");
check(probe.path === "E" && probe.tree.includes('"hasItems":true'), `handle.path/elements ${probe.tree}`);
check(/parse error/.test(probe.bad), `render() shows parse errors inline: ${probe.bad?.split("\n")[1]}`);
check(/element not found: nope/.test(probe.missing) && probe.h2 === "", "unknown element is reported inline");
await p.close();

await browser.close();
server.close();
console.log(failures.length === 0 ? "smoke passed" : `smoke FAILED: ${failures.length}`);
if (failures.length > 0) process.exitCode = 1;
