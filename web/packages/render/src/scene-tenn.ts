// Scene renderer for the scene-graph part of .tenn (docs/spec/tenn-scene.md 4, 4.2, 4.7/4.7a, 5b, 5c, 13).
// A port of go/scene: the same expansion of use/at/replace/slots, the same style cascade, the same flex
// layout (gfx/layout) and the same text estimate, so a file draws what go/cmd/tenn-render draws.
//
// The input is the model (@tenniarb/core): an element keeps its commands, an item keeps its block, so the
// scene tree is rebuilt here instead of in the editor, the viewer and the exporter separately.
//
// Not ported yet: transitions, CSS @keyframes of the animations (the still picture is their first frame, as
// in Go). element --from-file and template are ported; the calling side supplies the files (SceneLoadOptions).
import { TennParser, newCommand, newFloatNode, parseTenn } from "@tenniarb/core";
import type { DiagramItem, Element, IExecutionContext, LinkItem, TennNode } from "@tenniarb/core";
import { MarkdownLexer, getMaxRect } from "@tenniarb/markdown";
import type { Canvas2D } from "./canvas-types.ts";
import type { ImageDecoder } from "./images.ts";
import { parseColor } from "./color.ts";
import type { Color } from "./color.ts";
import { canvasQuirks, shadowScale } from "./drawable.ts";
import { createExecutionContext } from "./execution.ts";
import type { Point, Rect } from "./geometry.ts";
import { jsToString } from "./style.ts";

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
  /** The command that wrote the property, for the position of a diagnostic about it. */
  pos?: DiagPos;
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

/** The code an element generates (spec 5c): text with ${name} and <slot/> filled in by renderTemplate. */
export interface SceneTemplate {
  text: string;
  line: number;
  col: number;
}

/** What the calling side gives the renderer to follow `element --from-file` (spec 3a) and read templates. */
export interface SceneLoadOptions {
  /** The slash path of the file `element` was read from, relative to the workspace root; "" by default. */
  file?: string;
  /** Reads a file by slash path relative to the workspace root; null when it cannot be read. */
  readFile?: (path: string) => string | null;
  /** The text of the file `element` was read from, for the exact `template { ... }` and `%{ ... %}` text. */
  source?: string;
  /**
   * The classic item renderer's evaluator: a classic element reads `$(...)`/`${...}` through it, so the
   * values are the ones renderElement would draw. Built for the element here when absent.
   */
  context?: IExecutionContext | null;
  /**
   * Decodes the base64 of a classic inline image into something Canvas2D.drawImage takes; without it the
   * picture still sizes the box and the text, but nothing is drawn.
   */
  decodeImage?: ImageDecoder;
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
  file: string; // the slash path of the file the node is written in
  src: string | null; // the text of that file
  model: Element | null; // the model element the node was read from
  item: DiagramItem | null; // the model item or link the node was read from
  template: SceneTemplate | null; // element: the code it generates (spec 5c)
  mount: SceneNode | null; // element with --from-file: the root of the file that holds its content
  mountAt: SceneNode | null; // file root below a mount: the element that contains the mounting one
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
    file: parent?.file ?? "",
    src: parent?.src ?? null,
    model: null,
    item: null,
    template: null,
    mount: null,
    mountAt: null,
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
  const line = (c.token?.line ?? 0) + 1;
  return { name: cmdName(c), args, block: last !== undefined && hasBlock(c) ? last : null, pos: { line, col: c.token?.col ?? 1 } };
}

const holdsContent = (n: SceneNode): boolean => n.kind === "file" || n.kind === "element" || n.kind === "item" || n.kind === "slot";
const isContainer = (n: SceneNode): boolean => n.kind === "file" || n.kind === "element";

/** What a path segment may pick under n: elements, items and uses. A mounted element is its file. */
function named(n: SceneNode): SceneNode[] {
  const src = n.mount ?? n;
  const out = [...src.elems];
  for (const c of src.content) {
    if (c.kind === "item" || c.kind === "use") {
      out.push(c);
    }
  }
  return out;
}

interface DiagPos {
  line: number;
  col: number;
  file?: string;
}

class Diagnostics {
  readonly list: { message: string; warn: boolean; line: number; col: number; file: string }[] = [];

  fail(at: DiagPos, message: string): void {
    this.list.push({ message, warn: false, line: at.line, col: at.col, file: at.file ?? "" });
  }

  warn(at: DiagPos, message: string): void {
    this.list.push({ message, warn: true, line: at.line, col: at.col, file: at.file ?? "" });
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
    if (name === "template" && owner.kind === "element") {
      addTemplate(owner, c, diag);
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
  node.item = item;
  if (item.kind === "Link") {
    const link = item as LinkItem;
    node.from = link.source?.name ?? "";
    node.to = link.target?.name ?? "";
  }
  owner.content.push(node);
  fill(node, itemCommands(item), diag);
}

/** The commands of an element as the file wrote them: items and properties mixed, in text order (spec 9.3). */
function orderedEntries(el: Element): (TennNode | DiagramItem)[] {
  const entries: (TennNode | DiagramItem)[] = [...el.properties, ...el.items];
  const order = new Map<TennNode | DiagramItem, number>();
  for (const e of entries) {
    order.set(e, "token" in e ? ((e as TennNode).token?.pos ?? Number.MAX_SAFE_INTEGER) : ((e as DiagramItem).sourcePos ?? Number.MAX_SAFE_INTEGER));
  }
  entries.sort((a, b) => (order.get(a) ?? 0) - (order.get(b) ?? 0));
  return entries;
}

/** "element <name> --from-file <path>": the path as written, or null. */
function fromFileOf(extraArgs: TennNode[]): string | null {
  for (let i = 0; i < extraArgs.length; i += 1) {
    if (argText(extraArgs[i]) === "--from-file" && i + 1 < extraArgs.length) {
      return argText(extraArgs[i + 1]);
    }
  }
  return null;
}

/** The position of the command that holds token c in its file, 1-based, as go/scene reports it. */
function commandPos(file: string, src: string | null, c: TennNode): DiagPos {
  const t = c.token;
  const line = (t?.line ?? 0) + 1;
  if (t === null || src === null) {
    return { file, line, col: 1 };
  }
  // The model keeps no position for the element command: walk back to the start of its line instead.
  let i = Math.min(t.pos, src.length);
  while (i > 0 && src[i - 1] !== "\n") {
    i -= 1;
  }
  let col = 1;
  while (i < t.pos && (src[i] === " " || src[i] === "\t")) {
    i += 1;
    col += 1;
  }
  return { file, line, col };
}

const posText = (at: DiagPos): string => `${at.file !== undefined && at.file !== "" ? `${at.file}:` : ""}${at.line}:${at.col}`;

/** Join a path relative to the file in dir; "../" beyond the workspace root gives null (go/scene inside). */
function resolvePath(file: string, rel: string): string | null {
  const parts = [...(file === "" ? [] : file.split("/").slice(0, -1)), ...rel.split("/")];
  const out: string[] = [];
  for (const part of parts) {
    if (part === "" || part === ".") {
      continue;
    }
    if (part === "..") {
      if (out.length === 0) {
        return null;
      }
      out.pop();
      continue;
    }
    out.push(part);
  }
  return out.join("/");
}

interface LoadedFile {
  path: string;
  src: string;
  model: Element;
}

/** Reads the files `element --from-file` names; a file mounted twice is read once, a cycle is an error. */
class Loader {
  readonly diag: Diagnostics;
  readonly main: string;
  private readonly reader: ((path: string) => string | null) | null;
  private readonly files = new Map<string, LoadedFile>();
  private readonly stack: { path: string; at: DiagPos }[] = [];

  constructor(diag: Diagnostics, options: SceneLoadOptions) {
    this.diag = diag;
    this.main = options.file ?? "";
    this.reader = options.readFile ?? null;
    // The entry document is on the stack, so a cycle that comes back to it reports the whole way round.
    this.stack.push({ path: this.main, at: { file: this.main, line: 1, col: 1 } });
  }

  /** Pushes the file being loaded; false when it is already loading, which is the cycle. */
  enter(path: string, at: DiagPos): boolean {
    const i = this.stack.findIndex((f) => f.path === path);
    if (i < 0) {
      this.stack.push({ path, at });
      return true;
    }
    const hops: string[] = [];
    for (let k = i + 1; k < this.stack.length; k += 1) {
      hops.push(`${posText(this.stack[k]!.at)} imports ${this.stack[k]!.path}`);
    }
    hops.push(`${posText(at)} imports ${path}`);
    this.diag.fail(at, `import cycle: ${hops.join(", ")}`);
    return false;
  }

  leave(): void {
    this.stack.pop();
  }

  /** The file at path, from the cache, the loader, or the parse; null when it cannot be read. */
  read(path: string, at: DiagPos): LoadedFile | null {
    const cached = this.files.get(path);
    if (cached !== undefined) {
      return cached;
    }
    const raw = this.reader === null ? null : this.reader(path);
    if (raw === null) {
      this.diag.fail(at, `cannot read ${path}`);
      return null;
    }
    const parser = new TennParser();
    const tree = parser.parse(raw.charCodeAt(0) === 0xfeff ? raw.slice(1) : raw);
    for (const e of parser.errors.errors) {
      this.diag.fail({ file: path, line: e.line + 1, col: e.col }, e.message);
    }
    const loaded: LoadedFile = { path, src: raw, model: parseTenn(tree) };
    this.files.set(path, loaded);
    return loaded;
  }
}

function addElement(owner: SceneNode, el: Element, loader: Loader, diag: Diagnostics, seen: Set<Element>): void {
  if (seen.has(el)) {
    return;
  }
  seen.add(el);
  const node = newNode("element", el.name, owner, null);
  node.model = el;
  owner.elems.push(node);
  const rel = fromFileOf(el.extraArgs);
  if (rel !== null) {
    const at = commandPos(node.file, node.src, el.extraArgs[0]!);
    if (el.properties.count > 0 || el.items.length > 0 || el.elements.length > 0) {
      diag.fail(at, `element "${el.name}" takes its content from ${rel}: remove the block`);
    }
    const path = resolvePath(node.file, rel);
    if (path === null) {
      diag.fail(at, `element --from-file "${rel}" leaves the workspace root`);
      return;
    }
    if (!loader.enter(path, at)) {
      return;
    }
    const loaded = loader.read(path, at);
    if (loaded === null) {
      loader.leave();
      return;
    }
    // mountFile: the content, the elements and the type of the file root, the scope inside it its own file.
    const file = newNode("file", "", null, null);
    file.file = loaded.path;
    file.src = loaded.src;
    file.mountAt = owner;
    fillMounted(file, loaded.model, diag);
    for (const child of loaded.model.elements) {
      addElement(file, child, loader, diag, seen);
    }
    loader.leave();
    node.mount = file;
    if (node.type === "") {
      node.type = file.type;
    }
    return;
  }
  fill(node, orderedEntries(el), diag);
  for (const child of el.elements) {
    addElement(node, child, loader, diag, seen);
  }
}

/** The content of a mounted file root: what Go's mountFile copies into the mounting element. */
function fillMounted(owner: SceneNode, el: Element, diag: Diagnostics): void {
  for (const entry of orderedEntries(el)) {
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
    if (name === "kind") {
      owner.type = argText(splitCommand(c).args[0]).toLowerCase();
      continue;
    }
    if (!addContent(owner, c, name, diag)) {
      owner.props.push(toProp(c));
    }
  }
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

/** "template { ... }", "template %{ ... }" or "template \"...\"": the code the element generates (spec 5c). */
function addTemplate(owner: SceneNode, c: TennNode, diag: Diagnostics): void {
  if (owner.template !== null) {
    diag.warn(pos(c), "the element has a second template: the first one is used");
    return;
  }
  const { args } = splitCommand(c);
  let text = "";
  let form = false;
  if (hasBlock(c)) {
    form = true;
    const block = c.children![c.children!.length - 1]!;
    const src = owner.src;
    // shortcut: without the text of the file a block template reads as empty; renderTenn passes source.
    if (src !== null && block.token !== null) {
      // The bytes between the braces, as Go's Range does; the block's own tokens end before the closing brace.
      let end = block.token.pos + block.token.size;
      for (const child of block.children ?? []) {
        end = Math.max(end, nodeEnd(child));
      }
      let close = end;
      while (close < src.length && (src[close] === " " || src[close] === "\t" || src[close] === "\n" || src[close] === "\r")) {
        close += 1;
      }
      if (src[close] === "}") {
        text = src.slice(block.token.pos + 1, close);
      }
    }
  } else if (args.length > 0 && (args[0]!.kind === "MarkdownLit" || args[0]!.kind === "StringLit")) {
    form = true;
    const a = args[0]!;
    text = argText(a);
    // "%{ ... %}" is what a reader writes: the markdown literal closes at "}", so the percent of the
    // closing marker lands in the text and is cut here (go/scene addTemplate).
    if (text.endsWith("%")) {
      // shortcut: without the file text the closing character cannot be read, so a trailing percent is
      // taken for the marker; pass SceneLoadOptions.source for the exact Go check.
      const src = owner.src;
      if (a.token === null || src === null || src[a.token.pos + text.length] === "}") {
        text = text.slice(0, -1);
      }
    }
  }
  if (!form) {
    diag.fail(pos(c), "template needs text: template { ... } or template %{ ... }");
    return;
  }
  const p = pos(c);
  owner.template = { text: dedent(text), line: p.line, col: p.col };
}

/** The end of a command's text: its last token, blocks included. */
function nodeEnd(n: TennNode): number {
  let end = n.token === null ? 0 : n.token.pos + n.token.size;
  for (const c of n.children ?? []) {
    end = Math.max(end, nodeEnd(c));
  }
  return end;
}

/** Cut the blank lines around a template and the indentation its lines share, as go/scene dedent does. */
function dedent(text: string): string {
  const lines = text.replace(/^\n+|\n+$/g, "").split("\n");
  let indent = -1;
  for (const ln of lines) {
    if (ln.trim() === "") {
      continue;
    }
    const n = ln.length - ln.replace(/^[ \t]+/, "").length;
    indent = indent < 0 || n < indent ? n : indent;
  }
  const cut = indent < 0 ? 0 : indent;
  const out = lines.map((ln) => (ln.trim() === "" ? "" : ln.slice(cut)));
  while (out.length > 0 && out[0] === "") {
    out.shift();
  }
  while (out.length > 0 && out[out.length - 1] === "") {
    out.pop();
  }
  return out.join("\n");
}

// MARK: - Resolving a path (spec 4.2/4.3, go/scene/resolve.go)

/** The next scope above a: its parent, or from the root of a mounted file the place it is mounted at. */
function outward(a: SceneNode): SceneNode | null {
  return a.parent !== null ? a.parent : a.mountAt;
}

function lookup(scope: SceneNode | null, path: string): { node: SceneNode | null; message: string } {
  const segs = splitPath(path);
  if (segs === null) {
    return { node: null, message: `bad path "${path}"` };
  }
  for (let a = scope; a !== null; a = outward(a)) {
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

const cloneProp = (p: SceneProp): SceneProp => ({ name: p.name, args: p.args, block: p.block, val: p.val, pos: p.pos });

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
    // A mounted element is its file: the file root's properties and content (go/scene container + mountFile).
    if (n.mount !== null) {
      r.props = n.mount.props.map(cloneProp);
    }
    r.kids = this.contents((n.mount ?? n).content);
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
    const caller = this.vars;
    // An item with parameters drawn in place is an instance that overrides nothing (spec 5c).
    if (n.params.length > 0) {
      this.vars = this.defaults(n);
    }
    const r = this.itemBody(n);
    this.vars = caller;
    this.leave();
    return r;
  }

  private itemBody(n: SceneNode): RNode {
    const r = rnode("item", n, n.name, this.vars);
    r.kids = this.contents(n.content);
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
      case "item": {
        // not item(): the values of the use stay, the defaults of the declaration are not re-applied
        if (!this.enter(target)) {
          return null;
        }
        const r = this.itemBody(target);
        this.leave();
        return r;
      }
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

/** The number of argument i of a command read outside a prop. */
const argNum = (args: TennNode[], i: number): number | null =>
  args[i] === undefined ? null : propNum({ name: "", args, block: null }, i);

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

/**
 * A classic diagram's `styles { item … }` and `styles { line … }` are the look of every box and every
 * link of the element, not a style a node names: go/render read those two names that way.
 */
function classicDefaults(r: RNode): void {
  if (r.src === null) {
    return;
  }
  const item = r.src.styles.get("item");
  const line = r.src.styles.get("line");
  if (item === undefined && line === undefined) {
    return;
  }
  const walk = (n: RNode): void => {
    if (n.kind === "item") {
      n.props = [...defaultProps("item", item, n), ...n.props];
    } else if (n.kind === "link") {
      n.props = [...defaultProps("line", line, n), ...n.props];
    }
    for (const k of n.kids) {
      walk(k);
    }
  };
  for (const k of r.kids) {
    walk(k);
  }
}

/** The properties a default style gives n; a shadow of n's own replaces the default one. */
function defaultProps(name: string, st: TennStyle | undefined, n: RNode): SceneProp[] {
  if (st === undefined || styleRefs(n.props).includes(normalizeName(name))) {
    return [];
  }
  const ownShadow = n.props.some((p) => p.name === "shadow");
  return st.props.filter((p) => !(ownShadow && p.name === "shadow"));
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
function keyframes(rn: RNode, p: SceneProp, diag: Diagnostics): Animation | null {
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
        const fp = toProp(pc);
        // A frame property may be an expression, evaluated in the scope of the node the block is written on.
        const evaluated = hasExpr(fp) ? evaluate(diag, rn, fp) : fp;
        if (evaluated !== null) {
          f.props.push(evaluated);
        }
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
    const a = keyframes(rn, p, diag);
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

// MARK: - Inline images (go/scene/image.go)

/** A classic `image "name" @(base64)`: the picture of the box or the one a title or body names. */
interface InlineImage {
  data: string; // the base64 as written, the key of the decode cache
  w: number;
  h: number;
}

/** The box an @(name|width) reference asks for: the picture and the size the layout draws it at. */
interface InlineRef {
  im: InlineImage;
  w: number;
  h: number;
}

/** inlineImage: a classic `image "name" @(data)`, not the `image "path"` of a scene. */
function inlineImage(pr: SceneProp | null): pr is SceneProp {
  return pr !== null && pr.val === undefined && pr.args.length >= 2 && pr.args[1]!.kind === "Image";
}

const inlineBase64 = (data: string): string => data.replace(/[^A-Za-z0-9+/=]/g, "");

const base64Digits = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

/** Go's decodeBase64: the payload of an inline image, unknown characters dropped and the padding ignored. */
function decodeInlineBase64(data: string): Uint8Array | null {
  const clean = inlineBase64(data).replace(/=+$/, "");
  if (clean.length % 4 === 1) {
    return null;
  }
  const out = new Uint8Array(Math.floor((clean.length * 3) / 4));
  let bits = 0;
  let n = 0;
  let o = 0;
  for (const ch of clean) {
    const v = base64Digits.indexOf(ch);
    if (v < 0) {
      return null;
    }
    bits = (bits << 6) | v;
    n += 6;
    if (n >= 8) {
      n -= 8;
      out[o] = (bits >> n) & 0xff;
      o += 1;
      bits &= (1 << n) - 1;
    }
  }
  return out.subarray(0, o);
}

const be32 = (b: Uint8Array, i: number): number => (b[i]! << 24) | (b[i + 1]! << 16) | (b[i + 2]! << 8) | b[i + 3]!;
const le16 = (b: Uint8Array, i: number): number => b[i]! | (b[i + 1]! << 8);
const le24 = (b: Uint8Array, i: number): number => b[i]! | (b[i + 1]! << 8) | (b[i + 2]! << 16);

/** The pixel size of a PNG, JPEG, GIF or WebP, as Go's image.DecodeConfig reads it. */
function imageSize(raw: Uint8Array): { w: number; h: number } | null {
  const ascii = (at: number, len: number): string => {
    let s = "";
    for (let i = 0; i < len; i += 1) {
      s += String.fromCharCode(raw[at + i] ?? 0);
    }
    return s;
  };
  if (raw.length >= 24 && raw[0] === 0x89 && ascii(1, 3) === "PNG") {
    const w = be32(raw, 16);
    const h = be32(raw, 20);
    return w > 0 && h > 0 ? { w, h } : null;
  }
  if (raw.length >= 10 && (ascii(0, 6) === "GIF87a" || ascii(0, 6) === "GIF89a")) {
    const w = le16(raw, 6);
    const h = le16(raw, 8);
    return w > 0 && h > 0 ? { w, h } : null;
  }
  if (raw.length >= 4 && raw[0] === 0xff && raw[1] === 0xd8) {
    for (let i = 2; i + 9 < raw.length; ) {
      if (raw[i] !== 0xff) {
        i += 1;
        continue;
      }
      const marker = raw[i + 1]!;
      if (marker === 0xd8 || marker === 0x01 || (marker >= 0xd0 && marker <= 0xd7)) {
        i += 2;
        continue;
      }
      const len = (raw[i + 2]! << 8) | raw[i + 3]!;
      if (len < 2) {
        return null;
      }
      if (marker >= 0xc0 && marker <= 0xcf && marker !== 0xc4 && marker !== 0xc8 && marker !== 0xcc) {
        const h = (raw[i + 5]! << 8) | raw[i + 6]!;
        const w = (raw[i + 7]! << 8) | raw[i + 8]!;
        return w > 0 && h > 0 ? { w, h } : null;
      }
      i += 2 + len;
    }
    return null;
  }
  if (raw.length >= 30 && ascii(0, 4) === "RIFF" && ascii(8, 4) === "WEBP") {
    switch (ascii(12, 4)) {
      case "VP8 ":
        return { w: le16(raw, 26) & 0x3fff, h: le16(raw, 28) & 0x3fff };
      case "VP8L": {
        const bits = raw[21]! | (raw[22]! << 8) | (raw[23]! << 16) | (raw[24]! << 24);
        return { w: (bits & 0x3fff) + 1, h: ((bits >> 14) & 0x3fff) + 1 };
      }
      case "VP8X":
        return { w: le24(raw, 24) + 1, h: le24(raw, 27) + 1 };
    }
  }
  return null;
}

/** inlineConfig: the size of an inline picture, or why the data is not one. */
function inlineConfig(data: string): { w: number; h: number } | { why: string } {
  const raw = decodeInlineBase64(data);
  if (raw === null) {
    return { why: "the data is not base64" };
  }
  const size = imageSize(raw);
  return size ?? { why: "the file is not a PNG, JPEG, GIF or WebP image" };
}

/** Decoded pictures kept between builds, at 4 bytes a pixel, keyed by their base64 content; least recently used go first. */
// shortcut: a page above 64 MiB of pixels decodes again on the next build; Go keeps 512 MiB.
const inlineCacheBytes = 64 << 20;

const inlineCaches = new WeakMap<ImageDecoder, Map<string, { image: unknown; bytes: number }>>();

/** decodeInline: the picture behind a key of the cache, decoded once per content and decoder. */
function inlineRaster(prep: Prep, im: InlineImage): unknown | null {
  const decode = prep.decodeImage;
  if (decode === null) {
    return null;
  }
  let cache = inlineCaches.get(decode);
  if (cache === undefined) {
    cache = new Map();
    inlineCaches.set(decode, cache);
  }
  const key = inlineBase64(im.data);
  const hit = cache.get(key);
  if (hit !== undefined) {
    cache.delete(key);
    cache.set(key, hit);
    return hit.image;
  }
  let decoded;
  try {
    decoded = decode(key);
  } catch {
    decoded = null;
  }
  if (decoded === null) {
    return null;
  }
  const bytes = 4 * im.w * im.h;
  if (bytes <= inlineCacheBytes) {
    cache.set(key, { image: decoded.image, bytes });
    let total = 0;
    for (const e of cache.values()) {
      total += e.bytes;
    }
    for (const [k, e] of cache) {
      if (total <= inlineCacheBytes) {
        break;
      }
      cache.delete(k);
      total -= e.bytes;
    }
  }
  return decoded.image;
}

/** inlineImages: the `image "name" @(data)` of rn by name, decoded as far as the header. */
function inlineImages(prep: Prep, rn: RNode): Map<string, InlineImage> {
  const cached = prep.inlines.get(rn);
  if (cached !== undefined) {
    return cached;
  }
  const imgs = new Map<string, InlineImage>();
  prep.inlines.set(rn, imgs);
  if (!prep.classic) {
    return imgs;
  }
  for (const pr of rn.props) {
    if (!inlineImage(pr)) {
      continue;
    }
    const name = propText(pr, 0);
    if (name === "") {
      prep.diag.warn(pr.pos ?? rn.src ?? { line: 0, col: 0 }, `image takes a name: image "photo" @(...)`);
      continue;
    }
    const data = propText(pr, 1);
    const size = inlineConfig(data);
    if ("why" in size) {
      prep.diag.warn(pr.pos ?? rn.src ?? { line: 0, col: 0 }, `image "${name}": ${size.why}, the item is drawn empty`);
      continue;
    }
    imgs.set(name, { data, w: size.w, h: size.h });
  }
  return imgs;
}

/** The number of a `@(name|width)` style, as Swift's Int(_:radix: 10). */
function inlineInt(s: string): number | null {
  return /^[+-]?[0-9]+$/.test(s) ? Number(s) : null;
}

/** resolveInline: the picture the name of a reference selects, with the box @(name|width) asks for. */
function resolveInline(imgs: Map<string, InlineImage>, ref: string): InlineRef | null {
  const bar = ref.indexOf("|");
  const name = bar < 0 ? ref : ref.slice(0, bar);
  const im = imgs.get(name);
  if (im === undefined) {
    return null;
  }
  let w = im.w;
  let h = im.h;
  if (bar >= 0) {
    const style = ref.slice(bar + 1);
    let widthStr = style;
    let heightStr = "";
    const x = style.indexOf("x");
    if (x >= 0) {
      widthStr = style.slice(0, x).trim();
      heightStr = style.slice(x + 1).trim();
    }
    const width = inlineInt(widthStr);
    if (width !== null) {
      w = width;
    }
    const height = inlineInt(heightStr);
    if (height !== null) {
      h = height;
    }
    // A style without a width or a height keeps the aspect, and never grows the picture.
    if (widthStr === "" || heightStr === "") {
      const r = getMaxRect(w, h, im.w, im.h);
      w = r.width;
      h = r.height;
    }
  }
  return { im, w, h };
}

/** namesInline: a title, a body or the file the body reads refers to the inline image name. */
function namesInline(prep: Prep, rn: RNode, name: string): boolean {
  if (name === "") {
    return false;
  }
  const ref = `@(${name}`;
  for (const propName of ["title", "body"]) {
    const pr = prop(rn, propName);
    if (pr !== null && propRefers(pr, ref)) {
      return true;
    }
  }
  const body = prop(rn, "body");
  if (body !== null) {
    const file = bodyFileArg(body.args);
    if (file !== null) {
      const text = bodyFileText(prep, rn, file, body.pos ?? rn.src ?? { line: 0, col: 0 }, true);
      if (text !== null && text.includes(ref)) {
        return true;
      }
    }
  }
  return false;
}

/** propRefers: the text of a title or body names ref, in an argument, a value or a block. */
function propRefers(pr: SceneProp, ref: string): boolean {
  const nodesRef = (nodes: TennNode[]): boolean => nodes.some((n) => (n.getIdentText() ?? "").includes(ref));
  if ((pr.val ?? []).some((v) => v.includes(ref))) {
    return true;
  }
  if (nodesRef(pr.args)) {
    return true;
  }
  if (pr.block !== null) {
    for (const c of pr.block.children ?? []) {
      if (nodesRef(splitCommand(c).args)) {
        return true;
      }
    }
  }
  return false;
}

/** parenAt: a !(&/@ marker at i with a balanced (...) argument, the index after the closing one. */
function parenAt(s: string, i: number): { mark: string; arg: string; next: number } | null {
  const mark = s[i];
  if (i + 1 >= s.length || s[i + 1] !== "(" || (mark !== "!" && mark !== "&" && mark !== "@")) {
    return null;
  }
  let depth = 1;
  for (let j = i + 2; j < s.length; j += 1) {
    if (s[j] === "(") {
      depth += 1;
    } else if (s[j] === ")") {
      depth -= 1;
      if (depth === 0) {
        return { mark, arg: s.slice(i + 2, j), next: j + 1 };
      }
    }
  }
  return null;
}

/**
 * A classic title cut at the !(colour) and &(size) markup of the markdown, the colours and sizes
 * go/render gives the runs of a title; null when the title writes neither, so its text stays as written.
 */
function titleMarkupParts(prep: Prep, rn: RNode, ti: TextInfo): TitlePart[] | null {
  const title = ti.text;
  if (!title.includes("!(") && !title.includes("&(")) {
    return null;
  }
  const imgs = inlineImages(prep, rn);
  let color: Color | undefined;
  let font: Font | undefined;
  const parts: TitlePart[] = [];
  let plain = "";
  const flush = (): void => {
    if (plain === "") {
      return;
    }
    parts.push({ text: plain, img: null, font, color });
    plain = "";
  };
  for (let i = 0; i < title.length; ) {
    if (title[i] === "\\" && i + 1 < title.length && "@$*_#<~!".includes(title[i + 1]!)) {
      plain += title[i + 1];
      i += 2;
      continue;
    }
    const m = parenAt(title, i);
    if (m === null) {
      plain += title[i];
      i += 1;
      continue;
    }
    if (m.mark === "!") {
      flush();
      const bar = m.arg.indexOf("|");
      if (bar >= 0) {
        const c = sceneColor(m.arg.slice(0, bar));
        if (c !== null) {
          parts.push({ text: m.arg.slice(bar + 1), img: null, font, color: c });
        }
      } else if (m.arg.trim() === "") {
        color = undefined;
      } else {
        color = sceneColor(m.arg) ?? undefined;
        if (color === undefined) {
          prep.diag.warn(rn.src ?? { line: 0, col: 0 }, `!(${m.arg}): no such color, the default is used`);
        }
      }
    } else if (m.mark === "&") {
      flush();
      const bar = m.arg.indexOf("|");
      if (bar >= 0) {
        const v = markupSize(m.arg.slice(0, bar));
        if (v !== null) {
          parts.push({ text: m.arg.slice(bar + 1), img: null, font: { ...(font ?? ti.font), size: v }, color });
        }
      } else {
        const v = markupSize(m.arg);
        font = v === null ? undefined : { ...ti.font, size: v };
      }
    } else {
      const img = resolveInline(imgs, m.arg);
      if (img !== null) {
        flush();
        parts.push({ text: "", img, font, color });
      } else {
        plain += title.slice(i, m.next);
      }
    }
    i = m.next;
  }
  flush();
  return parts;
}

/** splitTitleImages: a title cut at the inline pictures it names; null when it names none. */
function splitTitleImages(title: string, imgs: Map<string, InlineImage>): TitlePart[] | null {
  if (imgs.size === 0) {
    return null;
  }
  const parts: TitlePart[] = [];
  let plain = 0;
  let found = false;
  let i = 0;
  while (i + 1 < title.length) {
    if (title[i] !== "@" || title[i + 1] !== "(") {
      i += 1;
      continue;
    }
    const j = title.indexOf(")", i + 2);
    if (j < 0) {
      break;
    }
    const img = resolveInline(imgs, title.slice(i + 2, j));
    if (img === null) {
      i += 2;
      continue;
    }
    if (i > plain) {
      parts.push({ text: title.slice(plain, i), img: null });
    }
    parts.push({ text: "", img });
    i = j + 1;
    plain = i;
    found = true;
  }
  if (!found) {
    return null;
  }
  if (plain < title.length) {
    parts.push({ text: title.slice(plain), img: null });
  }
  return parts;
}

/** The picture an item draws as its content: the inline image no title or body names. */
function inlineBoxImage(prep: Prep, rn: RNode): InlineImage | null {
  if (!prep.classic) {
    return null;
  }
  const pr = prop(rn, "image");
  if (!inlineImage(pr)) {
    return null;
  }
  const name = propText(pr, 0);
  const im = inlineImages(prep, rn).get(name);
  if (im === undefined || namesInline(prep, rn, name)) {
    return null;
  }
  return im;
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

/**
 * A span of a title: text in the item font or in a size of its own, or an inline picture of a classic
 * diagram. `color` marks a run the !(...) markup coloured; without it the run takes the text colour of
 * the item, which is only known when the title is drawn.
 */
interface TitlePart {
  text: string;
  img: InlineRef | null;
  font?: Font;
  color?: Color;
}

/** One laid out line of a title; h grows above lineH for a larger run or an inline picture. */
interface TitleLine {
  parts: TitlePart[];
  w: number;
  h: number;
  asc: number;
  desc: number;
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
  /** Set when the text names an inline picture; null draws the text as it is. */
  parts: TitlePart[] | null;
  prep: Prep;
}

/** gfx.EstimateText: 0.6 em per rune, ascent 0.8 em, descent 0.25 em. */
const fontWidth = (f: Font, s: string): number => [...s].length * (f.size * 0.6 + f.spacing);
const textWidth = (ti: TextInfo, s: string): number => fontWidth(ti.font, s);

/** The font of a part, the title font for a part that names none. */
const partFont = (ti: TextInfo, part: TitlePart): Font => part.font ?? ti.font;

/** The advance of a part, its picture or its text in its own font. */
const partWidth = (ti: TextInfo, part: TitlePart): number =>
  part.img !== null ? part.img.w : fontWidth(partFont(ti, part), part.text);

/** The cap height of SF over the font size, as go/render hangs an inline picture from the line. */
const capHeightRatio = 1490 / 2048;

function textNatural(ti: TextInfo): number {
  if (ti.parts !== null) {
    let w = 0;
    for (const ln of titleLayout(ti, Infinity)) {
      w = Math.max(w, ln.w);
    }
    return w;
  }
  let w = 0;
  for (const ln of ti.text.split("\n")) {
    w = Math.max(w, textWidth(ti, ln));
  }
  return w;
}

/** The height of the title at width w; an inline picture is taller than its line of text. */
function titleHeight(ti: TextInfo, w: number): number {
  if (ti.parts !== null) {
    let h = 0;
    for (const ln of titleLayout(ti, w)) {
      h += ln.h;
    }
    return h;
  }
  return textWrap(ti, w).length * ti.lineH;
}

/** The longest prefix of s that fits avail, in whole characters; "" when not even one does. */
function titleFitPrefix(ti: TextInfo, s: string, f: Font, avail: number): string {
  if (avail <= 0) {
    return "";
  }
  let cut = "";
  for (const g of s) {
    if (fontWidth(f, cut + g) > avail + 1e-9) {
      break;
    }
    cut += g;
  }
  return cut;
}

/** Go's textInfo.layout: the title broken at the references, an inline picture counting as one box. */
function titleLayout(ti: TextInfo, w: number): TitleLine[] {
  const lines: TitleLine[] = [];
  const empty = (): TitleLine => ({ parts: [], w: 0, h: ti.lineH, asc: ti.asc, desc: ti.desc });
  let line = empty();
  let started = false;
  const flush = (): void => {
    lines.push(line);
    line = empty();
    started = false;
  };
  const add = (part: TitlePart): void => {
    line.parts.push(part);
    if (part.img !== null) {
      line.w += part.img.w;
      line.h = Math.max(line.h, part.img.h);
      started = true;
      return;
    }
    const f = partFont(ti, part);
    line.w += fontWidth(f, part.text);
    if (f !== ti.font) {
      const box = lineBox(ti.prep, f.size);
      line.asc = Math.max(line.asc, box.asc);
      line.desc = Math.max(line.desc, box.desc);
      line.h = Math.max(line.h, box.h);
    }
    started = started || part.text !== "";
  };
  for (const part of ti.parts ?? []) {
    if (part.img !== null) {
      if (started && w > 0 && line.w + part.img.w > w + 1e-9) {
        flush();
      }
      add(part);
      continue;
    }
    const f = partFont(ti, part);
    part.text.split("\n").forEach((para, pi) => {
      if (pi > 0) {
        flush();
      }
      para.split(" ").forEach((word, i) => {
        hyphenPieces(word).forEach((piece, j) => {
          let text = piece;
          if (j === 0 && i > 0) {
            text = ` ${piece}`;
          }
          if (text === "" || (!started && text.trim() === "")) {
            return;
          }
          if (started && text.trim() !== "" && w > 0 && line.w + fontWidth(f, text) > w + 1e-9) {
            flush();
            text = piece;
          }
          // A word wider than the line is broken by characters in a classic diagram, as go/render drew a
          // title: the box grows down instead of running over its width.
          if (ti.prep.classic && w > 0 && text.trim() !== "") {
            while (text !== "") {
              if (fontWidth(f, text) <= w - line.w + 1e-9) {
                add({ text, img: null, font: part.font, color: part.color });
                break;
              }
              let cut = titleFitPrefix(ti, text, f, w - line.w);
              if (cut === "") {
                if (!started) {
                  cut = [...text][0] ?? "";
                } else {
                  flush();
                  continue;
                }
              }
              add({ text: cut, img: null, font: part.font, color: part.color });
              text = text.slice(cut.length);
              flush();
            }
            return;
          }
          if (text !== "") {
            add({ text, img: null, font: part.font, color: part.color });
          }
        });
      });
    });
  }
  if (started || lines.length === 0) {
    flush();
  }
  return lines;
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

// MARK: - Body: the markdown under the title (spec 5d, go/scene/body.go)

const bodyFontDrop = 2;
const bodyBlockGap = 5;
const bodyMonoFace = "Menlo";
const bodyListIndent = 16;
const bodyQuoteIndent = 12;
const bodyCodePad = 8;
const bodyBarW = 3;
const bodyHeadSize1 = 6; // above the body, h2 +4, h3 +2
const bodyHeadGap1 = 10;
const bodyHeadGap2 = 6;

/** One span of a line in one look: a markdown token after its glyphs are decided. */
interface BodyRun {
  text: string;
  font: Font;
  color?: Color; // the colour !(...) set, the body colour when unset
  code: boolean; // drawn over the code background
  img?: InlineRef; // an inline picture of a classic diagram: text is empty
}

type BodyBlock =
  | { kind: "h"; level: number; runs: BodyRun[] }
  | { kind: "p" | "q"; runs: BodyRun[] }
  | { kind: "l"; items: BodyRun[][] }
  | { kind: "c"; code: string[] };

/** One laid out line of the body. */
interface BodyLine {
  x: number; // indent of the line
  w: number; // width of its runs
  asc: number;
  desc: number;
  h: number; // line box height
  gap: number; // extra space before the line, between blocks
  quote: boolean; // a bar on the left
  code: boolean; // a background across the width
  runs: BodyRun[];
}

/** The markdown body of an item, ready to be measured and drawn. */
interface BodyText {
  font: Font; // body text, already 2 points below the title
  mono: Font; // code
  color: Color;
  codeBg: Color;
  bar: Color; // the bar of a quote
  lineH: number; // 0: the font's rounded line box
  gap: number; // between blocks
  padX: number;
  padY: number;
  blocks: BodyBlock[];
  pos: DiagPos; // the body command, for a warning about its height
  from: string; // the markdown file the text was read from; "" for text written in the .tenn
  /** The classic inline pictures the body may name with @(name). */
  imgs: Map<string, InlineImage>;
  prep: Prep;
}

/** gfx's estimate for one run: 0.6 em a rune plus its spacing, as textWidth. */
const bodyWidth = (s: string, f: Font): number => [...s].length * (f.size * 0.6 + f.spacing);

/** Two runs share a font when Go's gfx.Font values are equal. */
const bodyFontKey = (f: Font): string => `${f.name}\u0000${f.family}\u0000${f.weight}\u0000${f.italic}\u0000${f.size}\u0000${f.spacing}`;

/** A face Name leaves weight and style to the face, so the family takes over, as in Go. */
function bodyStyled(f: Font, weight: number, italic: boolean): Font {
  const out = { ...f };
  if (out.name !== "") {
    out.family = out.name.split(",")[0]!.trim();
    out.name = "";
  }
  if (weight !== 0) {
    out.weight = weight;
  }
  if (italic) {
    out.italic = true;
  }
  return out;
}

/** **bold**, *italic* and [label](url) mapped to the forms the markdown lexer reads. */
function normalizeInline(s: string): string {
  const boundary = (i: number): boolean => i === 0 || s[i - 1] === " " || s[i - 1] === "\t";
  let out = "";
  for (let i = 0; i < s.length; ) {
    switch (true) {
      case s[i] === "`": {
        const j = s.slice(i + 1).indexOf("`");
        if (j >= 0) {
          out += s.slice(i, i + j + 2);
          i += j + 2;
        } else {
          out += s[i];
          i += 1;
        }
        break;
      }
      case s.startsWith("**", i) && boundary(i): {
        const j = s.slice(i + 2).indexOf("**");
        if (j > 0) {
          out += `*${s.slice(i + 2, i + 2 + j)}*`;
          i += j + 4;
        } else {
          out += s[i];
          i += 1;
        }
        break;
      }
      case s[i] === "*" && boundary(i): {
        const j = s.slice(i + 1).indexOf("*");
        if (j > 0) {
          out += `_${s.slice(i + 1, i + 1 + j)}_`;
          i += j + 2;
        } else {
          out += s[i];
          i += 1;
        }
        break;
      }
      case s[i] === "[": {
        const mid = s.slice(i).indexOf("](");
        const end = mid >= 0 ? s.slice(i + mid + 2).indexOf(")") : -1;
        if (mid >= 0 && end >= 0) {
          out += s.slice(i + 1, i + mid);
          i += mid + 2 + end + 1;
        } else {
          out += s[i];
          i += 1;
        }
        break;
      }
      default:
        out += s[i];
        i += 1;
    }
  }
  return out;
}

/** `&(size)`: a number, else the markup sets nothing, as go/render reads it. */
function markupSize(s: string): number | null {
  const t = s.trim();
  const v = Number(t);
  return t === "" || !Number.isFinite(v) ? null : v;
}

/**
 * One line of markdown as runs: the lexer's tokens, code spans kept literal by normalizeInline.
 * `!(colour)` and `&(size)` set the colour and font until `!()` and `&()`; `!(colour|text)` and
 * `&(size|text)` style one span, as go/render gives them.
 */
function bodyInline(b: BodyText, s: string): BodyRun[] {
  const runs: BodyRun[] = [];
  let font = b.font;
  let color: Color | undefined;
  const add = (text: string, f: Font, code: boolean, c: Color | undefined = color): void => {
    if (text === "") {
      return;
    }
    const last = runs[runs.length - 1];
    if (
      last !== undefined &&
      bodyFontKey(last.font) === bodyFontKey(f) &&
      last.code === code &&
      last.color === c
    ) {
      last.text += text;
      return;
    }
    runs.push({ text, font: f, code, color: c });
  };
  const markupColor = (name: string): Color | undefined => {
    const c = sceneColor(name);
    if (c === null) {
      b.prep.diag.warn(b.pos, `!(${name}): no such color, the default is used`);
      return undefined;
    }
    return c;
  };
  const lexer = new MarkdownLexer(normalizeInline(s));
  for (let t = lexer.getToken(); t !== null; t = lexer.getToken()) {
    switch (t.type) {
      case "bold":
        add(t.literal, bodyStyled(font, 700, false), false);
        break;
      case "italic":
        add(t.literal, bodyStyled(font, 0, true), false);
        break;
      case "code":
        add(t.literal, b.mono, true);
        break;
      case "image": {
        const img = resolveInline(b.imgs, t.literal);
        if (img !== null) {
          runs.push({ text: "", font, code: false, img });
        } else {
          add(`@(${t.literal})`, font, false);
        }
        break;
      }
      case "color": {
        const bar = t.literal.indexOf("|");
        if (bar >= 0) {
          add(t.literal.slice(bar + 1), font, false, markupColor(t.literal.slice(0, bar)));
        } else if (t.literal.trim() === "") {
          color = undefined;
        } else {
          color = markupColor(t.literal);
        }
        break;
      }
      case "font": {
        const bar = t.literal.indexOf("|");
        if (bar >= 0) {
          const v = markupSize(t.literal.slice(0, bar));
          if (v !== null) {
            add(t.literal.slice(bar + 1), { ...font, size: v }, false);
          }
        } else {
          const v = markupSize(t.literal);
          font = v === null ? b.font : { ...b.font, size: v };
        }
        break;
      }
      case "underline":
        add(`<${t.literal}>`, font, false);
        break;
      case "scratch":
        add(`~${t.literal}~`, font, false);
        break;
      case "eof":
        break;
      default:
        add(t.literal, font, false);
    }
  }
  return runs;
}

/** "# ", "## " or "### "; more hashes are an ordinary paragraph. */
function headingLine(s: string): { level: number; rest: string } | null {
  let n = 0;
  while (n < s.length && s[n] === "#") {
    n += 1;
  }
  if (n === 0 || n > 3 || n === s.length || s[n] !== " ") {
    return null;
  }
  return { level: n, rest: s.slice(n).trim() };
}

function bulletLine(s: string): string | null {
  return s.length > 2 && "-*+".includes(s[0]!) && s[1] === " " ? s.slice(2).trim() : null;
}

/** "> text": one level of quote, the bar is drawn by the layout. */
function quoteLine(s: string): string | null {
  if (s === ">") {
    return "";
  }
  return s.length > 1 && s[0] === ">" && s[1] === " " ? s.slice(2).trim() : null;
}

/** Splits markdown into blocks and their inline runs. */
function bodyParse(b: BodyText, text: string): BodyBlock[] {
  const blocks: BodyBlock[] = [];
  let para: string[] = [];
  let quote: string[] = [];
  let list: BodyRun[][] | null = null;
  let code: string[] | null = null;
  const flushPara = (): void => {
    if (para.length > 0) {
      blocks.push({ kind: "p", runs: bodyInline(b, para.join(bodySep(b))) });
      para = [];
    }
  };
  const flushQuote = (): void => {
    if (quote.length > 0) {
      blocks.push({ kind: "q", runs: bodyInline(b, quote.join(bodySep(b))) });
      quote = [];
    }
  };
  const flushList = (): void => {
    if (list !== null) {
      blocks.push({ kind: "l", items: list });
      list = null;
    }
  };
  for (const line of text.split("\n")) {
    const trimmed = line.trim();
    if (code !== null) {
      if (trimmed.startsWith("```")) {
        blocks.push({ kind: "c", code });
        code = null;
      } else {
        code.push(line);
      }
      continue;
    }
    if (trimmed.startsWith("```")) {
      flushPara();
      flushQuote();
      flushList();
      code = [];
      continue;
    }
    if (trimmed === "") {
      flushPara();
      flushQuote();
      flushList();
      continue;
    }
    const q = quoteLine(trimmed);
    if (q !== null) {
      flushPara();
      flushList();
      quote.push(q);
      continue;
    }
    const h = headingLine(trimmed);
    if (h !== null) {
      flushPara();
      flushQuote();
      flushList();
      const size = b.font.size + bodyHeadSize1 - 2 * (h.level - 1);
      const runs = bodyInline(b, h.rest).map((r) => ({ ...r, font: { ...bodyStyled(r.font, 700, false), size } }));
      blocks.push({ kind: "h", level: h.level, runs });
      continue;
    }
    const item = bulletLine(trimmed);
    if (item !== null) {
      flushPara();
      flushQuote();
      if (list === null) {
        list = [];
      }
      list.push(bodyInline(b, item));
      continue;
    }
    flushList();
    flushQuote();
    para.push(trimmed);
  }
  flushPara();
  flushQuote();
  flushList();
  if (code !== null && code.length > 0) {
    // an unclosed fence is still code: the scene must not lose it
    blocks.push({ kind: "c", code });
  }
  return blocks;
}

/** Runs of spaces and runs of other characters, as the renderer wraps. */
function splitSpaces(s: string): string[] {
  const out: string[] = [];
  let i = 0;
  while (i < s.length) {
    const sp = s[i] === " " || s[i] === "\t";
    let end = i + 1;
    while (end < s.length && (s[end] === " " || s[end] === "\t") === sp) {
      end += 1;
    }
    out.push(s.slice(i, end));
    i = end;
  }
  return out;
}

function bodyFinish(b: BodyText, l: BodyLine): void {
  for (const run of l.runs) {
    if (run.img !== undefined) {
      continue;
    }
    const box = lineBox(b.prep, run.font.size);
    l.asc = Math.max(l.asc, box.asc);
    l.desc = Math.max(l.desc, box.desc);
  }
  l.h = b.prep.classic ? Math.floor(l.asc + l.desc) + 1 : Math.round(l.asc + l.desc);
  if (b.lineH > 0) {
    l.h = b.lineH;
  }
  for (const run of l.runs) {
    if (run.img !== undefined) {
      l.h = Math.max(l.h, run.img.h);
    }
  }
}

/**
 * Greedy wrap at spaces; text wider than the width stays on its line. first is the indent of the
 * first line, cont the indent of the continuation lines.
 */
/** The join of the lines of one paragraph: a space where a line break is soft, a newline in a classic
 *  diagram, where go/render breaks the line as written. */
function bodySep(b: BodyText): string {
  return b.prep.classic ? "\n" : " ";
}

/** The longest prefix of s that fits avail, in whole characters; "" when not even one does. */
function fitPrefix(b: BodyText, s: string, font: Font, avail: number): string {
  if (avail <= 0) {
    return "";
  }
  let cut = "";
  for (const g of s) {
    if (bodyWidth(cut + g, font) > avail + 1e-9) {
      break;
    }
    cut += g;
  }
  return cut;
}

function bodyWrap(b: BodyText, runs: BodyRun[], width: number, first: number, cont: number): BodyLine[] {
  const lines: BodyLine[] = [];
  const empty = (x: number): BodyLine => ({ x, w: 0, asc: 0, desc: 0, h: 0, gap: 0, quote: false, code: false, runs: [] });
  let line = empty(first);
  let started = false;
  const flush = (): void => {
    bodyFinish(b, line);
    lines.push(line);
    line = empty(cont);
    started = false;
  };
  const put = (text: string, run: BodyRun, w: number): void => {
    const last = line.runs[line.runs.length - 1];
    if (
      last !== undefined &&
      last.img === undefined &&
      bodyFontKey(last.font) === bodyFontKey(run.font) &&
      last.color === run.color &&
      last.code === run.code
    ) {
      last.text += text;
    } else {
      line.runs.push({ text, font: run.font, color: run.color, code: run.code });
    }
    line.w += w;
    started = true;
  };
  // classicWord breaks a word wider than the width by characters, as go/render did: an old body grows down
  // instead of running over its box.
  const classicWord = (word: string, run: BodyRun, w: number): boolean => {
    if (!b.prep.classic || width <= 0 || w <= width + 1e-9) {
      return false;
    }
    while (word !== "") {
      const avail = width - (line.x + line.w);
      const fit = bodyWidth(word, run.font);
      if (fit <= avail + 1e-9) {
        put(word, run, fit);
        return true;
      }
      let cut = fitPrefix(b, word, run.font, avail);
      if (cut === "") {
        if (!started) {
          cut = [...word][0] ?? "";
        } else {
          flush();
          continue;
        }
      }
      put(cut, run, bodyWidth(cut, run.font));
      word = word.slice(cut.length);
      flush();
    }
    return true;
  };
  for (const run of runs) {
    if (run.img !== undefined) {
      if (started && width > 0 && line.x + line.w + run.img.w > width + 1e-9) {
        flush();
      }
      line.runs.push(run);
      line.w += run.img.w;
      started = true;
      continue;
    }
    const segs = run.text.split("\n");
    for (let si = 0; si < segs.length; si++) {
      if (si > 0 && started) {
        flush(); // a newline of the text is a break, not a space
      }
      for (const tok of splitSpaces(segs[si]!)) {
        const space = tok.trim() === "";
        const w = bodyWidth(tok, run.font);
        if (space && !started) {
          continue; // leading spaces of a line are not drawn
        }
        if (!space && started && width > 0 && line.x + line.w + w > width + 1e-9) {
          flush();
        }
        if (!space && classicWord(tok, run, w)) {
          continue;
        }
        put(tok, run, w);
      }
    }
  }
  if (started || lines.length === 0) {
    flush();
  }
  return lines;
}

/** Breaks a code line at the width by characters: code has no spaces to break at. */
function bodyCodeLines(b: BodyText, src: string, width: number): string[] {
  if (width <= 0 || bodyWidth(src, b.mono) <= width + 1e-9) {
    return [src];
  }
  const out: string[] = [];
  let line = "";
  for (const g of [...src]) {
    if (line !== "" && bodyWidth(line + g, b.mono) > width + 1e-9) {
      out.push(line);
      line = g;
      continue;
    }
    line += g;
  }
  return [...out, line];
}

/** Lays the body out at width. A width of 0 or less does not wrap. */
function bodyLines(b: BodyText, width: number): BodyLine[] {
  const out: BodyLine[] = [];
  for (const blk of b.blocks) {
    let lines: BodyLine[] = [];
    switch (blk.kind) {
      case "h":
      case "p":
        lines = bodyWrap(b, blk.runs, width, 0, 0);
        break;
      case "q":
        lines = bodyWrap(b, blk.runs, width, bodyQuoteIndent, bodyQuoteIndent);
        for (const l of lines) {
          l.quote = true;
        }
        break;
      case "l": {
        const bulletW = bodyWidth("• ", b.font);
        blk.items.forEach((item, j) => {
          const runs = [{ text: "• ", font: b.font, code: false }, ...item];
          // go/render hangs no indent on a list of a classic body: the bullet stands where the text does
          // and a wrapped line returns to the left edge.
          const first = b.prep.classic ? 0 : bodyListIndent;
          const ls = bodyWrap(b, runs, width, first, first + bulletW);
          if (j === 0 && out.length > 0 && ls.length > 0) {
            ls[0]!.gap = b.gap;
          }
          lines.push(...ls);
        });
        if (lines.length === 0) {
          continue;
        }
        break;
      }
      case "c": {
        const inner = width - 2 * bodyCodePad;
        for (const src of blk.code) {
          for (const part of bodyCodeLines(b, src, inner)) {
            const line: BodyLine = {
              x: bodyCodePad,
              w: 0,
              asc: 0,
              desc: 0,
              h: 0,
              gap: 0,
              quote: false,
              code: true,
              runs: [{ text: part, font: b.mono, code: true }],
            };
            bodyFinish(b, line);
            lines.push(line);
          }
        }
        if (lines.length === 0) {
          continue;
        }
        break;
      }
    }
    if (out.length > 0 && blk.kind !== "l" && lines.length > 0) {
      lines[0]!.gap = b.gap;
      if (blk.kind === "h") {
        switch (blk.level) {
          case 1:
            lines[0]!.gap += bodyHeadGap1;
            break;
          case 2:
            lines[0]!.gap += bodyHeadGap2;
            break;
        }
      }
    }
    out.push(...lines);
  }
  return out;
}

/** The width of the widest line when nothing wraps. */
function bodyNatural(b: BodyText): number {
  let w = 0;
  for (const l of bodyLines(b, Infinity)) {
    w = Math.max(w, l.x + l.w);
  }
  return w;
}

/** The height of the body at width, its block gaps included. */
function bodyHeight(b: BodyText, width: number): number {
  let h = 0;
  for (const l of bodyLines(b, width)) {
    h += l.gap + l.h;
  }
  return h;
}

/** Go's %g: six significant digits, no trailing zeros, for the numbers of a warning. */
const goNum = (v: number): string => String(Number(v.toPrecision(6)));

/** Warns when a body read from a file needs more room than the item gives it: the page is fixed, the file grows. */
function warnBodyFit(prep: Prep, rn: RNode): void {
  const body = prep.bodies.get(rn);
  const ln = prep.frames.get(rn);
  if (body !== undefined && body.from !== "" && ln !== undefined) {
    let width = ln.rw - 2 * body.padX;
    let top = body.padY;
    const bn = prep.bodyAt.get(rn);
    if (bn !== undefined) {
      width = bn.rw;
      top = 0;
    } else {
      const ti = prep.texts.get(rn);
      if (ti !== undefined) {
        top = ti.padY + titleHeight(ti, ln.rw - 2 * ti.padX);
      }
    }
    const room = ln.rh - top - body.padY;
    const need = bodyHeight(body, width);
    if (need > room + 0.5) {
      prep.diag.warn(body.pos, `body --from-file "${body.from}" needs ${goNum(need)} points of height, the item leaves ${goNum(room)}: raise height or shorten the text`);
    }
  }
  for (const k of rn.kids) {
    if (k.kind === "item") {
      warnBodyFit(prep, k);
    }
  }
}

/** The text of a string or markdown literal; a closing "%}" leaves no percent in it. */
function markdownArg(src: string | null, a: TennNode | undefined): string {
  if (a === undefined) {
    return "";
  }
  let text = argText(a);
  if (text.endsWith("%") && (a.token === null || src === null || src[a.token.pos + text.length] === "}")) {
    text = text.slice(0, -1);
  }
  return text;
}

/** The path after --from-file among the arguments of a body command; "" when the flag has no path, null without the flag. */
function bodyFileArg(args: TennNode[]): string | null {
  for (let i = 0; i < args.length; i += 1) {
    if (argText(args[i]) === "--from-file") {
      return argText(args[i + 1]);
    }
  }
  return null;
}

/** An ATX heading: one to six hashes and a space; its level and title, or null. */
function headingAt(line: string): { level: number; title: string } | null {
  const s = line.trim();
  let n = 0;
  while (n < s.length && s[n] === "#") {
    n += 1;
  }
  if (n === 0 || n > 6 || n === s.length || s[n] !== " ") {
    return null;
  }
  return { level: n, title: s.slice(n + 1) };
}

/** A heading name as sections compare it: spaces collapsed, the closing hashes of an ATX heading dropped. */
function headingKey(s: string): string {
  return s.trim().replace(/#+$/, "").split(/\s+/).filter((w) => w !== "").join(" ");
}

/** The section under the heading name: the heading line and everything below up to the next heading of the same or a higher level. */
function bodySection(text: string, name: string): string | null {
  const want = headingKey(name);
  const lines = text.split("\n");
  let start = -1;
  let level = 0;
  for (let i = 0; i < lines.length; i += 1) {
    const h = headingAt(lines[i]!);
    if (h === null) {
      continue;
    }
    if (start < 0) {
      if (headingKey(h.title) === want) {
        start = i;
        level = h.level;
      }
      continue;
    }
    if (h.level <= level) {
      return lines.slice(start, i).join("\n");
    }
  }
  return start < 0 ? null : lines.slice(start).join("\n");
}

/** The headings of a markdown file, for the message about a section that is not there. */
function bodyHeadings(text: string): string[] {
  const out: string[] = [];
  for (const line of text.split("\n")) {
    const h = headingAt(line);
    if (h !== null) {
      out.push(headingKey(h.title));
    }
  }
  return out;
}

/**
 * The text of "body --from-file <path>": the whole markdown file, or the section named after "#".
 * The path is relative to the file that writes the body and may not leave the workspace root, as for
 * element --from-file. Every problem is reported here unless quiet, and null is returned then.
 */
function bodyFileText(prep: Prep, rn: RNode, ref: string, at: DiagPos, quiet = false): string | null {
  const fail = (message: string): void => {
    if (!quiet) {
      prep.diag.fail(at, message);
    }
  };
  const cut = ref.indexOf("#");
  const file = cut < 0 ? ref : ref.slice(0, cut);
  const section = cut < 0 ? "" : ref.slice(cut + 1);
  if (file === "") {
    fail("body --from-file needs a file: body --from-file \"docs/plan.md\"");
    return null;
  }
  const rel = resolvePath(rn.src?.file ?? "", file);
  if (rel === null) {
    fail(`body --from-file "${file}" leaves the workspace root`);
    return null;
  }
  const text = prep.readFile === null ? null : prep.readFile(rel);
  if (text === null) {
    fail(`body --from-file "${file}": no such file`);
    return null;
  }
  if (cut < 0) {
    return text;
  }
  const found = bodySection(text, section);
  if (found !== null) {
    return found;
  }
  const all = bodyHeadings(text);
  if (all.length > 0) {
    fail(`body --from-file "${ref}": ${rel} has no heading "${section}"; headings: ${all.join(", ")}`);
  } else {
    fail(`body --from-file "${ref}": ${rel} has no heading "${section}" and no headings at all`);
  }
  return null;
}

/** font-size of the item, the size the title is drawn with and the body is based on. */
function titleSize(prep: Prep, rn: RNode): number {
  const fs = propOf(prep, rn, "font-size");
  if (fs !== null) {
    const v = propNum(fs, 0);
    if (v !== null && v > 0) {
      return v;
    }
  }
  return defaultFontSize;
}

/**
 * Reads the body command of rn and parses its markdown; null when rn has no body or an empty one.
 * The forms are the classic renderer's: body "text", body %{ ... } and body { text "..."; ... }.
 */
function bodyOf(prep: Prep, rn: RNode, styleColor: Color, inCtx: boolean): BodyText | null {
  const pr = propOf(prep, rn, "body");
  if (pr === null) {
    return null;
  }
  const at: DiagPos = {
    file: rn.src?.file ?? "",
    line: pr.pos?.line ?? rn.src?.line ?? 0,
    col: pr.pos?.col ?? rn.src?.col ?? 0,
  };
  let size = 0;
  let lineH = 0;
  let gap = bodyBlockGap;
  let family = "";
  let mono = bodyMonoFace;
  let color = styleColor;
  let text = "";
  let from = "";
  const textArg = (i: number): string =>
    pr.val !== undefined ? (pr.val[i] ?? "") : i < pr.args.length ? markdownArg(rn.src?.src ?? null, pr.args[i]) : "";
  if (pr.block !== null) {
    for (const c of pr.block.children ?? []) {
      const name = cmdName(c);
      const { args } = splitCommand(c);
      switch (name) {
        case "text":
          if (args.length > 0) {
            text = markdownArg(rn.src?.src ?? null, args[0]);
          }
          break;
        case "font-size": {
          const v = argNum(args, 0);
          if (v !== null && v > 0) {
            size = v;
          } else {
            prep.diag.warn(at, "body font-size: not a number above 0, ignored");
          }
          break;
        }
        case "font-family":
          family = argText(args[0]);
          break;
        case "mono": {
          const s = argText(args[0]);
          if (s !== "") {
            mono = s;
          }
          break;
        }
        case "text-color": {
          const c2 = sceneColor(argText(args[0]));
          if (c2 !== null) {
            color = c2;
          } else {
            prep.diag.warn(at, `body text-color: "${argText(args[0])}" is not a color`);
          }
          break;
        }
        case "line-height": {
          const v = argNum(args, 0);
          if (v !== null && v > 0) {
            lineH = v;
          } else {
            prep.diag.warn(at, "body line-height: not a number above 0, ignored");
          }
          break;
        }
        case "gap": {
          const v = argNum(args, 0);
          if (v !== null && v >= 0) {
            gap = v;
          } else {
            prep.diag.warn(at, "body gap: not a number from 0, ignored");
          }
          break;
        }
      }
    }
  } else {
    text = textArg(0);
  }
  const fileArg = bodyFileArg(pr.args);
  if (fileArg !== null) {
    const fromFile = bodyFileText(prep, rn, fileArg, at);
    if (fromFile === null) {
      return null;
    }
    text = fromFile;
    from = fileArg;
  }
  if (text.trim() === "") {
    return null;
  }
  if (prep.classic) {
    // go/render reads a text `\n` of a body as a newline; a scene keeps the backslash (spec 4.3).
    text = text.replaceAll("\\n", "\n");
  }
  text = dedent(text).replaceAll("\t", "    ");

  if (size === 0) {
    size = titleSize(prep, rn) - bodyFontDrop;
  }
  if (size <= 0) {
    size = 1;
  }
  const font = fontOf(prep, rn, size);
  if (family !== "") {
    font.family = family;
    font.name = "";
  }
  const b: BodyText = {
    font,
    mono: { family: mono, weight: 0, italic: false, size, spacing: 0, name: "" },
    color,
    codeBg: sceneColor("grey-200") ?? noColor,
    bar: sceneColor("grey-400") ?? noColor,
    lineH,
    gap,
    // go/render insets the body of a classic item by the same 4 points as its title.
    padX: inCtx ? 0 : prep.classic ? classicTextPad : textPadX,
    padY: inCtx ? 0 : prep.classic ? classicTextPad : textPadY,
    blocks: [],
    pos: at,
    from,
    imgs: prep.classic ? inlineImages(prep, rn) : new Map(),
    prep,
  };
  b.blocks = bodyParse(b, text);
  return b;
}

/** Draws the body with its top left corner at x, y. */
function drawBody(ctx: Canvas2D, b: BodyText, x: number, y: number, width: number): void {
  ctx.save();
  ctx.textBaseline = "alphabetic";
  let yy = y;
  for (const l of bodyLines(b, width)) {
    yy += l.gap;
    if (l.code && width > 0) {
      ctx.fillStyle = sceneCss(b.codeBg);
      ctx.fillRect(x, yy, width, l.h);
    }
    if (l.quote) {
      ctx.fillStyle = sceneCss(b.bar);
      ctx.fillRect(x, yy, bodyBarW, l.h);
    }
    let xx = x + l.x;
    const base = yy + (l.h - (l.asc + l.desc)) / 2 + l.asc;
    for (const run of l.runs) {
      if (run.img !== undefined) {
        const raster = inlineRaster(b.prep, run.img.im);
        if (raster !== null) {
          ctx.drawImage(raster, xx, yy + (l.h - run.img.h) / 2, run.img.w, run.img.h);
        }
        xx += run.img.w;
        continue;
      }
      const w = bodyWidth(run.text, run.font);
      if (run.code && !l.code && w > 0) {
        ctx.fillStyle = sceneCss(b.codeBg);
        ctx.fillRect(xx, yy, w, l.h);
      }
      if (run.text !== "") {
        ctx.font = fontCss(run.font);
        ctx.fillStyle = sceneCss(run.color ?? b.color);
        ctx.fillText(run.text, xx, base);
      }
      xx += w;
    }
    yy += l.h;
  }
  ctx.restore();
}

// MARK: - Paint

interface Look {
  fill: Color;
  /** The `color linear` of the box, painted over fill (the first visible stop, as Go's gradient.base). */
  grad: SceneGradient | null;
  /** Every `shadow` of the box in file order, painted one pass each, the last on top. */
  shadows: SceneShadow[];
  border: Color;
  text: Color;
  lineW: number;
  /** `line-style` dashes for the border; empty is a solid line. */
  dash: number[];
  dashPhase: number;
  radius: [number, number, number, number];
  /** The radius a classic diagram gives every box; a hairline keeps square corners. */
  radiusDefault: boolean;
  /** `marker`: the text an old file draws in front of the title, an emoji of a mind map. */
  marker: string;
  /** `display`: rect when unset, else no-fill, circle, stack, text or none. */
  display: string;
  /** `layer` as the sort key among the siblings: 0 background, 1 the rest, 2 hover. */
  layer: number;
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

/** Go's prep.shadows: every `shadow` property in file order, the last painted on top. */
function shadowsOf(prep: Prep, rn: RNode): SceneShadow[] {
  const out: SceneShadow[] = [];
  for (const raw of rn.props) {
    if (raw.name !== "shadow") {
      continue;
    }
    const p = hasExpr(raw) ? evaluate(prep.diag, rn, raw) : raw;
    if (p === null) {
      continue;
    }
    const sh = shadowOf(prep, rn, p);
    if (sh !== null) {
      out.push(sh);
    }
  }
  return out;
}

/** Go's prep.shadowOf; blur 5 and black at 1/3 alpha by default. A bad offset drops that shadow. */
function shadowOf(prep: Prep, rn: RNode, p: SceneProp): SceneShadow | null {
  const at = p.pos ?? rn.src ?? { line: 0, col: 0 };
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
  bodies: Map<RNode, BodyText>;
  /** The text block a body takes in the flow of an auto-layout item, after its children. */
  bodyAt: Map<RNode, LayoutNode>;
  /** The ends and the look of every link, resolved once: go/scene's prep.ends and linkStyle. */
  links: Map<RNode, LinkPlan>;
  /** Where every elbow link leaves and enters the sides of its boxes, per parent. */
  linkSides: Map<RNode, LinkSides>;
  /** The static `rotate` of every node, read at build: go/scene reads it in emit. */
  rotations: Map<RNode, number>;
  /** The classic `image "name" @(...)` of a node by name, and the one that fills its box. */
  inlines: Map<RNode, Map<string, InlineImage>>;
  inlineBox: Map<RNode, InlineImage>;
  /** Reads the markdown of `body --from-file`; null when the caller gave no reader. */
  readFile: ((path: string) => string | null) | null;
  /** Decodes the base64 of a classic inline picture; null: the picture is sized but not drawn. */
  decodeImage: ImageDecoder | null;
  /** The element is a classic diagram (Go's sceneRoot is false): text inset 4, a size below 1 unset. */
  classic: boolean;
}

const defaultFontSize = 18;
const textPadX = 10;
const textPadY = 6;
/** The inset go/render gives a text leaf of a classic diagram when `padding` is not written. */
const classicTextPad = 4;
/** The corner radius go/render gives a box of a classic diagram. */
const classicRadius = 8;
/** The border colour go/render gives a box of a classic diagram: the generic grey 0.2, #424242. */
const classicBorder: Color = { r: 66 / 255, g: 66 / 255, b: 66 / 255, a: 1 };

/** The parts of the font size go/render takes as the ascent and the descent of a line. */
const classicAscent = 1980 / 2048;
const classicDescent = 432 / 2048;

/** The line box of a font at size: a scene estimates it as gfx does, a classic diagram takes the ratios
 *  and the rounding of go/render, so an old file keeps the spacing and the baseline of its picture. */
function lineBox(prep: Prep, size: number): { asc: number; desc: number; h: number } {
  if (prep.classic) {
    const asc = size * classicAscent;
    const desc = size * classicDescent;
    return { asc, desc, h: Math.floor(asc + desc) + 1 };
  }
  return { asc: size * 0.8, desc: size * 0.25, h: Math.round(size * 1.05) };
}
const fontName = "Helvetica, Arial, sans-serif";
const codeFont = "Inter";

// MARK: - The scene

export interface TennScene {
  /** The extent of the content, as go/scene Screen.W and H. */
  width: number;
  height: number;
  /** y-up bounds, as DrawableScene.getBounds. */
  bounds: Rect;
  diagnostics: { message: string; warn: boolean; line: number; col: number; file: string }[];
  /** Draws into a y-up canvas (the caller flips it once, as drawScene does); `at` is where its bottom edge goes. */
  draw(ctx: Canvas2D, at?: Point, moved?: (item: DiagramItem) => Point | null): void;
  /** The laid out items in paint order, for tests and the inspector. */
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
function fileTree(element: Element, diag: Diagnostics, options: SceneLoadOptions = {}): SceneNode {
  let root: Element = element;
  while (root.parent !== null) {
    root = root.parent;
  }
  const loader = new Loader(diag, options);
  const file = newNode("file", "", null, null);
  file.file = loader.main;
  file.src = options.source ?? null;
  const seen = new Set<Element>();
  addElement(file, root, loader, diag, seen);
  return file;
}

function sceneNodeFor(file: SceneNode, element: Element): SceneNode | null {
  let found: SceneNode | null = null;
  const walk = (n: SceneNode): void => {
    for (const e of named(n)) {
      if (e.kind !== "element") {
        continue;
      }
      if (e.model === element || (e.model === null && e.name === element.name)) {
        found = e;
        return;
      }
      walk(e);
      if (found !== null) {
        return;
      }
    }
  };
  walk(file);
  return found;
}

/** The prep of a screen: sizes, styles and text of every node, then the layout. */
function prepare(expanded: RNode, diag: Diagnostics, options: SceneLoadOptions = {}, classic: boolean): { prep: Prep; root: LayoutNode } {
  const prep: Prep = {
    diag,
    frames: new Map(),
    texts: new Map(),
    looks: new Map(),
    icons: new Map(),
    bodies: new Map(),
    bodyAt: new Map(),
    links: new Map(),
    linkSides: new Map(),
    rotations: new Map(),
    inlines: new Map(),
    inlineBox: new Map(),
    readFile: options.readFile ?? null,
    decodeImage: options.decodeImage ?? null,
    classic,
  };
  animations(expanded, new Map(), diag);
  const root = newLayout(expanded);
  root.dir = "abs";
  // The root sizes the screen: a number is its box (spec 5b), as in Go's root().
  const rw = propOf(prep, expanded, "width");
  const rv = rw === null ? null : propNum(rw, 0);
  if (rv !== null) {
    root.w = fixed(rv);
  }
  const rh = propOf(prep, expanded, "height");
  const rvH = rh === null ? null : propNum(rh, 0);
  if (rvH !== null) {
    root.h = fixed(rvH);
  }
  prep.frames.set(expanded, root);
  const turned = (n: RNode): void => {
    prep.rotations.set(n, rotationOf(prep, n));
    n.kids.forEach(turned);
  };
  turned(expanded);
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
    if ((a.kind === "StringLit" || a.kind === "MarkdownLit") && (a.getIdentText() ?? "").includes("${")) {
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
  return { name: p.name, args: p.args, block: p.block, val, pos: p.pos };
}

/** Go's prep.rotation: degrees clockwise, about the middle of what the item draws. */
function rotationOf(prep: Prep, rn: RNode): number {
  const p = propOf(prep, rn, "rotate");
  if (p === null) {
    return 0;
  }
  const v = propNum(p, 0);
  if (v !== null) {
    return v;
  }
  prep.diag.warn(p.pos ?? rn.src ?? { line: 0, col: 0 }, "rotate takes degrees, as in rotate 15; ignored");
  return 0;
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
    // A classic size below one is not written: an old file that computes 0 falls back to its text,
    // so it draws a dot and not an invisible box (go/scene size).
    if (prep.classic && v < 1) {
      return HUG;
    }
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
  // The name of a box that holds items is only a path segment; its text is a title if one is written.
  let title = inCtx || holdsItems(rn) ? "" : rn.name;
  const p = propOf(prep, rn, "title");
  if (p !== null) {
    title = propText(p, 0);
    // A bare text with spaces is several arguments, and only the first would be drawn.
    if (p.args.length > 1) {
      prep.diag.fail(p.pos ?? rn.src ?? { line: 0, col: 0 }, `title takes one value; quote a text with spaces: title "Sign in"`);
    }
  }
  const marker = prep.looks.get(rn)?.marker ?? "";
  if (marker !== "") {
    // A marker is drawn in front of the text, with a space, as go/render does it.
    title = `${marker} ${title}`;
  }
  if (title === "") {
    return null;
  }
  if (prep.classic) {
    // go/render reads a text `\n` as a newline, in a title and in the name a leaf draws.
    title = title.replaceAll("\\n", "\n");
  }
  let size = defaultFontSize;
  const fs = propOf(prep, rn, "font-size");
  if (fs !== null) {
    const v = propNum(fs, 0);
    if (v !== null && v > 0) {
      size = v;
    } else if (v !== null) {
      prep.diag.warn(fs.pos ?? rn.src ?? { line: 0, col: 0 }, `font-size: ${goNum(v)} is not above 0, the default is used`);
    }
  }
  const ti: TextInfo = {
    font: fontOf(prep, rn, size),
    padX: inCtx ? 0 : prep.classic ? classicTextPad : textPadX,
    padY: inCtx ? 0 : prep.classic ? classicTextPad : textPadY,
    ...(() => {
      const box = lineBox(prep, size);
      return { lineH: box.h, asc: box.asc, desc: box.desc };
    })(),
    align: "middle",
    text: title,
    parts: null,
    prep,
  };
  if (prep.classic) {
    ti.parts = titleMarkupParts(prep, rn, ti) ?? splitTitleImages(title, inlineImages(prep, rn));
  }
  const lh = propOf(prep, rn, "line-height");
  if (lh !== null) {
    const v = propNum(lh, 0);
    if (v !== null && v > 0) {
      ti.lineH = v;
    } else {
      prep.diag.warn(lh.pos ?? rn.src ?? { line: 0, col: 0 }, "line-height: not a number above 0, ignored");
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
    shadows: [],
    border: inCtx ? noColor : prep.classic ? classicBorder : colorBlack,
    text: up,
    lineW: 1,
    dash: [],
    dashPhase: 0,
    radius: [0, 0, 0, 0],
    radiusDefault: false,
    marker: "",
    display: "",
    layer: 1,
    clip: false,
    opacity: 1,
  };
  const fill = fillOf(prep, rn, look.fill);
  look.fill = fill.fill;
  look.grad = fill.grad;
  look.shadows = shadowsOf(prep, rn);
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
  const line = dashOf(prep, rn);
  look.dash = line.dash;
  look.dashPhase = line.phase;
  const radius = propOf(prep, rn, "corner-radius");
  if (radius !== null) {
    look.radius = radii(prep, radius);
  } else if (prep.classic) {
    // go/render rounds every box of a classic diagram by 8 points unless the file says otherwise.
    look.radius = [classicRadius, classicRadius, classicRadius, classicRadius];
    look.radiusDefault = true;
  }
  const marker = propOf(prep, rn, "marker");
  if (marker !== null) {
    // go/render draws the marker in front of the title: an emoji of a mind map is the text of its box.
    look.marker = propText(marker, 0);
  }
  const display = propOf(prep, rn, "display");
  if (display !== null) {
    look.display = propText(display, 0);
  }
  const layer = propOf(prep, rn, "layer");
  if (layer !== null) {
    const word = propText(layer, 0);
    look.layer = word === "background" ? 0 : word === "hover" ? 2 : 1;
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

/** Go's prep.dash: the dashes of a classic diagram's border; a bad word warns and leaves it solid. */
function dashOf(prep: Prep, rn: RNode): { dash: number[]; phase: number } {
  const p = propOf(prep, rn, "line-style");
  if (p === null) {
    return { dash: [], phase: 0 };
  }
  const s = propText(p, 0);
  switch (s) {
    case "dashed":
      return { dash: [5, 5], phase: 5 };
    case "dotted":
      return { dash: [1, 4], phase: 1 };
    case "solid":
      return { dash: [], phase: 0 };
  }
  prep.diag.warn(p.pos ?? rn.src ?? { line: 0, col: 0 }, `line-style takes solid, dashed or dotted, got "${s}"`);
  return { dash: [], phase: 0 };
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

/**
 * Go's prep.paintOrder: background items first, the rest in file order, then hover items; a link is no
 * item and stays in the middle pass. The order is the picture only, not the layout.
 */
function paintOrder(prep: Prep, rn: RNode): RNode[] {
  // A classic diagram draws its links first, so a line passes under the boxes it crosses: the Swift
  // editor inserts them at the front of the element.
  const rank = (k: RNode): number => (k.kind === "item" ? (prep.looks.get(k)?.layer ?? 1) : prep.classic ? 0 : 1);
  return rn.kids
    .map((k, i) => ({ k, i, rank: rank(k) }))
    .sort((a, b) => a.rank - b.rank || a.i - b.i)
    .map((e) => e.k);
}

/** Go's display none: the item takes room and gives bounds but is never drawn, its children with it. */
const drawn = (prep: Prep, rn: RNode): boolean => (prep.looks.get(rn)?.display ?? "") !== "none";

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
  const boxImage = inlineBoxImage(prep, rn);
  if (boxImage !== null) {
    prep.inlineBox.set(rn, boxImage);
    // A picture with no size written takes its own pixels as the size.
    if (prop(rn, "width") === null) {
      n.w = fixed(boxImage.w);
    }
    if (prop(rn, "height") === null) {
      n.h = fixed(boxImage.h);
    }
  }
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
  // padding insets the text of a leaf as well as the children of a container (go/scene item).
  n.pad = paddingOf(prep, rn);
  // The root of an instance that holds items is a container, not a box of its own.
  const look = lookOf(prep, rn, inCtx || (rn.inst && holdsItems(rn)), up);
  prep.looks.set(rn, look);
  // A body is the content of a leaf; an item that holds items shows them instead.
  const body = holdsItems(rn) ? null : bodyOf(prep, rn, look.text, inCtx);
  if (body !== null) {
    prep.bodies.set(rn, body);
  }
  for (const k of rn.kids) {
    if (k.kind !== "item" || flag(prep, k, "hidden")) {
      continue;
    }
    n.kids.push(itemNode(prep, k, auto, look.text));
  }
  // The body of an auto-layout item is a text block of its own after the children, so the flow
  // puts it below them and the item grows with it; a leaf paints it under its title instead.
  if (body !== null && auto && n.kids.length > 0) {
    const bn = newLayout(null);
    bn.w = FILL;
    bn.leafW = () => bodyNatural(body);
    bn.leafH = (w: number) => bodyHeight(body, w);
    body.padX = 0;
    body.padY = 0;
    n.kids.push(bn);
    prep.bodyAt.set(rn, bn);
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

/** An item without children takes the size of its title and body, or of its icon. */
function leaf(prep: Prep, rn: RNode, n: LayoutNode, inCtx: boolean): void {
  const icon = iconOf(prep, rn);
  if (icon !== null) {
    prep.icons.set(rn, icon);
    n.leafW = () => icon.vw;
    n.leafH = () => icon.vh;
  }
  const ti = textOf(prep, rn, inCtx || icon !== null);
  // An icon has no text body: the path is the content.
  const body = icon === null ? (prep.bodies.get(rn) ?? null) : null;
  const pad = propOf(prep, rn, "padding");
  let padX = ti?.padX ?? (inCtx ? 0 : prep.classic ? classicTextPad : textPadX);
  let padY = ti?.padY ?? (inCtx ? 0 : prep.classic ? classicTextPad : textPadY);
  // padding insets the text whether or not the item has auto-layout: a leaf with a written size reads
  // it as the space between its border and the text.
  if (pad !== null) {
    padX = n.pad[3];
    padY = n.pad[0];
  }
  if (ti === null && body === null) {
    // go/render keeps the inset around a classic item with no text: an eight-point dot, which old
    // diagrams use as markers and anchors.
    if (prep.classic && !inCtx) {
      n.leafW = () => 2 * padX;
      n.leafH = () => 2 * padY;
    }
    return;
  }
  if (ti !== null) {
    prep.texts.set(rn, ti);
  }
  if (ti !== null) {
    ti.padX = padX;
    ti.padY = padY;
  }
  if (body !== null) {
    body.padX = padX;
    body.padY = padY;
  }
  n.leafW = () => {
    const w = Math.max(ti === null ? 0 : textNatural(ti), body === null ? 0 : bodyNatural(body));
    // go/render kept the fraction of a measured line; only a scene rounds its leaf to whole points.
    return (prep.classic ? w : Math.ceil(w)) + 2 * padX;
  };
  n.leafH = (w: number) => {
    const inner = w - 2 * padX;
    let h = ti === null ? 0 : titleHeight(ti, inner);
    if (body !== null) {
      h += bodyHeight(body, inner);
    }
    return h + 2 * padY;
  };
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

/** Go's boxPath: an ellipse for display circle, else the rounded rectangle. */
/**
 * The corner radius of a classic box: go/render keeps a hairline square, since a divider of height 1
 * would otherwise draw rounded ends.
 */
function boxRadius(look: Look, w: number, h: number): [number, number, number, number] {
  if (look.radiusDefault && (w < 2 || h < 2)) {
    return [0, 0, 0, 0];
  }
  return look.radius;
}

function boxPath(
  ctx: Canvas2D,
  display: string,
  x: number,
  y: number,
  w: number,
  h: number,
  radius: [number, number, number, number],
): void {
  if (display === "circle") {
    ctx.beginPath();
    ctx.ellipse(x + w / 2, y + h / 2, Math.abs(w) / 2, Math.abs(h) / 2, 0, 0, Math.PI * 2);
    return;
  }
  roundRectPath(ctx, x, y, w, h, radius);
}

/** The shadow of one sheet of display stack, as Go's stackShadow: black at 1/3 alpha, 5 blur, 1 up and right. */
const stackShadow: SceneShadow = { dx: 1, dy: 1, blur: 5, color: { r: 0, g: 0, b: 0, a: 1 / 3 } };

/** The linear gradient of the box, through its centre at the angle and reaching the corners (Go's gradient.at). */
function fillGradient(ctx: Canvas2D, g: SceneGradient, w: number, h: number): boolean {
  // shortcut: the SVG writer has no gradient object, so an SVG export draws the base colour; add one to
  // SvgContext when the export needs gradients.
  if (typeof ctx.createLinearGradient !== "function") {
    return false;
  }
  const a = (g.angle * Math.PI) / 180;
  // Go's gradient.at in the box's y-down local frame: 0 degrees runs from the bottom of the box (the
  // first stop) up to its top (the last), as on screen.
  const dx = Math.sin(a);
  const dy = -Math.cos(a);
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

function drawText(ctx: Canvas2D, ti: TextInfo, w: number, h: number, leafNode: boolean, color: Color, shadow?: SceneShadow): void {
  if (ti.parts !== null) {
    drawTextParts(ctx, ti, w, h, leafNode, color, shadow);
    return;
  }
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
    if (shadow === undefined) {
      ctx.fillText(s, x, base);
    } else {
      withShadow(ctx, shadow, () => ctx.fillText(s, x, base));
    }
  });
  ctx.restore();
}

/** drawParts: a title that holds inline pictures, the text on its baseline and a picture on the cap height. */
function drawTextParts(ctx: Canvas2D, ti: TextInfo, w: number, h: number, leafNode: boolean, color: Color, shadow?: SceneShadow): void {
  const lines = titleLayout(ti, w - 2 * ti.padX);
  let total = 0;
  for (const ln of lines) {
    total += ln.h;
  }
  let y = ti.padY;
  if (leafNode) {
    y = (h - total) / 2;
  }
  ctx.save();
  ctx.font = fontCss(ti.font);
  ctx.textBaseline = "alphabetic";
  ctx.fillStyle = sceneCss(color);
  for (const ln of lines) {
    let x = ti.padX;
    if (leafNode) {
      switch (ti.align) {
        case "left":
          break;
        case "right":
          x = w - ti.padX - ln.w;
          break;
        default:
          x = (w - ln.w) / 2;
      }
    }
    const base = y + (ln.h - (ln.asc + ln.desc)) / 2 + ln.asc;
    for (const part of ln.parts) {
      if (part.img !== null) {
        const raster = inlineRaster(ti.prep, part.img.im);
        if (raster !== null) {
          ctx.drawImage(raster, x, base - (partFont(ti, part).size * capHeightRatio) / 2 - part.img.h / 2, part.img.w, part.img.h);
        }
        x += part.img.w;
        continue;
      }
      const f = partFont(ti, part);
      if (part.text !== "") {
        ctx.font = fontCss(f);
        ctx.fillStyle = sceneCss(part.color ?? color);
        if (shadow === undefined) {
          ctx.fillText(part.text, x, base);
        } else {
          withShadow(ctx, shadow, () => ctx.fillText(part.text, x, base));
        }
      }
      x += fontWidth(f, part.text);
    }
    y += ln.h;
  }
  ctx.restore();
}

/** drawImage: a picture in a box, `fit` fill, cover or contain, the corners cut by radius. */
function drawPicture(ctx: Canvas2D, prep: Prep, im: InlineImage, fit: string, w: number, h: number, radius: [number, number, number, number]): void {
  const raster = inlineRaster(prep, im);
  if (raster === null || im.w <= 0 || im.h <= 0) {
    return;
  }
  let x = 0;
  let y = 0;
  let dw = w;
  let dh = h;
  if (fit === "cover" || fit === "contain") {
    const s = fit === "cover" ? Math.max(w / im.w, h / im.h) : Math.min(w / im.w, h / im.h);
    dw = im.w * s;
    dh = im.h * s;
    x = (w - dw) / 2;
    y = (h - dh) / 2;
  }
  const cut = fit === "cover" || radius.some((r) => r !== 0);
  if (cut) {
    ctx.save();
    roundRectPath(ctx, 0, 0, w, h, radius);
    ctx.clip();
  }
  ctx.drawImage(raster, x, y, dw, dh);
  if (cut) {
    ctx.restore();
  }
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
 * DrawableScene does, so renderElement's flip and offset work unchanged. options gives the files it mounts.
 */
export function buildTennScene(element: Element, options: SceneLoadOptions = {}): TennScene {
  const diag = new Diagnostics();
  const file = fileTree(element, diag, options);
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
  const classic = !sceneRoot(expanded);
  if (classic) {
    classicDefaults(expanded);
    // A classic diagram takes its expressions from the element model, as go/render's ClassicValues
    // does; the classic renderer builds the same ExecutionContext, so both draw the same values.
    const context = options.context ?? createExecutionContext();
    if (options.context == null) {
      context.setElement(element);
    }
    classicValues(expanded, context);
  } else {
    evalTree(expanded, diag);
  }
  applyStyles(expanded);
  const { prep, root } = prepare(expanded, diag, options, classic);
  layoutRoot(root);
  planLinks(expanded, prep);
  warnBodyFit(prep, expanded);
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
    if (!drawn(prep, rn)) {
      return;
    }
    for (const k of paintOrder(prep, rn)) {
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

  const drawNode = (ctx: Canvas2D, rn: RNode, ln: LayoutNode, isRoot: boolean, moved?: (item: DiagramItem) => Point | null): void => {
    const look = prep.looks.get(rn);
    const item = rn.src?.item ?? null;
    const move = item === null ? null : (moved?.(item) ?? null);
    ctx.save();
    ctx.translate(ln.rx + (move?.x ?? 0), ln.ry - (move?.y ?? 0));
    const deg = prep.rotations.get(rn) ?? 0;
    if (deg !== 0) {
      const cx = ln.rw / 2;
      const cy = ln.rh / 2;
      ctx.translate(cx, cy);
      // The scene is drawn inside scale(1, -1), so a clockwise turn is a negative canvas angle.
      ctx.rotate((-deg * Math.PI) / 180);
      ctx.translate(-cx, -cy);
    }
    const opacity = look?.opacity ?? 1;
    if (opacity < 1) {
      ctx.globalAlpha = (ctx.globalAlpha ?? 1) * opacity;
    }
    const display = look?.display ?? "";
    if (!isRoot && look !== undefined && display !== "none") {
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
          const rad = boxRadius(look, w, h);
          if (look.clip) {
            roundRectPath(ctx, 0, 0, w, h, rad);
            ctx.clip();
          }
          // text draws no box at all; no-fill draws only its border.
          if (display !== "text") {
            if (display === "stack") {
              // Three sheets behind the box, stepped 2 up and right, each under the stack shadow.
              for (let i = 1; i <= 3; i += 1) {
                const d = 2 * (3 - i);
                boxPath(ctx, display, d, -d, w, h, rad);
                withShadow(ctx, stackShadow, () => {
                  ctx.fillStyle = sceneCss(look.fill);
                  ctx.fill();
                });
              }
            }
            // Go's picture(): one shadow pass per `shadow`, under the fill; a gradient is painted over an
            // opaque base, since a canvas gradient with a shadow casts it from the gradient's alpha; a
            // border without a fill casts the shadow instead.
            let shadows = look.shadows;
            if (display !== "no-fill" && look.fill.a > 0) {
              boxPath(ctx, display, 0, 0, w, h, rad);
              const base = { ...look.fill, a: 1 };
              for (const sh of shadows) {
                withShadow(ctx, sh, () => {
                  ctx.fillStyle = sceneCss(base);
                  ctx.fill();
                });
              }
              if (look.grad !== null) {
                if (!fillGradient(ctx, look.grad, w, h)) {
                  ctx.fillStyle = sceneCss(look.fill);
                  ctx.fill();
                }
              } else if (shadows.length === 0) {
                ctx.fillStyle = sceneCss(look.fill);
                ctx.fill();
              }
              shadows = [];
            }
            const boxImage = prep.inlineBox.get(rn);
            if (boxImage !== undefined && (display === "" || display === "rect")) {
              const fitProp = propOf(prep, rn, "fit");
              drawPicture(ctx, prep, boxImage, fitProp === null ? "fill" : propText(fitProp, 0), w, h, rad);
            }
            if (look.lineW > 0 && look.border.a > 0) {
              const inset = look.lineW / 2;
              const radius = rad.map((r) => Math.max(r - inset, 0)) as [number, number, number, number];
              boxPath(ctx, display, inset, inset, w - look.lineW, h - look.lineW, radius);
              ctx.lineWidth = look.lineW;
              ctx.strokeStyle = sceneCss(look.border);
              ctx.setLineDash(look.dash);
              ctx.lineDashOffset = look.dashPhase;
              if (shadows.length > 0) {
                for (const sh of shadows) {
                  withShadow(ctx, sh, () => ctx.stroke());
                }
              } else {
                ctx.stroke();
              }
              // The dash belongs to this border: a child icon or a link is not dashed.
              ctx.setLineDash([]);
              ctx.lineDashOffset = 0;
            }
          }
          const ti = prep.texts.get(rn);
          if (ti !== undefined) {
            // With a body the title goes to the top left and the body follows it. display text has no
            // box, so the shadow of the item falls on the glyphs.
            drawText(
              ctx,
              ti,
              w,
              h,
              ln.kids.length === 0 && prep.bodies.get(rn) === undefined,
              look.text,
              display === "text" ? look.shadows[0] : undefined,
            );
          }
          const body = prep.bodies.get(rn);
          if (body !== undefined && prep.bodyAt.get(rn) === undefined) {
            let y = body.padY;
            if (ti !== undefined) {
              y = ti.padY + titleHeight(ti, w - 2 * ti.padX);
            }
            drawBody(ctx, body, body.padX, y, w - 2 * body.padX);
          }
        }
      }
    }
    if (display !== "none") {
      for (const k of paintOrder(prep, rn)) {
        const kn = prep.frames.get(k);
        if (k.kind === "item" && kn !== undefined) {
          drawNode(ctx, k, kn, false, moved);
        } else if (k.kind === "link") {
          drawLink(ctx, prep, k, prep.linkSides.get(rn) ?? null);
        }
      }
    }
    const bodyAt = prep.bodyAt.get(rn);
    const flowBody = prep.bodies.get(rn);
    if (display !== "none" && bodyAt !== undefined && flowBody !== undefined) {
      drawBody(ctx, flowBody, bodyAt.rx, bodyAt.ry, bodyAt.rw);
    }
    ctx.restore();
  };

  return {
    width,
    height,
    // y-up, which the boxes are not: the boxes are go/scene's screen y, counted down from the top, and
    // draw() flips them up, so the content sits above the origin instead of below it.
    bounds: { x: minX, y: 0, width: maxX - minX, height: maxY - minY },
    diagnostics: diag.list,
    boxes,
    draw(ctx: Canvas2D, at: Point = { x: 0, y: 0 }, moved?: (item: DiagramItem) => Point | null): void {
      ctx.save();
      // The caller draws in a y-up world (drawScene flips the canvas once); the boxes are y-down, so the
      // picture is flipped around its bottom edge, where `at` puts it.
      ctx.translate(at.x, at.y + maxY);
      ctx.scale(1, -1);
      drawNode(ctx, expanded, root, true, moved);
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

/** Go's sceneRoot: an instance, an item with child nodes or auto-layout makes the element a scene. */
function sceneRoot(rn: RNode): boolean {
  return rn.kids.some((k) => k.kind === "item" && (k.inst || k.kids.length > 0 || prop(k, "auto-layout") !== null));
}

/** The arguments of a classic diagram as the element model evaluates them: the value, else its text. */
function classicValues(rn: RNode, context: IExecutionContext): void {
  const target = rn.src === null ? null : (rn.src.model ?? rn.src.item);
  const ev = target === null ? null : context.getEvaluated(target);
  for (let i = 0; i < rn.props.length; i += 1) {
    const p = rn.props[i]!;
    if (!hasExpr(p)) {
      continue;
    }
    const val = p.args.map((a) => {
      const token = a.token;
      return token !== null && ev !== null && ev.has(token) ? jsToString(ev.get(token)) : (a.getIdentText() ?? "");
    });
    rn.props[i] = { ...p, val };
  }
  for (const k of rn.kids) {
    classicValues(k, context);
  }
}

// Sides of a box in layout coordinates: X grows to the right, Y down.
const sideRight = 0;
const sideLeft = 1;
const sideTop = 2;
const sideBottom = 3;

// elbowRadius is the bend of an elbow link; corner-radius on the link overrides it.
const elbowRadius = 12;

interface ElbowCorner {
  side: number;
  f: number;
}

interface LinkSides {
  starts: Map<RNode, ElbowCorner>;
  finishes: Map<RNode, ElbowCorner>;
}

interface LinkPlan {
  ends: [RNode, RNode];
  color: Color;
  width: number;
  dash: number[];
  phase: number;
}

function sideWord(s: string): number | null {
  switch (s) {
    case "right":
      return sideRight;
    case "left":
      return sideLeft;
    case "top":
      return sideTop;
    case "bottom":
      return sideBottom;
  }
  return null;
}

const sideVertical = (side: number): boolean => side === sideLeft || side === sideRight;

/** route as written on a link, without a warning: the elbow layout reads it many times. */
function routeWord(prep: Prep, lk: RNode): string {
  const p = propOf(prep, lk, "route");
  return p === null ? "" : propText(p, 0);
}

const isElbow = (prep: Prep, lk: RNode): boolean => routeWord(prep, lk) === "elbow";

/** Go's prep.isQuad: layout quad on the link bends it into one curve through the control point. */
function isQuad(prep: Prep, lk: RNode): boolean {
  const p = propOf(prep, lk, "layout");
  return p !== null && propText(p, 0) === "quad";
}

/**
 * Go's linkControl: the middle of the two boxes plus the link's pos. pos is y up in the file and the
 * scene y down, so the vertical offset is negated; (0, 0) gives no control point.
 */
function linkControl(prep: Prep, lk: RNode, pa: Point, pb: Point): Point | null {
  const p = propOf(prep, lk, "pos");
  if (p === null) {
    return null;
  }
  const x = propNum(p, 0);
  const y = propNum(p, 1);
  if (x === null || y === null || (x === 0 && y === 0)) {
    return null;
  }
  return { x: (pa.x + pb.x) / 2 + x, y: (pa.y + pb.y) / 2 - y };
}

/** Sides the elbow leaves and enters: the ones the route names, else the sides that look at each other. */
function routeSides(prep: Prep, lk: RNode, a: LayoutNode, c: LayoutNode): [number, number] {
  const want = facingSides(a, c);
  const p = propOf(prep, lk, "route");
  if (p === null) {
    return want;
  }
  for (let i = 0; i < 2; i += 1) {
    if (i + 1 >= p.args.length) {
      break;
    }
    const word = propText(p, i + 1);
    const side = sideWord(word);
    if (side === null) {
      prep.diag.warn(p.pos ?? lk.src ?? { line: 0, col: 0 }, `a route side is left, right, top or bottom, got "${word}"`);
      continue;
    }
    want[i] = side;
  }
  return want;
}

/** checkRoute warns about a route the format does not know; a link without one is straight. */
function checkRoute(prep: Prep, lk: RNode): void {
  const s = routeWord(prep, lk);
  if (s === "" || s === "straight" || s === "elbow") {
    return;
  }
  const p = propOf(prep, lk, "route");
  prep.diag.warn(p?.pos ?? lk.src ?? { line: 0, col: 0 }, `route takes straight or elbow, got "${s}"`);
}

function facingSides(a: LayoutNode, c: LayoutNode): [number, number] {
  const dx = c.rx + c.rw / 2 - (a.rx + a.rw / 2);
  const dy = c.ry + c.rh / 2 - (a.ry + a.rh / 2);
  if (Math.abs(dx) >= Math.abs(dy)) {
    return dx >= 0 ? [sideRight, sideLeft] : [sideLeft, sideRight];
  }
  return dy >= 0 ? [sideBottom, sideTop] : [sideTop, sideBottom];
}

/** The point at f (0..1) of a side of n. */
function sideAnchor(n: LayoutNode, side: number, f: number): Point {
  switch (side) {
    case sideRight:
      return { x: n.rx + n.rw, y: n.ry + n.rh * f };
    case sideLeft:
      return { x: n.rx, y: n.ry + n.rh * f };
    case sideTop:
      return { x: n.rx + n.rw * f, y: n.ry };
  }
  return { x: n.rx + n.rw * f, y: n.ry + n.rh };
}

/** along is the centre of n on the axis a side of the other box runs along. */
function along(n: LayoutNode, side: number): number {
  return sideVertical(side) ? n.ry + n.rh / 2 : n.rx + n.rw / 2;
}

/** elbowSides places every elbow link of a parent on the sides of both its ends, fanned in `along` order. */
function elbowSides(prep: Prep, parent: RNode): LinkSides {
  interface Member {
    lk: RNode;
    order: number;
  }
  const startGroups = new Map<RNode, Map<number, Member[]>>();
  const endGroups = new Map<RNode, Map<number, Member[]>>();
  const sides = new Map<RNode, [number, number]>();
  const add = (groups: Map<RNode, Map<number, Member[]>>, node: RNode, side: number, m: Member): void => {
    const bySide = groups.get(node) ?? new Map<number, Member[]>();
    bySide.set(side, [...(bySide.get(side) ?? []), m]);
    groups.set(node, bySide);
  };
  for (const k of parent.kids) {
    if (k.kind !== "link" || !isElbow(prep, k)) {
      continue;
    }
    const plan = prep.links.get(k);
    if (plan === undefined) {
      continue;
    }
    const a = prep.frames.get(plan.ends[0]);
    const c = prep.frames.get(plan.ends[1]);
    if (a === undefined || c === undefined) {
      continue;
    }
    const [from, to] = routeSides(prep, k, a, c);
    sides.set(k, [from, to]);
    add(startGroups, plan.ends[0], from, { lk: k, order: along(c, to) });
    add(endGroups, plan.ends[1], to, { lk: k, order: along(a, from) });
  }
  const place = (groups: Map<RNode, Map<number, Member[]>>, which: 0 | 1): Map<RNode, ElbowCorner> => {
    const out = new Map<RNode, ElbowCorner>();
    for (const bySide of groups.values()) {
      for (const list of bySide.values()) {
        list.sort((x, y) => x.order - y.order);
        list.forEach((m, i) => out.set(m.lk, { side: sides.get(m.lk)![which], f: (i + 1) / (list.length + 1) }));
      }
    }
    return out;
  };
  return { starts: place(startGroups, 0), finishes: place(endGroups, 1) };
}

/** The corners of an elbow link, from sideAnchor of each end (go/scene elbow). */
function elbowPoints(prep: Prep, lk: RNode, sides: LinkSides | null): Point[] {
  const plan = prep.links.get(lk);
  if (plan === undefined) {
    return [];
  }
  const a = prep.frames.get(plan.ends[0]);
  const c = prep.frames.get(plan.ends[1]);
  if (a === undefined || c === undefined) {
    return [];
  }
  let sa = sides?.starts.get(lk);
  let sc = sides?.finishes.get(lk);
  if (sa === undefined || sc === undefined) {
    // The link was not in the groups: sides still have to be known for the drawing.
    const [from, to] = routeSides(prep, lk, a, c);
    sa = { side: from, f: 0.5 };
    sc = { side: to, f: 0.5 };
  }
  const pa = sideAnchor(a, sa.side, sa.f);
  const pc = sideAnchor(c, sc.side, sc.f);
  // A side that is a horizontal edge makes the line leave upwards or downwards, and the other way round.
  const leaves = !sideVertical(sa.side);
  const enters = !sideVertical(sc.side);
  if (leaves && !enters) {
    return [pa, { x: pa.x, y: pc.y }, pc];
  }
  if (!leaves && enters) {
    return [pa, { x: pc.x, y: pa.y }, pc];
  }
  if (leaves) {
    const my = (pa.y + pc.y) / 2;
    return [pa, { x: pa.x, y: my }, { x: pc.x, y: my }, pc];
  }
  const mx = (pa.x + pc.x) / 2;
  return [pa, { x: mx, y: pa.y }, { x: mx, y: pc.y }, pc];
}

/** The polyline with the interior corners rounded by r (go/scene elbowPath). */
function elbowPath(ctx: Canvas2D, points: Point[], r: number): void {
  ctx.moveTo(points[0]!.x, points[0]!.y);
  const toward = (from: Point, to: Point, len: number): Point => {
    const d = Math.hypot(to.x - from.x, to.y - from.y);
    return d === 0 ? { ...from } : { x: from.x + ((to.x - from.x) / d) * len, y: from.y + ((to.y - from.y) / d) * len };
  };
  for (let i = 1; i < points.length - 1; i += 1) {
    const prev = points[i - 1]!;
    const cur = points[i]!;
    const next = points[i + 1]!;
    const rr = Math.min(r, Math.hypot(cur.x - prev.x, cur.y - prev.y) / 2, Math.hypot(next.x - cur.x, next.y - cur.y) / 2);
    if (rr <= 0) {
      ctx.lineTo(cur.x, cur.y);
      continue;
    }
    const before = toward(cur, prev, rr);
    const after = toward(cur, next, rr);
    ctx.lineTo(before.x, before.y);
    ctx.quadraticCurveTo(cur.x, cur.y, after.x, after.y);
  }
  const last = points[points.length - 1]!;
  ctx.lineTo(last.x, last.y);
}

/** The ends and the look of every link, resolved once: go/scene's prep.links, before anything is drawn. */
function planLinks(root: RNode, prep: Prep): void {
  const walk = (parent: RNode): void => {
    const byName = new Map<string, RNode[]>();
    const byItem = new Map<DiagramItem, RNode>();
    for (const k of parent.kids) {
      if (k.kind === "item") {
        const list = byName.get(k.name.normalize("NFC")) ?? [];
        list.push(k);
        byName.set(k.name.normalize("NFC"), list);
        if (k.src?.item != null) {
          byItem.set(k.src.item, k);
        }
      }
    }
    for (const k of parent.kids) {
      if (k.kind !== "link") {
        continue;
      }
      const model = k.src?.item;
      const link = model != null && model.kind === "Link" ? (model as LinkItem) : null;
      // The model resolved source-index/target-index to items, which tells apart ends that share a name.
      const end = (name: string, propName: string, want: DiagramItem | null): RNode | null => {
        const direct = want === null ? undefined : byItem.get(want);
        if (direct !== undefined) {
          return direct;
        }
        const index = Math.trunc(num(prep, k, propName) ?? 0);
        const list = byName.get(name.normalize("NFC")) ?? [];
        return index >= 0 && index < list.length ? list[index]! : null;
      };
      const a = end(k.from, "source-index", link?.source ?? null);
      const c = end(k.to, "target-index", link?.target ?? null);
      if (a === null || c === null) {
        prep.diag.fail(k.src ?? { line: 0, col: 0 }, `link "${k.from}" "${k.to}": both ends must be items of the same parent`);
        continue;
      }
      if (prep.frames.get(a) === undefined || prep.frames.get(c) === undefined) {
        continue; // an end is hidden: the link goes with it
      }
      const line = dashOf(prep, k);
      prep.links.set(k, {
        ends: [a, c],
        color: colorOf(prep, k, "color", colorBlack),
        width: num(prep, k, "line-width") ?? 1,
        dash: line.dash,
        phase: line.phase,
      });
      checkRoute(prep, k);
    }
    prep.linkSides.set(parent, elbowSides(prep, parent));
    for (const k of parent.kids) {
      if (k.kind === "item") {
        walk(k);
      }
    }
  };
  walk(root);
}

/** A link line between the boxes of its two ends: straight edge to edge, the elbow route, or through its
 * control point (go/scene/build.go link). */
function drawLink(ctx: Canvas2D, prep: Prep, lk: RNode, sides: LinkSides | null): void {
  const plan = prep.links.get(lk);
  if (plan === undefined) {
    return;
  }
  const a = prep.frames.get(plan.ends[0]);
  const b = prep.frames.get(plan.ends[1]);
  if (a === undefined || b === undefined) {
    return;
  }
  const pa = { x: a.rx + a.rw / 2, y: a.ry + a.rh / 2 };
  const pb = { x: b.rx + b.rw / 2, y: b.ry + b.rh / 2 };
  if (pa.x === pb.x && pa.y === pb.y) {
    return;
  }
  const ctrl = linkControl(prep, lk, pa, pb);
  let from: Point;
  let to: Point;
  ctx.beginPath();
  if (isElbow(prep, lk)) {
    const points = elbowPoints(prep, lk, sides);
    if (points.length < 2) {
      return;
    }
    elbowPath(ctx, points, num(prep, lk, "corner-radius") ?? elbowRadius);
    from = points[points.length - 2]!;
    to = points[points.length - 1]!;
  } else if (ctrl !== null) {
    // The ends leave the boxes toward the control point, not toward each other.
    from = edgePoint(pa, a, ctrl);
    to = edgePoint(pb, b, ctrl);
    ctx.moveTo(from.x, from.y);
    if (isQuad(prep, lk)) {
      ctx.quadraticCurveTo(ctrl.x, ctrl.y, to.x, to.y);
    } else {
      ctx.lineTo(ctrl.x, ctrl.y);
      ctx.lineTo(to.x, to.y);
    }
    // The arrow keeps the direction of the last stretch: the control point into the target.
    from = ctrl;
  } else {
    from = edgePoint(pa, a, pb);
    to = edgePoint(pb, b, pa);
    ctx.moveTo(from.x, from.y);
    ctx.lineTo(to.x, to.y);
  }
  ctx.strokeStyle = sceneCss(plan.color);
  ctx.lineWidth = plan.width;
  ctx.setLineDash(plan.dash);
  ctx.lineDashOffset = plan.phase;
  ctx.stroke();
  // The dash belongs to this line: an arrow or the node painted next is not dashed.
  ctx.setLineDash([]);
  ctx.lineDashOffset = 0;
  const display = propOf(prep, lk, "display");
  if (display !== null && propText(display, 0) === "arrow") {
    // go/scene/build.go arrowHead: a triangle of 6 + line width at the target end.
    const size = 6 + plan.width;
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
    ctx.fillStyle = sceneCss(plan.color);
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

// MARK: - Templates (spec 5c, go/scene/template.go)

export interface RenderTemplateOptions extends SceneLoadOptions {
  /** A path from the file root to the element or the use to render; the element itself by default. */
  path?: string;
  /** Values that override the block of a use and the defaults of the declaration (tenniarb gen -set). */
  values?: Record<string, string>;
}

export interface TemplateError {
  message: string;
  line: number;
  col: number;
  file: string;
}

export interface TemplateResult {
  text: string;
  errors: TemplateError[];
}

/** The value the block of use gives a parameter: the last one written. */
function useProp(use: SceneNode | null, name: string): SceneProp | null {
  let found: SceneProp | null = null;
  for (const ov of use?.block ?? []) {
    if (ov.kind === "prop" && ov.prop.name === name) {
      found = ov.prop;
    }
  }
  return found;
}

/** The arguments of a property as words, each an expression evaluated without variables. */
function wordsOf(p: SceneProp): { value: string; ok: boolean } {
  const words: string[] = [];
  for (const a of p.args) {
    const v = evalArg(a, null);
    if (v === null) {
      return { value: words.join(" "), ok: false };
    }
    words.push(v);
  }
  return { value: words.join(" "), ok: true };
}

function templateError(errors: TemplateError[], n: SceneNode | null, message: string): void {
  errors.push({ message, line: n?.line ?? 0, col: n?.col ?? 0, file: n?.file ?? "" });
}

/** The value of every parameter of target: values, the block of the use, the default (spec 5c). */
function templateVars(target: SceneNode, use: SceneNode | null, values: Record<string, string>, errors: TemplateError[]): Record<string, string> {
  const vars: Record<string, string> = {};
  for (const prm of target.params) {
    if (Object.hasOwn(values, prm.name)) {
      vars[prm.name] = values[prm.name]!;
      continue;
    }
    const p = useProp(use, prm.name) ?? prm.def;
    if (p === null) {
      templateError(errors, target, `element "${target.name}" needs "${prm.name}": set it in the block of the use or with -set`);
      continue;
    }
    const { value, ok } = wordsOf(p);
    if (!ok) {
      templateError(errors, target, `parameter "${prm.name}": the expression is not evaluated`);
    }
    vars[prm.name] = value;
  }
  const unknown = Object.keys(values)
    .filter((name) => !target.params.some((p) => p.name === name))
    .sort();
  for (const name of unknown) {
    const declared = target.params.length === 0 ? "it declares no parameters" : `parameters: ${target.params.map((p) => `"${p.name}"`).join(", ")}`;
    templateError(errors, target, `unknown parameter "${name}" (${declared})`);
  }
  return vars;
}

/** A node the use passes to a slot as code: the title it sets, and its name otherwise. */
function nodeText(n: SceneNode, errors: TemplateError[]): string {
  let title: SceneProp | null = null;
  for (const p of n.props) {
    if (p.name === "title") {
      title = p;
    }
  }
  if (title === null) {
    return n.name;
  }
  const { value, ok } = wordsOf(title);
  if (!ok) {
    templateError(errors, n, "the title of the slot content is not evaluated");
  }
  return value;
}

/** The content of the slots, by normalized name ("" for the default one), with the fallback of the component. */
function templateSlots(target: SceneNode, use: SceneNode | null, errors: TemplateError[]): Map<string, string> {
  const slots = new Map<string, string>();
  for (const ov of use?.block ?? []) {
    if (ov.kind === "node") {
      slots.set("", (slots.get("") ?? "") + nodeText(ov.node, errors));
    } else if (ov.kind === "slot") {
      const name = normalizeName(ov.name);
      if (!slots.has(name)) {
        slots.set(name, ""); // a slot filled with no node still counts as filled
      }
      for (const sub of ov.block) {
        if (sub.kind === "node") {
          slots.set(name, (slots.get(name) ?? "") + nodeText(sub.node, errors));
        }
      }
    }
  }
  const walk = (n: SceneNode): void => {
    for (const c of (n.mount ?? n).content) {
      if (c.kind !== "slot") {
        walk(c);
        continue;
      }
      if (!slots.has(c.key)) {
        let fill = "";
        for (const sub of c.content) {
          if (sub.kind === "item") {
            fill += nodeText(sub, errors);
          }
        }
        if (fill !== "") { // an empty fallback leaves the slot unfilled, and that is an error
          slots.set(c.key, fill);
        }
      }
    }
  };
  walk(target);
  return slots;
}

/** "<slot/>" or "<slot name/>" at the start of s: the normalized name, "" for the default slot, and its length. */
function slotTag(s: string): { name: string; length: number } | null {
  if (!s.startsWith("<slot")) {
    return null;
  }
  const end = s.indexOf(">");
  if (end < 0) {
    return null;
  }
  const raw = s.slice(5, end);
  const name = (raw.endsWith("/") ? raw.slice(0, -1) : raw).trim();
  if (!/^[A-Za-z0-9_-]*$/.test(name)) {
    return null; // a self-closing slot of another form: the text stays
  }
  return { name: normalizeName(name), length: end + 1 };
}

/** The text of a template: ${name} is a value, <slot/> and <slot name/> the content the use passes. */
function templateText(tpl: SceneTemplate, vars: Record<string, string>, slots: Map<string, string>, file: string, errors: TemplateError[]): string {
  let out = "";
  for (let i = 0; i < tpl.text.length; ) {
    if (tpl.text.startsWith("${", i)) {
      const j = tpl.text.indexOf("}", i + 2);
      if (j >= 0) {
        const code = tpl.text.slice(i + 2, j);
        const v = evalCode(code, vars);
        if (v === null) {
          errors.push({ message: `\${${code}}: the expression is not evaluated`, line: tpl.line, col: tpl.col, file });
        } else {
          out += v;
        }
        i = j + 1;
        continue;
      }
    }
    const tag = slotTag(tpl.text.slice(i));
    if (tag !== null) {
      const s = slots.get(tag.name);
      if (s !== undefined) {
        out += s;
      } else if (tag.name === "") {
        errors.push({ message: "the default slot has no content: pass nodes in the block of the use", line: tpl.line, col: tpl.col, file });
      } else {
        errors.push({ message: `slot "${tag.name}" has no content: pass nodes in the block of the use`, line: tpl.line, col: tpl.col, file });
      }
      i += tag.length;
      continue;
    }
    out += tpl.text[i];
    i += 1;
  }
  return out;
}

/**
 * The code the element generates (spec 5c): an element or, with `path`, a use of one. The values override
 * the block of the use, and that the defaults; the slots take the nodes of the block. Diagnostics of the
 * file tree (a mounted file that does not read) are not part of the errors, as in go/scene.
 */
export function renderTemplate(element: Element, options: RenderTemplateOptions = {}): TemplateResult {
  const diag = new Diagnostics();
  const file = fileTree(element, diag, options);
  const errors: TemplateError[] = [];
  let use: SceneNode | null = null;
  let target: SceneNode | null;
  if (options.path !== undefined && options.path !== "") {
    // The model's file root is an element ("Root"); go/scene's file node holds the top-level elements.
    const found = lookup(file.elems[0] ?? file, options.path);
    if (found.node === null) {
      errors.push({ message: found.message, line: 0, col: 0, file: file.file });
      return { text: "", errors };
    }
    target = found.node;
  } else {
    target = sceneNodeFor(file, element);
  }
  if (target === null) {
    errors.push({ message: `element "${element.name}" is not in the file tree`, line: 0, col: 0, file: file.file });
    return { text: "", errors };
  }
  if (target.kind === "use") {
    use = target;
    const found = lookup(target.parent, target.path);
    if (found.node === null) {
      templateError(errors, target, found.message);
      return { text: "", errors };
    }
    target = found.node;
  }
  if (target.template === null) {
    templateError(errors, target, `the ${target.kind} "${target.name}" has no template`);
    return { text: "", errors };
  }
  const vars = templateVars(target, use, options.values ?? {}, errors);
  const slots = templateSlots(target, use, errors);
  const text = templateText(target.template, vars, slots, target.file, errors);
  return errors.length > 0 ? { text: "", errors } : { text, errors };
}
