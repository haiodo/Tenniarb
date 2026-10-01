import { test } from "node:test";
import assert from "node:assert/strict";
import { ImageProvider, MarkdownLexer, calcTitleFontSize, colorToHex, getMaxRect, parseColor, toHTML } from "../src/index.ts";

// HTMLPrinterTests.swift: a provider that always resolves to 100x50.
class MockImageProvider extends ImageProvider {
  override resolveImagePath(_path: string) {
    return { image: "", rect: { x: 0, y: 0, width: 100, height: 50 } };
  }
}

function generateHTML(markdown: string, imageProvider: ImageProvider = new MockImageProvider(1)): string {
  return toHTML(MarkdownLexer.getTokens(markdown), 16.0, "black", imageProvider);
}

test("testPlainText", () => {
  assert.equal(generateHTML("Hello, world!"), "<div><span>Hello, world!</span></div>");
});

// Expectations below were produced by the Swift HTMLPrinter.

test("inline styles", () => {
  assert.equal(generateHTML("*b* _i_ ~s~ <u>"), "<div><strong>b</strong><span> </span><em>i</em><span> </span><del>s</del><span> </span><u>u</u></div>");
  assert.equal(generateHTML("!(red|w) &(20|v)"), '<div><span style="color: #FF0000;">w</span><span> </span><span style="font-size: 20.0px;">v</span></div>');
  assert.equal(generateHTML("&(20) a &() b"), '<div><span style="font-size: 20.0px"> a </span><span style="font-size: 16.0px"> b</span></div>');
});

test("titles and bullets", () => {
  assert.equal(generateHTML("## T\n* a\n* b\n"), '<div><h2 style="font-size: 17.0px;">T</h2>\n<ul style="margin-left: 5px;">\n<li>*</li>\n<span> a</span><span>\n</span><li>*</li>\n<span> b</span><span>\n</span></ul>\n</div>');
  assert.deepEqual(calcTitleFontSize("### \tx", 16), ["x", 15, 3]);
});

test("escaping", () => {
  assert.equal(generateHTML("a<b&\"'"), "<div><span>a&lt;b&amp;&quot;&#39;</span></div>");
  assert.equal(generateHTML("&́"), "<div><span>&́</span></div>");
});

test("code and images", () => {
  assert.equal(generateHTML("`x`"), '<div><code style="background-color: gray-200; color: black;">x</code></div>');
  assert.equal(generateHTML("@(a)"), '<div><img src="a" width="100.0" height="50.0" alt="a" /><br></div>');
});

test("colors", () => {
  assert.deepEqual(parseColor("#ff8000", 0.5), { r: 1, g: 128 / 255, b: 0, a: 0.5 });
  assert.deepEqual(parseColor("nosuch"), { r: 0, g: 0, b: 0, a: 1 });
  assert.equal(colorToHex(parseColor("red")), "#FF0000");
  assert.equal(colorToHex(parseColor("blue-500")), "blue-500");
});

test("image sizes", () => {
  class P extends ImageProvider {
    override resolveImage() {
      return { image: "", size: { width: 100, height: 50 } };
    }
  }
  const sized = (path: string) => new P(1).resolveImagePath(path)?.rect;
  assert.deepEqual(sized("a"), { x: 0, y: 0, width: 100, height: 50 });
  assert.deepEqual(sized("a|40x40"), { x: 0, y: 0, width: 40, height: 40 });
  assert.deepEqual(sized("a|40"), { x: 0, y: 0, width: 40, height: 20 });
  assert.deepEqual(sized("a|x25"), { x: 0, y: 0, width: 50, height: 25 });
  assert.deepEqual(getMaxRect(200, 200, 100, 50), { x: 0, y: 0, width: 100, height: 50 });
});
