import type { DiagramItem, Element } from "@tenniarb/core";
import { allElements, getString } from "@tenniarb/render";

/** Port of SceneDrawView.getBodyText: `body "text"` or `body { text "..." }`, unprocessed. */
export function bodyText(item: DiagramItem): string {
  const block = item.properties.get("body")?.getChild(1) ?? null;
  const node = block?.kind === "BlockExpr" ? (block.getNamedElement("text")?.getChild(1) ?? null) : block;
  return getString(node, new Map()) ?? "";
}

/** Port of SearchBoxViewController: items whose name or body contains `query` (case-insensitive), by name. Swift looks in one element, this looks in the whole document. */
export function searchItems(root: Element, query: string): DiagramItem[] {
  const q = query.toLowerCase();
  if (q === "") return [];
  const hits = allElements(root).flatMap((e) => e.items.filter((i) => i.name.toLowerCase().includes(q) || bodyText(i).toLowerCase().includes(q)));
  return hits.sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0));
}
