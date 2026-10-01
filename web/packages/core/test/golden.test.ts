import { test } from "node:test";
import assert from "node:assert/strict";
import { TennLexer, TennParser, toStr } from "../src/index.ts";
import { toSwiftToken } from "./swift-units.ts";
import { fixtureNames, readGolden, readSource } from "./fixtures.ts";
import type { TennNode, TennToken } from "../src/index.ts";

function nodeJson(n: TennNode, source: string): object {
  return {
    kind: n.kind,
    ...(n.token !== null && { token: toSwiftToken(n.token, source) }),
    ...(n.children !== null && { children: n.children.map((c) => nodeJson(c, source)) }),
  };
}

function build(source: string) {
  const lexer = new TennLexer(source);
  const tokens: TennToken[] = [];
  for (let t = lexer.getToken(); t !== null; t = lexer.getToken()) {
    tokens.push(t);
  }
  const parser = new TennParser();
  // Swift error col is the token col in UTF-8 bytes; errors keep only the UTF-16 col, so take it from the token.
  const cols: number[] = [];
  const report = parser.errors.report.bind(parser.errors);
  parser.errors.report = (code, msg, token) => {
    report(code, msg, token);
    cols.push(token === null ? 0 : toSwiftToken(token, source).col);
  };
  const tree = parser.parse(source);
  return {
    tokens: tokens.map((t) => toSwiftToken(t, source)),
    tree: nodeJson(tree, source),
    printed: toStr(tree),
    printedClean: toStr(tree, 0, true),
    errors: parser.errors.errors.map((e, i) => ({ code: e.errorCode, message: e.message, line: e.line, col: cols[i] })),
  };
}

for (const f of fixtureNames()) {
  test(`golden ${f}`, () => {
    const expected = JSON.parse(readGolden(f.replace(/\.tenn$/, ".parse.json")));
    assert.deepEqual(build(readSource(f)), expected);
  });
}
