// Diagram canvas of a session (editor and mind map): view, drawing, pointer, wheel, keys, text overlay.
import type { DiagramItem } from "@tenniarb/core";
import { fontCss, shadowScale } from "@tenniarb/render";
import type { Point, Rect } from "@tenniarb/render";
import { drawIndicators } from "./indicators.ts";
import { mountQuickPanel } from "./quick-panel.ts";
import { hitTest, selectionZoom } from "./selection.ts";
import type { EditMode, EditorSession } from "./session.ts";

const isMac = typeof navigator !== "undefined" && navigator.userAgent.includes("Mac");

export interface CanvasOptions {
  /** Off-screen indicator colours. */
  dark: () => boolean;
  /** Quick style panel over a single selected item. */
  quickPanel: () => boolean;
  /** Cmd+Z / Cmd+Shift+Z; default the session's own. */
  undo?: () => void;
  redo?: () => void;
  /** A plain wheel pans; false leaves it to the page (zoom stays on Ctrl/Cmd + wheel). Default true. */
  wheelPans?: boolean;
  /** After each frame. */
  onDraw?: () => void;
}

export interface CanvasView {
  /** Screen = x + k * sceneX, y - k * sceneY. */
  readonly view: { x: number; y: number; k: number };
  redraw(): void;
  fit(): void;
  zoomAt(factor: number, cx: number, cy: number): void;
  centre(): void;
  /** Swift centerItem: pan so that `item` is at the view centre, `offset` scene units lower. */
  centerItem(item: DiagramItem, offset?: number): void;
  toScene(ev: { clientX: number; clientY: number }): Point;
  startEdit(item: DiagramItem, mode: EditMode): void;
  finishEdit(commit: boolean): void;
  isEditing(): boolean;
  destroy(): void;
}

/** `box` is the focusable container of `canvas`; `canvas` must be the measure context of the session. */
export function attachCanvas(box: HTMLElement, canvas: HTMLCanvasElement, session: EditorSession, opts: CanvasOptions): CanvasView {
  const ctx = canvas.getContext("2d")!;
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
    shadowScale.value = dpr;
    selectionZoom.value = view.k;
    session.draw(ctx);
    shadowScale.value = 1;
    drawIndicators(ctx, session.scene.drawables.values(), {
      ox: view.x / view.k,
      oy: (box.clientHeight - view.y) / view.k,
      k: view.k,
      w: box.clientWidth,
      h: box.clientHeight,
      dpr,
      dark: opts.dark(),
    });
    opts.onDraw?.();
    if (editing !== null) place(editing);
    quickPanel(opts.quickPanel() && !session.readonly && editing === null && !active && session.selection.length === 1 ? session.selection[0]! : null, view);
  }
  const quickPanel = mountQuickPanel(box, session);
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
    editing = null;
    session.editing = null; // before remove(): removing a focused textarea fires blur
    const text = ta.value;
    ta.remove();
    if (commit) session.commitEdit(item, mode, text);
    box.focus();
    redraw();
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
    session.editing = { item, rect: target.rect };
    place(editing);
    box.append(ta);
    redraw();
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

  function centre(): void {
    const b = session.scene.getBounds();
    view.x = box.clientWidth / 2 - (b.x + b.width / 2);
    view.y = box.clientHeight / 2 + (b.y + b.height / 2);
    redraw();
  }

  function centerItem(item: DiagramItem, offset = 0): void {
    const b = session.scene.drawables.get(item)?.getBounds();
    if (b === undefined) return;
    view.x = box.clientWidth / 2 - (b.x + b.width / 2) * view.k;
    view.y = box.clientHeight / 2 + (offset + b.y + b.height / 2) * view.k;
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
    if (ev.button !== 0) return;
    canvas.setPointerCapture(ev.pointerId);
    // Mac as Swift: Cmd toggles, Ctrl draws a link, Option drags with children. Elsewhere Ctrl toggles, Ctrl+Alt draws, Alt drags.
    const [toggle, line, alt] = isMac ? [ev.metaKey, ev.ctrlKey, ev.altKey] : [ev.ctrlKey && !ev.altKey, ev.ctrlKey && ev.altKey, ev.altKey && !ev.ctrlKey];
    active = session.down(toScene(ev), { toggle, band: ev.shiftKey, line, alt });
    last = active ? null : { x: ev.clientX, y: ev.clientY };
    redraw();
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
    if (active) session.up(toScene(ev), ev.type === "pointercancel");
    active = false;
    last = null;
    redraw();
  };
  canvas.addEventListener("pointerup", release);
  canvas.addEventListener("pointercancel", release);
  canvas.addEventListener("dblclick", (ev) => {
    const hit = session.readonly ? undefined : hitTest(session.scene, toScene(ev)).at(-1);
    if (hit === undefined) fit();
    else startEdit(hit, "name");
  });
  canvas.addEventListener(
    "wheel",
    (ev) => {
      if (ev.ctrlKey || ev.metaKey) {
        ev.preventDefault();
        const r = canvas.getBoundingClientRect();
        zoomAt(Math.exp(-ev.deltaY * 0.01), ev.clientX - r.left, ev.clientY - r.top);
      } else if (opts.wheelPans !== false) {
        ev.preventDefault();
        view.x -= ev.deltaX;
        view.y -= ev.deltaY;
        redraw();
      }
    },
    { passive: false },
  );
  const arrows: Record<string, [number, number]> = { ArrowLeft: [-1, 0], ArrowRight: [1, 0], ArrowUp: [0, 1], ArrowDown: [0, -1] };
  box.addEventListener("keydown", (ev) => {
    const mod = ev.metaKey || ev.ctrlKey;
    const key = ev.key.toLowerCase();
    if (mod && key === "z") {
      ev.preventDefault();
      if (ev.shiftKey) (opts.redo ?? (() => session.redo()))();
      else (opts.undo ?? (() => session.undo()))();
    } else if (mod && key === "d") {
      ev.preventDefault();
      session.duplicate();
    } else if (mod && key === "a") {
      ev.preventDefault();
      session.selectAll(ev.shiftKey ? "Item" : undefined);
    } else if (!mod && ev.key === "Enter" && session.selection.length > 0) {
      ev.preventDefault();
      startEdit(session.selection[0]!, ev.altKey ? "value" : ev.shiftKey ? "body" : "name");
    } else if (!mod && (ev.key === "Backspace" || ev.key === "Delete" || ev.key === "x")) {
      ev.preventDefault();
      session.deleteSelection();
    } else if (!mod && ev.key === "Tab") {
      ev.preventDefault();
      session.addNewItem(ev.altKey);
    } else if (arrows[ev.key] !== undefined && session.selection.length > 0) {
      ev.preventDefault();
      // Swift: Shift (from the edge) or Cmd (from the centre) + arrow resizes a single item, anything else moves the selection.
      const fromCentre = isMac ? ev.metaKey : ev.ctrlKey;
      if ((ev.shiftKey || fromCentre) && session.selection.length === 1) session.resizeBy(ev.key as Parameters<typeof session.resizeBy>[0], fromCentre);
      else session.moveBy(...arrows[ev.key]!);
    }
  });
  const ro = new ResizeObserver(redraw);
  ro.observe(box);

  return {
    view,
    redraw,
    fit,
    zoomAt,
    centre,
    centerItem,
    toScene,
    startEdit,
    finishEdit,
    isEditing: () => editing !== null,
    destroy: () => {
      ro.disconnect();
      cancelAnimationFrame(frame);
    },
  };
}
