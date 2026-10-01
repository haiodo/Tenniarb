// Canvas2D -> SVG string, no DOM. Output is in device space: the CTM is applied to path points
// when each segment is added, as Canvas does.

export interface TextMetricsLike {
  width: number;
  [k: string]: unknown;
}

export type TextMeasurer = (font: string, text: string) => TextMetricsLike;

export interface ImageLike {
  src: string;
  naturalWidth?: number;
  naturalHeight?: number;
  width?: number;
  height?: number;
}

export interface SvgContextOptions {
  width: number;
  height: number;
  measureText?: TextMeasurer;
  // Prefix for ids (filters, clips, images); set it when several SVGs share one HTML page.
  idPrefix?: string;
}

type Cmd = [string, ...number[]];
type Bounds = [number, number, number, number];
type Matrix = [number, number, number, number, number, number];

interface State {
  ctm: Matrix;
  fillStyle: string;
  strokeStyle: string;
  lineWidth: number;
  lineCap: string;
  lineJoin: string;
  miterLimit: number;
  dash: number[];
  dashOffset: number;
  globalAlpha: number;
  shadowColor: string;
  shadowBlur: number;
  shadowOffsetX: number;
  shadowOffsetY: number;
  font: string;
  textAlign: string;
  textBaseline: string;
  clip: string | null;
}

const BASELINE: Record<string, string> = {
  top: 'text-before-edge',
  hanging: 'hanging',
  middle: 'central',
  ideographic: 'ideographic',
  bottom: 'text-after-edge',
};
const ANCHOR: Record<string, string> = { start: 'start', left: 'start', center: 'middle', end: 'end', right: 'end' };
const FONT_UNITS: Record<string, number> = { px: 1, pt: 4 / 3, pc: 16, em: 16, rem: 16, in: 96, cm: 96 / 2.54, mm: 96 / 25.4 };

const fmt = (n: number): string => {
  const s = (Math.round(n * 1000) / 1000).toString();
  return s === '-0' ? '0' : s;
};

const esc = (s: string): string => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

const mul = (m: Matrix, n: Matrix): Matrix => [
  m[0] * n[0] + m[2] * n[1],
  m[1] * n[0] + m[3] * n[1],
  m[0] * n[2] + m[2] * n[3],
  m[1] * n[2] + m[3] * n[3],
  m[0] * n[4] + m[2] * n[5] + m[4],
  m[1] * n[4] + m[3] * n[5] + m[5],
];

const invert = (m: Matrix): Matrix | null => {
  const det = m[0] * m[3] - m[1] * m[2];
  if (!det || !Number.isFinite(det)) return null;
  return [
    m[3] / det,
    -m[1] / det,
    -m[2] / det,
    m[0] / det,
    (m[2] * m[5] - m[3] * m[4]) / det,
    (m[1] * m[4] - m[0] * m[5]) / det,
  ];
};

const apply = (m: Matrix, x: number, y: number): [number, number] => [m[0] * x + m[2] * y + m[4], m[1] * x + m[3] * y + m[5]];

const matrixAttr = (m: Matrix): string => `matrix(${m.map(fmt).join(' ')})`;

const pathData = (cmds: Cmd[], m?: Matrix): string => {
  let d = '';
  for (const c of cmds) {
    d += c[0];
    for (let i = 1; i < c.length; i += 2) {
      let x = c[i] as number;
      let y = c[i + 1] as number;
      if (m) [x, y] = apply(m, x, y);
      d += (i > 1 ? ' ' : '') + fmt(x) + ' ' + fmt(y);
    }
  }
  return d;
};

interface Rgba {
  rgb: string;
  a: number;
}

// Hex and rgb()/rgba() are split into color + opacity (older rasterizers drop rgba() in fill);
// anything else (names, hsl, ...) is passed through untouched.
function parseColor(c: string): Rgba {
  const s = c.trim().toLowerCase();
  let m = /^#([0-9a-f]{3,8})$/.exec(s);
  if (m) {
    let h = m[1] as string;
    if (h.length === 3 || h.length === 4) h = [...h].map((ch) => ch + ch).join('');
    if (h.length === 6) return { rgb: '#' + h, a: 1 };
    if (h.length === 8) return { rgb: '#' + h.slice(0, 6), a: parseInt(h.slice(6), 16) / 255 };
  }
  m = /^rgba?\(\s*([\d.]+%?)[\s,]+([\d.]+%?)[\s,]+([\d.]+%?)(?:[\s,/]+([\d.]+%?))?\s*\)$/.exec(s);
  if (m) {
    const ch = (v: string) => (v.endsWith('%') ? Math.round((parseFloat(v) * 255) / 100) : Math.round(parseFloat(v)));
    const a = m[4] === undefined ? 1 : m[4].endsWith('%') ? parseFloat(m[4]) / 100 : parseFloat(m[4]);
    return { rgb: `rgb(${ch(m[1] as string)},${ch(m[2] as string)},${ch(m[3] as string)})`, a: Math.min(1, Math.max(0, a)) };
  }
  if (s === 'transparent') return { rgb: '#000000', a: 0 };
  return { rgb: c.trim(), a: 1 };
}

interface FontInfo {
  style?: string;
  weight?: string;
  size: number;
  family: string;
}

function parseFont(font: string): FontInfo {
  const m = /^(.*?)(?:^|\s)(-?[\d.]+)(px|pt|pc|em|rem|in|cm|mm)(?:\s*\/\s*\S+)?\s+(.+)$/.exec(font.trim());
  const info: FontInfo = { size: 10, family: 'sans-serif' };
  if (!m) return info;
  info.size = parseFloat(m[2] as string) * (FONT_UNITS[m[3] as string] as number);
  info.family = (m[4] as string).trim();
  for (const t of (m[1] as string).split(/\s+/)) {
    if (t === 'italic' || t === 'oblique') info.style = t;
    else if (t === 'bold' || t === 'bolder' || t === 'lighter' || /^\d{3,4}$/.test(t)) info.weight = t;
  }
  return info;
}

const isSimilarity = (m: Matrix): boolean => {
  const eps = 1e-9 * (Math.abs(m[0]) + Math.abs(m[1]) + 1);
  return (Math.abs(m[0] - m[3]) < eps && Math.abs(m[1] + m[2]) < eps) || (Math.abs(m[0] + m[3]) < eps && Math.abs(m[1] - m[2]) < eps);
};

export class SvgContext {
  fillStyle: string = '#000000';
  strokeStyle: string = '#000000';
  lineWidth = 1;
  lineCap = 'butt';
  lineJoin = 'miter';
  miterLimit = 10;
  lineDashOffset = 0;
  globalAlpha = 1;
  shadowColor = 'rgba(0, 0, 0, 0)';
  shadowBlur = 0;
  shadowOffsetX = 0;
  shadowOffsetY = 0;
  font = '10px sans-serif';
  textAlign = 'start';
  textBaseline = 'alphabetic';

  readonly width: number;
  readonly height: number;

  #ctm: Matrix = [1, 0, 0, 1, 0, 0];
  #dash: number[] = [];
  #clip: string | null = null;
  #stack: State[] = [];
  #path: Cmd[] = [];
  // Pen and subpath start in device space; user-space pen is recovered through the current CTM.
  #cur: [number, number] | null = null;
  #start: [number, number] = [0, 0];
  #body: string[] = [];
  #openClip: string | null = null;
  #defs = new Map<string, string>();
  #ids = new Map<string, string>();
  #counter = 0;
  #measurer: TextMeasurer | undefined;
  #prefix: string;

  constructor(opts: SvgContextOptions) {
    this.width = opts.width;
    this.height = opts.height;
    this.#measurer = opts.measureText;
    this.#prefix = opts.idPrefix ?? '';
  }

  // ---- state ----

  save(): void {
    this.#stack.push({
      ctm: [...this.#ctm],
      fillStyle: this.fillStyle,
      strokeStyle: this.strokeStyle,
      lineWidth: this.lineWidth,
      lineCap: this.lineCap,
      lineJoin: this.lineJoin,
      miterLimit: this.miterLimit,
      dash: this.#dash,
      dashOffset: this.lineDashOffset,
      globalAlpha: this.globalAlpha,
      shadowColor: this.shadowColor,
      shadowBlur: this.shadowBlur,
      shadowOffsetX: this.shadowOffsetX,
      shadowOffsetY: this.shadowOffsetY,
      font: this.font,
      textAlign: this.textAlign,
      textBaseline: this.textBaseline,
      clip: this.#clip,
    });
  }

  // The current path is deliberately not part of the saved state.
  restore(): void {
    const s = this.#stack.pop();
    if (!s) return;
    this.#ctm = s.ctm;
    this.fillStyle = s.fillStyle;
    this.strokeStyle = s.strokeStyle;
    this.lineWidth = s.lineWidth;
    this.lineCap = s.lineCap;
    this.lineJoin = s.lineJoin;
    this.miterLimit = s.miterLimit;
    this.#dash = s.dash;
    this.lineDashOffset = s.dashOffset;
    this.globalAlpha = s.globalAlpha;
    this.shadowColor = s.shadowColor;
    this.shadowBlur = s.shadowBlur;
    this.shadowOffsetX = s.shadowOffsetX;
    this.shadowOffsetY = s.shadowOffsetY;
    this.font = s.font;
    this.textAlign = s.textAlign;
    this.textBaseline = s.textBaseline;
    this.#clip = s.clip;
  }

  setLineDash(segments: number[]): void {
    const a = Array.from(segments);
    if (a.some((v) => !Number.isFinite(v) || v < 0)) return;
    this.#dash = a.length % 2 ? a.concat(a) : a;
  }

  getLineDash(): number[] {
    return [...this.#dash];
  }

  // ---- transforms ----

  getTransform(): { a: number; b: number; c: number; d: number; e: number; f: number } {
    const [a, b, c, d, e, f] = this.#ctm;
    return { a, b, c, d, e, f };
  }

  translate(x: number, y: number): void {
    this.#ctm = mul(this.#ctm, [1, 0, 0, 1, x, y]);
  }

  scale(x: number, y: number): void {
    this.#ctm = mul(this.#ctm, [x, 0, 0, y, 0, 0]);
  }

  rotate(angle: number): void {
    const c = Math.cos(angle);
    const s = Math.sin(angle);
    this.#ctm = mul(this.#ctm, [c, s, -s, c, 0, 0]);
  }

  transform(a: number, b: number, c: number, d: number, e: number, f: number): void {
    if (![a, b, c, d, e, f].every(Number.isFinite)) return;
    this.#ctm = mul(this.#ctm, [a, b, c, d, e, f]);
  }

  setTransform(a: number | { a: number; b: number; c: number; d: number; e: number; f: number }, b = 0, c = 0, d = 0, e = 0, f = 0): void {
    const m: Matrix = typeof a === 'object' ? [a.a, a.b, a.c, a.d, a.e, a.f] : [a, b, c, d, e, f];
    if (!m.every(Number.isFinite)) return;
    this.#ctm = m;
  }

  resetTransform(): void {
    this.#ctm = [1, 0, 0, 1, 0, 0];
  }

  // ---- path ----

  beginPath(): void {
    this.#path = [];
    this.#cur = null;
  }

  closePath(): void {
    if (!this.#cur) return;
    this.#path.push(['Z']);
    this.#cur = this.#start;
  }

  moveTo(x: number, y: number): void {
    if (!Number.isFinite(x) || !Number.isFinite(y)) return;
    const p = apply(this.#ctm, x, y);
    this.#path.push(['M', p[0], p[1]]);
    this.#cur = this.#start = p;
  }

  lineTo(x: number, y: number): void {
    if (!Number.isFinite(x) || !Number.isFinite(y)) return;
    if (!this.#cur) return this.moveTo(x, y);
    const p = apply(this.#ctm, x, y);
    this.#path.push(['L', p[0], p[1]]);
    this.#cur = p;
  }

  quadraticCurveTo(cx: number, cy: number, x: number, y: number): void {
    if (![cx, cy, x, y].every(Number.isFinite)) return;
    if (!this.#cur) this.moveTo(cx, cy);
    const c = apply(this.#ctm, cx, cy);
    const p = apply(this.#ctm, x, y);
    this.#path.push(['Q', c[0], c[1], p[0], p[1]]);
    this.#cur = p;
  }

  bezierCurveTo(c1x: number, c1y: number, c2x: number, c2y: number, x: number, y: number): void {
    if (![c1x, c1y, c2x, c2y, x, y].every(Number.isFinite)) return;
    if (!this.#cur) this.moveTo(c1x, c1y);
    const a = apply(this.#ctm, c1x, c1y);
    const b = apply(this.#ctm, c2x, c2y);
    const p = apply(this.#ctm, x, y);
    this.#path.push(['C', a[0], a[1], b[0], b[1], p[0], p[1]]);
    this.#cur = p;
  }

  rect(x: number, y: number, w: number, h: number): void {
    if (![x, y, w, h].every(Number.isFinite)) return;
    this.moveTo(x, y);
    this.lineTo(x + w, y);
    this.lineTo(x + w, y + h);
    this.lineTo(x, y + h);
    this.closePath();
    this.moveTo(x, y);
  }

  roundRect(x: number, y: number, w: number, h: number, radii: number | number[] = 0): void {
    if (![x, y, w, h].every(Number.isFinite)) return;
    const r = Array.isArray(radii) ? radii : [radii];
    if (r.length < 1 || r.length > 4 || r.some((v) => !Number.isFinite(v) || v < 0)) throw new RangeError('roundRect: bad radii');
    let [tl, tr, br, bl] = (r.length === 1 ? [r[0], r[0], r[0], r[0]] : r.length === 2 ? [r[0], r[1], r[0], r[1]] : r.length === 3 ? [r[0], r[1], r[2], r[1]] : r) as number[] as [number, number, number, number];
    if (w < 0) {
      x += w;
      w = -w;
      [tl, tr, br, bl] = [tr, tl, bl, br];
    }
    if (h < 0) {
      y += h;
      h = -h;
      [tl, tr, br, bl] = [bl, br, tr, tl];
    }
    const k = Math.min(1, w / (tl + tr || 1), w / (bl + br || 1), h / (tl + bl || 1), h / (tr + br || 1));
    tl *= k; tr *= k; br *= k; bl *= k;
    this.moveTo(x + tl, y);
    this.lineTo(x + w - tr, y);
    this.#arcSegments(x + w - tr, y + tr, tr, tr, 0, -Math.PI / 2, 0, false);
    this.lineTo(x + w, y + h - br);
    this.#arcSegments(x + w - br, y + h - br, br, br, 0, 0, Math.PI / 2, false);
    this.lineTo(x + bl, y + h);
    this.#arcSegments(x + bl, y + h - bl, bl, bl, 0, Math.PI / 2, Math.PI, false);
    this.lineTo(x, y + tl);
    this.#arcSegments(x + tl, y + tl, tl, tl, 0, Math.PI, Math.PI * 1.5, false);
    this.closePath();
    this.moveTo(x, y);
  }

  arc(x: number, y: number, r: number, start: number, end: number, ccw = false): void {
    this.ellipse(x, y, r, r, 0, start, end, ccw);
  }

  ellipse(x: number, y: number, rx: number, ry: number, rot: number, start: number, end: number, ccw = false): void {
    if (![x, y, rx, ry, rot, start, end].every(Number.isFinite)) return;
    if (rx < 0 || ry < 0) throw new RangeError('ellipse: negative radius');
    const full = Math.PI * 2;
    let delta = end - start;
    if (!ccw && delta >= full) delta = full;
    else if (ccw && -delta >= full) delta = -full;
    else {
      delta = ((delta % full) + full) % full;
      if (ccw && delta > 0) delta -= full;
    }
    this.#arcSegments(x, y, rx, ry, rot, start, start + delta, true);
  }

  arcTo(x1: number, y1: number, x2: number, y2: number, r: number): void {
    if (![x1, y1, x2, y2, r].every(Number.isFinite)) return;
    if (r < 0) throw new RangeError('arcTo: negative radius');
    if (!this.#cur) return this.moveTo(x1, y1);
    const inv = invert(this.#ctm);
    if (!inv) return this.lineTo(x1, y1);
    const [x0, y0] = apply(inv, this.#cur[0], this.#cur[1]);
    const ax = x0 - x1, ay = y0 - y1, bx = x2 - x1, by = y2 - y1;
    const la = Math.hypot(ax, ay), lb = Math.hypot(bx, by);
    const cross = ax * by - ay * bx;
    if (r === 0 || !la || !lb || Math.abs(cross) < 1e-12 * la * lb) return this.lineTo(x1, y1);
    const ux = ax / la, uy = ay / la, vx = bx / lb, vy = by / lb;
    const half = Math.acos(Math.max(-1, Math.min(1, ux * vx + uy * vy))) / 2;
    const tl = r / Math.tan(half);
    const tx = x1 + ux * tl, ty = y1 + uy * tl;
    const ex = x1 + vx * tl, ey = y1 + vy * tl;
    let mx = ux + vx, my = uy + vy;
    const ml = Math.hypot(mx, my);
    mx /= ml; my /= ml;
    const cd = r / Math.sin(half);
    const cx = x1 + mx * cd, cy = y1 + my * cd;
    const a0 = Math.atan2(ty - cy, tx - cx);
    const a1 = Math.atan2(ey - cy, ex - cx);
    let d = a1 - a0;
    while (d > Math.PI) d -= Math.PI * 2;
    while (d < -Math.PI) d += Math.PI * 2;
    this.lineTo(tx, ty);
    this.#arcSegments(cx, cy, r, r, 0, a0, a0 + d, false);
  }

  // Cubic approximation, at most 90 degrees per piece; points go through the CTM like any other segment.
  #arcSegments(cx: number, cy: number, rx: number, ry: number, rot: number, a0: number, a1: number, connect: boolean): void {
    const cr = Math.cos(rot), sr = Math.sin(rot);
    const pt = (t: number): [number, number] => {
      const px = rx * Math.cos(t), py = ry * Math.sin(t);
      return [cx + px * cr - py * sr, cy + px * sr + py * cr];
    };
    const dt = (t: number): [number, number] => {
      const px = -rx * Math.sin(t), py = ry * Math.cos(t);
      return [px * cr - py * sr, px * sr + py * cr];
    };
    const [sx, sy] = pt(a0);
    if (connect) this.lineTo(sx, sy);
    const n = Math.max(1, Math.ceil(Math.abs(a1 - a0) / (Math.PI / 2) - 1e-9));
    const step = (a1 - a0) / n;
    const k = (4 / 3) * Math.tan(step / 4);
    for (let i = 0; i < n; i++) {
      const t0 = a0 + i * step, t1 = t0 + step;
      const p0 = pt(t0), p1 = pt(t1), d0 = dt(t0), d1 = dt(t1);
      this.bezierCurveTo(p0[0] + k * d0[0], p0[1] + k * d0[1], p1[0] - k * d1[0], p1[1] - k * d1[1], p1[0], p1[1]);
    }
  }

  #bounds(): Bounds {
    let x0 = Infinity, y0 = Infinity, x1 = -Infinity, y1 = -Infinity;
    for (const c of this.#path) {
      for (let i = 1; i < c.length; i += 2) {
        const x = c[i] as number, y = c[i + 1] as number;
        if (x < x0) x0 = x;
        if (x > x1) x1 = x;
        if (y < y0) y0 = y;
        if (y > y1) y1 = y;
      }
    }
    return [x0, y0, x1, y1];
  }

  // ---- paint ----

  fill(rule: string = 'nonzero'): void {
    if (!this.#path.length) return;
    const c = parseColor(this.fillStyle);
    const a = c.a * this.globalAlpha;
    const attrs = `d="${pathData(this.#path)}" fill="${esc(c.rgb)}"` + (a < 1 ? ` fill-opacity="${fmt(a)}"` : '') + (rule === 'evenodd' ? ' fill-rule="evenodd"' : '');
    this.#emit('path', attrs, undefined, undefined, this.#bounds());
  }

  stroke(): void {
    if (!this.#path.length || !(this.lineWidth > 0)) return;
    const c = parseColor(this.strokeStyle);
    const a = c.a * this.globalAlpha;
    let s = 1;
    let d: string;
    let tf: Matrix | undefined;
    if (isSimilarity(this.#ctm)) {
      s = Math.sqrt(Math.abs(this.#ctm[0] * this.#ctm[3] - this.#ctm[1] * this.#ctm[2]));
      d = pathData(this.#path);
    } else {
      // Pen shape follows the CTM at stroke time: draw in user space under that transform.
      const inv = invert(this.#ctm);
      if (!inv) return;
      d = pathData(this.#path, inv);
      tf = this.#ctm;
    }
    let attrs = `d="${d}" fill="none" stroke="${esc(c.rgb)}" stroke-width="${fmt(this.lineWidth * s)}"`;
    if (a < 1) attrs += ` stroke-opacity="${fmt(a)}"`;
    if (this.lineCap !== 'butt') attrs += ` stroke-linecap="${this.lineCap}"`;
    if (this.lineJoin !== 'miter') attrs += ` stroke-linejoin="${this.lineJoin}"`;
    if (this.lineJoin === 'miter' && this.miterLimit !== 4) attrs += ` stroke-miterlimit="${fmt(this.miterLimit)}"`;
    if (this.#dash.length && this.#dash.some((v) => v > 0)) {
      attrs += ` stroke-dasharray="${this.#dash.map((v) => fmt(v * s)).join(' ')}"`;
      if (this.lineDashOffset) attrs += ` stroke-dashoffset="${fmt(this.lineDashOffset * s)}"`;
    }
    const wide = this.lineWidth * s * (this.lineJoin === 'miter' ? Math.max(1, this.miterLimit) : 1);
    this.#emit('path', attrs, tf, undefined, this.#bounds(), wide / 2);
  }

  // Draws without touching the current path, like Canvas.
  fillRect(x: number, y: number, w: number, h: number): void {
    if (![x, y, w, h].every(Number.isFinite) || !w || !h) return;
    const c = parseColor(this.fillStyle);
    const a = c.a * this.globalAlpha;
    const m = this.#ctm;
    const axis = m[1] === 0 && m[2] === 0;
    const [rx, ry] = axis ? apply(m, x, y) : [x, y];
    const [rw, rh] = axis ? [w * m[0], h * m[3]] : [w, h];
    const bounds: Bounds = axis ? [Math.min(rx, rx + rw), Math.min(ry, ry + rh), Math.max(rx, rx + rw), Math.max(ry, ry + rh)] : [0, 0, 0, 0];
    const attrs = `x="${fmt(Math.min(rx, rx + rw))}" y="${fmt(Math.min(ry, ry + rh))}" width="${fmt(Math.abs(rw))}" height="${fmt(Math.abs(rh))}" fill="${esc(c.rgb)}"` + (a < 1 ? ` fill-opacity="${fmt(a)}"` : '');
    this.#emit('rect', axis ? attrs : attrs.replace(/x="[^"]*" y="[^"]*" width="[^"]*" height="[^"]*"/, `x="${fmt(x)}" y="${fmt(y)}" width="${fmt(w)}" height="${fmt(h)}"`), axis ? undefined : m, undefined, bounds);
  }

  clip(rule: string = 'nonzero'): void {
    const parent = this.#clip;
    const d = pathData(this.#path);
    const key = `clip|${parent}|${rule}|${d}`;
    let id = this.#ids.get(key);
    if (!id) {
      id = `${this.#prefix}c${++this.#counter}`;
      this.#ids.set(key, id);
      this.#defs.set(id, `<clipPath id="${id}"${parent ? ` clip-path="url(#${parent})"` : ''}><path d="${d}"${rule === 'evenodd' ? ' clip-rule="evenodd"' : ''}/></clipPath>`);
    }
    this.#clip = id;
  }

  // ---- text ----

  measureText(text: string): TextMetricsLike {
    if (!this.#measurer) throw new Error('SvgContext: measureText needs a measurer in the constructor options');
    return this.#measurer(this.font, text);
  }

  fillText(text: string, x: number, y: number, maxWidth?: number): void {
    const c = parseColor(this.fillStyle);
    const a = c.a * this.globalAlpha;
    this.#text(text, x, y, maxWidth, `fill="${esc(c.rgb)}"` + (a < 1 ? ` fill-opacity="${fmt(a)}"` : ''));
  }

  strokeText(text: string, x: number, y: number, maxWidth?: number): void {
    if (!(this.lineWidth > 0)) return;
    const c = parseColor(this.strokeStyle);
    const a = c.a * this.globalAlpha;
    let attrs = `fill="none" stroke="${esc(c.rgb)}" stroke-width="${fmt(this.lineWidth)}"`;
    if (a < 1) attrs += ` stroke-opacity="${fmt(a)}"`;
    if (this.lineJoin !== 'miter') attrs += ` stroke-linejoin="${this.lineJoin}"`;
    this.#text(text, x, y, maxWidth, attrs);
  }

  #text(text: string, x: number, y: number, maxWidth: number | undefined, paint: string): void {
    if (!Number.isFinite(x) || !Number.isFinite(y) || !text) return;
    const t = text.replace(/[\t\n\r\f\v]/g, ' ');
    const f = parseFont(this.font);
    let attrs = '';
    const m = this.#ctm;
    const translateOnly = m[0] === 1 && m[1] === 0 && m[2] === 0 && m[3] === 1;
    const identity = translateOnly && m[4] === 0 && m[5] === 0;
    attrs += translateOnly ? `x="${fmt(x + m[4])}" y="${fmt(y + m[5])}"` : `x="${fmt(x)}" y="${fmt(y)}"`;
    attrs += ` font-family="${esc(f.family)}" font-size="${fmt(f.size)}"`;
    if (f.style) attrs += ` font-style="${f.style}"`;
    if (f.weight) attrs += ` font-weight="${f.weight}"`;
    const anchor = ANCHOR[this.textAlign] ?? 'start';
    if (anchor !== 'start') attrs += ` text-anchor="${anchor}"`;
    const base = BASELINE[this.textBaseline];
    if (base) attrs += ` dominant-baseline="${base}"`;
    if (maxWidth !== undefined && maxWidth > 0 && this.#measurer && this.#measurer(this.font, t).width > maxWidth) {
      attrs += ` textLength="${fmt(maxWidth)}" lengthAdjust="spacingAndGlyphs"`;
    }
    if (/^\s|\s$|\s\s/.test(t)) attrs += ' xml:space="preserve"';
    this.#emit('text', `${attrs} ${paint}`, identity || translateOnly ? undefined : m, esc(t));
  }

  // ---- images ----

  drawImage(img: ImageLike, ...a: number[]): void {
    const src = typeof img === 'string' ? (img as string) : img.src;
    if (typeof src !== 'string' || !src) throw new Error('SvgContext.drawImage: image needs a string src (data URL)');
    const nw = img.naturalWidth ?? img.width;
    const nh = img.naturalHeight ?? img.height;
    const href = ` xlink:href="${esc(src)}"`;
    if (a.length === 4 || a.length === 2) {
      const [dx, dy] = a as [number, number];
      const dw = a.length === 4 ? (a[2] as number) : nw;
      const dh = a.length === 4 ? (a[3] as number) : nh;
      if (dw === undefined || dh === undefined) throw new Error('SvgContext.drawImage: unknown image size');
      if (nw && nh) {
        const id = this.#image(src, nw, nh);
        const tf = mul(this.#ctm, [dw / nw, 0, 0, dh / nh, dx, dy]);
        this.#emit('use', `xlink:href="#${id}"`, tf, undefined);
      } else {
        this.#emit('image', `x="${fmt(dx)}" y="${fmt(dy)}" width="${fmt(dw)}" height="${fmt(dh)}" preserveAspectRatio="none"${href}`, this.#ctm);
      }
      return;
    }
    if (a.length !== 8) throw new Error('SvgContext.drawImage: bad arguments');
    const [sx, sy, sw, sh, dx, dy, dw, dh] = a as [number, number, number, number, number, number, number, number];
    const inner = `<image width="${fmt(nw ?? sx + sw)}" height="${fmt(nh ?? sy + sh)}" preserveAspectRatio="none"${href}/>`;
    this.#emit('svg', `x="${fmt(dx)}" y="${fmt(dy)}" width="${fmt(dw)}" height="${fmt(dh)}" viewBox="${fmt(sx)} ${fmt(sy)} ${fmt(sw)} ${fmt(sh)}" preserveAspectRatio="none" overflow="hidden"`, this.#ctm, inner);
  }

  #image(src: string, w: number, h: number): string {
    const key = `img|${src}`;
    let id = this.#ids.get(key);
    if (!id) {
      id = `${this.#prefix}i${++this.#counter}`;
      this.#ids.set(key, id);
      this.#defs.set(id, `<image id="${id}" width="${fmt(w)}" height="${fmt(h)}" preserveAspectRatio="none" xlink:href="${esc(src)}"/>`);
    }
    return id;
  }

  // ---- output ----

  // bounds (device space) lets the filter use a small bbox-relative region; without them, or for
  // zero-extent shapes (lines), the region is the whole canvas, which rasterizers handle slowly.
  #filter(bounds?: Bounds, extra = 0): string | null {
    const c = parseColor(this.shadowColor);
    if (c.a <= 0 || !(this.shadowBlur > 0 || this.shadowOffsetX || this.shadowOffsetY)) return null;
    if (!Number.isFinite(this.shadowBlur) || this.shadowBlur < 0) return null;
    const sd = this.shadowBlur / 2;
    const dx = this.shadowOffsetX, dy = this.shadowOffsetY;
    const reach = 3 * sd + Math.max(Math.abs(dx), Math.abs(dy)) + extra;
    let region: string;
    let fx = 0, fy = 0;
    const w = bounds ? bounds[2] - bounds[0] : 0;
    const h = bounds ? bounds[3] - bounds[1] : 0;
    if (w > 1e-3 && h > 1e-3 && reach / Math.min(w, h) <= 4) {
      fx = Math.ceil((reach / w) * 4) / 4 || 0.25;
      fy = Math.ceil((reach / h) * 4) / 4 || 0.25;
      region = `x="${-fx}" y="${-fy}" width="${1 + 2 * fx}" height="${1 + 2 * fy}"`;
    } else {
      region = `filterUnits="userSpaceOnUse" x="${fmt(-reach)}" y="${fmt(-reach)}" width="${fmt(this.width + 2 * reach)}" height="${fmt(this.height + 2 * reach)}"`;
    }
    const key = `f|${fmt(dx)}|${fmt(dy)}|${fmt(sd)}|${c.rgb}|${fmt(c.a)}|${region}`;
    let id = this.#ids.get(key);
    if (!id) {
      id = `${this.#prefix}f${++this.#counter}`;
      this.#ids.set(key, id);
      this.#defs.set(id, `<filter id="${id}" ${region} color-interpolation-filters="sRGB"><feDropShadow dx="${fmt(dx)}" dy="${fmt(dy)}" stdDeviation="${fmt(sd)}" flood-color="${esc(c.rgb)}" flood-opacity="${fmt(c.a)}"/></filter>`);
    }
    return id;
  }

  // Filter goes on the outer element so shadow offsets stay in device space under a transform.
  #emit(tag: string, attrs: string, transform?: Matrix, inner?: string, bounds?: Bounds, extra = 0): void {
    if (this.#clip !== this.#openClip) {
      if (this.#openClip) this.#body.push('</g>');
      if (this.#clip) this.#body.push(`<g clip-path="url(#${this.#clip})">`);
      this.#openClip = this.#clip;
    }
    const f = this.#filter(bounds, extra);
    const tf = transform ? ` transform="${matrixAttr(transform)}"` : '';
    const el = inner === undefined ? `<${tag} ${attrs}${tf}/>` : tag === 'text' ? `<text ${attrs}${tf}>${inner}</text>` : `<${tag} ${attrs}${tf}>${inner}</${tag}>`;
    if (!f) this.#body.push(el);
    else if (transform) this.#body.push(`<g filter="url(#${f})">${el}</g>`);
    else this.#body.push(inner === undefined ? el.replace(/\/>$/, ` filter="url(#${f})"/>`) : el.replace(/^<(\w+)/, `<$1 filter="url(#${f})"`));
  }

  toString(): string {
    const w = fmt(this.width), h = fmt(this.height);
    const defs = this.#defs.size ? `<defs>${[...this.#defs.values()].join('')}</defs>` : '';
    return `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="${w}" height="${h}" viewBox="0 0 ${w} ${h}">${defs}${this.#body.join('')}${this.#openClip ? '</g>' : ''}</svg>`;
  }
}
