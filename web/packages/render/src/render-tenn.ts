// Node-only: .tenn text -> PDF / PNG / SVG through @napi-rs/canvas. Not exported from the browser entry.
import type { Canvas } from "@napi-rs/canvas";
import { GlobalFonts, PDFDocument, createCanvas, loadImage } from "@napi-rs/canvas";
import { TennParser, parseTenn } from "@tenniarb/core";
import type { Element, TennError } from "@tenniarb/core";
import { allElements, findElement } from "./elements.ts";
import { preloadImages } from "./images.ts";
import { canvasQuirks } from "./drawable.ts";
import { getSceneSize, renderElement } from "./render.ts";
import type { RenderOptions } from "./render.ts";
import { registerInterFonts } from "./node.ts";
import type { Canvas2D } from "./canvas-types.ts";
import { SvgContext } from "./svg-context.ts";

export interface RenderTennOptions {
  format: "pdf" | "png" | "svg";
  /** Element name or "A/B" path. Default: every element with items for pdf (one page each), the first one for png/svg. */
  element?: string;
  /** png only, default 2. */
  scale?: number;
  background?: string;
  darkMode?: boolean;
}

export class TennParseError extends Error {
  readonly errors: readonly TennError[];
  constructor(errors: readonly TennError[]) {
    super(errors.map((e) => `${e.line}:${e.col}: ${e.message}`).join("\n"));
    this.errors = errors;
  }
}

// Swift ExportManager: the image is the scene bounds grown by 30 on every side (60 per axis), content drawn at offset 15.
const EXTRA = 60;

/** @napi-rs/canvas segfaults (at GC) on truncated image data, so only complete PNG/JPEG/GIF/WebP files get to the decoder. */
function looksComplete(b: Buffer): boolean {
  const ends = (hex: string): boolean => b.subarray(-(hex.length / 2)).equals(Buffer.from(hex, "hex"));
  if (b.subarray(1, 4).toString("latin1") === "PNG") return ends("49454e44ae426082");
  if (b[0] === 0xff && b[1] === 0xd8) return ends("ffd9");
  if (b.subarray(0, 3).toString("latin1") === "GIF") return ends("3b");
  if (b.subarray(0, 4).toString("latin1") === "RIFF" && b.subarray(8, 12).toString("latin1") === "WEBP") return b.readUInt32LE(4) + 8 === b.length;
  return false;
}

const dataUrlMime = (b: Buffer): string => (b[0] === 0xff ? "image/jpeg" : b[0] === 0x47 ? "image/gif" : b[0] === 0x52 ? "image/webp" : "image/png");

export async function renderTenn(text: string, options: RenderTennOptions & { format: "svg" }): Promise<string>;
export async function renderTenn(text: string, options: RenderTennOptions): Promise<Buffer>;
export async function renderTenn(text: string, options: RenderTennOptions): Promise<Buffer | string> {
  registerInterFonts(GlobalFonts);
  const parser = new TennParser();
  const tree = parser.parse(text.charCodeAt(0) === 0xfeff ? text.slice(1) : text);
  if (parser.errors.hasErrors()) throw new TennParseError(parser.errors.errors);
  const model = parseTenn(tree);

  let elements: Element[];
  if (options.element !== undefined) {
    const found = findElement(model, options.element);
    if (found === null) throw new Error(`element not found: ${options.element}`);
    elements = [found];
  } else {
    // Pure containers (no items) would be blank pages.
    elements = allElements(model).filter((e) => e.items.length > 0);
    if (options.format !== "pdf") elements = elements.slice(0, 1);
  }
  if (elements.length === 0) throw new Error("no elements to render");

  // Held until the end: a canvas collected while its context is in use crashes @napi-rs/canvas.
  const measure = createCanvas(1, 1);
  const measureCtx = measure.getContext("2d");
  const pdf = options.format === "pdf" ? new PDFDocument() : null;
  const scale = options.format === "png" ? (options.scale ?? 2) : 1;
  const keep: unknown[] = [measure, measureCtx, pdf];
  let out: Buffer | string = "";

  canvasQuirks.shadowAlphaPower = options.format === "svg" ? 1 : 0.5;
  for (const el of elements) {
    const decodeImage = await preloadImages(el, async (base64) => {
      const bytes = Buffer.from(base64, "base64");
      if (!looksComplete(bytes)) return null;
      const img = await loadImage(bytes);
      keep.push(img);
      if (options.format === "svg") {
        return { image: { src: `data:${dataUrlMime(bytes)};base64,${base64}`, naturalWidth: img.width, naturalHeight: img.height }, width: img.width, height: img.height };
      }
      return { image: img, width: img.width, height: img.height };
    });
    const opts: RenderOptions = { decodeImage, background: options.background, darkMode: options.darkMode, measureContext: measureCtx as Canvas2D };
    const size = getSceneSize(el, opts);
    const w = Math.ceil(size.width - 30 + EXTRA);
    const h = Math.ceil(size.height - 30 + EXTRA);

    let ctx: Canvas2D;
    let canvas: Canvas | null = null;
    let svg: SvgContext | null = null;
    if (pdf !== null) {
      ctx = pdf.beginPage(w, h) as unknown as Canvas2D;
    } else if (options.format === "svg") {
      svg = new SvgContext({ width: w, height: h, measureText: (font, t) => {
        measureCtx.font = font;
        return { width: measureCtx.measureText(t).width };
      } });
      ctx = svg as unknown as Canvas2D;
    } else {
      const c = createCanvas(Math.ceil(w * scale), Math.ceil(h * scale)) as Canvas;
      const g = c.getContext("2d");
      g.scale(scale, scale);
      canvas = c;
      ctx = g as unknown as Canvas2D;
    }
    keep.push(canvas, ctx);
    // renderElement pads the scene by 15; the export margin is 30 (Swift ExportManager), so shift by the other 15.
    if (options.background !== undefined) {
      ctx.fillStyle = options.background;
      ctx.fillRect(0, 0, w, h);
    }
    ctx.save();
    ctx.translate(EXTRA / 4, EXTRA / 4);
    renderElement(ctx, el, { ...opts, background: undefined, measureContext: undefined });
    ctx.restore();

    if (pdf !== null) pdf.endPage();
    else if (svg !== null) out = svg.toString();
    else out = canvas!.encodeSync("png");
  }
  if (pdf !== null) out = pdf.close();
  return keep.length > 0 ? out : "";
}
