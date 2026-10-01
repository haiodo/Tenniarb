// ElementImageProvider from ElementScene.swift. Decoding bytes to something drawImage accepts is the host's job (no DOM here).
import { ImageProvider } from "@tenniarb/markdown";
import type { CachedImage } from "@tenniarb/markdown";
import type { DiagramItem, Element } from "@tenniarb/core";

/** `image` is passed to Canvas2D.drawImage as is; width/height are its natural size. */
export interface DecodedImage {
  image: unknown;
  width: number;
  height: number;
}

/** Must be synchronous and return an image that is ready to draw; see preloadImages for async loaders. */
export type ImageDecoder = (base64: string) => DecodedImage | null;

// Data(base64Encoded:, options: .ignoreUnknownCharacters)
const sanitizeBase64 = (data: string): string => data.replace(/[^A-Za-z0-9+/=]/g, "");

/**
 * Loads every `image name data` of the element's items with an async loader (browser Image.decode(), @napi-rs/canvas
 * loadImage) and returns the sync decoder the scene needs. Payloads that fail to load stay unresolved.
 */
export async function preloadImages(element: Element, load: (base64: string) => Promise<DecodedImage | null>): Promise<ImageDecoder> {
  const payloads = new Set<string>();
  for (const item of element.items) {
    item.properties.node.traverse((child) => {
      const data = child.getIdent(0) === "image" && child.getIdent(1) !== null ? child.getIdent(2) : null;
      if (data !== null) {
        payloads.add(sanitizeBase64(data));
      }
      return true;
    });
  }
  const loaded = new Map<string, DecodedImage | null>();
  await Promise.all(
    [...payloads].map(async (p) => {
      try {
        loaded.set(p, await load(p));
      } catch {
        loaded.set(p, null);
      }
    }),
  );
  return (base64) => loaded.get(base64) ?? null;
}

/** Cache images inside items with required processing. */
export class ElementImageProvider extends ImageProvider {
  item: DiagramItem;
  decode: ImageDecoder | null;
  /** The markdown package keeps the base64 string as CachedImage.image; this maps it to the decoded handle. */
  decoded = new Map<string, DecodedImage>();

  constructor(item: DiagramItem, scaleFactor: number, decode: ImageDecoder | null = null) {
    super(scaleFactor);
    this.item = item;
    this.decode = decode;
  }

  override resolveImage(name: string): CachedImage | null {
    let result: CachedImage | null = null;
    this.item.properties.node.traverse((child) => {
      const cmdName = child.getIdent(0);
      const imgName = child.getIdent(1);
      const imgData = child.getIdent(2);
      if (cmdName === "image" && imgName === name && imgData !== null && this.decode !== null) {
        const base64 = sanitizeBase64(imgData);
        let img: DecodedImage | null = null;
        try {
          img = this.decode(base64);
        } catch {
          img = null;
        }
        if (img !== null) {
          this.decoded.set(base64, img);
          result = { image: base64, size: { width: img.width, height: img.height } };
          return false;
        }
      }
      return true;
    });
    return result;
  }
}
