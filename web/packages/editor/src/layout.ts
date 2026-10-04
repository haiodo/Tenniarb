// Window layout of the Swift app (ViewController.loadView): outline on the left, on the right a title bar (document title, toolbar),
// the diagram and the properties text below. Sizes and limits are the Swift ones; colors follow the system appearance.
import { COMPONENT_ICON, GROUP_ICON } from "./icons.ts";

const CSS = `
.tn-split, .tn-menu, .tn-search, .tn-pop { --bg: #ececec; --fg: rgba(0,0,0,.85); --fg2: rgba(0,0,0,.5); --sep: #d1d1d1; --field: #fff; --field-line: rgba(0,0,0,.18); --capsule: rgba(0,0,0,.06); --capsule-line: rgba(0,0,0,.1);
  --hover: rgba(0,0,0,.06); --sel-idle: #cbcccc; --accent: #0a64d6; --menu: #f6f6f6; --menu-line: rgba(0,0,0,.18); }
@supports (color: -apple-system-control-accent) { .tn-split, .tn-menu, .tn-search, .tn-pop { --accent: -apple-system-control-accent; } }
@media (prefers-color-scheme: dark) {
  .tn-split, .tn-menu, .tn-search, .tn-pop { --bg: #2a2a2a; --fg: rgba(255,255,255,.85); --fg2: rgba(255,255,255,.5); --sep: #1b1b1b; --field: rgba(255,255,255,.08); --field-line: rgba(255,255,255,.15);
    --capsule: rgba(255,255,255,.12); --capsule-line: rgba(255,255,255,.14); --hover: rgba(255,255,255,.08); --sel-idle: #464646; --menu: #3a3a3a; --menu-line: rgba(255,255,255,.18); }
}
.tn-split { display: flex; height: 100%; min-height: 0; min-width: 0; background: var(--bg); color: var(--fg); color-scheme: light dark;
  font: 13px -apple-system, system-ui, sans-serif; -webkit-font-smoothing: antialiased; user-select: none; -webkit-user-select: none; }
.tn-split ::selection { background: color-mix(in srgb, var(--accent) 35%, transparent); }
.tn-right { flex: 1; min-width: 0; display: flex; flex-direction: column; background: var(--bg); }
/* Translucent window (macOS, under-window vibrancy as Swift's NSVisualEffectView): the title row and the properties show it always,
   the canvas with the transparent setting. PropertiesPanelController sets white 0.8, but the running Swift app shows the vibrancy. */
html.tn-glass, html.tn-glass body, .tn-glass .tn-split, .tn-glass .tn-right, .tn-glass .tn-titlebar, .tn-glass #props { background: transparent; }
#outline { width: 217px; flex: none; display: flex; flex-direction: column; overflow: hidden; }
#editor { flex: 1; min-height: 0; position: relative; }
#props { height: 127px; flex: none; overflow: hidden; user-select: text; -webkit-user-select: text; }
.tn-gutter { flex: none; position: relative; background: var(--sep); }
.tn-gutter::after { content: ""; position: absolute; z-index: 5; }
.tn-gutter.v { width: 1px; } .tn-gutter.v::after { inset: 0 -3px; cursor: col-resize; }
.tn-gutter.h { height: 1px; } .tn-gutter.h::after { inset: -3px 0; cursor: row-resize; }

.outline-header { height: 52px; flex: none; display: flex; align-items: center; justify-content: flex-end; padding: 0 10px; box-sizing: border-box; }
.tn-titlebar { height: 52px; flex: none; display: flex; align-items: center; gap: 16px; padding: 0 10px 0 15px; box-sizing: border-box; }
.tn-title { flex: 1; min-width: 0; font-size: 15px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.tn-bar { display: flex; align-items: center; gap: 16px; flex: none; }
.tn-group { display: flex; align-items: center; gap: 2px; }
.tn-zoom { width: 40px; text-align: center; font-size: 11px; }
.tn-btn, .tn-seg button { height: 28px; min-width: 32px; padding: 0 9px; box-sizing: border-box; display: inline-flex; align-items: center; justify-content: center; font: 11px -apple-system, system-ui, sans-serif;
  color: var(--fg); background: var(--capsule); border: 0; border-radius: 14px; box-shadow: inset 0 0 0 .5px var(--capsule-line), 0 1px 2px rgba(0,0,0,.08); }
.tn-btn:hover, .tn-seg button:hover { background: color-mix(in srgb, var(--capsule), var(--fg) 6%); }
.tn-btn:active, .tn-seg button:active { background: color-mix(in srgb, var(--capsule), var(--fg) 14%); }
.tn-btn:disabled { opacity: .4; }
.tn-btn svg, .tn-seg svg { width: 16px; height: 16px; flex: none; }
.tn-tint { color: #f0922b; }
.tn-seg { display: flex; flex: none; gap: 1px; }
.tn-seg button { min-width: 30px; border-radius: 0; }
.tn-seg button:first-child { border-radius: 14px 0 0 14px; } .tn-seg button:last-child { border-radius: 0 14px 14px 0; }
/* Quick style panel over the selected item (Swift showPopup); the delay is Swift's 0.2 s, so a double-click does not flash it. */
.tn-pop { position: absolute; z-index: 5; filter: drop-shadow(0 2px 6px rgba(0,0,0,.3)); animation: tn-pop-in .1s .2s backwards; }
@keyframes tn-pop-in { from { opacity: 0; } }
.tn-pop button { width: 36px; min-width: 0; padding: 0; font-size: 14px; }
.tn-pop button::after { content: "\\25BE"; margin-left: 2px; font-size: 8px; opacity: .6; }
.outline-tree { overflow: auto; flex: 1; outline: none; padding-bottom: 8px; }
.outline-rename { font: 14px -apple-system, system-ui, sans-serif; min-width: 0; flex: 1; user-select: text; -webkit-user-select: text; }
.row { display: flex; align-items: center; height: 27px; box-sizing: border-box; padding: 0 4px 0 calc(4px + var(--depth, 0) * 10px); cursor: default; white-space: nowrap; font-size: 14px; }
.row:hover { background: var(--hover); }
.row.sel { background: var(--sel-idle); }
.row.drop-in { outline: 1px solid var(--accent); outline-offset: -1px; }
.row.drop-before { box-shadow: inset 0 2px 0 var(--accent); }
.row.drop-after { box-shadow: inset 0 -2px 0 var(--accent); }
.outline-tree:focus-within .row.sel { background: var(--accent); color: #fff; }
.row .arrow { order: 0; width: 14px; flex: none; text-align: center; font-size: 8px; color: var(--fg2); cursor: pointer; }
.outline-tree:focus-within .row.sel .arrow { color: #fff; }
.outline-tree .row::before { order: 1; content: ""; width: 20px; height: 20px; margin-right: 4px; flex: none; background: url(${COMPONENT_ICON}) left center / 14px no-repeat; }
.outline-tree .row.group::before { background-image: url(${GROUP_ICON}); background-size: 15px; }
.row .name { order: 2; overflow: hidden; text-overflow: ellipsis; }
.hidden { display: none; }

.tn-menu { position: absolute; display: none; min-width: 160px; padding: 4px; background: var(--menu); color: var(--fg); border: 1px solid var(--menu-line);
  border-radius: 8px; box-shadow: 0 6px 24px rgba(0,0,0,.25); font: 13px -apple-system, system-ui, sans-serif; user-select: none; -webkit-user-select: none; z-index: 1000; }
.tn-menu.root, .tn-menu.open { display: block; }
.tn-menu .tn-menu { left: 100%; top: -5px; }
.tn-mi { position: relative; display: flex; align-items: center; gap: 8px; padding: 3px 22px 3px 10px; border-radius: 4px; white-space: nowrap; cursor: default; }
.tn-mi:hover { background: var(--accent); color: #fff; }
.tn-mi.has-sub::after { content: "\\25B8"; position: absolute; right: 7px; }
.tn-ic { order: -1; display: flex; width: 16px; height: 16px; }
.tn-search { position: fixed; width: 400px; height: 180px; box-sizing: border-box; padding: 8px 0; display: flex; flex-direction: column; gap: 8px; background: var(--menu); color: var(--fg); border: 1px solid var(--menu-line);
  border-radius: 8px; box-shadow: 0 6px 24px rgba(0,0,0,.25); font: 13px -apple-system, system-ui, sans-serif; z-index: 1000; }
.tn-search.tn-op { width: 300px; height: 50px; padding: 12px 0; }
.tn-search input.bad { background: #f00; color: #fff; }
.tn-search input { flex: none; height: 30px; margin: 0 8px; padding: 0 8px; box-sizing: border-box; font: inherit; color: inherit; background: var(--field); border: 1px solid var(--field-line); border-radius: 4px; outline: none; }
.tn-search > div { flex: 1; min-height: 0; overflow: auto; }
.tn-sr { height: 20px; line-height: 20px; padding: 0 10px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; user-select: none; -webkit-user-select: none; }
.tn-sr.sel { background: var(--accent); color: #fff; }
.tn-sep { height: 1px; margin: 4px 6px; background: var(--menu-line); }
`;

export interface Layout {
  outline: HTMLElement;
  editor: HTMLElement;
  props: HTMLElement;
  /** Toolbar container in the title bar; pass it to `mount` as `toolbar`. */
  bar: HTMLElement;
  setTitle(text: string): void;
}

/** Drag on `gutter` reports the pointer; the callback sizes the pane. */
function drag(gutter: HTMLElement, size: (ev: PointerEvent) => void): void {
  gutter.onpointerdown = (ev) => {
    gutter.setPointerCapture(ev.pointerId);
    ev.preventDefault();
  };
  gutter.onpointermove = (ev) => gutter.hasPointerCapture(ev.pointerId) && size(ev);
}

const clamp = (v: number, min: number, max: number): number => Math.max(min, Math.min(max, v));

/**
 * Fills `host` (give it a size) with the panes; pass them to `mount` as editor container, `properties`, `outline` and `toolbar`.
 * `glass`: the window is a transparent macOS one with a vibrancy effect behind the sidebar; the title rows drag the window.
 */
export function mountLayout(host: HTMLElement, opts: { glass?: boolean } = {}): Layout {
  if (document.getElementById("tn-layout-css") === null) {
    const style = document.createElement("style");
    style.id = "tn-layout-css";
    style.textContent = CSS;
    document.head.append(style);
  }
  const div = (id: string | null, cls = ""): HTMLElement => {
    const el = document.createElement("div");
    if (id !== null) el.id = id;
    el.className = cls;
    return el;
  };
  const [outline, editor, props, right] = [div("outline"), div("editor"), div("props"), div(null, "tn-right")];
  const [vGutter, hGutter] = [div(null, "tn-gutter v"), div(null, "tn-gutter h")];
  const [titlebar, title, bar] = [div(null, "tn-titlebar"), div(null, "tn-title"), div(null, "tn-bar")];
  for (const el of [titlebar, title]) el.setAttribute("data-tauri-drag-region", "");
  titlebar.append(title, bar);
  document.documentElement.classList.toggle("tn-glass", opts.glass === true);
  right.append(titlebar, editor, hGutter, props);
  host.classList.add("tn-split");
  host.replaceChildren(outline, vGutter, right);

  drag(vGutter, (ev) => (outline.style.width = `${clamp(ev.clientX - host.getBoundingClientRect().left, 150, 400)}px`));
  drag(hGutter, (ev) => {
    const r = right.getBoundingClientRect();
    props.style.height = `${clamp(r.bottom - ev.clientY, 50, r.height - 200)}px`;
  });
  return { outline, editor, props, bar, setTitle: (text) => void (title.textContent = text) };
}
