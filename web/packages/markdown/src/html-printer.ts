import { firstIndexOf, graphemes, trimmed } from "./characters.ts";
import { colorToHex, parseColor } from "./color-utils.ts";
import type { ImageProvider } from "./images.ts";
import type { MarkdownToken } from "./types.ts";

export function calcTitleFontSize(text: string, fontSize: number): [string, number, number] {
  let hlevel = 0;
  let calch = true;
  let result = "";
  for (const c of graphemes(text)) {
    if (calch) {
      if (c === "#") {
        hlevel += 1;
        continue;
      } else if (c === " " || c === "\t") {
        continue;
      } else {
        calch = false;
      }
    }
    result += c;
  }
  return [result, fontSize + 5 - hlevel * 2, hlevel];
}

// Swift Double(String): strtod syntax, nothing else (no whitespace, no "_"). NaN payloads and sign are not kept.
function parseDouble(s: string): number | null {
  const sign = s.startsWith("-") ? -1 : 1;
  const body = /^[+-]/.test(s) ? s.slice(1) : s;
  if (/^(inf|infinity)$/i.test(body)) {
    return sign * Infinity;
  }
  if (/^nan(\(\w*\))?$/i.test(body)) {
    return NaN;
  }
  if (/^(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$/.test(body)) {
    return sign * Number(body);
  }
  const hex = /^0[xX]([0-9a-fA-F]*)\.?([0-9a-fA-F]*)(?:[pP]([+-]?\d+))?$/.exec(body);
  if (hex !== null && hex[1] + hex[2] !== "") {
    return sign * Number(BigInt("0x" + (hex[1] + hex[2]))) * 2 ** (Number(hex[3] ?? 0) - 4 * hex[2].length);
  }
  return null;
}

// Swift "\(Double)": shortest round-trip digits, always with a fraction, exponent form outside 1e-4..<1e16.
function swiftDouble(n: number): string {
  if (Number.isNaN(n)) {
    return "nan";
  }
  if (!Number.isFinite(n)) {
    return n < 0 ? "-inf" : "inf";
  }
  if (Object.is(n, -0)) {
    return "-0.0";
  }
  const [mant, exp] = Math.abs(n).toExponential().split("e");
  const e = Number(exp);
  const sign = n < 0 ? "-" : "";
  const digits = mant.replace(".", "");
  if (e < -4 || e >= 16) {
    return `${sign}${mant}e${e < 0 ? "-" : "+"}${String(Math.abs(e)).padStart(2, "0")}`;
  }
  if (e < 0) {
    return `${sign}0.${"0".repeat(-e - 1)}${digits}`;
  }
  const int = digits.padEnd(e + 1, "0");
  return `${sign}${int.slice(0, e + 1)}.${int.slice(e + 1) || "0"}`;
}

// Swift String.hasPrefix/hasSuffix/contains compare Characters, and "\r\n" is a single Character.
const hasNewline = (s: string) => /(?<!\r)\n/.test(s);
const endsWithNewline = (s: string) => /(?<!\r)\n$/.test(s);

const escapes: Record<string, string> = { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" };

// Foundation's replacingOccurrences does not match when a combining mark (GCB Extend, not ZWJ) follows.
const escapable = /[&<>"'](?![\p{Grapheme_Extend}\p{Emoji_Modifier}])/gu;

const isBlank = (s: string) => trimmed(s) === "";

export function toHTML(tokens: MarkdownToken[], originalFont: number, textColor: string, imageProvider: ImageProvider): string {
  // The wrapper div no longer has a default font-size style.
  let result = "<div>";

  // State variables for the *current* effective style.
  let currentColor = "";
  let currentFontSize = originalFont;

  // State variables to track the *last* style applied to an element,
  // to minimize redundant style attributes.
  let lastAppliedColor = "";
  let lastAppliedFontSize = originalFont;

  let prevMultiCode = false;
  let lastLiteral = "";
  let lastToken = "eof";
  let inList = false;

  for (const t of tokens) {
    // Escape HTML special characters in text
    let literal = t.literal.replace(escapable, (c) => escapes[c]);

    if (prevMultiCode) {
      prevMultiCode = false;
      if (literal.startsWith("\n")) {
        literal = literal.slice(1);
      }
      const colorValue = parseColor("grey-200");
      result += `<pre style="background-color: ${colorToHex(colorValue)}; color: black;">\n</pre>`;
    }

    if (t.type !== "eof") {
      lastToken = t.type;
    }

    switch (t.type) {
      case "text": {
        const styleAttributes: string[] = [];

        // Only include font-size if it changed from the last applied size
        if (currentFontSize !== lastAppliedFontSize) {
          styleAttributes.push(`font-size: ${swiftDouble(currentFontSize)}px`);
          lastAppliedFontSize = currentFontSize;
        }

        // Only include color if it changed from the last applied color
        if (currentColor !== lastAppliedColor) {
          styleAttributes.push(currentColor);
          lastAppliedColor = currentColor;
        }

        const styleAttr = styleAttributes.length === 0 ? "" : ` style="${styleAttributes.join("; ")}"`;
        result += `<span${styleAttr}>${literal}</span>`;
        break;
      }
      case "bold":
        result += `<strong>${literal}</strong>`;
        break;
      case "bullet":
        if (!inList) {
          result += `<ul style="margin-left: ${5 * graphemes(literal).length}px;">\n`;
          inList = true;
        }
        result += `<li>${literal}</li>\n`;
        break;
      case "image": {
        const resolved = imageProvider.resolveImagePath(t.literal);
        if (resolved !== null) {
          result += `<img src="${t.literal}" width="${swiftDouble(resolved.rect.width)}" height="${swiftDouble(resolved.rect.height)}" alt="${t.literal}" />`;
        }
        break;
      }
      case "italic":
        result += `<em>${literal}</em>`;
        break;
      case "underline":
        result += `<u>${literal}</u>`;
        break;
      case "scratch":
        result += `<del>${literal}</del>`;
        break;
      case "title": {
        const [title, titleSize, hlevel] = calcTitleFontSize(literal, originalFont);
        const headerTag = `h${Math.min(Math.max(hlevel, 1), 6)}`; // Ensure h1-h6
        result += `<${headerTag} style="font-size: ${swiftDouble(titleSize)}px;">${title}</${headerTag}>\n`;
        break;
      }
      case "color": {
        const splitPos = firstIndexOf(literal, "|");
        if (splitPos >= 0) {
          const color = literal.slice(0, splitPos);
          const word = literal.slice(splitPos + 1);

          const colorHex = colorToHex(parseColor(color));
          result += `<span style="color: ${colorHex};">${word}</span>`;
        } else if (isBlank(literal)) {
          // Reset color to default (empty)
          currentColor = "";
        } else {
          // Set new global color
          currentColor = `color: ${colorToHex(parseColor(literal))};`;
        }
        break;
      }
      case "font": {
        const splitPos = firstIndexOf(literal, "|");
        if (splitPos >= 0) {
          const fontSize = parseDouble(literal.slice(0, splitPos));
          if (fontSize !== null) {
            const word = literal.slice(splitPos + 1);
            // Inline font size, always apply the style
            result += `<span style="font-size: ${swiftDouble(fontSize)}px;${currentColor}">${word}</span>`;
          }
        } else if (isBlank(literal)) {
          // Reset font size to default
          currentFontSize = originalFont;
        } else {
          const fontSize = parseDouble(literal);
          if (fontSize !== null) {
            // Set new global font size
            currentFontSize = fontSize;
          }
        }
        break;
      }
      case "code": {
        const colorValue = parseColor("grey-200");
        if (hasNewline(literal)) {
          if (!literal.startsWith("\n") && !endsWithNewline(lastLiteral)) {
            result += "<br>";
          }
          if (!endsWithNewline(literal)) {
            prevMultiCode = true;
          }
          result += `<pre style="background-color: ${colorToHex(colorValue)}; color: black;">${literal}</pre>`;
        } else {
          result += `<code style="background-color: ${colorToHex(colorValue)}; color: black;">${literal}</code>`;
        }
        break;
      }
    }
    lastLiteral = literal;
  }

  if (inList) {
    result += "</ul>\n";
  }

  if (lastToken === "image") {
    result += "<br>";
  }

  result += "</div>";

  return result;
}
