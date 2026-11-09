// Parity of the Go scene renderer and the web one: for every fixture, the web scene is drawn into a
// Canvas2D that records the paint calls, and the result is compared node by node with the golden made
// by go/cmd/tennlayout (web/fixtures/scene/<name>.layout.json).
//
// The golden is what Go paints: go/scene builds gfx pictures, tennlayout decodes them. The web side is
// read the same way - from the draw calls, not from re-reading the properties - so a divergence is a
// picture that differs, not a second implementation of the same rules.
//
// The files a fixture mounts with element --from-file are read from the fixture folder, as the CLI does.
import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import { readTenn } from "@tenniarb/core";
import type { Element } from "@tenniarb/core";
import { buildTennScene } from "../src/scene-tenn.ts";
import type { TennScene } from "../src/scene-tenn.ts";
import type { Canvas2D } from "../src/canvas-types.ts";
import { findElement } from "../src/elements.ts";
import "./helpers.ts";

const fixtures = fileURLToPath(new URL("../../../fixtures/scene/", import.meta.url));

interface Gradient {
  __gradient: true;
  angle: number;
  stops: { offset: number; color: string }[];
}

interface PathState {
  points: number;
  closed: boolean;
  arcs: number[];
  /** The commands as tennlayout writes a link: ["m"|"l"|"q"|"c", x, y, ...] and ["z"]. */
  verbs: (string | number)[][];
}

/** A stroked open path with no arcs: a link, drawn in the coordinates of the item that owns it. */
interface DrawnLink {
  path: (string | number)[][];
  color: string;
  width: number;
  dash: number[] | null;
  phase: number;
  arrow: boolean;
}

interface Rec {
  x: number;
  y: number;
  /**
   * The alpha of the parent when the node began; `own` is the alpha inside it, so own/base is the
   * opacity the node adds.
   */
  base: number;
  own: number | null;
  kids: Rec[];
  /** A child was flushed: a later open stroke is a link, not the icon of the node itself. */
  kidsStarted: boolean;
  radius: [number, number, number, number];
  fill: string | null;
  gradient: Gradient | null;
  border: string | null;
  borderWidth: number | null;
  text: string[];
  textColor: string | null;
  clip: boolean;
  /** Degrees clockwise, as `rotate` counts them in the file. */
  rotate: number;
  shadows: { dx: number; dy: number; blur: number; color: string }[];
  links: DrawnLink[];
}

const hex2 = (n: number): string => Math.round(Math.max(0, Math.min(255, n))).toString(16).padStart(2, "0");

/** "@tenniarb/render" writes rgb()/rgba(); #rrggbb or #rrggbbaa as tennlayout. */
function cssHex(css: string): string {
  const m = /^rgba?\(([^)]+)\)$/.exec(css.trim());
  if (m === null) return css.trim();
  const p = m[1]!.split(",").map((s) => Number(s.trim()));
  const a = p.length > 3 ? (p[3] ?? 1) : 1;
  const rgb = `#${hex2(p[0] ?? 0)}${hex2(p[1] ?? 0)}${hex2(p[2] ?? 0)}`;
  return a >= 1 ? rgb : rgb + hex2(a * 255);
}

/** Alpha of #rrggbbaa as 0..1; #rrggbb is opaque. */
function alphaOf(c: string): number {
  const m = /^#([0-9a-f]{6})([0-9a-f]{2})?$/i.exec(c);
  return m !== null && m[2] !== undefined ? parseInt(m[2], 16) / 255 : 1;
}

/** Go's gradient.base: the first stop that shows, the first one when all are transparent. */
function gradientBase(g: Gradient): string {
  return (g.stops.find((s) => alphaOf(s.color) > 0) ?? g.stops[0]!).color;
}

/** A Canvas2D that rebuilds the node tree from translate() calls and records what each node paints. */
function recorder(): { ctx: Canvas2D; roots: Rec[]; done: () => Rec[] } {
  const roots: Rec[] = [];
  const saved: { current: Rec | null; state: State }[] = [];
  const state: State = {
    alpha: 1,
    fill: "#000000",
    stroke: "#000000",
    lineWidth: 1,
    shadowColor: "",
    shadowBlur: 0,
    shadowDX: 0,
    shadowDY: 0,
    dash: [],
    lineDashOffset: 0,
    rotation: 0,
  };
  let current: Rec | null = null;
  let pending: { x: number; y: number; base: number } | null = null;
  let pen: PathState = { points: 0, closed: false, arcs: [], verbs: [] };

  const flush = (): void => {
    if (pending === null) return;
    const rec: Rec = {
      x: pending.x,
      y: pending.y,
      base: pending.base,
      own: null,
      kids: [],
      kidsStarted: false,
      radius: [0, 0, 0, 0],
      fill: null,
      gradient: null,
      border: null,
      borderWidth: null,
      text: [],
      textColor: null,
      clip: false,
      // The scene is drawn inside scale(1, -1), so a canvas rotate(a) turns the picture by -a.
      rotate: (-state.rotation * 180) / Math.PI,
      shadows: [],
      links: [],
    };
    (current === null ? roots : current.kids).push(rec);
    if (current !== null) {
      current.own ??= state.alpha;
      current.kidsStarted = true;
    }
    current = rec;
    pending = null;
  };
  const touch = (): void => {
    flush();
    if (current !== null) current.own ??= state.alpha;
  };
  const active = (): Rec | null => {
    touch();
    return current;
  };
  const shadowOf = (): Rec["shadows"][number] | null =>
    state.shadowBlur > 0 || state.shadowDX !== 0 || state.shadowDY !== 0 || state.shadowColor !== ""
      ? { dx: state.shadowDX, dy: state.shadowDY, blur: state.shadowBlur, color: cssHex(state.shadowColor) }
      : null;
  /** A shadow paints each pass; two `shadow` lines in the file mean two passes with the same fill. */
  const addShadow = (rec: Rec): void => {
    const sh = shadowOf();
    if (sh === null) return;
    const last = rec.shadows[rec.shadows.length - 1];
    if (last === undefined || last.dx !== sh.dx || last.dy !== sh.dy || last.blur !== sh.blur || last.color !== sh.color) {
      rec.shadows.push(sh);
    }
  };
  const radii = (): [number, number, number, number] | null => {
    if (pen.arcs.length !== 4) return null;
    const [tr, br, bl, tl] = pen.arcs as [number, number, number, number];
    return [tl, tr, br, bl];
  };
  const mergeRadius = (rec: Rec): void => {
    const r = radii();
    if (r === null) return;
    for (let i = 0; i < 4; i += 1) rec.radius[i] = Math.max(rec.radius[i]!, r[i]!);
  };
  const isArrow = (): boolean => pen.points === 3 && pen.closed;

  const ctx: Canvas2D = {
    save: () => {
      touch();
      saved.push({ current, state: { ...state } });
    },
    restore: () => {
      flush();
      const s = saved.pop();
      if (s !== undefined) {
        Object.assign(state, s.state);
        current = s.current;
      }
    },
    translate: (x: number, y: number) => {
      if (pending !== null) {
        // A turn about the middle of a node is translate, rotate, translate back: one position.
        pending.x += x;
        pending.y += y;
        return;
      }
      flush();
      pending = { x, y, base: state.alpha };
      // Each node turns about its own box: a child's angle is not its parent's.
      state.rotation = 0;
    },
    rotate: (a: number) => {
      state.rotation += a;
    },
    scale: () => {
      // The flip of TennScene.draw wraps the whole picture; translate(0,0) before it is not a node.
      pending = null;
    },
    beginPath: () => {
      touch();
      pen = { points: 0, closed: false, arcs: [], verbs: [] };
    },
    moveTo: (x, y) => {
      touch();
      pen.points += 1;
      pen.verbs.push(["m", x, y]);
    },
    lineTo: (x, y) => {
      touch();
      pen.points += 1;
      pen.verbs.push(["l", x, y]);
    },
    arcTo: (_x1, _y1, _x2, _y2, r) => {
      touch();
      pen.points += 1;
      pen.arcs.push(r);
    },
    quadraticCurveTo: (cx, cy, x, y) => {
      touch();
      pen.points += 1;
      pen.verbs.push(["q", cx, cy, x, y]);
    },
    closePath: () => {
      touch();
      pen.closed = true;
      pen.verbs.push(["z"]);
    },
    rect: () => {
      touch();
      pen.points += 4;
      pen.closed = true;
    },
    ellipse: () => {
      touch();
      pen.points += 1;
      pen.closed = true;
    },
    clip: () => {
      const rec = active();
      if (rec === null) return;
      rec.clip = true;
      mergeRadius(rec);
    },
    fill: () => {
      const rec = active();
      if (rec === null) return;
      if (isArrow()) {
        // The arrow head of a link: the stroke of the line was recorded just before it.
        const link = rec.links[rec.links.length - 1];
        if (link !== undefined) link.arrow = true;
        return;
      }
      mergeRadius(rec);
      if (typeof state.fill === "object") {
        rec.gradient = state.fill as Gradient;
        rec.fill = gradientBase(rec.gradient);
      } else {
        rec.fill = cssHex(state.fill);
      }
      addShadow(rec);
    },
    stroke: () => {
      const rec = active();
      if (rec === null) return;
      if (!pen.closed && pen.arcs.length === 0 && rec.kidsStarted) {
        // Links are drawn after the children of their parent; an icon is stroked before them.
        rec.links.push({
          path: pen.verbs,
          color: typeof state.stroke === "object" ? "#00000000" : cssHex(state.stroke),
          width: state.lineWidth,
          dash: state.dash.length > 0 ? [...state.dash] : null,
          phase: state.lineDashOffset,
          arrow: false,
        });
        return;
      }
      mergeRadius(rec);
      rec.border = typeof state.stroke === "object" ? "#00000000" : cssHex(state.stroke);
      rec.borderWidth = state.lineWidth;
      addShadow(rec);
    },
    fillRect: () => {},
    fillText: (text) => {
      const rec = active();
      if (rec === null) return;
      rec.text.push(text);
      rec.textColor = typeof state.fill === "object" ? null : cssHex(state.fill);
      // A glyph shadow is a shadow of the node like any other (OpTextShadow in Go).
      addShadow(rec);
    },
    measureText: (text) => ({ width: [...text].length * 10 }),
    drawImage: () => {},
    setLineDash: (segments) => {
      state.dash = [...segments];
    },
    get fillStyle() {
      return state.fill;
    },
    set fillStyle(v: string | object) {
      state.fill = v;
    },
    get strokeStyle() {
      return state.stroke;
    },
    set strokeStyle(v: string | object) {
      state.stroke = v;
    },
    get globalAlpha() {
      return state.alpha;
    },
    set globalAlpha(v: number) {
      state.alpha = v;
    },
    get font() {
      return "";
    },
    set font(_v: string) {},
    get shadowColor() {
      return state.shadowColor;
    },
    set shadowColor(v: string) {
      state.shadowColor = v;
    },
    get shadowBlur() {
      return state.shadowBlur;
    },
    set shadowBlur(v: number) {
      state.shadowBlur = v;
    },
    get shadowOffsetX() {
      return state.shadowDX;
    },
    set shadowOffsetX(v: number) {
      state.shadowDX = v;
    },
    get shadowOffsetY() {
      return state.shadowDY;
    },
    set shadowOffsetY(v: number) {
      state.shadowDY = v;
    },
    createLinearGradient(x0, y0, x1, y1) {
      const g = {
        __gradient: true,
        // tennlayout reads the angle of the line as atan2(dx, -dy); the minus puts 0 up, as in .tenn.
        angle: (Math.atan2(x1 - x0, -(y1 - y0)) * 180) / Math.PI,
        stops: [] as { offset: number; color: string }[],
        addColorStop(offset: number, color: string) {
          g.stops.push({ offset, color: cssHex(color) });
        },
        toString: () => "gradient",
      };
      return g;
    },
    get lineWidth() {
      return state.lineWidth;
    },
    set lineWidth(v: number) {
      state.lineWidth = v;
    },
    get lineDashOffset() {
      return state.lineDashOffset;
    },
    set lineDashOffset(v: number) {
      state.lineDashOffset = v;
    },
    textBaseline: "alphabetic",
  } as Canvas2D;

  const done = (): Rec[] => {
    flush();
    return preorder(roots);
  };
  return { ctx, roots, done };
}

interface State {
  alpha: number;
  fill: string | object;
  stroke: string | object;
  lineWidth: number;
  shadowColor: string;
  shadowBlur: number;
  shadowDX: number;
  shadowDY: number;
  dash: number[];
  lineDashOffset: number;
  rotation: number;
}

function preorder(roots: Rec[]): Rec[] {
  const out: Rec[] = [];
  const walk = (r: Rec): void => {
    out.push(r);
    r.kids.forEach(walk);
  };
  roots.forEach(walk);
  return out;
}

interface DumpNode {
  path: string;
  name: string;
  rect: { x: number; y: number; w: number; h: number };
  text: string[] | null;
  fill: string | null;
  border: string | null;
  borderWidth: number | null;
  textColor: string | null;
  radius: [number, number, number, number];
  clip: boolean;
  opacity: number;
  rotate: number;
  shadows: { dx: number; dy: number; blur: number; color: string }[];
  gradient: { angle: number; stops: { offset: number; color: string }[] } | null;
}

interface DumpLink {
  parent: string;
  path: (string | number)[][];
  color: string;
  width: number;
  dash: number[] | null;
  phase: number;
  arrow: boolean;
}

/** The web scene as tennlayout writes it: the tree of boxes paired with the recorder tree in paint order. */
function dump(scene: TennScene): { width: number; height: number; nodes: DumpNode[]; links: DumpLink[] } {
  const { ctx, done } = recorder();
  scene.draw(ctx);
  const recs = done();
  const boxes = scene.boxes;
  assert.equal(recs.length, boxes.length, `recorded ${recs.length} nodes, boxes has ${boxes.length}: ${boxes.map((b) => b.path).join(", ")}`);
  const abs: { x: number; y: number }[] = [];
  const nodes = boxes.map((b, i) => {
    const r = recs[i]!;
    assert.ok(Math.abs(r.x - b.x) < 0.001 && Math.abs(r.y - b.y) < 0.001, `node ${i} ${b.path}: recorded at ${r.x},${r.y}, box at ${b.x},${b.y}`);
    const parent = b.depth > 0 ? abs[b.depth - 1]! : { x: 0, y: 0 };
    const x = parent.x + b.x;
    const y = parent.y + b.y;
    abs[b.depth] = { x, y };
    return {
      path: b.path,
      name: b.name,
      rect: { x, y, w: b.width, h: b.height },
      text: r.text.length > 0 ? r.text : null,
      fill: r.fill,
      border: r.border,
      borderWidth: r.borderWidth,
      textColor: r.textColor,
      radius: r.radius,
      clip: r.clip,
      opacity: r.own === null || r.base === 0 ? 1 : r.own / r.base,
      rotate: r.rotate,
      shadows: r.shadows,
      gradient: r.gradient === null ? null : { angle: r.gradient.angle, stops: r.gradient.stops },
    } satisfies DumpNode;
  });
  const links: DumpLink[] = [];
  boxes.forEach((b, i) => {
    for (const lk of recs[i]!.links) links.push({ parent: b.path, ...lk });
  });
  return { width: scene.width, height: scene.height, nodes, links };
}

function near(a: number, b: number, tolerance: number): boolean {
  return Math.abs(a - b) <= tolerance;
}

function colorNear(a: string, b: string): boolean {
  const parse = (s: string): number[] => {
    const m = /^#([0-9a-f]{6})([0-9a-f]{2})?$/i.exec(s);
    return m === null ? [] : [0, 2, 4].map((i) => parseInt(m[1]!.slice(i, i + 2), 16)).concat(m[2] === undefined ? [255] : [parseInt(m[2], 16)]);
  };
  const pa = parse(a);
  const pb = parse(b);
  return pa.length === 4 && pb.length === 4 && pa.every((v, i) => Math.abs(v - pb[i]!) <= 1);
}

/** Differences of two dumps, in words; the test prints the first ten. */
function diff(want: ReturnType<typeof dump>, got: ReturnType<typeof dump>, limit = 10): string[] {
  const out: string[] = [];
  const add = (s: string): void => {
    if (out.length < limit) out.push(s);
  };
  if (want.nodes.length !== got.nodes.length) {
    add(`nodes: go has ${want.nodes.length}, web has ${got.nodes.length}`);
  }
  if (!near(want.width, got.width, 0.5)) add(`width: go ${want.width}, web ${got.width}`);
  if (!near(want.height, got.height, 0.5)) add(`height: go ${want.height}, web ${got.height}`);
  const n = Math.min(want.nodes.length, got.nodes.length);
  for (let i = 0; i < n; i += 1) {
    const a = want.nodes[i]!;
    const b = got.nodes[i]!;
    const at = `#${i} ${a.path || "(root)"}`;
    if (a.path !== b.path) add(`${at}: path go "${a.path}", web "${b.path}"`);
    if (a.name !== b.name) add(`${at}: name go "${a.name}", web "${b.name}"`);
    for (const k of ["x", "y", "w", "h"] as const) {
      if (!near(a.rect[k], b.rect[k], 0.5)) add(`${at}: rect.${k} go ${a.rect[k]}, web ${b.rect[k]}`);
    }
    if (JSON.stringify(a.text) !== JSON.stringify(b.text)) add(`${at}: text go ${JSON.stringify(a.text)}, web ${JSON.stringify(b.text)}`);
    for (const k of ["fill", "border", "textColor"] as const) {
      if ((a[k] === null) !== (b[k] === null)) add(`${at}: ${k} go ${String(a[k])}, web ${String(b[k])}`);
      else if (a[k] !== null && b[k] !== null && !colorNear(a[k]!, b[k]!)) add(`${at}: ${k} go ${a[k]}, web ${b[k]}`);
    }
    if (a.borderWidth !== null || b.borderWidth !== null) {
      if (a.borderWidth === null || b.borderWidth === null || !near(a.borderWidth, b.borderWidth, 0.5)) {
        add(`${at}: borderWidth go ${String(a.borderWidth)}, web ${String(b.borderWidth)}`);
      }
    }
    for (let k = 0; k < 4; k += 1) {
      if (!near(a.radius[k]!, b.radius[k]!, 0.5)) add(`${at}: radius[${k}] go ${a.radius[k]}, web ${b.radius[k]}`);
    }
    if (a.clip !== b.clip) add(`${at}: clip go ${a.clip}, web ${b.clip}`);
    if (!near(a.opacity, b.opacity, 0.01)) add(`${at}: opacity go ${a.opacity}, web ${b.opacity}`);
    if (!near(a.rotate, b.rotate, 0.1)) add(`${at}: rotate go ${a.rotate}, web ${b.rotate}`);
    if (a.shadows.length !== b.shadows.length) {
      add(`${at}: ${a.shadows.length} shadow(s) in go, ${b.shadows.length} in web`);
    } else {
      a.shadows.forEach((s, k) => {
        const t = b.shadows[k]!;
        for (const key of ["dx", "dy", "blur"] as const) {
          if (!near(s[key], t[key], 0.5)) add(`${at}: shadow ${k}.${key} go ${s[key]}, web ${t[key]}`);
        }
        if (!colorNear(s.color, t.color)) add(`${at}: shadow ${k} colour go ${s.color}, web ${t.color}`);
      });
    }
    if ((a.gradient === null) !== (b.gradient === null)) add(`${at}: gradient go ${JSON.stringify(a.gradient)}, web ${JSON.stringify(b.gradient)}`);
    else if (a.gradient !== null && b.gradient !== null) {
      if (!near(a.gradient.angle, b.gradient.angle, 0.5)) add(`${at}: gradient angle go ${a.gradient.angle}, web ${b.gradient.angle}`);
      if (a.gradient.stops.length !== b.gradient.stops.length) add(`${at}: gradient stops go ${a.gradient.stops.length}, web ${b.gradient.stops.length}`);
      else {
        a.gradient.stops.forEach((s, k) => {
          const t = b.gradient!.stops[k]!;
          if (!near(s.offset, t.offset, 0.005)) add(`${at}: gradient stop ${k} offset go ${s.offset}, web ${t.offset}`);
          if (!colorNear(s.color, t.color)) add(`${at}: gradient stop ${k} colour go ${s.color}, web ${t.color}`);
        });
      }
    }
  }
  const key = (l: DumpLink): string => `${l.parent}\u0000${JSON.stringify(l.path)}`;
  const wa = [...want.links].sort((x, y) => key(x).localeCompare(key(y)));
  const ga = [...got.links].sort((x, y) => key(x).localeCompare(key(y)));
  if (wa.length !== ga.length) add(`links: go has ${wa.length}, web has ${ga.length}`);
  for (let i = 0; i < Math.min(wa.length, ga.length); i += 1) {
    const a = wa[i]!;
    const b = ga[i]!;
    const at = `link ${i} (${a.parent})`;
    if (a.parent !== b.parent) {
      add(`${at}: parent go "${a.parent}", web "${b.parent}"`);
      continue;
    }
    if (!pathNear(a.path, b.path)) add(`${at}: path go ${JSON.stringify(a.path)}, web ${JSON.stringify(b.path)}`);
    if (!colorNear(a.color, b.color)) add(`${at}: colour go ${a.color}, web ${b.color}`);
    if (!near(a.width, b.width, 0.5)) add(`${at}: width go ${a.width}, web ${b.width}`);
    if (JSON.stringify(a.dash) !== JSON.stringify(b.dash)) add(`${at}: dash go ${JSON.stringify(a.dash)}, web ${JSON.stringify(b.dash)}`);
    if (!near(a.phase, b.phase, 0.5)) add(`${at}: dash phase go ${a.phase}, web ${b.phase}`);
    if (a.arrow !== b.arrow) add(`${at}: arrow go ${a.arrow}, web ${b.arrow}`);
  }
  return out;
}

/** The same verbs, with numbers within half a pixel. */
function pathNear(a: (string | number)[][], b: (string | number)[][]): boolean {
  if (a.length !== b.length) return false;
  return a.every((cmd, i) => {
    const other = b[i]!;
    if (cmd.length !== other.length || cmd[0] !== other[0]) return false;
    for (let k = 1; k < cmd.length; k += 1) {
      if (!near(cmd[k] as number, other[k] as number, 0.5)) return false;
    }
    return true;
  });
}

function load(name: string): { source: string; golden: ReturnType<typeof dump> & { element: string } } {
  const text = readFileSync(`${fixtures}${name}.tenn`, "utf8");
  const golden = JSON.parse(readFileSync(`${fixtures}${name}.layout.json`, "utf8")) as ReturnType<typeof dump> & { element: string };
  return { source: text, golden };
}

const names = readdirSync(fixtures)
  .filter((f) => f.endsWith(".layout.json"))
  .map((f) => f.slice(0, -".layout.json".length))
  .sort();

for (const name of names) {
  test(`scene parity: ${name}`, () => {
    const { source, golden } = load(name);
    const model = readTenn(source);
    assert.ok(model !== null, "the fixture parses");
    const element: Element | null = findElement(model, golden.element);
    assert.ok(element !== null, `element ${golden.element}`);
    const got = dump(
      buildTennScene(element, {
        file: `${name}.tenn`,
        source,
        readFile: (p) => {
          try {
            return readFileSync(`${fixtures}${p}`, "utf8");
          } catch {
            return null;
          }
        },
      }),
    );
    const differences = diff(golden, got);
    assert.ok(differences.length === 0, `${name}: ${differences.length} difference(s) from go/cmd/tennlayout\n${differences.join("\n")}`);
  });
}
