// Converts TS tokens (UTF-16 span positions, see src/lexer.ts) to the values the Swift lexer reports:
//   size = Characters in literal; col = UTF-8 bytes since the line counter reset;
//   pos  = UTF-8 byte offset of the lexer position at add time minus size.
// The add-time position is the span end, except for ";" where Swift adds before consuming it.
import type { TennToken } from "../src/index.ts";

const segmenter = new Intl.Segmenter(undefined, { granularity: "grapheme" });
const utf8Len = (s: string) => Buffer.byteLength(s, "utf8");

export function graphemeCount(s: string): number {
  let n = 0;
  for (const _ of segmenter.segment(s)) {
    n++;
  }
  return n;
}

export function toSwiftToken(t: TennToken, source: string): TennToken {
  const end = t.type === "semiColon" && t.literal === ";" ? t.pos : t.pos + t.size;
  const size = graphemeCount(t.literal);
  return {
    type: t.type,
    literal: t.literal,
    line: t.line,
    col: utf8Len(source.slice(end - t.col, end)),
    pos: utf8Len(source.slice(0, end)) - size,
    size,
  };
}
