import assert from "node:assert/strict";
import { test } from "node:test";
import { readTenn } from "@tenniarb/core";
import "../../render/test/helpers.ts"; // measure context + fonts
import { EditorSession } from "../src/session.ts";

const SRC = `element "A" {
  item "Alpha" { pos 0 0 }
  item "Beta" {
    pos 100 0
    body "find me"
  }
  element "A1" {
    item "Gamma" {
      pos 0 0
      body {
        text "Deep alpha text"
      }
    }
  }
}
element "B" {
  item "Delta" { pos 0 0 }
}`;

function make(readonly = false) {
  const changes: string[] = [];
  let elements = 0;
  const root = readTenn(SRC)!;
  const s = new EditorSession(root.elements[0]!, { evaluate: false, readonly, onChange: (t) => changes.push(t), onElement: () => elements++ });
  const [a, b] = root.elements;
  return { s, root, a: a!, a1: a!.elements[0]!, b: b!, changes, elements: () => elements };
}

test("setElement switches the scene and drops the selection; undo history stays", () => {
  const { s, b, elements } = make();
  s.selection = [s.element.items[0]!];
  s.setElement(b);
  assert.equal(s.element, b);
  assert.deepEqual(s.selection, []);
  assert.equal(s.scene.drawables.size, 1);
  assert.equal(elements(), 1);
  s.setElement(b);
  assert.equal(elements(), 1);
});

test("reveal selects an item of another element", () => {
  const { s, b } = make();
  const delta = b.items[0]!;
  s.reveal(delta);
  assert.equal(s.element, b);
  assert.deepEqual(s.selection, [delta]);
});

test("renameElement, addElement, duplicateElement, removeElement: one undo step each, onChange fires", () => {
  const { s, root, a, a1, changes } = make();
  s.renameElement(a1, "Inner");
  assert.equal(a1.name, "Inner");
  assert.equal(changes.length, 1);
  assert.match(changes[0]!, /element "Inner"/);
  s.renameElement(a1, "Inner"); // unchanged: nothing
  assert.equal(changes.length, 1);
  s.undo();
  assert.equal(a1.name, "A1");

  const added = s.addElement(a)!;
  assert.deepEqual(a.elements.at(-1), added);
  s.undo();
  assert.equal(a.elements.length, 1);
  const top = s.addElement()!;
  assert.equal(root.elements.at(-1), top);
  s.undo();

  const copy = s.duplicateElement(a1)!;
  assert.equal(a.elements[1], copy);
  assert.notEqual(copy.id, a1.id);
  assert.equal(copy.items.length, 1);
  s.undo();
  assert.equal(a.elements.length, 1);

  assert.equal(s.removeElement(a1), true);
  assert.equal(a.elements.length, 0);
  s.undo();
  assert.equal(a.elements[0], a1);
});

test("removing the edited element, and undoing the add of it, falls back to a live element", () => {
  const { s, a, a1 } = make();
  s.setElement(a1);
  s.removeElement(a1);
  assert.equal(s.element, a);
  s.undo();
  assert.equal(a.elements[0], a1);

  const added = s.addElement(a)!;
  s.setElement(added);
  s.undo();
  assert.equal(s.element, a);
  s.redo();
  assert.equal(a.elements.at(-1), added);
});

test("the last top-level element cannot be removed", () => {
  const { s, root, a, b } = make();
  assert.equal(s.removeElement(b), true);
  assert.equal(s.removeElement(a), false);
  assert.equal(root.elements.length, 1);
});

test("moveElement reparents as one undo step; into itself or a descendant is refused", () => {
  const { s, a, a1, b } = make();
  assert.equal(s.moveElement(a, a1), false);
  assert.equal(s.moveElement(a, a), false);
  assert.equal(s.moveElement(a1, b), true);
  assert.deepEqual([a.elements.length, b.elements[0], a1.parent], [0, a1, b]);
  s.undo();
  assert.deepEqual([a.elements[0], b.elements.length], [a1, 0]);
});

test("readonly: no element edits, navigation works", () => {
  const { s, a, a1, b, changes } = make(true);
  assert.equal(s.addElement(a), null);
  assert.equal(s.duplicateElement(a1), null);
  assert.equal(s.removeElement(a1), false);
  assert.equal(s.moveElement(a1, b), false);
  s.renameElement(a1, "X");
  assert.equal(a1.name, "A1");
  assert.equal(changes.length, 0);
  s.setElement(b);
  assert.equal(s.element, b);
});
