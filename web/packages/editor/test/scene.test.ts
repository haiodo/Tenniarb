// The editor keeps working on a scene element (spec 4, 5b): the session builds and draws it through the
// scene renderer, and a save writes the new syntax back.
import assert from "node:assert/strict";
import { test } from "node:test";
import { readTenn, toTennStr } from "@tenniarb/core";
import { SvgContext } from "@tenniarb/render";
import "../../render/test/helpers.ts";
import { EditorSession } from "../src/session.ts";

const SRC = `element "Button" {
    kind Component
    export let title = "Button"
    export let kind = primary
    styles {
        primary { color #6b3ad3; text-color #ffffff }
        secondary { color #ffffff }
        kind one-of primary secondary
    }
    item Btn {
        auto-layout row
        use-style \${kind}
        item Label { title \${title} }
    }
}

element "Screen" {
    item row {
        auto-layout row
        gap 16
        use Button as a { title "Save"; kind primary }
        use Button as b { title "Cancel"; kind secondary }
    }
}
`;

test("a scene element opens, draws and saves in the editor", () => {
  const model = readTenn(SRC);
  assert.ok(model !== null);
  const screen = model.elements.find((e) => e.name === "Screen")!;
  const session = new EditorSession(screen, {});
  const bounds = session.scene.getBounds();
  assert.ok(bounds.width > 0 && bounds.height > 0);

  const svg = new SvgContext({ width: Math.ceil(bounds.width) + 30, height: Math.ceil(bounds.height) + 30, measureText: () => ({ width: 10 }) });
  session.scene.offset = { x: 15, y: 15 - bounds.y };
  session.scene.draw(svg as never);
  const out = svg.toString();
  assert.match(out, /<text[^>]*>Save<\/text>/);
  assert.match(out, /<text[^>]*>Cancel<\/text>/);
  assert.match(out, /fill="rgb\(107,58,211\)"/);

  // A save keeps the styles and the parameters of the use (nothing silently reverts to a classic diagram).
  const text = toTennStr(model);
  for (const line of ["kind Component", "export let title", "one-of primary secondary", "use-style ${kind}", 'title "Save"']) {
    assert.ok(text.includes(line), `saved text keeps ${line}`);
  }
});
