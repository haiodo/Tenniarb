// Context menu in DOM, and the entries of SceneDrawView.menu(for:) (createStylesMenu, createQuickStyleMenu, createAlighMenu, createOrderMenu, createSelection).
import type { Point } from "@tenniarb/render";
import { MINUS_SVG, PLUS_SVG } from "./icons.ts";
import { copyItemHtml } from "./export.ts";
import { hitTest } from "./selection.ts";
import type { EditorSession } from "./session.ts";
import { MARKERS, OPTION_LABELS, quickStylesFor } from "./styles.ts";
import type { QuickStyle } from "./styles.ts";

/** `icon`: inline SVG markup. */
export type Entry = "-" | { label: string; icon?: string; run?: () => void; sub?: Entry[] };

/** Popup at client (x, y). Closes on a pick, a press outside, Esc, resize and blur. */
export function showMenu(x: number, y: number, entries: Entry[], onClose: () => void): void {
  const ac = new AbortController();
  const close = (): void => {
    ac.abort();
    root.remove();
    onClose();
  };

  function build(list: Entry[]): HTMLElement {
    const menu = document.createElement("div");
    menu.className = "tn-menu";
    for (const e of list) {
      const item = document.createElement("div");
      if (e === "-") {
        item.className = "tn-sep";
      } else {
        item.className = "tn-mi";
        item.textContent = e.label;
        if (e.icon !== undefined) {
          const ic = document.createElement("span");
          ic.className = "tn-ic";
          ic.innerHTML = e.icon;
          item.append(ic); // CSS order puts it before the label, the text node stays firstChild
        }
        if (e.sub !== undefined) item.append(sub(item, build(e.sub)));
        else
          item.onclick = () => {
            close();
            e.run?.();
          };
      }
      menu.append(item);
    }
    return menu;
  }

  // Opens to the right of the item, flips left or up when it would leave the window.
  function sub(item: HTMLElement, menu: HTMLElement): HTMLElement {
    item.classList.add("has-sub");
    item.onmouseenter = () => {
      menu.style.cssText = "";
      menu.classList.add("open");
      const r = menu.getBoundingClientRect();
      if (r.right > innerWidth) menu.style.cssText += "left:auto;right:100%;";
      if (r.bottom > innerHeight) menu.style.cssText += `top:${innerHeight - r.bottom - 6}px;`;
    };
    item.onmouseleave = () => menu.classList.remove("open");
    return menu;
  }

  const root = build(entries);
  root.classList.add("root");
  document.body.append(root);
  root.style.left = `${Math.max(Math.min(x, innerWidth - root.offsetWidth - 4), 4)}px`;
  root.style.top = `${Math.max(Math.min(y, innerHeight - root.offsetHeight - 4), 4)}px`;
  const opt = { signal: ac.signal };
  document.addEventListener("pointerdown", (ev) => !root.contains(ev.target as Node) && close(), { ...opt, capture: true });
  document.addEventListener("keydown", (ev) => ev.key === "Escape" && close(), opt);
  addEventListener("resize", close, opt);
  addEventListener("blur", close, opt);
}

/** Options of one quick style; markers are grouped as in the Swift marker menu. */
export function quickEntries(session: EditorSession, q: QuickStyle): Entry[] {
  const pick = (o: string): Entry => ({ label: OPTION_LABELS[o] ?? o, run: () => session.setQuickStyle(q.prop, o) });
  return q.prop === "marker" ? Object.entries(MARKERS).map(([label, list]): Entry => ({ label, sub: list.map(pick) })) : q.options.map(pick);
}

export function styleEntries(session: EditorSession): Entry[] {
  const names = session.styleNames();
  const style: Entry = {
    label: "Style",
    sub: [...names.map((n): Entry => ({ label: n, run: () => session.applyStyle(n) })), ...(names.length > 0 ? ["-" as const] : []), { label: "Define new style", run: () => void session.defineStyle() }],
  };
  const one = session.selection.length === 1 ? session.selection[0]! : null;
  if (session.selection.length === 0) return [style, { label: "Global Styles", sub: [{ label: "Enable shadows", run: () => session.enableShadows() }] }];
  if (one === null) return [style];

  const quick = quickStylesFor(one.kind as "Item" | "Link").flatMap((q): Entry[] => {
    const sub = quickEntries(session, q);
    return q.sep ? ["-", { label: q.label, sub }] : [{ label: q.label, sub }];
  });
  return [style, { label: "Quick Style", sub: quick }];
}

/** Swift menu(for:): `p` is the right-click point (scene space), `layout` runs "Test layout", `attach` "Attach image". Call after `session.pick(p)`. */
export function canvasEntries(session: EditorSession, p: Point, layout: () => void, attach: () => void): Entry[] {
  const sel = session.selection;
  const under = hitTest(session.scene, p, true);
  // Overlapping items to pick from; not offered when the plain hit test finds the same single one.
  const select: Entry[] = under.length === hitTest(session.scene, p).length && under.length <= 1 ? [] : ["-", { label: "Select", sub: under.map((i): Entry => ({ label: i.name || "Link", run: () => session.select([i]) })) }];
  const add: Entry = { label: "New item", icon: PLUS_SVG, run: () => session.addTopItem() };
  if (sel.length === 0) return [add, { label: "Test layout", run: layout }, "-", ...styleEntries(session), ...select];

  const align = (label: string, edge: Parameters<EditorSession["align"]>[0]): Entry => ({ label, run: () => session.align(edge) });
  return [
    add,
    "-",
    { label: "New linked item", run: () => session.addNewItem() },
    { label: "Linked styled item", run: () => session.addNewItem(true) },
    "-",
    ...styleEntries(session),
    ...(sel.length > 1 ? ["-" as const, { label: "Align", sub: [align("Leading Edges", "leading"), align("Trailing Edges", "trailing"), align("Top Edges", "top"), align("Bottom Edges", "bottom")] }] : []),
    "-",
    { label: "Duplicate", run: () => session.duplicate() },
    ...(sel.length === 1
      ? [
          "-" as const,
          { label: "Attach image", run: attach },
          { label: "Order", sub: [{ label: "Move Forward", run: () => session.order(true) }, { label: "Move Backward", run: () => session.order(false) }] },
          "-" as const,
          { label: "Export text as html", run: () => void copyItemHtml(session.scene, sel[0]!).catch((e) => console.warn("tenniarb: export failed", e)) },
        ]
      : []),
    ...select,
    "-",
    { label: "Delete", icon: MINUS_SVG, run: () => session.deleteSelection() },
  ];
}
