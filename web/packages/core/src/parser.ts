// Port of TennParser.swift.
import { TennLexer } from "./lexer.ts";
import { newIdent, newNode, TennNode } from "./model.ts";
import type { LexerError, TennLexerProtocol, TennToken, TennTokenType } from "./model.ts";

export type TennErrorCode =
  | "ok"
  | "parseError"
  | "unexpectedToken"
  | "unexpectedInput"
  | "invalidCommandStart"
  | "wrongBlockTerminator"
  | "EndOfFileDuringStringRead";

export interface TennError {
  readonly errorCode: TennErrorCode;
  readonly message: string;
  readonly line: number;
  readonly col: number;
}

export class TennErrorContainer {
  errors: TennError[] = [];

  report(code: TennErrorCode, msg: string, token: TennToken | null): void {
    this.errors.push({ errorCode: code, message: msg, line: token?.line ?? 0, col: token?.col ?? 0 });
  }

  hasErrors(): boolean {
    return this.errors.length > 0;
  }
}

export class TennParser {
  private lexer: TennLexerProtocol | null = null;
  private tok: TennToken | null = null;

  errors = new TennErrorContainer();

  factory: (source: string) => TennLexerProtocol = (source) => new TennLexer(source);

  constructor(lexerFactory?: (source: string) => TennLexerProtocol) {
    if (lexerFactory) {
      this.factory = lexerFactory;
    }
  }

  private reset(source: string): void {
    this.lexer = this.factory(source);
    this.lexer.errorHandler = (code: LexerError) => {
      switch (code) {
        case "EndOfExpressionReadError":
          this.errors.report("wrongBlockTerminator", "Unclosed expression terminal", null);
          break;
        case "EndOfLineReadString":
          this.errors.report("EndOfFileDuringStringRead", "Unclosed string terminal", null);
          break;
        case "UTF8Error":
          this.errors.report("unexpectedInput", "UTF8 conversion error", null);
          break;
      }
    };
  }

  private nextTok(): void {
    this.tok = this.lexer?.getToken() ?? null;
  }

  private eat(tokenType: TennTokenType): void {
    if (this.tok?.type === tokenType) {
      this.nextTok();
    } else {
      this.errors.report("unexpectedToken", `Unexpected token: ${this.tok?.type ?? "invalid"}`, this.tok);
    }
  }

  parse(source: string): TennNode {
    this.reset(source);

    const result = newNode("Statements");
    this.nextTok();

    if (this.tok === null) {
      this.errors.report("unexpectedInput", "Unexpected input...", null);
      return result;
    }

    const nextCmdMark = new Set<TennTokenType>(["semiColon", "eof"]);
    while (this.tok !== null && this.tok.type !== "eof") {
      const node = this.parseCommand(nextCmdMark);
      if (node !== null) {
        result.add(node);
      }
      if (this.errors.hasErrors()) {
        return result;
      }
      this.nextTok();
    }

    return result;
  }

  revert(token: TennToken): void {
    this.lexer?.revert(this.tok!);
    this.tok = token;
  }

  private parseCommand(endTokens: Set<TennTokenType>): TennNode | null {
    // Skip all semicolons.
    while (this.tok !== null && this.tok.type === "semiColon") {
      this.nextTok();
    }

    if (this.tok === null || endTokens.has(this.tok.type)) {
      return null;
    }

    const cmdNode = newNode("Command", this.tok);

    if (this.tok.type !== "symbol") {
      this.errors.report("invalidCommandStart", `Invalid command start symbol: ${this.tok.literal} `, this.tok);
      return null;
    }

    const checkEnd = (): boolean => {
      if (this.tok !== null && endTokens.has(this.tok.type)) {
        if (this.tok.type === "semiColon" && this.tok.literal === "\n") {
          const curToken: TennToken[] = [];
          while (this.tok !== null && this.tok.type === "semiColon" && this.tok.literal === "\n") {
            curToken.push(this.tok);
            this.nextTok();
          }
          const after = this.tok as TennToken | null; // nextTok() moved it; TS keeps the old narrowing
          if (after !== null && (after.type === "stringLit" || after.type === "curlyLe")) {
            return false;
          }
          while (curToken.length > 0) {
            this.revert(curToken.pop()!);
          }
          return true;
        }
        return true;
      }
      return false;
    };

    while (this.tok !== null && !checkEnd()) {
      // checkEnd may have advanced tok; Swift re-reads self.tok! here and so do we.
      const tok: TennToken = this.tok!;
      switch (tok.type) {
        case "symbol":
          cmdNode.add(newIdent(tok));
          break;
        case "stringLit":
          cmdNode.add(newNode("StringLit", tok));
          break;
        case "markdownLit":
          cmdNode.add(newNode("MarkdownLit", tok));
          break;
        case "intLit":
          cmdNode.add(newNode("IntLit", tok));
          break;
        case "floatLit":
          cmdNode.add(newNode("FloatLit", tok));
          break;
        case "expression":
          cmdNode.add(newNode("Expression", tok));
          break;
        case "expressionBlock":
          cmdNode.add(newNode("ExpressionBlock", tok));
          break;
        case "imageData":
          cmdNode.add(newNode("Image", tok));
          break;
        case "curlyLe": {
          const stmtNode = this.parseBlock("curlyLe", "curlyRi", tok);
          cmdNode.add(stmtNode);
          break;
        }
        default:
          return cmdNode;
      }
      if (this.errors.hasErrors()) {
        return cmdNode;
      }
      this.nextTok();
    }

    return cmdNode;
  }

  private parseBlock(stToken: TennTokenType, edToken: TennTokenType, currentTok: TennToken): TennNode {
    this.eat(stToken);

    const result = newNode("BlockExpr", currentTok);

    if (this.tok === null) {
      this.errors.report("parseError", "No more tokens parsing block", null);
      return result;
    }

    const nextCmdMark = new Set<TennTokenType>(["semiColon", "eof", edToken]);
    const endCheck = new Set<TennTokenType>([edToken, "eof"]);

    let curTok = currentTok;

    while (this.tok !== null && !endCheck.has(this.tok.type)) {
      curTok = this.tok;

      const node = this.parseCommand(nextCmdMark);
      if (node !== null) {
        result.add(node);
      }
      if (endCheck.has(this.tok!.type)) {
        break;
      }
      if (this.errors.hasErrors()) {
        return result;
      }
      this.nextTok();
    }

    if (this.tok === null || this.tok.type !== edToken) {
      this.errors.report("wrongBlockTerminator", "Wrong statements terminator", curTok);
      return result;
    }

    return result;
  }
}
