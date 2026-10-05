import assert from "node:assert/strict";
import { test } from "node:test";
import { readTenn, toStr } from "@tenniarb/core";
import "../../render/test/helpers.ts"; // measure context + fonts
import { EditorSession } from "../src/session.ts";
import { optionNodes, quickStylesFor } from "../src/styles.ts";

const SRC = `element "D" {
  styles { big { font-size 30 } }
  item "A" { pos 0 0 }
  item "B" {
    pos 200 0
    use-style big
  }
  link "A" "B" { display arrow }
}`;

function make(readonly = false) {
  const changes: string[] = [];
  const s = new EditorSession(readTenn(SRC)!.elements[0]!, { readonly, onChange: (t) => changes.push(t) });
  const [a, b, link] = s.element.items;
  return { s, changes, a: a!, b: b!, link: link! };
}

test("quickStylesFor: links have no item-only entries, display differs", () => {
  const link = quickStylesFor("Link").map((q) => q.prop);
  assert.ok(!link.includes("border-color") && !link.includes("width") && link.includes("color"));
  assert.deepEqual(quickStylesFor("Link").find((q) => q.prop === "display")!.options.slice(0, 2), ["solid", "arrow"]);
  assert.equal(quickStylesFor("Item").find((q) => q.prop === "display")!.options[0], "rect");
});

test("optionNodes: marker is a string, others split into idents", () => {
  assert.equal(optionNodes("marker", "🔥")[0]!.kind, "StringLit");
  assert.equal(toStr(optionNodes("layout", "top right")[1]!, 0, false), "right");
  assert.equal(optionNodes("shadow", "5 -5").length, 2);
});

test("styleNames lists named styles without item / line", () => {
  assert.deepEqual(make().s.styleNames(), ["big"]);
});

test("applyStyle: whole selection in one undo step, same style is a no-op", () => {
  const { s, a, b, changes } = make();
  s.selection = [a, b];
  s.applyStyle("big");
  assert.equal(changes.length, 1); // b already has it
  assert.match(s.propsText(a), /use-style big/);
  s.applyStyle("big");
  assert.equal(changes.length, 1);
  s.undo();
  assert.doesNotMatch(s.propsText(a), /use-style/);
  assert.match(s.propsText(b), /use-style big/);
});

test("applyStyle: two items change in one undo step", () => {
  const { s, a, b, changes } = make();
  s.selection = [a, b];
  s.applyStyle("other");
  assert.equal(changes.length, 1);
  assert.match(s.propsText(a), /use-style other/);
  assert.match(s.propsText(b), /use-style other/);
  s.undo();
  assert.doesNotMatch(s.propsText(a), /use-style/);
  assert.match(s.propsText(b), /use-style big/);
});

test("applyStyle replaces an existing use-style", () => {
  const { s, b } = make();
  s.selection = [b];
  s.applyStyle("other");
  assert.match(s.propsText(b), /use-style other/);
  assert.doesNotMatch(s.propsText(b), /use-style big/);
});

test("defineStyle adds new_style_N, undo removes it", () => {
  const { s } = make();
  assert.equal(s.defineStyle(), "new_style_2");
  assert.deepEqual(s.styleNames(), ["big", "new_style_2"]);
  s.undo();
  assert.deepEqual(s.styleNames(), ["big"]);
});

test("setQuickStyle: sets then replaces, single selection only, undo, links too", () => {
  const { s, a, b, link, changes } = make();
  s.selection = [a, b];
  s.setQuickStyle("color", "red");
  assert.equal(changes.length, 0);
  s.selection = [a];
  s.setQuickStyle("color", "red");
  s.setQuickStyle("color", "blue");
  assert.match(s.propsText(a), /color blue/);
  assert.doesNotMatch(s.propsText(a), /color red/);
  s.undo(); // both calls share one event, so one undo step
  assert.doesNotMatch(s.propsText(a), /color/);
  s.selection = [link];
  s.setQuickStyle("display", "arrows");
  assert.match(s.propsText(link), /display arrows/);
});

test("enableShadows creates styles.item.shadow once, undoable", () => {
  const { s } = make();
  s.enableShadows();
  s.enableShadows();
  assert.equal(s.text().split("shadow -5 -5 5").length, 2);
  assert.ok(s.text().includes("big"));
  s.undo();
  s.undo();
  assert.ok(!s.text().includes("shadow"));
});

test("readonly: no style change", () => {
  const { s, a, changes } = make(true);
  s.selection = [a];
  s.applyStyle("big");
  s.setQuickStyle("color", "red");
  s.enableShadows();
  assert.equal(s.defineStyle(), null);
  assert.equal(changes.length, 0);
});
