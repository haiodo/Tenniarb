// Playwright smoke of demo.html: node scripts/smoke.ts [outDir]. Not part of CI; Playwright comes from web/bench/node_modules.
// Needs dist/ (npm run build -w @tenniarb/mindmap). Two maps on two Y.Docs, each showing the other's selection as a peer.
import { createServer } from "node:http";
import { mkdirSync, readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { extname, join, normalize } from "node:path";
import { fileURLToPath } from "node:url";

const out = process.argv[2] ?? ".work/mindmap";
mkdirSync(out, { recursive: true });
const dist = fileURLToPath(new URL("../dist/", import.meta.url));
const { chromium } = createRequire(import.meta.url)("../../../bench/node_modules/playwright");

const types: Record<string, string> = { ".html": "text/html", ".js": "text/javascript" };
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
const p = await browser.newPage({ viewport: { width: 1400, height: 900 }, deviceScaleFactor: 2 });
const problems: string[] = [];
// The readback warning is about this script's own getImageData.
p.on("console", (m: { type(): string; text(): string }) => ["error", "warning"].includes(m.type()) && !m.text().includes("willReadFrequently") && problems.push(m.text()));
p.on("pageerror", (e: Error) => problems.push(String(e)));
await p.goto(`${base}/demo.html`);
await p.waitForFunction(() => (window as any).clients !== undefined);
await p.waitForTimeout(300);

const texts = () => p.evaluate(() => (window as any).clients.map((c: any) => c.ytext.toString()) as string[]);
const sel = (id: number) => p.evaluate((id: number) => (window as any).selections[id] ?? [], id) as Promise<string[]>;
// Device pixels of exactly this colour: peer outlines and labels.
const ink = (map: string, rgb: number[]) =>
  p.$eval(`${map} canvas`, (c: HTMLCanvasElement, rgb: number[]) => {
    const d = c.getContext("2d")!.getImageData(0, 0, c.width, c.height).data;
    let n = 0;
    for (let i = 0; i < d.length; i += 4) if (d[i] === rgb[0] && d[i + 1] === rgb[1] && d[i + 2] === rgb[2] && d[i + 3] === 255) n++;
    return n;
  }, rgb);
const [ORANGE, PURPLE] = [[0xe8, 0x59, 0x0c], [0x70, 0x48, 0xe8]];
const box = async (map: string) => (await p.locator(map).boundingBox())!;
const settle = () => p.waitForTimeout(150);

const drawn = await p.$$eval("canvas", (cs: HTMLCanvasElement[]) => cs.map((c) => c.getContext("2d")!.getImageData(0, 0, c.width, c.height).data.some((v, i) => i % 4 === 3 && v !== 0)));
check(drawn.join() === "true,true" && (await texts())[0] === (await texts())[1], "both maps drawn from one text");
await p.screenshot({ path: `${out}/1-start.png` });

// A point inside the single selected item of map 1: the quick panel sits 15 right of its left edge, 10 above its selector box.
const inSelected = async () => {
  const r = (await p.locator("#m1 .tn-pop").boundingBox())!;
  return { x: r.x - 5, y: r.y + r.height + 25 };
};

// Tab on empty space: a new item at the clicked point, selected; map 2 sees it and the peer outline.
const b1 = await box("#m1");
const P = { x: b1.x + 40, y: b1.y + b1.height - 80 };
const orange0 = await ink("#m2", ORANGE);
await p.mouse.click(P.x, P.y);
await p.keyboard.press("Tab");
await settle();
let [t1, t2] = await texts();
check(t1.includes('item "Untitled 1"') && t1 === t2, "Tab adds an item, synced to map 2");
check(JSON.stringify(await sel(1)) === '["item:Untitled 1#0"]', `onSelection reports the new item ${JSON.stringify(await sel(1))}`);
check((await ink("#m2", ORANGE)) > orange0 + 50, "map 2 draws the peer outline of User 1");
await p.screenshot({ path: `${out}/2-added.png` });
const inP = await inSelected();

// Drag: one change on drop, map 2 gets the new pos.
const posOf = (t: string, name: string) => new RegExp(`item "${name}" \\{\\n {4}pos (-?\\d+) (-?\\d+)`).exec(t)?.slice(1).map(Number);
const before = posOf(t1, "Untitled 1")!;
await p.mouse.move(inP.x, inP.y);
await p.mouse.down();
await p.mouse.move(inP.x + 60, inP.y - 40, { steps: 6 });
await p.mouse.up();
await settle();
[t1, t2] = await texts();
const after = posOf(t2, "Untitled 1")!;
check(after[0]! === before[0]! + 60 && after[1]! === before[1]! + 40 && t1 === t2, `drag moved the item ${before} -> ${after}`);
const at = { x: inP.x + 60, y: inP.y - 40 };

// Map 2 selects everything: map 1 shows User 2 on all items.
const b2 = await box("#m2");
await p.mouse.click(b2.x + b2.width - 30, b2.y + b2.height - 30);
await p.keyboard.press("Control+a");
await settle();
check((await sel(2)).includes("item:Untitled 1#0"), "map 2 select all reports the keys");
check((await ink("#m1", PURPLE)) > 100, "map 1 draws the peer outlines of User 2");

// Rename in map 1: map 2 keeps the renamed item selected (identity on remote rename) and reports the new key.
await p.mouse.click(at.x, at.y);
await p.keyboard.press("Enter");
await p.keyboard.type("Renamed");
await p.keyboard.press("Enter");
await settle();
[t1, t2] = await texts();
check(t1.includes('item "Renamed"') && !t1.includes('item "Untitled 1"') && t1 === t2, "rename synced");
check((await sel(2)).includes("item:Renamed#0"), `map 2 keeps the renamed item selected ${JSON.stringify(await sel(2))}`);
await p.screenshot({ path: `${out}/3-renamed.png` });

// Quick style: the colour segment of the panel over the selected item.
await p.locator('#m1 .tn-pop button[title="Color"]').click();
await p.locator("#m1 .tn-menu .tn-mi").nth(1).click();
await settle();
[t1, t2] = await texts();
check(/item "Renamed" \{\n {4}pos -?\d+ -?\d+\n {4}color \S+\n\}/.test(t1) && t1 === t2, "quick style adds a colour to the item");
await p.screenshot({ path: `${out}/4-styled.png` });

// Link: a second item, then ctrl-drag from the renamed one onto it.
await p.mouse.click(b1.x + 40, b1.y + 120);
await p.keyboard.press("Tab");
await settle();
const inQ = await inSelected();
await p.keyboard.down("Control");
await p.mouse.move(at.x, at.y);
await p.mouse.down();
await p.mouse.move(inQ.x, inQ.y, { steps: 6 });
await p.mouse.up();
await p.keyboard.up("Control");
await settle();
[t1, t2] = await texts();
check(t1.includes('link "Renamed" "Untitled 2"') && t1 === t2, "ctrl-drag links the two items");
await p.screenshot({ path: `${out}/5-linked.png` });

// A broken block from elsewhere: no throw, a badge with the count and the line in its tooltip, gone once fixed.
const broken = 'item "Broken" {\n    pos 0 0\n';
await p.evaluate((b: string) => (window as any).clients[0].ytext.insert((window as any).clients[0].ytext.length, b), broken);
await settle();
const badge = (map: string) => p.$eval(`${map} .tn-err`, (e: HTMLElement) => ({ hidden: e.hidden, text: e.textContent, title: e.title }));
const e1 = await badge("#m1");
const e2 = await badge("#m2");
check(!e1.hidden && e1.text === "1" && /^line \d+: unbalanced braces$/.test(e1.title) && !e2.hidden, `error badge on both maps (${e1.text}, "${e1.title}")`);
await p.locator("#m1 .tn-err").screenshot({ path: `${out}/6-badge.png` });
await p.screenshot({ path: `${out}/6-broken.png` });
await p.evaluate((n: number) => (window as any).clients[0].ytext.delete((window as any).clients[0].ytext.length - n, n), broken.length);
await settle();
check((await badge("#m1")).hidden && (await badge("#m2")).hidden, "badge hidden once the block is fixed");

// Remote delete of an item map 2 has selected: map 2 reports the selection without it.
await p.mouse.click(at.x, at.y);
await p.keyboard.press("Delete");
await settle();
const s2 = await sel(2);
check(!(await texts())[1].includes('item "Renamed"') && !s2.includes("item:Renamed#0") && s2.length > 0, `selection lost by a remote delete is reported ${JSON.stringify(s2)}`);

// Theme switch: both maps, peers still drawn.
await p.locator("#dark").check();
await settle();
const themes = await p.$$eval(".tn-mm", (els: HTMLElement[]) => els.map((e) => e.dataset.theme));
check(JSON.stringify(themes) === '["dark","dark"]', `theme switch reaches both maps ${themes}`);
check((await ink("#m1", PURPLE)) > 100, "peer outlines in the dark theme");
await p.screenshot({ path: `${out}/7-dark.png` });

check(problems.length === 0, `no console errors ${problems.slice(0, 3).join(" | ")}`);
await browser.close();
server.close();
if (failures.length > 0) {
  console.error(`${failures.length} check(s) failed`);
  process.exit(1);
}
