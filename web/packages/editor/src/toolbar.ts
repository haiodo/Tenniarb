// Title bar controls of EditorViewController.createTitleBar: [-] 100% [+] [100%]  [help]  [share]  [+|-].
import { BOOK_SVG, MINUS_SVG, PLUS_SVG, SHARE_SVG } from "./icons.ts";

export interface Toolbar {
  setZoom(percent: number): void;
}

export interface ToolbarActions {
  zoomOut(): void;
  zoomIn(): void;
  resetZoom(): void;
  /** Absent: the button stays, disabled. */
  help?: () => void;
  /** Client coordinates under the share button. */
  share(x: number, y: number): void;
  /** Absent: the +/- pair is not shown (readonly). */
  add?: () => void;
  remove?: () => void;
}

export function mountToolbar(host: HTMLElement, a: ToolbarActions): Toolbar {
  const el = <T extends HTMLElement>(tag: string, cls: string, parent: HTMLElement): T => {
    const e = document.createElement(tag) as T;
    e.className = cls;
    parent.append(e);
    return e;
  };
  const btn = (parent: HTMLElement, title: string, content: string, click: (b: HTMLButtonElement) => void, cls = ""): HTMLButtonElement => {
    const b = el<HTMLButtonElement>("button", `tn-btn ${cls}`, parent);
    b.title = title;
    b.innerHTML = content;
    b.onclick = () => click(b);
    return b;
  };

  host.replaceChildren();
  const zoom = el("div", "tn-group", host);
  btn(zoom, "Zoom out", MINUS_SVG, a.zoomOut, "tn-tint");
  const label = el("span", "tn-zoom", zoom);
  label.textContent = "100%";
  btn(zoom, "Zoom in", PLUS_SVG, a.zoomIn, "tn-tint");
  btn(zoom, "Reset zoom", "100%", a.resetZoom, "tn-text");
  btn(host, "Help", BOOK_SVG, () => a.help?.()).disabled = a.help === undefined;
  btn(host, "Share", SHARE_SVG, (b) => {
    const r = b.getBoundingClientRect();
    a.share(r.left, r.bottom + 4);
  });
  if (a.add !== undefined && a.remove !== undefined) {
    const seg = el("div", "tn-seg", host);
    btn(seg, "New item", PLUS_SVG, a.add);
    btn(seg, "Delete selection", MINUS_SVG, a.remove);
  }
  return { setZoom: (percent) => void (label.textContent = `${percent}%`) };
}
