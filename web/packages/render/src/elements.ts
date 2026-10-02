// Element lookup shared by the Node exporter and the browser embed.
import type { Element } from "@tenniarb/core";

/** Every descendant of `e`, parents before children. */
export function allElements(e: Element): Element[] {
  return e.elements.flatMap((c) => [c, ...allElements(c)]);
}

// Names may contain "/", so a path is matched name by name rather than split.
function byPath(parent: Element, rest: string): Element | null {
  for (const c of parent.elements) {
    if (c.name === rest) return c;
    const hit = rest.startsWith(c.name + "/") ? byPath(c, rest.slice(c.name.length + 1)) : null;
    if (hit !== null) return hit;
  }
  return null;
}

/** "A/B" path from `root`, else the first element named `spec` anywhere. */
export function findElement(root: Element, spec: string): Element | null {
  return byPath(root, spec) ?? allElements(root).find((c) => c.name === spec) ?? null;
}
