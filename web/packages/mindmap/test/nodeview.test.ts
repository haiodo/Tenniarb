// mindmapNodeView against a fake EditorView: dispatched steps are applied to a string document, as ProseMirror would.
import assert from "node:assert/strict";
import { test } from "node:test";
import "../../render/test/helpers.ts"; // measure context + fonts
import { MindmapDoc } from "../src/doc.ts";
import { mindmapNodeView } from "../src/index.ts";
import type { MindmapHandle, MindmapOptions, PMNode, PMTransaction } from "../src/index.ts";

const TEXT = `item "A" {\n    pos 0 0\n}\nitem "B" {\n    pos 150 0\n}\nitem "C" {\n    pos 150 -60\n}\nlink "A" "B"\nlink "A" "C"\n`;

test("mindmapNodeView: local edits become one transaction at the node, node updates reach the map", () => {
  (globalThis as { document?: unknown }).document = { createElement: () => ({ style: {} }) };
  const block = { name: "tenniarb" };
  // The block opens at 5 (after "<p>abc</p>"), its text starts at 6.
  let node: PMNode = { type: block, textContent: TEXT };
  const steps: [number, number, string][] = [];
  const metas: Record<string, unknown>[] = [];
  const view = {
    editable: true,
    get state() {
      const tr: PMTransaction & { ops: [number, number, string][]; meta: Record<string, unknown> } = {
        ops: [],
        meta: {},
        replaceWith(from, to, node) {
          this.ops.push([from, to, (node as { text: string }).text]);
          return this;
        },
        delete(from, to) {
          this.ops.push([from, to, ""]);
          return this;
        },
        setMeta(key, value) {
          this.meta[key] = value;
          return this;
        },
      };
      return { tr, schema: { text: (text: string) => (assert.ok(text !== ""), { text }) } };
    },
    dispatch(tr: PMTransaction) {
      const { ops, meta } = tr as PMTransaction & { ops: [number, number, string][]; meta: Record<string, unknown> };
      steps.push(...ops);
      metas.push(meta);
      let text = node.textContent;
      for (const [from, to, insert] of ops) text = text.slice(0, from - 6) + insert + text.slice(to - 6);
      node = { type: block, textContent: text };
      nv.update(node);
    },
  };
  let mm: MindmapDoc | null = null;
  const fake = (_el: HTMLElement, o: MindmapOptions): MindmapHandle => {
    const d = new MindmapDoc(o.text, { onChange: o.onChange });
    mm = d;
    return { update: (t) => d.update(t), getText: () => d.text, setPeers() {}, setTheme() {}, setReadonly() {}, fit() {}, destroy() {} };
  };
  const nv = mindmapNodeView({ theme: () => "light" }, fake)(node, view, () => 5);
  const s = mm!.session;
  s.undoManager.groupsByEvent = false;
  const item = (name: string) => s.element.items.find((i) => i.kind === "Item" && i.name === name)!;

  // Delete A: its block and both links go, two hunks in one transaction, last first.
  s.select([item("A")]);
  s.deleteSelection();
  assert.equal(metas.length, 1);
  assert.deepEqual(metas[0], { tenniarbMindmap: true });
  assert.ok(steps.length >= 2 && steps[0]![0] > steps[1]![0] && steps.every((st) => st[2] === ""), JSON.stringify(steps));
  assert.equal(node.textContent, mm!.text);
  assert.equal(node.textContent, `item "B" {\n    pos 150 0\n}\nitem "C" {\n    pos 150 -60\n}\n`);

  // An edit inside an item: a replace with a text node.
  s.select([item("C")]);
  s.setQuickStyle("color", "red");
  assert.equal(node.textContent, mm!.text);
  assert.match(node.textContent, /pos 150 -60\n {4}color red\n/);

  // Someone else's edit arrives as a node update; another node type is refused.
  const remote = node.textContent.replace("pos 150 0", "pos 300 0");
  assert.equal(nv.update({ type: block, textContent: remote }), true);
  assert.equal(item("B").x, 300);
  assert.equal(nv.update({ type: { name: "paragraph" }, textContent: "" }), false);
  assert.equal(nv.stopEvent({} as Event), true);
  assert.equal(nv.ignoreMutation(), true);
});
