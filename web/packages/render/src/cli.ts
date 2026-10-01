#!/usr/bin/env node
// tenniarb-render <in.tenn> -o <out.pdf|png|svg> [--element <name>] [--scale N]
import { readFileSync, writeFileSync } from "node:fs";
import { parseArgs } from "node:util";
import { TennParseError, renderTenn } from "./render-tenn.ts";

const USAGE = "usage: tenniarb-render <in.tenn> -o <out.pdf|png|svg> [--element <name>] [--scale N]";
const usage = (msg: string): never => {
  console.error(`${msg}\n${USAGE}`);
  process.exit(1);
};

let args: ReturnType<typeof parseArgs<{ allowPositionals: true; options: { o: { type: "string"; short: "o" }; element: { type: "string" }; scale: { type: "string" } } }>>;
try {
  args = parseArgs({ allowPositionals: true, options: { o: { type: "string", short: "o" }, element: { type: "string" }, scale: { type: "string" } } });
} catch (e) {
  usage((e as Error).message);
}
const [input] = args!.positionals;
const out = args!.values.o;
if (input === undefined || args!.positionals.length > 1 || out === undefined) usage("input file and -o are required");
const format = /\.(pdf|png|svg)$/i.exec(out!)?.[1]?.toLowerCase() as "pdf" | "png" | "svg" | undefined;
if (format === undefined) usage("output extension must be .pdf, .png or .svg");
const scale = args!.values.scale === undefined ? undefined : Number(args!.values.scale);
if (scale !== undefined && !(scale > 0)) usage("--scale must be a positive number");

let text: string;
try {
  text = readFileSync(input!, "utf8");
} catch (e) {
  usage(`cannot read ${input}: ${(e as Error).message}`);
}
try {
  writeFileSync(out!, await renderTenn(text!, { format: format!, element: args!.values.element, scale }));
} catch (e) {
  if (e instanceof TennParseError) {
    for (const err of e.errors) console.error(`${input}:${err.line}:${err.col}: ${err.message}`);
    process.exit(2);
  }
  console.error(`render failed: ${(e as Error).message}`);
  process.exit(3);
}
