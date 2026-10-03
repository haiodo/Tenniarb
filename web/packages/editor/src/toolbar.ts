// Style toolbar over the canvas: port of the Style / Quick Style menus of SceneDrawView.
import type { EditorSession } from "./session.ts";
import { quickStylesFor } from "./styles.ts";

export interface Toolbar {
  sync(): void;
  destroy(): void;
}

const NEW_STYLE = "+ New style";

export function mountToolbar(host: HTMLElement, session: EditorSession, onDone: () => void): Toolbar {
  const bar = document.createElement("div");
  bar.className = "tn-toolbar";
  bar.style.cssText = "position:absolute;top:6px;left:6px;right:6px;display:flex;flex-wrap:wrap;gap:4px;pointer-events:none";
  // Keys must not reach the canvas shortcuts (Backspace deletes the selection).
  bar.addEventListener("keydown", (ev) => ev.stopPropagation());
  host.append(bar);

  // The options only change with the selected kind and the styles list; rebuilding on every redraw would close an open select.
  let key = "";
  function select(label: string, name: string, options: string[], pick: (v: string) => void): HTMLSelectElement {
    const sel = document.createElement("select");
    sel.style.cssText = "pointer-events:auto;font:12px system-ui,sans-serif";
    sel.dataset["tn"] = name;
    sel.add(new Option(label, ""));
    for (const o of options) sel.add(new Option(o, o));
    sel.addEventListener("change", () => {
      const v = sel.value;
      sel.value = "";
      if (v !== "") pick(v);
      onDone();
    });
    return sel;
  }

  return {
    sync() {
      const one = session.selection.length === 1 ? session.selection[0]! : null;
      const names = session.styleNames();
      const next = `${one?.kind ?? session.selection.length}|${names.join(",")}`;
      if (next === key) return;
      key = next;
      const parts: HTMLElement[] = [select("Style", "style", [...names, NEW_STYLE], (v) => (v === NEW_STYLE ? session.defineStyle() : session.applyStyle(v)))];
      if (session.selection.length === 0) {
        const btn = document.createElement("button");
        btn.textContent = "Enable shadows";
        btn.style.cssText = "pointer-events:auto;font:12px system-ui,sans-serif";
        btn.dataset["tn"] = "shadows";
        btn.addEventListener("click", () => {
          session.enableShadows();
          onDone();
        });
        parts.push(btn);
      }
      if (one !== null) for (const q of quickStylesFor(one.kind as "Item" | "Link")) parts.push(select(q.label, q.prop, q.options, (v) => session.setQuickStyle(q.prop, v)));
      bar.replaceChildren(...parts);
    },
    destroy: () => bar.remove(),
  };
}
