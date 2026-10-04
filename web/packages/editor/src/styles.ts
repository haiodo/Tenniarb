// Quick-style menus of SceneDrawView (createQuickStyleMenu): property, label and the values it offers.
import { newIdent, newStrNode } from "@tenniarb/core";
import type { TennNode } from "@tenniarb/core";

export interface QuickStyle {
  prop: string;
  label: string;
  options: string[];
  /** Absent: items and links. */
  only?: "Item";
  /** Separator before the entry, as in the Swift menu. */
  sep?: true;
}

// Submenus of the Swift marker menu.
export const MARKERS: Record<string, string[]> = {
  "😀 Emoji": ["😀", "😛", "😱", "😵", "😷", "🐶", "🐱", "🐭", "🐰", "🦊", "🌻", "🌧", "🌎", "🔥", "❄️", "💦", "☂️"],
  "🔢 Numbers": ["0️⃣", "1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣", "9️⃣", "🔟"],
  "🖥 Objects": ["⌚️", "🖥", "🖨", "⌛️", "⏰", "⚒", "🧲", "💣", "🔒", "✂️", "🧸", "🎁"],
  "🔠 Symbols": ["🆗", "🆖", "#️⃣", "🔤", "ℹ️", "🚻", "🔃", "➕", "➖", "➗", "✖️", "♾", "💲", "✔️", "♠️", "♣️", "♥️", "♦️"],
};

/** Menu captions that differ from the option value (Swift prefixes colors and display / line styles with a glyph). */
export const OPTION_LABELS: Record<string, string> = {
  red: "🔴red", green: "🟢green", blue: "🔵blue", yellow: "🟡yellow", orange: "🟠orange", brown: "🟤brown", black: "⚫️black", purple: "🟣purple",
  rect: "■ rect", "no-fill": "□ no-fill", circle: "● circle", stack: "❐ stack", text: "≣ text",
  solid: "– solid", arrow: "→ arrow", arrows: "↔︎ arrows", "arrow-source": "← arrow-source", dashed: "⤍ dashed", dotted: "⤑ dotted",
};

const COLORS = ["red", "green", "blue", "yellow", "orange", "brown", "black", "purple"];

export const ITEM_DISPLAY = ["rect", "no-fill", "circle", "stack", "text"];
export const LINK_DISPLAY = ["solid", "arrow", "arrows", "arrow-source"];

export const QUICK_STYLES: QuickStyle[] = [
  { prop: "color", label: "Color", options: COLORS },
  { prop: "border-color", label: "Border color", options: COLORS, only: "Item" },
  { prop: "text-color", label: "Text Color", options: COLORS },
  { prop: "font-size", label: "Font", sep: true, options: ["8", "10", "12", "14", "16", "18", "20", "22", "26", "32", "36"] },
  { prop: "marker", label: "Markers", only: "Item", options: Object.values(MARKERS).flat() },
  { prop: "display", label: "Display", sep: true, options: [] }, // filled per kind by optionsFor
  { prop: "layout", label: "Layout", options: ["left", "right", "center", "top", "top right", "bottom", "bottom right"] },
  { prop: "line-style", label: "Line style", sep: true, options: ["solid", "dashed", "dotted"] },
  { prop: "line-width", label: "Line width", options: ["0.3", "0.5", "1", "1.5", "2", "5"] },
  { prop: "width", label: "Width", options: ["10", "50", "100", "300", "500"], only: "Item", sep: true },
  { prop: "height", label: "Height", options: ["10", "50", "100", "300", "500"], only: "Item" },
  { prop: "layer", label: "Layer", sep: true, options: ["background", "hover"] },
  { prop: "corner-radius", label: "Corner Radius", options: ["0", "5", "15"], only: "Item" },
  { prop: "shadow", label: "Shadow", options: ["5 -5", "5 5"] },
];

export function quickStylesFor(kind: "Item" | "Link"): QuickStyle[] {
  return QUICK_STYLES.filter((q) => q.only === undefined || q.only === kind).map((q) =>
    q.prop === "display" ? { ...q, options: kind === "Item" ? ITEM_DISPLAY : LINK_DISPLAY } : q,
  );
}

/** Marker is a string, everything else idents split on spaces (Swift markerMenuAction / layoutMenuAction / commandAction). */
export function optionNodes(prop: string, option: string): TennNode[] {
  return prop === "marker" ? [newStrNode(option)] : option.split(" ").map((v) => newIdent(v));
}
