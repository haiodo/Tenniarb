// DrawableScene: builds drawables from an Element; evaluated values come from the ExecutionContext exactly as in Swift.
// Editor-only members are left for stage 7: activeDrawables/activeElements/selectionBox/editingMode/editBoxBounds,
// updateActiveElements, updateLayout (drag), find, and the selector drawing in draw/drawBox.
import { LinkItem, persistenceStyleName } from "@tenniarb/core";
import { toHTML } from "@tenniarb/markdown";
import type { MarkdownToken } from "@tenniarb/markdown";
import type { DiagramItem, Element, ExecutionContextEvaluator, LayoutScene } from "@tenniarb/core";
import type { Canvas2D } from "./canvas-types.ts";
import { CircleBox, DrawableContainer, DrawableLine, EmptyBox, InvisibleBox, RoundBox } from "./drawable.ts";
import type { Drawable, DrawableLayer } from "./drawable.ts";
import { crossBox, rectZero } from "./geometry.ts";
import type { Point, Rect } from "./geometry.ts";
import { ElementImageProvider } from "./images.ts";
import type { ImageDecoder } from "./images.ts";
import { DrawableItemStyle, DrawableLineStyle, DrawableStyle, SceneStyle, getString } from "./style.ts";
import { TextBox, calculateSize, getTokens, prepareBodyText, toAttributedString } from "./text.ts";
import type { TextPosition } from "./text.ts";

export interface SceneOptions {
  scaleFactor?: number;
  buildChildren?: boolean;
  /** Build only these items instead of element.items. */
  items?: DiagramItem[];
  /** How base64 image data becomes something Canvas2D.drawImage takes; without it images are skipped. */
  decodeImage?: ImageDecoder;
}

function parseLayer(style: DrawableStyle): DrawableLayer {
  switch (style.layer) {
    case "background":
      return "Background";
    case "hover":
      return "Hover";
    default:
      return "Default";
  }
}

function parseLayout(style: DrawableItemStyle, horizontal: TextPosition, vertical: TextPosition): [TextPosition, TextPosition] {
  if (style.layout !== null) {
    const ls = new Set(style.layout.split(",").map((t) => t.trim()));
    if (ls.has("center")) {
      horizontal = "Center";
    }
    if (ls.has("left")) {
      horizontal = "Left";
    }
    if (ls.has("right")) {
      horizontal = "Right";
    }
    if (ls.has("middle")) {
      vertical = "Middle";
    }
    if (ls.has("top")) {
      vertical = "Top";
    }
    if (ls.has("fill")) {
      vertical = "Fill";
    }
    if (ls.has("bottom")) {
      vertical = "Bottom";
    }
  }
  return [horizontal, vertical];
}

export class DrawableScene extends DrawableContainer implements LayoutScene {
  offset: Point = { x: 0, y: 0 };

  drawables = new Map<DiagramItem, Drawable>();

  itemToLink = new Map<DiagramItem, DiagramItem[]>();

  lineToDrawable: DrawableLine | null = null;

  sceneStyle: SceneStyle;

  darkMode: boolean;

  executionContext: ExecutionContextEvaluator | null;

  readonly scaleFactor: number;
  readonly decodeImage: ImageDecoder | null;

  constructor(element: Element, darkMode: boolean, executionContext: ExecutionContextEvaluator | null, options: SceneOptions = {}) {
    super();
    this.sceneStyle = new SceneStyle(darkMode);
    this.darkMode = darkMode;
    this.executionContext = executionContext;
    this.scaleFactor = options.scaleFactor ?? 1;
    this.decodeImage = options.decodeImage ?? null;

    this.bounds = rectZero();
    this.append(this.buildElementScene(element, darkMode, options.buildChildren ?? true, options.items ?? element.items));
  }

  /** LayoutScene */
  getItemBounds(node: DiagramItem): Rect | null {
    return this.drawables.get(node)?.getBounds() ?? null;
  }

  override layout(bounds: Rect, dirty: Rect): void {
    this.bounds = bounds;
    super.layout(this.bounds, dirty);
  }

  /** Draw the scene at its `offset`. */
  override draw(context: Canvas2D, at: Point = this.offset): void {
    super.draw(context, at);
    this.lineToDrawable?.draw(context, at);
  }

  override drawBox(context: Canvas2D, at: Point = this.offset): void {
    super.drawBox(context, at);
    this.lineToDrawable?.draw(context, at);
  }

  /** Port of DrawableScene.updateLineTo: the preview line from `source` to `point`, or to the middle of `target` (the item under `point`, found by the caller) when given. */
  updateLineTo(source: DiagramItem, point: Point, target: DiagramItem | null): void {
    const sb = this.drawables.get(source)?.getBounds();
    if (sb === undefined) return;
    const tb = target === null ? undefined : this.drawables.get(target)?.getBounds();
    const end = tb === undefined ? point : { x: tb.x + tb.width / 2, y: tb.y + tb.height / 2 };
    const centre = { x: sb.x + sb.width / 2, y: sb.y + sb.height / 2 };
    const mid = crossBox(centre, end, sb) ?? centre;
    const to = tb === undefined ? end : (crossBox(mid, end, tb) ?? end);
    this.lineToDrawable = new DrawableLine(mid, to, new DrawableLineStyle(this.darkMode));
  }

  removeLineTo(): void {
    this.lineToDrawable = null;
  }

  private newImageProvider(item: DiagramItem): ElementImageProvider {
    return new ElementImageProvider(item, this.scaleFactor, this.decodeImage);
  }

  private buildRoundRect(
    bounds: Rect,
    style: DrawableItemStyle,
    e: DiagramItem,
    textBox: TextBox,
    elementDrawable: DrawableContainer,
    fill = true,
    stack = 0,
  ): void {
    const rectBox = new RoundBox(bounds, style, fill);
    rectBox.stack = stack;
    rectBox.append(textBox);

    rectBox.item = e;
    rectBox.layer = parseLayer(style);

    this.drawables.set(e, rectBox);
    elementDrawable.append(rectBox);
  }

  private buildCircle(bounds: Rect, style: DrawableStyle, e: DiagramItem, textBox: TextBox, elementDrawable: DrawableContainer, fill = true): void {
    const rectBox = new CircleBox(bounds, style, fill);
    rectBox.append(textBox);

    rectBox.item = e;
    rectBox.layer = parseLayer(style);

    this.drawables.set(e, rectBox);
    elementDrawable.append(rectBox);
  }

  private buildEmptyRect(bounds: Rect, style: DrawableStyle, e: DiagramItem, textBox: TextBox, elementDrawable: DrawableContainer): void {
    const rectBox = new EmptyBox(bounds, style);
    rectBox.append(textBox);
    rectBox.item = e;
    rectBox.layer = parseLayer(style);

    this.drawables.set(e, rectBox);
    elementDrawable.append(rectBox);
  }

  private buildInvisibleRect(bounds: Rect, style: DrawableStyle, e: DiagramItem, textBox: TextBox, elementDrawable: DrawableContainer): void {
    const rectBox = new InvisibleBox(bounds, style);
    rectBox.append(textBox);
    rectBox.item = e;
    rectBox.layer = parseLayer(style);

    this.drawables.set(e, rectBox);
    elementDrawable.append(rectBox);
  }

  buildItemDrawable(e: DiagramItem, elementDrawable: DrawableContainer): void {
    const name = e.name;

    const style = this.sceneStyle.defaultItemStyle.copy();
    const evaluatedValues = this.executionContext?.getEvaluated(e) ?? new Map();

    const imageProvider = this.newImageProvider(e);

    // parse uses with list of styles.

    let properties = e.properties;
    let parentEl = e.parent;
    const includeNode = e.properties.get(persistenceStyleName("Inherit"));
    if (includeNode !== null && includeNode.count > 1) {
      const ident = includeNode.getIdent(1);
      if (ident !== null) {
        let name = ident;
        while (parentEl !== null && name.startsWith("../")) {
          name = name.slice(3);
          parentEl = parentEl.parent;
        }
        const index = includeNode.getInt(2) ?? 0;
        if (parentEl !== null) {
          const parentItems = parentEl.items.filter((it) => it.name === name);
          if (index >= 0 && index < parentItems.length) {
            const parentItem = parentItems[index]!;
            properties = parentItem.properties.clone();
            // Smart overide of all paren properties with ours
            for (const node of e.properties) {
              const nodeName = node.getIdentText();
              const override = node.isNamedElement() && nodeName !== null ? properties.get(nodeName) : null;
              if (override !== null) {
                override.replace(node); // Replace existing with new one.
              } else {
                // Just add
                properties.append(node.clone());
              }
            }
          }
        }
      }
    }

    const styleNode = e.properties.get(persistenceStyleName("UseStyle"));
    const parent = e.parent;
    const stylesBlock = parent?.properties.get("styles")?.getChild(1) ?? null;
    if (styleNode !== null && styleNode.count > 1 && styleNode.children !== null && parent !== null && stylesBlock !== null) {
      const elementEvaluated = this.executionContext?.getEvaluated(parent) ?? new Map();
      for (const c of styleNode.children.slice(1)) {
        const ident = c.getIdentText();
        const styleData = ident === null ? null : (stylesBlock.getNamedElement(ident)?.getChild(1) ?? null);
        if (styleData !== null) {
          style.parseStyle(styleData, elementEvaluated);
        }
      }
    }

    style.parseStyle(properties, evaluatedValues);

    // Check if we have include, we need to take values from it and do override.

    let titleValue = name.length > 0 ? name : " ";
    if (style.title !== null) {
      titleValue = style.title;
    }
    titleValue = prepareBodyText(titleValue);
    // Marker value
    if (style.marker !== null) {
      titleValue = style.marker + " " + titleValue;
    }
    const shift: Point = { x: 0, y: 0 };

    let bodyAttrString = null;
    let bodyTokens: MarkdownToken[] | null = null;
    let bodyFontSize = 0;
    const bodyNode = properties.get("body");
    if (bodyNode !== null) {
      // Body could have custome properties like width, height, color, font-size, so we will parse it as is.
      const bodyStyle = style.copy();
      bodyStyle.fontSize -= 2; // Make a bit smaller for body
      let textValue = "";
      const bodyBlock = bodyNode.getChild(1);
      if (bodyBlock !== null) {
        if (bodyBlock.kind === "BlockExpr") {
          bodyStyle.parseStyle(bodyBlock, evaluatedValues);

          const txtNode = bodyBlock.getNamedElement("text")?.getChild(1) ?? null;
          if (txtNode !== null) {
            const txt = getString(txtNode, evaluatedValues);
            if (txt !== null) {
              textValue = txt;
            }
          }
        } else {
          const txt = getString(bodyBlock, evaluatedValues);
          if (txt !== null) {
            textValue = txt;
          }
        }
      }
      const [horizontal, vertical] = parseLayout(bodyStyle, "Left", "Middle");
      bodyTokens = getTokens((titleValue.length > 0 ? "\n" : "") + prepareBodyText(textValue));
      bodyFontSize = bodyStyle.fontSize;
      bodyAttrString = toAttributedString(
        bodyTokens,
        { size: bodyStyle.fontSize, bold: false, italic: false },
        bodyStyle.textColor,
        shift,
        imageProvider,
        [vertical, horizontal],
        style.lineSpacing,
      );
    }

    let vertical: TextPosition = "Middle";
    let horizontal: TextPosition = "Center";

    const titleTokens = getTokens(titleValue);

    if (bodyAttrString !== null) {
      vertical = "Fill";
      horizontal = "Left";
    }

    // If markdown has titles, bullets, we need to change horizontal layout to left one.
    if (titleTokens.some((t) => t.type === "bullet" || t.type === "title" || t.type === "code")) {
      horizontal = "Left";
    }

    [horizontal, vertical] = parseLayout(style, horizontal, vertical);

    const attrString = toAttributedString(
      titleTokens,
      { size: style.fontSize, bold: false, italic: false },
      style.textColor,
      shift,
      imageProvider,
      [horizontal, vertical],
    );

    if (bodyAttrString !== null) {
      attrString.append(bodyAttrString);
    }
    const textBounds = calculateSize(attrString);

    let offx = 4;
    let offy = 4;

    let wx = offx * 2;
    let wy = offy * 2;

    let width = textBounds.width;
    if (style.width !== null && style.width >= 1) {
      width = style.width;
      wx = 0;
    }

    let height = textBounds.height;
    if (style.height !== null && style.height >= 1) {
      height = style.height;
      wy = 0;
    }

    if (width - offx >= textBounds.width) {
      wx = 0;
    }
    if (height - offy >= textBounds.height) {
      wy = 0;
    }

    if (width - offx > textBounds.width && width <= 20) {
      wx = 0;
      offx = (width - textBounds.width) / 2;
    }
    if (height - offy > textBounds.height && height <= 20) {
      wy = 0;
      offy = (height - textBounds.height) / 2;
    }

    const sz = { width: width + shift.x + wx, height: height + shift.y + wy };
    // Swift assigns this to the scene's own `bounds`, which layout() overwrites later.
    const bounds: Rect = { x: e.x, y: e.y - sz.height, width: sz.width, height: sz.height };
    this.bounds = bounds;

    const finalTextBounds: Rect = { x: offx, y: offy, width: textBounds.width + shift.x, height: textBounds.height + shift.y };

    if (finalTextBounds.width + offx * 2 < width) {
      finalTextBounds.width = width - offx * 2;
    }
    if (finalTextBounds.height + offy * 2 < height) {
      finalTextBounds.height = height - offy * 2;
    }

    switch (vertical) {
      case "Middle":
        if (finalTextBounds.height > textBounds.height) {
          const yshift = finalTextBounds.height - textBounds.height;
          finalTextBounds.y = offy + yshift / 2;
          finalTextBounds.height -= yshift;
        } else if (bounds.height < finalTextBounds.height) {
          const yshift = finalTextBounds.height - bounds.height + offy;
          finalTextBounds.y = offy - yshift / 2;
        }
        break;
      case "Top":
        if (finalTextBounds.height > textBounds.height) {
          const yshift = finalTextBounds.height - textBounds.height;
          finalTextBounds.y = offy + yshift;
          finalTextBounds.height -= yshift;
        } else if (bounds.height < finalTextBounds.height) {
          const yshift = finalTextBounds.height - bounds.height + offy;
          finalTextBounds.y = offy - yshift / 2;
        }
        break;
      case "Fill":
        if (bounds.height < finalTextBounds.height) {
          const yshift = finalTextBounds.height - bounds.height;
          finalTextBounds.y = -1 * (offy + yshift);
        }
        break;
      case "Bottom":
        if (finalTextBounds.height > textBounds.height) {
          const yshift = finalTextBounds.height - textBounds.height;
          finalTextBounds.y = offy;
          finalTextBounds.height -= yshift;
        } else if (bounds.height < finalTextBounds.height) {
          const yshift = finalTextBounds.height - bounds.height + offy;
          finalTextBounds.y = offy - yshift / 2;
        }
        break;
    }

    const textBox = new TextBox(attrString, finalTextBounds);
    // Lazy: only "Export text as html" needs it.
    textBox.html = () => toHTML(titleTokens, style.fontSize, "", imageProvider) + (bodyTokens === null ? "" : toHTML(bodyTokens, bodyFontSize, "", imageProvider));

    switch (style.display) {
      case "text":
        this.buildEmptyRect(bounds, style, e, textBox, elementDrawable);
        break;
      case "no-fill":
        // Swift computes a text colour from the preference background here and never uses it.
        this.buildRoundRect(bounds, style, e, textBox, elementDrawable, false);
        break;
      case "stack":
        this.buildRoundRect(bounds, style, e, textBox, elementDrawable, true, 3);
        break;
      case "circle":
        this.buildCircle(bounds, style, e, textBox, elementDrawable);
        break;
      case "none":
        this.buildInvisibleRect(bounds, style, e, textBox, elementDrawable);
        break;
      default:
        this.buildRoundRect(bounds, style, e, textBox, elementDrawable, true);
    }
  }

  buildElementScene(element: Element, darkMode: boolean, buildChildren: boolean, items: DiagramItem[]): Drawable {
    const elementDrawable = new DrawableContainer();

    this.sceneStyle = new SceneStyle(darkMode);
    const evaluated = this.executionContext?.getEvaluated(element) ?? new Map();
    this.sceneStyle.parseStyle(element.properties, evaluated);

    const links: DiagramItem[] = [];

    if (buildChildren) {
      this.buildItems(items, elementDrawable, links);
      for (const e of links) {
        if (e instanceof LinkItem) {
          const linkStyle = this.sceneStyle.defaultLineStyle.copy();
          linkStyle.parseStyle(e.properties, evaluated);
          let sr: Rect = { x: 0, y: 0, width: 5, height: 5 };
          let tr: Rect = { x: 0, y: 5, width: 5, height: 5 };

          const srr = e.source === null ? undefined : this.drawables.get(e.source)?.getSelectorBounds();
          if (e.source !== null && srr !== undefined) {
            this.addLink(e.source, e);
            sr = srr;
          }
          const trr = e.target === null ? undefined : this.drawables.get(e.target)?.getSelectorBounds();
          if (e.target !== null && trr !== undefined) {
            this.addLink(e.target, e);
            tr = trr;
          }
          const linkDr = new DrawableLine(sr, tr, linkStyle, { x: e.x, y: e.y });

          if (e.name.length > 0) {
            linkDr.addLabel(e.name, this.newImageProvider(e));
          }

          linkDr.item = e;
          this.drawables.set(e, linkDr);
          elementDrawable.insert(linkDr, 0);
        }
      }
    }
    return elementDrawable;
  }

  addLink(itm: DiagramItem, link: DiagramItem): void {
    const links = this.itemToLink.get(itm) ?? [];
    links.push(link);
    this.itemToLink.set(itm, links);
  }

  buildItems(items: DiagramItem[], elementDrawable: DrawableContainer, links: DiagramItem[]): void {
    for (const e of items) {
      if (e.kind === "Item") {
        this.buildItemDrawable(e, elementDrawable);
      } else if (e.kind === "Link") {
        links.push(e);
      }
    }
  }
}
