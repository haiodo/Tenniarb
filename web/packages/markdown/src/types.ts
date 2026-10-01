export type MarkdownTokenType =
  | "text"
  | "bold" // * bold \* text * - all until next *
  | "italic" // _ italic \_ text * - all until next _
  | "image" // @(image name|640x480), @(image name|640), @(image name|x480)
  | "color" // !(red), !(#ffeeff), !(red|word) !()- default color => global text color
  | "font" // &(20|word), &(20) Some text &() - Define a different font-size option
  | "expression" // ${expression}
  | "title" // ## Title value
  | "bullet" // * some value
  | "code" // `some code`
  | "underline" // <underscore>
  | "scratch" // ~text~
  | "eof";

// pos and size are UTF-16 units (Swift: Characters); col stays in Characters, see lexer.ts.
export interface MarkdownToken {
  type: MarkdownTokenType;
  literal: string;
  line: number;
  col: number;
  pos: number;
  size: number;
}

export type LexerError = "EndOfLineReadString" | "EndOfExpressionReadError" | "UTF8Error";
