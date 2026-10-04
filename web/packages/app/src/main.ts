import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import { Menu, MenuItem, PredefinedMenuItem, Submenu } from "@tauri-apps/api/menu";
import { LogicalPosition } from "@tauri-apps/api/dpi";
import { Effect, getAllWindows, getCurrentWindow } from "@tauri-apps/api/window";
import { WebviewWindow } from "@tauri-apps/api/webviewWindow";
import { message, open, save } from "@tauri-apps/plugin-dialog";
import { BaseDirectory, mkdir, readTextFile, rename, writeFile, writeTextFile } from "@tauri-apps/plugin-fs";
import { mount, mountLayout } from "@tenniarb/editor";
import type { EditorHandle } from "@tenniarb/editor";
import { closeAction, docTitle, fileName, newDocumentText, pushRecent, windowLabel } from "./doc.ts";

const win = getCurrentWindow();
const filters = [{ name: "Tenniarb", extensions: ["tenn"] }];
const autosaveDelay = 2000;
const recentFile = "recent.json";
const appData = { baseDir: BaseDirectory.AppData };

// Same as tauri.macos.conf.json, which only covers the first window.
const isMac = navigator.userAgent.includes("Macintosh");
const macWindow = {
  transparent: true,
  titleBarStyle: "overlay" as const,
  hiddenTitle: true,
  trafficLightPosition: new LogicalPosition(18, 19),
  windowEffects: { effects: [Effect.LiquidGlassRegular, Effect.Sidebar] },
};

const layout = mountLayout(document.getElementById("app")!, { glass: isMac });

let handle: EditorHandle | null = null;
let path: string | null = null;
let dirty = false;
let rev = 0; // bumped on each edit: a save that raced with an edit must not clear `dirty`
let timer: ReturnType<typeof setTimeout> | undefined;

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
  if (path !== null) timer = setTimeout(() => void saveDoc(), autosaveDelay);
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
    ],
  });
  const edit = await Submenu.new({
    text: "Edit",
    items: [
      await cmd("Undo", () => undoRedo(false), "CmdOrCtrl+Z"),
      await cmd("Redo", () => undoRedo(true), "CmdOrCtrl+Shift+Z"),
      await sep(),
      await std("Cut"),
      await std("Copy"),
      await std("Paste"),
      await cmd("Delete", () => handle?.session.deleteSelection()),
      await std("SelectAll"),
    ],
  });
  const windowMenu = await Submenu.new({
    text: "Window",
    items: [await std("Minimize"), await std("Maximize"), await sep(), await std("BringAllToFront")],
  });
  const menu = await Menu.new({ items: [app, file, edit, windowMenu] });
  await (await menu.setAsAppMenu())?.close();
  await windowMenu.setAsWindowsMenuForNSApp();
}

// One document per window, so the close policy lives here; Cmd+W, the red button and Quit all end up in this handler.
void win.onCloseRequested(async (ev) => {
  const action = closeAction(path, dirty);
  if (action === "close") return;
  ev.preventDefault();
  if (action === "save" && !(await saveDoc())) return;
  if (action === "ask") {
    const answer = await message(`Do you want to save the changes made to "${docTitle(path, false)}"?`, {
      title: "Tenniarb",
      kind: "warning",
      buttons: { yes: "Save", no: "Don't Save", cancel: "Cancel" },
    });
    if (answer === "Save" ? !(await saveDoc()) : answer !== "Don't Save") return;
  }
  await win.destroy();
});

async function saveFile(name: string, data: Blob): Promise<void> {
  const p = await save({ defaultPath: name });
  if (p !== null) await writeFile(p, new Uint8Array(await data.arrayBuffer()));
}

const mountDoc = (text: string): Promise<EditorHandle> =>
  mount(layout.editor, { text, onChange, properties: layout.props, outline: layout.outline, toolbar: layout.bar, saveFile, fontBaseUrl: "fonts/" });

async function start(): Promise<void> {
  const p = new URLSearchParams(location.search).get("path");
  try {
    handle = await mountDoc(p === null ? newDocumentText : await readTextFile(p));
    path = p;
  } catch (e) {
    // Unreadable or unparsable file: fall back to an empty untitled document in this window.
    await fail(`Could not open ${p === null ? "the document" : fileName(p)}.`, e);
    handle = await mountDoc(newDocumentText);
  }
  updateTitle();
}

await start();
await buildMenu();
void win.onFocusChanged(({ payload }) => payload && void buildMenu());
void listen("open-paths", () => void drainOpened());
await drainOpened();
