// Goto Item popup (Swift SearchBoxViewController): a search field over a result list, in a popup over the canvas.
import type { DiagramItem, Element } from "@tenniarb/core";
import { bodyText } from "./session.ts";

/** Items of `element` whose name or body contains `query` (case-insensitive), by name. Empty query matches all. */
export function searchItems(element: Element, query: string): DiagramItem[] {
  const q = query.toLowerCase();
  return element.items.filter((i) => i.name.toLowerCase().includes(q) || bodyText(i).toLowerCase().includes(q)).sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0));
}

/** Result row text: "name - body", newlines shown as \n. */
export function rowText(item: DiagramItem): string {
  const body = bodyText(item);
  return (body === "" ? item.name : `${item.name} - ${body}`).replaceAll("\n", "\\n");
}

/**
 * Popup centred over `host`, 400x180 as in Swift. Arrows move the selection (wrapping) and `onActive` fires on each change,
 * Enter and Esc close, so does a press outside, resize and blur. Returns the closer.
 */
export function showSearch(host: HTMLElement, element: Element, onActive: (item: DiagramItem) => void, onClose: () => void): () => void {
  const ac = new AbortController();
  let items: DiagramItem[] = [];
  let row = -1;
  const close = (): void => {
    if (ac.signal.aborted) return;
    ac.abort();
    root.remove();
    onClose();
  };

  const root = document.createElement("div");
  root.className = "tn-search";
  const input = document.createElement("input");
  input.placeholder = "Search...";
  input.spellcheck = false;
  const list = document.createElement("div");
  root.append(input, list);

  const select = (i: number): void => {
    list.children[row]?.classList.remove("sel");
    row = i;
    list.children[i]?.classList.add("sel");
    list.children[i]?.scrollIntoView({ block: "nearest" });
    onActive(items[i]!);
  };
  input.oninput = () => {
    items = searchItems(element, input.value);
    list.replaceChildren(...items.map((item) => Object.assign(document.createElement("div"), { className: "tn-sr", textContent: rowText(item) })));
    row = -1;
    if (items.length > 0) select(0);
  };
  list.onclick = (ev) => {
    const i = [...list.children].indexOf(ev.target as HTMLElement);
    if (i >= 0) select(i);
  };
  // Keys stay here: the canvas would take Backspace, x and the arrows.
  input.onkeydown = (ev) => {
    ev.stopPropagation();
    if (ev.key === "Escape" || ev.key === "Enter") close();
    else if (ev.key === "ArrowUp" || ev.key === "ArrowDown") {
      const n = items.length;
      if (n > 0) select(ev.key === "ArrowUp" ? (row > 0 ? row - 1 : n - 1) : (row + 1) % n);
    } else return;
    ev.preventDefault();
  };

  document.body.append(root);
  const r = host.getBoundingClientRect();
  root.style.left = `${r.left + (r.width - root.offsetWidth) / 2}px`;
  root.style.top = `${r.top + (r.height - root.offsetHeight) / 2}px`;
  input.focus();
  const opt = { signal: ac.signal };
  document.addEventListener("pointerdown", (ev) => !root.contains(ev.target as Node) && close(), { ...opt, capture: true });
  addEventListener("resize", close, opt);
  addEventListener("blur", close, opt);
  return close;
}
