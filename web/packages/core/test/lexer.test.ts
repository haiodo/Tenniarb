import { test } from "node:test";
import assert from "node:assert/strict";
import { TennLexer } from "../src/index.ts";
import { graphemeCount, toSwiftToken } from "./swift-units.ts";
import type { LexerError, TennToken } from "../src/index.ts";

function tokenize(source: string): TennToken[] {
  const lexer = new TennLexer(source);
  const result: TennToken[] = [];
  while (result.length < 512) {
    const t = lexer.getToken();
    if (t === null) break;
    result.push(t);
    if (t.type === "eof") break;
  }
  return result;
}

const types = (s: string) => tokenize(s).map((t) => t.type);
const noEof = (s: string) => tokenize(s).filter((t) => t.type !== "eof");
const literals = (s: string) => noEof(s).map((t) => t.literal);

test("symbols and eof", () => {
  const lexer = new TennLexer("qwe asd");
  const tokens = [lexer.getToken(), lexer.getToken(), lexer.getToken()];
  assert.deepEqual(tokens.map((t) => t?.literal), ["qwe", "asd", "\0"]);
  assert.deepEqual(tokens.map((t) => t?.type), ["symbol", "symbol", "eof"]);
  assert.equal(lexer.getToken(), null);
});

test("emoji symbol", () => {
  assert.deepEqual(literals("qwe 😈"), ["qwe", "😈"]);
  assert.deepEqual(types("qwe 😈"), ["symbol", "symbol", "eof"]);
});

test("number literals", () => {
  assert.deepEqual(noEof("1 42 -7").map((t) => t.type), ["intLit", "intLit", "intLit"]);
  assert.deepEqual(noEof("1.5 -0.25").map((t) => t.type), ["floatLit", "floatLit"]);
  assert.deepEqual(noEof("12ab 1.2.3").map((t) => t.type), ["symbol", "symbol"]);
});

test("strings", () => {
  const a = noEof('name "hello world"');
  assert.deepEqual(a.map((t) => t.type), ["symbol", "stringLit"]);
  assert.equal(a[1]!.literal, "hello world");
  assert.equal(noEof("name 'hello'")[1]!.literal, "hello");
  assert.equal(noEof('s "a\\"b"')[1]!.literal, 'a"b');
});

test("unterminated string reports error", () => {
  const lexer = new TennLexer('name "unterminated');
  const errors: LexerError[] = [];
  lexer.errorHandler = (e) => errors.push(e);
  while (lexer.getToken() !== null);
  assert.ok(errors.includes("EndOfLineReadString"));
});

test("blocks and separators", () => {
  assert.deepEqual(types("a { b }"), ["symbol", "curlyLe", "symbol", "curlyRi", "eof"]);
  assert.deepEqual(types("a; b"), ["symbol", "semiColon", "symbol", "eof"]);
  assert.deepEqual(literals("a\nb"), ["a", "\n", "b"]);
  const nested = types("a { b { c } }");
  assert.equal(nested.filter((t) => t === "curlyLe").length, 2);
  assert.equal(nested.filter((t) => t === "curlyRi").length, 2);
});

test("comments", () => {
  assert.deepEqual(literals("a // ignored\nb").filter((l) => l !== "\n"), ["a", "b"]);
  assert.deepEqual(literals("a /* ignored */ b"), ["a", "b"]);
  assert.deepEqual(literals("a /* line one\nline two */ b").filter((l) => l !== "\n"), ["a", "b"]);
});

test("expressions, markdown, image", () => {
  const e = noEof("x $(1 + 2)");
  assert.deepEqual(e.map((t) => t.type), ["symbol", "expression"]);
  assert.equal(e[1]!.literal, "1 + 2");
  const b = noEof("x ${var a = 1; a}");
  assert.deepEqual(b.map((t) => t.type), ["symbol", "expressionBlock"]);
  assert.equal(b[1]!.literal, "var a = 1; a");
  const m = noEof("doc %{**bold**}");
  assert.deepEqual(m.map((t) => t.type), ["symbol", "markdownLit"]);
  assert.equal(m[1]!.literal, "**bold**");
  const i = noEof("img @(AAAA)");
  assert.deepEqual(i.map((t) => t.type), ["symbol", "imageData"]);
  assert.equal(i[1]!.literal, "AAAA");
  assert.ok(!noEof("$x").some((t) => t.type === "expression"));
});

test("line and size", () => {
  const syms = tokenize("a\nbb").filter((t) => t.type === "symbol");
  assert.equal(syms[0]!.line, 0);
  assert.equal(syms[1]!.line, 1);
  assert.equal(syms[1]!.size, 2);
});

test("revert", () => {
  const lexer = new TennLexer("a b");
  const first = lexer.getToken()!;
  lexer.revert(first);
  assert.equal(lexer.getToken()?.literal, "a");
  assert.equal(lexer.getToken()?.literal, "b");
});

test("empty and whitespace-only input", () => {
  assert.deepEqual(types(""), ["eof"]);
  assert.deepEqual(types("   \t  "), ["eof"]);
});

// pos/size/col are UTF-16 code units; the Swift values come from swift-units.ts.
test("pos, size and col units", () => {
  const [a, b] = noEof("a 😈");
  assert.deepEqual([a!.pos, a!.size, a!.col], [0, 1, 1]);
  assert.deepEqual([b!.pos, b!.size, b!.col], [2, 2, 4]); // surrogate pair
  assert.equal(tokenize("é").at(-1)!.pos, 1);
  const s = noEof('x "a\\"b"')[1]!; // content span is the source text between the quotes
  assert.deepEqual([s.literal, s.pos, s.size], ['a"b', 3, 4]);
});

test("swift unit conversion", () => {
  const src = "qwe 😈";
  const sw = tokenize(src).map((t) => toSwiftToken(t, src));
  assert.deepEqual([sw[1]!.col, sw[1]!.pos, sw[1]!.size], [8, 7, 1]);
  assert.equal(graphemeCount("e\u0301x"), 2);
});
