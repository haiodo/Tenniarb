import assert from "node:assert/strict";
import { test } from "node:test";
import { readTenn } from "@tenniarb/core";
import type { DiagramItem, LinkItem } from "@tenniarb/core";
import { DrawableLine } from "@tenniarb/render";
import type { Point } from "@tenniarb/render";
import "../../render/test/helpers.ts"; // measure context + fonts
import { EditorSession } from "../src/session.ts";
import { hitTest, itemsInRect } from "../src/selection.ts";

const SRC = `element "D" {
  item "A" { pos 0 0 }
  item "B" { pos 200 0 }
  item "C" { pos 0 -150 }
  link "A" "B" { display arrow }
  link "B" "C" { display arrow }
}`;

function make(opts: { readonly?: boolean } = {}) {
  const changes: string[] = [];
  const s = new EditorSession(readTenn(SRC)!.elements[0]!, { evaluate: false, onChange: (t) => changes.push(t), ...opts });
  s.undoManager.groupsByEvent = false; // the tests undo step by step inside one tick
  const [a, b, c] = s.element.items;
  const centre = (i: typeof a): Point => {
    const r = s.scene.drawables.get(i!)!.getSelectorBounds();
    return { x: r.x + r.width / 2, y: r.y + r.height / 2 };
  };
  const ends = (i: number): [Point, Point] => {
    const l = s.scene.drawables.get(s.element.items[i]!) as DrawableLine;
    return [{ ...l.source }, { ...l.target }];
  };
  return { s, a: a!, b: b!, c: c!, centre, ends, changes };
}

test("hitTest: box, line between boxes, empty", () => {
  const { s, a, b, centre } = make();
  assert.deepEqual(hitTest(s.scene, centre(a)), [a]);
  const [la] = s.element.items.filter((i) => i.kind === "Link");
  const mid = { x: (centre(a).x + centre(b).x) / 2, y: (centre(a).y + centre(b).y) / 2 };
  assert.deepEqual(hitTest(s.scene, mid), [la]);
  assert.deepEqual(hitTest(s.scene, { x: 5000, y: 5000 }), []);
});

test("click selects, cmd-click toggles, empty click clears, band selects", () => {
  const { s, a, b, c, centre } = make();
  assert.equal(s.down(centre(a)), true);
  s.up(centre(a));
  assert.deepEqual(s.selection, [a]);
  s.down(centre(b), { toggle: true });
  s.up(centre(b));
  assert.deepEqual(s.selection, [a, b]);
  s.down(centre(a), { toggle: true });
  s.up(centre(a));
  assert.deepEqual(s.selection, [b]);
  assert.equal(s.down({ x: 5000, y: 5000 }), false); // nothing grabbed: caller pans
  assert.deepEqual(s.selection, []);

  const sel = (): DiagramItem[] => s.selection; // TS narrows s.selection to never[] after the [] assertion above
  const bc = s.scene.drawables.get(c)!.getSelectorBounds();
  s.down({ x: bc.x - 20, y: bc.y - 20 }, { band: true });
  s.move({ x: bc.x + bc.width + 20, y: bc.y + bc.height + 20 });
  assert.ok(sel().includes(c) && !sel().includes(a)); // plus the B-C link crossing the band
  s.up({ x: bc.x + bc.width + 20, y: bc.y + bc.height + 20 });
  assert.ok(sel().includes(c));
  assert.equal(s.band, null);
  assert.equal(itemsInRect(s.scene, { x: -1e4, y: -1e4, width: 2e4, height: 2e4 }).length, 5);
});

test("drag updates the model through the store as one undo step", () => {
  const { s, a, b, centre, changes } = make();
  s.down(centre(a), { toggle: true });
  s.down(centre(b), { toggle: true });
  s.up(centre(b));
  assert.deepEqual(s.selection, [a, b]);
  const p = centre(a);
  s.down(p);
  s.move({ x: p.x + 20, y: p.y + 10 });
  s.up({ x: p.x + 50, y: p.y + 30 });
  assert.deepEqual([a.x, a.y, b.x, b.y], [50, 30, 250, 30]);
  assert.equal(changes.length, 1);
  assert.match(changes[0]!, /pos 50\.0 30\.0|pos 50 30/);

  s.undo();
  assert.deepEqual([a.x, a.y, b.x, b.y], [0, 0, 200, 0]);
  assert.equal(changes.length, 2);
  assert.equal(s.undoManager.canUndo, false);
  s.redo();
  assert.deepEqual([a.x, a.y, b.x, b.y], [50, 30, 250, 30]);
});

test("a click without movement and a drag that returns home leave no undo entry", () => {
  const { s, a, centre, changes } = make();
  const p = centre(a);
  s.down(p);
  s.up(p);
  s.down(p);
  s.move({ x: p.x + 40, y: p.y });
  s.up(p);
  assert.equal(s.undoManager.canUndo, false);
  assert.equal(changes.length, 0);
});

test("attached lines follow while dragging and match a fresh build after commit", () => {
  const { s, a, centre, ends } = make();
  const ab0 = ends(3);
  const bc0 = ends(4);
  const p = centre(a);
  s.down(p);
  s.move({ x: p.x, y: p.y + 100 });
  const abLive = ends(3);
  assert.notDeepEqual(abLive, ab0);
  assert.deepEqual(ends(4), bc0); // B-C does not touch A
  s.up({ x: p.x, y: p.y + 100 });
  assert.deepEqual(ends(3), abLive);
});

test("dragging a link changes its control point", () => {
  const { s } = make();
  const link = s.element.items[3]!;
  const r = s.scene.drawables.get(link)!.getBounds();
  const p = { x: r.x + r.width / 2, y: r.y + r.height / 2 };
  s.down(p);
  assert.deepEqual(s.selection, [link]);
  s.move({ x: p.x + 30, y: p.y + 40 });
  s.up({ x: p.x + 30, y: p.y + 40 });
  assert.deepEqual([link.x, link.y], [30, 40]);
});

test("readonly: no selection, no drag, no undo", () => {
  const { s, a, centre, changes } = make({ readonly: true });
  assert.equal(s.down(centre(a)), false);
  s.move({ x: 100, y: 100 });
  s.up({ x: 100, y: 100 });
  assert.deepEqual(s.selection, []);
  assert.equal(a.x, 0);
  assert.equal(changes.length, 0);
});

test("rename: one undo step, onChange, unchanged text is a no-op", () => {
  const { s, a, changes } = make();
  assert.equal(s.editTarget(a, "name")!.text, "A");
  s.commitEdit(a, "name", "A");
  assert.equal(changes.length, 0);
  s.commitEdit(a, "name", "Alpha\nline");
  assert.equal(a.name, "Alpha\nline");
  assert.equal(changes.length, 1);
  assert.match(changes[0]!, /Alpha/);
  s.undo();
  assert.equal(a.name, "A");
  assert.equal(s.undoManager.canUndo, false);
});

test("body: created, edited in place, undone", async () => {
  const { s, a, changes } = make();
  s.commitEdit(a, "body", "one");
  assert.equal(s.editTarget(a, "body")!.text, "one");
  await Promise.resolve(); // the undo manager groups by microtask
  s.commitEdit(a, "body", "one\ntwo");
  assert.equal(s.editTarget(a, "body")!.text, "one\ntwo");
  assert.equal(changes.length, 2);
  assert.ok(s.scene.drawables.get(a)!.getSelectorBounds().height > 0);
  s.undo();
  assert.equal(s.editTarget(a, "body")!.text, "one");
  s.undo();
  assert.equal(s.editTarget(a, "body")!.text, "");
  assert.equal(a.properties.get("body"), null);
});

test("copy: selected items and the links between them, as .tenn text", () => {
  const { s, a, b, c } = make();
  assert.equal(s.copyText(), null);
  s.selection = [a, b, s.element.items[3]!];
  const text = s.copyText()!;
  assert.match(text, /item "A"/);
  assert.match(text, /link "A" "B"/);
  assert.doesNotMatch(text, /item "C"/);
  s.selection = [c];
  assert.doesNotMatch(s.copyText()!, /link/);
});

test("paste: new items with remapped links, selected, one undo step; garbage ignored", () => {
  const { s, a, b, changes } = make();
  s.selection = [a, b, s.element.items[3]!];
  const text = s.copyText()!;
  assert.equal(s.paste("item \"broken {"), false);
  assert.equal(s.paste("hello"), false);
  assert.equal(changes.length, 0);

  assert.equal(s.paste(text), true);
  assert.equal(s.element.items.length, 8);
  assert.equal(changes.length, 1);
  const [na, nb, nl] = s.selection as [DiagramItem, DiagramItem, LinkItem];
  assert.deepEqual(s.selection.map((i) => i.kind), ["Item", "Item", "Link"]);
  assert.ok(![a, b].includes(na) && !s.element.items.slice(0, 5).includes(na));
  assert.equal(nl.source, na);
  assert.equal(nl.target, nb);
  assert.ok(s.scene.drawables.has(nl));

  s.undo();
  assert.equal(s.element.items.length, 5);
  assert.deepEqual(s.selection, []);
  assert.equal(s.undoManager.canUndo, false);
});

test("cut and delete remove the selection and its links in one step", () => {
  const { s, b, changes } = make();
  s.selection = [b];
  const text = s.cut()!;
  assert.match(text, /item "B"/);
  assert.equal(s.element.items.length, 2); // A and C stay, both links went with B
  assert.equal(changes.length, 1);
  assert.deepEqual(s.selection, []);
  s.undo();
  assert.equal(s.element.items.length, 5);
  s.selection = [s.element.items[3]!];
  s.deleteSelection();
  assert.equal(s.element.items.length, 4);
  s.undo();
  assert.equal(s.element.items.length, 5);
});

test("duplicate: shifted copies, links to them attach to the copy", () => {
  const { s, b } = make();
  s.selection = [b];
  s.duplicate();
  assert.equal(s.element.items.length, 7); // B copy plus the A->B link cloned onto it
  const [nb, nl] = s.selection as [DiagramItem, LinkItem];
  assert.deepEqual([nb.name, nb.x, nb.y], ["B", 275, 0]);
  assert.equal(nl.target, nb);
  assert.equal(nl.source, s.element.items[0]);
  s.undo();
  assert.equal(s.element.items.length, 5);
});

test("readonly: no edit, paste, delete or duplicate; copy stays", () => {
  const { s, a, changes } = make({ readonly: true });
  assert.equal(s.editTarget(a, "name"), null);
  s.commitEdit(a, "name", "X");
  assert.equal(s.paste("item \"X\" { pos 1 1 }"), false);
  s.selection = [a];
  s.deleteSelection();
  s.duplicate();
  assert.equal(s.element.items.length, 5);
  assert.equal(a.name, "A");
  assert.equal(changes.length, 0);
  assert.match(s.copyText()!, /item "A"/);
});

test("props: text of the selected item or of the element when nothing is selected", () => {
  const { s, a, centre } = make();
  assert.equal(s.propsTarget(), s.element);
  assert.match(s.propsText(s.element), /^name "D"/);
  s.down(centre(a));
  s.up(centre(a));
  assert.equal(s.propsTarget(), a);
  assert.match(s.propsText(a), /pos 0\.0 0\.0/);
});

test("props: apply goes through the store as one undo step and fires onChange", () => {
  const { s, a, changes } = make();
  assert.equal(s.applyProps(a, 'name "A"\npos 0 0\ncolor red\nfontSize 20'), true);
  assert.match(s.propsText(a), /color red/);
  assert.match(s.propsText(a), /fontSize 20/);
  assert.equal(changes.length, 1);
  s.undo();
  assert.doesNotMatch(s.propsText(a), /color red/);
  assert.equal(changes.length, 2);
  s.redo();
  assert.match(s.propsText(a), /color red/);
});

test("props: parse errors, removed target and readonly apply nothing", () => {
  const { s, a, changes } = make();
  assert.equal(s.applyProps(a, "color {"), false);
  s.store.removeItems(s.element, [a], s.undoManager, () => {});
  const n = changes.length;
  assert.equal(s.applyProps(a, 'name "A"'), false);
  assert.equal(changes.length, n);
  assert.equal(make({ readonly: true }).s.applyProps(make().a, 'name "X"'), false);
});

test("props: expression values by line", () => {
  const s = new EditorSession(readTenn(SRC)!.elements[0]!);
  const a = s.element.items[0]!;
  const values = s.propsValues(a, 'name "A"\npos 0 0\nfontSize $(10 + 5)');
  assert.equal(values.get(2), "15");
  assert.equal(s.propsValues(a, "fontSize $(1 +").size, 0);
});

const pos = (...items: DiagramItem[]): number[] => items.flatMap((i) => [i.x, i.y]);

test("arrows: every selected item one grid step (5) snapped to the grid, one undo step", () => {
  const { s, a, b, changes } = make();
  a.x = 3;
  s.selection = [a, b];
  s.moveBy(1, 0);
  assert.deepEqual(pos(a, b), [5, 0, 205, 0]);
  s.moveBy(0, -1);
  assert.deepEqual(pos(a, b), [5, -5, 205, -5]);
  assert.equal(changes.length, 2);
  s.undo();
  assert.deepEqual(pos(a, b), [5, 0, 205, 0]);
  s.moveBy(-1, 0);
  s.moveBy(-1, 0);
  assert.equal(a.x, -5); // Swift drops the remainder of the truncated value: toward zero
});

test("shift / cmd + arrows: resize one item by a grid step, cmd keeps the centre, one undo step", () => {
  const { s, a, changes } = make();
  s.selection = [a];
  const props = () => s.propsText(a);
  s.resizeBy("ArrowRight", false);
  const w = Number(/width ([\d.]+)/.exec(props())![1]);
  assert.equal(changes.length, 1);
  s.resizeBy("ArrowRight", true);
  assert.ok(Math.abs(Number(/width ([\d.]+)/.exec(props())![1]) - (w + 5)) < 1e-3);
  assert.equal(a.x, -2.5);
  s.resizeBy("ArrowDown", false);
  assert.match(props(), /height /);
  assert.equal(changes.length, 3);
  s.undo();
  assert.doesNotMatch(props(), /height /);
  s.resizeBy("ArrowUp", true);
  s.resizeBy("ArrowUp", true);
  assert.equal(Number(/height ([\d.]+)/.exec(props())![1]) >= 10, true);
  s.selection = [a, s.element.items[1]!];
  const n = changes.length;
  s.resizeBy("ArrowRight", false); // more than one selected: nothing
  assert.equal(changes.length, n);
});

test("Tab: a linked item at the pivot; Option+Tab copies the style properties", () => {
  const { s, a } = make();
  s.applyProps(a, 'name "A"\npos 0 0\ncolor red');
  s.selection = [a];
  s.addNewItem(true);
  const [item, link] = [s.element.items.at(-2)!, s.element.items.at(-1) as LinkItem];
  assert.deepEqual([link.kind, link.source, link.target, s.selection], ["Link", a, item, [item]]);
  assert.match(s.propsText(item), /color red/);
  s.undo();
  assert.equal(s.element.items.length, 5);
  s.selection = [a];
  s.addNewItem();
  assert.doesNotMatch(s.propsText(s.element.items.at(-2)!), /color red/);
  s.selection = [s.element.items[3]!]; // a link: nothing
  const n = s.element.items.length;
  s.addNewItem();
  assert.equal(s.element.items.length, n);
});

test("selectAll: everything, items, links, none; readonly keeps the selection empty", () => {
  const { s } = make();
  s.selectAll();
  assert.equal(s.selection.length, 5);
  s.selectAll("Item");
  assert.deepEqual(s.selection.map((i) => i.kind), ["Item", "Item", "Item"]);
  s.selectAll("Link");
  assert.equal(s.selection.length, 2);
  s.select([]);
  assert.equal(s.selection.length, 0);
  const ro = make({ readonly: true }).s;
  ro.selectAll();
  assert.equal(ro.selection.length, 0);
});

test("paste as item: the text and a ${text} title, selected, one undo step", () => {
  const { s, changes } = make();
  s.pasteAsItem("hello\nworld");
  const item = s.element.items.at(-1)!;
  assert.deepEqual([item.name, s.selection, changes.length], ["pasted 1", [item], 1]);
  assert.match(s.propsText(item), /text/);
  assert.match(s.propsText(item), /title .*\$\{text\}/);
  s.undo();
  assert.equal(s.element.items.length, 5);
});

test("paste as item set: an item per non-empty line, arrow links from the first, one undo step", () => {
  const { s, changes } = make();
  s.pasteAsItemSet("one\n\ntwo\nthree");
  const added = s.element.items.slice(5);
  assert.deepEqual(added.map((i) => i.kind + i.name), ["Itemone", "Link", "Itemtwo", "Link", "Itemthree"]);
  assert.deepEqual(added.filter((i) => i.kind === "Item").map((i) => i.y), [0, -35, -70]);
  const [l1, l2] = added.filter((i) => i.kind === "Link") as LinkItem[];
  assert.deepEqual([l1!.source, l2!.source, l1!.target!.name, l2!.target!.name], [added[0], added[0], "two", "three"]);
  assert.match(s.propsText(l1!), /display arrow/);
  assert.equal(changes.length, 1);
  s.undo();
  assert.equal(s.element.items.length, 5);
  s.pasteAsItemSet("\n");
  assert.equal(changes.length, 2);
});

test("align: leading, trailing, top, bottom edges of the selected items, one undo step", () => {
  const { s, a, b, c, changes } = make();
  s.selection = [a, b, c];
  s.align("trailing");
  const right = (i: DiagramItem) => i.x + s.scene.drawables.get(i)!.getBounds().width;
  assert.ok(Math.abs(right(a) - right(b)) < 1e-9 && Math.abs(right(b) - right(c)) < 1e-9);
  s.undo();
  s.align("leading");
  assert.deepEqual([a.x, b.x, c.x], [0, 0, 0]);
  s.undo();
  s.align("top");
  assert.deepEqual([a.y, b.y, c.y], [0, 0, 0]);
  s.undo();
  s.align("bottom");
  const bottom = (i: DiagramItem) => i.y - s.scene.drawables.get(i)!.getBounds().height;
  assert.ok(Math.abs(bottom(a) - bottom(c)) <= 1 && Math.abs(c.y + 150) <= 1); // the target edge is rounded, as in Swift
  assert.equal(changes.length, 7);
});

test("order: forward puts the item last, backward first, one undo step", () => {
  const { s, a, b } = make();
  s.selection = [a];
  s.order(true);
  assert.equal(s.element.items.at(-1), a);
  s.order(false);
  assert.equal(s.element.items[0], a);
  s.undo();
  assert.equal(s.element.items.at(-1), a);
  s.selection = [a, b];
  const n = s.undoManager.canUndo;
  s.order(true);
  assert.equal(s.element.items.at(-1), a);
  assert.equal(n, true);
});

test("edit value: the `value` property (or field-name) as a number, a symbol or a string; one undo step", () => {
  const { s, a, changes } = make();
  assert.equal(s.editTarget(a, "value")!.text, "");
  s.commitEdit(a, "value", "42");
  assert.match(s.propsText(a), /value 42\b/);
  assert.equal(s.editTarget(a, "value")!.text, "42");
  s.commitEdit(a, "value", "3.5x");
  assert.match(s.propsText(a), /value 3\.5x/);
  s.commitEdit(a, "value", "12 apples");
  assert.match(s.propsText(a), /value "12 apples"/);
  s.commitEdit(a, "value", "12 apples");
  assert.equal(changes.length, 3);
  s.undo();
  assert.match(s.propsText(a), /value 3\.5x/);
  s.applyProps(a, 'name "A"\npos 0 0\nfield-name total\ntotal 7');
  assert.equal(s.editTarget(a, "value")!.text, "7");
  s.commitEdit(a, "value", "8");
  assert.match(s.propsText(a), /total 8/);
});

test("test layout: positions change in one undo step", () => {
  const { s, a, b, c, changes } = make();
  const before = pos(a, b, c);
  s.testLayout({ x: -500, y: -400, width: 1000, height: 800 });
  assert.notDeepEqual(pos(a, b, c), before);
  assert.equal(changes.length, 1);
  s.undo();
  assert.deepEqual(pos(a, b, c), before);
});

test("readonly: keys, align, order, paste as item and layout change nothing", () => {
  const { s, a, changes } = make({ readonly: true });
  s.selection = [a];
  s.moveBy(1, 0);
  s.resizeBy("ArrowRight", false);
  s.addNewItem(true);
  s.align("leading");
  s.order(true);
  s.pasteAsItem("x");
  s.pasteAsItemSet("x");
  s.testLayout({ x: 0, y: 0, width: 100, height: 100 });
  assert.equal(changes.length, 0);
});

test("ctrl-drag from an item to another adds a link in one undo step, preview is removed", () => {
  const { s, a, c, centre, changes } = make();
  const links = (): number => s.element.items.filter((i) => i.kind === "Link").length;
  assert.equal(s.down(centre(a), { line: true }), true);
  s.move(centre(c));
  assert.ok(s.scene.lineToDrawable !== null);
  s.up(centre(c));
  assert.equal(s.scene.lineToDrawable, null);
  assert.equal(links(), 3);
  const l = s.element.items.at(-1) as LinkItem;
  assert.deepEqual([l.source, l.target], [a, c]);
  assert.equal(changes.length, 1);
  s.undo();
  assert.equal(links(), 2);
  assert.equal(s.undoManager.canUndo, false);
});

test("ctrl-drag dropped on empty space, on the source, or cancelled adds nothing", () => {
  const { s, a, c, centre, changes } = make();
  s.down(centre(a), { line: true });
  s.move({ x: 5000, y: 5000 });
  s.up({ x: 5000, y: 5000 });
  s.down(centre(a), { line: true });
  s.move({ x: centre(a).x + 3, y: centre(a).y });
  s.up(centre(a));
  s.down(centre(a), { line: true });
  s.move(centre(c));
  s.up(centre(c), true);
  assert.equal(s.element.items.length, 5);
  assert.equal(changes.length, 0);
  assert.equal(s.scene.lineToDrawable, null);
});

test("option-drag moves the item with everything reachable along outgoing links", () => {
  const { s, a, b, c, centre } = make();
  const p = centre(b);
  s.down(p, { alt: true });
  assert.deepEqual(s.selection, [b, c]);
  s.move({ x: p.x + 10, y: p.y + 10 });
  s.up({ x: p.x + 10, y: p.y + 10 });
  assert.deepEqual([a.x, a.y, b.x, b.y, c.x, c.y], [0, 0, 210, 10, 10, -140]);
  assert.equal(s.undoManager.canUndo, true);
  s.undo();
  assert.deepEqual([b.x, c.x], [200, 0]);
});

test("operate: replaces/adds/removes properties on every selected item, one undo step", () => {
  const { s, a, b, changes } = make();
  s.select([a, b]);
  const n = changes.length;
  assert.equal(s.operate("pos 10 20\ncolor red\n-display"), true);
  assert.equal(changes.length, n + 1);
  for (const i of [a, b]) {
    assert.deepEqual([i.x, i.y], [10, 20]);
    assert.match(s.propsText(i), /color red/);
  }
  s.undo();
  assert.deepEqual([a.x, a.y, b.x, b.y], [0, 0, 200, 0]);
  assert.doesNotMatch(s.propsText(a), /color/);
});

test("operate: -name removes, parse error / no selection / no change change nothing", () => {
  const { s, a, changes } = make();
  s.select([a]);
  s.operate("color red");
  assert.equal(s.operate("-color"), true);
  assert.doesNotMatch(s.propsText(a), /color/);
  const n = changes.length;
  assert.equal(s.operate("color {"), false);
  assert.equal(s.operate("-nothing"), true);
  s.select([]);
  assert.equal(s.operate("color red"), false);
  assert.equal(changes.length, n);
  assert.equal(make({ readonly: true }).s.operate("color red"), false);
});

test("click on overlapping items cycles through them, a selected one under the point keeps the selection", () => {
  const s = new EditorSession(readTenn(`element "D" { item "A" { pos 0 0 }\n item "B" { pos 0 0 } }`)!.elements[0]!, { evaluate: false });
  const r = s.scene.drawables.get(s.element.items[0]!)!.getSelectorBounds();
  const p = { x: r.x + r.width / 2, y: r.y + r.height / 2 };
  const click = () => (s.down(p), s.up(p, false), s.selection[0]);
  const first = click();
  assert.equal(click(), first); // already selected: kept
  s.selection = [];
  assert.notEqual(click(), first);
  s.selection = [];
  assert.equal(click(), first);
});

test("paste image: no selection - a new item with image and title, selected, one undo step", () => {
  const { s, changes } = make();
  s.pasteImage("a.png", "QUJD");
  const item = s.selection[0]!;
  assert.equal(item.name, "Untitled 1");
  assert.ok(s.text().includes("QUJD"));
  assert.equal(item.properties.get("title")?.getChild(1)?.getIdentText(), "@(a.png|96)\n${name}");
  assert.equal(changes.length, 1);
  s.undo();
  assert.equal(s.element.items.length, 5);
});

test("paste image / attach image: into the selected item, one undo step; readonly ignores", () => {
  const { s, a, changes } = make();
  s.selection = [a!];
  s.pasteImage("a.png", "QUJD");
  assert.equal(a!.properties.get("image")?.getIdent(1), "a.png");
  assert.equal(a!.properties.get("title")?.getChild(1)?.getIdentText(), "@(a.png|96)\n${name}");
  s.attachImage("b.jpg", "REVG");
  assert.equal(changes.length, 2);
  assert.equal(a!.properties.get("title")?.getChild(1)?.getIdentText(), "@(a.png|96)\n${name}");
  assert.ok(s.text().includes("REVG"));
  s.undo();
  assert.ok(!s.text().includes("REVG") && s.text().includes("QUJD"));
  const ro = make({ readonly: true });
  ro.s.pasteImage("a.png", "QUJD");
  ro.s.attachImage("a.png", "QUJD");
  assert.equal(ro.changes.length, 0);
});

test("outline copy / paste / cut of elements: .tenn text, one undo step each", () => {
  const { s, changes } = make();
  const text = s.copyElement(s.element);
  assert.match(text, /element "D"/);
  assert.equal(s.pasteElements(s.element, "hello"), false);
  assert.equal(s.pasteElements(s.element, 'element "broken {'), false);
  assert.equal(changes.length, 0);

  assert.equal(s.pasteElements(s.element, text), true);
  assert.equal(s.element.elements.length, 1);
  assert.equal(s.element.elements[0]!.items.length, 5);
  assert.equal(changes.length, 1);
  s.undo();
  assert.equal(s.element.elements.length, 0);

  const root = s.root;
  assert.equal(s.pasteElements(root, text), true);
  assert.equal(root.elements.length, 2);
  const pasted = root.elements[1]!;
  assert.equal(s.cutElement(pasted), s.copyElement(pasted));
  assert.equal(root.elements.length, 1);
  assert.equal(s.cutElement(root.elements[0]!).length > 0, true); // last top-level element stays
  assert.equal(root.elements.length, 1);

  const ro = make({ readonly: true });
  assert.equal(ro.s.pasteElements(ro.s.element, text), false);
});
