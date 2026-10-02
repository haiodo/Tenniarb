import { TennParser, parseTenn } from "@tenniarb/core";
import type { Element } from "@tenniarb/core";
import { buildScene, findElement, preloadImages } from "@tenniarb/render";
import type { DecodedImage } from "@tenniarb/render";
import { defaultFontBase, loadFonts } from "./fonts.ts";
import { elementTree, pathOf, pickElement } from "./util.ts";
import type { ElementNode } from "./util.ts";

export interface EmbedOptions {
  /** Element name or "A/B" path. Default: the first element with items. */
  element?: string;
  /** Pan (drag), zoom (ctrl/cmd + wheel), double-click to fit. Default true. */
  interactive?: boolean;
  /** false: expressions are not run; use it for documents you do not trust. Default true. */
  evaluate?: boolean;
  /** Directory with Inter-*.woff2. Default: next to the script. Ignored by the standalone build. */
  fontBaseUrl?: string;
  /** A plain wheel pans instead of scrolling the page (full-page viewers). Default false. */
  panOnWheel?: boolean;
}

export interface EmbedHandle {
  setElement(path: string): Promise<void>;
  fit(): void;
  zoomBy(factor: number): void;
  /** Back to 100%, centred. */
  reset(): void;
  destroy(): void;
  elements(): ElementNode[];
  /** Path of the shown element ("" when nothing is shown). */
  readonly path: string;
  /** Last build / draw times in ms. */
  readonly stats: { buildMs: number; drawMs: number };
}

const live = new WeakMap<HTMLElement, EmbedHandle>();

async function decode(base64: string): Promise<DecodedImage | null> {
  try {
    const bytes = Uint8Array.from(atob(base64), (c) => c.charCodeAt(0));
    const bmp = await createImageBitmap(new Blob([bytes]));
    return { image: bmp, width: bmp.width, height: bmp.height };
  } catch {
    return null; // corrupt payload: renders as missing
  }
}

export function showError(el: HTMLElement, title: string, lines: string[]): void {
  const pre = document.createElement("pre");
  pre.className = "tenn-error";
  pre.style.cssText = "margin:0;padding:8px 12px;white-space:pre-wrap;font:12px/1.5 ui-monospace,Menlo,monospace;color:#b00020;border-left:3px solid #b00020";
  pre.textContent = [title, ...lines].join("\n");
  console.error("tenniarb:", pre.textContent);
  el.replaceChildren(pre);
}

/** Never rejects: parse and build errors are shown inside `el`. */
export async function render(el: HTMLElement, text: string, opts: EmbedOptions = {}): Promise<EmbedHandle> {
  live.get(el)?.destroy();
  el.replaceChildren();
  // Measured on the emptied element: only an explicit CSS height survives.
  const fixedHeight = el.clientHeight > 0;
  const interactive = opts.interactive ?? true;

  const parser = new TennParser();
  const tree = parser.parse(text.charCodeAt(0) === 0xfeff ? text.slice(1) : text);
  const root = parser.errors.hasErrors() ? null : parseTenn(tree);

  const stats = { buildMs: 0, drawMs: 0 };
  let current: Element | null = null;
  let alive = true;
  const handle: EmbedHandle = {
    setElement: (p) => setElement(p),
    fit: () => fit(),
    zoomBy: (f) => zoomAt(f, box.clientWidth / 2, box.clientHeight / 2),
    reset: () => zoomAt(1 / view.k, box.clientWidth / 2, box.clientHeight / 2),
    destroy,
    elements: () => (root === null ? [] : elementTree(root)),
    get path() {
      return current === null ? "" : pathOf(current);
    },
    stats,
  };
  live.set(el, handle);

  const box = document.createElement("div");
  box.style.cssText = `position:relative;overflow:hidden;width:100%;margin:0 auto;${fixedHeight ? "height:100%" : ""}`;
  const canvas = document.createElement("canvas");
  canvas.style.cssText = `display:block;position:absolute;inset:0;width:100%;height:100%;${interactive ? "cursor:grab" : ""}`;
  box.append(canvas);
  const ctx = canvas.getContext("2d")!;
  const dark = matchMedia("(prefers-color-scheme: dark)");

  let shown: { scene: ReturnType<typeof buildScene>; width: number; height: number } | null = null;
  const view = { x: 0, y: 0, k: 1 };
  let frame = 0;

  function draw(): void {
    frame = 0;
    const dpr = window.devicePixelRatio || 1;
    const w = box.clientWidth;
    const h = box.clientHeight;
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
  }
  const redraw = (): void => {
    if (alive && frame === 0) frame = requestAnimationFrame(draw);
  };

  function fit(): void {
    if (shown === null) return;
    const w = box.clientWidth;
    const h = box.clientHeight;
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

  let token = 0;
  async function show(e: Element): Promise<void> {
    const mine = ++token;
    if (e.items.length === 0) {
      const names = e.elements.map((c) => pathOf(c));
      showError(el, `"${pathOf(e)}" has no items to draw`, names.length > 0 ? ["nested elements:", ...names] : []);
      return;
    }
    await loadFonts(opts.fontBaseUrl ?? defaultFontBase).catch((err) => console.warn("tenniarb: font loading failed", err));
    const t0 = performance.now();
    const decodeImage = await preloadImages(e, decode);
    if (mine !== token || !alive) return;
    try {
      const scene = buildScene(e, { darkMode: dark.matches, evaluate: opts.evaluate, decodeImage, measureContext: ctx });
      const b = scene.getBounds();
      scene.offset = { x: 15 - b.x, y: 15 - b.y };
      shown = { scene, width: b.width + 30, height: b.height + 30 };
    } catch (err) {
      showError(el, `cannot render "${pathOf(e)}"`, [String(err)]);
      return;
    }
    stats.buildMs = performance.now() - t0;
    current = e;
    if (!fixedHeight) {
      box.style.aspectRatio = `${shown.width} / ${shown.height}`;
      box.style.maxWidth = `${Math.ceil(shown.width)}px`; // never upscale past 100%
    }
    fit();
  }

  async function setElement(spec: string): Promise<void> {
    const e = root === null ? null : findElement(root, spec);
    if (e === null) {
      if (root !== null) showError(el, `element not found: ${spec}`, []);
      return;
    }
    if (!el.contains(box)) el.replaceChildren(box);
    await show(e);
  }

  function destroy(): void {
    alive = false;
    ro.disconnect();
    dark.removeEventListener("change", onDark);
    cancelAnimationFrame(frame);
    box.remove();
    if (live.get(el) === handle) live.delete(el);
  }

  const onDark = (): void => void (current && show(current));
  const ro = new ResizeObserver(() => fit());

  if (interactive) {
    let drag: { x: number; y: number } | null = null;
    // Touch keeps scrolling the page, so there is no touch pan/pinch; it needs pointer-pair tracking.
    canvas.addEventListener("pointerdown", (ev) => {
      if (ev.pointerType === "touch") return;
      drag = { x: ev.clientX, y: ev.clientY };
      canvas.setPointerCapture(ev.pointerId);
      canvas.style.cursor = "grabbing";
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
      canvas.style.cursor = "grab";
    };
    canvas.addEventListener("pointerup", endDrag);
    canvas.addEventListener("pointercancel", endDrag);
    canvas.addEventListener("dblclick", fit);
    canvas.addEventListener(
      "wheel",
      (ev) => {
        // ctrl+wheel is how browsers report a trackpad pinch.
        const zoom = ev.ctrlKey || ev.metaKey;
        if (!zoom && !opts.panOnWheel) return; // let the page scroll
        ev.preventDefault();
        if (zoom) {
          const r = canvas.getBoundingClientRect();
          zoomAt(Math.exp(-ev.deltaY * 0.01), ev.clientX - r.left, ev.clientY - r.top);
        } else {
          view.x -= ev.deltaX;
          view.y -= ev.deltaY;
          redraw();
        }
      },
      { passive: false },
    );
  }

  if (root === null) {
    showError(el, `${parser.errors.errors.length} parse error(s)`, parser.errors.errors.map((er) => `${er.line}:${er.col} ${er.message}`));
    return handle;
  }
  const first = pickElement(root, opts.element);
  if (first === null) {
    showError(el, opts.element === undefined ? "nothing to show: the document has no elements" : `element not found: ${opts.element}`, []);
    return handle;
  }
  el.append(box);
  ro.observe(box);
  dark.addEventListener("change", onDark);
  await show(first);
  return handle;
}
