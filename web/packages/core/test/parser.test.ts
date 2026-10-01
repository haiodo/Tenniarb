import { test } from "node:test";
import assert from "node:assert/strict";
import { newBlockExpr, newCommand, newIdent, newIntNode, newStrNode, parse, toStr, TennLexer, TennParser } from "../src/index.ts";

test("parse and print", () => {
  const { tree, parser } = parse('element "Root" {\n  item First {\n    pos 10 20\n  }\n}');
  assert.equal(parser.errors.hasErrors(), false);
  assert.equal(toStr(tree), 'element "Root" {\n    item First {\n        pos 10 20\n    }\n}');
  assert.equal(toStr(tree, 0, true), 'element Root {\n    item First {\n        pos 10 20\n    }\n}');
});

test("node accessors", () => {
  const { tree } = parse("a { pos 10 20\n name \"x y\"\n flag true }");
  const block = tree.getChild([0, 1])!;
  assert.equal(block.kind, "BlockExpr");
  assert.equal(block.getNamedElement("pos")?.getInt(1), 10);
  assert.equal(block.getValue("name", ""), "x y");
  assert.equal(block.getValue("flag", false), true);
  assert.equal(block.getValue("missing", 7), 7);
  assert.equal(tree.getIdent(0, 0), "a");
  assert.equal(tree.getChild([0, 9]), null);
});

test("builders print", () => {
  const cmd = newCommand("el", newStrNode("A"), newBlockExpr(newCommand("pos", newIntNode(1), newIntNode(2))));
  assert.equal(toStr(cmd), 'el "A" {\n    pos 1 2\n}');
  assert.equal(toStr(newIdent("x")), "x");
});

test("same parse regardless of factory", () => {
  const src = "element Root {\n  item A {\n    pos 1 2\n  }\n}";
  const p = new TennParser((s) => new TennLexer(s));
  assert.equal(toStr(p.parse(src)), toStr(parse(src).tree));
});

test("errors", () => {
  const unclosed = parse("a { b").parser.errors.errors;
  assert.deepEqual(unclosed.map((e) => e.errorCode), ["wrongBlockTerminator"]);
  assert.equal(unclosed[0]!.message, "Wrong statements terminator");

  const str = parse('a "abc').parser.errors.errors;
  assert.equal(str[0]!.errorCode, "EndOfFileDuringStringRead");
  assert.equal(str[0]!.message, "Unclosed string terminal");

  const start = parse("1 a").parser.errors.errors;
  assert.deepEqual(start.map((e) => e.errorCode), ["invalidCommandStart"]);
  assert.equal(start[0]!.message, "Invalid command start symbol: 1 ");

  const expr = parse("a $(1").parser.errors.errors;
  assert.equal(expr[0]!.errorCode, "wrongBlockTerminator");
  assert.equal(expr[0]!.message, "Unclosed expression terminal");
});

test("empty source", () => {
  const { tree, parser } = parse("");
  assert.equal(parser.errors.hasErrors(), false);
  assert.equal(tree.count, 0);
});
