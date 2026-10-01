// Port of DrawableStyle & co from ElementScene.swift. `evaluations` are the ExecutionContext values keyed by token (JSValue in Swift).
import { ModelProperties, persistenceStyleName } from "@tenniarb/core";
import type { EvaluatedValues, TennNode } from "@tenniarb/core";
import type { Canvas2D } from "./canvas-types.ts";
import { colorBlack, colorWhite, getTextColorBasedOn, parseColor } from "./color.ts";
import type { Color } from "./color.ts";
import type { Point, Size } from "./geometry.ts";

// JSValue.toString() / toDouble()
export const jsToString = (v: unknown): string => String(v);
export const jsToDouble = (v: unknown): number => Number(v);

export function getString(child: TennNode | null, evaluations: EvaluatedValues): string | null {
  if (child === null) {
    return null;
  }
  // Check if we have override for value
  if (child.token !== null && evaluations.has(child.token)) {
    return jsToString(evaluations.get(child.token));
  }
  return child.getIdentText();
}

export function updateLineStyle(context: Canvas2D, style: DrawableStyle): void {
  switch (style.lineStyle) {
    case "dotted":
      context.setLineDash([1, 4]);
      context.lineDashOffset = 1;
      break;
    case "dashed":
      context.setLineDash([5]);
      context.lineDashOffset = 5;
      break;
  }
  context.lineWidth = style.lineWidth;
}

const floatRe = /^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$/;

export class DrawableStyle {
  color: Color = { r: 1, g: 1, b: 1, a: 1 };
  textColorValue: Color | null = null;

  get textColor(): Color {
    return this.textColorValue ?? getTextColorBasedOn(this.color);
  }

  borderColor: Color = colorBlack;
  fontSize = 18.0;
  width: number | null = null;
  height: number | null = null;
  darkMode = false;

  lineStyle: string | null = null;

  shadow: Size | null = null;
  shadowBlur = 4;
  shadowColor: Color | null = null;

  lineWidth = 0;

  /** default/not specified - item box, text - just a text box, etc. */
  display: string | null = null;

  /** A child layout specification. */
  layout: string | null = null;

  /** default, background (drawn first, not selected by default), hover (drawn last). */
  layer: string | null = null;

  /** A pattern to include all properties during display from named parent element. */
  inherit: string | null = null;
  inheritIndex = 0;

  constructor(darkMode: boolean) {
    this.darkMode = darkMode;
    this.reset();
  }

  newCopy(): DrawableStyle {
    return new DrawableStyle(this.darkMode);
  }

  copy(): DrawableStyle {
    const result = this.newCopy();
    result.color = this.color;
    result.textColorValue = this.textColorValue;
    result.borderColor = this.borderColor;
    result.fontSize = this.fontSize;
    result.width = this.width;
    result.height = this.height;
    result.display = this.display;
    result.layout = this.layout;
    result.lineStyle = this.lineStyle;

    result.shadow = this.shadow;
    result.shadowColor = this.shadowColor;
    result.shadowBlur = this.shadowBlur;
    result.lineWidth = this.lineWidth;
    result.layer = this.layer;

    return result;
  }

  reset(): void {
    // Reset to default values
    if (this.darkMode) {
      this.color = { r: 0.2, g: 0.2, b: 0.2, a: 1 };
      this.textColorValue = null;
      this.borderColor = colorWhite;
    } else {
      this.color = { r: 1, g: 1, b: 1, a: 1 };
      this.textColorValue = null;
      this.borderColor = colorBlack;
    }

    this.fontSize = 18;
    this.width = null;
    this.height = null;
    this.display = null;
    this.layout = null;
    this.lineStyle = null;

    this.shadow = null;
    this.shadowColor = null;
    this.shadowBlur = 5;
    this.lineWidth = 0.3;
  }

  getFloat(child: TennNode | null, evaluations: EvaluatedValues): number | null {
    if (child === null) {
      return null;
    }
    // Check if we have override for value
    if (child.token !== null && evaluations.has(child.token)) {
      return jsToDouble(evaluations.get(child.token));
    }
    const val = child.getIdentText();
    if (val !== null && floatRe.test(val)) {
      return Math.fround(Number(val));
    }
    return null;
  }

  // Swift tells Int/Double/Float apart from everything else (NSNumber vs String...).
  getComponentValue(value: unknown): number {
    return typeof value === "number" ? value : 255;
  }

  getColor(child: TennNode | null, evaluations: EvaluatedValues, alpha = 1.0): Color | null {
    if (child === null) {
      return null;
    }
    // Check if we have override for value
    if (child.token !== null && evaluations.has(child.token)) {
      const ev = evaluations.get(child.token);
      if (Array.isArray(ev)) {
        let r = 0;
        let g = 0;
        let b = 0;
        let al = alpha;
        if (ev.length >= 3) {
          r = this.getComponentValue(ev[0]);
          g = this.getComponentValue(ev[1]);
          b = this.getComponentValue(ev[2]);
        }
        if (ev.length === 4) {
          al = this.getComponentValue(ev[3]) / 255.0;
        }
        return { r: r / 255.0, g: g / 255.0, b: b / 255.0, a: al };
      }
      return parseColor(jsToString(ev).toLowerCase(), alpha);
    }
    const text = child.getIdentText();
    if (text !== null) {
      return parseColor(text.toLowerCase(), alpha);
    }
    return null;
  }

  parseStyleLine(cmdName: string, child: TennNode, evaluations: EvaluatedValues): void {
    switch (cmdName) {
      case persistenceStyleName("Color"): {
        const color = this.getColor(child.getChild(1), evaluations, 1);
        if (color !== null) {
          this.color = color;
        }
        break;
      }
      case persistenceStyleName("TextColor"): {
        const color = this.getColor(child.getChild(1), evaluations, 1);
        if (color !== null) {
          this.textColorValue = color;
        }
        break;
      }
      case persistenceStyleName("FontSize"): {
        const value = this.getFloat(child.getChild(1), evaluations);
        if (value !== null) {
          this.fontSize = value;
          if (this.fontSize > 37) {
            this.fontSize = 36;
          } else if (this.fontSize < 4) {
            this.fontSize = 4;
          }
        }
        break;
      }
      case persistenceStyleName("Display"): {
        const value = child.getIdent(1);
        if (value !== null) {
          this.display = value;
        }
        break;
      }
      case persistenceStyleName("Layer"): {
        const value = child.getIdent(1);
        if (value !== null) {
          this.layer = value;
        }
        break;
      }
      case persistenceStyleName("Layout"): {
        let layout = "";
        for (let i = 1; i < child.count; i++) {
          const value = child.getIdent(i);
          if (value !== null) {
            if (layout.length > 0) {
              layout += ", ";
            }
            layout += value;
          }
        }
        this.layout = layout;
        break;
      }
      case persistenceStyleName("Width"): {
        const value = this.getFloat(child.getChild(1), evaluations);
        if (value !== null) {
          this.width = value > 10000 ? 10000 : value;
        }
        break;
      }
      case persistenceStyleName("Height"): {
        const value = this.getFloat(child.getChild(1), evaluations);
        if (value !== null) {
          this.height = value > 10000 ? 10000 : value;
        }
        break;
      }
      case persistenceStyleName("BorderColor"): {
        const color = this.getColor(child.getChild(1), evaluations, 1);
        if (color !== null) {
          this.borderColor = color;
        }
        break;
      }
      case persistenceStyleName("LineStyle"): {
        const value = child.getIdent(1);
        if (value !== null) {
          this.lineStyle = value;
        }
        break;
      }
      case persistenceStyleName("LineWidth"): {
        const value = this.getFloat(child.getChild(1), evaluations);
        if (value !== null) {
          this.lineWidth = value;
        }
        break;
      }
      case persistenceStyleName("Inherit"): {
        const value = child.getIdent(0);
        if (value !== null) {
          this.inherit = value;

          const indexValue = child.getInt(1);
          if (indexValue !== null) {
            this.inheritIndex = indexValue;
          }
        }
        break;
      }
      case persistenceStyleName("Shadow"): {
        const xOffset = child.getFloat(1);
        const yOffset = child.getFloat(2);
        if (xOffset !== null && yOffset !== null) {
          this.shadow = { width: xOffset, height: yOffset };
        }
        const blur = child.getFloat(3);
        if (blur !== null) {
          this.shadowBlur = blur;
        }
        const clr = this.getColor(child.getChild(4), evaluations);
        if (clr !== null) {
          this.shadowColor = clr;
        }
        break;
      }
    }
  }

  parseStyle(source: ModelProperties | TennNode, evaluations: EvaluatedValues): void {
    const node = source instanceof ModelProperties ? source.node : source;
    for (const child of node.children ?? []) {
      const cmdName = child.getIdent(0);
      if (child.kind === "Command" && child.count > 0 && cmdName !== null) {
        this.parseStyleLine(cmdName, child, evaluations);
      }
    }
  }
}

export class DrawableLineStyle extends DrawableStyle {
  override newCopy(): DrawableStyle {
    return new DrawableLineStyle(this.darkMode);
  }

  override copy(): DrawableLineStyle {
    return super.copy() as DrawableLineStyle;
  }

  override reset(): void {
    super.reset();

    if (this.darkMode) {
      this.color = { r: 1, g: 1, b: 1, a: 1 };
      this.textColorValue = null;
      this.borderColor = colorWhite;
    } else {
      this.color = { r: 0.2, g: 0.2, b: 0.2, a: 1 };
      this.textColorValue = null;
      this.borderColor = colorBlack;
    }

    this.lineStyle = null;

    this.lineWidth = 1;
  }
}

export class DrawableItemStyle extends DrawableStyle {
  title: string | null = null;
  marker: string | null = null;

  cornerRadius: number[] = [];

  lineSpacing = 0;

  override newCopy(): DrawableStyle {
    return new DrawableItemStyle(this.darkMode);
  }

  override copy(): DrawableItemStyle {
    const result = super.copy() as DrawableItemStyle;
    result.title = this.title;
    result.marker = this.marker;
    result.cornerRadius.push(...this.cornerRadius);
    result.lineSpacing = this.lineSpacing;

    return result;
  }

  override parseStyleLine(cmdName: string, child: TennNode, evaluations: EvaluatedValues): void {
    switch (cmdName) {
      case persistenceStyleName("Title"): {
        this.title = child.getIdent(1);
        const t = child.getChild(1)?.token;
        if (t != null && evaluations.has(t)) {
          this.title = jsToString(evaluations.get(t));
        }
        break;
      }
      case persistenceStyleName("Marker"): {
        this.marker = child.getIdent(1);
        const t = child.getChild(1)?.token;
        if (t != null && evaluations.has(t)) {
          this.marker = jsToString(evaluations.get(t));
        }
        break;
      }
      case persistenceStyleName("CornerRadius"):
        this.cornerRadius = [];
        for (let ind = 1; ind < child.count; ind++) {
          const val = child.getFloat(ind);
          if (val !== null) {
            this.cornerRadius.push(Math.min(val, 15));
          }
        }
        break;
      case persistenceStyleName("LineSpacing"): {
        const val = child.getFloat(1);
        if (val !== null) {
          this.lineSpacing = val;
        }
        break;
      }
      default:
        super.parseStyleLine(cmdName, child, evaluations);
    }
  }
}

export class SceneStyle extends DrawableStyle {
  gridSpan: Point = { x: 5, y: 5 };

  defaultItemStyle: DrawableItemStyle;
  defaultLineStyle: DrawableLineStyle;

  constructor(darkMode: boolean) {
    super(darkMode);
    this.defaultItemStyle = new DrawableItemStyle(darkMode);
    this.defaultLineStyle = new DrawableLineStyle(darkMode);
    this.defaultLineStyle.fontSize = 12;
  }

  override parseStyleLine(cmdName: string, child: TennNode, evaluations: EvaluatedValues): void {
    switch (cmdName) {
      case persistenceStyleName("Styles"): {
        // Default styles for entire diagram
        const childBlock = child.getChild(1);
        if (childBlock !== null && childBlock.kind === "BlockExpr" && childBlock.children !== null) {
          for (const styleChild of childBlock.children) {
            const styleName = styleChild.getIdent(0);
            if (styleChild.kind !== "Command" || styleChild.count === 0 || styleName === null) {
              continue;
            }
            const styleProps = styleChild.getChild(1);
            if (styleProps === null || styleProps.kind !== "BlockExpr" || styleProps.children === null) {
              continue;
            }
            if (styleName === "item") {
              this.defaultItemStyle.reset();
              this.defaultItemStyle.parseStyle(new ModelProperties(styleProps.children), evaluations);
            } else if (styleName === "line") {
              this.defaultLineStyle.reset();
              this.defaultLineStyle.parseStyle(new ModelProperties(styleProps.children), evaluations);
            }
          }
        }
        break;
      }
      case persistenceStyleName("Grid"): {
        const x = child.getFloat(1);
        const y = child.getFloat(2);
        if (x !== null && y !== null) {
          this.gridSpan = { x, y };
        }
        break;
      }
      default:
        super.parseStyleLine(cmdName, child, evaluations);
    }
  }
}
