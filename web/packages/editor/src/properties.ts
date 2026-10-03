// DOM-free part of the properties panel: token colors and parse diagnostics. Port of TextPropertiesDelegate.highlight.
import { TennLexer, TennParser } from "@tenniarb/core";

export type HighlightClass = "symbol" | "string" | "number" | "expression";
export interface HighlightRange {
  from: number;
  to: number;
  cls: HighlightClass;
}

// Swift TennColors, white / dark.
export const COLORS: Record<"light" | "dark", Record<HighlightClass, string>> = {
  light: { symbol: "#815f03", string: "#1c00cf", number: "#1c00cf", expression: "#646485" },
  dark: { symbol: "#75b492", string: "#fc6a5d", number: "#9686f5", expression: "#c67c48" },
};

/** Lexer pos/size cover the content between the delimiters; the ranges widen them as the Swift highlighter does. */
export function highlightRanges(text: string): HighlightRange[] {
  const out: HighlightRange[] = [];
  const add = (from: number, size: number, cls: HighlightClass): void => {
    const f = Math.max(from, 0);
    const to = Math.min(from + size, text.length);
    if (to > f) out.push({ from: f, to, cls });
  };
  const lexer = new TennLexer(text);
  for (let t = lexer.getToken(); t !== null && t.type !== "eof"; t = lexer.getToken()) {
    if (t.size === 0) continue;
    switch (t.type) {
      case "symbol":
        add(t.pos, t.size, "symbol");
        break;
      case "stringLit":
        add(t.pos - 1, t.size + 2, "string");
        break;
      case "markdownLit":
        add(t.pos - 2, t.size + 3, "string");
        break;
      case "expression":
      case "expressionBlock":
        add(t.pos - 2, t.size + 3, "expression"); // `$(` / `${` before, `)` / `}` after
        break;
      case "intLit":
      case "floatLit":
        add(t.pos, t.size, "number");
        break;
    }
  }
  return out;
}

export interface TextDiagnostic {
  from: number;
  to: number;
  message: string;
}

/** Parse errors carry only line/col of the token end, so the whole (trimmed) line is marked. */
export function parseDiagnostics(text: string): TextDiagnostic[] {
  const parser = new TennParser();
  parser.parse(text);
  const lines = text.split("\n");
  return parser.errors.errors.map((e) => {
    const line = Math.min(e.line, lines.length - 1);
    let from = lines.slice(0, line).reduce((n, l) => n + l.length + 1, 0);
    const raw = lines[line]!;
    from += raw.length - raw.trimStart().length;
    return { from, to: Math.min(Math.max(from + raw.trim().length, from + 1), text.length), message: e.message };
  });
}
