// Element tree list (viewer navigator, editor outline). Classes .row (.group: has items) .arrow .name .children .sel .hidden are styled by the page; rows carry --depth.
import type { ElementNode } from "./util.ts";

export interface TreeView {
  /** Replaces the rows; collapsed state and the filter survive. */
  build(nodes: readonly ElementNode[]): void;
  /** Hides rows whose name (and whose descendants' names) do not contain `query`. */
  filter(query: string): void;
  /** Highlights the row and opens its ancestors. */
  select(id: string): void;
  /** Arrow keys of NSOutlineView: id of the row to select next, or null when the key only expands / collapses (or there is nowhere to go). */
  navigate(id: string, key: "ArrowUp" | "ArrowDown" | "ArrowLeft" | "ArrowRight"): string | null;
  row(id: string): HTMLElement | undefined;
  /** The text span of a row. */
  name(id: string): HTMLElement | undefined;
}

interface Entry {
  node: ElementNode;
  row: HTMLElement;
  name: HTMLElement;
  box: HTMLElement | null;
  arrow: HTMLElement | null;
  parent: string | null;
}

export function createTree(host: HTMLElement, onSelect: (node: ElementNode) => void): TreeView {
  let nodes: readonly ElementNode[] = [];
  let query = "";
  const rows = new Map<string, Entry>();
  const collapsed = new Set<string>();

  function setCollapsed(id: string, value: boolean): void {
    const r = rows.get(id)!;
    if (value) collapsed.add(id);
    else collapsed.delete(id);
    r.box?.classList.toggle("hidden", value);
    if (r.arrow) r.arrow.textContent = value ? "▶" : "▼";
  }

  function add(parent: HTMLElement, list: readonly ElementNode[], parentId: string | null, depth: number): void {
    for (const child of list) {
      const row = document.createElement("div");
      row.className = child.hasItems ? "row group" : "row";
      row.style.setProperty("--depth", String(depth));
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
        add(box, child.children, child.id, depth + 1);
        arrow.onclick = (ev) => {
          ev.stopPropagation();
          setCollapsed(child.id, !box!.classList.contains("hidden"));
        };
      }
      row.onclick = () => onSelect(child);
      rows.set(child.id, { node: child, row, name, box, arrow: box ? arrow : null, parent: parentId });
      if (box) setCollapsed(child.id, collapsed.has(child.id));
    }
  }

  function filter(q: string): void {
    query = q.trim().toLowerCase();
    const visit = (list: readonly ElementNode[]): void => {
      for (const c of list) {
        visit(c.children);
        const hit = query === "" || c.name.toLowerCase().includes(query);
        const sub = c.children.some((k) => !rows.get(k.id)!.row.classList.contains("hidden"));
        rows.get(c.id)!.row.classList.toggle("hidden", !(hit || sub));
        if (query !== "" && sub) setCollapsed(c.id, false);
      }
    };
    visit(nodes);
  }

  return {
    build(next) {
      nodes = next;
      rows.clear();
      host.replaceChildren();
      add(host, nodes, null, 0);
      filter(query);
    },
    filter,
    select(id) {
      for (const { row } of rows.values()) row.classList.remove("sel");
      const r = rows.get(id);
      if (r === undefined) return;
      for (let p = r.parent; p !== null; p = rows.get(p)!.parent) setCollapsed(p, false);
      r.row.classList.add("sel");
      r.row.scrollIntoView({ block: "nearest" });
    },
    navigate(id, key) {
      const r = rows.get(id);
      if (r === undefined) return null;
      if (key === "ArrowLeft") {
        if (r.box === null || collapsed.has(id)) return r.parent;
        setCollapsed(id, true);
        return null;
      }
      if (key === "ArrowRight") {
        if (r.box === null) return null;
        if (!collapsed.has(id)) return r.node.children[0]!.id;
        setCollapsed(id, false);
        return null;
      }
      const order: string[] = [];
      const walk = (list: readonly ElementNode[]): void => {
        for (const c of list) {
          if (rows.get(c.id)!.row.classList.contains("hidden")) continue;
          order.push(c.id);
          if (!collapsed.has(c.id)) walk(c.children);
        }
      };
      walk(nodes);
      return order[order.indexOf(id) + (key === "ArrowDown" ? 1 : -1)] ?? null;
    },
    row: (id) => rows.get(id)?.row,
    name: (id) => rows.get(id)?.name,
  };
}
