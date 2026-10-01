import { firstIndexOf, trimmed } from "./characters.ts";

export interface Size {
  width: number;
  height: number;
}

export interface Rect {
  x: number;
  y: number;
  width: number;
  height: number;
}

// image is whatever the renderer loads (e.g. a data URL), it is not interpreted here.
export interface CachedImage {
  image: string;
  size: Size;
}

const int64Min = -(2n ** 63n);
const int64Max = 2n ** 63n - 1n;

// Swift Int(_:radix: 10): optional sign and ASCII digits only, nil outside Int64.
function parseInt10(s: string): number | null {
  if (!/^[+-]?[0-9]+$/.test(s)) {
    return null;
  }
  const v = BigInt(s);
  return v < int64Min || v > int64Max ? null : Number(v);
}

/**
 Cache images inside items with required processing.
 */
export class ImageProvider {
  readonly scaleFactor: number;
  images = new Map<string, CachedImage>();

  constructor(scaleFactor: number) {
    this.scaleFactor = scaleFactor;
  }

  resolveImage(_name: string): CachedImage | null {
    // Not implemented by default
    return null;
  }

  // path is named and style combimned parsed from @(name|style)
  resolveImagePath(path: string): { image: string; rect: Rect } | null {
    // If image already cached.
    let image = this.images.get(path) ?? null;
    if (image !== null) {
      return { image: image.image, rect: { x: 0, y: 0, width: image.size.width, height: image.size.height } };
    }
    let name = path;
    let style = "";
    const pos = firstIndexOf(path, "|");
    if (pos >= 0) {
      name = path.slice(0, pos);
      style = path.slice(pos + 1);
    }

    // Retrieve image data from properties, if not cached
    image = this.resolveImage(name);

    if (image !== null) {
      let width = image.size.width;
      let height = image.size.height;
      if (style !== "") {
        let widthStr = style;
        let heightStr = "";
        // We need to apply style is applicable
        const xPos = firstIndexOf(style, "x");
        if (xPos >= 0) {
          widthStr = trimmed(style.slice(0, xPos));
          heightStr = trimmed(style.slice(xPos + 1));
        }

        // This is aspect scale.
        const newWidth = widthStr === "" ? null : parseInt10(widthStr);
        if (newWidth !== null) {
          width = newWidth;
        }
        const newHeight = heightStr === "" ? null : parseInt10(heightStr);
        if (newHeight !== null) {
          height = newHeight;
        }

        if (widthStr === "" || heightStr === "") {
          const r = getMaxRect(width, height, image.size.width, image.size.height);
          width = r.width;
          height = r.height;
        }
      }
      // The Swift version also rescales the bitmap to size * scaleFactor; the renderer does that when drawing.
      image.size = { width, height };

      this.images.set(path, image);
      return { image: image.image, rect: { x: 0, y: 0, width, height } };
    }

    return null;
  }
}

export function getMaxRect(maxWidth: number, maxHeight: number, imageWidth: number, imageHeight: number): Rect {
  // Get ratio (landscape or portrait)
  const ratiox = maxWidth / imageWidth;
  const ratioy = maxHeight / imageHeight;

  // Swift.min(a, b) is b < a ? b : a, which differs from Math.min on NaN.
  let ratio = ratioy < ratiox ? ratioy : ratiox;

  // Calculate new size based on the ratio
  if (ratio > 1) {
    ratio = 1;
  }
  return { x: 0, y: 0, width: imageWidth * ratio, height: imageHeight * ratio };
}
