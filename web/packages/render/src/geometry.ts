// CGRect / CGPoint helpers over core's plain Rect, plus the crossing math of SceneMath.swift the renderer needs.
import type { Rect } from "@tenniarb/core";

export type { Rect };

export interface Point {
  x: number;
  y: number;
}

export interface Size {
  width: number;
  height: number;
}

export const rectZero = (): Rect => ({ x: 0, y: 0, width: 0, height: 0 });

export const minX = (r: Rect): number => Math.min(r.x, r.x + r.width);
export const maxX = (r: Rect): number => Math.max(r.x, r.x + r.width);
export const minY = (r: Rect): number => Math.min(r.y, r.y + r.height);
export const maxY = (r: Rect): number => Math.max(r.y, r.y + r.height);
export const midX = (r: Rect): number => (minX(r) + maxX(r)) / 2;
export const midY = (r: Rect): number => (minY(r) + maxY(r)) / 2;

export function union(a: Rect, b: Rect): Rect {
  const x0 = Math.min(minX(a), minX(b));
  const y0 = Math.min(minY(a), minY(b));
  return { x: x0, y: y0, width: Math.max(maxX(a), maxX(b)) - x0, height: Math.max(maxY(a), maxY(b)) - y0 };
}

export function intersects(a: Rect, b: Rect): boolean {
  return minX(a) < maxX(b) && minX(b) < maxX(a) && minY(a) < maxY(b) && minY(b) < maxY(a);
}

export function insetBy(r: Rect, dx: number, dy: number): Rect {
  return { x: r.x + dx, y: r.y + dy, width: r.width - 2 * dx, height: r.height - 2 * dy };
}

export function containsRect(outer: Rect, inner: Rect): boolean {
  return minX(outer) <= minX(inner) && maxX(outer) >= maxX(inner) && minY(outer) <= minY(inner) && maxY(outer) >= maxY(inner);
}

// Calc point to cross two lines.
export function crossLine(p1: Point, p2: Point, p3: Point, p4: Point): Point | null {
  const d = (p1.x - p2.x) * (p4.y - p3.y) - (p1.y - p2.y) * (p4.x - p3.x);
  const da = (p1.x - p3.x) * (p4.y - p3.y) - (p1.y - p3.y) * (p4.x - p3.x);
  const db = (p1.x - p2.x) * (p1.y - p3.y) - (p1.y - p2.y) * (p1.x - p3.x);

  const ta = da / d;
  const tb = db / d;

  if (ta >= 0 && ta <= 1 && tb >= 0 && tb <= 1) {
    return { x: p1.x + ta * (p2.x - p1.x), y: p1.y + ta * (p2.y - p1.y) };
  }
  return null;
}

export function crossBox(p1: Point, p2: Point, rect: Rect): Point | null {
  const ox = rect.x;
  const oy = rect.y;
  return (
    crossLine(p1, p2, { x: ox, y: oy }, { x: ox + rect.width, y: oy }) ??
    crossLine(p1, p2, { x: ox, y: oy }, { x: ox, y: oy + rect.height }) ??
    crossLine(p1, p2, { x: ox, y: oy + rect.height }, { x: ox + rect.width, y: oy + rect.height }) ??
    crossLine(p1, p2, { x: ox + rect.width, y: oy }, { x: ox + rect.width, y: oy + rect.height })
  );
}
