import { test } from "node:test";
import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { readTenn } from "@tenniarb/core";
import { base64, elementPath, interactiveHtml, pngHtml, printCss } from "../src/export.ts";

const tenn = 'element "Doc" {\n element "A & \\"B\\"" {\n item "x"\n }\n}';
const model = readTenn(tenn)!;
const el = model.elements[0]!.elements[0]!;

test("elementPath skips the model root", () => {
  assert.equal(elementPath(el), 'Doc/A & "B"');
});

test("interactiveHtml: embedded document round-trips, attributes escaped", () => {
  const html = interactiveHtml("var b=1;", el, tenn + "\nж");
  assert.match(html, /^<!doctype html>/);
  assert.match(html, /<title>A &amp; &quot;B&quot;<\/title>/);
  assert.match(html, /<script>var b=1;<\/script>/);
  assert.match(html, /data-element="Doc\/A &amp; &quot;B&quot;"/);
  const b64 = />([^<]+)<\/script>\n<\/body>/.exec(html)![1]!;
  assert.equal(new TextDecoder().decode(Uint8Array.from(atob(b64), (c) => c.charCodeAt(0))), tenn + "\nж");
});

test("pngHtml matches generateHtml", () => {
  assert.equal(pngHtml("QUJD", 10, 20.5), '<html>\n\t<body>\n\t\t<img style="border: 1px solid #eeeeee;" width="10" height="20.5" src="data:image/png;base64,QUJD"/>\n\t</body>\n</html>');
});

test("base64 handles chunks over 32k", () => {
  const big = new Uint8Array(100_000).fill(65);
  assert.equal(base64(big), Buffer.from(big).toString("base64"));
});

const bundle = new URL("../../embed/dist/tenniarb-embed.standalone.min.js", import.meta.url);
test("the embed bundle can sit inside <script>", { skip: !existsSync(bundle) }, () => {
  assert.ok(!/<\/script/i.test(readFileSync(bundle, "utf8")));
});

test("printCss: pdf sizes the page to the diagram without margin, print keeps paper margins", () => {
  assert.match(printCss(300, 200, true), /@page\{size:300px 200px;margin:0\}/);
  assert.match(printCss(300, 200, false), /@page\{margin:15mm\}/);
  assert.match(printCss(1, 1, false), /body>\*:not\(#tn-print\)\{display:none!important\}/);
});
