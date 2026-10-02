import { INTER_FONTS } from "@tenniarb/render";

// Defined by scripts/build.ts for the standalone build: { "Inter-Regular.woff2": base64, ... }. Absent elsewhere.
declare const __INLINE_FONTS__: Record<string, string> | undefined;
const inlineFonts = typeof __INLINE_FONTS__ === "undefined" ? null : __INLINE_FONTS__;

// import.meta.url is blanked in the IIFE build, where the script's own src is the only anchor (valid during load only).
const scriptUrl = typeof document !== "undefined" && document.currentScript instanceof HTMLScriptElement ? document.currentScript.src : import.meta.url;
const dirOf = (url: string): string => new URL("./", url || location.href).href;
export const defaultFontBase = dirOf(scriptUrl);

const loaded = new Map<string, Promise<void>>();

/** Loads the four Inter faces once per base URL; every diagram on the page shares the promise. */
export function loadFonts(base: string = defaultFontBase): Promise<void> {
  const key = inlineFonts === null ? base : "";
  let p = loaded.get(key);
  if (p === undefined) {
    const dir = base.endsWith("/") ? base : base + "/";
    p = Promise.all(
      INTER_FONTS.map(async (f) => {
        const file = f.file.replace(".ttf", ".woff2");
        const src = inlineFonts === null ? `${dir}${file}` : `data:font/woff2;base64,${inlineFonts[file]}`;
        const face = new FontFace(f.family, `url(${src})`, { weight: f.weight, style: f.style });
        document.fonts.add(await face.load());
      }),
    ).then(() => undefined);
    // A failed load must not poison later render() calls; the diagram falls back to a system font.
    p.catch(() => loaded.delete(key));
    loaded.set(key, p);
  }
  return p;
}
