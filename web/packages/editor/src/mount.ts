import { readTenn, toSyncJson, toTennStr } from "@tenniarb/core";
import type { DiagramItem, Element } from "@tenniarb/core";
import { decode, defaultFontBase, loadFonts } from "@tenniarb/embed";
import { allElements, colorWhite, createExecutionContext, cssColor, findElement, getFontFamily, DEFAULT_FONT_FAMILY, getSceneSize, getTextColorBasedOn, parseColor, preloadImages, renderElement, setMeasureContext, SvgContext, withAlpha } from "@tenniarb/render";
import type { Canvas2D, Color, DecodedImage, ImageDecoder, Point } from "@tenniarb/render";
import { attachCanvas } from "./canvas.ts";
import type { CanvasView } from "./canvas.ts";
import { base64, interactiveHtml, pngHtml, printCss } from "./export.ts";
import { DOC_SVG, HTML_SVG, IMAGE_SVG, JSON_SVG, PDF_SVG } from "./icons.ts";
import { mountOutline } from "./outline.ts";
import type { Outline } from "./outline.ts";
import { mountProperties } from "./properties-panel.ts";
import type { PropertiesPanel } from "./properties-panel.ts";
import { EditorSession } from "./session.ts";
import { canvasEntries, showMenu } from "./menu.ts";
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
  let cv: CanvasView | undefined;
  const session = new EditorSession(element, {
    darkMode: diagramDark(),
    exec: createExecutionContext({ decodeImage }),
    decodeImage,
    measureContext: ctx,
    readonly: opts.readonly,
    onChange: (text) => {
      opts.onChange?.(text);
      panel?.sync(true);
      outline?.sync();
    },
    onElement: () => {
      cv?.finishEdit(false);
      cv?.fit();
      outline?.sync();
    },
    onRedraw: () => {
      cv?.redraw();
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

  cv = attachCanvas(box, canvas, session, {
    dark: () => scheme.matches,
    quickPanel: () => settings.quickPanel,
    onDraw: () => bar?.setZoom(Math.trunc(view.k * 100)),
  });
  const { view, redraw, fit, zoomAt, centre, centerItem, toScene, startEdit, finishEdit, isEditing } = cv;
  const destroyCanvas = cv.destroy;
  const zoomCentre = (factor: number): void => zoomAt(factor, box.clientWidth / 2, box.clientHeight / 2);

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
  box.addEventListener("keydown", (ev) => {
    if (ev.metaKey || ev.ctrlKey || ev.key !== " " || session.selection.length === 0) return;
    ev.preventDefault();
    operation();
  });
  // Clipboard events instead of the async API: no permission prompt. The text overlay keeps its native copy/paste.
  const clip = (ev: ClipboardEvent, text: string | null): void => {
    if (text === null) return;
    ev.clipboardData?.setData("text/plain", text);
    ev.preventDefault();
  };
  box.addEventListener("copy", (ev) => !isEditing() && clip(ev, session.copyText()));
  box.addEventListener("cut", (ev) => !isEditing() && clip(ev, session.cut()));
  box.addEventListener("paste", (ev) => {
    const file = [...(ev.clipboardData?.files ?? [])].find((f) => f.type.startsWith("image/"));
    if (!isEditing() && file !== undefined && !opts.readonly) {
      ev.preventDefault();
      void addImage(async () => file, false);
    } else if (!isEditing() && session.paste(ev.clipboardData?.getData("text/plain") ?? "")) ev.preventDefault();
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
      destroyCanvas();
      box.remove();
    },
  };
}
