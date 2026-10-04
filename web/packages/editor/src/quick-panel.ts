// Quick style panel above the single selected item (Swift SceneDrawView.showPopup): a segment per property, each opens its menu.
import type { DiagramItem } from "@tenniarb/core";
import { quickEntries, showMenu } from "./menu.ts";
import type { EditorSession } from "./session.ts";
import { quickStylesFor } from "./styles.ts";

// Swift segment order; links get only the last three.
const SEGMENTS: { prop: string; label: string; item?: true }[] = [
  { prop: "marker", label: "✑", item: true },
  { prop: "font-size", label: "Ƭ", item: true },
  { prop: "display", label: "❑" },
  { prop: "color", label: "🔴", item: true },
  { prop: "line-style", label: "⊞" },
  { prop: "line-width", label: "〰" },
];

/** Call on every redraw: `item` null hides the panel; the panel is moved by the view (screen = x + k * sceneX, y - k * sceneY). */
export function mountQuickPanel(box: HTMLElement, session: EditorSession): (item: DiagramItem | null, view: { x: number; y: number; k: number }) => void {
  let shown: { item: DiagramItem; el: HTMLElement } | null = null;
  return (item, view) => {
    const d = item === null ? undefined : session.scene.drawables.get(item);
    if (item === null || d === undefined) {
      shown?.el.remove();
      shown = null;
      return;
    }
    if (shown?.item !== item) {
      shown?.el.remove();
      const el = document.createElement("div");
      el.className = "tn-pop tn-seg";
      el.onpointerdown = (ev) => ev.preventDefault(); // keeps the canvas focus
      const styles = quickStylesFor(item.kind as "Item" | "Link");
      for (const s of SEGMENTS) {
        const q = styles.find((x) => x.prop === s.prop);
        if (q === undefined || (s.item && item.kind !== "Item")) continue;
        const b = document.createElement("button");
        b.textContent = s.label;
        b.title = q.label;
        b.onclick = () => {
          const r = b.getBoundingClientRect();
          showMenu(r.left, r.bottom + 2, quickEntries(session, q), () => box.focus());
        };
        el.append(b);
      }
      box.append(el);
      shown = { item, el };
    }
    // Swift: 15 right of the item's left edge, 10 above its top; here above the selector box (5 scene units out) and its glow.
    const b = d.getSelectorBounds();
    const { el } = shown;
    el.style.left = `${Math.max(0, Math.min(view.x + b.x * view.k + 15, box.clientWidth - el.offsetWidth))}px`;
    el.style.top = `${Math.max(0, view.y - (b.y + b.height + 5) * view.k - 10 - el.offsetHeight)}px`;
  };
}
