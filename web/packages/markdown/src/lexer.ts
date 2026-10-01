import { graphemeCount, graphemes } from "./characters.ts";
import type { LexerError, MarkdownToken, MarkdownTokenType } from "./types.ts";

// Works on Characters like the Swift lexer. Token pos/size and errorHandler positions are
// UTF-16 (offsets maps Character index -> UTF-16 offset); col is the Swift per-line Character
// counter and stays in Characters, it cannot be expressed as a source offset (see readUntil).
export class MarkdownLexer {
  private bufferCount: number;
  private pos = 0;
  private currentLine = 0;
  private currentChar = 0;

  private lineState = true;

  private tokenBuffer: MarkdownToken[] = [];
  private chars: string[];
  private offsets: number[];
  private currentCharValue: string;
  private prevCharacter = "\0";
  private r = "";

  errorHandler: ((error: LexerError, startPos: number, pos: number) => void) | null = null;

  constructor(code: string) {
    this.chars = graphemes(code);
    this.bufferCount = this.chars.length;
    this.offsets = [0];
    for (const c of this.chars) {
      this.offsets.push(this.offsets[this.offsets.length - 1] + c.length);
    }
    this.currentCharValue = this.chars[0] ?? "\0";
  }

  revert(tok: MarkdownToken): void {
    this.tokenBuffer.unshift(tok);
  }

  private add(type: MarkdownTokenType, literal: string, startPos: number): void {
    this.tokenBuffer.push({
      type,
      literal,
      line: this.currentLine,
      col: this.currentChar - graphemeCount(literal),
      pos: this.offsets[startPos],
      size: literal.length,
    });
  }

  private addCheck(startPos: number): void {
    if (this.r !== "") {
      this.add("text", this.r, startPos);
      this.r = "";
    }
  }

  private inc(): void {
    this.currentChar += 1;
    this.pos += 1;
    this.prevCharacter = this.currentCharValue;
    this.currentCharValue = this.chars[this.pos] ?? "\0";
  }

  private next(): string {
    return this.chars[this.pos + 1] ?? "\0";
  }

  getToken(): MarkdownToken | null {
    if (this.tokenBuffer.length > 0) {
      return this.tokenBuffer.shift()!;
    }

    this.r = "";
    let startPos = this.pos;

    let wasWhiteSpace = this.pos === 0;
    if (this.pos > 0 && this.pos < this.bufferCount) {
      switch (this.prevCharacter) {
        case " ":
        case "\t":
        case "\r":
        case "\n":
          wasWhiteSpace = true;
          break;
      }
    }

    while (this.pos < this.bufferCount) {
      const cc = this.currentCharValue;
      switch (cc) {
        case " ":
        case "\t":
          // Preserve spaces and tabs in text
          if (this.r === "") {
            startPos = this.pos;
          }
          this.r += cc;
          this.inc();
          wasWhiteSpace = true;
          break;
        case "\r":
        case "\n":
          // Add current text buffer before newline
          if (this.r !== "") {
            this.addCheck(startPos);
            startPos = this.pos;
          }
          // Add newline as separate token
          this.add("text", cc, this.pos);
          this.inc();
          if (cc === "\n") {
            this.currentLine += 1;
            this.currentChar = 0;
            this.lineState = true; // Mark as new line is started and we need to capture prefixes.
          }
          wasWhiteSpace = true;
          startPos = this.pos;
          break;
        case "\\": {
          // Skip next if required to skip
          const nc = this.next();
          if (this.r === "") {
            startPos = this.pos;
          }
          this.inc();
          switch (nc) {
            case "@":
            case "$":
            case "*":
            case "_":
            case "#":
            case "<":
            case "~":
            case "!":
              this.r += nc;
              this.inc();
              break;
            default:
              this.r += cc;
          }
          wasWhiteSpace = false;
          break;
        }
        case "@":
          if (this.next() === "(") {
            this.addCheck(startPos);
            startPos = this.pos;
            this.readUntil("(", ")", "image");
          } else {
            if (this.r === "") {
              startPos = this.pos;
            }
            this.r += cc;
            this.inc();
          }
          wasWhiteSpace = false;
          break;
        case "!":
          if (this.next() === "(") {
            this.addCheck(startPos);
            startPos = this.pos;
            this.readUntil("(", ")", "color", true);
          } else {
            if (this.r === "") {
              startPos = this.pos;
            }
            this.r += cc;
            this.inc();
          }
          wasWhiteSpace = false;
          break;
        case "&":
          if (this.next() === "(") {
            this.addCheck(startPos);
            startPos = this.pos;
            this.readUntil("(", ")", "font", true);
          } else {
            if (this.r === "") {
              startPos = this.pos;
            }
            this.r += cc;
            this.inc();
          }
          wasWhiteSpace = false;
          break;
        case "*":
          // Check if this is bullets list, if we have at least 1 space and all spaces before it will be bullet.
          if (this.lineState && this.next() === " ") {
            // Only whitespaces before, and at least one space
            this.addCheck(startPos);
            startPos = this.pos;
            this.r += cc;
            this.inc();
            this.add("bullet", this.r, startPos);
            this.r = "";
          } else if (wasWhiteSpace && this.next() !== " ") {
            this.addCheck(startPos); // Add previous line
            startPos = this.pos;
            this.inc();
            // This is potentially ** ** - strong or * * emphasize
            if (this.processUntilCharExceptNewLine("*")) {
              this.add("bold", this.r, startPos);
            } else {
              if (this.r === "") {
                this.r += "*"; // Just *
              }
              this.add("text", this.r, startPos);
            }
            this.r = "";
          } else {
            // Just *
            if (this.r === "") {
              startPos = this.pos;
            }
            this.r += cc;
            this.inc();
          }
          wasWhiteSpace = false;
          break;
        case "_":
        case "<":
        case "~": {
          if (wasWhiteSpace && this.next() !== " ") {
            this.addCheck(startPos);
            startPos = this.pos;
            this.inc();
            if (this.processUntilCharExceptNewLine(cc === "_" ? "_" : cc === "<" ? ">" : "~")) {
              this.add(cc === "_" ? "italic" : cc === "<" ? "underline" : "scratch", this.r, startPos);
            } else {
              this.add("text", this.r, startPos);
            }
            this.r = "";
          } else {
            if (this.r === "") {
              startPos = this.pos;
            }
            this.r += cc;
            this.inc();
          }
          wasWhiteSpace = false;
          break;
        }
        case "#":
          this.addCheck(startPos);
          startPos = this.pos;
          this.r += cc;
          this.inc();
          this.processUntilNewLine();
          wasWhiteSpace = true;
          this.add("title", this.r, startPos);
          this.r = "";
          break;
        case "$": {
          const nc = this.next();
          if (nc === "(") {
            this.addCheck(startPos);
            startPos = this.pos;
            this.readUntil("(", ")", "expression");
          } else if (nc === "{") {
            this.addCheck(startPos);
            startPos = this.pos;
            this.readUntil("{", "}", "expression");
          } else {
            if (this.r === "") {
              startPos = this.pos;
            }
            this.r += cc;
            this.inc();
          }
          wasWhiteSpace = false;
          break;
        }
        case "`":
          this.addCheck(startPos);
          startPos = this.pos;
          this.readUntilWithEscaping("`", "code");
          wasWhiteSpace = false;
          break;
        default:
          wasWhiteSpace = false;
          if (this.r === "") {
            startPos = this.pos;
          }
          this.r += cc;
          this.inc();
      }
      // Do not collect more whitespace characters.
      if (!wasWhiteSpace) {
        this.lineState = false;
      }
    }

    this.addCheck(startPos);

    if (this.pos === this.bufferCount) {
      this.add("eof", "\0", this.pos);
      this.inc();
    }

    return this.tokenBuffer.shift() ?? null;
  }

  private processUntilNewLine(): void {
    // End of line comment
    while (this.pos < this.bufferCount) {
      const cc = this.currentCharValue;
      if (cc === "\n") {
        this.currentLine += 1;
        this.lineState = true; // Mark as new line is started and we need to capture prefixes.
        this.inc();
        this.currentChar = 0;
        break;
      }
      this.r += cc;
      this.inc();
    }
  }

  private processUntilCharExceptNewLine(c: string): boolean {
    while (this.pos < this.bufferCount) {
      const cc = this.currentCharValue;
      if (cc === "\n") {
        this.currentLine += 1;
        this.inc();
        this.currentChar = 0;
        this.lineState = true; // Mark as new line is started and we need to capture prefixes.
        return false;
      }
      if (cc === c) {
        // We found out character, return
        this.inc();
        return true;
      }
      this.r += cc;
      this.inc();
    }
    return false;
  }

  private readUntilWithEscaping(lit: string, type: MarkdownTokenType): void {
    this.inc();

    let foundEnd = false;
    const stPos = this.pos;
    let content = "";
    while (this.pos < this.bufferCount) {
      const curChar = this.currentCharValue;
      if (curChar === "\n") {
        this.currentLine += 1;
        this.currentChar = 0;
        content += curChar;
      } else if (curChar === lit) {
        this.add(type, content, stPos);
        content = "";
        this.inc();
        foundEnd = true;
        break;
      } else if (curChar === "\\" && this.next() === lit) {
        content += this.next();
        this.inc();
      } else {
        content += curChar;
      }
      this.inc();
    }
    if (content !== "") {
      this.add(type, content, stPos);
      content = "";
    }
    if (!foundEnd) {
      this.errorHandler?.("EndOfLineReadString", this.offsets[stPos], this.offsets[this.pos]);
    }
  }

  private readUntil(startLit: string, endLit: string, type: MarkdownTokenType, addEmpty = false): void {
    this.inc();
    this.inc();

    const stPos = this.pos;
    let foundEnd = false;
    let indent = 1;
    const startLine = this.currentLine;
    let content = "";
    while (this.pos < this.bufferCount) {
      const curChar = this.currentCharValue;
      if (curChar === "\n") {
        this.currentLine += 1;
        this.currentChar = 0;
        content += curChar;
      } else if (curChar === startLit) {
        indent += 1;
        content += curChar;
      } else if (curChar === endLit) {
        indent -= 1;
        if (indent === 0) {
          foundEnd = true;
          break;
        }
        content += curChar;
      } else {
        content += curChar;
      }
      this.inc();
    }

    if (!foundEnd) {
      this.errorHandler?.("EndOfExpressionReadError", this.offsets[stPos], this.offsets[this.pos]);
    } else {
      if (content !== "" || addEmpty) {
        this.tokenBuffer.push({
          type,
          literal: content,
          line: startLine,
          col: this.currentChar - graphemeCount(content),
          pos: this.offsets[stPos],
          size: content.length,
        });
        content = "";
      }
      this.inc();
    }
  }

  static getTokens(code: string): MarkdownToken[] {
    const lexer = new MarkdownLexer(code);
    const tokens: MarkdownToken[] = [];

    for (let t = lexer.getToken(); t !== null; t = lexer.getToken()) {
      tokens.push(t);
    }
    return tokens;
  }
}
