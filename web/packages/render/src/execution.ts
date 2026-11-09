// The classic item renderer's evaluator: an ExecutionContext whose text measure and item size come from
// this package's own layout, so the scene port reads classic expressions the same way renderElement does.
import { ExecutionContext } from "@tenniarb/core";
import type { DiagramItem } from "@tenniarb/core";
import { DrawableScene } from "./scene.ts";
import { DrawableContainer } from "./drawable.ts";
import type { Size } from "./geometry.ts";
import type { ImageDecoder } from "./images.ts";
import { AttributedString, layoutLines, systemFont } from "./text.ts";

/** utils.textSize: a frame of at most 1000 wide in the default font (CTFramesetterSuggestFrameSizeWithConstraints). */
function measureTextSize(text: string, fontSize: number): Size {
  const lines = layoutLines(AttributedString.plain(text, systemFont(fontSize)), 1000);
  let width = 0;
  let height = 0;
  for (const l of lines) {
    width = Math.max(width, l.width);
    height += l.textAscent + l.textDescent;
  }
  return { width: Math.ceil(width), height: Math.ceil(height) };
}

/** ExecutionContext whose utils.textSize and item width/height come from real text layout and drawable bounds. */
export function createExecutionContext(options: { evaluate?: boolean; decodeImage?: ImageDecoder } = {}): ExecutionContext {
  const context: ExecutionContext = new ExecutionContext({
    evaluate: options.evaluate,
    measureText: measureTextSize,
    // Swift builds a throwaway scene per evaluation round and reads the item's drawable size from it.
    itemSize: (item) => {
      if (item.parent === null) {
        return null;
      }
      const scene = new DrawableScene(item.parent, false, context, { buildChildren: false, decodeImage: options.decodeImage });
      scene.buildItemDrawable(item, new DrawableContainer());
      const bounds = scene.drawables.get(item)?.getSelectorBounds();
      return bounds === undefined ? null : { width: bounds.width, height: bounds.height };
    },
  });
  return context;
}
