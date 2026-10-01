import { TennParser, parseTenn } from "@tenniarb/core";
import type { Element, TennError } from "@tenniarb/core";
import { INTER_FONTS, buildScene, preloadImages } from "@tenniarb/render";
import type { DecodedImage } from "@tenniarb/render";

const $ = <T extends HTMLElement>(id: string): T => document.getElementById(id) as T;
const canvas = $<HTMLCanvasElement>("canvas");
const main = $("main");
const tree = $("tree");
const status = $("status");
const ctx = canvas.getContext("2d")!;
const dark = matchMedia("(prefers-color-scheme: dark)");

type Scene = ReturnType<typeof buildScene>;
interface Shown { element: Element; scene: Scene; width: number; height: number }

let root: Element | null = null;
let shown: Shown | null = null;
let view = { x: 0, y: 0, k: 1 };
let frame = 0;
const rows = new Map<Element, { row: HTMLElement; box: HTMLElement | null; arrow: HTMLElement | null }>();
// Last frame's draw time; read by scripts/smoke.ts.
(window as unknown as { __viewer: object }).__viewer = { drawMs: 0, buildMs: 0, name: "", items: 0 };
const stats = (window as unknown as { __viewer: { drawMs: number; buildMs: number; name: string; items: number } }).__viewer;

const fontsReady = Promise.all(
  INTER_FONTS.map(async (f) => {
    const face = new FontFace(f.family, `url(fonts/${f.file})`, { weight: f.weight, style: f.style });
    document.fonts.add(await face.load());
  }),
);

async function decode(base64: string): Promise<DecodedImage | null> {
  try {
    const bytes = Uint8Array.from(atob(base64), (c) => c.charCodeAt(0));
    const bmp = await createImageBitmap(new Blob([bytes]));
    return { image: bmp, width: bmp.width, height: bmp.height };
  } catch {
    return null; // corrupt payload: renders as missing
  }
}

const pathOf = (e: Element): string => {
  const names: string[] = [];
  for (let c: Element | null = e; c !== null && c.parent !== null; c = c.parent) names.unshift(encodeURIComponent(c.name));
  return "/" + names.join("/");
};

function findByPath(path: string): Element | null {
  let cur = root;
  for (const part of path.split("/").filter((p) => p !== "")) {
    const name = decodeURIComponent(part);
    cur = cur?.elements.find((c) => c.name === name) ?? null;
  }
  return cur === root ? null : cur;
}

const firstWithItems = (e: Element): Element | null =>
  e.items.length > 0 && e.parent !== null ? e : e.elements.map(firstWithItems).find((c) => c !== null) ?? e.elements[0] ?? null;

function buildTree(parent: HTMLElement, e: Element): void {
  for (const child of e.elements) {
    const row = document.createElement("div");
    row.className = "row";
    const arrow = document.createElement("span");
    arrow.className = "arrow";
    const name = document.createElement("span");
    name.className = "name";
    name.textContent = child.name || "(unnamed)";
    row.append(arrow, name);
    parent.append(row);
    let box: HTMLElement | null = null;
    if (child.elements.length > 0) {
      box = document.createElement("div");
      box.className = "children";
      parent.append(box);
      buildTree(box, child);
      arrow.textContent = "▼";
      arrow.onclick = (ev) => {
        ev.stopPropagation();
        setCollapsed(child, !box!.classList.contains("hidden"));
      };
    }
    row.onclick = () => (location.hash = pathOf(child));
    rows.set(child, { row, box, arrow: box ? arrow : null });
  }
}

function setCollapsed(e: Element, collapsed: boolean): void {
  const r = rows.get(e)!;
  r.box?.classList.toggle("hidden", collapsed);
  if (r.arrow) r.arrow.textContent = collapsed ? "▶" : "▼";
}

function applyFilter(q: string): void {
  q = q.trim().toLowerCase();
  const visit = (e: Element): boolean => {
    let any = false;
    for (const c of e.elements) {
      const hit = q === "" || c.name.toLowerCase().includes(q);
      const sub = visit(c);
      rows.get(c)!.row.classList.toggle("hidden", !(hit || sub));
      if (q !== "" && sub) setCollapsed(c, false);
      any ||= hit || sub;
    }
    return any;
  };
  if (root) visit(root);
}

const viewSize = () => ({ w: canvas.clientWidth, h: canvas.clientHeight });

function draw(): void {
  frame = 0;
  const dpr = window.devicePixelRatio || 1;
  const { w, h } = viewSize();
  if (canvas.width !== Math.round(w * dpr) || canvas.height !== Math.round(h * dpr)) {
    canvas.width = Math.round(w * dpr);
    canvas.height = Math.round(h * dpr);
  }
  ctx.setTransform(1, 0, 0, 1, 0, 0);
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  if (shown === null) return;
  const t0 = performance.now();
  ctx.setTransform(dpr * view.k, 0, 0, dpr * view.k, dpr * view.x, dpr * view.y);
  // Same flip as renderElement: the scene is y-up.
  ctx.translate(0, shown.height);
  ctx.scale(1, -1);
  shown.scene.draw(ctx);
  stats.drawMs = performance.now() - t0;
  status.textContent = `${Math.round(view.k * 100)}%`;
}

const redraw = (): void => {
  if (frame === 0) frame = requestAnimationFrame(draw);
};

function fit(): void {
  if (shown === null) return;
  const { w, h } = viewSize();
  view.k = Math.min(w / shown.width, h / shown.height, 1);
  view.x = (w - shown.width * view.k) / 2;
  view.y = (h - shown.height * view.k) / 2;
  redraw();
}

function zoomAt(factor: number, cx: number, cy: number): void {
  const k = Math.min(Math.max(view.k * factor, 0.02), 16);
  view.x = cx - ((cx - view.x) / view.k) * k;
  view.y = cy - ((cy - view.y) / view.k) * k;
  view.k = k;
  redraw();
}

let showToken = 0;
async function show(e: Element): Promise<void> {
  const token = ++showToken;
  await fontsReady;
  const t0 = performance.now();
  const decodeImage = await preloadImages(e, decode);
  if (token !== showToken) return;
  const scene = buildScene(e, { darkMode: dark.matches, decodeImage, measureContext: ctx });
  const b = scene.getBounds();
  scene.offset = { x: 15 - b.x, y: 15 - b.y };
  shown = { element: e, scene, width: b.width + 30, height: b.height + 30 };
  stats.buildMs = performance.now() - t0;
  stats.name = pathOf(e);
  stats.items = e.items.length;
  fit();
}

function select(e: Element | null): void {
  for (const { row } of rows.values()) row.classList.remove("sel");
  if (e === null) return;
  for (let p = e.parent; p !== null; p = p.parent) if (rows.has(p)) setCollapsed(p, false);
  const row = rows.get(e)!.row;
  row.classList.add("sel");
  row.scrollIntoView({ block: "nearest" });
  document.title = `${e.name} - Tenniarb`;
  void show(e);
}

function onHash(): void {
  const e = findByPath(location.hash.slice(1));
  if (e !== null) select(e);
}

function showErrors(errors: readonly TennError[], source: string): void {
  const box = $("errors");
  box.classList.remove("hidden");
  $("empty").classList.add("hidden");
  const h = document.createElement("h3");
  h.textContent = `${source}: ${errors.length} parse error(s)`;
  const pre = document.createElement("pre");
  pre.textContent = errors.map((er) => `${er.line}:${er.col} ${er.message}`).join("\n");
  box.replaceChildren(h, pre);
}

function load(text: string, source: string): void {
  const parser = new TennParser();
  const parsed = parser.parse(text.charCodeAt(0) === 0xfeff ? text.slice(1) : text);
  if (parser.errors.hasErrors()) {
    showErrors(parser.errors.errors, source);
    return;
  }
  root = parseTenn(parsed);
  shown = null;
  rows.clear();
  tree.replaceChildren();
  buildTree(tree, root);
  $("errors").classList.add("hidden");
  $("empty").classList.add("hidden");
  const e = findByPath(location.hash.slice(1)) ?? firstWithItems(root);
  if (e === null) return;
  if (location.hash.slice(1) !== pathOf(e)) history.replaceState(null, "", "#" + pathOf(e));
  select(e);
}

async function loadFile(file: File): Promise<void> {
  load(await file.text(), file.name);
}

async function loadUrl(url: string): Promise<void> {
  try {
    const res = await fetch(url);
    if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
    load(await res.text(), url);
  } catch (err) {
    showErrors([{ line: 0, col: 0, message: `cannot load: ${String(err)}` } as TennError], url);
  }
}

let drag: { x: number; y: number } | null = null;
canvas.addEventListener("pointerdown", (ev) => {
  drag = { x: ev.clientX, y: ev.clientY };
  canvas.setPointerCapture(ev.pointerId);
  canvas.classList.add("drag");
});
canvas.addEventListener("pointermove", (ev) => {
  if (drag === null) return;
  view.x += ev.clientX - drag.x;
  view.y += ev.clientY - drag.y;
  drag = { x: ev.clientX, y: ev.clientY };
  redraw();
});
const endDrag = (): void => {
  drag = null;
  canvas.classList.remove("drag");
};
canvas.addEventListener("pointerup", endDrag);
canvas.addEventListener("pointercancel", endDrag);
canvas.addEventListener(
  "wheel",
  (ev) => {
    ev.preventDefault();
    // ctrl+wheel is how browsers report a trackpad pinch; a plain wheel/two-finger scroll pans.
    if (ev.ctrlKey || ev.metaKey) {
      const r = canvas.getBoundingClientRect();
      zoomAt(Math.exp(-ev.deltaY * 0.01), ev.clientX - r.left, ev.clientY - r.top);
      return;
    }
    view.x -= ev.deltaX;
    view.y -= ev.deltaY;
    redraw();
  },
  { passive: false },
);

const zoomCenter = (f: number): void => zoomAt(f, canvas.clientWidth / 2, canvas.clientHeight / 2);
addEventListener("keydown", (ev) => {
  if (ev.target instanceof HTMLInputElement || ev.metaKey || ev.ctrlKey) return;
  if (ev.key === "+" || ev.key === "=") zoomCenter(1.25);
  else if (ev.key === "-") zoomCenter(0.8);
  else if (ev.key === "0") zoomAt(1 / view.k, canvas.clientWidth / 2, canvas.clientHeight / 2);
});
$("zin").onclick = () => zoomCenter(1.25);
$("zout").onclick = () => zoomCenter(0.8);
$("z100").onclick = () => zoomCenter(1 / view.k);
$("fit").onclick = fit;
$("filter").addEventListener("input", (ev) => applyFilter((ev.target as HTMLInputElement).value));
$("open").onclick = () => $("picker").click();
$("picker").onchange = (ev) => {
  const f = (ev.target as HTMLInputElement).files?.[0];
  if (f) void loadFile(f);
};
addEventListener("dragover", (ev) => {
  ev.preventDefault();
  main.classList.add("over");
});
addEventListener("dragleave", () => main.classList.remove("over"));
addEventListener("drop", (ev) => {
  ev.preventDefault();
  main.classList.remove("over");
  const f = ev.dataTransfer?.files[0];
  if (f) void loadFile(f);
});
addEventListener("hashchange", onHash);
addEventListener("resize", redraw);
dark.addEventListener("change", () => shown && void show(shown.element));

const url = new URLSearchParams(location.search).get("file");
if (url !== null) void loadUrl(url);
