// Port of the static drawing part of ElementScene.swift. Geometry stays in Swift's y-up space; renderElement flips the
// canvas once (scale(1, -1)), and anything with an upright orientation (text, images) flips back locally.
// Editor-only (stage 7): find/hit-test, SelectorBox, SelectorLine, DrawableLayer.Hover states driven by selection.
import type { Canvas2D } from "./canvas-types.ts";
import { cssColor } from "./color.ts";
import type { Color } from "./color.ts";
import { crossBox, insetBy, intersects, maxX, maxY, midX, midY, minX, minY, rectZero, union } from "./geometry.ts";
import type { Point, Rect, Size } from "./geometry.ts";
import { DrawableLineStyle, DrawableStyle, DrawableItemStyle, updateLineStyle } from "./style.ts";
import { TextBox, calculateSize, getTokens, toAttributedString } from "./text.ts";
import type { ImageLookup } from "./text.ts";
import type { DiagramItem } from "@tenniarb/core";

// Some debug variabes
export const OPTION_perform_clip = true;

/** CGContext default shadow colour: black at 1/3 alpha. */
const defaultShadowColor: Color = { r: 0, g: 0, b: 0, a: 1 / 3 };

/** @napi-rs/canvas squares shadowColor alpha when shadowBlur > 0 (PNG and PDF); the Node renderer sets 0.5 to undo it. */
export const canvasQuirks = { shadowAlphaPower: 1 };

// CGContext.setShadow(offset:blur:color:). The offset is y-up in Swift; the canvas shadow offset is in device space, which is y-down.
function setShadow(context: Canvas2D, offset: Size, blur: number, color: Color | null): void {
  context.shadowOffsetX = offset.width;
  context.shadowOffsetY = -offset.height;
  context.shadowBlur = blur;
  const c = color ?? defaultShadowColor;
  context.shadowColor = cssColor({ ...c, a: blur > 0 ? c.a ** canvasQuirks.shadowAlphaPower : c.a });
}

/** CGMutablePath: recorded so it can be replayed into the context and measured. */
export class Path {
  private ops: (readonly ["M" | "L", number, number] | readonly ["A", number, number, number, number, number] | readonly ["Q", number, number, number, number] | readonly ["Z"])[] = [];

  move(to: Point): void {
    this.ops.push(["M", to.x, to.y]);
  }

  line(to: Point): void {
    this.ops.push(["L", to.x, to.y]);
  }

  /** addArc(tangent1End:tangent2End:radius:) */
  arc(t1: Point, t2: Point, radius: number): void {
    this.ops.push(["A", t1.x, t1.y, t2.x, t2.y, radius]);
  }

  quad(to: Point, control: Point): void {
    this.ops.push(["Q", control.x, control.y, to.x, to.y]);
  }

  close(): void {
    this.ops.push(["Z"]);
  }

  /** CGPath.boundingBox: all points including control points. */
  get boundingBox(): Rect {
    let x0 = Infinity;
    let y0 = Infinity;
    let x1 = -Infinity;
    let y1 = -Infinity;
    for (const op of this.ops) {
      for (let i = 1; i < op.length - (op[0] === "A" ? 1 : 0); i += 2) {
        x0 = Math.min(x0, op[i] as number);
        x1 = Math.max(x1, op[i] as number);
        y0 = Math.min(y0, op[i + 1] as number);
        y1 = Math.max(y1, op[i + 1] as number);
      }
    }
    return { x: x0, y: y0, width: x1 - x0, height: y1 - y0 };
  }

  /** context.addPath(path) on a fresh canvas path. */
  addTo(context: Canvas2D): void {
    for (const op of this.ops) {
      switch (op[0]) {
        case "M":
          context.moveTo(op[1], op[2]);
          break;
        case "L":
          context.lineTo(op[1], op[2]);
          break;
        case "A":
          context.arcTo(op[1], op[2], op[3], op[4], op[5]);
          break;
        case "Q":
          context.quadraticCurveTo(op[1], op[2], op[3], op[4]);
          break;
        case "Z":
          context.closePath();
          break;
      }
    }
  }
}

export type DrawableLayer = "Background" | "Default" | "Hover";

/** A basic drawable element */
export interface Drawable {
  isVisible(): boolean;

  isSelectable(): boolean;

  /** raw drag */
  drawBox(context: Canvas2D, at: Point): void;

  draw(context: Canvas2D, at: Point): void;

  /** Layout children */
  layout(bounds: Rect, dirty: Rect): void;

  getSelectorBounds(): Rect;

  /** Return bounds of element */
  getBounds(): Rect;

  traverse(op: (itm: Drawable) => boolean): void;
}

export class ItemDrawable implements Drawable {
  item: DiagramItem | null = null;
  visible = true;
  layer: DrawableLayer = "Default";

  bounds: Rect = rectZero();

  drawBox(_context: Canvas2D, _at: Point): void {}

  draw(_context: Canvas2D, _at: Point): void {}

  layout(_bounds: Rect, _dirty: Rect): void {
    this.visible = true;
  }

  getSelectorBounds(): Rect {
    return this.getBounds();
  }

  getBounds(): Rect {
    return this.bounds;
  }

  isVisible(): boolean {
    return this.visible;
  }

  traverse(op: (itm: Drawable) => boolean): void {
    op(this);
  }

  isSelectable(): boolean {
    return this.layer === "Default";
  }
}

export class DrawableContainer extends ItemDrawable {
  children: Drawable[] | null = null;

  constructor(childs: Drawable[] = []) {
    super();
    if (childs.length > 0) {
      this.children = [...childs];
    }
  }

  override traverse(op: (itm: Drawable) => boolean): void {
    if (op(this) && this.children !== null) {
      for (const c of this.children) {
        if (!op(c)) {
          return;
        }
      }
    }
  }

  append(child: Drawable): void {
    (this.children ??= []).push(child);
  }

  insert(child: Drawable, at: number): void {
    (this.children ??= []).splice(at, 0, child);
  }

  override drawBox(context: Canvas2D, at: Point): void {
    for (const c of this.children ?? []) {
      if (c.isVisible()) {
        c.drawBox(context, at);
      }
    }
  }

  override layout(_bounds: Rect, dirty: Rect): void {
    const selfBounds = this.getSelectorBounds();

    for (const c of this.children ?? []) {
      c.layout(selfBounds, dirty);
    }
    this.visible = intersects(dirty, this.getBounds());
  }

  override draw(context: Canvas2D, at: Point): void {
    const ch = this.children;
    if (ch === null) {
      return;
    }
    // Background draw
    for (const c of ch) {
      if (c instanceof ItemDrawable && c.layer === "Background" && c.isVisible()) {
        c.draw(context, at);
      }
    }
    // Normal draw, skip background, hover ones
    for (const c of ch) {
      if (c instanceof ItemDrawable && (c.layer === "Background" || c.layer === "Hover")) {
        continue;
      }
      if (c.isVisible()) {
        c.draw(context, at);
      }
    }
    // Hover draw
    for (const c of ch) {
      if (c instanceof ItemDrawable && c.layer === "Hover" && c.isVisible()) {
        c.draw(context, at);
      }
    }
  }

  override getBounds(): Rect {
    let rect = rectZero();
    let first = true;
    for (const c of this.children ?? []) {
      const cbounds = c.getBounds();
      if (first) {
        rect = cbounds;
        first = false;
      } else {
        rect = union(rect, cbounds);
      }
    }
    return rect;
  }
}

export function getShadowRect(bounds: Rect, style: DrawableStyle): Rect {
  const result: Rect = { x: bounds.x, y: bounds.y, width: bounds.width, height: bounds.height };
  // In case we had shadow, we need to extend bounds a bit.
  const shadow = style.shadow;
  if (shadow !== null) {
    if (shadow.width < 0) {
      result.x += shadow.width - style.shadowBlur * 2;
      result.width += -1 * shadow.width + style.shadowBlur * 4;
    }
    if (shadow.width > 0) {
      result.width += shadow.width + style.shadowBlur * 2;
    }

    if (shadow.height < 0) {
      result.y += shadow.height - style.shadowBlur * 2;
      result.height += -1 * shadow.height + style.shadowBlur * 4;
    }
    if (shadow.height > 0) {
      result.height += shadow.height + style.shadowBlur * 2;
    }
  }
  return result;
}

export class RoundBox extends DrawableContainer {
  style: DrawableItemStyle;

  fill = true;
  stack = 0;
  stackStep: Point = { x: 2, y: 2 };

  path: Path | null = null;

  constructor(bounds: Rect, style: DrawableItemStyle, fill: boolean) {
    super();
    this.style = style;
    this.fill = fill;
    this.setPath(bounds);
  }

  setPath(rect: Rect): void {
    const radius = [...this.style.cornerRadius];
    const baseRadius = radius.length > 0 ? radius[0]! : 8;
    while (radius.length < 4) {
      radius.push(baseRadius);
    }

    this.bounds = rect;
    const path = new Path();
    path.move({ x: midX(rect), y: minY(rect) });
    path.arc({ x: maxX(rect), y: minY(rect) }, { x: maxX(rect), y: maxY(rect) }, radius[2]!);
    path.arc({ x: maxX(rect), y: maxY(rect) }, { x: minX(rect), y: maxY(rect) }, radius[1]!);
    path.arc({ x: minX(rect), y: maxY(rect) }, { x: minX(rect), y: minY(rect) }, radius[0]!);
    path.arc({ x: minX(rect), y: minY(rect) }, { x: maxX(rect), y: minY(rect) }, radius[3]!);
    path.close();
    this.path = path;
  }

  override drawBox(context: Canvas2D, at: Point): void {
    // We only need to draw rect for shadow
    this.doDraw(context, at);
  }

  override draw(context: Canvas2D, at: Point): void {
    context.save();

    const pos = this.style.shadow;
    if (pos !== null) {
      context.save();
      setShadow(context, pos, this.style.shadowBlur, this.style.shadowColor);
      this.doDraw(context, at);
      context.restore();
    } else {
      this.doDraw(context, at);
    }

    const clipBounds: Rect = { x: this.bounds.x + at.x, y: this.bounds.y + at.y, width: this.bounds.width - 1, height: this.bounds.height };
    if (OPTION_perform_clip) {
      context.beginPath();
      context.rect(clipBounds.x, clipBounds.y, clipBounds.width, clipBounds.height);
      context.clip();
    }
    super.draw(context, { x: minX(this.bounds) + at.x, y: minY(this.bounds) + at.y });
    context.restore();
  }

  doDraw(context: Canvas2D, at: Point): void {
    context.save();

    context.lineWidth = this.style.lineWidth;
    context.strokeStyle = cssColor(this.style.borderColor);
    context.fillStyle = cssColor(this.style.color);

    updateLineStyle(context, this.style);

    context.translate(at.x, at.y);

    for (let i = 1; i <= this.stack; i++) {
      context.save();
      setShadow(context, { width: 1, height: 1 }, 5, null);
      context.translate(this.stackStep.x * (this.stack - i), this.stackStep.y * (this.stack - i));
      context.beginPath();
      this.path!.addTo(context);
      context.fill();

      context.restore();
    }

    context.beginPath();
    this.path!.addTo(context);

    if (this.fill) {
      context.fill();
      context.stroke();
    } else {
      context.stroke();
    }

    context.restore();
  }

  override layout(_bounds: Rect, dirty: Rect): void {
    const selfBounds = this.bounds;

    for (const c of this.children ?? []) {
      c.layout(selfBounds, dirty);
    }
    this.visible = intersects(dirty, this.getBounds());
  }

  override getSelectorBounds(): Rect {
    return this.bounds;
  }

  override getBounds(): Rect {
    return insetBy(getShadowRect(this.bounds, this.style), -1 * this.style.lineWidth, -1 * this.style.lineWidth);
  }
}

export class EmptyBox extends DrawableContainer {
  style: DrawableStyle;

  constructor(bounds: Rect, style: DrawableStyle) {
    super();
    this.style = style;
    this.bounds = bounds;
  }

  setPath(rect: Rect): void {
    this.bounds = rect;
  }

  override drawBox(_context: Canvas2D, _at: Point): void {
    // We only need to draw rect for shadow
  }

  override draw(context: Canvas2D, at: Point): void {
    context.save();

    const pos = this.style.shadow;
    if (pos !== null) {
      setShadow(context, pos, this.style.shadowBlur, this.style.shadowColor);
    }
    if (OPTION_perform_clip) {
      context.beginPath();
      context.rect(this.bounds.x + at.x, this.bounds.y + at.y, this.bounds.width, this.bounds.height);
      context.clip();
    }
    super.draw(context, { x: minX(this.bounds) + at.x, y: minY(this.bounds) + at.y });
    context.restore();
  }

  override getBounds(): Rect {
    return this.bounds;
  }
}

export class InvisibleBox extends EmptyBox {
  override isVisible(): boolean {
    return false;
  }
}

export function arrow(from: Point, to: Point, tailWidth: number, headWidth: number, headLength: number): Path | null {
  const length = Math.hypot(to.x - from.x, to.y - from.y);
  if (length < 5) {
    return null;
  }
  const tailLength = length - headLength;

  const points: Point[] = [
    { x: 0, y: tailWidth / 2.0 },
    { x: tailLength, y: tailWidth / 2.0 },
    { x: tailLength, y: headWidth / 2.0 },
    { x: length, y: 0 },
    { x: tailLength, y: -headWidth / 2.0 },
    { x: tailLength, y: -tailWidth / 2.0 },
    { x: 0, y: -tailWidth / 2.0 },
  ];

  const cosine = (to.x - from.x) / length;
  const sine = (to.y - from.y) / length;
  // CGAffineTransform(a: cosine, b: sine, c: -sine, d: cosine, tx: start.x, ty: start.y)
  const path = new Path();
  points.forEach((p, i) => {
    const q = { x: cosine * p.x - sine * p.y + from.x, y: sine * p.x + cosine * p.y + from.y };
    if (i === 0) {
      path.move(q);
    } else {
      path.line(q);
    }
  });
  path.close();
  return path;
}

export class ImageBox implements Drawable {
  pos: Point;
  size: Size;
  img: unknown;

  constructor(pos: Point, size: Size, img: unknown) {
    this.pos = pos;
    this.size = size;
    this.img = img;
  }

  traverse(op: (itm: Drawable) => boolean): void {
    op(this);
  }

  drawBox(context: Canvas2D, at: Point): void {
    this.draw(context, at);
  }

  draw(context: Canvas2D, at: Point): void {
    // drawImage is upright in y-down space, so flip around the image's own rect.
    context.save();
    context.translate(this.pos.x + at.x, this.pos.y + at.y + this.size.height);
    context.scale(1, -1);
    context.drawImage(this.img, 0, 0, this.size.width, this.size.height);
    context.restore();
  }

  layout(_bounds: Rect, _dirty: Rect): void {}

  isVisible(): boolean {
    return true;
  }

  isSelectable(): boolean {
    return true;
  }

  getSelectorBounds(): Rect {
    return this.getBounds();
  }

  getBounds(): Rect {
    return { x: this.pos.x, y: this.pos.y, width: this.size.width, height: this.size.height };
  }
}

export class CircleBox extends DrawableContainer {
  style: DrawableStyle;
  fill = true;

  constructor(bounds: Rect, style: DrawableStyle, fill: boolean) {
    super();
    this.style = style;
    this.fill = fill;
    this.setPath(bounds);
  }

  setPath(rect: Rect): void {
    this.bounds = rect;
  }

  override drawBox(context: Canvas2D, at: Point): void {
    // We only need to draw rect for shadow
    this.doDraw(context, at);
  }

  override draw(context: Canvas2D, at: Point): void {
    context.save();

    context.save();
    const pos = this.style.shadow;
    if (pos !== null) {
      setShadow(context, pos, this.style.shadowBlur, this.style.shadowColor);
    }
    context.beginPath();
    this.doDraw(context, at);
    context.restore();
    const clipBounds: Rect = { x: this.bounds.x + at.x + 2, y: this.bounds.y + at.y + 2, width: this.bounds.width - 4, height: this.bounds.height - 4 };
    if (OPTION_perform_clip) {
      context.beginPath();
      addEllipse(context, clipBounds);
      context.clip();
    }

    super.draw(context, { x: minX(this.bounds) + at.x, y: minY(this.bounds) + at.y });
    context.restore();
  }

  doDraw(context: Canvas2D, at: Point): void {
    context.save();

    context.lineWidth = this.style.lineWidth;
    updateLineStyle(context, this.style);
    context.strokeStyle = cssColor(this.style.borderColor);
    context.fillStyle = cssColor(this.style.color);

    context.translate(at.x, at.y);

    context.beginPath();
    addEllipse(context, this.bounds);

    if (this.fill) {
      context.fill();
      context.stroke();
    } else {
      context.stroke();
    }

    context.restore();
  }

  override layout(_bounds: Rect, dirty: Rect): void {
    const selfBounds = this.bounds;

    for (const c of this.children ?? []) {
      c.layout(selfBounds, dirty);
    }
    this.visible = intersects(dirty, this.getBounds());
  }

  override getSelectorBounds(): Rect {
    return this.bounds;
  }

  override getBounds(): Rect {
    return getShadowRect(this.bounds, this.style);
  }
}

// CGContext.addEllipse(in:)
function addEllipse(context: Canvas2D, r: Rect): void {
  context.ellipse(midX(r), midY(r), Math.abs(r.width) / 2, Math.abs(r.height) / 2, 0, 0, Math.PI * 2);
}

export class DrawableLine extends ItemDrawable {
  source: Point = { x: 0, y: 0 };
  target: Point = { x: 0, y: 0 };

  extraPoints: Point[] = [];

  sourceRect: Rect = rectZero();
  targetRect: Rect = rectZero();
  lineWidth = 1;
  style: DrawableLineStyle;
  control: Point;

  label: TextBox | null = null;

  /** Either between two boxes (laid out right away) or between two exact points. */
  constructor(source: Rect | Point, target: Rect | Point, style: DrawableLineStyle, control: Point = { x: 0, y: 0 }) {
    super();
    this.style = style;
    this.control = control;
    if ("width" in source && "width" in target) {
      this.sourceRect = source;
      this.targetRect = target;
      this.updateLayout(source, target);
    } else {
      this.source = source as Point;
      this.target = target as Point;
    }
  }

  addLabel(label: string, imageProvider: ImageLookup): void {
    const shift: Point = { x: 0, y: 0 };
    const attrStr = toAttributedString(
      getTokens(label.replaceAll("\\n", "\n")),
      { size: this.style.fontSize, bold: false, italic: false },
      this.style.borderColor,
      shift,
      imageProvider,
      ["Center", "Middle"],
    );
    const size = calculateSize(attrStr);
    this.label = new TextBox(attrStr, { x: 0, y: 0, width: size.width, height: size.height });
  }

  updateLayout(sr: Rect, tr: Rect): void {
    this.extraPoints = [];

    const layout = this.style.layout;
    if (layout !== null && layout.startsWith("middle")) {
      const x1 = sr.x;
      const x2 = tr.x;
      const w1 = sr.width;
      const w2 = tr.width;

      const y1 = sr.y;
      const y2 = tr.y;
      const h1 = sr.height;
      const h2 = tr.height;

      const cx = !(x1 + w1 < x2 || x1 > x2 + w2);
      const cy = !(y1 + h1 < y2 || y1 > y2 + h2);

      if (cx && !cy) {
        // Variant A
        if (y1 + h1 < y2) {
          // Below
          this.source = { x: x1 + w1, y: y1 + h1 / 2.0 };
          this.target = { x: x2 + w2, y: y2 + h2 / 2.0 };

          const offset = Math.max(x1 + w1, x2 + w2) + 20;

          this.extraPoints.push({ x: offset, y: this.source.y });
          this.extraPoints.push({ x: offset, y: this.target.y });
        } else {
          // Under
          this.source = { x: x1, y: y1 + h1 / 2.0 };
          this.target = { x: x2, y: y2 + h2 / 2.0 };

          const offset = Math.min(x1, x2) - 20;

          this.extraPoints.push({ x: offset, y: this.source.y });
          this.extraPoints.push({ x: offset, y: this.target.y });
        }
      } else if (!cx && cy) {
        // Variant B
        if (x1 + w1 < x2) {
          // Left
          this.source = { x: x1 + w1 / 2, y: y1 };
          this.target = { x: x2 + w2 / 2, y: y2 };

          const offset = Math.min(y1, y2) - 20;

          this.extraPoints.push({ x: this.source.x, y: offset });
          this.extraPoints.push({ x: this.target.x, y: offset });
        } else {
          // Right
          this.source = { x: x1 + w1 / 2, y: y1 + h1 };
          this.target = { x: x2 + w2 / 2, y: y2 + h2 };

          const offset = Math.max(y1 + h1, y2 + h2) + 20;

          this.extraPoints.push({ x: this.source.x, y: offset });
          this.extraPoints.push({ x: this.target.x, y: offset });
        }
      } else if (x1 + w1 < x2) {
        // variant C, Left
        if (y1 + h1 < y2) {
          // Up
          this.source = { x: x1 + w1, y: y1 + h1 / 2 };
          this.target = { x: x2 + w2 / 2, y: y2 };
          this.extraPoints.push({ x: this.target.x, y: this.source.y });
        } else {
          // Down
          this.source = { x: x1 + w1 / 2, y: y1 };
          this.target = { x: x2, y: y2 + h2 / 2 };
          this.extraPoints.push({ x: this.source.x, y: this.target.y });
        }
      } else if (y1 + h1 < y2) {
        // Right and Up
        this.source = { x: x1 + w1 / 2, y: y1 + h1 };
        this.target = { x: x2 + w2, y: y2 + h2 / 2 };
        this.extraPoints.push({ x: this.source.x, y: this.source.y });
      } else {
        this.source = { x: x1, y: y1 + h2 / 2 };
        this.target = { x: x2 + w2 / 2, y: y2 + h2 };
        this.extraPoints.push({ x: this.target.x, y: this.source.y });
      }
    } else {
      const p1 = { x: midX(sr), y: midY(sr) };
      const p2 = { x: midX(tr), y: midY(tr) };

      const hasControl = this.control.x !== 0 || this.control.y !== 0;

      let fromToLen = Math.sqrt(Math.pow(p1.x - p2.x, 2) + Math.pow(p1.y - p2.y, 2));
      if (fromToLen === 0) {
        fromToLen = 1;
      }
      const fTo = { x: (p2.x - p1.x) / fromToLen, y: (p2.y - p1.y) / fromToLen };
      const ctrlPoint = { x: p1.x + this.control.x + (fTo.x * fromToLen) / 2, y: p1.y + (fTo.y * fromToLen) / 2 + this.control.y };

      if (hasControl) {
        this.extraPoints.push(ctrlPoint);
      }
      this.source = crossBox(p1, hasControl ? ctrlPoint : p2, sr) ?? p1;
      this.target = crossBox(hasControl ? ctrlPoint : p1, p2, tr) ?? p2;
    }
  }

  override drawBox(context: Canvas2D, at: Point): void {
    this.draw(context, at);
  }

  private getLabelPosition(lbl: TextBox): Point {
    const lblBounds = lbl.getBounds();
    let nx = (this.source.x + this.target.x) / 2;

    if (Math.abs(this.source.y - this.target.y) < 10) {
      nx -= lblBounds.width / 2;
    }
    let ny = (this.source.y + this.target.y) / 2;
    if (Math.abs(this.source.x - this.target.x) < 10) {
      ny -= lblBounds.height / 2;
    }
    return { x: nx + 5, y: ny };
  }

  /** Path, label position, draw-arrow flag, and where the line really starts/ends once shortened for arrows. */
  buildPath(point: Point): { path: Path; labelPoint: Point | null; drawArrow: boolean; fromPt: Point; toPt: Point } {
    const path = new Path();
    let labelPoint: Point | null = null;
    const drawArrowTarget = this.style.display === "arrow" || this.style.display === "arrows";
    const drawArrowSoure = this.style.display === "arrow-source" || this.style.display === "arrows";
    const drawArrow = drawArrowTarget || drawArrowSoure;

    let fromPt: Point = { x: this.source.x + point.x, y: this.source.y + point.y };
    let fromPtLast = fromPt;
    let toPt: Point = { x: this.target.x + point.x, y: this.target.y + point.y };

    path.move(fromPt);

    const quad = this.style.layout === "quad";

    let quadCp = fromPt;
    for (const ep of this.extraPoints) {
      // Move from pt to new location
      fromPtLast = { x: ep.x + point.x, y: ep.y + point.y };

      if (drawArrowSoure) {
        // We need to move source point a bit less
        const px = fromPtLast.x - fromPt.x;
        const py = fromPtLast.y - fromPt.y;

        const plen = Math.sqrt(px * px + py * py);
        if (plen > 10) {
          const ltoPt = { x: fromPt.x + (px / plen) * 5, y: fromPt.y + (py / plen) * 5 };
          path.move(ltoPt);
          fromPt = ltoPt;
        }
      }
      if (quad) {
        quadCp = fromPtLast;
      } else {
        path.line(fromPtLast);
      }

      if (this.label !== null) {
        labelPoint = { x: ep.x + 5, y: ep.y - this.label.getBounds().height };
      }
    }

    if (this.extraPoints.length === 0 && this.label !== null) {
      labelPoint = this.getLabelPosition(this.label);
    }

    if (drawArrowTarget) {
      // We need to draw a bit less
      const px = toPt.x - fromPtLast.x;
      const py = toPt.y - fromPtLast.y;

      const plen = Math.sqrt(px * px + py * py);

      if (plen > 10) {
        const ltoPt = { x: toPt.x - (px / plen) * 5, y: toPt.y - (py / plen) * 5 };
        if (quad) {
          path.quad(ltoPt, quadCp);
        } else {
          path.line(ltoPt);
        }
        toPt = ltoPt;
      } else if (quad) {
        path.quad(toPt, quadCp);
      } else {
        path.line(toPt);
      }
    } else if (quad) {
      path.quad(toPt, quadCp);
    } else {
      path.line(toPt);
    }
    return { path, labelPoint, drawArrow, fromPt, toPt };
  }

  override draw(context: Canvas2D, at: Point): void {
    context.save();

    context.lineWidth = this.lineWidth;
    context.strokeStyle = cssColor(this.style.color);
    context.fillStyle = cssColor(this.style.color);

    const pos = this.style.shadow;
    if (pos !== null) {
      setShadow(context, pos, this.style.shadowBlur, this.style.shadowColor);
    }

    updateLineStyle(context, this.style);

    const { path, labelPoint, drawArrow, fromPt, toPt } = this.buildPath(at);

    context.beginPath();
    path.addTo(context);
    context.stroke();

    if (drawArrow) {
      const extra = this.extraPoints;
      let spt = extra.length > 0 ? { x: extra[0]!.x + at.x, y: extra[0]!.y + at.y } : toPt;
      let ept = extra.length > 0 ? { x: extra[extra.length - 1]!.x + at.x, y: extra[extra.length - 1]!.y + at.y } : fromPt;

      context.beginPath();
      if (this.style.display === "arrow" || this.style.display === "arrows") {
        const px = ept.x - toPt.x;
        const py = ept.y - toPt.y;

        const plen = Math.sqrt(px * px + py * py);
        if (plen > 10) {
          ept = { x: toPt.x + (px / plen) * 10, y: toPt.y + (py / plen) * 10 };
        }
        arrow(ept, toPt, 0, 10, 10)?.addTo(context);
      }
      if (this.style.display === "arrow-source" || this.style.display === "arrows") {
        const px = spt.x - fromPt.x;
        const py = spt.y - fromPt.y;

        const plen = Math.sqrt(px * px + py * py);
        if (plen > 10) {
          spt = { x: fromPt.x + (px / plen) * 10, y: fromPt.y + (py / plen) * 10 };
        }
        arrow(spt, fromPt, 0, 10, 10)?.addTo(context);
      }
      context.fill();
      context.stroke();
    }

    if (this.label !== null && labelPoint !== null) {
      this.label.draw(context, { x: at.x + labelPoint.x, y: at.y + labelPoint.y });
    }

    context.restore();
  }

  override layout(_bounds: Rect, _dirty: Rect): void {}

  override isVisible(): boolean {
    return true;
  }

  override getBounds(): Rect {
    let minXv = Math.min(this.source.x, this.target.x);
    let maxXv = Math.max(this.source.x, this.target.x);
    let minYv = Math.min(this.source.y, this.target.y);
    let maxYv = Math.max(this.source.y, this.target.y);

    for (const ep of this.extraPoints) {
      minXv = Math.min(ep.x, minXv);
      maxXv = Math.max(ep.x, maxXv);
      minYv = Math.min(ep.y, minYv);
      maxYv = Math.max(ep.y, maxYv);
    }

    const { path, drawArrow } = this.buildPath({ x: 0, y: 0 });

    const box = path.boundingBox;

    minXv = Math.min(minX(box), minXv);
    maxXv = Math.max(maxX(box), maxXv);
    minYv = Math.min(minY(box), minYv);
    maxYv = Math.max(maxY(box), maxYv);

    if (drawArrow) {
      minXv -= 10;
      minYv -= 10;
      maxXv += 10;
      maxYv += 10;
    }

    return insetBy(
      { x: minXv, y: minYv, width: Math.abs(maxXv - minXv), height: Math.max(Math.abs(maxYv - minYv), 5.0) },
      -1 * this.style.lineWidth,
      -1 * this.style.lineWidth,
    );
  }

  override getSelectorBounds(): Rect {
    let minXv = Math.min(this.source.x, this.target.x);
    let maxXv = Math.max(this.source.x, this.target.x);
    let minYv = Math.min(this.source.y, this.target.y);
    let maxYv = Math.max(this.source.y, this.target.y);

    for (const ep of this.extraPoints) {
      minXv = Math.min(ep.x, minXv);
      maxXv = Math.max(ep.x, maxXv);
      minYv = Math.min(ep.y, minYv);
      maxYv = Math.max(ep.y, maxYv);

      if (this.label !== null) {
        const point = { x: ep.x + 5, y: ep.y - this.label.getBounds().height };
        const lblb = this.label.getSelectorBounds();
        maxXv = Math.max(maxXv, point.x + lblb.width + 5);
        minYv = Math.min(minYv, point.y - 5);
      }
    }
    if (this.extraPoints.length === 0 && this.label !== null) {
      const point = this.getLabelPosition(this.label);
      const lblb = this.label.getSelectorBounds();

      maxXv = Math.max(maxXv, point.x + lblb.width + 5);

      minYv = Math.min(minYv, point.y - 5);
    }
    return insetBy(
      { x: minXv, y: minYv, width: Math.abs(maxXv - minXv), height: Math.max(Math.abs(maxYv - minYv), 5.0) },
      -1 * this.style.lineWidth,
      -1 * this.style.lineWidth,
    );
  }
}
