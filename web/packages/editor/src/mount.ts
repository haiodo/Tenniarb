import { readTenn } from "@tenniarb/core";
import type { DiagramItem, Element } from "@tenniarb/core";
import { decode, defaultFontBase, loadFonts } from "@tenniarb/embed";
import { allElements, findElement, preloadImages } from "@tenniarb/render";
import type { Point } from "@tenniarb/render";
import { EditorSession } from "./session.ts";

export interface EditorOptions {
  text: string;
  /** Serialized document after each committed change. */
  onChange?: (text: string) => void;
  /** No selection or edits; pan and zoom stay. */
  readonly?: boolean;
  /** Element name or "A/B" path. Default: the first element with items. */
  element?: string;
  /** Directory with Inter-*.woff2. */
  fontBaseUrl?: string;
}

export interface EditorHandle {
  readonly session: EditorSession;
  undo(): void;
  redo(): void;
  fit(): void;
  selection(): DiagramItem[];
  /** Client (viewport) coordinates of the centre of an item. */
  screenOf(item: DiagramItem): Point;
  text(): string;
  destroy(): void;
}

/** Throws when the text does not parse or the element is missing. */
export async function mount(el: HTMLElement, opts: EditorOptions): Promise<EditorHandle> {
  const root = readTenn(opts.text);
  if (root === null) throw new Error("tenniarb: parse errors in the document");
  const element: Element | null = opts.element !== undefined ? findElement(root, opts.element) : (allElements(root).find((e) => e.items.length > 0) ?? null);
  if (element === null) throw new Error("tenniarb: nothing to edit");
  await loadFonts(opts.fontBaseUrl ?? defaultFontBase).catch((err) => console.warn("tenniarb: font loading failed", err));

  el.replaceChildren();
  const box = document.createElement("div");
  box.tabIndex = 0;
  box.style.cssText = "position:relative;overflow:hidden;width:100%;height:100%;outline:none";
  const canvas = document.createElement("canvas");
  canvas.style.cssText = "display:block;position:absolute;inset:0;width:100%;height:100%;touch-action:none";
  box.append(canvas);
  el.append(box);
  const ctx = canvas.getContext("2d")!;

  const session = new EditorSession(element, {
    darkMode: matchMedia("(prefers-color-scheme: dark)").matches,
    decodeImage: await preloadImages(element, decode),
    measureContext: ctx,
    readonly: opts.readonly,
    onChange: opts.onChange,
    onRedraw: () => redraw(),
  });

  // Screen = view.x + k * x, view.y - k * y: the scene is y-up and does not depend on its own bounds, so edits never shift the picture.
  const view = { x: 0, y: 0, k: 1 };
  let frame = 0;
  function draw(): void {
    frame = 0;
    const dpr = window.devicePixelRatio || 1;
    const w = Math.round(box.clientWidth * dpr);
    const h = Math.round(box.clientHeight * dpr);
    if (canvas.width !== w || canvas.height !== h) {
      canvas.width = w;
      canvas.height = h;
    }
    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.clearRect(0, 0, w, h);
    ctx.setTransform(dpr * view.k, 0, 0, -dpr * view.k, dpr * view.x, dpr * view.y);
    session.draw(ctx);
  }
  const redraw = (): void => {
    if (frame === 0) frame = requestAnimationFrame(draw);
  };

  function fit(): void {
    const b = session.scene.getBounds();
    const [w, h] = [box.clientWidth, box.clientHeight];
    view.k = Math.min((w - 30) / b.width, (h - 30) / b.height, 1);
    view.x = (w - b.width * view.k) / 2 - b.x * view.k;
    view.y = (h - b.height * view.k) / 2 + (b.y + b.height) * view.k;
    redraw();
  }

  function zoomAt(factor: number, cx: number, cy: number): void {
    const k = Math.min(Math.max(view.k * factor, 0.02), 16);
    view.x = cx - ((cx - view.x) / view.k) * k;
    view.y = cy - ((cy - view.y) / view.k) * k;
    view.k = k;
    redraw();
  }

  const toScene = (ev: { clientX: number; clientY: number }): Point => {
    const r = canvas.getBoundingClientRect();
    return { x: (ev.clientX - r.left - view.x) / view.k, y: (view.y - (ev.clientY - r.top)) / view.k };
  };

  let last: { x: number; y: number } | null = null; // set while panning
  let active = false; // a press is owned by the session
  canvas.addEventListener("pointerdown", (ev) => {
    box.focus();
    canvas.setPointerCapture(ev.pointerId);
    active = session.down(toScene(ev), { toggle: ev.metaKey || ev.ctrlKey, band: ev.shiftKey });
    last = active ? null : { x: ev.clientX, y: ev.clientY };
  });
  canvas.addEventListener("pointermove", (ev) => {
    if (active) return session.move(toScene(ev));
    if (last === null) return;
    view.x += ev.clientX - last.x;
    view.y += ev.clientY - last.y;
    last = { x: ev.clientX, y: ev.clientY };
    redraw();
  });
  const release = (ev: PointerEvent): void => {
    if (active) session.up(toScene(ev));
    active = false;
    last = null;
  };
  canvas.addEventListener("pointerup", release);
  canvas.addEventListener("pointercancel", release);
  canvas.addEventListener("dblclick", fit);
  canvas.addEventListener(
    "wheel",
    (ev) => {
      ev.preventDefault();
      if (ev.ctrlKey || ev.metaKey) {
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
  box.addEventListener("keydown", (ev) => {
    if (ev.key.toLowerCase() !== "z" || !(ev.metaKey || ev.ctrlKey)) return;
    ev.preventDefault();
    if (ev.shiftKey) session.redo();
    else session.undo();
  });
  const ro = new ResizeObserver(redraw);
  ro.observe(box);

  fit();
  return {
    session,
    undo: () => session.undo(),
    redo: () => session.redo(),
    fit,
    selection: () => session.selection,
    screenOf: (item) => {
      const b = session.scene.drawables.get(item)!.getSelectorBounds();
      const r = canvas.getBoundingClientRect();
      return { x: r.left + view.x + (b.x + b.width / 2) * view.k, y: r.top + view.y - (b.y + b.height / 2) * view.k };
    },
    text: () => session.text(),
    destroy: () => {
      ro.disconnect();
      cancelAnimationFrame(frame);
      box.remove();
    },
  };
}
