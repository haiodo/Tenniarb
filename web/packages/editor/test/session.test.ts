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
