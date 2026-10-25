// Worker: node bench.ts <coreSrcDir> <lex|parse> <file> <runs>; prints one JSON line.
import { readFileSync } from "node:fs";
import { pathToFileURL } from "node:url";

const [dir, mode, file, runsArg] = process.argv.slice(2);
const { TennLexer } = await import(pathToFileURL(`${dir}/lexer.ts`).href);
const { TennParser } = await import(pathToFileURL(`${dir}/parser.ts`).href);
const src = readFileSync(file, "utf8");
const runs = Number(runsArg);

let sink = 0;
function lexOnce(): number {
  const lx = new TennLexer(src);
  let n = 0;
  for (let t = lx.getToken(); t; t = lx.getToken()) {
    n++;
    if (t.type === "eof") break;
  }
  return n;
}
function parseOnce() {
  const p = new TennParser();
  return { tree: p.parse(src), errors: p.errors.errors.length };
}
function countNodes(n: { children: any[] | null }): number {
  let c = 1;
  if (n.children) for (const x of n.children) c += countNodes(x);
  return c;
}
function once() {
  if (mode === "lex") sink += lexOnce();
  else sink += parseOnce().tree.children?.length ?? 0;
}

const now = () => Number(process.hrtime.bigint()) / 1e6;
let t0 = now();
once();
const first = now() - t0;
for (let i = 0; i < 3; i++) once();
const times: number[] = [];
for (let i = 0; i < runs; i++) {
  t0 = now();
  once();
  times.push(now() - t0);
}
times.sort((a, b) => a - b);
let tokens = 0, nodes = 0, errors = 0;
if (mode === "lex") tokens = lexOnce();
else {
  const r = parseOnce();
  nodes = countNodes(r.tree);
  errors = r.errors;
}
console.log(JSON.stringify({ tokens, nodes, errors, first, median: times[times.length >> 1], min: times[0], runs, sink }));
