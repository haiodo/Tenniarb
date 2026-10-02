// Pure helpers (no DOM at import time) so node:test can cover them.
import type { Element } from "@tenniarb/core";
import { allElements, findElement } from "@tenniarb/render";

export interface ElementNode {
  name: string;
  /** "A/B" path accepted by setElement. */
  path: string;
  hasItems: boolean;
  children: ElementNode[];
}

export function pathOf(e: Element): string {
  const names: string[] = [];
  for (let c: Element | null = e; c !== null && c.parent !== null; c = c.parent) names.unshift(c.name);
  return names.join("/");
}

export const elementTree = (e: Element): ElementNode[] =>
  e.elements.map((c) => ({ name: c.name, path: pathOf(c), hasItems: c.items.length > 0, children: elementTree(c) }));

/** `spec` is a name or "A/B" path; without it the first element with items (else the first element). */
export function pickElement(root: Element, spec?: string): Element | null {
  if (spec !== undefined) return findElement(root, spec);
  const all = allElements(root);
  return all.find((e) => e.items.length > 0) ?? all[0] ?? null;
}

export function parseBool(value: string | null, fallback: boolean): boolean {
  if (value === null) return fallback;
  return !["false", "0", "no", "off"].includes(value.trim().toLowerCase());
}
