import { createTree, render } from "@tenniarb/embed";
import type { ElementNode, EmbedHandle } from "@tenniarb/embed";

const $ = <T extends HTMLElement>(id: string): T => document.getElementById(id) as T;
const main = $("main");
const stage = $("stage");
const tree = $("tree");

let handle: EmbedHandle | null = null;
const byPath = new Map<string, ElementNode>();
// Read by scripts/smoke.ts.
Object.defineProperty(window, "__viewer", {
  get: () => ({
    drawMs: handle?.stats.drawMs ?? 0,
    buildMs: handle?.stats.buildMs ?? 0,
    name: handle?.path ?? "",
    items: byPath.get(handle?.path ?? "")?.hasItems ? 1 : 0,
  }),
});

const hashOf = (path: string): string => "/" + path.split("/").map(encodeURIComponent).join("/");
const pathFromHash = (): string => location.hash.slice(1).split("/").filter((p) => p !== "").map(decodeURIComponent).join("/");

const treeView = createTree(tree, (node) => (location.hash = hashOf(node.path)));

function index(list: readonly ElementNode[]): void {
  for (const n of list) {
    byPath.set(n.path, n);
    index(n.children);
  }
}

function highlight(path: string): void {
  const node = byPath.get(path);
  if (node === undefined) return;
  treeView.select(node.id);
  document.title = `${path.split("/").at(-1)} - Tenniarb`;
}

async function onHash(): Promise<void> {
  const path = pathFromHash();
  if (handle === null || !byPath.has(path)) return;
  await handle.setElement(path);
  highlight(path);
}

async function load(text: string): Promise<void> {
  handle?.destroy();
  $("empty").classList.add("hidden");
  const wanted = pathFromHash();
  handle = await render(stage, text, { element: wanted === "" ? undefined : wanted, panOnWheel: true, fontBaseUrl: "fonts/" });
  const nodes = handle.elements();
  byPath.clear();
  index(nodes);
  treeView.build(nodes);
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
$("filter").addEventListener("input", (ev) => treeView.filter((ev.target as HTMLInputElement).value));
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
