// Port of TennPrinter.swift.
import type { TennNode } from "./model.ts";

const spaces = "    ";

function quote(val: string): string {
  return val.replaceAll('"', '\\"');
}

export function childsToStr(node: TennNode, ind: number, clean: boolean): string {
  let result = "";
  const children = node.children;
  if (children !== null) {
    children.forEach((c, i) => {
      result += toStr(c, ind, clean);
      if (i !== children.length - 1) {
        result += node.kind === "BlockExpr" || node.kind === "Statements" ? "\n" : " ";
      }
    });
  }
  return result;
}

export function toStr(node: TennNode, indent = 0, clean = false): string {
  let result = "";

  if (node.kind === "Command") {
    result += spaces.repeat(indent);
  }
  const tok = node.token;
  if (tok !== null) {
    switch (node.kind) {
      case "CharLit":
      case "IntLit":
      case "Ident":
      case "FloatLit":
        result += tok.literal;
        break;
      case "StringLit":
        result += clean ? tok.literal : `"${quote(tok.literal)}"`;
        break;
      case "Expression":
        result += `$(${tok.literal})`;
        break;
      case "Image":
        result += `@(${tok.literal})`;
        break;
      case "ExpressionBlock":
        result += `\${${tok.literal}}`;
        break;
      case "MarkdownLit":
        result += `%{${tok.literal}}`;
        break;
    }
  }
  let ind = indent;
  let postfix: string | null = null;
  if (node.kind === "BlockExpr") {
    result += "{\n";
    postfix = node.count > 0 ? `\n${spaces.repeat(indent)}}` : `${spaces.repeat(indent)}}`;
    ind += 1;
  }
  result += childsToStr(node, ind, clean);
  if (postfix !== null) {
    result += postfix;
  }
  return result;
}
