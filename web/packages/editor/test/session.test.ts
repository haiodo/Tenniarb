import assert from "node:assert/strict";
import { test } from "node:test";
import { readTenn } from "@tenniarb/core";
import type { DiagramItem } from "@tenniarb/core";
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
