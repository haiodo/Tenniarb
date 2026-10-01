// Swift Strings are Character (grapheme cluster) based; these helpers reproduce that where it is observable.
const segmenter = new Intl.Segmenter(undefined, { granularity: "grapheme" });

// Nothing below U+0300 combines with a neighbour, except CR LF.
const simple = /^[^̀-￿]*$/;

export function graphemes(s: string): string[] {
  if (simple.test(s) && !s.includes("\r\n")) {
    return s.split("");
  }
  return Array.from(segmenter.segment(s), (g) => g.segment);
}

export function graphemeCount(s: string): number {
  return simple.test(s) && !s.includes("\r\n") ? s.length : graphemes(s).length;
}

// UTF-16 index of the first Character equal to ch (a single ASCII char), -1 if none.
export function firstIndexOf(s: string, ch: string): number {
  const i = s.indexOf(ch);
  if (i < 0 || ((i === 0 || s.charCodeAt(i - 1) < 0x300) && (i + 1 >= s.length || s.charCodeAt(i + 1) < 0x300))) {
    return i;
  }
  let off = 0;
  for (const g of graphemes(s)) {
    if (g === ch) {
      return off;
    }
    off += g.length;
  }
  return -1;
}

export function startsWithChar(s: string, ch: string): boolean {
  return s.startsWith(ch) && (s.length === ch.length || s.charCodeAt(ch.length) < 0x300 || graphemes(s)[0] === ch);
}

// CharacterSet.whitespacesAndNewlines: Zs/Zl/Zp, U+0009-U+000D, U+0085.
export const space = "[\\t-\\r \\u0085\\u00a0\\u1680\\u2000-\\u200a\\u2028\\u2029\\u202f\\u205f\\u3000]";
const leading = new RegExp(`^${space}+`);
const trailing = new RegExp(`${space}+$`);

export function trimmed(s: string): string {
  return s.replace(leading, "").replace(trailing, "");
}
