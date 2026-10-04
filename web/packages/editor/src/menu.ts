// Context menu in DOM, and the Style / Quick Style entries of SceneDrawView (createStylesMenu, createQuickStyleMenu).
import type { EditorSession } from "./session.ts";
import { MARKERS, OPTION_LABELS, quickStylesFor } from "./styles.ts";

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

export function styleEntries(session: EditorSession): Entry[] {
  const names = session.styleNames();
  const style: Entry = {
    label: "Style",
    sub: [...names.map((n): Entry => ({ label: n, run: () => session.applyStyle(n) })), ...(names.length > 0 ? ["-" as const] : []), { label: "Define new style", run: () => void session.defineStyle() }],
  };
  const one = session.selection.length === 1 ? session.selection[0]! : null;
  if (session.selection.length === 0) return [style, { label: "Global Styles", sub: [{ label: "Enable shadows", run: () => session.enableShadows() }] }];
  if (one === null) return [style];

  const pick = (prop: string) => (o: string): Entry => ({ label: OPTION_LABELS[o] ?? o, run: () => session.setQuickStyle(prop, o) });
  const quick = quickStylesFor(one.kind as "Item" | "Link").flatMap((q): Entry[] => {
    const sub = q.prop === "marker" ? Object.entries(MARKERS).map(([label, list]): Entry => ({ label, sub: list.map(pick("marker")) })) : q.options.map(pick(q.prop));
    return q.sep ? ["-", { label: q.label, sub }] : [{ label: q.label, sub }];
  });
  return [style, { label: "Quick Style", sub: quick }];
}
