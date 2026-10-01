// Converts TS tokens (pos/size in UTF-16) to the Character units the Swift lexer reports; col is already in Characters.
import { graphemeCount } from "../src/characters.ts";
import type { MarkdownToken } from "../src/index.ts";

export function toSwiftToken(t: MarkdownToken, source: string): MarkdownToken {
  return { ...t, pos: graphemeCount(source.slice(0, t.pos)), size: graphemeCount(t.literal) };
}
