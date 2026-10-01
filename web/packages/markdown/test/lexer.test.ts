import { test } from "node:test";
import assert from "node:assert/strict";
import { MarkdownLexer } from "../src/index.ts";
import type { LexerError, MarkdownToken, MarkdownTokenType } from "../src/index.ts";
import { toSwiftToken } from "./swift-units.ts";

function expectToken(tokens: MarkdownToken[], i: number, type: MarkdownTokenType, literal: string, pos: number) {
  assert.equal(tokens[i].pos, pos, `#${i} pos`);
  assert.equal(tokens[i].literal, literal, `#${i} literal`);
  assert.equal(tokens[i].type, type, `#${i} type`);
}

function drain(lexer: MarkdownLexer): MarkdownToken[] {
  const tokens: MarkdownToken[] = [];
  for (let t = lexer.getToken(); t !== null; t = lexer.getToken()) {
    tokens.push(t);
  }
  return tokens;
}

const boldCode = `* 1. *Re-connect* local NSM only(if pod are same)
* 1.1 Modify NSMD(1/2) stored connection info
* 1.2 do Request() on local Dataplane
* 1.3 return connection to NSC/NSE.
* - Dataplane/NSMD1 is potential fail points here.
* 2. Cleanup and configure new connection.
* 2.1 NSMD1 do *Close()* on local Dataplane.
* 2.2 NSMD1 do Close() on remote NSMD2
* 2.3 NDMS2 do Close() on local DataPlane
* 2.4 NSMD2 do Close() on local NSE
* 2.5 do "Connection" with all steps again.`;

const basicCode = `*box* text
# title A

Regular text *bold* line _italic_ line.

@(my_image|640)
`;

// cmkdownTests.swift

test("example", () => {
  const tokens = MarkdownLexer.getTokens("*Display* queries/paths\nin *Responses*\nRM-13104");
  assert.equal(tokens.length, 8);
  expectToken(tokens, 0, "bold", "Display", 0);
});

test("hWorld", () => {
  const tokens = MarkdownLexer.getTokens("Hello World!");
  assert.equal(tokens.length, 2);
  assert.equal(tokens[0].type, "text");
  assert.equal(tokens[0].literal, "Hello World!");
  assert.equal(tokens[1].type, "eof");
});

test("testBoldParsing2", () => {
  const tokens = MarkdownLexer.getTokens(boldCode);
  assert.equal(tokens.length, 37);
  expectToken(tokens, 22, "bold", "Close()", 279);
});

test("testBasicParsing", () => {
  const tokens = drain(new MarkdownLexer(basicCode));
  assert.equal(tokens.length, 15);
  assert.equal(tokens[10].pos, 61);
  assert.equal(tokens[12].literal, "my_image|640");
  assert.equal(tokens[12].type, "image");
});

test("testItalicParsing", () => {
  const tokens = MarkdownLexer.getTokens("_italic_ text _more italic_");
  assert.equal(tokens.length, 4);
  expectToken(tokens, 0, "italic", "italic", 0);
  expectToken(tokens, 2, "italic", "more italic", 14);
});

test("testCodeParsing", () => {
  const tokens = MarkdownLexer.getTokens("`code` and more `more code`");
  assert.equal(tokens.length, 4);
  expectToken(tokens, 0, "code", "code", 1);
  expectToken(tokens, 2, "code", "more code", 17);
});

test("testTitleParsing", () => {
  const tokens = MarkdownLexer.getTokens("# Title\n## Second Title\n### Third Title");
  assert.equal(tokens.length, 4);
  expectToken(tokens, 0, "title", "# Title", 0);
  expectToken(tokens, 1, "title", "## Second Title", 8);
  expectToken(tokens, 2, "title", "### Third Title", 24);
});

test("testUnderlineParsing", () => {
  const tokens = MarkdownLexer.getTokens("<underline> text <more underline>");
  assert.equal(tokens.length, 4);
  expectToken(tokens, 0, "underline", "underline", 0);
  expectToken(tokens, 2, "underline", "more underline", 17);
});

test("testScratchParsing", () => {
  const tokens = MarkdownLexer.getTokens("~scratch~ text ~more scratch~");
  assert.equal(tokens.length, 4);
  expectToken(tokens, 0, "scratch", "scratch", 0);
  expectToken(tokens, 2, "scratch", "more scratch", 15);
});

test("testColorParsing", () => {
  const tokens = MarkdownLexer.getTokens("!(red) text !(#ff0000) more");
  assert.equal(tokens.length, 5);
  expectToken(tokens, 0, "color", "red", 2);
  expectToken(tokens, 2, "color", "#ff0000", 14);
});

test("testFontParsing", () => {
  const tokens = MarkdownLexer.getTokens("&(14) text &(20) more");
  assert.equal(tokens.length, 5);
  expectToken(tokens, 0, "font", "14", 2);
  expectToken(tokens, 2, "font", "20", 13);
});

test("testExpressionParsing", () => {
  const tokens = MarkdownLexer.getTokens("${expression} text ${another}");
  assert.equal(tokens.length, 4);
  expectToken(tokens, 0, "expression", "expression", 2);
  expectToken(tokens, 2, "expression", "another", 21);
});

test("testComplexMarkdown", () => {
  const tokens = MarkdownLexer.getTokens(`# Main Title

This is *bold* and _italic_ text with \`code\` and <underline> and ~scratch~.

* Bullet item
  * Nested bullet

!(red) Colored text !(#ff0000) more colored.

&(14) Font size 14 &(20) font size 20.

\${expression} and \${another expression}

@(image.png|640x480) Image with size.`);
  assert.equal(tokens.length, 43);
  expectToken(tokens, 0, "title", "# Main Title", 0);
  expectToken(tokens, 3, "bold", "bold", 22);
  expectToken(tokens, 5, "italic", "italic", 33);
  expectToken(tokens, 7, "code", "code", 53);
  expectToken(tokens, 9, "underline", "underline", 63);
  expectToken(tokens, 11, "scratch", "scratch", 79);
  expectToken(tokens, 14, "text", "\n", 90);
  expectToken(tokens, 15, "bullet", "*", 91);
  expectToken(tokens, 19, "bullet", "*", 107);
  expectToken(tokens, 23, "color", "red", 126);
  expectToken(tokens, 25, "color", "#ff0000", 146);
  expectToken(tokens, 29, "font", "14", 172);
  expectToken(tokens, 31, "font", "20", 191);
  expectToken(tokens, 35, "expression", "expression", 212);
  expectToken(tokens, 37, "expression", "another expression", 230);
  expectToken(tokens, 40, "image", "image.png|640x480", 253);
});

// TenniarbTests/MarkDownTests.swift

test("MarkdownTests.testBoldParsing", () => {
  const tokens = MarkdownLexer.getTokens("*Display* queries/paths\nin *Responses*\nRM-13104");
  assert.equal(tokens.length, 8);
  expectToken(tokens, 0, "bold", "Display", 0);
});

test("MarkdownTests.testBoldParsing2", () => {
  const tokens = MarkdownLexer.getTokens(boldCode);
  assert.equal(tokens.length, 37);
  expectToken(tokens, 22, "bold", "Close()", 279);
});

test("MarkdownTests.testBasicParsing", () => {
  const tokens = drain(new MarkdownLexer(basicCode));
  assert.equal(tokens.length, 15);
  expectToken(tokens, 12, "image", "my_image|640", 65);
});

// Not covered by the Swift tests; expectations come from the Swift lexer (see fuzz/).

test("line and col", () => {
  const tokens = MarkdownLexer.getTokens("a\nbb *x*");
  assert.deepEqual(
    tokens.map((t) => [t.type, t.literal, t.line, t.col]),
    [["text", "a", 0, 0], ["text", "\n", 0, 0], ["text", "bb ", 1, 0], ["bold", "x", 1, 5], ["eof", "\0", 1, 5]],
  );
});

test("unterminated constructs", () => {
  const errors: [LexerError, number, number][] = [];
  const lex = (s: string) => {
    const lexer = new MarkdownLexer(s);
    lexer.errorHandler = (e, a, b) => errors.push([e, a, b]);
    return drain(lexer).map((t) => [t.type, t.literal]);
  };
  // The content of an unterminated expression is dropped, an unterminated code span is kept.
  assert.deepEqual(lex("a ${b"), [["text", "a "], ["eof", "\0"]]);
  assert.deepEqual(lex("`ab"), [["code", "ab"], ["eof", "\0"]]);
  assert.deepEqual(errors, [["EndOfExpressionReadError", 4, 5], ["EndOfLineReadString", 1, 3]]);
});

test("escapes, empty tokens, revert", () => {
  assert.deepEqual(MarkdownLexer.getTokens("a\\*b \\q").map((t) => t.literal), ["a*b \\q", "\0"]);
  assert.deepEqual(MarkdownLexer.getTokens("!() ``").map((t) => [t.type, t.literal]), [["color", ""], ["text", " "], ["code", ""], ["eof", "\0"]]);
  const lexer = new MarkdownLexer("a");
  const first = lexer.getToken()!;
  lexer.revert(first);
  assert.equal(lexer.getToken(), first);
});

test("CRLF is one Character, not a newline", () => {
  assert.deepEqual(MarkdownLexer.getTokens("a\r\n*b*").map((t) => [t.type, t.literal]), [["text", "a\r\n*b*"], ["eof", "\0"]]);
});

test("UTF-16 positions convert to Character units", () => {
  const source = "\u{1F600} é *b*";
  const bold = MarkdownLexer.getTokens(source).find((t) => t.type === "bold")!;
  assert.deepEqual([bold.pos, bold.size], [6, 1]);
  const swift = toSwiftToken(bold, source);
  assert.deepEqual([swift.pos, swift.size], [4, 1]);
});
