// Embeddable mind map: the editor canvas over the text of one element body, without the expression engine.
import { Element, ElementModel } from "@tenniarb/core";
import type { DiagramItem } from "@tenniarb/core";
import { DEFAULT_FONT_FAMILY, DrawableLine, SvgContext, drawScene, layoutScene, setFontFamily } from "@tenniarb/render";
import type { Canvas2D } from "@tenniarb/render";
import { attachCanvas } from "@tenniarb/editor/canvas";
import { loadFonts } from "@tenniarb/embed";
import type { MindmapHandle, MindmapNodeView, MindmapOptions, NodeViewOptions, Peer, PMNode, PMView, RenderResult, Theme } from "../mindmap.d.ts";
import { MindmapDoc } from "./doc.ts";
import { keys, parseBlock, reconcile } from "./text.ts";

export type * from "../mindmap.d.ts";

// Theme by attribute, not prefers-color-scheme: the host decides. Menus open inside the map, so they inherit it.
const CSS = `
.tn-mm { --fg: rgba(0,0,0,.85); --accent: #0a64d6; --menu: #f6f6f6; --menu-line: rgba(0,0,0,.18); --capsule-line: rgba(0,0,0,.1); color-scheme: light; }
.tn-mm[data-theme=dark] { --fg: rgba(255,255,255,.85); --menu: #3a3a3a; --menu-line: rgba(255,255,255,.18); --capsule-line: rgba(255,255,255,.14); color-scheme: dark; }
.tn-mm .tn-pop { position: absolute; z-index: 5; display: flex; gap: 1px; filter: drop-shadow(0 2px 6px rgba(0,0,0,.3)); animation: tn-mm-in .1s .2s backwards; }
@keyframes tn-mm-in { from { opacity: 0; } }
.tn-mm .tn-pop button { width: 36px; height: 28px; padding: 0; border: 0; border-radius: 0; font-size: 14px; color: var(--fg); background: var(--menu); box-shadow: inset 0 0 0 .5px var(--capsule-line); }
.tn-mm .tn-pop button:first-child { border-radius: 14px 0 0 14px; } .tn-mm .tn-pop button:last-child { border-radius: 0 14px 14px 0; }
.tn-mm .tn-pop button:hover { background: color-mix(in srgb, var(--menu), var(--fg) 8%); }
.tn-mm .tn-pop button::after { content: "\\25BE"; margin-left: 2px; font-size: 8px; opacity: .6; }
.tn-mm .tn-menu { position: absolute; display: none; min-width: 160px; padding: 4px; background: var(--menu); color: var(--fg); border: 1px solid var(--menu-line);
  border-radius: 8px; box-shadow: 0 6px 24px rgba(0,0,0,.25); font: 13px -apple-system, system-ui, sans-serif; user-select: none; -webkit-user-select: none; z-index: 1000; }
.tn-mm .tn-menu.root, .tn-mm .tn-menu.open { display: block; }
.tn-mm .tn-menu.root { position: fixed; }
.tn-mm .tn-menu .tn-menu { left: 100%; top: -5px; }
.tn-mm .tn-mi { position: relative; display: flex; align-items: center; gap: 8px; padding: 3px 22px 3px 10px; border-radius: 4px; white-space: nowrap; cursor: default; }
.tn-mm .tn-mi:hover { background: var(--accent); color: #fff; }
.tn-mm .tn-mi.has-sub::after { content: "\\25B8"; position: absolute; right: 7px; }
.tn-mm .tn-sep { height: 1px; margin: 4px 6px; background: var(--menu-line); }
.tn-mm .tn-err { position: absolute; top: 6px; right: 6px; min-width: 16px; padding: 0 4px; box-sizing: border-box; border-radius: 8px; background: #d93025; color: #fff;
  font: 600 10px/16px -apple-system, system-ui, sans-serif; text-align: center; opacity: .75; cursor: default; user-select: none; -webkit-user-select: none; }
`;

const SYSTEM_FONT = '-apple-system, BlinkMacSystemFont, "Segoe UI", system-ui';
// Shared by every map on the page: the measuring and drawing font is global in render.
let family = SYSTEM_FONT;
let renders = 0;

export function create(el: HTMLElement, opts: MindmapOptions): MindmapHandle {
  if (document.getElementById("tn-mindmap-css") === null) document.head.append(Object.assign(document.createElement("style"), { id: "tn-mindmap-css", textContent: CSS }));
  setFontFamily(family);
  const box = document.createElement("div");
  box.className = "tn-mm";
  box.tabIndex = 0;
  box.dataset.theme = opts.theme;
  box.style.cssText = "position:relative;overflow:hidden;width:100%;height:100%;outline:none";
  const canvas = document.createElement("canvas");
  canvas.style.cssText = "display:block;position:absolute;inset:0;width:100%;height:100%;touch-action:none";
  const badge = document.createElement("div");
  badge.className = "tn-err";
  badge.hidden = true;
  box.append(canvas, badge);
  el.replaceChildren(box);

  let selected = "";
  let peers: Peer[] = [];
  const doc: MindmapDoc = new MindmapDoc(opts.text, {
    darkMode: opts.theme === "dark",
    readonly: opts.readonly,
    measureContext: canvas.getContext("2d")!,
    onChange: opts.onChange,
    onErrors: (errors) => {
      badge.hidden = errors.length === 0;
      badge.textContent = String(errors.length);
      badge.title = errors.map((e) => `line ${e.line}: ${e.message}`).join("\n");
      opts.onErrors?.(errors);
    },
    onRedraw: () => {
      cv.redraw();
      const keys = doc.keys();
      if (opts.onSelection === undefined || keys.join("\n") === selected) return;
      selected = keys.join("\n");
      opts.onSelection(keys);
    },
  });
  const cv = attachCanvas(box, canvas, doc.session, {
    dark: () => box.dataset.theme === "dark",
    quickPanel: () => true,
    undo: opts.onUndo,
    redo: opts.onRedo,
    wheelPans: false,
    // Peers in screen space, so outlines and labels keep their size at any zoom.
    onDraw: () => {
      if (peers.length === 0) return;
      const all = doc.session.element.items;
      const byKey = new Map(keys(all).map((k, n) => [k, all[n]!]));
      const { x, y, k } = cv.view;
      const ctx = canvas.getContext("2d")!;
      const dpr = window.devicePixelRatio || 1;
      ctx.save();
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.setLineDash([]);
      ctx.lineWidth = 2;
      ctx.font = `600 11px ${family}`;
      const labels = new Map<DiagramItem, number>();
      for (const p of peers) {
        for (const key of p.keys) {
          const item = byKey.get(key);
          const d = item === undefined ? undefined : doc.session.scene.drawables.get(item);
          ctx.strokeStyle = ctx.fillStyle = p.color;
          if (d instanceof DrawableLine) {
            ctx.beginPath();
            for (const pt of [d.source, ...d.extraPoints, d.target]) ctx.lineTo(x + pt.x * k, y - pt.y * k);
            ctx.stroke();
          } else if (d !== undefined) {
            const b = d.getSelectorBounds();
            const [left, top, bottom] = [x + b.x * k - 4, y - (b.y + b.height) * k - 4, y - b.y * k + 4];
            ctx.beginPath();
            ctx.roundRect(left, top, b.width * k + 8, bottom - top, 6);
            ctx.stroke();
            // Below the item: the quick panel takes the space above. Several peers on one item line up.
            const at = labels.get(item!) ?? left;
            const w = ctx.measureText(p.name).width + 8;
            ctx.beginPath();
            ctx.roundRect(at, bottom + 2, w, 16, 3);
            ctx.fill();
            ctx.fillStyle = "#fff";
            ctx.fillText(p.name, at + 4, bottom + 14);
            labels.set(item!, at + w + 2);
          }
        }
      }
      ctx.restore();
    },
  });
  // The host may attach `el` before giving it a size: fit once there is one.
  const sized = new ResizeObserver(() => {
    if (box.clientWidth === 0) return;
    sized.disconnect();
    cv.fit();
  });
  sized.observe(box);
  if (opts.inter !== undefined) {
    loadFonts(opts.inter).then(
      () => {
        family = DEFAULT_FONT_FAMILY;
        setFontFamily(family);
        doc.session.rebuild();
        cv.redraw();
      },
      (err) => console.warn("tenniarb: font loading failed", err),
    );
  }

  return {
    update: (text) => {
      // Peer keys follow their items through renames and index shifts until the peers send new ones.
      const before = new Map(keys(doc.session.element.items).map((k, n) => [k, doc.session.element.items[n]!]));
      doc.update(text);
      const now = doc.session.element.items;
      const after = new Map(keys(now).map((k, n) => [now[n]!, k]));
      peers = peers.map((p) => ({ ...p, keys: p.keys.map((k) => after.get(before.get(k)!) ?? k) }));
    },
    getText: () => doc.text,
    setPeers: (next) => {
      peers = next;
      cv.redraw();
    },
    setTheme: (theme: Theme) => {
      box.dataset.theme = theme;
      doc.session.setDarkMode(theme === "dark");
    },
    setReadonly: (readonly) => {
      cv.finishEdit(false);
      doc.session.setReadonly(readonly);
    },
    fit: () => cv.fit(),
    destroy: () => {
      sized.disconnect();
      cv.destroy();
      box.remove();
    },
  };
}

export function render(text: string, opts: { theme?: Theme } = {}): RenderResult {
  const element = new Element("mindmap");
  new ElementModel().add(element);
  reconcile(element, parseBlock(text));
  setFontFamily(family);
  const measure = document.createElement("canvas").getContext("2d")!;
  const scene = layoutScene(element, null, { darkMode: opts.theme === "dark", measureContext: measure });
  const b = scene.getBounds();
  // drawScene's default padding of 15.
  const svg = new SvgContext({ width: b.width + 30, height: b.height + 30, idPrefix: `tnm${++renders}-`, measureText: (font, t) => ((measure.font = font), { width: measure.measureText(t).width }) });
  const { width, height } = drawScene(svg as unknown as Canvas2D, scene);
  return { svg: svg.toString(), width, height };
}

/** `make` is a test seam: a map without DOM. */
export function mindmapNodeView(opts: NodeViewOptions, make = create): (node: PMNode, view: PMView, getPos: () => number | undefined) => MindmapNodeView {
  return (node, view, getPos) => {
    const dom = document.createElement("div");
    dom.className = "tenniarb-mindmap";
    dom.style.height = "360px";
    let readonly = !view.editable;
    const map = make(dom, {
      text: node.textContent,
      theme: opts.theme(),
      readonly,
      inter: opts.inter,
      onUndo: opts.undo,
      onRedo: opts.redo,
      onSelection: opts.onSelection,
      onErrors: opts.onErrors,
      onChange: (changes) => {
        const pos = getPos();
        if (pos === undefined) return;
        const tr = view.state.tr;
        // Last to first, so that the offsets of the earlier ones stay valid; schema.text("") throws, an empty insert is a delete.
        for (const c of [...changes].reverse()) {
          if (c.insert === "") tr.delete(pos + 1 + c.from, pos + 1 + c.to);
          else tr.replaceWith(pos + 1 + c.from, pos + 1 + c.to, view.state.schema.text(c.insert));
        }
        view.dispatch(tr.setMeta("tenniarbMindmap", true));
      },
    });
    return {
      dom,
      map,
      update: (next) => {
        if (next.type !== node.type) return false;
        map.update(next.textContent);
        if (readonly !== !view.editable) map.setReadonly((readonly = !view.editable));
        return true;
      },
      destroy: () => map.destroy(),
      // ProseMirror asks only about events inside `dom`: all of them belong to the map.
      stopEvent: () => true,
      ignoreMutation: () => true,
    };
  };
}
