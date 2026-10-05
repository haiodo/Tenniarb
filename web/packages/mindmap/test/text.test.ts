import assert from "node:assert/strict";
import { test } from "node:test";
import "../../render/test/helpers.ts"; // measure context + fonts
import { MindmapDoc } from "../src/doc.ts";
import { diffLines, keys, parseBlock, printItems } from "../src/text.ts";
import type { Change } from "../src/text.ts";

export const apply = (text: string, changes: Change[]): string => changes.reduceRight((t, c) => t.slice(0, c.from) + c.insert + t.slice(c.to), text);

const SRC = `styles {
    hot { color red }
}
item "Root" {
    pos 0 0
    use-style hot
}
item "A" {
    pos 150 0
    color blue
}
item "B" {
    pos 150 -60
}
link "Root" "A"
link "Root" "B" {
    display arrow
}
`;

function make(text = SRC) {
  const log: { changes: Change[]; text: string }[] = [];
  const doc = new MindmapDoc(text, { onChange: (changes, t) => log.push({ changes, text: t }) });
  const s = doc.session;
  s.undoManager.groupsByEvent = false;
  const item = (name: string) => s.element.items.find((i) => i.kind === "Item" && i.name === name)!;
  // Every emitted change must turn the old text into the new one, by whole lines.
  const check = (before: string) => {
    const last = log.at(-1)!;
    assert.equal(apply(before, last.changes), last.text);
    for (const c of last.changes) {
      assert.ok(c.from === 0 || before[c.from - 1] === "\n", "change starts a line");
      assert.ok(c.to === before.length || before[c.to - 1] === "\n", "change ends a line");
      assert.ok(c.insert === "" || c.insert.endsWith("\n"));
    }
    return last;
  };
  return { doc, s, log, item, check };
}

test("printItems: a property per line, integer pos, link indexes", () => {
  const { items } = parseBlock(`item "A" {\n    pos 10.6 -3.2\n    color red\n}\nitem "A"\nlink "A" "A" {\n    target-index 1\n}\n`);
  const p = printItems(items);
  assert.equal(p.get(items[0]!), `item "A" {\n    pos 11 -3\n    color red\n}`);
  assert.equal(p.get(items[1]!), `item "A" {\n    pos 0 0\n}`);
  assert.equal(p.get(items[2]!), `link "A" "A" {\n    target-index 1\n}`);
});

test("parseBlock: broken blocks and stray lines are skipped, the rest renders", () => {
  const b = parseBlock(`item "A" {\n    pos 1 1\n}\n    pos 9 9\nitem "B" {\n    pos 3 3\nitem "C" {\n    pos 4 4\n}\n}\nitem "D" { "x" }\nitem "E"\n`);
  assert.deepEqual(
    b.items.map((i) => i.name),
    ["A", "C", "E"],
  );
  // The orphan and the unclosed B join the region of A, the stray "}" and the bad D that of C.
  assert.deepEqual(
    b.regions.map((r) => [r.start, r.end, r.items.map((i) => i.name).join()]),
    [
      [0, 6, "A"],
      [6, 11, "C"],
      [11, 12, "E"],
    ],
  );
  assert.doesNotThrow(() => parseBlock(`}}}{{ "\n%{ unclosed`));
  assert.equal(parseBlock("").items.length, 0);
});

test("parseBlock: duplicate properties, the last wins; element statements are kept", () => {
  const b = parseBlock(`color green\nitem "A" {\n    pos 1 1\n    color red\n    pos 5 5\n    color blue\n}\n`);
  const a = b.items[0]!;
  assert.deepEqual([a.x, a.y, [...a.properties].map((p) => p.getIdent(1))], [5, 5, ["blue"]]);
  assert.deepEqual(
    b.props.map((p) => p.getIdent(0)),
    ["color"],
  );
});

test("keys: name and occurrence, links by their ends", () => {
  const { items } = parseBlock(`item "A"\nitem "B"\nitem "A"\nlink "A" "B"\nlink "A" "B" {\n    source-index 1\n}\nlink "A" "B"\n`);
  assert.deepEqual(keys(items), ["item:A#0", "item:B#0", "item:A#1", "link:item:A#0>item:B#0#0", "link:item:A#1>item:B#0#0", "link:item:A#0>item:B#0#1"]);
});

test("diffLines: minimal whole-line hunks with offsets in the old text", () => {
  const a = "a\nb\nc\nd\ne\n";
  const b = "a\nB\nc\nd\nE\nf\n";
  const changes = diffLines(a, b);
  assert.deepEqual(changes, [
    { from: 2, to: 4, insert: "B\n" },
    { from: 8, to: 10, insert: "E\nf\n" },
  ]);
  assert.equal(apply(a, changes), b);
  assert.deepEqual(diffLines(a, a), []);
  assert.deepEqual(diffLines("x\ny", "x\ny\nz\n"), [{ from: 2, to: 3, insert: "y\nz\n" }]);
  for (const [x, y] of [["", "a\n"], ["a\n", ""], ["a\nb\n", "b\na\n"]]) assert.equal(apply(x!, diffLines(x!, y!)), y);
});

test("drag: one change on drop, only the pos line", () => {
  const { s, log, item, check } = make();
  const a = item("A");
  const at = s.scene.drawables.get(a)!.getSelectorBounds();
  const p = { x: at.x + 5, y: at.y + 5 };
  s.down(p);
  for (let i = 1; i <= 10; i++) s.move({ x: p.x + i * 3.3, y: p.y - i * 2.1 });
  assert.equal(log.length, 0);
  s.up({ x: p.x + 33.3, y: p.y - 21 });
  const last = check(SRC);
  assert.equal(log.length, 1);
  assert.deepEqual(last.changes, [{ from: SRC.indexOf("    pos 150 0"), to: SRC.indexOf("    color blue"), insert: "    pos 183 -21\n" }]);
  assert.deepEqual([a.x, a.y], [183, -21]);
});

test("rename rewrites the header and the links to it, in place", () => {
  const { s, item, check } = make();
  s.commitEdit(item("Root"), "name", "Centre");
  const { text, changes } = check(SRC);
  assert.equal(changes.length, 2); // the header, the two adjacent link lines
  assert.equal(text, SRC.replace('item "Root"', 'item "Centre"').replaceAll('link "Root"', 'link "Centre"'));
});

test("Tab adds a linked item at the end; delete removes the item with its links", () => {
  const { s, doc, item, check } = make();
  const b = item("B");
  s.down({ x: s.scene.drawables.get(b)!.getSelectorBounds().x + 2, y: s.scene.drawables.get(b)!.getSelectorBounds().y + 2 });
  s.up({ x: s.scene.drawables.get(b)!.getSelectorBounds().x + 2, y: s.scene.drawables.get(b)!.getSelectorBounds().y + 2 });
  s.addNewItem();
  const added = check(SRC);
  assert.ok(added.text.startsWith(SRC));
  assert.match(added.text.slice(SRC.length), /^item "Untitled 1" \{\n {4}pos \d+ -60\n\}\nlink "B" "Untitled 1"\n$/);
  const before = doc.text;
  s.select([item("A")]);
  s.deleteSelection();
  const removed = check(before);
  assert.ok(!removed.text.includes('item "A"') && !removed.text.includes('link "Root" "A"'));
  assert.ok(removed.text.includes("styles {"));
});

test("quick style: duplicate property lines are rewritten once the item is edited", () => {
  const text = `item "A" {\n    pos 0 10\n    color red\n    color blue\n}\nitem "B" {\n    color red\n    color green\n}\n`;
  const { s, item, check } = make(text);
  s.select([item("A")]);
  s.setQuickStyle("color", "yellow");
  assert.equal(check(text).text, `item "A" {\n    pos 0 10\n    color yellow\n}\nitem "B" {\n    color red\n    color green\n}\n`);
});

test("orphan lines go with the next write of their region, untouched elsewhere", () => {
  const text = `item "A" {\n    pos 0 10\n}\n    color red\nitem "B" {\n    pos 100 10\n}\n    pos 5 5\n`;
  const { s, item, check } = make(text);
  s.select([item("A")]);
  s.setQuickStyle("color", "blue");
  assert.equal(check(text).text, `item "A" {\n    pos 0 10\n    color blue\n}\nitem "B" {\n    pos 100 10\n}\n    pos 5 5\n`);
});

test("update: echo is a no-op, items keep identity, selection survives; bad text never throws", () => {
  const { s, doc, log, item } = make();
  const a = item("A");
  s.select([a]);
  s.setQuickStyle("color", "red");
  const scene = s.scene;
  doc.update(log.at(-1)!.text);
  assert.equal(s.scene, scene, "own echo does not rebuild");
  doc.update(doc.text.replace("    pos 150 -60", "    pos 300 -60").replace('item "Root"', 'item "Hub"'));
  assert.equal(item("A"), a);
  assert.deepEqual(doc.keys(), ["item:A#0"]);
  assert.deepEqual(s.selection, [a]);
  assert.equal(item("B").y, -60);
  assert.equal(item("B").x, 300);
  assert.equal(s.element.items.filter((i) => i.kind === "Link").length, 0, "links to the old name dangle");
  assert.doesNotThrow(() => doc.update(`item "A" {\n    pos 1 2\nitem "Z" { "\n}}}`));
  assert.equal(s.element.items.length, 0);
  assert.deepEqual(doc.keys(), []);
});

test("update during a drag keeps the dragged item under the pointer and commits it on drop", () => {
  const { s, doc, log, item, check } = make();
  const a = item("A");
  const at = s.scene.drawables.get(a)!.getSelectorBounds();
  const p = { x: at.x + 5, y: at.y + 5 };
  s.down(p);
  s.move({ x: p.x + 40, y: p.y });
  doc.update(doc.text.replace("    color blue", "    color green"));
  assert.equal(s.scene.drawables.get(a)!.getSelectorBounds().x, at.x + 40);
  const before = doc.text;
  s.up({ x: p.x + 40, y: p.y });
  assert.equal(log.length, 1);
  assert.match(check(before).text, /pos 190 0\n {4}color green/);
});

test("links to a missing item are not drawn and go with the next write of the region above", () => {
  const text = `item "A" {\n    pos 0 10\n}\nlink "A" "Gone"\nitem "B" {\n    pos 100 10\n}\n`;
  const { s, item, check } = make(text);
  assert.equal(s.element.items.length, 2);
  s.select([item("A")]);
  s.setQuickStyle("color", "red");
  assert.equal(check(text).text, `item "A" {\n    pos 0 10\n    color red\n}\nitem "B" {\n    pos 100 10\n}\n`);
});

test("update: someone else's text clears the session undo, own edits stay undoable", () => {
  const { s, doc, item } = make();
  s.select([item("A")]);
  s.setQuickStyle("color", "red");
  assert.ok(s.undoManager.canUndo);
  doc.update(doc.text); // own echo
  assert.ok(s.undoManager.canUndo);
  doc.update(doc.text.replace("    pos 150 -60", "    pos 150 -90"));
  assert.ok(!s.undoManager.canUndo && !s.undoManager.canRedo);
  const before = doc.text;
  s.undo();
  assert.equal(doc.text, before, "a stale value is not restored");
});

test("update: a remote rename keeps the item, its links and the selection", () => {
  const { s, doc, item } = make();
  const [root, link] = [item("Root"), s.element.items.find((i) => i.kind === "Link")!];
  s.select([root, link]);
  doc.update(doc.text.replace('item "Root"', 'item "Hub"').replaceAll('link "Root"', 'link "Hub"'));
  assert.equal(item("Hub"), root);
  assert.deepEqual(s.selection, [root, link]);
  assert.deepEqual(doc.keys(), ["item:Hub#0", "link:item:Hub#0>item:A#0#0"]);
  // Moved elsewhere too, but the block stays on its line.
  doc.update(doc.text.replace('item "Hub" {\n    pos 0 0', 'item "Core" {\n    pos 10 10'));
  assert.equal(item("Core"), root);
  // Two renames at once are not guessed: new items, the selection is gone.
  doc.update(doc.text.replace('item "Core"', 'item "X"').replace('item "A"', 'item "Y"'));
  assert.notEqual(item("X"), root);
  assert.deepEqual(s.selection, []);
});

test("errors: skipped statements by first line, reported when they change", () => {
  assert.deepEqual(parseBlock(`item "A" {\n    pos 1 1\n}\nitem "B" {\n    pos 2 2\nitem "C"\nitem "D" { "x" }\n`).errors.map((e) => e.line), [4, 7]);
  const seen: number[][] = [];
  const doc = new MindmapDoc(`item "A" {\n`, { onErrors: (errors) => seen.push(errors.map((e) => e.line)) });
  doc.update(`item "A" {\n}\nitem "B" {\n`);
  doc.update(`item "A" {\n}\nitem "B" {\n\n`);
  doc.update(`item "A" {\n}\n`);
  assert.deepEqual(seen, [[1], [3], []]);
});

test("Tab after select() places the new item right of the selected one", () => {
  const { s, item } = make();
  const b = item("B");
  s.select([item("A")]);
  s.select([b]);
  s.addNewItem();
  const added = s.selection[0]!;
  assert.equal(added.y, b.y);
  assert.ok(added.x > b.x + 10 && added.x < b.x + 200, `${added.x}`);
});
