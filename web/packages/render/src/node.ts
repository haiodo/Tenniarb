// Node-only helpers (file paths of the bundled fonts); import from "@tenniarb/render/node".
import { fileURLToPath } from "node:url";
import { INTER_FONTS } from "./fonts.ts";

export function interFontPath(file: string): string {
  return fileURLToPath(new URL(`../fonts/${file}`, import.meta.url));
}

/** `GlobalFonts` from @napi-rs/canvas. Every file registers under the family name "Inter"; weight/style come from the file itself. */
export function registerInterFonts(globalFonts: { registerFromPath(path: string, family?: string): unknown }): void {
  for (const f of INTER_FONTS) {
    globalFonts.registerFromPath(interFontPath(f.file), f.family);
  }
}

export * from "./render-tenn.ts";
