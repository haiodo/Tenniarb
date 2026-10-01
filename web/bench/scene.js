// Tenniarb-like scene on the Canvas2D API. Mirrors the CGContext calls used in ElementScene.swift:
// rounded box paths, fill/stroke, shadow, dash, clip, translate, text, ellipses, links with arrows.

export function makeScene(n, seed = 1) {
  let s = seed;
  const rnd = () => ((s = (s * 16807) % 2147483647) / 2147483647);
  const cols = Math.ceil(Math.sqrt(n));
  const items = [];
  for (let i = 0; i < n; i++) {
    items.push({
      x: (i % cols) * 180 + rnd() * 40,
      y: Math.floor(i / cols) * 110 + rnd() * 30,
      w: 120 + rnd() * 40,
      h: 50,
      title: `Item ${i} ${['alpha', 'beta', 'gamma', 'delta'][i % 4]}`,
      body: i % 3 === 0 ? `value = ${Math.round(rnd() * 1000)} USD` : null,
      circle: i % 7 === 0,
    });
  }
  const links = [];
  for (let i = 1; i < n; i++) {
    links.push({ a: i, b: Math.floor(rnd() * i), dash: i % 5 === 0 });
  }
  return { items, links };
}

function roundRect(ctx, x, y, w, h, r) {
  ctx.beginPath();
  ctx.moveTo(x + r, y);
  ctx.arcTo(x + w, y, x + w, y + h, r);
  ctx.arcTo(x + w, y + h, x, y + h, r);
  ctx.arcTo(x, y + h, x, y, r);
  ctx.arcTo(x, y, x + w, y, r);
  ctx.closePath();
}

export function drawScene(ctx, scene, ox = 0, oy = 0, font = 'sans-serif') {
  const { items, links } = scene;
  ctx.save();
  ctx.translate(ox, oy);

  ctx.lineWidth = 1;
  ctx.strokeStyle = '#555';
  for (const l of links) {
    const a = items[l.a], b = items[l.b];
    const x1 = a.x + a.w / 2, y1 = a.y + a.h / 2, x2 = b.x + b.w / 2, y2 = b.y + b.h / 2;
    ctx.setLineDash(l.dash ? [4, 3] : []);
    ctx.beginPath();
    ctx.moveTo(x1, y1);
    ctx.lineTo(x2, y2);
    ctx.stroke();
    const ang = Math.atan2(y2 - y1, x2 - x1);
    ctx.setLineDash([]);
    ctx.beginPath();
    ctx.moveTo(x2, y2);
    ctx.lineTo(x2 - 8 * Math.cos(ang - 0.4), y2 - 8 * Math.sin(ang - 0.4));
    ctx.lineTo(x2 - 8 * Math.cos(ang + 0.4), y2 - 8 * Math.sin(ang + 0.4));
    ctx.closePath();
    ctx.fillStyle = '#555';
    ctx.fill();
  }

  for (const it of items) {
    ctx.save();
    ctx.shadowColor = 'rgba(0,0,0,0.35)';
    ctx.shadowBlur = 6;
    ctx.shadowOffsetX = 2;
    ctx.shadowOffsetY = 2;
    if (it.circle) {
      ctx.beginPath();
      ctx.ellipse(it.x + it.w / 2, it.y + it.h / 2, it.w / 2, it.h / 2, 0, 0, Math.PI * 2);
    } else {
      roundRect(ctx, it.x, it.y, it.w, it.h, 8);
    }
    ctx.fillStyle = '#fdf6e3';
    ctx.fill();
    ctx.restore();
    ctx.strokeStyle = '#268bd2';
    ctx.lineWidth = 1.5;
    ctx.stroke();

    ctx.save();
    ctx.clip();
    ctx.fillStyle = '#073642';
    ctx.font = `bold 13px ${font}`;
    ctx.fillText(it.title, it.x + 8, it.y + 20);
    if (it.body) {
      ctx.font = `12px ${font}`;
      ctx.fillText(it.body, it.x + 8, it.y + 38);
    }
    ctx.restore();
  }
  ctx.restore();
}
