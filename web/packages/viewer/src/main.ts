import { render } from "@tenniarb/embed";
import type { ElementNode, EmbedHandle } from "@tenniarb/embed";

const $ = <T extends HTMLElement>(id: string): T => document.getElementById(id) as T;
const main = $("main");
const stage = $("stage");
const tree = $("tree");

let handle: EmbedHandle | null = null;
let nodes: ElementNode[] = [];
const hasItems = new Map<string, boolean>();
const rows = new Map<string, { row: HTMLElement; box: HTMLElement | null; arrow: HTMLElement | null }>();
// Read by scripts/smoke.ts.
Object.defineProperty(window, "__viewer", {
  get: () => ({
    drawMs: handle?.stats.drawMs ?? 0,
    buildMs: handle?.stats.buildMs ?? 0,
    name: handle?.path ?? "",
    items: hasItems.get(handle?.path ?? "") ? 1 : 0,
  }),
});

const hashOf = (path: string): string => "/" + path.split("/").map(encodeURIComponent).join("/");
const pathFromHash = (): string => location.hash.slice(1).split("/").filter((p) => p !== "").map(decodeURIComponent).join("/");

function buildTree(parent: HTMLElement, list: readonly ElementNode[]): void {
  for (const child of list) {
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
    if (child.children.length > 0) {
      box = document.createElement("div");
      box.className = "children";
      parent.append(box);
      buildTree(box, child.children);
      arrow.textContent = "▼";
      arrow.onclick = (ev) => {
        ev.stopPropagation();
        setCollapsed(child.path, !box!.classList.contains("hidden"));
      };
    }
    row.onclick = () => (location.hash = hashOf(child.path));
    hasItems.set(child.path, child.hasItems);
    rows.set(child.path, { row, box, arrow: box ? arrow : null });
  }
}

function setCollapsed(path: string, collapsed: boolean): void {
  const r = rows.get(path)!;
  r.box?.classList.toggle("hidden", collapsed);
  if (r.arrow) r.arrow.textContent = collapsed ? "▶" : "▼";
}

function applyFilter(q: string): void {
  q = q.trim().toLowerCase();
  const visit = (list: readonly ElementNode[]): boolean => {
    let any = false;
    for (const c of list) {
      const hit = q === "" || c.name.toLowerCase().includes(q);
      const sub = visit(c.children);
      rows.get(c.path)!.row.classList.toggle("hidden", !(hit || sub));
      if (q !== "" && sub) setCollapsed(c.path, false);
      any ||= hit || sub;
    }
    return any;
  };
  visit(nodes);
}

function highlight(path: string): void {
  for (const { row } of rows.values()) row.classList.remove("sel");
  const r = rows.get(path);
  if (r === undefined) return;
  const parts = path.split("/");
  for (let i = 1; i < parts.length; i++) {
    const p = parts.slice(0, i).join("/");
    if (rows.get(p)?.box) setCollapsed(p, false);
  }
  r.row.classList.add("sel");
  r.row.scrollIntoView({ block: "nearest" });
  document.title = `${parts.at(-1)} - Tenniarb`;
}

async function onHash(): Promise<void> {
  const path = pathFromHash();
  if (handle === null || !rows.has(path)) return;
  await handle.setElement(path);
  highlight(path);
}

async function load(text: string): Promise<void> {
  handle?.destroy();
  $("empty").classList.add("hidden");
  const wanted = pathFromHash();
  handle = await render(stage, text, { element: wanted === "" ? undefined : wanted, panOnWheel: true, fontBaseUrl: "fonts/" });
  nodes = handle.elements();
  rows.clear();
  hasItems.clear();
  tree.replaceChildren();
  buildTree(tree, nodes);
  if (handle.path === "") return;
  if (pathFromHash() !== handle.path) history.replaceState(null, "", "#" + hashOf(handle.path));
  highlight(handle.path);
}

async function loadUrl(url: string): Promise<void> {
  try {
    const res = await fetch(url);
    if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
    await load(await res.text());
  } catch (err) {
    stage.textContent = `cannot load ${url}: ${String(err)}`;
  }
}

const loadFile = async (file: File): Promise<void> => load(await file.text());

addEventListener("keydown", (ev) => {
  if (ev.target instanceof HTMLInputElement || ev.metaKey || ev.ctrlKey) return;
  if (ev.key === "+" || ev.key === "=") handle?.zoomBy(1.25);
  else if (ev.key === "-") handle?.zoomBy(0.8);
  else if (ev.key === "0") handle?.reset();
});
$("zin").onclick = () => handle?.zoomBy(1.25);
$("zout").onclick = () => handle?.zoomBy(0.8);
$("z100").onclick = () => handle?.reset();
$("fit").onclick = () => handle?.fit();
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
addEventListener("hashchange", () => void onHash());

const url = new URLSearchParams(location.search).get("file");
if (url !== null) void loadUrl(url);
