import { space, startsWithChar, trimmed } from "./characters.ts";
import { ColorNames } from "./color-names.ts";

export { ColorNames };

// CGColor components, 0...1.
export interface RGBA {
  r: number;
  g: number;
  b: number;
  a: number;
}

export function hexStringToUIColor(hexString: string, alpha = 1.0): RGBA {
  hexString = trimmed(hexString);

  if (startsWithChar(hexString, "#")) {
    hexString = hexString.slice(1);
  }
  // Scanner.scanHexInt64: skips leading whitespace, accepts an 0x prefix, stops at the first non-hex
  // char, saturates on overflow, leaves 0 when there are no digits.
  const m = new RegExp(`^${space}*(?:0[xX])?([0-9a-fA-F]*)`).exec(hexString)!;
  let color = m[1] === "" ? 0n : BigInt("0x" + m[1]);
  if (color > 0xffffffffffffffffn) {
    color = 0xffffffffffffffffn;
  }
  return {
    r: Number((color >> 16n) & 0xffn) / 255.0,
    g: Number((color >> 8n) & 0xffn) / 255.0,
    b: Number(color & 0xffn) / 255.0,
    a: alpha,
  };
}

export function parseColor(color: string, alpha = 1.0): RGBA {
  if (startsWithChar(color, "#")) {
    return hexStringToUIColor(color, alpha);
  }
  if (Object.hasOwn(ColorNames, color)) {
    return hexStringToUIColor(ColorNames[color], alpha);
  }

  return { r: 0, g: 0, b: 0, a: alpha };
}

// Swift iterates the dictionary (random order) when several names share a hex; here the first one in table order wins.
export function colorToHex(color: RGBA): string {
  const hex = (v: number) => Math.round(Math.fround(v * 255)).toString(16).toUpperCase().padStart(2, "0");
  const hexString = `#${hex(color.r)}${hex(color.g)}${hex(color.b)}`;

  for (const [key, value] of Object.entries(ColorNames)) {
    if (value === hexString) {
      return key;
    }
  }

  return hexString;
}
