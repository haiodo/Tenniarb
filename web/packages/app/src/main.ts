import { invoke } from "@tauri-apps/api/core";
import { emit, listen } from "@tauri-apps/api/event";
import { Menu, MenuItem, PredefinedMenuItem, Submenu } from "@tauri-apps/api/menu";
import { LogicalPosition, LogicalSize } from "@tauri-apps/api/dpi";
import { Effect, getAllWindows, getCurrentWindow } from "@tauri-apps/api/window";
import { WebviewWindow } from "@tauri-apps/api/webviewWindow";
import { message, open, save } from "@tauri-apps/plugin-dialog";
import { BaseDirectory, mkdir, readDir, readFile, readTextFile, remove, rename, writeFile, writeTextFile } from "@tauri-apps/plugin-fs";
import { setFontFamily } from "@tenniarb/render";
import { mount, mountLayout } from "@tenniarb/editor";
import type { EditorHandle } from "@tenniarb/editor";
import { loadSettings, settingsEvent } from "./settings-store.ts";
import type { EditorSettings } from "@tenniarb/editor/settings";
import { closeAction, docTitle, fileName, newDocumentText, parseFrames, pushRecent, untitledDir, untitledFile, untitledLabels, windowLabel } from "./doc.ts";
import type { Frame } from "./doc.ts";

const win = getCurrentWindow();
const filters = [{ name: "Tenniarb", extensions: ["tenn"] }];
const autosaveDelay = 2000;
const recentFile = "recent.json";
const framesFile = "windows.json";
const appQuit = "app-quit";
const appData = { baseDir: BaseDirectory.AppData };

// Same as tauri.macos.conf.json, which only covers the first window.
const isMac = navigator.userAgent.includes("Macintosh");
if (isMac) setFontFamily("-apple-system");
const macWindow = {
  transparent: true,
  titleBarStyle: "overlay" as const,
  hiddenTitle: true,
  trafficLightPosition: new LogicalPosition(18, 19),
  windowEffects: { effects: [Effect.UnderWindowBackground] },
};

const layout = mountLayout(document.getElementById("app")!, { glass: isMac });

let handle: EditorHandle | null = null;
let path: string | null = null;
let dirty = false;
let rev = 0; // bumped on each edit: a save that raced with an edit must not clear `dirty`
let timer: ReturnType<typeof setTimeout> | undefined;
let frameTimer: ReturnType<typeof setTimeout> | undefined;
let quitting = false; // Quit keeps an edited untitled document for the next launch instead of asking

function updateTitle(): void {
  const title = docTitle(path, dirty);
  layout.setTitle(title);
  void win.setTitle(title);
}

async function loadRecent(): Promise<string[]> {
  try {
    return JSON.parse(await readTextFile(recentFile, appData)) as string[];
  } catch {
    return [];
  }
}

async function saveRecent(list: string[]): Promise<void> {
  await mkdir("", { ...appData, recursive: true });
  await writeTextFile(recentFile, JSON.stringify(list), appData);
}

// Read-modify-write on a shared file: two windows adding at the same moment can lose one entry.
async function addRecent(p: string): Promise<void> {
  await saveRecent(pushRecent(await loadRecent(), p)).catch((e) => console.warn("recent list not saved", e));
}

const dropUntitled = (): Promise<void> => remove(untitledFile(win.label), appData).catch(() => undefined);

async function stashUntitled(): Promise<void> {
  await mkdir(untitledDir, { ...appData, recursive: true });
  await writeTextFile(untitledFile(win.label), handle!.text(), appData);
}

async function loadFrames(): Promise<Record<string, Frame>> {
  return parseFrames(await readTextFile(framesFile, appData).catch(() => ""));
}

// Swift saveWindowPosition: logical units, inner size, keyed by the document path; untitled windows are not remembered.
async function saveFrame(): Promise<void> {
  if (path === null) return;
  const f = await win.scaleFactor();
  const pos = (await win.outerPosition()).toLogical(f);
  const size = (await win.innerSize()).toLogical(f);
  const frames = await loadFrames();
  frames[path] = { x: pos.x, y: pos.y, width: size.width, height: size.height };
  await mkdir("", { ...appData, recursive: true });
  await writeTextFile(framesFile, JSON.stringify(frames), appData);
}

async function fail(what: string, e: unknown): Promise<void> {
  await message(`${what}\n${e instanceof Error ? e.message : String(e)}`, { title: "Tenniarb", kind: "error" });
}

async function writeTo(p: string): Promise<boolean> {
  clearTimeout(timer);
  const seen = rev;
  try {
    // tmp + rename, like `write(atomically: true)` in Swift
    await writeTextFile(`${p}.tmp`, handle!.text());
    await rename(`${p}.tmp`, p);
  } catch (e) {
    await fail(`Could not save ${fileName(p)}.`, e);
    return false;
  }
  if (rev === seen) dirty = false;
  if (path === null) await dropUntitled();
  path = p;
  updateTitle();
  await addRecent(p);
  return true;
}

async function saveAs(): Promise<boolean> {
  const p = await save({ filters, defaultPath: path ?? "Untitled.tenn" });
  return p !== null && writeTo(p);
}

const saveDoc = (): Promise<boolean> => (path === null ? saveAs() : writeTo(path));

function onChange(): void {
  rev++;
  dirty = true;
  updateTitle();
  clearTimeout(timer);
  timer = setTimeout(() => void (path === null ? stashUntitled() : saveDoc()).catch((e) => console.warn("autosave failed", e)), autosaveDelay);
}

async function openWindow(label: string, p: string | null): Promise<void> {
  const url = p === null ? "index.html" : `index.html?path=${encodeURIComponent(p)}`;
  const w = new WebviewWindow(label, { url, title: docTitle(p, false), width: 944, height: 764, ...(isMac ? macWindow : {}) });
  await new Promise((resolve, reject) => {
    void w.once("tauri://created", resolve);
    void w.once("tauri://error", (e) => reject(new Error(String(e.payload))));
  });
}

const newDoc = (): Promise<void> => openWindow(`untitled-${Date.now()}`, null);

async function openPath(p: string): Promise<void> {
  const existing = await WebviewWindow.getByLabel(windowLabel(p));
  if (existing !== null) return existing.setFocus();
  await openWindow(windowLabel(p), p);
  await addRecent(p);
  // As in AppKit, an untouched untitled window is replaced by the opened document.
  if (path === null && !dirty) await win.destroy();
}

async function openSettings(): Promise<void> {
  const existing = await WebviewWindow.getByLabel("settings");
  if (existing !== null) return existing.setFocus();
  new WebviewWindow("settings", { url: "settings.html", title: "Settings", width: 500, height: 224, resizable: false, minimizable: false, maximizable: false });
}

async function openHelp(): Promise<void> {
  const existing = await WebviewWindow.getByLabel("help");
  if (existing !== null) return existing.setFocus();
  new WebviewWindow("help", { url: "help/index.html", title: "Tenniarb Help", width: 800, height: 600 });
}

async function quit(): Promise<void> {
  await emit(appQuit);
  for (const w of await getAllWindows()) void w.close();
}

async function openDialog(): Promise<void> {
  const picked = await open({ filters, multiple: true });
  for (const p of picked ?? []) await openPath(p);
}

async function drainOpened(): Promise<void> {
  for (const p of await invoke<string[]>("take_opened")) await openPath(p);
}

function undoRedo(redo: boolean): void {
  // The menu accelerator may swallow the key before the page sees it: replay it on the focused element (canvas box, CodeMirror).
  const target = document.activeElement;
  if (target === null || target === document.body) return redo ? handle?.redo() : handle?.undo();
  target.dispatchEvent(new KeyboardEvent("keydown", { key: "z", metaKey: true, ctrlKey: true, shiftKey: redo, bubbles: true, cancelable: true }));
}

const textFocused = (): boolean => {
  const t = document.activeElement;
  return t instanceof HTMLInputElement || t instanceof HTMLTextAreaElement || (t instanceof HTMLElement && t.isContentEditable);
};

// Swift reads NSPasteboard; the async API may ask the webview for permission, and a failure just leaves the document alone.
function pasteAs(method: "pasteAsItem" | "pasteAsItemSet"): void {
  void navigator.clipboard.readText().then((text) => handle?.session[method](text), (e) => console.warn("clipboard is not readable", e));
}

// Revert to Saved (NSDocument): the file on disk replaces the edited text; undo history starts over.
async function revert(): Promise<void> {
  if (path === null || !dirty) return;
  const answer = await message(`Do you want to revert to the most recently saved version of "${docTitle(path, false)}"?`, {
    title: "Tenniarb",
    kind: "warning",
    buttons: { ok: "Revert", cancel: "Cancel" },
  });
  if (answer !== "Revert") return;
  try {
    const text = await readTextFile(path);
    clearTimeout(timer);
    handle?.destroy();
    handle = await mountDoc(text);
    dirty = false;
    updateTitle();
  } catch (e) {
    await fail(`Could not revert ${fileName(path)}.`, e);
  }
}

const sep = (): Promise<PredefinedMenuItem> => PredefinedMenuItem.new({ item: "Separator" });
const std = (item: NonNullable<Parameters<typeof PredefinedMenuItem.new>[0]>["item"]): Promise<PredefinedMenuItem> => PredefinedMenuItem.new({ item });
const cmd = (text: string, action: () => void, accelerator?: string): Promise<MenuItem> => MenuItem.new({ text, action, accelerator });

async function buildMenu(): Promise<void> {
  const recent = await loadRecent();
  const recentItems = await Promise.all(recent.map((p) => cmd(fileName(p), () => void openPath(p))));
  const openRecent = await Submenu.new({
    text: "Open Recent",
    items: [...recentItems, await sep(), await cmd("Clear Menu", () => void saveRecent([]).then(buildMenu))],
  });
  const app = await Submenu.new({
    text: "Tenniarb",
    items: [
      await std({ About: null }),
      await sep(),
      await cmd("Settings...", () => void openSettings(), "CmdOrCtrl+,"),
      await sep(),
      await std("Services"),
      await sep(),
      await std("Hide"),
      await std("HideOthers"),
      await std("ShowAll"),
      await sep(),
      await cmd("Quit Tenniarb", () => void getAllWindows().then((all) => all.forEach((w) => void w.close())), "CmdOrCtrl+Q"),
    ],
  });
  const file = await Submenu.new({
    text: "File",
    items: [
      await cmd("New", () => void newDoc(), "CmdOrCtrl+N"),
      await cmd("Open...", () => void openDialog(), "CmdOrCtrl+O"),
      openRecent,
      await sep(),
      await std("CloseWindow"),
      await cmd("Save", () => void saveDoc(), "CmdOrCtrl+S"),
      await cmd("Save As...", () => void saveAs(), "CmdOrCtrl+Shift+S"),
      await cmd("Revert to Saved", () => void revert()),
      await sep(),
      // No native page setup in a webview: both items open the print dialog, which has paper size and orientation.
      await cmd("Page Setup...", () => handle?.print(), "CmdOrCtrl+Shift+P"),
      await cmd("Print...", () => handle?.print(), "CmdOrCtrl+P"),
    ],
  });
  const edit = await Submenu.new({
    text: "Edit",
    items: [
      await cmd("Undo", () => undoRedo(false), "CmdOrCtrl+Z"),
      await cmd("Redo", () => undoRedo(true), "CmdOrCtrl+Shift+Z"),
      await sep(),
      // No Cmd+N on New item: File > New owns it, as in the AppKit menu where the first match wins.
      await Submenu.new({
        text: "Add",
        items: [
          await cmd("New item", () => handle?.session.addTopItem()),
          await cmd("New Linked item", () => handle?.session.addNewItem()),
          await cmd("Linked styled item", () => handle?.session.addNewItem(true), "Alt+Tab"),
        ],
      }),
      await cmd("Inherit", () => handle?.session.inherit()),
      await cmd("Delete", () => handle?.session.deleteSelection()),
      await sep(),
      await cmd("Edit value", () => handle?.edit("value"), "Alt+Enter"),
      // No accelerator: a Space shortcut would swallow typing in the webview inputs; the canvas handles Space itself.
      await cmd("Show edit box / Operation", () => handle?.operation()),
      await sep(),
      await std("Cut"),
      await std("Copy"),
      await std("Paste"),
      await cmd("Paste as Item", () => pasteAs("pasteAsItem")),
      await cmd("Paste as Item set", () => pasteAs("pasteAsItemSet")),
      await sep(),
      // The accelerator replaces the native select-all, so a text field gets it back by hand.
      await cmd("Select All", () => (textFocused() ? void document.execCommand("selectAll") : handle?.session.selectAll()), "CmdOrCtrl+A"),
      await cmd("Select All Items", () => handle?.session.selectAll("Item"), "CmdOrCtrl+Shift+A"),
      await cmd("Select All Links", () => handle?.session.selectAll("Link")),
      await cmd("Select None", () => handle?.session.select([])),
    ],
  });
  const navigate = await Submenu.new({ text: "Navigate", items: [await cmd("Goto Item", () => handle?.gotoItem(), "CmdOrCtrl+R")] });
  const view = await Submenu.new({
    text: "View",
    items: [
      await std("Fullscreen"),
      await sep(),
      await cmd("Zoom In", () => handle?.zoomIn(), "CmdOrCtrl+="),
      await cmd("Zoom Out", () => handle?.zoomOut(), "CmdOrCtrl+-"),
      await cmd("Reset Zoom", () => handle?.resetZoom(), "CmdOrCtrl+0"),
    ],
  });
  const windowMenu = await Submenu.new({
    text: "Window",
    items: [await std("Minimize"), await std("Maximize"), await sep(), await std("BringAllToFront")],
  });
  const help = await Submenu.new({ text: "Help", items: [await cmd("Tenniarb Help", () => void openHelp(), "CmdOrCtrl+Shift+/")] });
  const menu = await Menu.new({ items: [app, file, edit, navigate, view, windowMenu, help] });
  await (await menu.setAsAppMenu())?.close();
  await windowMenu.setAsWindowsMenuForNSApp();
}

// Settings and Help without a document would outlive it (Swift terminates after the last window), so they go with the last one.
async function closeAux(): Promise<void> {
  const all = await getAllWindows();
  const aux = ["settings", "help"];
  if (all.some((w) => w.label !== win.label && !aux.includes(w.label))) return;
  for (const w of all) if (aux.includes(w.label)) await w.destroy();
}

// One document per window, so the close policy lives here; Cmd+W, the red button and Quit all end up in this handler.
void win.onCloseRequested(async (ev) => {
  clearTimeout(frameTimer);
  await saveFrame().catch((e) => console.warn("window position not saved", e));
  const action = closeAction(path, dirty);
  if (action === "close") return closeAux();
  ev.preventDefault();
  if (action === "ask" && quitting) {
    await stashUntitled();
    await closeAux();
    return win.destroy();
  }
  if (action === "save" && !(await saveDoc())) return;
  if (action === "ask") {
    const answer = await message(`Do you want to save the changes made to "${docTitle(path, false)}"?`, {
      title: "Tenniarb",
      kind: "warning",
      buttons: { yes: "Save", no: "Don't Save", cancel: "Cancel" },
    });
    if (answer === "Save" ? !(await saveDoc()) : answer !== "Don't Save") return;
    clearTimeout(timer);
    await dropUntitled();
  }
  await closeAux();
  await win.destroy();
});

async function saveFile(name: string, data: Blob): Promise<void> {
  const p = await save({ defaultPath: name });
  if (p !== null) await writeFile(p, new Uint8Array(await data.arrayBuffer()));
}

async function pickImage(): Promise<File | null> {
  const p = await open({ filters: [{ name: "Image", extensions: ["png", "jpg", "jpeg"] }] });
  return p === null ? null : new File([new Uint8Array(await readFile(p))], fileName(p), { type: /\.png$/i.test(p) ? "image/png" : "image/jpeg" });
}

let settings: EditorSettings;
const mountDoc = (text: string): Promise<EditorHandle> =>
  mount(layout.editor, { text, onChange, properties: layout.props, outline: layout.outline, toolbar: layout.bar, saveFile, pickImage, onHelp: () => void openHelp(), fontBaseUrl: "fonts/", embedBundleUrl: "tenniarb-embed.min.js", settings });

async function restoreFrame(p: string): Promise<void> {
  const f = (await loadFrames())[p];
  if (f === undefined) return;
  await win.setSize(new LogicalSize(f.width, f.height));
  await win.setPosition(new LogicalPosition(f.x, f.y));
}

async function start(): Promise<void> {
  settings = await loadSettings();
  const p = new URLSearchParams(location.search).get("path");
  try {
    const stashed = p === null ? await readTextFile(untitledFile(win.label), appData).catch(() => null) : null;
    handle = await mountDoc(p === null ? (stashed ?? newDocumentText) : await readTextFile(p));
    path = p;
    dirty = stashed !== null;
    if (p !== null) await restoreFrame(p);
  } catch (e) {
    // Unreadable or unparsable file: fall back to an empty untitled document in this window.
    await fail(`Could not open ${p === null ? "the document" : fileName(p)}.`, e);
    handle = await mountDoc(newDocumentText);
  }
  updateTitle();
  const onFrame = (): void => {
    clearTimeout(frameTimer);
    frameTimer = setTimeout(() => void saveFrame().catch((e) => console.warn("window position not saved", e)), 500);
  };
  void win.onMoved(onFrame);
  void win.onResized(onFrame);
  // Only the first window of a launch reopens the untitled documents left by the previous one; they come back under their own labels.
  if (win.label !== "main") return;
  const names = await readDir(untitledDir, appData).then((es) => es.map((e) => e.name), () => []);
  for (const label of untitledLabels(names)) if (label !== "main") await openWindow(label, null);
}

await start();
await buildMenu();
void listen(appQuit, () => (quitting = true));
void win.onFocusChanged(({ payload }) => payload && void buildMenu());
void listen<EditorSettings>(settingsEvent, ({ payload }) => handle?.setSettings(payload));
void listen("open-paths", () => void drainOpened());
await drainOpened();
