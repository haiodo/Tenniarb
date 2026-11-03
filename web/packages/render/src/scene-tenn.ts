// Scene renderer for the scene-graph part of .tenn (docs/spec/tenn-scene.md 4, 4.2, 4.7/4.7a, 5b, 5c, 13).
// A port of go/scene: the same expansion of use/at/replace/slots, the same style cascade, the same flex
// layout (gfx/layout) and the same text estimate, so a file draws what go/cmd/tenn-render draws.
//
// The input is the model (@tenniarb/core): an element keeps its commands, an item keeps its block, so the
// scene tree is rebuilt here instead of in the editor, the viewer and the exporter separately.
//
// Not ported yet: element --from-file (a mounted library), template, transitions, CSS @keyframes of the
// animations (the still picture is their first frame, as in Go).
import { newCommand, newFloatNode } from "@tenniarb/core";
import type { DiagramItem, Element, LinkItem, TennNode } from "@tenniarb/core";
import type { Canvas2D } from "./canvas-types.ts";
import { parseColor } from "./color.ts";
import type { Color } from "./color.ts";
import { canvasQuirks, shadowScale } from "./drawable.ts";
import type { Point, Rect } from "./geometry.ts";

// MARK: - Names and paths (spec 4.2, go/scene/names.go)

const namePart = /[\p{L}\p{M}\p{N}]/u;

/** convertNameJS: a rune that is not a letter, mark or number becomes "_". */
export function normalizeName(name: string): string {
  let out = "";
  for (const r of name) {
    out += namePart.test(r) ? r : "_";
  }
  return out;
}

/** One step of a dotted path: a bare segment names a node by its key, a quoted one by its name as written. */
export interface Segment {
  name: string;
  quoted: boolean;
}

/** A segment as a path writes it; a quote inside is written \", as the lexer reads it. */
export function quoteName(name: string): string {
  return `"${name.replaceAll('"', '\\"')}"`;
}

/** Split a dotted path; null for an empty segment, an unterminated quote or text after a quote that is no dot. */
export function splitPath(path: string): Segment[] | null {
  const segs: Segment[] = [];
  let i = 0;
  while (i < path.length) {
    if (path[i] === '"') {
      let text = "";
      let j = i + 1;
      let closed = false;
      for (; j < path.length; ) {
        if (path[j] === "\\" && path[j + 1] === '"') {
          text += '"';
          j += 2;
        } else if (path[j] === '"') {
          closed = true;
          j += 1;
          break;
        } else {
          text += path[j];
          j += 1;
        }
      }
      if (!closed || (j < path.length && path[j] !== ".")) {
        return null;
      }
      segs.push({ name: text, quoted: true });
      i = j;
      if (i === path.length) {
        break;
      }
      i += 1; // the dot
      if (i === path.length) {
        return null;
      }
      continue;
    }
    let j = path.indexOf(".", i);
    if (j < 0) {
      j = path.length;
    }
    if (j === i) {
      return null;
    }
    segs.push({ name: path.slice(i, j), quoted: false });
    i = j;
    if (i === path.length) {
      break;
    }
    i += 1;
    if (i === path.length) {
      return null;
    }
  }
  return segs.length === 0 ? null : segs;
}

const segKey = (s: Segment): string => normalizeName(s.name);

function segMatches(s: Segment, node: { name: string; key: string }): boolean {
  return s.quoted ? s.name.normalize("NFC") === node.name.normalize("NFC") : segKey(s) === node.key;
}

const argText = (a: TennNode | undefined): string => a?.getIdentText() ?? "";

/** The path a command writes from its arguments: bare words and names in quotes, joined by dots. */
function pathPrefix(args: TennNode[]): { text: string; used: number } | null {
  let best = "";
  let used = 0;
  let need = true; // a segment may start here
  let acc = "";
  for (let i = 0; i < args.length; i += 1) {
    const a = args[i]!;
    const text = a.getIdentText();
    if (text === null) {
      break;
    }
    if (a.kind === "StringLit") {
      if (!need) {
        break;
      }
      acc += quoteName(text);
      need = false;
    } else {
      let piece = "";
      let cur: boolean = need;
      let bad = false;
      const parts = text.split(".");
      for (let j = 0; j < parts.length; j += 1) {
        const part = parts[j]!;
        if (j > 0) {
          piece += ".";
          cur = true;
        }
        if (part === "") {
          continue;
        }
        if (!cur) {
          bad = true;
          break;
        }
        piece += part;
        cur = false;
      }
      if (bad) {
        break;
      }
      acc += piece;
      need = cur;
    }
    // A path that ends at a dot is not complete yet: it may still become one.
    if (splitPath(acc) !== null) {
      best = acc;
      used = i + 1;
    }
  }
  return used === 0 ? null : { text: best, used };
}

// MARK: - The file tree (spec 5a, go/scene/load.go + tree.go)

export type SceneKind = "file" | "element" | "item" | "link" | "use" | "slot";

/** A command that is not structure: a property of its node, in file order. */
export interface SceneProp {
  name: string;
  args: TennNode[];
  block: TennNode | null;
  /** The evaluated text of every argument, when the property holds an expression (spec 6). */
  val?: string[];
}

export interface TennStyle {
  props: SceneProp[];
  kids: { key: string; style: TennStyle }[];
}

export interface SceneGroup {
  name: string;
  anyOf: boolean;
  styles: string[];
}

export interface SceneParam {
  name: string;
  def: SceneProp | null;
}

export type Override =
  | { kind: "prop"; prop: SceneProp; line: number; col: number }
  | { kind: "node"; node: SceneNode; line: number; col: number }
  | { kind: "slot"; name: string; block: Override[]; line: number; col: number }
  | { kind: "at" | "replace"; path: string; block: Override[]; line: number; col: number };

/** A node of the file tree: what is written, before use is expanded. */
export interface SceneNode {
  kind: SceneKind;
  name: string;
  key: string;
  line: number;
  col: number;
  parent: SceneNode | null;
  props: SceneProp[];
  content: SceneNode[];
  elems: SceneNode[];
  styles: Map<string, TennStyle>;
  groups: SceneGroup[];
  type: string;
  params: SceneParam[];
  path: string; // use
  block: Override[]; // use
  from: string; // link
  to: string; // link
}

function newNode(kind: SceneKind, name: string, parent: SceneNode | null, at: TennNode | null): SceneNode {
  return {
    kind,
    name,
    key: normalizeName(name),
    line: at?.token?.line ?? 0,
    col: at?.token?.col ?? 0,
    parent,
    props: [],
    content: [],
    elems: [],
    styles: new Map(),
    groups: [],
    type: "",
    params: [],
    path: "",
    block: [],
    from: "",
    to: "",
  };
}

const cmdName = (c: TennNode): string => (c.kind === "Command" ? (c.getIdent(0) ?? "") : "");

function splitCommand(c: TennNode): { args: TennNode[]; block: TennNode[] } {
  const args: TennNode[] = [];
  const block: TennNode[] = [];
  for (const a of c.children?.slice(1) ?? []) {
    if (a.kind === "BlockExpr") {
      block.push(...(a.children ?? []));
    } else {
      args.push(a);
    }
  }
  return { args, block };
}

const hasBlock = (c: TennNode): boolean => {
  const ch = c.children ?? [];
  return ch.length > 0 && ch[ch.length - 1]!.kind === "BlockExpr";
};

function toProp(c: TennNode): SceneProp {
  const { args, block } = splitCommand(c);
  const last = c.children?.[c.children.length - 1];
  return { name: cmdName(c), args, block: last !== undefined && hasBlock(c) ? last : null };
}

const holdsContent = (n: SceneNode): boolean => n.kind === "file" || n.kind === "element" || n.kind === "item" || n.kind === "slot";
const isContainer = (n: SceneNode): boolean => n.kind === "file" || n.kind === "element";

/** What a path segment may pick under n: elements, items and uses. */
function named(n: SceneNode): SceneNode[] {
  const out = [...n.elems];
  for (const c of n.content) {
    if (c.kind === "item" || c.kind === "use") {
      out.push(c);
    }
  }
  return out;
}

class Diagnostics {
  readonly list: { message: string; warn: boolean; line: number; col: number }[] = [];

  fail(at: { line: number; col: number }, message: string): void {
    this.list.push({ message, warn: false, line: at.line, col: at.col });
  }

  warn(at: { line: number; col: number }, message: string): void {
    this.list.push({ message, warn: true, line: at.line, col: at.col });
  }
}

// MARK: - Parsing the model into the file tree

const pos = (n: SceneNode | TennNode): { line: number; col: number } =>
  "token" in n ? { line: n.token?.line ?? 0, col: n.token?.col ?? 0 } : { line: n.line, col: n.col };

/** The commands of a model item: pos is a property again, the rest of the block is read as written. */
function itemCommands(item: DiagramItem): TennNode[] {
  const out = [...item.properties];
  if (item.x !== 0 || item.y !== 0) {
    out.unshift(newCommand("pos", newFloatNode(item.x), newFloatNode(item.y)));
  }
  return out;
}

function fill(owner: SceneNode, commands: (TennNode | DiagramItem)[], diag: Diagnostics): void {
  for (const entry of commands) {
    if (isModelItem(entry)) {
      addModelContent(owner, entry, diag);
      continue;
    }
    const c = entry;
    if (c.kind !== "Command") {
      continue;
    }
    const name = cmdName(c);
    if (name === "") {
      continue;
    }
    if (name === "element" && isContainer(owner)) {
      addElement(owner, c, diag);
      continue;
    }
    if (addContent(owner, c, name, diag)) {
      continue;
    }
    if (name === "styles" && (isContainer(owner) || owner.kind === "item")) {
      addStyles(owner, c, diag);
      continue;
    }
    if (name === "kind") {
      const { args } = splitCommand(c);
      owner.type = argText(args[0]).toLowerCase();
      continue;
    }
    if (name === "export" && (owner.kind === "element" || owner.kind === "item")) {
      addParam(owner, c, diag);
      continue;
    }
    owner.props.push(toProp(c));
  }
}

function isModelItem(entry: TennNode | DiagramItem): entry is DiagramItem {
  return (entry as DiagramItem).properties !== undefined;
}

function addModelContent(owner: SceneNode, item: DiagramItem, diag: Diagnostics): void {
  if (!holdsContent(owner) && owner.kind !== "use") {
    return;
  }
  const node = newNode(item.kind === "Link" ? "link" : "item", item.name, owner, null);
  if (item.kind === "Link") {
    const link = item as LinkItem;
    node.from = link.source?.name ?? "";
    node.to = link.target?.name ?? "";
  }
  owner.content.push(node);
  fill(node, itemCommands(item), diag);
}

function addElement(owner: SceneNode, c: TennNode, diag: Diagnostics): void {
  const { block } = splitCommand(c);
  const node = newNode("element", argText(c.children?.[1]), owner, c);
  owner.elems.push(node);
  fill(node, block, diag);
}

function addContent(owner: SceneNode, c: TennNode, name: string, diag: Diagnostics): SceneNode | null {
  if (!holdsContent(owner) && owner.kind !== "use") {
    return null;
  }
  const { args, block } = splitCommand(c);
  let node: SceneNode | null = null;
  switch (name) {
    case "item":
      node = newNode("item", argText(c.children?.[1]), owner, c);
      fill(node, block, diag);
      break;
    case "slot":
      if (owner.kind !== "item") {
        return null;
      }
      node = newNode("slot", argText(args[0]), owner, c);
      fill(node, block, diag);
      break;
    case "link":
      node = newNode("link", "", owner, c);
      node.from = argText(args[0]);
      node.to = argText(args[1]);
      for (const b of block) {
        if (cmdName(b) !== "") {
          node.props.push(toProp(b));
        }
      }
      break;
    case "use":
      node = newUse(owner, c, args, block, diag);
      break;
    default:
      return null;
  }
  if (node !== null) {
    owner.content.push(node);
  }
  return node;
}

/** "use <path> as <name> { overrides }"; a bare use stays a property. The path may hold quoted names. */
function newUse(owner: SceneNode, c: TennNode, args: TennNode[], block: TennNode[], diag: Diagnostics): SceneNode | null {
  if (args.length === 0) {
    return null;
  }
  const at = pos(c);
  let as = -1;
  for (let i = 0; i < args.length; i += 1) {
    if (args[i]!.kind === "Ident" && argText(args[i]) === "as") {
      as = i;
      break;
    }
  }
  if (as < 1 || as + 2 !== args.length) {
    diag.fail(at, "use needs the form: use <path> as <name>");
    return null;
  }
  const path = pathPrefix(args.slice(0, as));
  if (path === null) {
    diag.fail(at, "bad path: separate segments with dots, and write a name with spaces in quotes");
    return null;
  }
  const node = newNode("use", argText(args[as + 1]), owner, c);
  node.path = path.text;
  node.block = overrides(node, block, diag);
  return node;
}

function overrides(use: SceneNode, cmds: TennNode[], diag: Diagnostics): Override[] {
  const out: Override[] = [];
  for (const c of cmds) {
    const name = cmdName(c);
    if (name === "") {
      continue;
    }
    const { args, block } = splitCommand(c);
    const at = pos(c);
    switch (name) {
      case "at":
      case "replace": {
        const path = pathPrefix(args);
        if (path === null) {
          diag.fail(at, "bad path: separate segments with dots, and write a name with spaces in quotes");
          continue;
        }
        out.push({ kind: name, path: path.text, block: overrides(use, block, diag), ...at });
        break;
      }
      case "slot":
        out.push({ kind: "slot", name: argText(args[0]), block: overrides(use, block, diag), ...at });
        break;
      default: {
        const node = addContent(use, c, name, diag);
        if (node !== null) {
          use.content.pop(); // a node of the block belongs to the instance, not to the use
          out.push({ kind: "node", node, line: at.line, col: at.col });
        } else {
          out.push({ kind: "prop", prop: toProp(c), line: at.line, col: at.col });
        }
      }
    }
  }
  return out;
}

/** "styles { name { ... } ... kind one-of a b }" (spec 5b). */
function addStyles(owner: SceneNode, c: TennNode, diag: Diagnostics): void {
  const { block } = splitCommand(c);
  for (const b of block) {
    const name = cmdName(b);
    if (name === "") {
      continue;
    }
    if (!hasBlock(b)) {
      const { args } = splitCommand(b);
      const mode = argText(args[0]);
      if (mode === "one-of" || mode === "any-of") {
        const styles: string[] = [];
        for (let i = 1; i < args.length; i += 1) {
          styles.push(normalizeName(argText(args[i])));
        }
        owner.groups.push({ name: normalizeName(name), anyOf: mode === "any-of", styles });
      }
      continue;
    }
    owner.styles.set(normalizeName(name), readStyle(b, diag));
  }
}

function readStyle(c: TennNode, diag: Diagnostics): TennStyle {
  const st: TennStyle = { props: [], kids: [] };
  const { block } = splitCommand(c);
  for (const cmd of block) {
    const name = cmdName(cmd);
    if (name === "") {
      continue;
    }
    if (hasBlock(cmd)) {
      if (name === "animation") {
        diag.warn(pos(cmd), "the keyframes block goes in the element, not in a style: write it next to styles");
        continue;
      }
      st.kids.push({ key: normalizeName(name), style: readStyle(cmd, diag) });
      continue;
    }
    st.props.push(toProp(cmd));
  }
  return st;
}

/** "export let name = value": a parameter of the component, with its default (spec 5c). */
function addParam(owner: SceneNode, c: TennNode, diag: Diagnostics): void {
  const { args } = splitCommand(c);
  const ok = (args.length === 2 || args.length === 4) && argText(args[0]) === "let" && (args.length === 2 || argText(args[2]) === "=");
  if (!ok) {
    diag.fail(pos(c), "a parameter is written: export let <name> or export let <name> = <value>");
    return;
  }
  const name = argText(args[1]);
  if (owner.params.some((p) => p.name === name)) {
    diag.warn(pos(c), `parameter "${name}" is declared twice`);
    return;
  }
  const def: SceneProp | null = args.length === 4 ? { name, args: args.slice(3), block: null } : null;
  owner.params.push({ name, def });
}

// MARK: - Resolving a path (spec 4.2/4.3, go/scene/resolve.go)

function lookup(scope: SceneNode | null, path: string): { node: SceneNode | null; message: string } {
  const segs = splitPath(path);
  if (segs === null) {
    return { node: null, message: `bad path "${path}"` };
  }
  for (let a = scope; a !== null; a = a.parent) {
    let cur = named(a).filter((n) => segMatches(segs[0]!, n));
    if (cur.length === 0) {
      continue;
    }
    let missing = "";
    for (const seg of segs.slice(1)) {
      const next: SceneNode[] = [];
      for (const h of cur) {
        next.push(...named(h).filter((n) => segMatches(seg, n)));
      }
      if (next.length === 0) {
        missing = `path ${path}: nothing named "${seg.name}" under ${cur.map((n) => quoteName(n.name)).join(", ")}`;
        break;
      }
      cur = next;
    }
    if (missing !== "") {
      return { node: null, message: missing };
    }
    if (cur.length > 1) {
      return { node: null, message: `ambiguous path ${path}: candidates ${cur.map((n) => quoteName(n.name)).join(", ")}` };
    }
    return { node: cur[0]!, message: "" };
  }
  return { node: null, message: `path ${path}: "${segs[0]!.name}" is not defined here or above` };
}

// MARK: - Expansion (go/scene/expand.go)

type RKind = "item" | "link" | "slot";

/** A node after use and slots are expanded: what is drawn. */
interface RNode {
  kind: RKind;
  name: string;
  key: string;
  props: SceneProp[];
  kids: RNode[];
  slot: string; // slot: normalized name, "" for the default one
  inst: boolean; // the root of an instance
  touched: boolean; // slot: at/replace went through it
  replaced: boolean; // slot: declared by the nodes of a replace
  from: string;
  to: string;
  vars: Record<string, string> | null;
  def: SceneNode | null; // the definition the root of an instance stands for
  src: SceneNode | null;
}

const cloneProp = (p: SceneProp): SceneProp => ({ name: p.name, args: p.args, block: p.block, val: p.val });

function rnode(kind: RKind, n: SceneNode | null, name: string, vars: Record<string, string> | null): RNode {
  return {
    kind,
    name,
    key: normalizeName(name),
    props: n === null ? [] : n.props.map(cloneProp),
    kids: [],
    slot: "",
    inst: false,
    touched: false,
    replaced: false,
    from: "",
    to: "",
    vars,
    def: null,
    src: n,
  };
}

interface SlotFill {
  name: string;
  line: number;
  col: number;
  nodes: RNode[];
  explicit: boolean;
}

class Expander {
  private readonly stack: SceneNode[] = [];
  private vars: Record<string, string> | null = null;
  private readonly diag: Diagnostics;

  constructor(diag: Diagnostics) {
    this.diag = diag;
  }

  container(n: SceneNode): RNode | null {
    if (!this.enter(n)) {
      return null;
    }
    const r = rnode("item", n, n.name, this.vars);
    r.kids = this.contents(n.content);
    this.leave();
    return r;
  }

  contents(nodes: SceneNode[]): RNode[] {
    const out: RNode[] = [];
    for (const c of nodes) {
      const r = this.content(c);
      if (r !== null) {
        out.push(r);
      }
    }
    return out;
  }

  content(n: SceneNode): RNode | null {
    switch (n.kind) {
      case "item":
        return this.item(n);
      case "use":
        return this.use(n, false);
      case "link": {
        const r = rnode("link", n, "", this.vars);
        r.from = n.from;
        r.to = n.to;
        return r;
      }
      case "slot": {
        const r = rnode("slot", n, n.name, this.vars);
        r.slot = n.key;
        r.kids = this.contents(n.content);
        return r;
      }
      default:
        return null;
    }
  }

  private item(n: SceneNode): RNode | null {
    if (!this.enter(n)) {
      return null;
    }
    const r = rnode("item", n, n.name, this.vars);
    r.kids = this.contents(n.content);
    this.leave();
    return r;
  }

  private use(n: SceneNode, asTarget: boolean): RNode | null {
    if (!this.enter(n)) {
      return null;
    }
    const found = lookup(n.parent, n.path);
    if (found.node === null) {
      this.diag.fail(n, found.message);
      this.leave();
      return null;
    }
    const target = found.node;
    const caller = this.vars;
    const { values, used } = this.params(target, n, caller);
    this.vars = values ?? caller;
    const inst = this.instance(target, n);
    this.vars = caller;
    this.leave();
    if (inst === null) {
      return null;
    }
    inst.def = target;
    inst.name = n.name;
    inst.key = n.key;
    inst.inst = true;
    inst.src = n;
    inst.props = inst.props.filter((p) => p.name !== "pos"); // the component's pos is its library place
    this.apply(inst, n.block, used);
    if (!asTarget) {
      finalize(inst);
    }
    return inst;
  }

  private instance(target: SceneNode, use: SceneNode): RNode | null {
    switch (target.kind) {
      case "item":
      case "link":
        return this.content(target);
      case "use":
        return this.use(target, true);
      case "element":
      case "file":
        return this.container(target);
      default:
        this.diag.fail(use, `path ${use.path} names a ${target.kind}, not a node`);
        return null;
    }
  }

  /** The parameters of instance target in use (spec 5c). */
  private params(target: SceneNode, use: SceneNode, caller: Record<string, string> | null): { values: Record<string, string> | null; used: Set<Override> } {
    const used = new Set<Override>();
    if (target.params.length === 0) {
      return { values: null, used };
    }
    const given = new Map<string, SceneProp>();
    for (const ov of use.block) {
      if (ov.kind === "prop" && target.params.some((p) => p.name === ov.prop.name)) {
        given.set(ov.prop.name, ov.prop);
        used.add(ov);
      }
    }
    const values: Record<string, string> = {};
    const missing: string[] = [];
    for (const prm of target.params) {
      const givenProp = given.get(prm.name);
      const prop = givenProp ?? prm.def;
      if (prop === null) {
        missing.push(prm.name);
        continue;
      }
      values[prm.name] = evalWords(prop, givenProp !== undefined ? caller : null, this.diag);
    }
    if (missing.length > 0) {
      this.diag.fail(use, `element "${target.name}" needs ${missing.map((m) => `"${m}"`).join(", ")}: set it in the block of the use`);
    }
    return { values, used };
  }

  /** The parameter values of an element expanded on its own, not through a use. */
  defaults(n: SceneNode): Record<string, string> | null {
    if (n.params.length === 0) {
      return null;
    }
    const values: Record<string, string> = {};
    for (const prm of n.params) {
      values[prm.name] = prm.def === null ? "" : evalWords(prm.def, null, this.diag);
    }
    return values;
  }

  expandElement(n: SceneNode): RNode | null {
    this.vars = this.defaults(n);
    const r = this.container(n);
    if (r !== null) {
      finalize(r);
    }
    return r;
  }

  private enter(n: SceneNode): boolean {
    if (this.stack.includes(n)) {
      this.diag.fail(n, `cycle: ${this.stack.map((s) => s.name).join(" -> ")} -> ${n.name}`);
      return false;
    }
    this.stack.push(n);
    return true;
  }

  private leave(): void {
    this.stack.pop();
  }

  /** The block of a use: properties override, nodes go to the default slot, at/replace reach in, slot fills. */
  private apply(root: RNode, block: Override[], used: Set<Override>): void {
    const fills: SlotFill[] = [];
    const add = (name: string, at: { line: number; col: number }, nodes: RNode[], explicit: boolean): void => {
      const found = fills.find((f) => f.name === name);
      if (found !== undefined) {
        found.nodes.push(...nodes);
        found.explicit = found.explicit || explicit;
        return;
      }
      fills.push({ name, line: at.line, col: at.col, nodes, explicit });
    };
    for (const ov of block) {
      switch (ov.kind) {
        case "prop":
          if (!used.has(ov)) {
            root.props.push(cloneProp(ov.prop));
          }
          break;
        case "node": {
          const r = this.content(ov.node);
          if (r !== null) {
            add("", ov, [r], false);
          }
          break;
        }
        case "slot":
          add(normalizeName(ov.name), ov, this.blockNodes(ov.block, false), true);
          break;
        case "at": {
          const at = this.find(root, ov.path, ov);
          if (at !== null) {
            this.apply(at.parent.kids[at.i]!, ov.block, new Set());
          }
          break;
        }
        case "replace": {
          const at = this.find(root, ov.path, ov);
          if (at !== null) {
            const nodes = this.blockNodes(ov.block, true);
            markReplaced(nodes);
            at.parent.kids.splice(at.i, 1, ...nodes);
          }
          break;
        }
      }
    }
    for (const f of fills) {
      this.fillSlot(root, f);
    }
  }

  /** The nodes of a slot or replace block; slots too in a replace (spec 4.7a). */
  private blockNodes(block: Override[], slots: boolean): RNode[] {
    const out: RNode[] = [];
    for (const ov of block) {
      if (ov.kind === "node") {
        const r = this.content(ov.node);
        if (r !== null) {
          out.push(r);
        }
      } else if (ov.kind === "slot" && slots) {
        out.push({
          kind: "slot",
          name: ov.name,
          key: normalizeName(ov.name),
          props: [],
          kids: this.blockNodes(ov.block, false),
          slot: normalizeName(ov.name),
          inst: false,
          touched: false,
          replaced: false,
          from: "",
          to: "",
          vars: this.vars,
          def: null,
          src: null,
        });
      } else {
        this.diag.fail(ov, "only item, link and use are allowed here");
      }
    }
    return out;
  }

  /** Follow a path of names from the instance root: items, instances and slots, no links. */
  private find(root: RNode, path: string, at: { line: number; col: number }): { parent: RNode; i: number } | null {
    const segs = splitPath(path);
    if (segs === null) {
      this.diag.fail(at, `bad path "${path}"`);
      return null;
    }
    let nodes = [root];
    let locs: { parent: RNode; i: number }[] = [];
    for (const seg of segs) {
      locs = [];
      for (const p of nodes) {
        for (let i = 0; i < p.kids.length; i += 1) {
          if (matchesSeg(p.kids[i]!, seg)) {
            locs.push({ parent: p, i });
          }
        }
      }
      if (locs.length === 0) {
        this.diag.fail(at, `path ${path}: nothing named "${seg.name}" inside the instance`);
        return null;
      }
      nodes = locs.map((l) => l.parent.kids[l.i]!);
      for (const n of nodes) {
        if (n.kind === "slot") {
          n.touched = true;
        }
      }
    }
    if (nodes.length > 1) {
      this.diag.fail(at, `ambiguous path ${path}: candidates ${nodes.map((n) => quoteName(n.name)).join(", ")}`);
      return null;
    }
    if (nodes[0]!.kind === "slot") {
      this.diag.fail(at, `path ${path} names a slot, not a node; fill it with slot ${segs[segs.length - 1]!.name} { ... }`);
      return null;
    }
    return locs[0]!;
  }

  private fillSlot(root: RNode, f: SlotFill): void {
    if (f.name === "" && f.nodes.length === 0 && !f.explicit) {
      return;
    }
    let at = findSlot(root, f.name);
    if (at === null && f.name !== "") {
      const hits = forwardedSlots(root, f.name);
      if (hits.length > 1) {
        this.diag.fail(f, `slot "${f.name}" is in ${hits.length} nested components: ${hits.map((h) => h.path).join(", ")}; give the wrapper a slot of its own`);
        return;
      }
      at = hits.length === 1 ? hits[0]!.loc : null;
    }
    if (at !== null) {
      const slot = at.parent.kids[at.i]!;
      if (slot.touched) {
        this.diag.warn(f, `slot ${slotSegment(slot)} is filled here, the at/replace changes to its fallback are dropped`);
      }
      at.parent.kids.splice(at.i, 1, ...f.nodes);
      return;
    }
    const have = slotNames(root).join(", ") || "none";
    if (f.name === "") {
      this.diag.fail(f, `nodes passed, but the component has no default slot (slots: ${have})`);
      return;
    }
    this.diag.fail(f, `unknown slot "${f.name}" (slots: ${have})`);
  }
}

function matchesSeg(k: RNode, seg: Segment): boolean {
  if (k.kind === "item") {
    return seg.quoted ? seg.name.normalize("NFC") === k.name.normalize("NFC") : k.key === segKey(seg);
  }
  if (k.kind === "slot") {
    return seg.quoted ? normalizeName(seg.name) === k.slot : slotSegment(k) === segKey(seg);
  }
  return false;
}

const slotSegment = (k: RNode): string => (k.slot === "" ? "default" : k.slot);

const nodeSegment = (n: RNode): string => (n.name !== "" ? n.name : n.key);

function markReplaced(nodes: RNode[]): void {
  for (const n of nodes) {
    if (n.kind === "slot") {
      n.replaced = true;
    }
    markReplaced(n.kids);
  }
}

function finalize(r: RNode): void {
  const kids: RNode[] = [];
  for (const k of r.kids) {
    finalize(k);
    if (k.kind === "slot") {
      kids.push(...k.kids);
    } else {
      kids.push(k);
    }
  }
  r.kids = kids;
}

function findSlotIn(r: RNode, name: string, replaced: boolean): { parent: RNode; i: number } | null {
  for (let i = 0; i < r.kids.length; i += 1) {
    const k = r.kids[i]!;
    if (k.kind === "slot" && k.slot === name && k.replaced === replaced) {
      return { parent: r, i };
    }
    if (k.inst) {
      continue;
    }
    const found = findSlotIn(k, name, replaced);
    if (found !== null) {
      return found;
    }
  }
  return null;
}

function findSlot(r: RNode, name: string): { parent: RNode; i: number } | null {
  return findSlotIn(r, name, true) ?? findSlotIn(r, name, false);
}

function joinPath(prefix: string, seg: string): string {
  if (prefix === "") {
    return seg;
  }
  if (seg === "") {
    return prefix;
  }
  return `${prefix}.${seg}`;
}

/** The slots named name that live in the components r uses, in file order: a wrapper passes them through. */
function forwardedSlots(r: RNode, name: string): { loc: { parent: RNode; i: number }; path: string }[] {
  const collect = (n: RNode, prefix: string, inInstance: boolean, replaced: boolean, out: { loc: { parent: RNode; i: number }; path: string }[]): void => {
    for (let i = 0; i < n.kids.length; i += 1) {
      const k = n.kids[i]!;
      const p = joinPath(prefix, nodeSegment(k));
      if (k.kind === "slot") {
        if (inInstance && k.slot === name && k.replaced === replaced) {
          out.push({ loc: { parent: n, i }, path: joinPath(p, slotSegment(k)) });
        }
      } else if (k.inst) {
        collect(k, p, true, replaced, out);
      } else {
        collect(k, p, inInstance, replaced, out);
      }
    }
  };
  for (const replaced of [true, false]) {
    const out: { loc: { parent: RNode; i: number }; path: string }[] = [];
    collect(r, "", false, replaced, out);
    if (out.length > 0) {
      return out;
    }
  }
  return [];
}

function slotNames(r: RNode, out: string[] = []): string[] {
  for (const k of r.kids) {
    if (k.inst) {
      continue;
    }
    if (k.kind === "slot") {
      out.push(k.slot === "" ? "(default)" : k.slot);
    }
    slotNames(k, out);
  }
  return out;
}

// MARK: - Expressions (spec 6, 5c)

const plainNumber = /^-?\d+(\.\d+)?$/;

/** Go's exec.SceneEvalVars: a value that only looks like a number stays text, the rest are JS values. */
export function evalCode(code: string, vars: Record<string, string> | null): string | null {
  const names = Object.keys(vars ?? {});
  const values = names.map((n) => {
    const v = vars![n]!;
    return plainNumber.test(v) && String(Number(v)) === v ? Number(v) : v;
  });
  try {
    // shortcut: new Function is the whole evaluator; a page with a CSP that forbids it gets no expressions
    // (a value check still draws). Swap in the shared ExecutionContext when scenes need its names.
    const fn = new Function(...names, `"use strict"; return (${code});`);
    return String(fn(...values));
  } catch {
    return null;
  }
}

/** The text of an argument, or the value of an expression; a string holding ${...} is a template. */
function evalArg(a: TennNode, vars: Record<string, string> | null): string | null {
  const text = a.getIdentText() ?? "";
  let code: string;
  if (a.kind === "Expression" || a.kind === "ExpressionBlock") {
    code = text;
  } else if (a.kind === "StringLit" && text.includes("${")) {
    code = `\`${text}\``;
  } else {
    return text;
  }
  return evalCode(code, vars);
}

const propText = (p: SceneProp, i: number): string => (p.val !== undefined ? (p.val[i] ?? "") : argText(p.args[i]));

const numberRe = /^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$/;

function propNum(p: SceneProp, i: number): number | null {
  if (p.val !== undefined) {
    const v = p.val[i];
    return v === undefined || !numberRe.test(v) ? null : Number(v);
  }
  const a = p.args[i];
  if (a === undefined || (a.kind !== "IntLit" && a.kind !== "FloatLit")) {
    return null;
  }
  const v = a.getIdentText();
  return v === null || !numberRe.test(v) ? null : Number(v);
}

/** The arguments of a property as words, each an expression evaluated in vars. */
function evalWords(p: SceneProp, vars: Record<string, string> | null, diag: Diagnostics): string {
  const words: string[] = [];
  for (const a of p.args) {
    const v = evalArg(a, vars);
    if (v === null) {
      diag.warn(p as unknown as SceneNode, `${p.name}: the expression is not evaluated`);
      return words.join(" ");
    }
    words.push(v);
  }
  return words.join(" ");
}

// MARK: - Styles (spec 5b, go/scene/styles.go)

const inheritFont = ["font-family", "font-size", "font-weight", "font-style", "letter-spacing"];

/** The styles a node names with use-style, in order; "-name" drops the one named before. */
export function styleRefs(props: SceneProp[]): string[] {
  let names: string[] = [];
  for (const p of props) {
    if (p.name !== "use-style") {
      continue;
    }
    for (let i = 0; i < p.args.length; i += 1) {
      const word = propText(p, i);
      for (const w of word.split(/\s+/).filter((s) => s !== "")) {
        if (w.startsWith("-")) {
          const key = normalizeName(w.slice(1));
          names = names.filter((n) => n !== key);
        } else {
          names.push(normalizeName(w));
        }
      }
    }
  }
  return names;
}

/** The node whose styles a node may name: itself with a styles block, else the nearest element above. */
function styleOwner(n: SceneNode): SceneNode | null {
  if (n.styles.size > 0 || n.groups.length > 0) {
    return n;
  }
  for (let p = n.parent; p !== null; p = p.parent) {
    if (p.styles.size > 0 || p.groups.length > 0) {
      return p;
    }
  }
  return null;
}

function afterUseStyle(props: SceneProp[], add: SceneProp[]): SceneProp[] {
  if (add.length === 0) {
    return props;
  }
  let at = 0;
  props.forEach((p, i) => {
    if (p.name === "use-style") {
      at = i + 1;
    }
  });
  return [...props.slice(0, at), ...add, ...props.slice(at)];
}

function applyKidStyles(r: RNode, st: TennStyle): void {
  for (const sk of st.kids) {
    for (const k of r.kids) {
      if (k.key === sk.key) {
        k.props = [...sk.style.props, ...k.props];
        applyKidStyles(k, sk.style);
      }
    }
  }
}

function inheritProps(k: RNode, parent: RNode): void {
  if (k.kind !== "item") {
    return;
  }
  for (const name of inheritFont) {
    if (prop(k, name) !== null) {
      continue;
    }
    const p = prop(parent, name);
    if (p !== null) {
      k.props.unshift(p);
    }
  }
}

/** The last property of that name, or null. */
function prop(r: RNode, name: string): SceneProp | null {
  for (let i = r.props.length - 1; i >= 0; i -= 1) {
    if (r.props[i]!.name === name) {
      return r.props[i]!;
    }
  }
  return null;
}

/** Give every node the properties of the styles it names, in the place its use-style is written. */
function applyStyles(r: RNode): void {
  if (r.src !== null) {
    const def = r.def ?? r.src;
    const owner = styleOwner(def);
    const names = styleRefs(r.props);
    if (owner !== null) {
      const added: SceneProp[] = [];
      for (const name of names) {
        const st = owner.styles.get(name);
        if (st !== undefined) {
          added.push(...st.props);
        }
      }
      r.props = afterUseStyle(r.props, added);
      // Kid blocks prepend, so the styles go in reverse: the one named last stays nearest the end.
      for (let i = names.length - 1; i >= 0; i -= 1) {
        const st = owner.styles.get(names[i]!);
        if (st !== undefined) {
          applyKidStyles(r, st);
        }
      }
    }
  }
  for (const k of r.kids) {
    inheritProps(k, r);
    applyStyles(k);
  }
}

// MARK: - Animations (spec 13, go/scene/anim.go)

const animatedProps = new Set(["rotate", "scale", "opacity", "color", "border-color", "text-color", "pos"]);
const layoutProps = new Set([
  "width",
  "height",
  "min-width",
  "max-width",
  "min-height",
  "max-height",
  "gap",
  "line-gap",
  "padding",
  "auto-layout",
  "wrap",
  "align",
  "justify",
  "absolute",
]);
const easings = new Set(["linear", "ease", "ease-in", "ease-out", "ease-in-out"]);

interface Frame {
  percent: string;
  offset: number;
  props: SceneProp[];
}

interface Animation {
  name: string;
  frames: Frame[];
}

function frameOffset(sel: string): number | null {
  if (sel === "from") {
    return 0;
  }
  if (sel === "to") {
    return 1;
  }
  if (!sel.endsWith("%")) {
    return null;
  }
  const v = Number(sel.slice(0, -1));
  return Number.isFinite(v) && v >= 0 && v <= 100 ? v / 100 : null;
}

const parseDuration = (s: string): number | null => {
  const text = s.trim().toLowerCase();
  let unit = 1000;
  let num = text;
  if (text.endsWith("ms")) {
    unit = 1;
    num = text.slice(0, -2);
  } else if (text.endsWith("s")) {
    num = text.slice(0, -1);
  } else {
    return null;
  }
  const v = Number(num);
  return num !== "" && Number.isFinite(v) && v >= 0 ? v * unit : null;
};

/** The frames of "animation <name> { <frame> { properties } ... }", sorted by offset. */
function keyframes(p: SceneProp, diag: Diagnostics): Animation | null {
  const name = propText(p, 0);
  if (name === "") {
    diag.warn(p as unknown as SceneNode, "animation: the block needs a name, as in animation spin { from { ... } to { ... } }");
    return null;
  }
  const a: Animation = { name, frames: [] };
  for (const c of p.block?.children ?? []) {
    if (c.kind !== "Command") {
      continue;
    }
    const sel = cmdName(c);
    const off = frameOffset(sel);
    if (off === null) {
      diag.warn(pos(c), `animation ${name}: frame "${sel}" is not from, to or a percentage`);
      continue;
    }
    const f: Frame = { percent: sel, offset: off, props: [] };
    for (const pc of splitCommand(c).block) {
      const pn = cmdName(pc);
      if (pn === "") {
        continue;
      }
      if (animatedProps.has(pn)) {
        f.props.push(toProp(pc));
      } else if (layoutProps.has(pn)) {
        diag.warn(pos(pc), `animation ${name}: ${pn} is a layout property and is not animated, the frame is ignored`);
      } else {
        diag.warn(pos(pc), `animation ${name}: ${pn} is not animated: rotate, scale, opacity, color, border-color, text-color or pos`);
      }
    }
    a.frames.push(f);
  }
  if (a.frames.length === 0) {
    diag.warn(p as unknown as SceneNode, `animation ${name}: the block has no frames (from { ... } to { ... })`);
    return null;
  }
  a.frames.sort((x, y) => x.offset - y.offset);
  return a;
}

/** The first frame of the animation a node plays is part of the still picture, as in Go. */
// shortcut: an interactive SVG export would need the @keyframes CSS and an id per node (go/scene maps them
// by group order); a still picture and a canvas view take the first frame, so that is all this builds.
function animations(rn: RNode, defs: Map<string, Animation>, diag: Diagnostics): void {
  const local = new Map(defs);
  const declared = new Set<string>();
  for (const p of rn.props) {
    if (p.name !== "animation" || p.block === null) {
      continue;
    }
    const a = keyframes(p, diag);
    if (a === null) {
      continue;
    }
    if (declared.has(a.name)) {
      diag.warn(p as unknown as SceneNode, `animation ${a.name}: the frames are declared twice, the last one is used`);
    }
    declared.add(a.name);
    local.set(a.name, a);
  }
  for (const p of rn.props) {
    if (p.name !== "animation" || p.block !== null) {
      continue;
    }
    const name = propText(p, 0);
    if (name === "") {
      diag.warn(p as unknown as SceneNode, "animation takes a name and a duration: animation spin 1s linear infinite");
      continue;
    }
    const def = local.get(name);
    if (def === undefined) {
      diag.warn(p as unknown as SceneNode, `animation ${name}: no keyframes block with that name in the element`);
      continue;
    }
    if (p.args.length < 2) {
      diag.warn(p as unknown as SceneNode, `animation ${name}: the duration is missing, as in animation ${name} 1s linear infinite`);
      continue;
    }
    const word = propText(p, 1);
    const dur = parseDuration(word);
    if (dur === null) {
      diag.warn(p as unknown as SceneNode, `animation ${name}: "${word}" is not a duration, as in 300ms or 1s`);
      continue;
    }
    if (dur <= 0) {
      diag.warn(p as unknown as SceneNode, `animation ${name}: the duration must be above zero`);
      continue;
    }
    rn.props.push(...def.frames[0]!.props.map(cloneProp));
  }
  for (const k of rn.kids) {
    animations(k, local, diag);
  }
}

// MARK: - Layout (port of gfx/layout and go/scene/layout.go)

type Dir = "row" | "column" | "abs";
type Align = "start" | "center" | "end" | "space-between";
interface Size {
  kind: "hug" | "fixed" | "fill";
  v: number;
}

const HUG: Size = { kind: "hug", v: 0 };
const fixed = (v: number): Size => ({ kind: "fixed", v });
const FILL: Size = { kind: "fill", v: 0 };

interface LayoutNode {
  w: Size;
  h: Size;
  minW: number;
  maxW: number;
  minH: number;
  maxH: number;
  x: number;
  y: number;
  absolute: boolean;
  dir: Dir;
  gap: number;
  pad: [number, number, number, number];
  align: Align;
  justify: Align;
  wrap: boolean;
  lineGap: number;
  kids: LayoutNode[];
  leafW: (() => number) | null;
  leafH: ((w: number) => number) | null;
  rx: number;
  ry: number;
  rw: number;
  rh: number;
  iw: number;
  iwOK: boolean;
  tag: RNode | null;
}

function newLayout(tag: RNode | null): LayoutNode {
  return {
    w: HUG,
    h: HUG,
    minW: 0,
    maxW: 0,
    minH: 0,
    maxH: 0,
    x: 0,
    y: 0,
    absolute: false,
    dir: "row",
    gap: 0,
    pad: [0, 0, 0, 0],
    align: "start",
    justify: "start",
    wrap: false,
    lineGap: 0,
    kids: [],
    leafW: null,
    leafH: null,
    rx: 0,
    ry: 0,
    rw: 0,
    rh: 0,
    iw: 0,
    iwOK: false,
    tag,
  };
}

const clamp = (v: number, lo: number, hi: number): number => {
  let out = v;
  if (hi > 0 && out > hi) {
    out = hi;
  }
  if (out < lo) {
    out = lo;
  }
  return out;
};

const clampW = (n: LayoutNode, w: number): number => clamp(w, n.minW, n.maxW);
const clampH = (n: LayoutNode, h: number): number => clamp(h, n.minH, n.maxH);

function flow(n: LayoutNode): LayoutNode[] {
  return n.kids.some((k) => k.absolute) ? n.kids.filter((k) => !k.absolute) : n.kids;
}

const gapsOf = (gap: number, n: number): number => gap * Math.max(n - 1, 0);

function forget(n: LayoutNode): void {
  n.iwOK = false;
  for (const k of n.kids) {
    forget(k);
  }
}

function intrinsicW(n: LayoutNode): number {
  if (n.w.kind === "fixed") {
    return clampW(n, n.w.v);
  }
  if (n.iwOK) {
    return n.iw;
  }
  let w = 0;
  const kids = n.dir !== "abs" ? flow(n) : n.kids;
  if (n.kids.length === 0) {
    w = n.leafW !== null ? n.leafW() : 0;
  } else if (n.dir === "abs") {
    for (const k of kids) {
      w = Math.max(w, k.x + intrinsicW(k));
    }
  } else if (n.dir === "column" && n.wrap && n.h.kind === "fixed") {
    w = wrappedColumnsW(n, kids);
  } else if (n.dir === "row") {
    w = n.pad[1] + n.pad[3] + gapsOf(n.gap, kids.length);
    for (const k of kids) {
      w += intrinsicW(k);
    }
  } else {
    for (const k of kids) {
      w = Math.max(w, intrinsicW(k));
    }
    w += n.pad[1] + n.pad[3];
  }
  w = clampW(n, w);
  n.iw = w;
  n.iwOK = true;
  return w;
}

function wrappedColumnsW(n: LayoutNode, kids: LayoutNode[]): number {
  const avail = clampH(n, n.h.v) - n.pad[0] - n.pad[2];
  let w = 0;
  let colW = 0;
  let colH = 0;
  let cols = 0;
  kids.forEach((k, i) => {
    layout(k, intrinsicW(k), k.h.kind === "fixed" ? k.h.v : -1);
    if (i > 0 && colH + n.gap + k.rh > avail + 1e-9) {
      w += colW;
      colW = 0;
      colH = 0;
      cols += 1;
    } else if (i > 0) {
      colH += n.gap;
    }
    colW = Math.max(colW, k.rw);
    colH += k.rh;
  });
  if (kids.length > 0) {
    w += colW;
    cols += 1;
  }
  return w + n.lineGap * Math.max(cols - 1, 0) + n.pad[1] + n.pad[3];
}

const kidW = (n: LayoutNode, avail: number): number => {
  if (n.w.kind === "fixed") {
    return clampW(n, n.w.v);
  }
  if (n.w.kind === "fill") {
    return clampW(n, avail);
  }
  return intrinsicW(n);
};

function layout(n: LayoutNode, w: number, h: number): void {
  w = clampW(n, w);
  if (h >= 0) {
    h = clampH(n, h);
  }
  n.rw = w;
  if (n.kids.length === 0) {
    n.rh = Math.max(h, 0);
    if (n.leafH !== null) {
      if (h < 0) {
        n.rh = clampH(n, n.leafH(w));
      } else {
        n.leafH(w); // the leaf still wraps at w
      }
    }
  } else if (n.dir === "abs") {
    layoutAbs(n, w, h);
  } else {
    layoutFlow(n, w, h);
    layoutAbsolute(n);
  }
}

function layoutAbs(n: LayoutNode, w: number, h: number): void {
  let maxB = 0;
  for (const k of n.kids) {
    const kh = k.h.kind === "fixed" ? k.h.v : k.h.kind === "fill" ? h : -1;
    layout(k, kidW(k, w), kh);
    k.rx = k.x;
    k.ry = k.y;
    maxB = Math.max(maxB, k.ry + k.rh);
  }
  n.rh = h;
  if (h < 0) {
    n.rh = clampH(n, maxB);
  }
}

function layoutAbsolute(n: LayoutNode): void {
  for (const k of n.kids) {
    if (!k.absolute) {
      continue;
    }
    const kh = k.h.kind === "fixed" ? k.h.v : k.h.kind === "fill" ? n.rh : -1;
    layout(k, kidW(k, n.rw), kh);
    k.rx = k.x;
    k.ry = k.y;
  }
}

function hugH(n: LayoutNode, content: number): { h: number; inner: number } {
  const h = clampH(n, content + n.pad[0] + n.pad[2]);
  const inner = h === content + n.pad[0] + n.pad[2] ? content : h - n.pad[0] - n.pad[2];
  return { h, inner };
}

function layoutFlow(n: LayoutNode, w: number, h: number): void {
  const kids = flow(n);
  if (n.wrap) {
    layoutWrap(n, kids, w, h);
    return;
  }
  const [pt, pr, pb, pl] = n.pad;
  const innerW = w - pl - pr;
  const gaps = gapsOf(n.gap, kids.length);
  let innerH = h - pt - pb;
  if (h < 0) {
    innerH = -1;
  }
  if (n.dir === "row") {
    let fixedSum = 0;
    let nfill = 0;
    for (const k of kids) {
      if (k.w.kind === "fill") {
        nfill += 1;
      } else {
        fixedSum += kidW(k, 0);
      }
    }
    let share = 0;
    if (nfill > 0) {
      share = Math.max((innerW - fixedSum - gaps) / nfill, 0);
    }
    let contentH = 0;
    for (const k of kids) {
      const kh = k.h.kind === "fixed" ? k.h.v : k.h.kind === "fill" ? innerH : -1;
      layout(k, kidW(k, share), kh);
      contentH = Math.max(contentH, k.rh);
    }
    if (h < 0) {
      const hug = hugH(n, contentH);
      h = hug.h;
      innerH = hug.inner;
      for (const k of kids) {
        if (k.h.kind === "fill") {
          layout(k, k.rw, innerH);
        }
      }
    }
    n.rh = h;
    place(n, kids, true, pl, pt, innerW, innerH, gaps);
    return;
  }
  let sum = 0;
  let nfill = 0;
  for (const k of kids) {
    if (k.h.kind === "fill" && innerH >= 0) {
      nfill += 1;
      continue;
    }
    layout(k, kidW(k, innerW), k.h.kind === "fixed" ? k.h.v : -1);
    sum += k.rh;
  }
  if (nfill > 0) {
    const share = Math.max((innerH - sum - gaps) / nfill, 0);
    for (const k of kids) {
      if (k.h.kind === "fill") {
        layout(k, kidW(k, innerW), share);
        sum += k.rh;
      }
    }
  }
  if (h < 0) {
    const hug = hugH(n, sum + gaps);
    h = hug.h;
    innerH = hug.inner;
  }
  n.rh = h;
  place(n, kids, false, pl, pt, innerW, innerH, gaps);
}

interface Line {
  kids: LayoutNode[];
  main: number;
  cross: number;
}

function layoutWrap(n: LayoutNode, kids: LayoutNode[], w: number, h: number): void {
  const [pt, pr, pb, pl] = n.pad;
  const horiz = n.dir === "row";
  const innerW = w - pl - pr;
  let innerH = h - pt - pb;
  if (h < 0) {
    innerH = -1;
  }
  let mainAvail = horiz ? innerW : innerH < 0 ? Infinity : innerH;
  const mainOf = (k: LayoutNode): number => (horiz ? k.rw : k.rh);
  const crossOf = (k: LayoutNode): number => (horiz ? k.rh : k.rw);
  const growsMain = (k: LayoutNode): boolean => (horiz ? k.w.kind === "fill" : k.h.kind === "fill");
  const fixedH = (k: LayoutNode): number => (k.h.kind === "fixed" ? k.h.v : -1);

  const lines: Line[] = [];
  let cur: Line = { kids: [], main: 0, cross: 0 };
  for (const k of kids) {
    layout(k, intrinsicW(k), fixedH(k));
    const m = mainOf(k);
    if (cur.kids.length > 0 && cur.main + n.gap + m > mainAvail + 1e-9) {
      lines.push(cur);
      cur = { kids: [], main: 0, cross: 0 };
    }
    if (cur.kids.length > 0) {
      cur.main += n.gap;
    }
    cur.main += m;
    cur.kids.push(k);
  }
  if (cur.kids.length > 0) {
    lines.push(cur);
  }

  let maxMain = 0;
  let total = 0;
  lines.forEach((ln, i) => {
    let nfill = 0;
    for (const k of ln.kids) {
      if (growsMain(k)) {
        nfill += 1;
      }
    }
    const free = mainAvail - ln.main;
    if (nfill > 0 && free !== 0 && free !== Infinity) {
      ln.main = gapsOf(n.gap, ln.kids.length);
      for (const k of ln.kids) {
        const share = free / nfill;
        if (growsMain(k) && horiz) {
          layout(k, Math.max(k.rw + share, 0), fixedH(k));
        } else if (growsMain(k)) {
          layout(k, k.rw, Math.max(k.rh + share, 0));
        }
        ln.main += mainOf(k);
      }
    }
    for (const k of ln.kids) {
      ln.cross = Math.max(ln.cross, crossOf(k));
    }
    for (const k of ln.kids) {
      if (horiz && k.h.kind === "fill") {
        layout(k, k.rw, ln.cross);
      } else if (!horiz && k.w.kind === "fill") {
        layout(k, ln.cross, fixedH(k));
      }
    }
    maxMain = Math.max(maxMain, ln.main);
    total += ln.cross;
    if (i > 0) {
      total += n.lineGap;
    }
  });

  if (h < 0) {
    const hug = hugH(n, horiz ? total : maxMain);
    h = hug.h;
    innerH = hug.inner;
  }
  if (!horiz) {
    mainAvail = innerH;
  }
  n.rh = h;
  let off = 0;
  for (const ln of lines) {
    const g = gapsOf(n.gap, ln.kids.length);
    if (horiz) {
      place(n, ln.kids, true, pl, pt + off, innerW, ln.cross, g);
    } else {
      place(n, ln.kids, false, pl + off, pt, ln.cross, mainAvail, g);
    }
    off += ln.cross + n.lineGap;
  }
}

function place(n: LayoutNode, kids: LayoutNode[], horiz: boolean, pl: number, pt: number, innerW: number, innerH: number, gaps: number): void {
  let used = 0;
  for (const k of kids) {
    used += horiz ? k.rw : k.rh;
  }
  const mainSize = horiz ? innerW : innerH;
  const crossSize = horiz ? innerH : innerW;
  let gap = n.gap;
  let off = 0;
  switch (n.justify) {
    case "center":
      off = (mainSize - used - gaps) / 2;
      break;
    case "end":
      off = mainSize - used - gaps;
      break;
    case "space-between":
      if (kids.length > 1) {
        gap = (mainSize - used) / (kids.length - 1);
      }
      break;
  }
  for (const k of kids) {
    const ks = horiz ? k.rw : k.rh;
    const cs = horiz ? k.rh : k.rw;
    let co = 0;
    switch (n.align) {
      case "center":
        co = (crossSize - cs) / 2;
        break;
      case "end":
        co = crossSize - cs;
        break;
    }
    if (horiz) {
      k.rx = pl + off;
      k.ry = pt + co;
    } else {
      k.rx = pl + co;
      k.ry = pt + off;
    }
    off += ks + gap;
  }
}

function layoutRoot(n: LayoutNode): void {
  forget(n);
  let w = intrinsicW(n);
  if (n.w.kind === "fixed") {
    w = clampW(n, n.w.v);
  }
  layout(n, w, n.h.kind === "fixed" ? n.h.v : -1);
  n.rx = 0;
  n.ry = 0;
}

// MARK: - Text (go/scene/layout.go textInfo)

interface Font {
  family: string;
  weight: number;
  italic: boolean;
  size: number;
  spacing: number;
  /** The face when no family, weight or style is set; "" when the family is used. */
  name: string;
}

interface TextInfo {
  font: Font;
  padX: number;
  padY: number;
  lineH: number;
  align: "left" | "middle" | "right";
  asc: number;
  desc: number;
  text: string;
}

/** gfx.EstimateText: 0.6 em per rune, ascent 0.8 em, descent 0.25 em. */
const textWidth = (ti: TextInfo, s: string): number => [...s].length * (ti.font.size * 0.6 + ti.font.spacing);

function textNatural(ti: TextInfo): number {
  let w = 0;
  for (const ln of ti.text.split("\n")) {
    w = Math.max(w, textWidth(ti, ln));
  }
  return w;
}

/** Split a word after each "-" that sits between a letter or digit and another character. */
function hyphenPieces(word: string): string[] {
  if (!word.includes("-")) {
    return [word];
  }
  const out: string[] = [];
  let start = 0;
  const rs = [...word];
  for (let i = 1; i + 1 < rs.length; i += 1) {
    const prev = rs[i - 1]!;
    if (rs[i] === "-" && /[\p{L}\p{N}]/u.test(prev) && rs[i + 1] !== "-") {
      out.push(rs.slice(start, i + 1).join(""));
      start = i + 1;
    }
  }
  out.push(rs.slice(start).join(""));
  return out;
}

/** Greedy wrap at spaces and after a hyphen inside a word, as in Go. */
function textWrap(ti: TextInfo, w: number): string[] {
  const out: string[] = [];
  for (const para of ti.text.split("\n")) {
    let line = "";
    para.split(" ").forEach((word, i) => {
      hyphenPieces(word).forEach((piece, j) => {
        const cand = j === 0 && i > 0 ? `${line} ${piece}` : line + piece;
        if (i === 0 && j === 0) {
          line = piece;
        } else if (textWidth(ti, cand) <= w + 1e-9) {
          line = cand;
        } else {
          out.push(line);
          line = piece;
        }
      });
    });
    out.push(line);
  }
  return out;
}

// MARK: - Paint

interface Look {
  fill: Color;
  /** The `color linear` of the box, painted over fill (the first visible stop, as Go's gradient.base). */
  grad: SceneGradient | null;
  shadow: SceneShadow | null;
  border: Color;
  text: Color;
  lineW: number;
  radius: [number, number, number, number];
  clip: boolean;
  opacity: number;
}

/** A linear gradient as Go's `gradient`: the line runs at angle degrees (0 up, 90 right) through the centre. */
interface SceneGradient {
  angle: number;
  stops: { offset: number; color: Color }[];
}

/** `shadow dx dy [blur [color]]`; y is up as in pos, the canvas offset is device space, y down. */
interface SceneShadow {
  dx: number;
  dy: number;
  blur: number;
  color: Color;
}

const colorBlack: Color = { r: 0, g: 0, b: 0, a: 1 };
const colorWhite: Color = { r: 1, g: 1, b: 1, a: 1 };
const noColor: Color = { r: 0, g: 0, b: 0, a: 0 };

/** Go's parseColor plus none/transparent. */
function sceneColor(s: string): Color | null {
  const text = s.trim().toLowerCase();
  if (text === "none" || text === "transparent") {
    return noColor;
  }
  return parseColor(text, 1);
}

/** Go's contrast: black on a light fill, white on a dark one. */
function contrast(c: Color): Color {
  const sum = (c.r * 255 * 299 + c.g * 255 * 587 + c.b * 255 * 114) / 1000;
  return sum + 0.5 >= 129 ? colorBlack : colorWhite;
}

/** Go's prep.color: the colour of the property, or def when it is not a colour. */
function colorOf(prep: Prep, rn: RNode, name: string, def: Color): Color {
  const p = propOf(prep, rn, name);
  return p === null ? def : (sceneColor(propText(p, 0)) ?? def);
}

/** Go's prep.fill: `color <c>` or `color linear <angle> <c> [pos] ...`; the gradient's base is the fill. */
function fillOf(prep: Prep, rn: RNode, def: Color): { fill: Color; grad: SceneGradient | null } {
  const p = propOf(prep, rn, "color");
  if (p === null) {
    return { fill: def, grad: null };
  }
  if (propText(p, 0) !== "linear") {
    return { fill: colorOf(prep, rn, "color", def), grad: null };
  }
  const grad = gradientOf(prep, rn, p);
  if (grad === null) {
    return { fill: def, grad: null };
  }
  return { fill: gradientBase(grad), grad };
}

/** Go's prep.gradient: positions 0..1 for every stop or for none, growing; a bad one warns and gives null. */
function gradientOf(prep: Prep, rn: RNode, p: SceneProp): SceneGradient | null {
  const bad = (message: string): null => {
    prep.diag.warn(rn.src ?? { line: 0, col: 0 }, `color linear: ${message}, the default is used`);
    return null;
  };
  const angle = propNum(p, 1);
  if (angle === null || !Number.isFinite(angle)) {
    return bad("the angle is not a number");
  }
  const stops: SceneGradient["stops"] = [];
  let placed = 0;
  for (let i = 2; i < p.args.length; ) {
    const text = propText(p, i);
    const color = sceneColor(text);
    if (color === null) {
      return bad(`"${text}" is not a color`);
    }
    i += 1;
    const offset = propNum(p, i);
    if (offset !== null) {
      i += 1;
      placed += 1;
    }
    stops.push({ offset: offset ?? 0, color });
  }
  const n = stops.length;
  if (n < 2) {
    return bad("needs at least 2 stops");
  }
  if (placed === 0) {
    for (let i = 0; i < n; i += 1) {
      stops[i]!.offset = i / (n - 1);
    }
  } else if (placed !== n) {
    return bad("give a position to every stop or to none");
  }
  for (let i = 0; i < n; i += 1) {
    const off = stops[i]!.offset;
    if (off < 0 || off > 1 || (i > 0 && off < stops[i - 1]!.offset)) {
      return bad("positions must grow within 0..1");
    }
  }
  return { angle: ((angle % 360) + 360) % 360, stops };
}

/** Go's gradient.base: the first stop that shows, the first one when all are transparent. */
function gradientBase(g: SceneGradient): Color {
  return g.stops.find((s) => s.color.a > 0)?.color ?? g.stops[0]!.color;
}

const defaultShadowBlur = 5;
const defaultShadowColor: Color = { r: 0, g: 0, b: 0, a: 1 / 3 };

/** Go's prep.shadow; blur 5 and black at 1/3 alpha by default. A bad offset drops the shadow. */
function shadowOf(prep: Prep, rn: RNode): SceneShadow | null {
  const p = propOf(prep, rn, "shadow");
  if (p === null) {
    return null;
  }
  const at = rn.src ?? { line: 0, col: 0 };
  const dx = propNum(p, 0);
  const dy = propNum(p, 1);
  if (dx === null || dy === null) {
    prep.diag.warn(at, "shadow takes dx dy [blur [color]], numbers for the offset; ignored");
    return null;
  }
  const sh: SceneShadow = { dx, dy, blur: defaultShadowBlur, color: defaultShadowColor };
  if (p.args.length > 2) {
    const v = propNum(p, 2);
    if (v !== null && v >= 0) {
      sh.blur = v;
    } else {
      prep.diag.warn(at, `shadow: the blur is not a number from 0, ${defaultShadowBlur} is used`);
    }
  }
  if (p.args.length > 3) {
    const text = propText(p, 3);
    const color = sceneColor(text);
    if (color !== null) {
      sh.color = color;
    } else {
      prep.diag.warn(at, `shadow: "${text}" is not a color, the default is used`);
    }
  }
  if (p.args.length > 4) {
    prep.diag.warn(at, "shadow takes dx dy [blur [color]], the extra values are ignored");
  }
  return sh;
}

interface Icon {
  segments: PathCommand[];
  vx: number;
  vy: number;
  vw: number;
  vh: number;
  fill: Color;
  stroke: Color;
}

/** Go's SVG colour: the hex digits themselves, rounded to 0..255 (no Generic RGB conversion). */
function sceneCss(c: Color): string {
  const q = (v: number): number => Math.round(Math.max(0, Math.min(1, v)) * 255);
  return c.a >= 1 ? `rgb(${q(c.r)},${q(c.g)},${q(c.b)})` : `rgba(${q(c.r)},${q(c.g)},${q(c.b)},${Math.round(Math.max(0, c.a) * 1000) / 1000})`;
}

/** The drawing of one scene: nodes, their layout and their look. */
interface Prep {
  diag: Diagnostics;
  frames: Map<RNode, LayoutNode>;
  texts: Map<RNode, TextInfo>;
  looks: Map<RNode, Look>;
  icons: Map<RNode, Icon>;
}

const defaultFontSize = 18;
const textPadX = 10;
const textPadY = 6;
const fontName = "Helvetica, Arial, sans-serif";
const codeFont = "Inter";

// MARK: - The scene

export interface TennScene {
  /** The extent of the content, as go/scene Screen.W and H. */
  width: number;
  height: number;
  /** y-up bounds, as DrawableScene.getBounds. */
  bounds: Rect;
  diagnostics: { message: string; warn: boolean; line: number; col: number }[];
  draw(ctx: Canvas2D, at?: Point): void;
  /** The laid out items in tree order, for tests and the inspector. */
  boxes: { name: string; path: string; x: number; y: number; width: number; height: number; depth: number }[];
}

/** model.SceneElement: an element with a use or an item that holds nodes or auto-layout is a scene (5a.17). */
export function isSceneElement(element: Element): boolean {
  const isUse = (c: TennNode): boolean => c.getIdent(0) === "use" && c.count > 2;
  for (const c of element.properties) {
    if (isUse(c)) {
      return true;
    }
  }
  // The commands of an item block: a nested item, a use, a slot or auto-layout makes the element a scene.
  for (const item of element.items) {
    for (const c of item.properties) {
      const name = c.getIdent(0);
      if (name === "auto-layout" || name === "slot" || name === "item" || isUse(c)) {
        return true;
      }
    }
  }
  return false;
}

/** The file tree of the element: its own element, its ancestors and the elements beside them. */
function fileTree(element: Element, diag: Diagnostics): SceneNode {
  let root: Element = element;
  while (root.parent !== null) {
    root = root.parent;
  }
  const file = newNode("file", "", null, null);
  const seen = new Set<Element>();
  const addElement = (owner: SceneNode, el: Element): void => {
    if (seen.has(el)) {
      return;
    }
    seen.add(el);
    const node = newNode("element", el.name, owner, null);
    owner.elems.push(node);
    const entries: (TennNode | DiagramItem)[] = [...el.properties, ...el.items];
    // The model keeps items and properties apart; the file order is their positions (spec 9.3).
    const order = new Map<TennNode | DiagramItem, number>();
    for (const e of entries) {
      order.set(e, "token" in e ? ((e as TennNode).token?.pos ?? Number.MAX_SAFE_INTEGER) : ((e as DiagramItem).sourcePos ?? Number.MAX_SAFE_INTEGER));
    }
    entries.sort((a, b) => (order.get(a) ?? 0) - (order.get(b) ?? 0));
    fill(node, entries, diag);
    // Nested elements come after the content, as persistence writes them; scope does not depend on order.
    for (const child of el.elements) {
      addElement(node, child);
    }
  };
  addElement(file, root);
  return file;
}

function sceneNodeFor(file: SceneNode, element: Element): SceneNode | null {
  let found: SceneNode | null = null;
  const walk = (n: SceneNode): void => {
    for (const e of n.elems) {
      if (e.name === element.name && found === null) {
        found = e;
        return;
      }
      walk(e);
    }
  };
  walk(file);
  return found;
}

/** The prep of a screen: sizes, styles and text of every node, then the layout. */
function prepare(expanded: RNode, diag: Diagnostics): { prep: Prep; root: LayoutNode } {
  const prep: Prep = { diag, frames: new Map(), texts: new Map(), looks: new Map(), icons: new Map() };
  animations(expanded, new Map(), diag);
  const root = newLayout(expanded);
  root.dir = "abs";
  prep.frames.set(expanded, root);
  for (const k of expanded.kids) {
    if (k.kind === "item" && !flag(prep, k, "hidden")) {
      root.kids.push(itemNode(prep, k, false, colorBlack));
    }
  }
  return { prep, root };
}

function propOf(prep: Prep, rn: RNode, name: string): SceneProp | null {
  const p = prop(rn, name);
  if (p === null) {
    return null;
  }
  if (!hasExpr(p)) {
    return p;
  }
  return evaluate(prep.diag, rn, p);
}

function hasExpr(p: SceneProp): boolean {
  if (p.val !== undefined) {
    return false;
  }
  for (const a of p.args) {
    if (a.kind === "Expression" || a.kind === "ExpressionBlock") {
      return true;
    }
    if (a.kind === "StringLit" && (a.getIdentText() ?? "").includes("${")) {
      return true;
    }
  }
  return false;
}

function evaluate(diag: Diagnostics, rn: RNode, p: SceneProp): SceneProp | null {
  const val: string[] = [];
  for (const a of p.args) {
    const v = evalArg(a, rn.vars);
    if (v === null) {
      diag.warn(rn.src ?? { line: 0, col: 0 }, `${p.name}: the expression "${a.getIdentText() ?? ""}" is not evaluated`);
      return null;
    }
    val.push(v);
  }
  return { name: p.name, args: p.args, block: p.block, val };
}

function num(prep: Prep, rn: RNode, name: string): number | null {
  const p = propOf(prep, rn, name);
  return p === null ? null : propNum(p, 0);
}

function word(prep: Prep, rn: RNode, name: string, allowed: string[]): string | null {
  const p = propOf(prep, rn, name);
  if (p === null) {
    return null;
  }
  const s = propText(p, 0);
  if (allowed.includes(s)) {
    return s;
  }
  if (name === "auto-layout" && s === "") {
    return null; // "auto-layout" alone is a row
  }
  prep.diag.fail(rn.src ?? { line: 0, col: 0 }, `${name} takes one of ${allowed.join(", ")}, got "${s}"`);
  return null;
}

function flag(prep: Prep, rn: RNode, name: string): boolean {
  const p = propOf(prep, rn, name);
  if (p === null) {
    return false;
  }
  const s = propText(p, 0);
  return s === "yes" || s === "true";
}

function sizeOf(prep: Prep, rn: RNode, name: string, inCtx: boolean, parentAuto: boolean): Size {
  const p = propOf(prep, rn, name);
  if (p === null || p.args.length === 0) {
    return HUG;
  }
  const v = propNum(p, 0);
  if (v !== null) {
    return fixed(v);
  }
  const s = propText(p, 0);
  if (s === "hug" || s === "fill") {
    if (!inCtx) {
      prep.diag.fail({ line: 0, col: 0 }, `${name} ${s} needs auto-layout on the item or on its parent`);
      return HUG;
    }
    if (s === "fill") {
      if (!parentAuto) {
        return HUG;
      }
      return FILL;
    }
    return HUG;
  }
  prep.diag.fail({ line: 0, col: 0 }, `${name} takes a number, hug or fill, got "${s}"`);
  return HUG;
}

function paddingOf(prep: Prep, rn: RNode): [number, number, number, number] {
  const p = propOf(prep, rn, "padding");
  if (p === null) {
    return [0, 0, 0, 0];
  }
  const v: number[] = [];
  for (let i = 0; i < p.args.length; i += 1) {
    const f = propNum(p, i);
    if (f === null) {
      prep.diag.fail({ line: 0, col: 0 }, "padding takes numbers");
      return [0, 0, 0, 0];
    }
    v.push(f);
  }
  if (v.length === 1) {
    return [v[0]!, v[0]!, v[0]!, v[0]!];
  }
  if (v.length === 2) {
    return [v[0]!, v[1]!, v[0]!, v[1]!];
  }
  if (v.length === 4) {
    return [v[0]!, v[1]!, v[2]!, v[3]!];
  }
  prep.diag.fail({ line: 0, col: 0 }, `padding takes 1, 2 or 4 numbers, got ${v.length}`);
  return [0, 0, 0, 0];
}

function textOf(prep: Prep, rn: RNode, inCtx: boolean): TextInfo | null {
  let title = inCtx ? "" : rn.name;
  const p = propOf(prep, rn, "title");
  if (p !== null) {
    title = propText(p, 0);
  }
  if (title === "") {
    return null;
  }
  let size = defaultFontSize;
  const fs = propOf(prep, rn, "font-size");
  if (fs !== null) {
    const v = propNum(fs, 0);
    if (v !== null && v > 0) {
      size = v;
    }
  }
  const ti: TextInfo = {
    font: fontOf(prep, rn, size),
    padX: inCtx ? 0 : textPadX,
    padY: inCtx ? 0 : textPadY,
    lineH: Math.round(size * 1.05),
    align: "middle",
    asc: size * 0.8,
    desc: size * 0.25,
    text: title,
  };
  const lh = propOf(prep, rn, "line-height");
  if (lh !== null) {
    const v = propNum(lh, 0);
    if (v !== null && v > 0) {
      ti.lineH = v;
    }
  }
  const layoutProp = propOf(prep, rn, "layout");
  if (layoutProp !== null) {
    const w = propText(layoutProp, 0);
    if (w === "left" || w === "right") {
      ti.align = w;
    }
  }
  const ls = propOf(prep, rn, "line-spacing");
  if (ls !== null) {
    ti.lineH += propNum(ls, 0) ?? 0;
  }
  return ti;
}

/** font-family, font-weight, font-style and letter-spacing, as Go reads them. */
function fontOf(prep: Prep, rn: RNode, size: number): Font {
  const f: Font = { family: "", weight: 0, italic: false, size, spacing: 0, name: "" };
  const ls = propOf(prep, rn, "letter-spacing");
  if (ls !== null) {
    f.spacing = propNum(ls, 0) ?? 0;
  }
  const fam = propOf(prep, rn, "font-family");
  if (fam !== null) {
    f.family = propText(fam, 0);
  }
  const w = propOf(prep, rn, "font-weight");
  if (w !== null) {
    const s = propText(w, 0);
    if (s === "normal") {
      f.weight = 400;
    } else if (s === "bold") {
      f.weight = 700;
    } else {
      const v = propNum(w, 0);
      if (v !== null && v >= 100 && v <= 900) {
        f.weight = v;
      }
    }
  }
  const st = propOf(prep, rn, "font-style");
  if (st !== null) {
    const s = propText(st, 0);
    f.italic = s === "italic" || s === "oblique";
  }
  if (f.family === "" && f.weight === 0 && !f.italic) {
    f.name = fontName;
  } else if (f.family === "") {
    f.family = fontName.split(",")[0]!.trim();
  }
  return f;
}

/** The CSS font of the scene text; the SVG backend writes family, weight and size from it. */
function fontCss(f: Font): string {
  const family = f.family !== "" ? f.family : f.name !== "" ? f.name : codeFont;
  const weight = f.weight > 0 ? `${f.weight} ` : "";
  const italic = f.italic ? "italic " : "";
  return `${italic}${weight}${f.size}px ${family}`;
}

function lookOf(prep: Prep, rn: RNode, inCtx: boolean, up: Color): Look {
  const look: Look = {
    fill: inCtx ? noColor : colorWhite,
    grad: null,
    shadow: null,
    border: inCtx ? noColor : colorBlack,
    text: up,
    lineW: 1,
    radius: [0, 0, 0, 0],
    clip: false,
    opacity: 1,
  };
  const fill = fillOf(prep, rn, look.fill);
  look.fill = fill.fill;
  look.grad = fill.grad;
  look.shadow = shadowOf(prep, rn);
  look.border = colorOf(prep, rn, "border-color", look.border);
  let text = up;
  if (look.fill.a > 0) {
    text = contrast(look.fill);
  }
  look.text = colorOf(prep, rn, "text-color", text);
  const lw = num(prep, rn, "line-width");
  if (lw !== null) {
    look.lineW = lw;
  }
  const radius = propOf(prep, rn, "corner-radius");
  if (radius !== null) {
    look.radius = radii(prep, radius);
  }
  const opacity = num(prep, rn, "opacity");
  if (opacity !== null && opacity >= 0 && opacity <= 1) {
    look.opacity = opacity;
  }
  if (inCtx) {
    look.clip = flag(prep, rn, "clip");
  }
  return look;
}

/** corner-radius: one value for all corners or four in CSS order. */
function radii(prep: Prep, p: SceneProp): [number, number, number, number] {
  const first = propNum(p, 0) ?? 0;
  const out: [number, number, number, number] = [first, first, first, first];
  if (p.args.length === 4) {
    for (let i = 0; i < 4; i += 1) {
      const v = propNum(p, i);
      if (v === null) {
        prep.diag.warn({ line: 0, col: 0 }, `corner-radius: radius ${i + 1} is not a number, the first value is used for all corners`);
        return [first, first, first, first];
      }
      out[i] = v;
    }
  }
  return out;
}

const holdsItems = (rn: RNode): boolean => rn.kids.some((k) => k.kind === "item");

function itemNode(prep: Prep, rn: RNode, parentAuto: boolean, up: Color): LayoutNode {
  const n = newLayout(rn);
  prep.frames.set(rn, n);
  const auto = propOf(prep, rn, "auto-layout") !== null;
  const inCtx = auto || parentAuto;
  const posProp = propOf(prep, rn, "pos");
  if (posProp !== null) {
    n.x = propNum(posProp, 0) ?? 0;
    // Y is up in .tenn; pos is the top left corner, so the layout y is the negative one (5a.8).
    n.y = 0 - (propNum(posProp, 1) ?? 0);
  }
  n.w = sizeOf(prep, rn, "width", inCtx, parentAuto);
  n.h = sizeOf(prep, rn, "height", inCtx, parentAuto);
  if (inCtx) {
    n.minW = num(prep, rn, "min-width") ?? 0;
    n.maxW = num(prep, rn, "max-width") ?? 0;
    n.minH = num(prep, rn, "min-height") ?? 0;
    n.maxH = num(prep, rn, "max-height") ?? 0;
    n.absolute = flag(prep, rn, "absolute");
  }
  if (auto) {
    const dir = word(prep, rn, "auto-layout", ["row", "column", "abs"]);
    if (dir !== null) {
      n.dir = dir as Dir;
    }
    n.gap = num(prep, rn, "gap") ?? 0;
    n.lineGap = num(prep, rn, "line-gap") ?? 0;
    n.wrap = flag(prep, rn, "wrap");
    n.pad = paddingOf(prep, rn);
    const align = word(prep, rn, "align", ["start", "center", "end"]);
    if (align !== null) {
      n.align = align as Align;
    }
    const justify = word(prep, rn, "justify", ["start", "center", "end", "space-between"]);
    if (justify !== null) {
      n.justify = justify as Align;
    }
  } else {
    n.dir = "abs";
  }
  // The root of an instance that holds items is a container, not a box of its own.
  const look = lookOf(prep, rn, inCtx || (rn.inst && holdsItems(rn)), up);
  prep.looks.set(rn, look);
  for (const k of rn.kids) {
    if (k.kind !== "item" || flag(prep, k, "hidden")) {
      continue;
    }
    n.kids.push(itemNode(prep, k, auto, look.text));
  }
  if (n.kids.length === 0) {
    leaf(prep, rn, n, inCtx);
  } else if (!rn.inst) {
    const ti = textOf(prep, rn, inCtx);
    if (ti !== null) {
      prep.texts.set(rn, ti);
    }
  }
  return n;
}

/** An item without children takes the size of its title, or of its icon. */
function leaf(prep: Prep, rn: RNode, n: LayoutNode, inCtx: boolean): void {
  const icon = iconOf(prep, rn);
  if (icon !== null) {
    prep.icons.set(rn, icon);
    n.leafW = () => icon.vw;
    n.leafH = () => icon.vh;
  }
  const ti = textOf(prep, rn, inCtx || icon !== null);
  if (ti === null) {
    return;
  }
  prep.texts.set(rn, ti);
  const pad = propOf(prep, rn, "padding");
  if (pad !== null && propOf(prep, rn, "auto-layout") !== null) {
    ti.padY = n.pad[0];
    ti.padX = n.pad[3];
  }
  n.leafW = () => Math.ceil(textNatural(ti)) + 2 * ti.padX;
  n.leafH = (w: number) => textWrap(ti, w - 2 * ti.padX).length * ti.lineH + 2 * ti.padY;
}

/** path and view-box of a leaf: a vector icon drawn in place of the box (5a.16). */
function iconOf(prep: Prep, rn: RNode): Icon | null {
  const p = propOf(prep, rn, "path");
  if (p === null) {
    return null;
  }
  const d = propText(p, 0);
  const segments = parseSvgPath(d);
  if (segments === null || segments.length === 0) {
    prep.diag.warn({ line: 0, col: 0 }, "path: not an SVG path, the item is drawn as a box");
    return null;
  }
  let vx = 0;
  let vy = 0;
  let vw = 0;
  let vh = 0;
  const bounds = pathBounds(segments);
  if (bounds !== null) {
    [vx, vy, vw, vh] = bounds;
  }
  const vb = propOf(prep, rn, "view-box");
  if (vb !== null) {
    const v = [0, 1, 2, 3].map((i) => propNum(vb, i));
    if (v.every((x) => x !== null) && v[2]! > 0 && v[3]! > 0) {
      [vx, vy, vw, vh] = [v[0]!, v[1]!, v[2]!, v[3]!];
    }
  }
  if (vw <= 0 || vh <= 0) {
    prep.diag.warn({ line: 0, col: 0 }, "path: empty bounds, give a view-box");
    return null;
  }
  const hasFill = propOf(prep, rn, "color") !== null;
  const hasStroke = propOf(prep, rn, "border-color") !== null;
  let fill: Color = noColor;
  let stroke: Color = noColor;
  if (!hasFill && !hasStroke) {
    fill = prep.looks.get(rn)?.text ?? colorBlack;
  } else {
    const c = propOf(prep, rn, "color");
    if (c !== null) {
      fill = sceneColor(propText(c, 0)) ?? noColor;
    }
    const s = propOf(prep, rn, "border-color");
    if (s !== null) {
      stroke = sceneColor(propText(s, 0)) ?? noColor;
    }
  }
  return { segments, vx, vy, vw, vh, fill, stroke };
}

// MARK: - SVG path data (the icon subset: M L H V C S Q T A Z, absolute and relative)

export type PathCommand =
  | { op: "M" | "L"; x: number; y: number }
  | { op: "Q"; cx: number; cy: number; x: number; y: number }
  | { op: "Z" };

/** Parse SVG path data into move/line/quadratic commands, curves and arcs flattened. */
export function parseSvgPath(d: string): PathCommand[] | null {
  const tokens = d.match(/[a-zA-Z]|[-+]?(?:\d+\.?\d*|\.\d+)(?:[eE][-+]?\d+)?/g);
  if (tokens === null) {
    return null;
  }
  const out: PathCommand[] = [];
  let i = 0;
  let cx = 0;
  let cy = 0;
  let sx = 0;
  let sy = 0;
  let cmd = "";
  let lastCtrl: { x: number; y: number } | null = null;
  let lastOp = "";
  const num = (): number | null => {
    const t = tokens[i];
    if (t === undefined || /[a-zA-Z]/.test(t)) {
      return null;
    }
    i += 1;
    return Number(t);
  };
  const curve = (x1: number, y1: number, x2: number, y2: number, x: number, y: number): void => {
    // Flatten a cubic (or a quadratic) into short lines: colours and sizes are what the scene needs.
    const n = clamp(Math.ceil(Math.hypot(x - cx, y - cy) / 4), 1, 48);
    for (let k = 1; k <= n; k += 1) {
      const t = k / n;
      const u = 1 - t;
      const px = u * u * u * cx + 3 * u * u * t * x1 + 3 * u * t * t * x2 + t * t * t * x;
      const py = u * u * u * cy + 3 * u * u * t * y1 + 3 * u * t * t * y2 + t * t * t * y;
      out.push({ op: "L", x: px, y: py });
    }
    cx = x;
    cy = y;
  };
  const arc = (rx: number, ry: number, rot: number, large: boolean, sweep: boolean, x: number, y: number): void => {
    const phi = (rot * Math.PI) / 180;
    const cos = Math.cos(phi);
    const sin = Math.sin(phi);
    const dx = (cx - x) / 2;
    const dy = (cy - y) / 2;
    const x1 = cos * dx + sin * dy;
    const y1 = -sin * dx + cos * dy;
    let a = Math.abs(rx);
    let b = Math.abs(ry);
    const lambda = (x1 * x1) / (a * a) + (y1 * y1) / (b * b);
    if (lambda > 1) {
      const s = Math.sqrt(lambda);
      a *= s;
      b *= s;
    }
    const sign = large === sweep ? -1 : 1;
    const num0 = a * a * b * b - a * a * y1 * y1 - b * b * x1 * x1;
    const den = a * a * y1 * y1 + b * b * x1 * x1;
    const coef = sign * Math.sqrt(Math.max(0, num0 / den));
    const cxp = (coef * a * y1) / b;
    const cyp = (-coef * b * x1) / a;
    const ccx = cos * cxp - sin * cyp + (cx + x) / 2;
    const ccy = sin * cxp + cos * cyp + (cy + y) / 2;
    const angle = (ux: number, uy: number, vx: number, vy: number): number => {
      const dot = ux * vx + uy * vy;
      const len = Math.hypot(ux, uy) * Math.hypot(vx, vy);
      const a0 = Math.acos(clamp(dot / len, -1, 1));
      return ux * vy - uy * vx < 0 ? -a0 : a0;
    };
    const start = angle(1, 0, (x1 - cxp) / a, (y1 - cyp) / b);
    let delta = angle((x1 - cxp) / a, (y1 - cyp) / b, (-x1 - cxp) / a, (-y1 - cyp) / b);
    if (!sweep && delta > 0) {
      delta -= 2 * Math.PI;
    } else if (sweep && delta < 0) {
      delta += 2 * Math.PI;
    }
    const steps = clamp(Math.ceil(Math.abs(delta) / (Math.PI / 16)), 1, 64);
    for (let k = 1; k <= steps; k += 1) {
      const t = start + (delta * k) / steps;
      const px = ccx + a * Math.cos(t) * cos - b * Math.sin(t) * sin;
      const py = ccy + a * Math.cos(t) * sin + b * Math.sin(t) * cos;
      out.push({ op: "L", x: px, y: py });
    }
    cx = x;
    cy = y;
  };
  while (i < tokens.length) {
    const t = tokens[i]!;
    if (/[a-zA-Z]/.test(t)) {
      cmd = t;
      i += 1;
    } else if (cmd === "") {
      return null;
    }
    const rel = cmd === cmd.toLowerCase();
    const ox = rel ? cx : 0;
    const oy = rel ? cy : 0;
    switch (cmd.toUpperCase()) {
      case "M":
      case "L": {
        const x = num();
        const y = num();
        if (x === null || y === null) {
          return null;
        }
        cx = ox + x;
        cy = oy + y;
        out.push({ op: cmd.toUpperCase() as "M" | "L", x: cx, y: cy });
        if (cmd === "M") {
          sx = cx;
          sy = cy;
          cmd = "L"; // further pairs are lines
        }
        lastOp = "";
        break;
      }
      case "H": {
        const x = num();
        if (x === null) {
          return null;
        }
        cx = ox + x;
        out.push({ op: "L", x: cx, y: cy });
        break;
      }
      case "V": {
        const y = num();
        if (y === null) {
          return null;
        }
        cy = oy + y;
        out.push({ op: "L", x: cx, y: cy });
        break;
      }
      case "C":
      case "S":
      case "Q":
      case "T": {
        const args: number[] = [];
        const count = cmd.toUpperCase() === "C" ? 6 : cmd.toUpperCase() === "S" || cmd.toUpperCase() === "Q" ? 4 : 2;
        for (let k = 0; k < count; k += 1) {
          const v = num();
          if (v === null) {
            return null;
          }
          args.push(v);
        }
        if (cmd.toUpperCase() === "Q") {
          const qx = ox + args[0]!;
          const qy = oy + args[1]!;
          const x = ox + args[2]!;
          const y = oy + args[3]!;
          const c1x = cx + (2 / 3) * (qx - cx);
          const c1y = cy + (2 / 3) * (qy - cy);
          const c2x = x + (2 / 3) * (qx - x);
          const c2y = y + (2 / 3) * (qy - y);
          curve(c1x, c1y, c2x, c2y, x, y);
          lastCtrl = { x: qx, y: qy };
        } else if (cmd.toUpperCase() === "C" || cmd.toUpperCase() === "S") {
          let x1: number;
          let y1: number;
          let rest: number[];
          if (cmd.toUpperCase() === "C") {
            x1 = ox + args[0]!;
            y1 = oy + args[1]!;
            rest = args.slice(2);
          } else {
            x1 = lastOp === "C" || lastOp === "S" ? 2 * cx - (lastCtrl?.x ?? cx) : cx;
            y1 = lastOp === "C" || lastOp === "S" ? 2 * cy - (lastCtrl?.y ?? cy) : cy;
            rest = args;
          }
          const x2 = ox + rest[0]!;
          const y2 = oy + rest[1]!;
          const x = ox + rest[2]!;
          const y = oy + rest[3]!;
          curve(x1, y1, x2, y2, x, y);
          lastCtrl = { x: x2, y: y2 };
        } else {
          // T: the reflection of the last control point
          const x = ox + args[0]!;
          const y = oy + args[1]!;
          const qx: number = lastOp === "Q" || lastOp === "T" ? 2 * cx - (lastCtrl?.x ?? cx) : cx;
          const qy: number = lastOp === "Q" || lastOp === "T" ? 2 * cy - (lastCtrl?.y ?? cy) : cy;
          const c1x = cx + (2 / 3) * (qx - cx);
          const c1y = cy + (2 / 3) * (qy - cy);
          const c2x = x + (2 / 3) * (qx - x);
          const c2y = y + (2 / 3) * (qy - y);
          curve(c1x, c1y, c2x, c2y, x, y);
          lastCtrl = { x: qx, y: qy };
        }
        lastOp = cmd.toUpperCase() === "Q" || cmd.toUpperCase() === "T" ? "Q" : "C";
        break;
      }
      case "A": {
        const rx = num();
        const ry = num();
        const rot = num();
        const large = num();
        const sweep = num();
        const x = num();
        const y = num();
        if (rx === null || ry === null || rot === null || large === null || sweep === null || x === null || y === null) {
          return null;
        }
        arc(rx, ry, rot, large !== 0, sweep !== 0, ox + x, oy + y);
        lastOp = "";
        break;
      }
      case "Z": {
        out.push({ op: "Z" });
        cx = sx;
        cy = sy;
        lastOp = "";
        break;
      }
      default:
        return null;
    }
  }
  return out;
}

function pathBounds(segments: PathCommand[]): [number, number, number, number] | null {
  let x0 = Infinity;
  let y0 = Infinity;
  let x1 = -Infinity;
  let y1 = -Infinity;
  for (const s of segments) {
    if (s.op === "Z") {
      continue;
    }
    x0 = Math.min(x0, s.x);
    x1 = Math.max(x1, s.x);
    y0 = Math.min(y0, s.y);
    y1 = Math.max(y1, s.y);
  }
  return x0 === Infinity ? null : [x0, y0, x1 - x0, y1 - y0];
}

// MARK: - Drawing

function roundRectPath(ctx: Canvas2D, x: number, y: number, w: number, h: number, radius: [number, number, number, number]): void {
  const [tl, tr, br, bl] = radius.map((r) => Math.max(0, Math.min(r, Math.min(w, h) / 2))) as [number, number, number, number];
  ctx.beginPath();
  ctx.moveTo(x + tl, y);
  ctx.lineTo(x + w - tr, y);
  ctx.arcTo(x + w, y, x + w, y + tr, tr);
  ctx.lineTo(x + w, y + h - br);
  ctx.arcTo(x + w, y + h, x + w - br, y + h, br);
  ctx.lineTo(x + bl, y + h);
  ctx.arcTo(x, y + h, x, y + h - bl, bl);
  ctx.lineTo(x, y + tl);
  ctx.arcTo(x, y, x + tl, y, tl);
  ctx.closePath();
}

/** The linear gradient of the box, through its centre at the angle and reaching the corners (Go's gradient.at). */
function fillGradient(ctx: Canvas2D, g: SceneGradient, w: number, h: number): boolean {
  // shortcut: the SVG writer has no gradient object, so an SVG export draws the base colour; add one to
  // SvgContext when the export needs gradients.
  if (typeof ctx.createLinearGradient !== "function") {
    return false;
  }
  const a = (g.angle * Math.PI) / 180;
  // draw() flips the canvas once, so y is up here and 0 degrees points up as it does on screen in Go.
  const dx = Math.sin(a);
  const dy = Math.cos(a);
  const half = (Math.abs(w * dx) + Math.abs(h * dy)) / 2;
  const line = ctx.createLinearGradient(w / 2 - dx * half, h / 2 - dy * half, w / 2 + dx * half, h / 2 + dy * half);
  for (const s of g.stops) {
    line.addColorStop(s.offset, sceneCss(s.color));
  }
  ctx.fillStyle = line;
  ctx.fill();
  return true;
}

/** Paint with the shadow of Go's gfx.Shadow: the shape and its shadow, the shadow is y-up in scene space. */
function withShadow(ctx: Canvas2D, sh: SceneShadow, paint: () => void): void {
  ctx.save();
  // The canvas offset is device space and y-down; blur and alpha are the quirks of drawable.setShadow.
  ctx.shadowOffsetX = sh.dx * shadowScale.value;
  ctx.shadowOffsetY = -sh.dy * shadowScale.value;
  ctx.shadowBlur = sh.blur * shadowScale.value;
  ctx.shadowColor = sceneCss({ ...sh.color, a: sh.blur > 0 ? sh.color.a ** canvasQuirks.shadowAlphaPower : sh.color.a });
  paint();
  ctx.restore();
}

function drawText(ctx: Canvas2D, ti: TextInfo, w: number, h: number, leafNode: boolean, color: Color): void {
  const lines = textWrap(ti, w - 2 * ti.padX);
  const y0 = leafNode ? (h - lines.length * ti.lineH) / 2 : ti.padY;
  ctx.save();
  ctx.font = fontCss(ti.font);
  ctx.textBaseline = "alphabetic";
  ctx.fillStyle = sceneCss(color);
  lines.forEach((s, i) => {
    if (s === "") {
      return;
    }
    let x = ti.padX;
    if (leafNode) {
      switch (ti.align) {
        case "left":
          break;
        case "right":
          x = w - ti.padX - textWidth(ti, s);
          break;
        default:
          x = (w - textWidth(ti, s)) / 2;
      }
    }
    const base = y0 + i * ti.lineH + (ti.lineH - (ti.asc + ti.desc)) / 2 + ti.asc;
    // The scene draws y down (the two flips of renderElement and of draw() cancel), so the baseline
    // needs no local flip: the glyphs stand on it as usual.
    ctx.fillText(s, x, base);
  });
  ctx.restore();
}

function drawIcon(ctx: Canvas2D, icon: Icon, w: number, h: number, lineW: number): void {
  const s = Math.min(w / icon.vw, h / icon.vh);
  const ox = (w - icon.vw * s) / 2 - icon.vx * s;
  const oy = (h - icon.vh * s) / 2 - icon.vy * s;
  ctx.beginPath();
  for (const seg of icon.segments) {
    if (seg.op === "M") {
      ctx.moveTo(ox + seg.x * s, oy + seg.y * s);
    } else if (seg.op === "L") {
      ctx.lineTo(ox + seg.x * s, oy + seg.y * s);
    } else if (seg.op === "Q") {
      ctx.quadraticCurveTo(ox + seg.cx * s, oy + seg.cy * s, ox + seg.x * s, oy + seg.y * s);
    } else {
      ctx.closePath();
    }
  }
  if (icon.fill.a > 0) {
    ctx.fillStyle = sceneCss(icon.fill);
    ctx.fill();
  }
  if (lineW > 0 && icon.stroke.a > 0) {
    ctx.lineWidth = lineW;
    ctx.strokeStyle = sceneCss(icon.stroke);
    ctx.stroke();
  }
}

function pathOfNode(rn: RNode, path: string): string {
  return path === "" ? rn.key : `${path}.${rn.key}`;
}

// MARK: - Build

/**
 * Lay out and paint a scene element (spec 4, 5b, 5c, 13). The result draws itself in y-up space, as
 * DrawableScene does, so renderElement's flip and offset work unchanged.
 */
export function buildTennScene(element: Element): TennScene {
  const diag = new Diagnostics();
  const file = fileTree(element, diag);
  const source = sceneNodeFor(file, element);
  const boxes: TennScene["boxes"] = [];
  if (source === null) {
    return { width: 0, height: 0, bounds: { x: 0, y: 0, width: 0, height: 0 }, diagnostics: diag.list, draw: () => {}, boxes };
  }
  const expander = new Expander(diag);
  const expanded = expander.expandElement(source);
  if (expanded === null) {
    return { width: 0, height: 0, bounds: { x: 0, y: 0, width: 0, height: 0 }, diagnostics: diag.list, draw: () => {}, boxes };
  }
  evalTree(expanded, diag);
  applyStyles(expanded);
  const { prep, root } = prepare(expanded, diag);
  layoutRoot(root);
  const width = root.rw;
  const height = root.rh;

  const walk = (rn: RNode, path: string, depth: number, ox: number, oy: number): void => {
    const ln = prep.frames.get(rn);
    if (ln === undefined) {
      return;
    }
    boxes.push({ name: rn.name, path, x: ln.rx, y: ln.ry, width: ln.rw, height: ln.rh, depth });
    const nx = ox + ln.rx;
    const ny = oy + ln.ry;
    if (!(rn === expanded && depth === 0)) {
      // gfx.Root.SubtreeBounds: an item above or left of the origin is part of the picture too.
      minX = Math.min(minX, nx);
      minY = Math.min(minY, ny);
      maxX = Math.max(maxX, nx + ln.rw);
      maxY = Math.max(maxY, ny + ln.rh);
    }
    for (const k of rn.kids) {
      if (k.kind === "item") {
        walk(k, pathOfNode(k, path), depth + 1, nx, ny);
      }
    }
  };
  let minX = 0;
  let minY = 0;
  let maxX = width;
  let maxY = height;
  walk(expanded, "", 0, 0, 0);

  const drawNode = (ctx: Canvas2D, rn: RNode, ln: LayoutNode, isRoot: boolean): void => {
    const look = prep.looks.get(rn);
    ctx.save();
    ctx.translate(ln.rx, ln.ry);
    const opacity = look?.opacity ?? 1;
    if (opacity < 1) {
      ctx.globalAlpha = (ctx.globalAlpha ?? 1) * opacity;
    }
    if (!isRoot && look !== undefined) {
      const w = ln.rw;
      const h = ln.rh;
      if (w > 0 && h > 0) {
        const icon = prep.icons.get(rn);
        if (icon !== undefined) {
          drawIcon(ctx, icon, w, h, look.lineW);
          const ti = prep.texts.get(rn);
          if (ti !== undefined) {
            drawText(ctx, ti, w, h, true, look.text);
          }
        } else {
          if (look.clip) {
            roundRectPath(ctx, 0, 0, w, h, look.radius);
            ctx.clip();
          }
          // Go's picture(): the shadow goes under the fill; a gradient is painted over an opaque base, since
          // a canvas gradient with a shadow casts it from the gradient's alpha; a border without a fill casts it.
          let shadow = look.shadow;
          if (look.fill.a > 0) {
            roundRectPath(ctx, 0, 0, w, h, look.radius);
            if (look.grad !== null) {
              if (shadow !== null) {
                const base = { ...look.fill, a: 1 };
                withShadow(ctx, shadow, () => {
                  ctx.fillStyle = sceneCss(base);
                  ctx.fill();
                });
              }
              if (!fillGradient(ctx, look.grad, w, h)) {
                ctx.fillStyle = sceneCss(look.fill);
                ctx.fill();
              }
            } else if (shadow !== null) {
              ctx.fillStyle = sceneCss(look.fill);
              withShadow(ctx, shadow, () => ctx.fill());
            } else {
              ctx.fillStyle = sceneCss(look.fill);
              ctx.fill();
            }
            shadow = null;
          }
          if (look.lineW > 0 && look.border.a > 0) {
            const inset = look.lineW / 2;
            const radius = look.radius.map((r) => Math.max(r - inset, 0)) as [number, number, number, number];
            roundRectPath(ctx, inset, inset, w - look.lineW, h - look.lineW, radius);
            ctx.lineWidth = look.lineW;
            ctx.strokeStyle = sceneCss(look.border);
            if (shadow !== null) {
              withShadow(ctx, shadow, () => ctx.stroke());
            } else {
              ctx.stroke();
            }
          }
          const ti = prep.texts.get(rn);
          if (ti !== undefined) {
            drawText(ctx, ti, w, h, ln.kids.length === 0, look.text);
          }
        }
      }
    }
    for (const k of rn.kids) {
      const kn = prep.frames.get(k);
      if (k.kind === "item" && kn !== undefined) {
        drawNode(ctx, k, kn, false);
      } else if (k.kind === "link") {
        drawLink(ctx, prep, k, rn);
      }
    }
    ctx.restore();
  };

  return {
    width,
    height,
    bounds: { x: minX, y: 0 - maxY, width: maxX - minX, height: maxY - minY },
    diagnostics: diag.list,
    boxes,
    draw(ctx: Canvas2D, at: Point = { x: 0, y: 0 }): void {
      ctx.save();
      ctx.translate(at.x, at.y);
      ctx.scale(1, -1);
      drawNode(ctx, expanded, root, true);
      ctx.restore();
    },
  };
}

/** The expressions of every property of the screen, before the styles pick one by value (spec 5c, 6). */
function evalTree(rn: RNode, diag: Diagnostics): void {
  rn.props = rn.props.map((p) => (hasExpr(p) ? (evaluate(diag, rn, p) ?? p) : p));
  for (const k of rn.kids) {
    evalTree(k, diag);
  }
}

/** A link line between the boxes of its two ends, from edge to edge (go/scene/build.go link). */
function drawLink(ctx: Canvas2D, prep: Prep, lk: RNode, parent: RNode): void {
  const byName = new Map<string, RNode[]>();
  for (const k of parent.kids) {
    if (k.kind === "item") {
      const list = byName.get(k.name.normalize("NFC")) ?? [];
      list.push(k);
      byName.set(k.name.normalize("NFC"), list);
    }
  }
  const end = (name: string, prop: string): RNode | null => {
    const index = num(prep, lk, prop) ?? 0;
    const list = byName.get(name.normalize("NFC")) ?? [];
    return index >= 0 && index < list.length ? list[index]! : null;
  };
  const aNode = end(lk.from, "source-index");
  const bNode = end(lk.to, "target-index");
  if (aNode === null || bNode === null) {
    return;
  }
  const a = prep.frames.get(aNode);
  const b = prep.frames.get(bNode);
  if (a === undefined || b === undefined) {
    return;
  }
  const pa = { x: a.rx + a.rw / 2, y: a.ry + a.rh / 2 };
  const pb = { x: b.rx + b.rw / 2, y: b.ry + b.rh / 2 };
  const from = edgePoint(pa, a, pb);
  const to = edgePoint(pb, b, pa);
  const color = propOf(prep, lk, "color");
  const col = color !== null ? (sceneColor(propText(color, 0)) ?? colorBlack) : colorBlack;
  const width = num(prep, lk, "line-width") ?? 1;
  ctx.strokeStyle = sceneCss(col);
  ctx.lineWidth = width;
  ctx.beginPath();
  ctx.moveTo(from.x, from.y);
  ctx.lineTo(to.x, to.y);
  ctx.stroke();
  const display = propOf(prep, lk, "display");
  if (display !== null && propText(display, 0) === "arrow") {
    // go/scene/build.go arrowHead: a triangle of 6 + line width at the target end.
    const size = 6 + width;
    const dx = to.x - from.x;
    const dy = to.y - from.y;
    const len = Math.hypot(dx, dy) || 1;
    const bx = to.x - (dx / len) * size;
    const by = to.y - (dy / len) * size;
    const nx = (-dy / len) * (size / 2);
    const ny = (dx / len) * (size / 2);
    ctx.beginPath();
    ctx.moveTo(to.x, to.y);
    ctx.lineTo(bx + nx, by + ny);
    ctx.lineTo(bx - nx, by - ny);
    ctx.closePath();
    ctx.fillStyle = sceneCss(col);
    ctx.fill();
  }
}

function edgePoint(center: Point, n: LayoutNode, other: Point): Point {
  const dx = other.x - center.x;
  const dy = other.y - center.y;
  let t = Infinity;
  if (dx !== 0) {
    t = Math.min(t, n.rw / 2 / Math.abs(dx));
  }
  if (dy !== 0) {
    t = Math.min(t, n.rh / 2 / Math.abs(dy));
  }
  t = Math.min(t, 1);
  return { x: center.x + dx * t, y: center.y + dy * t };
}
