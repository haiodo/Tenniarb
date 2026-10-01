// usage: node pw.mjs <chromium|webkit> <headed|headless> <dpr> <out.json> [channel-args]
import { chromium, webkit } from 'playwright';
import http from 'http'; import fs from 'fs';
const [,, br, mode, dpr, outf] = process.argv;
const srv = http.createServer((q, r) => {
  const f = '.' + (q.url.split('?')[0] === '/' ? '/bench.html' : q.url.split('?')[0]);
  try { const d = fs.readFileSync(f); r.setHeader('content-type', f.endsWith('.js') ? 'text/javascript' : 'text/html'); r.end(d); } catch { r.statusCode = 404; r.end(); }
}).listen(0);
const port = srv.address().port;
const b = await (br === 'webkit' ? webkit : chromium).launch({ headless: mode === 'headless', args: br === 'chromium' ? ['--disable-frame-rate-limit-not'] : [] });
const ctx = await b.newContext({ viewport: { width: 1280, height: 800 }, deviceScaleFactor: +dpr });
const p = await ctx.newPage();
p.on('pageerror', e => console.error('pageerror', e.message));
await p.goto(`http://localhost:${port}/`);
await p.waitForFunction('window.__results', null, { timeout: 120000 });
const res = await p.evaluate('window.__results');
res.env = { browser: br, mode, version: b.version() };
fs.writeFileSync(outf, JSON.stringify(res, null, 1));
console.log(br, mode, b.version(), res.ua);
for (const r of res.runs) console.log(r.n, r.shadows, 'med', r.medianFrameMs, 'p95', r.p95FrameMs, 'fps', r.fps, 'draw', r.medianDrawMs, r.p95DrawMs);
await b.close(); srv.close();
