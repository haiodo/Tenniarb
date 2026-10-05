// A session over the block text, without DOM: local edits come out as line changes, `update` takes any text in.
import { Element, ElementModel } from "@tenniarb/core";
import { EditorSession } from "@tenniarb/editor/session";
import type { SessionOptions } from "@tenniarb/editor/session";
import { diffLines, keys, parseBlock, reconcile, writeBlock } from "./text.ts";
import type { Block, Change } from "./text.ts";

export interface DocOptions extends Pick<SessionOptions, "darkMode" | "readonly" | "measureContext" | "onRedraw"> {
  onChange?: (changes: Change[], text: string) => void;
  /** Initially when there are any, then whenever the skipped statements change. */
  onErrors?: (errors: Block["errors"]) => void;
}

export class MindmapDoc {
  readonly session: EditorSession;
  text = "";
  private block: Block;
  private readonly opts: DocOptions;

  constructor(text: string, opts: DocOptions = {}) {
    this.opts = opts;
    const element = new Element("mindmap");
    new ElementModel().add(element);
    this.block = parseBlock(text);
    reconcile(element, this.block);
    this.text = text;
    this.session = new EditorSession(element, { ...opts, onChange: () => this.flush() });
    if (this.block.errors.length > 0) opts.onErrors?.(this.block.errors);
  }

  /**
   * Text from the host: its own echo is a no-op; items keep identity by key, so selection and a drag survive.
   * Any other text clears the session undo: its steps restore whole values that someone else may have changed since.
   */
  update(text: string): void {
    if (text === this.text) return;
    this.session.undoManager.removeAllActions();
    this.load(text);
  }

  private load(text: string): void {
    const prev = this.block;
    this.text = text;
    this.block = parseBlock(text);
    reconcile(this.session.element, this.block, prev);
    if (JSON.stringify(this.block.errors) !== JSON.stringify(prev.errors)) this.opts.onErrors?.(this.block.errors);
    this.session.rebuild();
    this.opts.onRedraw?.();
  }

  keys(items = this.session.selection): string[] {
    const all = this.session.element.items;
    const k = keys(all);
    return items.flatMap((i) => (all.includes(i) ? [k[all.indexOf(i)]!] : []));
  }

  private flush(): void {
    const next = writeBlock(this.block, this.session.element.items);
    if (next === this.text) return;
    const changes = diffLines(this.text, next);
    // Parsed back so that the model is what others read: integer positions, regions of the new text.
    this.load(next);
    this.opts.onChange?.(changes, next);
  }
}
