// Port of SceneDrawView.drawRulers: dots at the view edges for items that are outside it, the dot grows with their count.
import type { Drawable } from "@tenniarb/render";

const YCOUNT = 20;
const XCOUNT = 30;

/**
 * `ox`, `oy`: scene -> zoom space (zoom space is y-up and `k` times smaller than the screen, as Swift's scaled context).
 * Draws in screen space over a canvas of `w` x `h` CSS pixels.
 */
export function drawIndicators(ctx: CanvasRenderingContext2D, drawables: Iterable<Drawable>, o: { ox: number; oy: number; k: number; w: number; h: number; dpr: number; dark: boolean }): void {
  const { ox, oy, k, dpr } = o;
  const [w, h] = [o.w / k, o.h / k];
  const [ystep, xstep] = [h / YCOUNT, w / XCOUNT];
  const left = new Array<number>(YCOUNT + 1).fill(0);
  const right = new Array<number>(YCOUNT + 1).fill(0);
  const top = new Array<number>(XCOUNT + 1).fill(0);
  const bottom = new Array<number>(XCOUNT + 1).fill(0);
  const clampY = (v: number): number => Math.min(Math.max(Math.trunc(v / ystep), 0), YCOUNT);

  for (const d of drawables) {
    const db = d.getBounds();
    const x = db.x + ox;
    const y = db.y + oy;
    if (x + db.width < 0) left[clampY(y + db.height / 2)]!++;
    if (x > w) right[clampY(y + db.height / 2)]!++;
    // Items above or below fall into the x-column; those beyond the corners count towards the side boxes.
    const xpos = Math.trunc((x + db.width / 2) / xstep);
    if (y + db.height < 0) {
      if (xpos >= XCOUNT) right[0]!++;
      else if (xpos <= 0) left[0]!++;
      else bottom[xpos]!++;
    }
    if (y > h) {
      if (xpos >= XCOUNT) right[YCOUNT]!++;
      else if (xpos <= 0) left[YCOUNT]!++;
      else top[xpos]!++;
    }
  }

  ctx.save();
  ctx.setTransform(dpr * k, 0, 0, -dpr * k, 0, dpr * o.h);
  const color = o.dark ? "rgb(227,157,68)" : "rgb(0,0,0)";
  ctx.fillStyle = color;
  ctx.strokeStyle = color;
  ctx.lineWidth = 1;
  ctx.shadowColor = "rgba(0,0,0,0.33)";
  ctx.shadowOffsetX = 3 * dpr;
  ctx.shadowOffsetY = 3 * dpr;
  ctx.shadowBlur = 5 * dpr;
  ctx.beginPath();
  const dot = (x: number, y: number, count: number): void => {
    const size = 5 + Math.min(count, 50) * 0.2;
    ctx.moveTo(x + size, y + size / 2);
    ctx.ellipse(x + size / 2, y + size / 2, size / 2, size / 2, 0, 0, Math.PI * 2);
  };
  const cap = (n: number): number => Math.min(n, 50) * 0.2;
  for (let i = 0; i <= YCOUNT; i++) {
    const y = ystep * i + 2 - (i === YCOUNT ? cap(left[i]!) + 7 : 0);
    if (left[i]! > 0) dot(2, y, left[i]!);
    if (right[i]! > 0) dot(w - 7 - cap(right[i]!), i === YCOUNT ? h - cap(right[i]!) - 7 : y, right[i]!);
  }
  for (let i = 0; i <= XCOUNT; i++) {
    const x = xstep * i + 2;
    if (bottom[i]! > 0) dot(x - (i === XCOUNT ? cap(bottom[i]!) + 7 : 0), 2, bottom[i]!);
    if (top[i]! > 0) dot(x - (i === XCOUNT ? cap(top[i]!) + 7 : 0), h - 7 - cap(top[i]!), top[i]!);
  }
  ctx.fill();
  ctx.stroke();
  ctx.restore();
}
