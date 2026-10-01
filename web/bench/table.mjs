import fs from 'fs';
const fn = ['mac-tauri','mac-webkit-headed','mac-webkit-headless','mac-chromium-headed','mac-chromium-headless','linux-tauri','linux-webkit-headed','linux-chromium-headed'];
for (const f of fn) { const r = JSON.parse(fs.readFileSync(`results/${f}.json`)); console.log(`${f} dpr=${r.dpr} ${r.w}x${r.h}`);
  const row = s => [100,500,1000,2000].map(n => { const x = r.runs.find(x => x.n===n && x.shadows===s); return `${x.medianFrameMs}/${x.p95FrameMs}/${x.fps}|${x.medianDrawMs}/${x.p95DrawMs}`; }).join('  ');
  console.log(' sh on :', row(true)); console.log(' sh off:', row(false)); }
