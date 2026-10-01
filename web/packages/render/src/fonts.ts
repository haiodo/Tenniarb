// The bundled default font. Files live in ../fonts; hosts register them explicitly (node.ts for Node, FontFace in a browser).
// Never rely on system font names: @napi-rs/canvas and skia-canvas each substitute unknown families silently.
import { DEFAULT_FONT_FAMILY } from "./text.ts";

export interface FontFile {
  family: string;
  weight: "normal" | "bold";
  style: "normal" | "italic";
  /** Path relative to the package's fonts directory. */
  file: string;
}

export const INTER_FONTS: readonly FontFile[] = [
  { family: DEFAULT_FONT_FAMILY, weight: "normal", style: "normal", file: "Inter-Regular.ttf" },
  { family: DEFAULT_FONT_FAMILY, weight: "bold", style: "normal", file: "Inter-Bold.ttf" },
  { family: DEFAULT_FONT_FAMILY, weight: "normal", style: "italic", file: "Inter-Italic.ttf" },
  { family: DEFAULT_FONT_FAMILY, weight: "bold", style: "italic", file: "Inter-BoldItalic.ttf" },
];
