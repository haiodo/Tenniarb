// Port of TennLexerModel.swift and TennModel.swift. Enum values are the Swift case names.

export type TennTokenType =
  | "invalid"
  | "eof"
  | "symbol"
  | "floatLit"
  | "intLit"
  | "stringLit"
  | "curlyLe"
  | "curlyRi"
  | "comma"
  | "colon"
  | "semiColon"
  | "expression"
  | "expressionBlock"
  | "markdownLit"
  | "imageData";

export interface TennToken {
  readonly type: TennTokenType;
  readonly literal: string;
  readonly line: number;
  readonly col: number;
  readonly pos: number;
  readonly size: number;
}

export function newToken(type: TennTokenType, literal: string, line = 0, col = 0, pos = 0, size = 0): TennToken {
  return { type, literal, line, col, pos, size };
}

export type LexerError = "EndOfLineReadString" | "EndOfExpressionReadError" | "UTF8Error";

export interface TennLexerProtocol {
  getToken(): TennToken | null;
  revert(tok: TennToken): void;
  errorHandler: ((error: LexerError, startPos: number, pos: number) => void) | null;
}

export type TennNodeKind =
  | "Empty"
  | "Ident"
  | "CharLit"
  | "IntLit"
  | "FloatLit"
  | "StringLit"
  | "MarkdownLit"
  | "Command"
  | "Statements"
  | "BlockExpr"
  | "Expression"
  | "ExpressionBlock"
  | "Image";

const intRe = /^[+-]?\d+$/;
// CharacterSet.whitespacesAndNewlines, enumerated from Foundation.
const WS = "[\\t-\\r \\u0085\\u00a0\\u1680\\u2000-\\u200b\\u2028\\u2029\\u202f\\u205f\\u3000]+";
const trimRe = new RegExp(`^${WS}|${WS}$`, "g");
const floatRe = /^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$/;

export class TennNode {
  readonly kind: TennNodeKind;
  token: TennToken | null;
  children: TennNode[] | null = null;
  private named: Map<string, TennNode> | null = null;

  constructor(kind: TennNodeKind, tok: TennToken | null = null) {
    this.kind = kind;
    this.token = tok;
  }

  get count(): number {
    return this.children?.length ?? 0;
  }

  clone(): TennNode {
    const result = new TennNode(this.kind, this.token);
    if (this.children !== null) {
      result.children = [];
      for (const c of this.children) {
        result.add(c.clone());
      }
    }
    return result;
  }

  replace(node: TennNode): void {
    this.children = null;
    if (node.children !== null) {
      for (const c of node.children) {
        this.add(c.clone());
      }
    }
  }

  traverse(visitor: (node: TennNode) => boolean): boolean {
    if (!visitor(this)) {
      return false;
    }
    if (this.children !== null) {
      for (const c of this.children) {
        if (!c.traverse(visitor)) {
          return false;
        }
      }
    }
    return true;
  }

  add(...nodes: TennNode[]): void {
    for (const node of nodes) {
      if (this.children === null) {
        this.children = [];
      }
      this.children.push(node);

      if (this.kind === "BlockExpr") {
        if (this.named === null) {
          this.named = new Map();
        }
        const name = node.getIdent(0);
        if (name !== null) {
          this.named.set(name, node);
        }
      }
    }
  }

  getNamedElement(name: string): TennNode | null {
    if (this.kind !== "BlockExpr") {
      return null;
    }
    return this.named?.get(name) ?? null;
  }

  // Swift removes from a copy of `named`, so the lookup table keeps the entry. Kept as is.
  removeNamed(name: string): boolean {
    if (this.children !== null) {
      const oldSize = this.children.length;
      this.children = this.children.filter((itm) => itm.getIdent(0) !== name);
      return oldSize !== this.children.length;
    }
    return false;
  }

  getBlock(index: number): TennNode[] {
    return this.getChild(index)?.children ?? [];
  }

  getIdentText(): string | null {
    switch (this.kind) {
      case "Ident":
      case "StringLit":
      case "IntLit":
      case "FloatLit":
      case "CharLit":
      case "ExpressionBlock":
      case "Expression":
      case "MarkdownLit":
      case "Image":
        return this.token?.literal ?? null;
      default:
        return null;
    }
  }

  getIdent(...childIndex: number[]): string | null {
    return this.getChild(childIndex)?.getIdentText() ?? null;
  }

  getInt(...childIndex: number[]): number | null {
    const val = this.getChild(childIndex)?.getIdentText();
    return val != null && intRe.test(val) ? Number(val) : null;
  }

  getFloat(...childIndex: number[]): number | null {
    const val = this.getChild(childIndex)?.getIdentText();
    return val != null && floatRe.test(val) ? Math.fround(Number(val)) : null;
  }

  isNamedElement(): boolean {
    return this.kind === "Command" && this.count > 0 && this.children![0]!.kind === "Ident";
  }

  getChild(childIndex: number | number[]): TennNode | null {
    const path = typeof childIndex === "number" ? [childIndex] : childIndex;
    let nde: TennNode = this;
    for (const pos of path) {
      const nchilds = nde.children;
      if (nchilds === null || pos < 0 || pos >= nchilds.length) {
        return null;
      }
      nde = nchilds[pos]!;
    }
    return nde;
  }

  private getValueStr(name: string): string | null {
    const cmd = this.getNamedElement(name);
    if (cmd !== null && cmd.count > 1) {
      return cmd.getIdent(1);
    }
    return null;
  }

  // Swift has three overloads (String/Int/Bool); the default value's type picks one here.
  getValue(name: string, defaultValue: string): string;
  getValue(name: string, defaultValue: number): number;
  getValue(name: string, defaultValue: boolean): boolean;
  getValue(name: string, defaultValue: string | number | boolean): string | number | boolean {
    const raw = this.getValueStr(name);
    if (raw === null) {
      return defaultValue;
    }
    const value = raw.replace(trimRe, "");
    if (typeof defaultValue === "string") {
      return value;
    }
    if (typeof defaultValue === "number") {
      return intRe.test(value) ? Number(value) : defaultValue;
    }
    return value === "true" ? true : value === "false" ? false : defaultValue;
  }
}

export function newIdent(token: TennToken | string): TennNode {
  return new TennNode("Ident", typeof token === "string" ? newToken("symbol", token) : token);
}

export function newStrNode(literal: string): TennNode {
  return new TennNode("StringLit", newToken("stringLit", literal));
}

export function newImageNode(literal: string): TennNode {
  return new TennNode("Image", newToken("imageData", literal));
}

export function newMarkdownNode(literal: string): TennNode {
  return new TennNode("MarkdownLit", newToken("markdownLit", literal));
}

// Mirrors Swift String(Double): shortest digits, exponent form outside [1e-4, 2^53], two-digit exponent.
export function swiftDouble(value: number): string {
  if (Number.isNaN(value)) {
    return "nan";
  }
  if (!Number.isFinite(value)) {
    return value < 0 ? "-inf" : "inf";
  }
  if (value === 0) {
    return Object.is(value, -0) ? "-0.0" : "0.0";
  }
  const sign = value < 0 ? "-" : "";
  const a = Math.abs(value);
  const [mant, exp] = a.toExponential().split("e");
  const digits = mant.replace(".", "");
  const e = Number(exp);
  if (a > 2 ** 53 || a < 1e-4) {
    const frac = digits.length > 1 ? "." + digits.slice(1) : "";
    return `${sign}${digits[0]}${frac}e${e < 0 ? "-" : "+"}${String(Math.abs(e)).padStart(2, "0")}`;
  }
  if (e < 0) {
    return `${sign}0.${"0".repeat(-e - 1)}${digits}`;
  }
  const int = digits.length > e + 1 ? digits.slice(0, e + 1) : digits.padEnd(e + 1, "0");
  return `${sign}${int}.${digits.slice(e + 1) || "0"}`;
}

export function newFloatNode(value: number): TennNode {
  return new TennNode("FloatLit", newToken("floatLit", swiftDouble(value)));
}

export function newIntNode(value: number): TennNode {
  return new TennNode("IntLit", newToken("intLit", String(value)));
}

export function newNode(kind: TennNodeKind, token: TennToken | null = null): TennNode {
  return new TennNode(kind, token);
}

export function newBlockExpr(...children: TennNode[]): TennNode {
  const nde = new TennNode("BlockExpr", null);
  nde.add(...children);
  return nde;
}

export function newCommand(name: string, ...childNodes: TennNode[]): TennNode {
  const nde = new TennNode("Command");
  nde.add(newIdent(name));
  nde.add(...childNodes);
  return nde;
}
