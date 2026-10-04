// Hit-test, rubber band and selection overlay of SceneDrawView / DrawableScene (find, updateActiveElements, SelectorBox, SelectorLine).
import type { DiagramItem } from "@tenniarb/core";
import { DrawableLine, ItemDrawable, containsPoint, intersects } from "@tenniarb/render";
import type { Canvas2D, DrawableScene, Point, Rect } from "@tenniarb/render";

/** Items under `point` in build order (last is drawn on top); lines only count where no box is hit. `all`: also boxes on non-default layers (Swift allowAll). */
export function hitTest(scene: DrawableScene, point: Point, all = false): DiagramItem[] {
  const boxes: DiagramItem[] = [];
  const lines: DiagramItem[] = [];
  for (const [item, d] of scene.drawables) {
    if (!(d instanceof ItemDrawable) || !(all || d.isSelectable())) continue;
    if (d instanceof DrawableLine) {
      if (d.find(point)) lines.push(item);
    } else if (containsPoint(d.getSelectorBounds(), point)) {
      boxes.push(item);
    }
  }
  return boxes.length > 0 ? boxes : lines;
}

export function itemsInRect(scene: DrawableScene, rect: Rect): DiagramItem[] {
  return [...scene.drawables].filter(([, d]) => d.isSelectable() && intersects(rect, d.getBounds())).map(([item]) => item);
}

function dashed(ctx: Canvas2D, width: number): void {
  ctx.strokeStyle = "rgb(0,0,255)";
  ctx.lineWidth = width;
  ctx.shadowColor = "rgb(0,0,255)";
  ctx.shadowBlur = 5;
  ctx.setLineDash([5]);
  ctx.lineDashOffset = 5;
}

function box(ctx: Canvas2D, r: Rect, radius: number): void {
  const [x0, y0, x1, y1] = [r.x, r.y, r.x + r.width, r.y + r.height];
  ctx.beginPath();
  ctx.moveTo((x0 + x1) / 2, y0);
  ctx.arcTo(x1, y0, x1, y1, radius);
  ctx.arcTo(x1, y1, x0, y1, radius);
  ctx.arcTo(x0, y1, x0, y0, radius);
  ctx.arcTo(x0, y0, x1, y0, radius);
  ctx.closePath();
  ctx.stroke();
}

/** Call in scene space (the transform scene.draw uses). */
export function drawSelection(ctx: Canvas2D, scene: DrawableScene, selected: DiagramItem[], band: Rect | null): void {
  ctx.save();
  for (const item of selected) {
    const d = scene.drawables.get(item);
    if (d instanceof DrawableLine) {
      dashed(ctx, 1.5);
      ctx.beginPath();
      [d.source, ...d.extraPoints, d.target].forEach((p, i) => (i === 0 ? ctx.moveTo(p.x, p.y) : ctx.lineTo(p.x, p.y)));
      ctx.stroke();
    } else if (d !== undefined) {
      const b = d.getSelectorBounds();
      dashed(ctx, 1);
      box(ctx, { x: b.x - 5, y: b.y - 5, width: b.width + 10, height: b.height + 10 }, 9);
    }
  }
  if (band !== null) {
    dashed(ctx, 1);
    box(ctx, band, 0);
  }
  ctx.restore();
}
