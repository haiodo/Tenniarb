// Replaces NSAttributedString + CoreText (CTFramesetter) with runs laid out on top of Canvas2D measureText.
// Swift's calculateSize never wraps (path width is greatestFiniteMagnitude); wrapping happens only when TextBox.draw
// runs the same layout at the frame width. Both follow that here.
import { MarkdownLexer, calcTitleFontSize, parseColor as parseMarkdownColor } from "@tenniarb/markdown";
import type { ImageProvider, MarkdownToken } from "@tenniarb/markdown";
import type { Canvas2D } from "./canvas-types.ts";
import { colorBlack, cssColor } from "./color.ts";
import type { Color } from "./color.ts";
import type { Drawable } from "./drawable.ts";
import type { Point, Rect, Size } from "./geometry.ts";

export const DEFAULT_FONT_FAMILY = "Inter";

let fontFamily = DEFAULT_FONT_FAMILY;

/** CSS family used by fontCss. Call before mount/render; text is measured per call, nothing is cached by family. */
export function setFontFamily(name: string): void {
  fontFamily = name;
}

export const getFontFamily = (): string => fontFamily;

// Line metrics of SF (NSFont.systemFont, CTLineGetTypographicBounds: 10pt -> 9.668/2.109, linear in size), not Inter's,
// so boxes get the Swift height. Inter 4 (unitsPerEm 2048) capHeight 1490 is only for strikethrough.
// Constants instead of fontBoundingBox*, which not every Canvas2D backend implements.
const ASCENT = 1980 / 2048;
const DESCENT = 432 / 2048;
const CAP_HEIGHT = 1490 / 2048;

export type TextPosition = "Left" | "Right" | "Center" | "Top" | "Fill" | "Bottom" | "Middle";

/** NSFont: only what the renderer varies. */
export interface FontSpec {
  size: number;
  bold: boolean;
  italic: boolean;
}

export const systemFont = (size: number): FontSpec => ({ size, bold: false, italic: false });

export function fontCss(f: FontSpec): string {
  const size = Number.isFinite(f.size) && f.size > 0 ? f.size : 18;
  return `${f.italic ? "italic " : ""}${f.bold ? "bold " : ""}${size}px ${fontFamily}, "Apple Color Emoji", "Noto Color Emoji", "Segoe UI Emoji", sans-serif`;
}

const fontAscent = (f: FontSpec): number => f.size * ASCENT;
const fontDescent = (f: FontSpec): number => f.size * DESCENT;
const fontCapHeight = (f: FontSpec): number => f.size * CAP_HEIGHT;

/** NSParagraphStyle */
export interface ParagraphStyle {
  alignment: "natural" | "left" | "center" | "right";
  lineSpacing: number;
  headIndent: number;
  paragraphSpacing: number;
  paragraphSpacingBefore: number;
}

export const defaultParagraphStyle = (): ParagraphStyle => ({
  alignment: "natural",
  lineSpacing: 0,
  headIndent: 0,
  paragraphSpacing: 0,
  paragraphSpacingBefore: 0,
});

/** NSTextAttachment: image plus bounds relative to the baseline (y up, as Swift). */
export interface Attachment {
  image: unknown;
  width: number;
  height: number;
  offsetY: number;
}

/** One uniformly attributed span of an NSAttributedString. */
export interface TextRun {
  text: string;
  font: FontSpec;
  paragraph: ParagraphStyle;
  color: Color | null;
  background: Color | null;
  underline: boolean;
  strikethrough: boolean;
  attachment: Attachment | null;
}

export class AttributedString {
  runs: TextRun[] = [];

  /** Plain text in one font, for callers without markdown (utils.textSize, tests). */
  static plain(text: string, font: FontSpec, paragraph: ParagraphStyle = defaultParagraphStyle()): AttributedString {
    const result = new AttributedString();
    result.append(text, font, paragraph);
    return result;
  }

  append(text: string | AttributedString, font?: FontSpec, paragraph?: ParagraphStyle, attrs: Partial<TextRun> = {}): void {
    if (text instanceof AttributedString) {
      this.runs.push(...text.runs);
      return;
    }
    this.runs.push({
      text,
      font: font!,
      paragraph: paragraph ?? defaultParagraphStyle(),
      color: null,
      background: null,
      underline: false,
      strikethrough: false,
      attachment: null,
      ...attrs,
    });
  }

  get string(): string {
    return this.runs.map((r) => r.text).join("");
  }

  get length(): number {
    return this.string.length;
  }
}

// MARK: - Measuring

let measureContext: Pick<Canvas2D, "font" | "measureText"> | null = null;

/** Any Canvas2D context with the bundled font registered; renderElement sets it to its own context. */
export function setMeasureContext(ctx: Pick<Canvas2D, "font" | "measureText"> | null): void {
  measureContext = ctx;
}

function textWidth(font: FontSpec, text: string): number {
  if (measureContext === null) {
    throw new Error("render: no text measurer, call setMeasureContext(ctx) or renderElement first");
  }
  measureContext.font = fontCss(font);
  return measureContext.measureText(text).width;
}

// MARK: - Layout

interface Segment {
  run: TextRun;
  text: string;
  width: number;
}

export interface TextLine {
  segments: Segment[];
  width: number;
  /** Ascent/descent of the fonts only, what CTLineGetTypographicBounds reports. */
  textAscent: number;
  textDescent: number;
  /** textAscent/textDescent with the font of the line's terminating "\n" (set on the last line of a paragraph). */
  endAscent: number;
  endDescent: number;
  /** Tallest inline image on the line. */
  maxImageHeight: number;
  paragraph: ParagraphStyle;
  /** Index inside its paragraph: continuation lines use headIndent. */
  index: number;
  lastOfParagraph: boolean;
}

interface Token {
  run: TextRun;
  text: string;
  width: number;
  space: boolean;
}

function newLine(paragraph: ParagraphStyle, index: number): TextLine {
  return {
    segments: [],
    width: 0,
    textAscent: 0,
    textDescent: 0,
    endAscent: 0,
    endDescent: 0,
    maxImageHeight: 0,
    paragraph,
    index,
    lastOfParagraph: false,
  };
}

function addToken(line: TextLine, tok: Token): void {
  const last = line.segments[line.segments.length - 1];
  if (last !== undefined && last.run === tok.run && tok.run.attachment === null) {
    last.text += tok.text;
    last.width += tok.width;
  } else {
    line.segments.push({ run: tok.run, text: tok.text, width: tok.width });
  }
  line.width += tok.width;
  line.textAscent = Math.max(line.textAscent, fontAscent(tok.run.font));
  line.textDescent = Math.max(line.textDescent, fontDescent(tok.run.font));
  if (tok.run.attachment !== null) {
    line.maxImageHeight = Math.max(line.maxImageHeight, tok.run.attachment.height);
  }
}

function tokenize(run: TextRun, text: string): Token[] {
  if (run.attachment !== null) {
    return [{ run, text, width: run.attachment.width, space: false }];
  }
  const result: Token[] = [];
  for (const m of text.matchAll(/[^ \t]+|[ \t]+/g)) {
    result.push({ run, text: m[0], width: textWidth(run.font, m[0]), space: m[0][0] === " " || m[0][0] === "\t" });
  }
  return result;
}

// A word wider than the line breaks between characters, as NSLayoutManager does.
function splitWord(tok: Token, avail: number): Token[] {
  const parts: Token[] = [];
  let cur = "";
  for (const ch of tok.text) {
    if (cur !== "" && textWidth(tok.run.font, cur + ch) > avail) {
      parts.push({ run: tok.run, text: cur, width: textWidth(tok.run.font, cur), space: false });
      cur = "";
    }
    cur += ch;
  }
  parts.push({ run: tok.run, text: cur, width: textWidth(tok.run.font, cur), space: false });
  return parts;
}

/** Lines of `attr`; with maxWidth null only "\n" breaks lines. A trailing "\n" does not start another line (CoreText). */
export function layoutLines(attr: AttributedString, maxWidth: number | null): TextLine[] {
  // Split into paragraphs of tokens; an empty paragraph keeps the run holding its "\n" for the font.
  const paragraphs: { tokens: Token[]; style: ParagraphStyle; emptyRun: TextRun; endRun: TextRun | null }[] = [];
  let tokens: Token[] = [];
  let style: ParagraphStyle | null = null;
  let emptyRun: TextRun | null = null;
  for (const run of attr.runs) {
    const parts = run.attachment !== null ? [run.text] : run.text.split("\n");
    parts.forEach((part, i) => {
      if (i > 0) {
        paragraphs.push({ tokens, style: style ?? run.paragraph, emptyRun: emptyRun ?? run, endRun: run });
        tokens = [];
        style = null;
        emptyRun = null;
      }
      if (part.length > 0) {
        style ??= run.paragraph;
        emptyRun ??= run;
        tokens.push(...tokenize(run, part));
      }
    });
  }
  if (tokens.length > 0) {
    paragraphs.push({ tokens, style: style!, emptyRun: emptyRun!, endRun: null });
  }

  const lines: TextLine[] = [];
  for (const p of paragraphs) {
    let line = newLine(p.style, 0);
    if (p.tokens.length === 0) {
      addToken(line, { run: p.emptyRun, text: "", width: 0, space: false });
    }
    const avail = (l: TextLine): number => (maxWidth ?? Infinity) - (l.index > 0 ? p.style.headIndent : 0);
    for (let tok of p.tokens) {
      const hasWord = line.segments.some((s) => s.run.attachment !== null || s.text.trim() !== "");
      if (!tok.space && hasWord && line.width + tok.width > avail(line)) {
        // Spaces at the wrap point hang past the edge and do not count.
        while (line.segments.length > 0 && line.segments[line.segments.length - 1]!.text.trim() === "" && line.segments[line.segments.length - 1]!.run.attachment === null) {
          line.width -= line.segments.pop()!.width;
        }
        lines.push(line);
        line = newLine(p.style, line.index + 1);
      }
      if (!tok.space && tok.run.attachment === null && line.width === 0 && tok.width > avail(line) && avail(line) > 0) {
        const pieces = splitWord(tok, avail(line));
        for (const piece of pieces.slice(0, -1)) {
          addToken(line, piece);
          lines.push(line);
          line = newLine(p.style, line.index + 1);
        }
        tok = pieces[pieces.length - 1]!;
      }
      if (tok.space && line.width === 0 && line.index > 0) {
        continue; // leading spaces of a continuation line are dropped
      }
      addToken(line, tok);
    }
    // CTLine includes the "\n" glyph's font, NSAttributedString.draw does not: only calculateSize sees it.
    line.endAscent = Math.max(line.textAscent, p.endRun === null ? 0 : fontAscent(p.endRun.font));
    line.endDescent = Math.max(line.textDescent, p.endRun === null ? 0 : fontDescent(p.endRun.font));
    lines.push(line);
    lines[lines.length - 1]!.lastOfParagraph = true;
  }
  return lines;
}

const lineAscent = (l: TextLine): number => Math.max(l.textAscent, l.endAscent);
const lineDescent = (l: TextLine): number => Math.max(l.textDescent, l.endDescent);

/** DrawableScene.calculateSize(attrStr:) */
export function calculateSize(attrStr: AttributedString): Size {
  const lines = layoutLines(attrStr, null);
  const size: Size = { width: 0, height: 0 };
  let maxWidth = 0;
  let frameHeight = 0;
  for (const l of lines) {
    let maxHeight = Math.floor(lineAscent(l) + lineDescent(l));
    if (l.maxImageHeight > 0 && l.maxImageHeight > maxHeight) {
      maxHeight = l.maxImageHeight;
    }
    let maxPsAdd = 0;
    for (const s of l.segments) {
      const ps = s.run.paragraph;
      maxPsAdd = Math.max(maxPsAdd, ps.paragraphSpacingBefore + ps.paragraphSpacing + ps.lineSpacing);
    }
    // Swift also weighs font.boundingRectForFont.height + descender, which never exceeds the line height for these fonts.
    maxWidth = Math.max(maxWidth, l.width);
    size.height += maxHeight + maxPsAdd + 1;

    frameHeight += lineAscent(l) + lineDescent(l) + l.paragraph.lineSpacing + (l.lastOfParagraph ? l.paragraph.paragraphSpacing : 0);
  }
  size.width = maxWidth;
  frameHeight = Math.ceil(frameHeight);
  if (size.height < frameHeight) {
    size.height = frameHeight;
  }
  return size;
}

// MARK: - Drawing

/** NSAttributedString.draw(in:) for a y-up context: lays out at `width`, first line hangs from the top of the rect. */
export function drawAttributedString(context: Canvas2D, attr: AttributedString, rect: Rect): void {
  const lines = layoutLines(attr, rect.width);
  context.save();
  // Into y-down text space with the origin at the rect's top-left corner.
  context.translate(rect.x, rect.y + rect.height);
  context.scale(1, -1);
  context.textBaseline = "alphabetic";

  let y = 0;
  for (const l of lines) {
    const ps = l.paragraph;
    if (l.index === 0) {
      y += ps.paragraphSpacingBefore;
    }
    const ascent = Math.max(l.textAscent, ...l.segments.map((s) => (s.run.attachment === null ? 0 : s.run.attachment.offsetY + s.run.attachment.height)));
    const descent = Math.max(l.textDescent, ...l.segments.map((s) => (s.run.attachment === null ? 0 : -s.run.attachment.offsetY)));
    const baseline = y + ascent;

    const indent = l.index > 0 ? ps.headIndent : 0;
    const avail = rect.width - indent;
    let x = indent;
    if (ps.alignment === "center") {
      x += (avail - l.width) / 2;
    } else if (ps.alignment === "right") {
      x += avail - l.width;
    }

    for (const s of l.segments) {
      const run = s.run;
      if (run.attachment !== null) {
        context.drawImage(run.attachment.image, x, baseline - run.attachment.offsetY - run.attachment.height, run.attachment.width, run.attachment.height);
      } else if (s.text.length > 0) {
        if (run.background !== null) {
          context.fillStyle = cssColor(run.background);
          context.fillRect(x, y, s.width, ascent + descent);
        }
        context.font = fontCss(run.font);
        context.fillStyle = cssColor(run.color ?? colorBlack);
        context.fillText(s.text, x, baseline);
        const thickness = Math.max(1, run.font.size / 14);
        if (run.underline) {
          context.fillRect(x, baseline + thickness, s.width, thickness);
        }
        if (run.strikethrough) {
          context.fillRect(x, baseline - fontCapHeight(run.font) / 2, s.width, thickness);
        }
      }
      x += s.width;
    }
    y += ascent + descent + ps.lineSpacing + (l.lastOfParagraph ? ps.paragraphSpacing : 0);
  }
  context.restore();
}

// MARK: - Markdown to attributed string (MarkDownAttributedPrinter + DrawableScene.toAttributedString)

const floatRe = /^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$/;
const isBlank = (s: string): boolean => s.trim().length === 0;

/** Decoded image per base64 payload; filled by ElementImageProvider, read at draw time. */
export type ImageLookup = ImageProvider & { decoded?: Map<string, { image: unknown }> };

export function toAttributedStr(
  tokens: MarkdownToken[],
  originalFont: FontSpec,
  paragraphStyle: ParagraphStyle,
  foregroundColor: Color,
  shift: Point,
  imageProvider: ImageLookup,
): AttributedString {
  const result = new AttributedString();

  let currentColor = foregroundColor;
  let font = originalFont;

  let prevMultiCode = false;
  let lastLiteral = "";
  let lastToken: MarkdownToken["type"] = "eof";
  const codeBackground = parseMarkdownColor("grey-200");
  const attr = (text: string, f: FontSpec, ps: ParagraphStyle, color: Color | null, extra: Partial<TextRun> = {}): void =>
    result.append(text, f, ps, { color, ...extra });

  for (const t of tokens) {
    let literal = t.literal;
    if (prevMultiCode) {
      prevMultiCode = false;
      if (literal.startsWith("\n")) {
        literal = literal.slice(1);
      }
      attr("\n", font, paragraphStyle, colorBlack, { background: codeBackground });
    }
    if (t.type !== "eof") {
      lastToken = t.type;
    }
    switch (t.type) {
      case "text":
        attr(literal, font, paragraphStyle, currentColor);
        break;
      case "bold":
        attr(literal, { size: font.size, bold: true, italic: false }, paragraphStyle, currentColor);
        break;
      case "bullet": {
        const ps = { ...paragraphStyle, headIndent: 5 * [...literal].length };
        if (ps.headIndent > shift.x) {
          shift.x = ps.headIndent;
        }
        attr("•", font, ps, currentColor);
        break;
      }
      case "image": {
        const resolved = imageProvider.resolveImagePath(t.literal);
        if (resolved !== null) {
          const decoded = imageProvider.decoded?.get(resolved.image);
          if (decoded !== undefined) {
            const r = resolved.rect;
            // We need to add at least one space to be able to see image.
            attr("￼", font, paragraphStyle, null, {
              attachment: { image: decoded.image, width: r.width, height: r.height, offsetY: fontCapHeight(font) / 2 - r.height / 2 },
            });
            attr(" ", font, paragraphStyle, colorBlack);
          }
        }
        break;
      }
      case "italic":
        attr(literal, { ...font, italic: true }, paragraphStyle, currentColor);
        break;
      case "underline":
        attr(literal, font, paragraphStyle, currentColor, { underline: true });
        break;
      case "scratch":
        attr(literal, font, paragraphStyle, currentColor, { strikethrough: true });
        break;
      case "title": {
        const [title, titleSize] = calcTitleFontSize(literal, font.size);
        attr(title, systemFont(titleSize), { ...paragraphStyle, paragraphSpacing: 5 }, currentColor);
        break;
      }
      case "color": {
        const splitPos = literal.indexOf("|");
        if (splitPos >= 0) {
          attr(literal.slice(splitPos + 1), font, paragraphStyle, parseMarkdownColor(literal.slice(0, splitPos)));
        } else if (isBlank(literal)) {
          currentColor = foregroundColor;
        } else {
          currentColor = parseMarkdownColor(literal);
        }
        break;
      }
      case "font": {
        const splitPos = literal.indexOf("|");
        if (splitPos >= 0) {
          const fs = literal.slice(0, splitPos);
          if (floatRe.test(fs)) {
            attr(literal.slice(splitPos + 1), systemFont(Number(fs)), paragraphStyle, currentColor);
          }
        } else if (isBlank(literal) || !floatRe.test(literal)) {
          font = originalFont;
        } else {
          font = systemFont(Number(literal));
        }
        break;
      }
      case "code":
        if (literal.includes("\n")) {
          if (!literal.startsWith("\n") && !lastLiteral.endsWith("\n")) {
            attr("\n", font, paragraphStyle, colorBlack);
          }
          if (!literal.endsWith("\n")) {
            prevMultiCode = true;
          }
        }
        attr(literal, font, paragraphStyle, colorBlack, { background: codeBackground });
        break;
      default:
        break;
    }
    lastLiteral = literal;
  }

  if (lastToken === "image") {
    attr("\n", font, paragraphStyle, colorBlack);
  }
  return result;
}

/** DrawableScene.toAttributedString */
export function toAttributedString(
  tokens: MarkdownToken[],
  font: FontSpec,
  color: Color,
  shift: Point,
  imageProvider: ImageLookup,
  layout: TextPosition[],
  lineSpacing = 0,
): AttributedString {
  const textStyle = defaultParagraphStyle();
  textStyle.lineSpacing = lineSpacing;
  for (const pos of layout) {
    switch (pos) {
      case "Center":
        textStyle.alignment = "center";
        break;
      case "Left":
        textStyle.alignment = "left";
        break;
      case "Right":
        textStyle.alignment = "right";
        break;
    }
  }
  return toAttributedStr(tokens, font, textStyle, color, shift, imageProvider);
}

export const getTokens = (code: string): MarkdownToken[] => MarkdownLexer.getTokens(code);

// MARK: - Body text

const spaceCount = (value: string): number => {
  let count = 0;
  for (const c of value) {
    if (c !== " ") {
      break;
    }
    count++;
  }
  return count;
};

const isBlankLine = (s: string): boolean => s.trim().length === 0;

export function prepareBodyText(textValue: string): string {
  const content = textValue.replaceAll("\\n", "\n").replaceAll("\t", "    ");

  const lines = content.split("\n");
  let minCount = Infinity;

  // Remove first empty line
  if (lines.length > 0 && isBlankLine(lines[0]!)) {
    lines.shift();
  }

  // remove last empty line.
  if (lines.length > 0 && isBlankLine(lines[lines.length - 1]!)) {
    lines.pop();
  }

  for (const l of lines) {
    if (!isBlankLine(l)) {
      minCount = Math.min(minCount, spaceCount(l));
    }
  }

  return lines.map((body) => (isBlankLine(body) ? body + " " : [...body].slice(minCount).join(""))).join("\n");
}

// MARK: - TextBox

export class TextBox {
  frame: Rect;
  attrStr: AttributedString;
  /** Markdown source of the text as HTML (scene.ts sets it). */
  html: () => string = () => "";

  constructor(text: AttributedString, bounds: Rect) {
    this.frame = bounds;
    this.attrStr = text;
  }

  setFrame(bounds: Rect): void {
    this.frame = bounds;
  }

  isVisible(): boolean {
    return true;
  }

  isSelectable(): boolean {
    return true;
  }

  drawBox(_context: Canvas2D, _at: Point): void {}

  traverse(op: (itm: Drawable) => boolean): void {
    op(this);
  }

  draw(context: Canvas2D, point: Point): void {
    const atr: Rect = { x: point.x + this.frame.x, y: point.y + this.frame.y, width: this.frame.width, height: this.frame.height };
    drawAttributedString(context, this.attrStr, atr);
  }

  layout(_parentBounds: Rect, _dirty: Rect): void {}

  getSelectorBounds(): Rect {
    return this.frame;
  }

  getBounds(): Rect {
    return this.frame;
  }
}
