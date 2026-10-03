// Outline panel: element tree (port of OutlineViewControllerDelegate) and item search (port of SearchBoxViewController).
import { createTree, elementTree, pathOf } from "@tenniarb/embed";
import type { ElementNode, TreeView } from "@tenniarb/embed";
import type { DiagramItem, Element } from "@tenniarb/core";
import { bodyText, searchItems } from "./search.ts";
import type { EditorSession } from "./session.ts";

export interface Outline {
  /** Follow the session: tree structure, names, current element. */
  sync(): void;
  destroy(): void;
}

const signature = (list: readonly ElementNode[]): string => list.map((n) => `${n.id}:${n.name}(${signature(n.children)})`).join(",");

function button(label: string, title: string, click: () => void): HTMLButtonElement {
  const b = document.createElement("button");
  b.textContent = label;
  b.title = title;
  b.onclick = click;
  return b;
}

export function mountOutline(host: HTMLElement, session: EditorSession, opts: { readonly?: boolean; onFocusCanvas?: () => void } = {}): Outline {
  const ro = opts.readonly === true;
  host.replaceChildren();
  const header = document.createElement("div");
  header.className = "outline-header";
  const input = document.createElement("input");
  input.type = "search";
  input.placeholder = "Search items";
  input.autocomplete = "off";
  const results = document.createElement("div");
  results.className = "outline-results hidden";
  const treeHost = document.createElement("div");
  treeHost.className = "outline-tree";
  treeHost.tabIndex = 0;
  host.append(header, results, treeHost);

  let sig = "";
  const byId = new Map<string, Element>();
  const tree: TreeView = createTree(treeHost, (n) => session.setElement(byId.get(n.id)!));

  // The current element plays the role of the selected row in Swift.
  const add = (parent?: Element): void => {
    const el = session.addElement(parent);
    if (el !== null) session.setElement(el);
  };
  header.append(input);
  if (!ro) {
    header.append(
      button("+", "New element inside the current one", () => add(session.element)),
      button("+ Top", "New top-level element", () => add()),
      button("Copy", "Duplicate the current element", () => void session.duplicateElement(session.element)),
      button("-", "Delete the current element", () => void session.removeElement(session.element)),
    );
  }

  function rename(id: string): void {
    const el = byId.get(id);
    const span = tree.name(id);
    if (ro || el === undefined || span === undefined) return;
    const edit = document.createElement("input");
    edit.value = el.name;
    edit.className = "outline-rename";
    let done = false;
    const finish = (commit: boolean): void => {
      if (done) return;
      done = true;
      edit.replaceWith(span);
      if (commit) session.renameElement(el, edit.value);
    };
    edit.onkeydown = (ev) => {
      ev.stopPropagation();
      if (ev.key === "Enter") finish(true);
      else if (ev.key === "Escape") finish(false);
    };
    edit.onblur = () => finish(true);
    span.replaceWith(edit);
    edit.focus();
    edit.select();
  }

  // Drop on a row moves into that element, on empty space - to the top level (Swift also reorders by index and copies into descendants).
  let dragged: Element | null = null;
  function wire(list: readonly ElementNode[]): void {
    for (const n of list) {
      wire(n.children);
      const row = tree.row(n.id)!;
      const el = byId.get(n.id)!;
      row.draggable = !ro;
      row.ondblclick = () => rename(n.id);
      if (ro) continue;
      row.ondragstart = (ev) => {
        dragged = el;
        ev.dataTransfer?.setData("text/plain", el.name);
      };
      row.ondragover = (ev) => ev.preventDefault();
      row.ondrop = (ev) => {
        ev.preventDefault();
        ev.stopPropagation();
        if (dragged !== null) session.moveElement(dragged, el);
        dragged = null;
      };
    }
  }
  treeHost.ondragover = (ev) => ev.preventDefault();
  treeHost.ondrop = (ev) => {
    ev.preventDefault();
    if (!ro && dragged !== null) session.moveElement(dragged, session.root);
    dragged = null;
  };
  treeHost.onkeydown = (ev) => {
    const mod = ev.metaKey || ev.ctrlKey;
    if (mod && ev.key.toLowerCase() === "z") ev.shiftKey ? session.redo() : session.undo();
    else if (ev.key === "Enter") rename(session.element.id);
    else return;
    ev.preventDefault();
  };

  // Search: the list replaces the tree while there is a query; arrows walk the hits and show each one, as the selection did in Swift.
  let hits: DiagramItem[] = [];
  let cur = -1;
  function show(i: number): void {
    cur = i;
    [...results.children].forEach((c, k) => c.classList.toggle("sel", k === i));
    results.children[i]?.scrollIntoView({ block: "nearest" });
    if (hits[i] !== undefined) session.reveal(hits[i]!);
  }
  function close(): void {
    input.value = "";
    hits = [];
    results.classList.add("hidden");
    treeHost.classList.remove("hidden");
    opts.onFocusCanvas?.();
  }
  input.oninput = () => {
    hits = searchItems(session.root, input.value);
    results.replaceChildren(
      ...hits.map((item, i) => {
        const row = document.createElement("div");
        row.className = "row";
        const body = bodyText(item);
        const label = (body === "" ? item.name : `${item.name} - ${body}`).replaceAll("\n", "\\n");
        const where = document.createElement("span");
        where.className = "where";
        where.textContent = pathOf(item.parent!);
        const text = document.createElement("span");
        text.className = "name";
        text.textContent = label;
        row.append(text, where);
        row.onclick = () => show(i);
        return row;
      }),
    );
    const searching = input.value !== "";
    results.classList.toggle("hidden", !searching);
    treeHost.classList.toggle("hidden", searching);
    if (hits.length > 0) show(0);
  };
  input.onkeydown = (ev) => {
    if (ev.key === "Escape" || ev.key === "Enter") close();
    else if ((ev.key === "ArrowDown" || ev.key === "ArrowUp") && hits.length > 0) show((cur + (ev.key === "ArrowDown" ? 1 : hits.length - 1)) % hits.length);
    else return;
    ev.preventDefault();
  };

  function sync(): void {
    const nodes = elementTree(session.root);
    const next = signature(nodes);
    if (next !== sig) {
      sig = next;
      byId.clear();
      const visit = (list: readonly Element[]): void => list.forEach((e) => (byId.set(e.id, e), visit(e.elements)));
      visit(session.root.elements);
      tree.build(nodes);
      wire(nodes);
    }
    tree.select(session.element.id);
  }
  sync();
  return { sync, destroy: () => host.replaceChildren() };
}
