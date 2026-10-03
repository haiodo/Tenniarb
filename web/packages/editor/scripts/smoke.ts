// Playwright smoke: node scripts/smoke.ts [outDir]. Not part of CI; Playwright comes from web/bench/node_modules.
// Needs dist/ (npm run build -w @tenniarb/editor).
import { createServer } from "node:http";
import { mkdirSync, readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { extname, join, normalize } from "node:path";

const out = process.argv[2] ?? ".work/editor";
mkdirSync(out, { recursive: true });
const dist = new URL("../dist/", import.meta.url).pathname;
const { chromium } = createRequire(import.meta.url)("../../../bench/node_modules/playwright");

const types: Record<string, string> = { ".html": "text/html", ".js": "text/javascript", ".woff2": "font/woff2" };
const server = createServer((req, res) => {
  const file = normalize(join(dist, decodeURIComponent(new URL(req.url ?? "/", "http://x").pathname)));
  try {
    if (!file.startsWith(dist)) throw new Error("outside");
    res.setHeader("content-type", types[extname(file)] ?? "application/octet-stream");
    res.end(readFileSync(file));
  } catch {
    res.statusCode = 404;
    res.end("not found");
  }
}).listen(0, "127.0.0.1");
await new Promise((r) => server.once("listening", r));
const base = `http://127.0.0.1:${(server.address() as { port: number }).port}`;

const failures: string[] = [];
const check = (ok: boolean, what: string): void => {
  console.log(`${ok ? "ok  " : "FAIL"} ${what}`);
  if (!ok) failures.push(what);
};

const browser = await chromium.launch();
const p = await browser.newPage({ viewport: { width: 1100, height: 800 }, deviceScaleFactor: 2 });
const problems: string[] = [];
p.on("console", (m: { type(): string; text(): string }) => ["error", "warning"].includes(m.type()) && problems.push(m.text()));
p.on("pageerror", (e: Error) => problems.push(String(e)));
await p.goto(`${base}/demo.html`);
await p.waitForFunction(() => (window as any).editor !== undefined);
await p.waitForTimeout(500);

// Scene state the checks compare: first item position and both ends of the first link.
const state = () =>
  p.evaluate(() => {
    const s = (window as any).editor.session;
    const item = s.element.items[0];
    const link = s.element.items.find((i: any) => i.kind === "Link");
    const l = s.scene.drawables.get(link);
    return { x: item.x, y: item.y, src: l.source, dst: l.target, changes: (window as any).changes, sel: s.selection.length };
  });
const at = () => p.evaluate(() => { const e = (window as any).editor; return e.screenOf(e.session.element.items[0]); });

const ink = await p.$eval("canvas", (c: HTMLCanvasElement) => c.getContext("2d")!.getImageData(0, 0, c.width, c.height).data.some((v, i) => i % 4 === 3 && v !== 0));
check(ink, "canvas is drawn");
check(problems.length === 0, `no console errors ${problems.slice(0, 3).join(" | ")}`);
await p.screenshot({ path: `${out}/1-before.png` });
const s0 = await state();

const pt = await at();
await p.mouse.click(pt.x, pt.y);
await p.waitForTimeout(100);
check((await state()).sel === 1, "click selects the item");
await p.screenshot({ path: `${out}/2-selected.png` });

await p.mouse.move(pt.x, pt.y);
await p.mouse.down();
await p.mouse.move(pt.x + 80, pt.y + 60, { steps: 6 });
await p.mouse.up();
await p.waitForTimeout(100);
const s1 = await state();
check(s1.x > s0.x && s1.y < s0.y, `drag moved the model (${s0.x},${s0.y}) -> (${s1.x},${s1.y})`);
check(JSON.stringify([s1.src, s1.dst]) !== JSON.stringify([s0.src, s0.dst]), "link re-routed");
check(s1.changes === 1, `onChange fired once (${s1.changes})`);
check((await p.evaluate(() => (window as any).lastText)).includes(`pos ${s1.x}`), "onChange text carries the new position");
await p.screenshot({ path: `${out}/3-dragged.png` });

await p.keyboard.press("Control+z");
await p.waitForTimeout(100);
const s2 = await state();
check(s2.x === s0.x && s2.y === s0.y && JSON.stringify([s2.src, s2.dst]) === JSON.stringify([s0.src, s0.dst]), "ctrl+z restores item and link");
check(s2.changes === 2, `onChange fired on undo (${s2.changes})`);
await p.screenshot({ path: `${out}/4-undone.png` });
await p.keyboard.press("Control+Shift+z");
await p.waitForTimeout(100);
const s3 = await state();
check(s3.x === s1.x && s3.y === s1.y, "ctrl+shift+z redoes");

// empty click clears; drag on empty pans without touching the model
await p.mouse.click(1050, 760);
check((await state()).sel === 0, "click on empty space clears selection");
const before = await p.$eval("canvas", (c: HTMLCanvasElement) => c.toDataURL());
await p.mouse.move(1050, 760);
await p.mouse.down();
await p.mouse.move(1000, 700, { steps: 4 });
await p.mouse.up();
check((await p.$eval("canvas", (c: HTMLCanvasElement) => c.toDataURL())) !== before && (await state()).changes === s3.changes, "empty drag pans, model untouched");

// readonly
await p.click("#ro");
await p.waitForFunction(() => document.getElementById("status")!.textContent === "readonly");
const pr = await at();
await p.mouse.click(pr.x, pr.y);
await p.mouse.move(pr.x, pr.y);
await p.mouse.down();
await p.mouse.move(pr.x + 80, pr.y + 60, { steps: 4 });
await p.mouse.up();
const sr = await state();
check(sr.sel === 0 && sr.x === s0.x && sr.y === s0.y && sr.changes === s3.changes, "readonly: no selection, no move, no onChange");
await p.screenshot({ path: `${out}/5-readonly.png` });

check(problems.length === 0, `no console errors at the end ${problems.slice(0, 3).join(" | ")}`);
await browser.close();
server.close();
if (failures.length > 0) {
  console.error(`${failures.length} check(s) failed`);
  process.exit(1);
}
