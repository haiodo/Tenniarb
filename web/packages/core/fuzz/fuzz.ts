// Differential fuzz: Swift tenn lexer/parser/printer (compiled standalone) vs the TS port.
// Usage: node fuzz.ts [N=2000] [seed=1]. Prints mismatches, exits 1 if any.
import { execFileSync, spawnSync } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";
import { TennLexer, TennParser, toStr } from "../src/index.ts";
import type { TennNode, TennToken } from "../src/index.ts";
import { toSwiftToken } from "../test/swift-units.ts";

const N = Number(process.argv[2] ?? 2000);
let seed = Number(process.argv[3] ?? 1);
const rnd = () => (seed = (seed * 1664525 + 1013904223) >>> 0) / 2 ** 32;

const root = fileURLToPath(new URL("../../../../", import.meta.url));
const here = fileURLToPath(new URL("./", import.meta.url));
const tmp = mkdtempSync(tmpdir() + "/tenn-fuzz-");
const bin = tmp + "/tennref";
const swiftSources = ["TennLexer", "TennLexerModel", "TennModel", "TennParser", "TennPrinter"].map(
  (f) => `${root}Tenniarb/document/tenn/${f}.swift`,
);
execFileSync("swiftc", ["-O", ...swiftSources, here + "main.swift", "-o", bin], { stdio: "inherit" });

const ascii = ["a", "b", "1", "2.5", " ", "\n", "\n", ";", "{", "}", '"x"', "'y'", "$(1)", "%{m}", "@(i)", "//c\n", "/*c*/", "\r\n", "-", "\\", '"', "'", "$", "%", "@(", "\0", "\t"];
const uni = ["é", "日本", "٣", "\u{1F600}", "﻿", "é", "​", "\u0085", "\u{1F468}‍\u{1F469}"];
const alphabets = [ascii, [...ascii, ...uni], [...uni, " ", "\n", "{", "}", '"', ";", "a", "$("]];

const inputs: string[] = [];
for (let i = 0; i < N; i++) {
  const alpha = alphabets[i % alphabets.length];
  const len = 1 + Math.floor(rnd() * 40);
  let s = "";
  for (let j = 0; j < len; j++) {
    s += alpha[Math.floor(rnd() * alpha.length)];
  }
  inputs.push(s);
}

const sw = spawnSync(bin, { input: inputs.map((s) => Buffer.from(s, "utf8").toString("base64")).join("\n") + "\n", encoding: "utf8", maxBuffer: 2 ** 30 });
rmSync(tmp, { recursive: true });
// The Swift lexer also prints diagnostics to stdout; results are the JSON objects.
const swLines = sw.stdout.split("\n").filter((l) => l.startsWith("{"));
if (sw.status !== 0 || swLines.length !== N) {
  console.log(`SWIFT FAILED: status ${sw.status}, ${swLines.length}/${N} results\n${sw.stderr.slice(-500)}`);
  process.exit(1);
}

function nodeJson(n: TennNode, source: string): object {
  return {
    kind: n.kind,
    ...(n.token !== null && { token: toSwiftToken(n.token, source) }),
    ...(n.children !== null && { children: n.children.map((c) => nodeJson(c, source)) }),
  };
}

function runTs(source: string) {
  const lexer = new TennLexer(source);
  const tokens: TennToken[] = [];
  for (let t = lexer.getToken(); t !== null; t = lexer.getToken()) {
    tokens.push(t);
    if (t.type === "eof" || tokens.length > 100000) break;
  }
  const parser = new TennParser();
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

const canon = (o: unknown): unknown =>
  Array.isArray(o) ? o.map(canon)
  : o && typeof o === "object" ? Object.fromEntries(Object.keys(o).sort().map((k) => [k, canon((o as Record<string, unknown>)[k])]))
  : o;

let bad = 0;
for (let i = 0; i < N; i++) {
  const a = canon(JSON.parse(swLines[i])) as Record<string, unknown>;
  let b: Record<string, unknown>;
  try {
    b = canon(JSON.parse(JSON.stringify(runTs(inputs[i])))) as Record<string, unknown>;
  } catch (e) {
    bad++;
    console.log("TS THROW", JSON.stringify(inputs[i]), String(e));
    continue;
  }
  const keys = Object.keys(a).filter((k) => JSON.stringify(a[k]) !== JSON.stringify(b[k]));
  if (keys.length > 0) {
    bad++;
    if (bad <= 20) console.log("DIFF", keys.join(","), JSON.stringify(inputs[i]));
  }
}
console.log(`fuzz: ${N} inputs, ${bad} mismatches`);
process.exit(bad > 0 ? 1 : 0);
