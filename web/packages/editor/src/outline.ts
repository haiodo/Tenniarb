// Outline panel: element tree (port of OutlineViewControllerDelegate).
import { createTree, elementTree } from "@tenniarb/embed";
import type { ElementNode, TreeView } from "@tenniarb/embed";
import type { Element } from "@tenniarb/core";
import { MINUS_SVG, PLUS_SVG } from "./icons.ts";
import { showMenu } from "./menu.ts";
import type { Entry } from "./menu.ts";
import type { EditorSession } from "./session.ts";

export interface Outline {
  /** Follow the session: tree structure, names, current element. */
  sync(): void;
  destroy(): void;
}

const signature = (list: readonly ElementNode[]): string => list.map((n) => `${n.id}:${n.name}:${n.hasItems ? 1 : 0}(${signature(n.children)})`).join(",");

function button(icon: string, title: string, click: () => void): HTMLButtonElement {
  const b = document.createElement("button");
  b.innerHTML = icon;
  b.title = title;
  b.onclick = click;
  return b;
}

export function mountOutline(host: HTMLElement, session: EditorSession, opts: { readonly?: boolean; onFocusCanvas?: () => void } = {}): Outline {
  const ro = opts.readonly === true;
  host.replaceChildren();
  const header = document.createElement("div");
  header.className = "outline-header";
  header.setAttribute("data-tauri-drag-region", "");
  const treeHost = document.createElement("div");
  treeHost.className = "outline-tree";
  treeHost.tabIndex = 0;
  host.append(header, treeHost);

  let sig = "";
  const byId = new Map<string, Element>();
  const tree: TreeView = createTree(treeHost, (n) => session.setElement(byId.get(n.id)!));

  // The current element plays the role of the selected row in Swift.
  const add = (parent?: Element): void => {
    const el = session.addElement(parent);
    if (el !== null) session.setElement(el);
  };
  if (!ro) {
    const seg = document.createElement("div");
    seg.className = "tn-seg";
    seg.append(button(PLUS_SVG, "New element inside the current one", () => add(session.element)), button(MINUS_SVG, "Delete the current element", () => void session.removeElement(session.element)));
    header.append(seg);
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

  // Swift's outline menu, plus a top-level add: the toolbar "+" only adds inside the current element.
  const rowMenu = (): Entry[] => [
    { label: "New element", run: () => add(session.element) },
    { label: "New top-level element", run: () => add() },
    { label: "Duplicate", run: () => void session.duplicateElement(session.element) },
    "-",
    { label: "Delete", run: () => void session.removeElement(session.element) },
  ];

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
      row.oncontextmenu = (ev) => {
        ev.preventDefault();
        session.setElement(el);
        showMenu(ev.clientX, ev.clientY, rowMenu(), () => treeHost.focus());
      };
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
    if (mod) {
      if (ev.key.toLowerCase() !== "z") return;
      if (ev.shiftKey) session.redo();
      else session.undo();
    } else if (ev.key === "Enter") rename(session.element.id);
    else if (ev.key === "Tab") {
      // Swift: a new item in the diagram, keyboard focus goes to the canvas
      if (ro) return;
      session.addTopItem();
      opts.onFocusCanvas?.();
    } else if (ev.key.startsWith("Arrow")) {
      const next = tree.navigate(session.element.id, ev.key as Parameters<typeof tree.navigate>[1]);
      if (next !== null) session.setElement(byId.get(next)!);
    } else return;
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
