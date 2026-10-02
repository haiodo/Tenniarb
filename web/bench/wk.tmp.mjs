import { webkit } from 'playwright';
const b = await webkit.launch(); const p = await b.newPage({ viewport: { width: 1400, height: 900 } });
const msgs = []; p.on('console', m => msgs.push(m.type() + ': ' + m.text())); p.on('pageerror', e => msgs.push('pageerror: ' + e.message + '\n' + (e.stack || '').split('\n').slice(0, 4).join('\n')));
await p.goto('file:///Users/haiodo/Develop/private/tenniarb/.work/embed/swift-export-basic.html');
await p.waitForTimeout(3000);
console.log(JSON.stringify(await p.evaluate(() => [...document.querySelectorAll('canvas')].map(c => [c.width, c.height]))));
console.log(msgs.join('\n').slice(0, 3000));
await p.screenshot({ path: '/Users/haiodo/Develop/private/tenniarb/.work/embed/webkit.png' });
await b.close();
