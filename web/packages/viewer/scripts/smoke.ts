// Playwright smoke: node scripts/smoke.ts file.tenn outDir. Clicks every navigator row, checks console and canvas, times frames.
// Not part of CI. Playwright comes from web/bench/node_modules.
import { spawn } from "node:child_process";
import { mkdirSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";

const [file, out = ".work/viewer"] = process.argv.slice(2);
if (file === undefined) throw new Error("usage: smoke.ts file.tenn [outDir]");
mkdirSync(out, { recursive: true });
const { chromium } = createRequire(import.meta.url)("../../../bench/node_modules/playwright");

const server = spawn("node", [fileURLToPath(new URL("./serve.ts", import.meta.url)), resolve(file)], { env: { ...process.env, NO_OPEN: "1" } });
const url: string = await new Promise((res) => server.stdout.once("data", (d) => res(String(d).trim())));

const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1600, height: 1000 }, deviceScaleFactor: 2 });
const problems: string[] = [];
page.on("console", (m: { type(): string; text(): string }) => m.type() === "error" && !m.text().includes("has no items to draw") && problems.push(m.text()));
page.on("pageerror", (e: Error) => problems.push(String(e)));
await page.goto(url);
await page.waitForSelector(".row");

const names: string[] = await page.$$eval(".row .name", (els: HTMLElement[]) => els.map((e) => e.textContent ?? ""));
const results: { path: string; ms: number; build: number; blank: boolean }[] = [];
for (let i = 0; i < names.length; i++) {
  const row = (await page.$$(".row"))[i];
  await row.evaluate((r: HTMLElement) => r.click());
  await page.waitForFunction((n: string) => ((window as any).__viewer.name as string).endsWith(n), names[i]);
  const samples: number[] = [];
  for (let j = 0; j < 5; j++) {
    await page.mouse.move(800, 500);
    await page.mouse.wheel(0, j % 2 ? 40 : -40);
    await page.waitForTimeout(60);
    samples.push(await page.evaluate(() => (window as any).__viewer.drawMs));
  }
  const info = await page.evaluate(() => {
    const c = document.querySelector("#stage canvas") as HTMLCanvasElement | null;
    let blank = true;
    // Container rows (no items) show the "has no items" message instead of a canvas.
    if (c !== null) {
      const d = c.getContext("2d")!.getImageData(0, 0, c.width, c.height).data;
      for (let k = 3; k < d.length; k += 4) if (d[k] !== 0) { blank = false; break; }
    }
    return { blank, name: (window as any).__viewer.name, build: (window as any).__viewer.buildMs, items: (window as any).__viewer.items };
  });
  samples.sort((a, b) => a - b);
  results.push({ path: info.name, ms: samples[2], build: info.build, blank: info.blank && info.items > 0 });
}
writeFileSync(`${out}/results.json`, JSON.stringify({ problems, results }, null, 2));

await page.screenshot({ path: `${out}/page.png` });
const biggest = [...results].sort((a, b) => b.ms - a.ms).slice(0, 3);
for (const [i, r] of biggest.entries()) {
  await page.evaluate((h: string) => (location.hash = "/" + h.split("/").map(encodeURIComponent).join("/")), r.path);
  await page.waitForFunction((n: string) => (window as any).__viewer.name === n, r.path);
  await page.waitForTimeout(200);
  await page.locator("#main").screenshot({ path: `${out}/big${i + 1}.png` });
}
await browser.close();
server.kill();

const ms = results.map((r) => r.ms).sort((a, b) => a - b);
console.log(`elements ${results.length}, draw ms median ${ms[ms.length >> 1].toFixed(1)} max ${ms.at(-1)!.toFixed(1)}`);
console.log("blank:", results.filter((r) => r.blank).map((r) => r.path).join(", ") || "none");
console.log("console errors:", problems.length, problems.slice(0, 5));
if (problems.length > 0 || results.some((r) => r.blank)) process.exitCode = 1;
