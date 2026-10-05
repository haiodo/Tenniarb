// Text protocol of the mind map block: the body of one element, read per region and written back by whole lines.
import { DiagramItem, LinkItem, ModelProperties, TennLexer, TennNode, TennParser, buildItemData, newBlockExpr, newCommand, newIdent, newIntNode, newStrNode, parseItems, storeItems, toStr } from "@tenniarb/core";
import type { Element } from "@tenniarb/core";
import type { Change } from "../mindmap.d.ts";

export type { Change };

/** Lines [start, end) of the source and the items written there; `text` is how they print as read. */
export interface Region {
  start: number;
  end: number;
  items: DiagramItem[];
  text: string;
}

export interface Block {
  items: DiagramItem[];
  /** Element-level statements (styles and the like). */
  props: TennNode[];
  regions: Region[];
  lines: string[];
  /** Skipped statements: 1-based first line in the text. */
  errors: { line: number; message: string }[];
}

const isItem = (n: TennNode): boolean => (n.getIdent(0) === "item" && n.count >= 2) || (n.getIdent(0) === "link" && n.count >= 3);

// Never throws. Broken statements, indented non-item lines at the top and links to missing items (a merge's leftovers)
// are skipped and join the region above, so its next write drops them. Duplicate properties of an item: the last wins.
export function parseBlock(text: string): Block {
  const lines = text.split("\n");
  if (lines.at(-1) === "") lines.pop();
  const starts: number[] = [];
  let off = 0;
  for (const l of lines) {
    starts.push(off);
    off += l.length + 1;
  }
  const lineAt = (pos: number): number => {
    let [lo, hi] = [0, starts.length - 1];
    while (lo < hi) {
      const mid = (lo + hi + 1) >> 1;
      if (starts[mid]! <= pos) lo = mid;
      else hi = mid - 1;
    }
    return lo;
  };

  const chunks: { start: number; end: number; broken: boolean }[] = [];
  let cur: (typeof chunks)[number] | null = null;
  let depth = 0;
  const lexer = new TennLexer(text);
  for (let tok = lexer.getToken(); tok !== null && tok.type !== "eof"; tok = lexer.getToken()) {
    if (tok.type === "semiColon") continue;
    const line = lineAt(tok.pos);
    const head = starts[line]! + /^[ \t]*/.exec(lines[line]!)![0].length;
    // At the top a line starts a statement unless it opens the block of the one above (a stray "}" is a broken one).
    // Inside a block only an unindented symbol does: canonical text indents everything nested, so the block lost its "}".
    const top = depth === 0 ? tok.type !== "curlyLe" : tok.type === "symbol" && head === starts[line];
    if ((tok.pos === head && top) || cur === null) {
      if (cur !== null && depth > 0) cur.broken = true;
      depth = 0;
      cur = { start: line, end: line + 1, broken: tok.type !== "symbol" };
      chunks.push(cur);
    }
    if (tok.type === "curlyLe") depth++;
    else if (tok.type === "curlyRi" && --depth < 0) {
      cur.broken = true;
      depth = 0;
    }
    cur.end = Math.max(cur.end, lineAt(tok.pos + tok.size) + 1);
  }
  if (cur !== null && depth > 0) cur.broken = true;

  const regions: Region[] = [];
  const props: TennNode[] = [];
  const cmds = new TennNode("Statements");
  const owners: Region[] = [];
  const errors: Block["errors"] = [];
  for (const c of chunks) {
    const parser = new TennParser();
    const tree = parser.parse(lines.slice(c.start, c.end).join("\n"));
    const all = c.broken || parser.errors.hasErrors() ? [] : (tree.children ?? []);
    if (all.length === 0) errors.push({ line: c.start + 1, message: c.broken ? "unbalanced braces" : (parser.errors.errors[0]?.message ?? "empty statement") });
    const indented = /^[ \t]/.test(lines[c.start]!);
    const own = all.filter(isItem);
    if (own.length === 0 && (indented || all.length === 0) && regions.length > 0) {
      regions.at(-1)!.end = c.end;
      continue;
    }
    const region: Region = { start: c.start, end: c.end, items: [], text: "" };
    if (!indented) props.push(...all.filter((n) => !isItem(n)));
    for (const n of own) {
      cmds.add(n);
      owners.push(region);
    }
    regions.push(region);
  }

  const items: DiagramItem[] = [];
  const dangling = new Set<Region>();
  // parseItems returns one item per item / link command, in order.
  parseItems(cmds).forEach((item, n) => {
    // A link to a missing item (renamed or deleted elsewhere) is not drawn.
    if (item instanceof LinkItem && (item.source === null || item.target === null)) return void dangling.add(owners[n]!);
    const seen = new Set<string>();
    const kept = [...item.properties].reverse().filter((p) => {
      const name = p.getIdent(0);
      if (name === null) return true;
      if (seen.has(name)) return false;
      seen.add(name);
      return true;
    });
    if (kept.length !== item.properties.count) item.properties = new ModelProperties(kept.reverse());
    owners[n]!.items.push(item);
    items.push(item);
  });
  // A region of such links only is a leftover as well.
  for (let k = regions.length - 1; k > 0; k--) {
    if (!dangling.has(regions[k]!) || regions[k]!.items.length > 0) continue;
    regions[k - 1]!.end = regions[k]!.end;
    regions.splice(k, 1);
  }
  const printed = printItems(items);
  for (const r of regions) r.text = r.items.map((i) => printed.get(i)!).join("\n");
  return { items, props, regions, lines, errors };
}

/** Canonical text of each item: a property per line, `pos` in integers and always there for items. Link indexes count over `items`. */
export function printItems(items: DiagramItem[]): Map<DiagramItem, string> {
  const nodes = storeItems(items).children ?? [];
  return new Map(
    items.map((item, n) => {
      const node = item instanceof LinkItem ? nodes[n]! : newCommand("item", newStrNode(item.name), newBlockExpr());
      if (!(item instanceof LinkItem)) buildItemData(item, node.getChild(2)!, true);
      const pos = node.getChild(node.count - 1)?.getNamedElement("pos");
      if (pos !== null && pos !== undefined) pos.children = [newIdent("pos"), newIntNode(Math.round(item.x)), newIntNode(Math.round(item.y))];
      return [item, toStr(node, 0, false)];
    }),
  );
}

/** Untouched regions keep their lines (duplicates and orphans included), changed ones are rewritten in place, new items go last. */
export function writeBlock(block: Block, items: DiagramItem[]): string {
  const printed = printItems(items);
  const out: string[] = [];
  let at = 0;
  for (const r of block.regions) {
    out.push(...block.lines.slice(at, r.start));
    const now = r.items.filter((i) => printed.has(i)).map((i) => printed.get(i)!).join("\n");
    if (now === r.text) out.push(...block.lines.slice(r.start, r.end));
    else if (now !== "") out.push(now);
    at = r.end;
  }
  out.push(...block.lines.slice(at));
  const placed = new Set(block.regions.flatMap((r) => r.items));
  for (const i of items) if (!placed.has(i)) out.push(printed.get(i)!);
  return out.length === 0 ? "" : out.join("\n") + "\n";
}

/**
 * Item identity across texts: `item:<name>#<n>`, n counting items of that name in text order (as link source-index does);
 * `link:<source key>><target key>#<n>`. A rename is a new key.
 */
export function keys(items: DiagramItem[]): string[] {
  const count = new Map<string, number>();
  const key = (base: string): string => {
    const n = count.get(base) ?? 0;
    count.set(base, n + 1);
    return `${base}#${n}`;
  };
  const named = new Map<DiagramItem, string>();
  for (const i of items) if (!(i instanceof LinkItem)) named.set(i, key(`item:${i.name}`));
  return items.map((i) => (i instanceof LinkItem ? key(`link:${named.get(i.source!)}>${named.get(i.target!)}`) : named.get(i)!));
}

/**
 * Puts `block` into `element`: an item with a live counterpart of the same key is updated in place and keeps its identity.
 * Rename: exactly one live item key gone and one new item key, at the same `pos` or the same first line of its region
 * (`prev`, the block `element` was read from) - the live item takes the new name first, so it and its links keep identity.
 */
export function reconcile(element: Element, block: Block, prev?: Block): void {
  const keyed = keys(block.items);
  const old = keys(element.items);
  const [oldSet, newSet] = [new Set(old), new Set(keyed)];
  const gone = element.items.filter((i, n) => i.kind === "Item" && !newSet.has(old[n]!));
  const added = block.items.filter((i, n) => i.kind === "Item" && !oldSet.has(keyed[n]!));
  const start = (b: Block | undefined, i: DiagramItem): number | undefined => b?.regions.find((r) => r.items.includes(i))?.start;
  if (gone.length === 1 && added.length === 1) {
    const [g, a] = [gone[0]!, added[0]!];
    if ((g.x === a.x && g.y === a.y) || start(prev, g) === start(block, a)) g.name = a.name;
  }
  const live = new Map(keys(element.items).map((k, n) => [k, element.items[n]!]));
  const swap = new Map<DiagramItem, DiagramItem>();
  block.items.forEach((p, n) => {
    const l = live.get(keyed[n]!) ?? p;
    if (l !== p) {
      [l.x, l.y, l.name, l.description, l.properties] = [p.x, p.y, p.name, p.description, p.properties];
    }
    l.parent = element;
    swap.set(p, l);
  });
  for (const [p, l] of swap) {
    if (p instanceof LinkItem && l instanceof LinkItem) [l.source, l.target] = [swap.get(p.source!)!, swap.get(p.target!)!];
  }
  block.items = block.items.map((p) => swap.get(p)!);
  for (const r of block.regions) r.items = r.items.map((p) => swap.get(p)!);
  element.items = block.items;
  element.properties = new ModelProperties(block.props);
}

/** Whole-line diff, offsets in `a`. */
export function diffLines(a: string, b: string): Change[] {
  const A = a.match(/[^\n]*\n|[^\n]+$/g) ?? [];
  const B = b.match(/[^\n]*\n|[^\n]+$/g) ?? [];
  let p = 0;
  while (p < A.length && p < B.length && A[p] === B[p]) p++;
  let s = 0;
  while (s < A.length - p && s < B.length - p && A[A.length - 1 - s] === B[B.length - 1 - s]) s++;
  const [x, y] = [A.slice(p, A.length - s), B.slice(p, B.length - s)];
  // LCS table over the changed middle: O(n * m), fine for blocks of hundreds of lines; Myers if they grow.
  const L = Array.from({ length: x.length + 1 }, () => new Uint32Array(y.length + 1));
  for (let i = x.length - 1; i >= 0; i--) {
    for (let j = y.length - 1; j >= 0; j--) L[i]![j] = x[i] === y[j] ? L[i + 1]![j + 1]! + 1 : Math.max(L[i + 1]![j]!, L[i]![j + 1]!);
  }
  const changes: Change[] = [];
  let off = A.slice(0, p).reduce((n, l) => n + l.length, 0);
  let cur: Change | null = null;
  let [i, j] = [0, 0];
  while (i < x.length || j < y.length) {
    if (i < x.length && j < y.length && x[i] === y[j]) {
      cur = null;
      off += x[i++]!.length;
      j++;
      continue;
    }
    if (cur === null) changes.push((cur = { from: off, to: off, insert: "" }));
    if (j < y.length && (i === x.length || L[i]![j + 1]! >= L[i + 1]![j]!)) cur.insert += y[j++]!;
    else {
      off += x[i++]!.length;
      cur.to = off;
    }
  }
  return changes;
}
