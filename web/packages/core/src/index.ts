export * from "./model.ts";
export { TennLexer, detectSymbolType } from "./lexer.ts";
export { TennParser, TennErrorContainer } from "./parser.ts";
export type { TennError, TennErrorCode } from "./parser.ts";
export { toStr, childsToStr } from "./printer.ts";

import { TennParser } from "./parser.ts";
import type { TennNode } from "./model.ts";
import { parseTenn } from "./persistence.ts";
import type { ElementModel } from "./element-model.ts";

export function parse(source: string): { tree: TennNode; parser: TennParser } {
  const parser = new TennParser();
  return { tree: parser.parse(source), parser };
}
export * from "./element-model.ts";
export * from "./persistence.ts";

// Mirrors Document.read: String(contentsOf:) drops a leading BOM; null when the parser reports errors.
export function readTenn(text: string): ElementModel | null {
  const parser = new TennParser();
  const tree = parser.parse(text.charCodeAt(0) === 0xfeff ? text.slice(1) : text);
  return parser.errors.hasErrors() ? null : parseTenn(tree);
}
export * from "./element-properties.ts";
export * from "./element-operations.ts";
export * from "./undo-manager.ts";
export * from "./layout-base.ts";
export * from "./grid-layout.ts";
export * from "./tree-layout.ts";
export * from "./spring-layout.ts";
export * from "./execution.ts";
export * from "./sync.ts";
