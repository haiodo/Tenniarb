// Differential fuzz: Swift cmkdown (lexer, HTML printer, colors, image sizes; compiled standalone) vs the TS port.
// Usage: node fuzz.ts [N=2000] [seed=1]. Prints mismatches, exits 1 if any.
import { execFileSync, spawnSync } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";
import { ColorNames, ImageProvider, MarkdownLexer, colorToHex, parseColor, toHTML } from "../src/index.ts";
import type { CachedImage, LexerError } from "../src/index.ts";
import { graphemeCount } from "../src/characters.ts";
import { toSwiftToken } from "../test/swift-units.ts";

const N = Number(process.argv[2] ?? 2000);
let seed = Number(process.argv[3] ?? 1);
const rnd = () => (seed = (seed * 1664525 + 1013904223) >>> 0) / 2 ** 32;
const pick = <T>(a: T[]): T => a[Math.floor(rnd() * a.length)];

const root = fileURLToPath(new URL("../../../../", import.meta.url));
const here = fileURLToPath(new URL("./", import.meta.url));
const tmp = mkdtempSync(tmpdir() + "/md-fuzz-");
const bin = tmp + "/mdref";
const swiftSources = ["Types", "MarkdownLexer", "HTMLPrinter", "ColorUtils", "Images"].map(
  (f) => `${root}build/SourcePackages/checkouts/cmkdown/Sources/cmkdown/${f}.swift`,
);
execFileSync("swiftc", ["-O", "-suppress-warnings", "-swift-version", "5", ...swiftSources, here + "main.swift", "-o", bin], { stdio: "inherit" });

const nums = ["14", "16", "20.5", "1e5", "0x10", "inf", "nan", "+5", ".5", "5.", "1e400", "-0", "1e-7", "1_0", " 5", "1e16", "1e15", "123456789012345678", "0.0001", "0.00001", "abc", "0x1p3", "Infinity", "-3", "007"];
const colors = ["red", "#ff0000", "#abc", "grey-200", "gray-200", "amber", "amber-500", "deeppurple", "bluegrey", "blue-grey-500", "#EEEEEE", "#FFC107", "clear", "transparent", "#0xff", "# ff", "0xff", "#ffffffffffffffffffff", "#12345678", "#", "RED", "constructor", "__proto__", "zz"];
const images = ["a", "nope", "logo", "i|20", "i|x30", "i|100x", "i|40x40", "i| 5 x 6 ", "i|+7x+8", "i|ax", "i|9x9x9", "i|1x1", "i|x", "i|", "ii|2.5"];
const ascii = ["a", "b", "1", " ", " ", "\n", "\n", "\t", "\r", "\r\n", "*", "*", "_", "<", ">", "~", "#", "## ", "`", "@(", "!(", "&(", "$(", "${", ")", "}", "|", "\\", "\\*", "\\`", "&", "'", '"', "x", "* ", "\0"];
const struct = [
  ...colors.flatMap((c) => [`!(${c})`, `!(${c}|w)`]), "!()", "!( )", "!(|x)",
  ...nums.flatMap((n) => [`&(${n})`, `&(${n}|w)`]), "&()", "&( )",
  ...images.map((i) => `@(${i})`), "@()", "${e}", "$(a)", "`c`", "```a\nb```", "`a\nb`", "`a\nb\n`", "\n`x\ny`\n", "# T\n", "## T", "### x\n", "* item\n", " * item\n", "*b*", "_i_", "~s~", "<u>", "word ",
];
const uni = ["é", "日本", "́", "\u{1F600}", "‍", "\u{1F468}‍\u{1F469}", "\u{1F1FA}\u{1F1F8}", " ", " ", "\u0085", "﻿", "᠎", "　", "؀"];
const alphabets = [ascii, [...ascii, ...struct], [...ascii, ...struct, ...uni], [...uni, ...ascii, "|", "*", " ", "\n", "&", "<", "'", "!(", "#"]];

const inputs: string[] = [];
for (let i = 0; i < N; i++) {
  const alpha = alphabets[i % alphabets.length];
  const len = 1 + Math.floor(rnd() * 30);
  let s = "";
  for (let j = 0; j < len; j++) {
    s += pick(alpha);
  }
  inputs.push(s);
}
inputs.push(...colors, ...nums, "", "\0");

const sw = spawnSync(bin, { input: inputs.map((s) => Buffer.from(s, "utf8").toString("base64")).join("\n") + "\n", encoding: "utf8", maxBuffer: 2 ** 30, env: { ...process.env, SWIFT_DETERMINISTIC_HASHING: "1" } });
rmSync(tmp, { recursive: true });
const swLines = sw.stdout.split("\n").filter((l) => l.startsWith("{"));
if (sw.status !== 0 || swLines.length !== inputs.length) {
  console.log(`SWIFT FAILED: status ${sw.status} signal ${sw.signal}, ${swLines.length}/${inputs.length} results, last input ${JSON.stringify(inputs[swLines.length])}\n${sw.stderr.slice(-500)}`);
  process.exit(1);
}

// Swift picks any of the names sharing a hex (dictionary order), TS the first one.
const firstWithValue = new Map<string, string>();
for (const [k, v] of Object.entries(ColorNames)) {
  if (!firstWithValue.has(v)) firstWithValue.set(v, k);
}
const canonName = (n: string) => (Object.hasOwn(ColorNames, n) ? firstWithValue.get(ColorNames[n]) : n);
const canonHtml = (s: string) => s.replace(/(?<=color: )[A-Za-z0-9#-]+/g, (n) => canonName(n)!);

// Same fixed-size provider as main.swift.
class P extends ImageProvider {
  override resolveImage(name: string): CachedImage | null {
    if (name.startsWith("n")) return null;
    return { image: "", size: { width: 40 + 10 * (graphemeCount(name) % 7), height: 20 + 5 * (graphemeCount(name) % 5) } };
  }
}

function runTs(source: string) {
  const errors: object[] = [];
  const lexer = new MarkdownLexer(source);
  lexer.errorHandler = (e: LexerError, a, b) =>
    errors.push({ error: e, start: graphemeCount(source.slice(0, a)), pos: graphemeCount(source.slice(0, b)) });
  const tokens = [];
  for (let t = lexer.getToken(); t !== null; t = lexer.getToken()) tokens.push(t);
  const c = parseColor(source, 0.5);
  return {
    tokens: tokens.map((t) => toSwiftToken(t, source)),
    errors,
    html: canonHtml(toHTML(tokens, 16, "black", new P(2))),
    color: [c.r, c.g, c.b, c.a],
    hex: canonName(colorToHex(c)),
  };
}

const canon = (o: unknown): unknown =>
  Array.isArray(o) ? o.map(canon)
  : o && typeof o === "object" ? Object.fromEntries(Object.keys(o).sort().map((k) => [k, canon((o as Record<string, unknown>)[k])]))
  : o;

let bad = 0;
let skipped = 0;
for (let i = 0; i < inputs.length; i++) {
  const a = canon(JSON.parse(swLines[i])) as Record<string, unknown>;
  const b = canon(JSON.parse(JSON.stringify(runTs(inputs[i])))) as Record<string, unknown>;
  if (a.html === null) {
    skipped++;
    delete b.html;
    delete a.html;
  } else {
    a.html = canonHtml(a.html as string);
  }
  if (a.color === undefined) {
    delete b.color;
  }
  if (a.hex === undefined) {
    delete b.hex;
  } else {
    a.hex = canonName(a.hex as string);
  }
  const keys = ["tokens", "errors", "html", "color", "hex"].filter((k) => JSON.stringify(a[k]) !== JSON.stringify(b[k]));
  if (keys.length > 0) {
    bad++;
    if (bad <= 20) {
      console.log("DIFF", keys.join(","), JSON.stringify(inputs[i]));
      for (const k of keys) console.log(`  swift ${k}: ${JSON.stringify(a[k])}\n  ts    ${k}: ${JSON.stringify(b[k])}`);
    }
  }
}
console.log(`fuzz: ${inputs.length} inputs, ${bad} mismatches (${skipped} html skipped: unknown color or huge hex traps Swift)`);
process.exit(bad > 0 ? 1 : 0);
