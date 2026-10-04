// Text builders of the ExportManager share menu: generateHtml, generateInteractiveHtml, elementPath.
import type { DiagramItem, Element } from "@tenniarb/core";
import { TextBox } from "@tenniarb/render";
import type { DrawableScene } from "@tenniarb/render";

export function base64(bytes: Uint8Array): string {
  let s = "";
  for (let i = 0; i < bytes.length; i += 0x8000) s += String.fromCharCode(...bytes.subarray(i, i + 0x8000));
  return btoa(s);
}

const attr = (v: string): string => v.replaceAll("&", "&amp;").replaceAll('"', "&quot;").replaceAll("<", "&lt;");

/** The parentless model root is not part of the path. */
export function elementPath(el: Element): string {
  const names: string[] = [];
  for (let cur: Element | null = el; cur !== null && cur.parent !== null; cur = cur.parent) names.unshift(cur.name);
  return names.join("/");
}

export const pngHtml = (pngBase64: string, width: number, height: number): string =>
  `<html>\n\t<body>\n\t\t<img style="border: 1px solid #eeeeee;" width="${width}" height="${height}" src="data:image/png;base64,${pngBase64}"/>\n\t</body>\n</html>`;

/** `bundle` is the embed IIFE, `tenn` the whole document. */
export const interactiveHtml = (bundle: string, el: Element, tenn: string): string =>
  `<!doctype html>\n<html>\n<head>\n<meta charset="utf-8">\n<title>${attr(el.name)}</title>\n<style>html, body { margin: 0; height: 100%; }</style>\n</head>\n<body>\n<script>${bundle}</script>\n<script type="text/x-tenn" data-encoding="base64" style="height: 100vh" data-element="${attr(elementPath(el))}">${base64(new TextEncoder().encode(tenn))}</script>\n</body>\n</html>`;

/** SceneDrawView.copyItemAsHTML: the item's text boxes as HTML to the clipboard (text/html and text/plain). */
export async function copyItemHtml(scene: DrawableScene, item: DiagramItem): Promise<void> {
  let html = "";
  scene.drawables.get(item)?.traverse((d) => {
    if (d instanceof TextBox) html = d.html();
    return true;
  });
  const blob = (type: string): Blob => new Blob([html], { type });
  await navigator.clipboard.write([new ClipboardItem({ "text/html": blob("text/html"), "text/plain": blob("text/plain") })]);
}

/** Print-only layout: the host is hidden on screen, the rest of the page is hidden in print. `pdf`: page = the diagram, no margin (Swift exportPdf). */
export const printCss = (width: number, height: number, pdf: boolean): string =>
  `@page{${pdf ? `size:${width}px ${height}px;margin:0` : "margin:15mm"}}#tn-print{display:none}@media print{html,body{height:auto!important;overflow:visible!important;background:#fff!important}body>*:not(#tn-print){display:none!important}#tn-print{display:block;height:100vh;overflow:hidden}#tn-print svg{display:block;max-width:100%;max-height:100%;margin:auto}}`;
