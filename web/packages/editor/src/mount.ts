import { readTenn, toTennStr } from "@tenniarb/core";
import type { DiagramItem, Element } from "@tenniarb/core";
import { decode, defaultFontBase, loadFonts } from "@tenniarb/embed";
import { allElements, fontCss, findElement, getSceneSize, preloadImages, renderElement, setMeasureContext } from "@tenniarb/render";
import type { Canvas2D, ImageDecoder, Point, Rect } from "@tenniarb/render";
import { drawIndicators } from "./indicators.ts";
import { IMAGE_SVG, DOC_SVG } from "./icons.ts";
import { hitTest } from "./selection.ts";
import { mountOutline } from "./outline.ts";
import type { Outline } from "./outline.ts";
import { mountProperties } from "./properties-panel.ts";
import type { PropertiesPanel } from "./properties-panel.ts";
import { EditorSession } from "./session.ts";
import { showMenu, styleEntries } from "./menu.ts";
import type { Entry } from "./menu.ts";
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
  /** Container for the outline: element tree (switch, rename, add, delete, drag into another element, arrow keys). Readonly keeps navigation. */
  outline?: HTMLElement;
  /** Container for the title bar controls: zoom, help, share (exports), add / remove item (not in readonly). */
  toolbar?: HTMLElement;
  /** Help button handler; without it the button is disabled. */
  onHelp?: () => void;
  /** Receives an export (PNG, element .tenn). Default: a browser download. */
  saveFile?: (name: string, data: Blob) => void | Promise<void>;
  /** Directory with Inter-*.woff2. */
  fontBaseUrl?: string;
}

export interface EditorHandle {
  readonly session: EditorSession;
  undo(): void;
  redo(): void;
  fit(): void;
  /** Swift zoomIn / zoomOut / resetZoom: steps of 0.75 around the view centre, 100% centred on the scene. */
  zoomIn(): void;
  zoomOut(): void;
  resetZoom(): void;
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
  const element: Element | null = opts.element !== undefined ? findElement(root, opts.element) : (allElements(root).find((e) => e.items.length > 0) ?? allElements(root)[0] ?? null);
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
  let bar: Toolbar | undefined;
  const darkMode = matchMedia("(prefers-color-scheme: dark)").matches;
  const decodeImage = await decoderFor(root);
  const session = new EditorSession(element, {
    darkMode,
    decodeImage,
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
    },
  });
  if (opts.properties !== undefined) {
    opts.properties.replaceChildren();
    panel = mountProperties(opts.properties, session, { darkMode, readonly: opts.readonly });
    panel.sync(true);
  }

  if (opts.outline !== undefined) outline = mountOutline(opts.outline, session, { readonly: opts.readonly, onFocusCanvas: () => box.focus() });

  const scheme = matchMedia("(prefers-color-scheme: dark)");
  const follow = (): void => {
    session.setDarkMode(scheme.matches);
    panel?.setDark(scheme.matches);
  };
  scheme.addEventListener("change", follow);

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
    drawIndicators(ctx, session.scene.drawables.values(), {
      ox: view.x / view.k,
      oy: (box.clientHeight - view.y) / view.k,
      k: view.k,
      w: box.clientWidth,
      h: box.clientHeight,
      dpr,
      dark: scheme.matches,
    });
    bar?.setZoom(Math.trunc(view.k * 100));
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
  const zoomCentre = (factor: number): void => zoomAt(factor, box.clientWidth / 2, box.clientHeight / 2);

  function resetZoom(): void {
    const b = session.scene.getBounds();
    view.k = 1;
    view.x = box.clientWidth / 2 - (b.x + b.width / 2);
    view.y = box.clientHeight / 2 + (b.y + b.height / 2);
    redraw();
  }

  // Swift ExportManager.renderImage: light scheme, margin 30, the selection only when there is one. Transparent background.
  async function renderPng(): Promise<Blob> {
    const dpr = window.devicePixelRatio || 1;
    const o = { darkMode: false, padding: 30, items: session.selection.length > 0 ? [...session.selection] : undefined, decodeImage, measureContext: ctx };
    const size = getSceneSize(session.element, o);
    const c = document.createElement("canvas");
    c.width = Math.ceil(size.width * dpr);
    c.height = Math.ceil(size.height * dpr);
    const g = c.getContext("2d")!;
    g.scale(dpr, dpr);
    renderElement(g as unknown as Canvas2D, session.element, { ...o, measureContext: undefined });
    setMeasureContext(ctx); // renderElement switched the shared measuring context to `g`
    return new Promise((resolve, reject) => c.toBlob((b) => (b === null ? reject(new Error("PNG encoding failed")) : resolve(b)), "image/png"));
  }

  const saveFile =
    opts.saveFile ??
    ((name: string, data: Blob): void => {
      const a = document.createElement("a");
      a.href = URL.createObjectURL(data);
      a.download = name;
      a.click();
      setTimeout(() => URL.revokeObjectURL(a.href), 1000);
    });
  // Swift export menu entries that exist here; HTML, JSON and PDF exports are not ported.
  const exportEntries = (): Entry[] => {
    const name = session.element.name;
    const fail = (e: unknown): void => console.warn("tenniarb: export failed", e);
    return [
      { label: "Export as PNG", icon: IMAGE_SVG, run: () => void renderPng().then((b) => saveFile(`${name}.png`, b)).catch(fail) },
      { label: "Copy as PNG", icon: IMAGE_SVG, run: () => void renderPng().then((b) => navigator.clipboard.write([new ClipboardItem({ "image/png": b })])).catch(fail) },
      "-",
      { label: "Export current to file", icon: DOC_SVG, run: () => void Promise.resolve(saveFile(`${name}.tenn`, new Blob([toTennStr(session.element)], { type: "text/plain" }))).catch(fail) },
    ];
  };

  if (opts.toolbar !== undefined) {
    const edit = opts.readonly ? {} : { add: () => (session.addNewItem(), box.focus()), remove: () => (session.deleteSelection(), box.focus()) };
    bar = mountToolbar(opts.toolbar, {
      zoomOut: () => zoomCentre(0.75),
      zoomIn: () => zoomCentre(1 / 0.75),
      resetZoom,
      help: opts.onHelp,
      share: (x, y) => showMenu(x, y, exportEntries(), () => {}),
      ...edit,
    });
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
  canvas.addEventListener("contextmenu", (ev) => {
    ev.preventDefault();
    if (opts.readonly) return;
    finishEdit(true);
    session.pick(toScene(ev));
    showMenu(ev.clientX, ev.clientY, styleEntries(session), () => box.focus());
  });
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
    zoomIn: () => zoomCentre(1 / 0.75),
    zoomOut: () => zoomCentre(0.75),
    resetZoom,
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
      scheme.removeEventListener("change", follow);
      ro.disconnect();
      cancelAnimationFrame(frame);
      box.remove();
    },
  };
}
