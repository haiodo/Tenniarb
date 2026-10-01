// CGColor as RGBA in 0...1. This is the app's parseColor (Optional, hex 6/8 digits); cmkdown's (never fails) comes from @tenniarb/markdown.
import { ColorNames } from "@tenniarb/markdown";

export interface Color {
  r: number;
  g: number;
  b: number;
  a: number;
}

export const colorBlack: Color = { r: 0, g: 0, b: 0, a: 1 };
export const colorWhite: Color = { r: 1, g: 1, b: 1, a: 1 };

// CGColor(red:green:blue:alpha:) is Generic RGB (gamma 1.8, own primaries); Swift draws it into an sRGB context, which converts.
// Columns are Generic RGB primaries in linear sRGB, measured with CGColor.converted(to: extendedLinearSRGB).
const GENERIC_TO_LINEAR_SRGB = [
  [1.025241732597351, -0.026545243337750435, 0.0013035525334998965],
  [0.019400764256715775, 0.9480088353157043, 0.032590351998806],
  [-0.0017621302977204323, -0.001434493693523109, 1.0031965970993042],
];

const clamp01 = (v: number): number => Math.max(0, Math.min(1, v));
const srgbEncode = (v: number): number => (v <= 0.0031308 ? 12.92 * v : 1.055 * v ** (1 / 2.4) - 0.055);

/** The sRGB channels (0...1) Swift ends up with for a Generic RGB colour. */
export function genericToSRGB(c: Color): [number, number, number] {
  const lin = [c.r, c.g, c.b].map((v) => clamp01(v) ** 1.8);
  return GENERIC_TO_LINEAR_SRGB.map((row) => srgbEncode(clamp01(row[0]! * lin[0]! + row[1]! * lin[1]! + row[2]! * lin[2]!))) as [number, number, number];
}

export function cssColor(c: Color): string {
  const [r, g, b] = genericToSRGB(c).map((v) => Math.round(v * 255));
  return `rgba(${r},${g},${b},${clamp01(c.a)})`;
}

export function withAlpha(c: Color, a: number): Color {
  return { r: c.r, g: c.g, b: c.b, a };
}

const hexDigits = /^[0-9a-fA-F]+/;

// Foundation Scanner.scanHexInt64: skips leading whitespace, optional 0x, then takes the leading run of hex digits.
function scanHexInt64(s: string): number | null {
  const m = hexDigits.exec(s.replace(/^[\t-\r ]+/, "").replace(/^0[xX]/, ""));
  return m === null ? null : parseInt(m[0], 16);
}

const trimWs = (s: string): string => s.trim();

// PreferencesController.parseColor
export function parseColor(hex: string, alpha = 1.0): Color | null {
  let hexSanitized = trimWs(hex).replaceAll("#", "");

  const name = hexSanitized.toLowerCase();
  if (Object.hasOwn(ColorNames, name)) {
    hexSanitized = ColorNames[name]!.replaceAll("#", "");
  }

  const rgb = scanHexInt64(hexSanitized);
  if (rgb === null) {
    return null;
  }
  const count = [...hexSanitized].length;
  if (count === 6) {
    return { r: ((rgb >>> 16) & 0xff) / 255, g: ((rgb >>> 8) & 0xff) / 255, b: (rgb & 0xff) / 255, a: alpha };
  }
  if (count === 8) {
    return { r: ((rgb >>> 24) & 0xff) / 255, g: ((rgb >>> 16) & 0xff) / 255, b: ((rgb >>> 8) & 0xff) / 255, a: (rgb & 0xff) / 255 };
  }
  return null;
}

export const styleBlack = colorBlack;
export const styleWhite = colorWhite;

export function getTextColorBasedOn(color: Color): Color {
  const sum = swiftRound((color.r * 255 * 299 + color.g * 255 * 587 + color.b * 255 * 114) / 1000);
  return sum > 128 ? styleBlack : styleWhite;
}

// Swift round(): half away from zero.
export const swiftRound = (x: number): number => Math.sign(x) * Math.round(Math.abs(x));
