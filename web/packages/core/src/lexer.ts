// Port of TennLexer.swift.
// Unlike Swift (UTF-8 bytes for col/pos, Characters for size) all positions here are UTF-16 code
// units, so they index the JS source string directly:
//   pos  - start of the token's source span; for string/expression/markdown/image literals the span
//          is the content between the delimiters, for eof it is empty at the end of input
//   size - length of that span (differs from literal.length for strings with escapes)
//   col  - code units since the line counter was last reset, read after the token (as in Swift)
// test/swift-units.ts converts these back to Swift's values.
import { newToken } from "./model.ts";
import type { LexerError, TennLexerProtocol, TennToken, TennTokenType } from "./model.ts";

const NUL = 0;
const TAB = 0x09;
const LF = 0x0a;
const CR = 0x0d;
const SPACE = 0x20;
const DQUOTE = 0x22;
const DOLLAR = 0x24;
const PERCENT = 0x25;
const QUOTE = 0x27;
const LPAREN = 0x28;
const RPAREN = 0x29;
const STAR = 0x2a;
const SLASH = 0x2f;
const SEMI = 0x3b;
const AT = 0x40;
const BACKSLASH = 0x5c;
const LCURLY = 0x7b;
const RCURLY = 0x7d;

const segmenter = new Intl.Segmenter(undefined, { granularity: "grapheme" });
const asciiRe = /^[\x00-\x7f]*$/;
const decimalDigitRe = /^\p{Nd}/u;

export function detectSymbolType(value: string): TennTokenType {
  // Symbols never contain CR/LF, so plain ASCII is one grapheme per code unit.
  const chars = asciiRe.test(value) ? value.split("") : Array.from(segmenter.segment(value), (s) => s.segment);
  let skipFirst = chars[0] === "-";
  let dot = false;
  let i = 0;
  for (const c of chars) {
    if (skipFirst) {
      skipFirst = false;
      continue;
    }
    if (c === ".") {
      if (i === 0 || dot) {
        return "symbol";
      }
      dot = true;
      continue;
    }
    // CharacterSet.decimalDigits on the first unicode scalar of the character
    if (!decimalDigitRe.test(c)) {
      return "symbol";
    }
    i += 1;
  }
  if (dot) {
    return "floatLit";
  }
  return "intLit";
}

export class TennLexer implements TennLexerProtocol {
  private currentLine = 0;
  private currentChar = 0;
  private bufferCount: number;
  private pos = 0;

  private tokenBuffer: TennToken[] = [];
  private blockState: TennTokenType[] = [];
  private code: string;

  private currentCharValue: number;

  errorHandler: ((error: LexerError, startPos: number, pos: number) => void) | null = null;

  constructor(code: string) {
    this.code = code;
    this.bufferCount = code.length;
    this.currentCharValue = code.length > 0 ? code.charCodeAt(0) : NUL;
  }

  revert(tok: TennToken): void {
    this.tokenBuffer.unshift(tok);
  }

  // Span is code.slice(start, end); line/col are taken now, like in Swift.
  private add(type: TennTokenType, literal: string, start: number, end: number): void {
    this.tokenBuffer.push(newToken(type, literal, this.currentLine, this.currentChar, start, end - start));
  }

  // Swift decodes every slice with String(bytes:encoding:), which drops a leading U+FEFF.
  private slice(start: number, end: number): string {
    const s = this.code.slice(start, end);
    return s.charCodeAt(0) === 0xfeff ? s.slice(1) : s;
  }

  private addCheck(pattern: string): void {
    if (pattern.length > 0) {
      this.add(detectSymbolType(pattern), pattern, this.pos - pattern.length, this.pos);
    }
  }

  private inc(): void {
    this.currentChar += 1;
    this.pos += 1;
    this.currentCharValue = this.pos < this.bufferCount ? this.code.charCodeAt(this.pos) : NUL;
  }

  private next(): number {
    return this.pos + 1 < this.bufferCount ? this.code.charCodeAt(this.pos + 1) : NUL;
  }

  private readString(lit: number): void {
    this.inc();

    let foundEnd = false;
    let stPos = this.pos;
    const contentStart = this.pos;
    let r = "";
    while (this.pos < this.bufferCount) {
      if (this.currentCharValue === LF) {
        this.currentLine += 1;
        this.currentChar = 0;
      } else if (this.currentCharValue === lit) {
        r += this.slice(stPos, this.pos);
        this.add("stringLit", r, contentStart, this.pos);
        r = "";
        this.inc();
        stPos = this.pos;
        foundEnd = true;
        break;
      } else if (this.currentCharValue === BACKSLASH && this.next() === lit) {
        r += this.slice(stPos, this.pos);
        r += String.fromCharCode(lit);
        this.inc();
        this.inc();
        stPos = this.pos;
        continue;
      }
      this.inc();
    }
    r += this.slice(stPos, this.pos);
    if (r.length > 0) {
      this.add("stringLit", r, contentStart, this.pos);
    }
    if (!foundEnd) {
      this.errorHandler?.("EndOfLineReadString", stPos, this.pos);
    }
  }

  private skipCComment(): void {
    this.inc(); // Skip /
    this.inc(); // Skip *

    while (this.pos < this.bufferCount) {
      if (this.currentCharValue === LF) {
        this.currentLine += 1;
        this.currentChar = 0;
      } else if (this.currentCharValue === STAR && this.next() === SLASH) {
        this.inc();
        this.inc();
        return;
      }
      this.inc();
    }
  }

  private processComment(): void {
    if (this.next() === STAR) {
      this.skipCComment();
    } else if (this.next() === SLASH) {
      // End of line comment
      while (this.pos < this.bufferCount) {
        if (this.currentCharValue === LF) {
          this.currentLine += 1;
          this.currentChar = 0;
          break;
        }
        this.inc();
      }
    } else {
      this.inc();
    }
  }

  private processNewLine(cc: number): boolean {
    this.inc();
    if (cc === LF) {
      this.currentLine += 1;
      this.currentChar = 0;
      // Check if we need to send a delimiter semicolon symbol.
      if (this.blockState.length === 0 || this.blockState[0] === "curlyLe") {
        this.add("semiColon", "\n", this.pos - 1, this.pos);
      }
    }
    return this.tokenBuffer.length > 0;
  }

  private returnToken(): TennToken {
    return this.tokenBuffer.shift()!;
  }

  getToken(): TennToken | null {
    if (this.tokenBuffer.length > 0) {
      return this.returnToken();
    }

    let r = "";
    let stPos = this.pos;

    const append = (): void => {
      if (stPos < this.pos) {
        r += this.slice(stPos, this.pos);
        stPos = this.pos;
      }
    };

    const flush = (): void => {
      if (r.length > 0) {
        this.addCheck(r);
        r = "";
      }
    };

    while (this.pos < this.bufferCount) {
      const cc = this.currentCharValue;
      switch (cc) {
        case SPACE:
        case TAB:
        case CR:
        case LF:
          if (stPos < this.pos) {
            append();
            this.addCheck(r);
            r = "";
          }
          if (this.processNewLine(cc)) {
            return this.returnToken();
          }
          stPos = this.pos;
          break;
        case LCURLY:
          append();
          flush();
          this.processCurlyOpen(cc);
          stPos = this.pos;
          break;
        case RCURLY:
          append();
          flush();
          if (this.processCurlyClose(cc)) {
            return this.returnToken();
          }
          stPos = this.pos;
          break;
        case SEMI:
          append();
          flush();
          this.add("semiColon", ";", this.pos, this.pos + 1);
          this.inc();
          if (this.tokenBuffer.length > 0) {
            return this.returnToken();
          }
          stPos = this.pos;
          break;
        case SLASH:
          append();
          flush();
          this.processComment();
          stPos = this.pos;
          break;
        case PERCENT:
          if (this.next() === LCURLY) {
            append();
            flush();
            this.readExpression(LCURLY, RCURLY, "markdownLit");
            stPos = this.pos;
          } else {
            this.inc();
          }
          break;
        case AT:
          if (this.next() === LPAREN) {
            append();
            flush();
            this.readImage(RPAREN, "imageData");
            stPos = this.pos;
          } else {
            this.inc();
          }
          break;
        case DOLLAR: {
          const nc = this.next();
          if (nc === LPAREN) {
            append();
            flush();
            this.readExpression(LPAREN, RPAREN, "expression");
            stPos = this.pos;
          } else if (nc === LCURLY) {
            append();
            flush();
            this.readExpression(LCURLY, RCURLY, "expressionBlock");
            stPos = this.pos;
          } else {
            this.inc();
          }
          break;
        }
        case QUOTE:
        case DQUOTE:
          append();
          flush();
          this.readString(cc);
          if (this.tokenBuffer.length > 0) {
            return this.returnToken();
          }
          stPos = this.pos;
          break;
        default:
          this.inc();
      }
    }

    append();
    flush();

    if (this.pos === this.bufferCount) {
      this.add("eof", "\0", this.pos, this.pos);
      this.inc();
    }

    return this.tokenBuffer.length > 0 ? this.returnToken() : null;
  }

  private processCurlyOpen(cc: number): void {
    this.inc();
    this.add("curlyLe", String.fromCharCode(cc), this.pos - 1, this.pos);
    this.blockState.unshift("curlyLe");
  }

  private processCurlyClose(cc: number): boolean {
    this.inc();
    this.add("curlyRi", String.fromCharCode(cc), this.pos - 1, this.pos);

    if (this.blockState.length === 0) {
      return false;
    }
    this.blockState.shift();
    return this.tokenBuffer.length > 0;
  }

  private readExpression(startLit: number, endLit: number, type: TennTokenType): void {
    this.inc();
    this.inc();

    const stPos = this.pos;
    let foundEnd = false;
    let indent = 1;
    const startLine = this.currentLine;
    while (this.pos < this.bufferCount) {
      const curChar = this.currentCharValue;
      if (curChar === LF) {
        this.currentLine += 1;
        this.currentChar = 0;
      } else if (curChar === startLit) {
        indent += 1;
      } else if (curChar === endLit) {
        indent -= 1;
        if (indent === 0) {
          foundEnd = true;
          break;
        }
      }
      this.inc();
    }
    this.finishExpression(stPos, foundEnd, startLine, type);
  }

  private readImage(endLit: number, type: TennTokenType): void {
    this.inc();
    this.inc();

    const stPos = this.pos;
    let foundEnd = false;
    const startLine = this.currentLine;
    while (this.pos < this.bufferCount) {
      const curChar = this.currentCharValue;
      if (curChar === LF) {
        this.currentLine += 1;
        this.currentChar = 0;
      } else if (curChar === endLit) {
        foundEnd = true;
        break;
      }
      this.inc();
    }
    this.finishExpression(stPos, foundEnd, startLine, type);
  }

  // Tail shared by readExpression and readImage in Swift (identical code there).
  private finishExpression(stPos: number, foundEnd: boolean, startLine: number, type: TennTokenType): void {
    const r = this.slice(stPos, this.pos);
    if (!foundEnd) {
      this.errorHandler?.("EndOfExpressionReadError", stPos, this.pos);
    } else {
      if (r.length > 0) {
        this.tokenBuffer.push(newToken(type, r, startLine, this.currentChar, stPos, this.pos - stPos));
      }
      this.inc();
    }
  }
}
