// Public entry points: build a scene, measure it, draw it. No DOM: the Canvas2D context is passed in.
import type { DiagramItem, Element, ExecutionContext, ExecutionContextEvaluator } from "@tenniarb/core";
import type { Canvas2D } from "./canvas-types.ts";
import { DrawableContainer } from "./drawable.ts";
import type { Drawable } from "./drawable.ts";
import { createExecutionContext } from "./execution.ts";
import type { Rect, Size } from "./geometry.ts";
import type { ImageDecoder } from "./images.ts";
import { DrawableScene } from "./scene.ts";
import { buildTennScene, isSceneElement } from "./scene-tenn.ts";
import type { SceneLoadOptions, TennScene } from "./scene-tenn.ts";
import { setMeasureContext } from "./text.ts";

export interface RenderOptions {
  darkMode?: boolean;
  /** false: expressions are not run, tokens render as written. Default true. */
  evaluate?: boolean;
  /** Reuse an evaluated context (editor); by default one is created and set to `element`. */
  executionContext?: ExecutionContext;
  /** Margin around the scene, default 15. */
  padding?: number;
  /** CSS colour filling the whole image before drawing; transparent by default. */
  background?: string;
  /** Draw only these items (Swift: DrawableScene items:). */
  items?: DiagramItem[];
  decodeImage?: ImageDecoder;
  /** Context used to measure text when no earlier call has set one (buildScene/getSceneSize); renderElement uses its own. */
  measureContext?: Pick<Canvas2D, "font" | "measureText">;
  /** The file the element was read from and how to read the files it mounts with `element --from-file`. */
  files?: SceneLoadOptions;
}

/** The classic renderer's evaluator, shared with the scene port so both run the same expressions. */
export { createExecutionContext } from "./execution.ts";

/**
 * A scene element (spec 4, 5b, 5c) drawn by the port of go/scene instead of the classic item renderer:
 * the same DrawableScene surface (bounds, layout, draw in y-up space), so every caller stays the same.
 * The expanded nodes are not the model items, so nothing in `drawables`: an editor shows the picture but
 * does not select its parts yet.
 */
class SceneView extends DrawableScene {
  readonly tenn: TennScene;

  constructor(element: Element, options: RenderOptions) {
    super(element, options.darkMode ?? false, null, {});
    this.tenn = buildTennScene(element, options.files);
  }

  /** The classic item renderer has nothing to build: this scene draws its own nodes. */
  override buildElementScene(): Drawable {
    return new DrawableContainer();
  }

  override getBounds(): Rect {
    return this.tenn.bounds;
  }

  override layout(_bounds: Rect, _dirty: Rect): void {}

  override draw(context: Canvas2D, at = this.offset): void {
    this.tenn.draw(context, at);
  }

  override drawBox(context: Canvas2D, at = this.offset): void {
    this.tenn.draw(context, at);
  }
}

/** Evaluated, built and laid out scene, ready for draw(). */
export function buildScene(element: Element, options: RenderOptions = {}): DrawableScene {
  if (options.measureContext !== undefined) {
    setMeasureContext(options.measureContext);
  }
  if (isSceneElement(element)) {
    return new SceneView(element, options);
  }
  let executionContext = options.executionContext;
  if (executionContext === undefined) {
    executionContext = createExecutionContext({ evaluate: options.evaluate, decodeImage: options.decodeImage });
    executionContext.setElement(element);
  }
  return layoutScene(element, executionContext, options);
}

/** buildScene with a given context; null renders tokens as written. Bundles that call only this never pull in the evaluator. */
export function layoutScene(element: Element, executionContext: ExecutionContextEvaluator | null, options: Omit<RenderOptions, "evaluate" | "executionContext"> = {}): DrawableScene {
  if (options.measureContext !== undefined) {
    setMeasureContext(options.measureContext);
  }
  if (isSceneElement(element)) {
    return new SceneView(element, options);
  }
  const scene = new DrawableScene(element, options.darkMode ?? false, executionContext, { items: options.items, decodeImage: options.decodeImage });
  const bounds = scene.getBounds();
  scene.layout(bounds, bounds);
  return scene;
}

/** Size of the image renderElement produces, without drawing (e.g. for a PDF page). Needs a measure context. */
export function getSceneSize(element: Element, options: RenderOptions = {}): Size {
  const bounds = buildScene(element, options).getBounds();
  const padding = options.padding ?? 15;
  return { width: bounds.width + padding * 2, height: bounds.height + padding * 2 };
}

/**
 * Draws `element` into the top-left corner of `ctx` and returns the rect it covers ({0, 0, width, height}, canvas units).
 * Swift's scene is y-up, so the canvas is flipped once here; text and images flip back locally.
 */
export function renderElement(ctx: Canvas2D, element: Element, options: RenderOptions = {}): Rect {
  setMeasureContext(ctx);
  return drawScene(ctx, buildScene(element, options), options);
}

/** renderElement for a scene built by the caller. */
export function drawScene(ctx: Canvas2D, scene: DrawableScene, options: Pick<RenderOptions, "padding" | "background"> = {}): Rect {
  const bounds = scene.getBounds();
  const padding = options.padding ?? 15;
  const result: Rect = { x: 0, y: 0, width: bounds.width + padding * 2, height: bounds.height + padding * 2 };

  ctx.save();
  if (options.background !== undefined) {
    ctx.fillStyle = options.background;
    ctx.fillRect(0, 0, result.width, result.height);
  }
  ctx.translate(0, result.height);
  ctx.scale(1, -1);
  scene.offset = { x: padding - bounds.x, y: padding - bounds.y };
  scene.draw(ctx);
  ctx.restore();
  return result;
}
