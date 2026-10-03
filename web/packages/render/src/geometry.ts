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

/** CGRect.contains(point): min edges inclusive, max edges exclusive. */
export const containsPoint = (r: Rect, p: Point): boolean => p.x >= minX(r) && p.x < maxX(r) && p.y >= minY(r) && p.y < maxY(r);

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

/** Hit test of a point against segment p1-p2 with a 7 unit tolerance (SceneMath.crossPointLine). */
export function crossPointLine(p1: Point, p2: Point, p: Point): boolean {
  if (Math.hypot(p1.x - p.x, p1.y - p.y) < 7 || Math.hypot(p2.x - p.x, p2.y - p.y) < 7) {
    return true;
  }
  if (p1.x === p2.x && p1.y === p2.y) {
    return false;
  }
  const a = p2.y - p1.y;
  const b = p1.x - p2.x;
  const c = -(p1.x * a + p1.y * b);
  if (Math.abs(a * p.x + b * p.y + c) / Math.hypot(a, b) >= 7) {
    return false;
  }
  // The foot of the perpendicular must fall inside the segment's box, padded to 5 for flat lines.
  let r: Rect = { x: Math.min(p1.x, p2.x), y: Math.min(p1.y, p2.y), width: Math.abs(p1.x - p2.x), height: Math.abs(p1.y - p2.y) };
  if (r.height < 5) {
    r = { x: r.x, y: r.y - 2.5, width: r.width, height: r.height + 5 };
  }
  if (r.width < 5) {
    r = { x: r.x - 2.5, y: r.y, width: r.width + 5, height: r.height };
  }
  const d = a * a + b * b;
  return containsPoint(r, { x: (b * (b * p.x - a * p.y) - a * c) / d, y: (a * (-b * p.x + a * p.y) - b * c) / d });
}
