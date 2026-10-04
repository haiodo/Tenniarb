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

export function mountOutline(host: HTMLElement, session: EditorSession, opts: { readonly?: boolean; onFocusCanvas?: () => void; expandLevel?: number } = {}): Outline {
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

  // Swift acceptDrop: the middle of a row moves into that element, near its top / bottom edge - before / after it; empty space - the top level.
  let dragged: Element | null = null;
  const zone = (row: HTMLElement, ev: DragEvent): "before" | "in" | "after" => {
    const r = row.getBoundingClientRect();
    const y = (ev.clientY - r.top) / r.height;
    return y < 0.25 ? "before" : y > 0.75 ? "after" : "in";
  };
  const mark = (row: HTMLElement, z: string | null): void => {
    for (const c of ["before", "in", "after"]) row.classList.toggle(`drop-${c}`, c === z);
  };
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
      row.ondragover = (ev) => {
        ev.preventDefault();
        mark(row, zone(row, ev));
      };
      row.ondragleave = () => mark(row, null);
      row.ondragend = () => mark(row, null);
      row.ondrop = (ev) => {
        ev.preventDefault();
        ev.stopPropagation();
        const z = zone(row, ev);
        mark(row, null);
        if (dragged !== null) {
          const parent = z === "in" ? el : el.parent!;
          session.moveElement(dragged, parent, z === "in" ? parent.elements.length : parent.elements.indexOf(el) + (z === "after" ? 1 : 0));
        }
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
  // Clipboard events, as on the canvas: the native Edit menu items trigger them too.
  treeHost.addEventListener("copy", (ev) => {
    ev.clipboardData?.setData("text/plain", session.copyElement(session.element));
    ev.preventDefault();
  });
  treeHost.addEventListener("cut", (ev) => {
    if (ro) return;
    ev.clipboardData?.setData("text/plain", session.cutElement(session.element));
    ev.preventDefault();
  });
  treeHost.addEventListener("paste", (ev) => {
    if (!ro && session.pasteElements(session.element, ev.clipboardData?.getData("text/plain") ?? "")) ev.preventDefault();
  });
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
  if (opts.expandLevel !== undefined) {
    tree.expand(opts.expandLevel);
    tree.select(session.element.id);
  }
  return { sync, destroy: () => host.replaceChildren() };
}
