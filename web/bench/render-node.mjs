// Headless render timing: makeScene(1000) to PNG/PDF/SVG via @napi-rs/canvas. Run with node or bun.
import { createCanvas, PDFDocument, SvgExportFlag } from '@napi-rs/canvas';
import { makeScene, drawScene } from './scene.js';

const scene = makeScene(1000), W = 6000, H = 3800, font = 'Arial';
const render = {
  png: () => { const c = createCanvas(W, H); drawScene(c.getContext('2d'), scene, 10, 10, font); return c.toBuffer('image/png'); },
  pdf: () => { const d = new PDFDocument(); drawScene(d.beginPage(W, H), scene, 10, 10, font); d.endPage(); return d.close(); },
  svg: () => { const c = createCanvas(W, H, SvgExportFlag.NoPrettyXML); drawScene(c.getContext('2d'), scene, 10, 10, font); return c.getContent(); },
};
const rt = typeof Bun !== 'undefined' ? 'bun' : 'node';
for (const [fmt, f] of Object.entries(render)) {
  const t = [];
  let bytes = 0;
  for (let i = 0; i < 5; i++) { const s = performance.now(); bytes = (await f()).length; t.push(performance.now() - s); }
  t.sort((a, b) => a - b);
  console.log(`${rt} ${fmt} n=1000 ${W}x${H} median ${t[2].toFixed(0)}ms bytes=${bytes}`);
}
