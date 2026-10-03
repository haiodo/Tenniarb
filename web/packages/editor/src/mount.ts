import { readTenn } from "@tenniarb/core";
import type { DiagramItem, Element } from "@tenniarb/core";
import { decode, defaultFontBase, loadFonts } from "@tenniarb/embed";
import { allElements, fontCss, findElement, preloadImages } from "@tenniarb/render";
import type { ImageDecoder, Point, Rect } from "@tenniarb/render";
import { hitTest } from "./selection.ts";
import { mountOutline } from "./outline.ts";
import type { Outline } from "./outline.ts";
import { mountProperties } from "./properties-panel.ts";
import type { PropertiesPanel } from "./properties-panel.ts";
import { EditorSession } from "./session.ts";
import { mountToolbar } from "./toolbar.ts";
import type { Toolbar } from "./toolbar.ts";
import type { EditMode } from "./session.ts";

export interface EditorOptions {
  text: string;
  /** Serialized document after each committed change. */
  onChange?: (text: string) => void;
  /** No selection or edits; pan and zoom stay. */
  readonly?: boolean;
  /** Element name or "A/B" path. Default: the first element with items. */
  element?: string;
  /** Container for the properties panel: Tenn text of the selected item (or the element), applied after a pause. */
  properties?: HTMLElement;
  /** Container for the outline: element tree (switch, rename, add, delete, drag into another element) and item search. Readonly keeps navigation and search. */
  outline?: HTMLElement;
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

// The scene needs a sync decoder and the edited element can change, so every element's images are loaded up front.
async function decoderFor(root: Element): Promise<ImageDecoder> {
  const decoders = await Promise.all(allElements(root).map((e) => preloadImages(e, decode)));
  return (base64) => {
    for (const d of decoders) {
      const image = d(base64);
      if (image !== null) return image;
    }
    return null;
  };
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

  let panel: PropertiesPanel | undefined;
  let outline: Outline | undefined;
  let toolbar: Toolbar | undefined;
  const darkMode = matchMedia("(prefers-color-scheme: dark)").matches;
  const session = new EditorSession(element, {
    darkMode,
    decodeImage: await decoderFor(root),
    measureContext: ctx,
    readonly: opts.readonly,
    onChange: (text) => {
      opts.onChange?.(text);
      panel?.sync(true);
      outline?.sync();
    },
    onElement: () => {
      finishEdit(false);
      fit();
      outline?.sync();
    },
    onRedraw: () => {
      redraw();
      panel?.sync();
      toolbar?.sync();
    },
  });
  if (opts.properties !== undefined) {
    opts.properties.replaceChildren();
    panel = mountProperties(opts.properties, session, { darkMode, readonly: opts.readonly });
    panel.sync(true);
  }

  if (opts.outline !== undefined) outline = mountOutline(opts.outline, session, { readonly: opts.readonly, onFocusCanvas: () => box.focus() });

  if (!opts.readonly) {
    toolbar = mountToolbar(box, session, () => box.focus());
    toolbar.sync();
  }

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
    if (editing !== null) place(editing);
  }
  const redraw = (): void => {
    if (frame === 0) frame = requestAnimationFrame(draw);
  };

  // Text overlay, scene-space rect mapped through the current view.
  let editing: { item: DiagramItem; mode: EditMode; ta: HTMLTextAreaElement; rect: Rect; fontSize: number } | null = null;
  function place(e: NonNullable<typeof editing>): void {
    const st = e.ta.style;
    st.left = `${view.x + e.rect.x * view.k}px`;
    st.top = `${view.y - (e.rect.y + e.rect.height) * view.k}px`;
    st.width = `${e.rect.width * view.k}px`;
    st.height = `${e.rect.height * view.k}px`;
    st.font = fontCss({ size: e.fontSize * view.k, bold: false, italic: false });
  }

  function finishEdit(commit: boolean): void {
    if (editing === null) return;
    const { item, mode, ta } = editing;
    editing = null; // before remove(): removing a focused textarea fires blur
    const text = ta.value;
    ta.remove();
    if (commit) session.commitEdit(item, mode, text);
    box.focus();
  }

  function startEdit(item: DiagramItem, mode: EditMode): void {
    finishEdit(true);
    const target = session.editTarget(item, mode);
    if (target === null) return;
    const ta = document.createElement("textarea");
    ta.value = target.text;
    ta.style.cssText =
      "position:absolute;box-sizing:border-box;resize:none;margin:0;padding:2px 6px;border:1px solid gray;border-radius:8px;outline:none;color-scheme:light dark;background:Canvas;color:CanvasText";
    // Enter commits, Shift+Enter is a newline, Esc cancels.
    ta.addEventListener("keydown", (ev) => {
      ev.stopPropagation();
      if (ev.key === "Escape") finishEdit(false);
      else if (ev.key === "Enter" && !ev.shiftKey) {
        ev.preventDefault();
        finishEdit(true);
      }
    });
    ta.addEventListener("blur", () => finishEdit(true));
    editing = { item, mode, ta, rect: target.rect, fontSize: target.fontSize };
    place(editing);
    box.append(ta);
    ta.focus();
    ta.select();
  }

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
  canvas.addEventListener("dblclick", (ev) => {
    const hit = opts.readonly ? undefined : hitTest(session.scene, toScene(ev)).at(-1);
    if (hit === undefined) fit();
    else startEdit(hit, "name");
  });
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
    const mod = ev.metaKey || ev.ctrlKey;
    const key = ev.key.toLowerCase();
    if (mod && key === "z") {
      ev.preventDefault();
      if (ev.shiftKey) session.redo();
      else session.undo();
    } else if (mod && key === "d") {
      ev.preventDefault();
      session.duplicate();
    } else if (!mod && ev.key === "Enter" && session.selection.length > 0) {
      ev.preventDefault();
      startEdit(session.selection[0]!, ev.shiftKey ? "body" : "name");
    } else if (!mod && (ev.key === "Backspace" || ev.key === "Delete")) {
      ev.preventDefault();
      session.deleteSelection();
    }
  });
  // Clipboard events instead of the async API: no permission prompt. The text overlay keeps its native copy/paste.
  const clip = (ev: ClipboardEvent, text: string | null): void => {
    if (text === null) return;
    ev.clipboardData?.setData("text/plain", text);
    ev.preventDefault();
  };
  box.addEventListener("copy", (ev) => editing === null && clip(ev, session.copyText()));
  box.addEventListener("cut", (ev) => editing === null && clip(ev, session.cut()));
  box.addEventListener("paste", (ev) => {
    if (editing === null && session.paste(ev.clipboardData?.getData("text/plain") ?? "")) ev.preventDefault();
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
      panel?.destroy();
      outline?.destroy();
      toolbar?.destroy();
      ro.disconnect();
      cancelAnimationFrame(frame);
      box.remove();
    },
  };
}
