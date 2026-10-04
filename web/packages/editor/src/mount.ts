import { readTenn, toSyncJson, toTennStr } from "@tenniarb/core";
import type { DiagramItem, Element } from "@tenniarb/core";
import { decode, defaultFontBase, loadFonts } from "@tenniarb/embed";
import { allElements, colorWhite, cssColor, fontCss, findElement, getFontFamily, DEFAULT_FONT_FAMILY, getSceneSize, getTextColorBasedOn, parseColor, preloadImages, renderElement, setMeasureContext, SvgContext, withAlpha } from "@tenniarb/render";
import type { Canvas2D, Color, DecodedImage, ImageDecoder, Point, Rect } from "@tenniarb/render";
import { drawIndicators } from "./indicators.ts";
import { base64, interactiveHtml, pngHtml, printCss } from "./export.ts";
import { DOC_SVG, HTML_SVG, IMAGE_SVG, JSON_SVG, PDF_SVG } from "./icons.ts";
import { hitTest } from "./selection.ts";
import { mountOutline } from "./outline.ts";
import type { Outline } from "./outline.ts";
import { mountProperties } from "./properties-panel.ts";
import type { PropertiesPanel } from "./properties-panel.ts";
import { EditorSession } from "./session.ts";
import { canvasEntries, showMenu } from "./menu.ts";
import { mountQuickPanel } from "./quick-panel.ts";
import { showOperation } from "./operation.ts";
import { showSearch } from "./search.ts";
import type { Entry } from "./menu.ts";
import { defaultSettings } from "./settings.ts";
import type { EditorSettings } from "./settings.ts";
import { mountToolbar } from "./toolbar.ts";
import type { Toolbar } from "./toolbar.ts";
import type { EditMode } from "./session.ts";

const isMac = typeof navigator !== "undefined" && navigator.userAgent.includes("Mac");

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
  /** "Attach image" file picker (png / jpeg); null: cancelled. Default: a file input. */
  pickImage?: () => Promise<File | null>;
  /** URL of the embed standalone IIFE (tenniarb-embed.min.js); fetched on "Export as interactive HTML", the item is hidden without it. */
  embedBundleUrl?: string;
  /** Directory with Inter-*.woff2. */
  fontBaseUrl?: string;
  /** Preferences; missing fields take `defaultSettings`. Outline expansion is applied once, on mount. */
  settings?: Partial<EditorSettings>;
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
  /** Overlay editor for the first selected item (Swift editTitle / editBody / editValue); nothing without a selection. */
  edit(mode: EditMode): void;
  /** Swift Goto Item: popup search over the items of the current element; arrows select and centre, Enter / Esc close. */
  gotoItem(): void;
  /** Swift showOperationBox: popup under the selection applying Tenn property commands to the selected items; nothing without a selection. */
  operation(): void;
  /** Whole element as vector SVG through the system print dialog (Save as PDF there); `pdf` sizes the page to the diagram. */
  print(pdf?: boolean): void;
  /** Swift centerItem: pan so that `item` is at the view centre, `offset` scene units lower. */
  centerItem(item: DiagramItem, offset?: number): void;
  /** Client (viewport) coordinates of the centre of an item. */
  screenOf(item: DiagramItem): Point;
  text(): string;
  /** Live preferences update (background, export); the outline expansion is not redone. */
  setSettings(s: Partial<EditorSettings>): void;
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
  if (getFontFamily() === DEFAULT_FONT_FAMILY) await loadFonts(opts.fontBaseUrl ?? defaultFontBase).catch((err) => console.warn("tenniarb: font loading failed", err));

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
  let settings: EditorSettings = { ...defaultSettings, ...opts.settings };
  const scheme = matchMedia("(prefers-color-scheme: dark)");
  const darkMode = scheme.matches;
  const background = (): Color => parseColor(scheme.matches ? settings.backgroundDark : settings.background) ?? parseColor(scheme.matches ? defaultSettings.backgroundDark : defaultSettings.background)!;
  // Swift isDiagramDarkMode: the diagram goes dark when the chosen background wants white text.
  const diagramDark = (): boolean => getTextColorBasedOn(background()) === colorWhite;
  const preloaded = await decoderFor(root);
  // Images added after mount: decoded asynchronously first, because the scene decodes synchronously.
  const added = new Map<string, DecodedImage>();
  const decodeImage: ImageDecoder = (b64) => added.get(b64) ?? preloaded(b64);
  const session = new EditorSession(element, {
    darkMode: diagramDark(),
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

  if (opts.outline !== undefined) {
    outline = mountOutline(opts.outline, session, { readonly: opts.readonly, onFocusCanvas: () => box.focus(), expandLevel: settings.autoExpand ? settings.expandLevel : -1 });
  }

  // On the box, not the canvas: the off-screen indicators check relies on an otherwise transparent canvas.
  const paintBackground = (): void => void (box.style.background = cssColor(settings.transparent ? withAlpha(background(), 0.1) : background()));
  paintBackground();
  const follow = (): void => {
    paintBackground();
    session.setDarkMode(diagramDark());
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
    quickPanel(settings.quickPanel && !opts.readonly && editing === null && !active && session.selection.length === 1 ? session.selection[0]! : null, view);
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
    editing = null; // before remove(): removing a focused textarea fires blur
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
  const zoomCentre = (factor: number): void => zoomAt(factor, box.clientWidth / 2, box.clientHeight / 2);

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

  let closeSearch: (() => void) | undefined;
  function gotoItem(): void {
    closeSearch?.();
    finishEdit(true);
    closeSearch = showSearch(box, session.element, (item) => (session.select([item]), centerItem(item, 120)), () => (closeSearch = undefined, box.focus()));
  }
  let closeOperation: (() => void) | undefined;
  function operation(): void {
    if (opts.readonly || session.selection.length === 0) return;
    closeOperation?.();
    finishEdit(true);
    const bs = session.selection.map((i) => session.scene.drawables.get(i)!.getSelectorBounds());
    const [x0, x1] = [Math.min(...bs.map((b) => b.x)), Math.max(...bs.map((b) => b.x + b.width))];
    const [y0, y1] = [Math.min(...bs.map((b) => b.y)), Math.max(...bs.map((b) => b.y + b.height))];
    const r = canvas.getBoundingClientRect();
    // Scene y is up; the anchor is the client rect of the selection bounds.
    const anchor = { x: r.left + view.x + x0 * view.k, y: r.top + view.y - y1 * view.k, width: (x1 - x0) * view.k, height: (y1 - y0) * view.k };
    closeOperation = showOperation(anchor, (t) => session.operate(t), () => (closeOperation = undefined, box.focus()));
  }
  // The browser reloads on Cmd+R; the Tauri menu accelerator dispatches here itself.
  const onGoto = (ev: KeyboardEvent): void => {
    if (!(ev.metaKey || ev.ctrlKey) || ev.key.toLowerCase() !== "r" || ev.altKey || ev.shiftKey || !(box.contains(document.activeElement) || opts.outline?.contains(document.activeElement))) return;
    ev.preventDefault();
    gotoItem();
  };
  document.addEventListener("keydown", onGoto);
  // Without the Tauri menu the browser would print the whole editor UI.
  const onPrint = (ev: KeyboardEvent): void => {
    if (!(ev.metaKey || ev.ctrlKey) || ev.key.toLowerCase() !== "p" || ev.altKey || ev.shiftKey || !(box.contains(document.activeElement) || opts.outline?.contains(document.activeElement))) return;
    ev.preventDefault();
    print();
  };
  document.addEventListener("keydown", onPrint);

  function resetZoom(): void {
    view.k = 1;
    centre();
  }

  // Swift ExportManager.renderImage: light scheme, margin 30, the selection only when there is one; the fill is a fixed colour, not the preference one.
  async function renderPng(): Promise<{ blob: Blob; width: number; height: number }> {
    const dpr = settings.exportNativeScale ? window.devicePixelRatio || 1 : 1;
    const o = { darkMode: false, background: settings.exportBackground ? "#e7e9eb" : undefined, padding: 30, items: session.selection.length > 0 ? [...session.selection] : undefined, decodeImage, measureContext: ctx };
    const size = getSceneSize(session.element, o);
    const c = document.createElement("canvas");
    c.width = Math.ceil(size.width * dpr);
    c.height = Math.ceil(size.height * dpr);
    const g = c.getContext("2d")!;
    g.scale(dpr, dpr);
    renderElement(g as unknown as Canvas2D, session.element, { ...o, measureContext: undefined });
    setMeasureContext(ctx); // renderElement switched the shared measuring context to `g`
    const blob = await new Promise<Blob>((resolve, reject) => c.toBlob((b) => (b === null ? reject(new Error("PNG encoding failed")) : resolve(b)), "image/png"));
    return { blob, ...size };
  }

  // Swift exportPdf: light scheme, margin 30, no background, the whole element. Tauri on macOS replaces window.print with the dialog of the
  // whole webview, so the SVG goes into the page and print CSS hides the rest.
  function print(pdf = false): void {
    const o = { darkMode: false, padding: 30, decodeImage, measureContext: ctx };
    const { width, height } = getSceneSize(session.element, o);
    const svg = new SvgContext({ width, height, measureText: (font, t) => ((ctx.font = font), { width: ctx.measureText(t).width }) });
    renderElement(svg as unknown as Canvas2D, session.element, { ...o, measureContext: undefined });
    setMeasureContext(ctx);
    const host = document.getElementById("tn-print") ?? document.body.appendChild(Object.assign(document.createElement("div"), { id: "tn-print" }));
    host.innerHTML = `<style>${printCss(Math.ceil(width), Math.ceil(height), pdf)}</style>${svg}`;
    window.print();
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
  // Swift ExportManager.exportTypes; PDF goes through print (no PDF writer in the browser).
  const exportEntries = (): Entry[] => {
    const name = session.element.name;
    const fail = (e: unknown): void => console.warn("tenniarb: export failed", e);
    const save = (file: string, type: string, data: BlobPart): Promise<void> => Promise.resolve(saveFile(file, new Blob([data], { type }))).catch(fail);
    const png = (): Promise<Blob> => renderPng().then((r) => r.blob);
    // Swift sizes the <img> by the scene bounds; the padded image size is what the PNG really is.
    const html = async (): Promise<string> => {
      const { blob, width, height } = await renderPng();
      return pngHtml(base64(new Uint8Array(await blob.arrayBuffer())), width, height);
    };
    const copyHtml = (h: string): Promise<void> => navigator.clipboard.write([new ClipboardItem({ "text/html": new Blob([h], { type: "text/html" }), "text/plain": new Blob([h], { type: "text/plain" }) })]);
    const run = (f: () => Promise<unknown>) => (): void => void f().catch(fail);
    const interactive = opts.embedBundleUrl;
    return [
      { label: "Export as HTML", icon: HTML_SVG, run: run(async () => save(`${name}.html`, "text/html", await html())) },
      ...(interactive === undefined ? [] : [{ label: "Export as interactive HTML", icon: HTML_SVG, run: run(async () => save(`${name}.html`, "text/html", interactiveHtml(await (await fetch(interactive)).text(), session.element, session.text()))) }]),
      { label: "Export as PNG", icon: IMAGE_SVG, run: run(async () => saveFile(`${name}.png`, await png())) },
      { label: "Export as PDF", icon: PDF_SVG, run: () => print(true) },
      { label: "Export as JSON", icon: JSON_SVG, run: () => void save(`${name}.json`, "application/json", toSyncJson(session.element)) },
      "-",
      { label: "Copy as HTML", icon: HTML_SVG, run: run(async () => copyHtml(await html())) },
      { label: "Copy as PNG", icon: IMAGE_SVG, run: run(async () => navigator.clipboard.write([new ClipboardItem({ "image/png": await png() })])) },
      { label: "Copy as JSON", icon: JSON_SVG, run: run(() => navigator.clipboard.writeText(toSyncJson(session.element))) },
      "-",
      { label: "Export current to file", icon: DOC_SVG, run: () => void save(`${name}.tenn`, "text/plain", toTennStr(session.element)) },
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
  canvas.addEventListener("contextmenu", (ev) => {
    ev.preventDefault();
    // WebKit sends it for ctrl+click too; Swift's menu(for:) ignores the left button.
    if (isMac && ev.ctrlKey && ev.button === 0) return;
    if (opts.readonly) return;
    finishEdit(true);
    const p = toScene(ev);
    session.pick(p);
    const [w, h] = [box.clientWidth, box.clientHeight];
    // Swift centres the view on the result of the layout, keeping the zoom.
    const layout = (): void => (session.testLayout({ x: -w / 2, y: -h / 2, width: w, height: h }), centre());
    showMenu(ev.clientX, ev.clientY, canvasEntries(session, p, layout, () => void addImage(pickImage, true)), () => box.focus());
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
  const arrows: Record<string, [number, number]> = { ArrowLeft: [-1, 0], ArrowRight: [1, 0], ArrowUp: [0, 1], ArrowDown: [0, -1] };
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
    } else if (mod && key === "a") {
      ev.preventDefault();
      session.selectAll(ev.shiftKey ? "Item" : undefined);
    } else if (!mod && ev.key === " " && session.selection.length > 0) {
      ev.preventDefault();
      operation();
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
  // Clipboard events instead of the async API: no permission prompt. The text overlay keeps its native copy/paste.
  const clip = (ev: ClipboardEvent, text: string | null): void => {
    if (text === null) return;
    ev.clipboardData?.setData("text/plain", text);
    ev.preventDefault();
  };
  box.addEventListener("copy", (ev) => editing === null && clip(ev, session.copyText()));
  box.addEventListener("cut", (ev) => editing === null && clip(ev, session.cut()));
  box.addEventListener("paste", (ev) => {
    const file = [...(ev.clipboardData?.files ?? [])].find((f) => f.type.startsWith("image/"));
    if (editing === null && file !== undefined && !opts.readonly) {
      ev.preventDefault();
      void addImage(async () => file, false);
    } else if (editing === null && session.paste(ev.clipboardData?.getData("text/plain") ?? "")) ev.preventDefault();
  });
  const pickImage =
    opts.pickImage ??
    ((): Promise<File | null> =>
      new Promise((resolve) => {
        const input = document.createElement("input");
        input.type = "file";
        input.accept = "image/png,image/jpeg";
        input.onchange = () => resolve(input.files?.[0] ?? null);
        input.oncancel = () => resolve(null);
        input.click();
      }));
  // Swift stores PNG whatever the source was (tiffRepresentation -> png); `attach`: into the selected item, else a paste.
  async function addImage(pick: () => Promise<File | null>, attach: boolean): Promise<void> {
    try {
      const file = await pick();
      if (file === null) return;
      const bmp = await createImageBitmap(file);
      let blob: Blob = file;
      if (file.type !== "image/png") {
        const c = document.createElement("canvas");
        [c.width, c.height] = [bmp.width, bmp.height];
        c.getContext("2d")!.drawImage(bmp, 0, 0);
        blob = await new Promise<Blob>((resolve, reject) => c.toBlob((b) => (b === null ? reject(new Error("PNG encoding failed")) : resolve(b)), "image/png"));
      }
      const data = base64(new Uint8Array(await blob.arrayBuffer()));
      added.set(data, { image: bmp, width: bmp.width, height: bmp.height });
      if (attach) session.attachImage(file.name, data);
      else session.pasteImage(file.name || "image.png", data);
    } catch (e) {
      console.warn("tenniarb: image failed", e);
    }
  }
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
    gotoItem,
    operation,
    print,
    centerItem,
    edit: (mode) => session.selection[0] !== undefined && startEdit(session.selection[0], mode),
    screenOf: (item) => {
      const b = session.scene.drawables.get(item)!.getSelectorBounds();
      const r = canvas.getBoundingClientRect();
      return { x: r.left + view.x + (b.x + b.width / 2) * view.k, y: r.top + view.y - (b.y + b.height / 2) * view.k };
    },
    text: () => session.text(),
    setSettings: (s) => {
      settings = { ...settings, ...s };
      paintBackground();
      session.setDarkMode(diagramDark());
      redraw();
    },
    destroy: () => {
      panel?.destroy();
      outline?.destroy();
      scheme.removeEventListener("change", follow);
      document.removeEventListener("keydown", onGoto);
      document.removeEventListener("keydown", onPrint);
      closeSearch?.();
      closeOperation?.();
      ro.disconnect();
      cancelAnimationFrame(frame);
      box.remove();
    },
  };
}
