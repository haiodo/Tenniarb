import { test } from "node:test";
import assert from "node:assert/strict";
import {
  DiagramItem,
  Element,
  ElementModel,
  ElementModelStore,
  ExecutionContext,
  LinkItem,
  ModelEvent,
  newBlockExpr,
  newCommand,
  newIntNode,
  newNode,
  newStrNode,
  newToken,
} from "../src/index.ts";
import type { TennNode, TennToken } from "../src/index.ts";

const exprToken = (literal: string): TennToken => newToken("expression", literal);
const exprNode = (token: TennToken): TennNode => newNode("Expression", token);
const blockToken = (literal: string): TennToken => newToken("expressionBlock", literal);
const blockNode = (token: TennToken): TennNode => newNode("ExpressionBlock", token);

function item(name: string, ...props: TennNode[]): DiagramItem {
  const itm = new DiagramItem("Item", name);
  itm.properties.appendContentsOf(props);
  return itm;
}

function diagram(...items: DiagramItem[]): Element {
  const element = new Element("Diag");
  for (const itm of items) {
    element.add(itm);
  }
  return element;
}

function run(element: Element, options = {}): ExecutionContext {
  const ctx = new ExecutionContext(options);
  ctx.setElement(element);
  return ctx;
}

// ExecutionContextTests.swift

test("testSimpleIntAssignment", () => {
  const tok = exprToken("5");
  const a = item("A", newCommand("x", exprNode(tok)));
  const ctx = run(diagram(a));
  assert.equal(ctx.getEvaluated(a).get(tok), 5);
});

test("testChainedExpression", () => {
  const xTok = blockToken("var x = 5; x");
  const yTok = exprToken("x + 3");
  const a = item("A", newCommand("x", blockNode(xTok)), newCommand("y", exprNode(yTok)));
  const ev = run(diagram(a)).getEvaluated(a);
  assert.equal(ev.get(xTok), 5);
  assert.equal(ev.get(yTok), 8);
});

test("testExpressionWithParens", () => {
  const xTok = blockToken("var x = 5; x");
  const zTok = exprToken("(x * 2)");
  const a = item("A", newCommand("x", blockNode(xTok)), newCommand("z", exprNode(zTok)));
  assert.equal(run(diagram(a)).getEvaluated(a).get(zTok), 10);
});

test("testSiblingItemReference", () => {
  const b = item("B", newCommand("val", newIntNode(42)));
  const refTok = exprToken("items.B.val");
  const a = item("A", newCommand("bval", exprNode(refTok)));
  const ctx = run(diagram(b, a));
  assert.equal(ctx.getEvaluated(a).get(refTok), 42);
});

test("testUpdateOnPositionChange", () => {
  const tok = exprToken("pos.x + 1");
  const a = item("A", newCommand("derived", exprNode(tok)));
  a.x = 10;
  const element = diagram(a);
  const ctx = run(element);
  assert.equal(ctx.getEvaluated(a).get(tok), 11);

  a.x = 20;
  const event = new ModelEvent("Layout", element);
  event.items.set(a, "Update");
  ctx.notifyChanges(event);
  assert.equal(ctx.getEvaluated(a).get(tok), 21);
});

test("testCircularReferenceDoesNotCrash", () => {
  const aTok = exprToken("items.B ? (items.B.bval || 0) + 1 : 1");
  const bTok = exprToken("items.A ? (items.A.aval || 0) + 1 : 1");
  const a = item("A", newCommand("aval", exprNode(aTok)));
  const b = item("B", newCommand("bval", exprNode(bTok)));
  const ctx = run(diagram(a, b));
  assert.notEqual(ctx.getEvaluated(a).get(aTok), undefined);
  assert.notEqual(ctx.getEvaluated(b).get(bTok), undefined);
});

test("testInvalidExpressionReturnsGracefully", () => {
  const tok = exprToken("this is not js !!!");
  const a = item("A", newCommand("broken", exprNode(tok)));
  const ev = run(diagram(a)).getEvaluated(a);
  assert.ok(ev.get(tok) instanceof SyntaxError);
});

test("testInvalidExpressionBlockReturnsGracefully", () => {
  const tok = blockToken("function( { broken }");
  const a = item("A", newCommand("x", blockNode(tok)));
  assert.ok(run(diagram(a)).getEvaluated(a).get(tok) instanceof SyntaxError);
});

test("testGetEvaluatedWithNodeOverride", () => {
  const a = item("A");
  const ctx = run(diagram(a));
  const tok = exprToken("2 + 3");
  const result = ctx.getEvaluated(a, newBlockExpr(newCommand("v", exprNode(tok))), null);
  assert.equal(result.get(tok), 5);
});

test("testUpdateAllCallsNotifier", () => {
  const a = item("A", newCommand("v", exprNode(exprToken("1 + 1"))));
  const ctx = run(diagram(a));
  let called = 0;
  ctx.updateAll(() => called++);
  assert.equal(called, 1);
});

// Engine behaviour beyond the Swift unit tests (calc-*.tenn goldens cover most of it too)

test("error values: thrown value is stored as is, evaluation continues", () => {
  const t1 = blockToken('throw "s"');
  const t2 = exprToken("1 + 1");
  const a = item("A", newCommand("t", blockNode(t1)), newCommand("ok", exprNode(t2)));
  const ev = run(diagram(a)).getEvaluated(a);
  assert.equal(ev.get(t1), "s");
  assert.equal(ev.get(t2), 2);
});

test("a name is deleted before the item is re-evaluated", () => {
  const tok = exprToken("sv + 1");
  const a = item("S", newCommand("sv", exprNode(tok)));
  assert.ok(run(diagram(a)).getEvaluated(a).get(tok) instanceof ReferenceError);
});

test("cycle stops after 100 iterations", () => {
  const cTok = exprToken("(items.D.dval || 0) + 1");
  const dTok = exprToken("(items.C.cval || 0) + 1");
  const c = item("C", newCommand("cval", exprNode(cTok)));
  const d = item("D", newCommand("dval", exprNode(dTok)));
  const ctx = run(diagram(c, d));
  assert.equal(ctx.getEvaluated(c).get(cTok), 200);
  assert.equal(ctx.getEvaluated(d).get(dTok), 201);
});

test("items proxy resolves converted names only", () => {
  const spaced = exprToken("items.my_item.val");
  const raw = exprToken('items["my item"]');
  const my = item("my item", newCommand("val", newIntNode(7)));
  const a = item("A", newCommand("s", exprNode(spaced)), newCommand("r", exprNode(raw)));
  const ev = run(diagram(my, a)).getEvaluated(a);
  assert.equal(ev.get(spaced), 7);
  assert.equal(ev.get(raw), undefined);
});

test("template strings are evaluated", () => {
  const str = newStrNode("Hello ${name} ${n + 1}");
  const a = item("A", newCommand("n", newIntNode(3)), newCommand("title", str));
  assert.equal(run(diagram(a)).getEvaluated(a).get(str.token!), "Hello A 4");
});

test("functions declared in element scope stay visible to items", () => {
  const define = blockToken("function twice(v) { return v * 2 + k }");
  const tok = exprToken("twice(k)");
  const element = diagram(item("A", newCommand("k", newIntNode(5)), newCommand("r", exprNode(tok))));
  element.properties.append(newCommand("define", blockNode(define)));
  const ctx = run(element);
  assert.equal(ctx.getEvaluated(element.items[0]!).get(tok), 15);
});

test("sum, byTag, inputs, outputs, edges", () => {
  const sumTok = exprToken('sum("cost")');
  const sumW = exprToken('sum("cost", "w")');
  const tagTok = exprToken('byTag("cost").map(function(i) { return i.name })');
  const outTok = exprToken("outputs().map(function(i) { return i.name })");
  const inTok = exprToken("inputs().length");
  const edgTok = exprToken("edges().length");
  const a = item("A", newCommand("cost", newIntNode(10)), newCommand("s", exprNode(sumTok)), newCommand("t", exprNode(tagTok)));
  a.properties.append(newCommand("o", exprNode(outTok)));
  const b = item("B", newCommand("cost", newIntNode(5)), newCommand("w", newIntNode(2)));
  b.properties.append(newCommand("sw", exprNode(sumW)));
  b.properties.append(newCommand("i", exprNode(inTok)));
  b.properties.append(newCommand("e", exprNode(edgTok)));
  const element = diagram(a, b);
  element.items.push(new LinkItem("Link", "", a, b));
  const ctx = run(element);
  assert.equal(ctx.getEvaluated(a).get(sumTok), 15);
  assert.deepEqual(ctx.getEvaluated(a).get(tagTok), ["A", "B"]);
  assert.deepEqual(ctx.getEvaluated(a).get(outTok), ["B"]);
  assert.equal(ctx.getEvaluated(b).get(sumW), 2);
  assert.equal(ctx.getEvaluated(b).get(inTok), 1);
  assert.equal(ctx.getEvaluated(b).get(edgTok), 1);
});

test("notifyChanges adds dependants that changed to the event", () => {
  const tok = exprToken("items.B.val + 1");
  const a = item("A", newCommand("r", exprNode(tok)));
  const b = item("B", newCommand("val", newIntNode(1)));
  const element = diagram(a, b);
  const ctx = run(element);
  assert.equal(ctx.getEvaluated(a).get(tok), 2);

  b.properties.node.children![0]!.children![1] = newIntNode(5);
  const event = new ModelEvent("Structure", element);
  event.items.set(b, "Update");
  ctx.notifyChanges(event);
  assert.equal(ctx.getEvaluated(a).get(tok), 6);
  assert.equal(event.items.get(a), "Update");
});

test("Remove and Append events update the items proxy", () => {
  const tok = exprToken("items.B ? items.B.val : -1");
  const a = item("A", newCommand("r", exprNode(tok)));
  const b = item("B", newCommand("val", newIntNode(1)));
  const element = diagram(a, b);
  const ctx = run(element);
  assert.equal(ctx.getEvaluated(a).get(tok), 1);

  element.remove(b);
  const removed = new ModelEvent("Structure", element);
  removed.items.set(b, "Remove");
  ctx.notifyChanges(removed);
  assert.equal(ctx.getEvaluated(a).get(tok), -1);

  element.add(b);
  const added = new ModelEvent("Structure", element);
  added.items.set(b, "Append");
  ctx.notifyChanges(added);
  assert.equal(ctx.getEvaluated(a).get(tok), 1);
});

test("store drives the context", () => {
  const tok = exprToken("pos.x * 2");
  const a = item("A", newCommand("d", exprNode(tok)));
  const element = diagram(a);
  const model = new ElementModel();
  model.add(element);
  const store = new ElementModelStore(model);
  const ctx = run(element);
  store.executionContext = ctx;
  store.updatePosition(a, { x: 4, y: 0 }, null, () => {});
  assert.equal(ctx.getEvaluated(a).get(tok), 8);
});

test("itemSize hook defines width, height and defaults", () => {
  const w = exprToken("width + defaults.height");
  const a = item("A", newCommand("r", exprNode(w)));
  const ctx = run(diagram(a), { itemSize: () => ({ width: 10.5, height: 4.4 }) });
  assert.equal(ctx.getEvaluated(a).get(w), 15);
  const undef = exprToken("width");
  const b = item("B", newCommand("r", exprNode(undef)));
  assert.ok(run(diagram(b)).getEvaluated(b).get(undef) instanceof ReferenceError);
});

test("evaluate: false runs nothing and reports nothing", () => {
  const tok = exprToken("globalThis.__ran = true");
  const str = newStrNode("${name}");
  const a = item("A", newCommand("x", exprNode(tok)), newCommand("t", str));
  const element = diagram(a);
  const ctx = run(element, { evaluate: false });
  assert.equal(ctx.getEvaluated(a).size, 0);
  assert.equal(ctx.getEvaluated(element).size, 0);
  assert.equal((globalThis as Record<string, unknown>)["__ran"], undefined);
  let called = 0;
  ctx.updateAll(() => called++);
  assert.equal(called, 0);
  ctx.notifyChanges(new ModelEvent("Layout", element));
});

test("commands with non-identifier names do not break the context", () => {
  const tok = exprToken("font_size + 1");
  const a = item("A", newCommand("font-size", newIntNode(10)), newCommand("r", exprNode(tok)));
  assert.equal(run(diagram(a)).getEvaluated(a).get(tok), 11);
});

test("implicit globals stay in the context", () => {
  const set = blockToken("leaked_y = 3");
  const read = exprToken("leaked_y");
  const a = item("A", newCommand("s", blockNode(set)), newCommand("r", exprNode(read)));
  const ctx = run(diagram(a));
  assert.equal(ctx.getEvaluated(a).get(read), 3);
  assert.equal("leaked_y" in globalThis, false);
  const b = item("B", newCommand("r", exprNode(read)));
  assert.ok(run(diagram(b)).getEvaluated(b).get(read) instanceof ReferenceError);
});

test("document names win over host globals", () => {
  const toks = ["name", "status", "top"].map(exprToken);
  const props = [newCommand("status", newIntNode(7)), newCommand("top", newStrNode("t"))];
  toks.forEach((t, i) => props.push(newCommand("r" + i, exprNode(t))));
  const a = item("A", ...props);
  const ev = run(diagram(a)).getEvaluated(a);
  assert.deepEqual(toks.map((t) => ev.get(t)), ["A", 7, "t"]);
});

test("var persists across expressions", () => {
  const t1 = blockToken("var shared = 4; 0");
  const t2 = exprToken("shared + 1");
  const a = item("A", newCommand("x", blockNode(t1)), newCommand("y", exprNode(t2)));
  assert.equal(run(diagram(a)).getEvaluated(a).get(t2), 5);
});
