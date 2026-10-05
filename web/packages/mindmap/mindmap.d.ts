// Public API of @tenniarb/mindmap; src/index.ts implements it.

export type Theme = "light" | "dark";

/** Replaces [from, to) of the previous text: whole lines, ascending, not overlapping. */
export interface Change {
  from: number;
  to: number;
  insert: string;
}

/**
 * Another user's selection, drawn as an outline in `color` with a `name` label. Keys as `onSelection` gives them:
 * `item:<name>#<n>` (n-th item of that name in the text), `link:<source key>><target key>#<n>`; unknown keys are skipped.
 */
export interface Peer {
  id: string;
  name: string;
  color: string;
  keys: string[];
}

/** A statement the tolerant parser skipped; `line` is 1-based in the block text. */
export interface BlockError {
  line: number;
  message: string;
}

export interface MindmapOptions {
  /** Block text: the body of one element, without the `element "..." { }` wrapper. */
  text: string;
  theme: Theme;
  readonly?: boolean;
  /** Directory with Inter-*.woff2: load Inter and use it instead of the system font (for every map on the page). */
  inter?: string;
  /** After each local edit; `text` is the whole new block. A drag reports once, on drop. */
  onChange?(changes: Change[], text: string): void;
  /**
   * Given: Cmd+Z / Cmd+Shift+Z go to the host (Yjs UndoManager, y-prosemirror `undo`) instead of the map's own undo.
   * Without them the map keeps its own history of local edits, and any `update` with someone else's text clears it:
   * its steps restore whole values that may be stale by then. Collaborative hosts should pass both.
   */
  onUndo?(): void;
  onRedo?(): void;
  /** Keys of the selected items on every change, including a selection lost or re-keyed by `update`. */
  onSelection?(keys: string[]): void;
  /** Skipped statements, on create when there are any and whenever they change; a corner badge shows the count too. */
  onErrors?(errors: BlockError[]): void;
}

export interface MindmapHandle {
  /**
   * Text from the host (an echo of onChange or someone else's edit); never throws, broken item blocks are skipped.
   * Items keep identity (selection, a drag, peers) by key. A rename is a new key, so it is guessed: when exactly one item
   * key disappears and one appears, at the same `pos` or on the same first line of its block, the old item takes the new name.
   */
  update(text: string): void;
  getText(): string;
  /** Replaces all peers. Their keys follow renames made by `update` until the next call. */
  setPeers(peers: Peer[]): void;
  setTheme(theme: Theme): void;
  setReadonly(readonly: boolean): void;
  fit(): void;
  destroy(): void;
}

export interface RenderResult {
  svg: string;
  width: number;
  height: number;
}

/** Interactive map in `el` (give it a size). */
export declare function create(el: HTMLElement, opts: MindmapOptions): MindmapHandle;

/** Static SVG of a block (previews, export). Needs a DOM for text measuring. */
export declare function render(text: string, opts?: { theme?: Theme }): RenderResult;

// ProseMirror, structurally: just what mindmapNodeView touches, so that the package does not depend on prosemirror.
export interface PMNode {
  readonly type: unknown;
  readonly textContent: string;
}

export interface PMTransaction {
  replaceWith(from: number, to: number, content: unknown): PMTransaction;
  delete(from: number, to: number): PMTransaction;
  setMeta(key: string, value: unknown): PMTransaction;
}

export interface PMView {
  readonly state: { readonly tr: PMTransaction; readonly schema: { text(text: string): unknown } };
  readonly editable: boolean;
  dispatch(tr: PMTransaction): void;
}

export interface NodeViewOptions {
  /** Read once per node view; later changes go through `map.setTheme` (subscribe to the theme store in the host). */
  theme(): Theme;
  inter?: string;
  /** Cmd+Z / Cmd+Shift+Z, e.g. the editor's undo / redo commands. */
  undo?(): void;
  redo?(): void;
  onSelection?(keys: string[]): void;
  onErrors?(errors: BlockError[]): void;
}

export interface MindmapNodeView {
  /** 360px high by default; style `.tenniarb-mindmap` or `dom` to change it. */
  dom: HTMLElement;
  /** For setTheme, setPeers, fit. */
  map: MindmapHandle;
  /** False for another node type; otherwise `map.update(node.textContent)` and the editable state. */
  update(node: PMNode): boolean;
  destroy(): void;
  /** True: events in the map are the map's. */
  stopEvent(event: Event): boolean;
  /** True: the map redraws its own DOM. */
  ignoreMutation(): boolean;
}

/**
 * ProseMirror NodeView factory for a text block (`code: true`, `content: "text*"`) whose text is the map.
 * A local edit is one transaction of `replaceWith` / `delete` at `getPos() + 1 + offset` with meta `tenniarbMindmap: true`;
 * readonly follows `view.editable`.
 */
export declare function mindmapNodeView(opts: NodeViewOptions): (node: PMNode, view: PMView, getPos: () => number | undefined) => MindmapNodeView;
