// The editor keeps working on a scene element (spec 4, 5b): the session builds and draws it through the
// scene renderer, and a save writes the new syntax back. A classic diagram draws its picture through the
// same port while selection and hit-test stay on the old drawables.
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

/** The box of the first filled path the session draws, in scene units. */
function drawnBox(session: EditorSession): { x: number; y: number; width: number; height: number } {
  const svg = new SvgContext({ width: 300, height: 200, measureText: () => ({ width: 10 }) });
  session.draw(svg as never);
  const d = /<path d="([^"]+)"/.exec(svg.toString())![1]!;
  const n = [...d.matchAll(/-?\d+(?:\.\d+)?/g)].map((m) => Number(m[0]));
  const xs = n.filter((_, i) => i % 2 === 0);
  const ys = n.filter((_, i) => i % 2 === 1);
  const [x0, x1, y0, y1] = [Math.min(...xs), Math.max(...xs), Math.min(...ys), Math.max(...ys)];
  return { x: x0, y: y0, width: x1 - x0, height: y1 - y0 };
}

test("a classic element is drawn by the scene port where its drawables are", () => {
  const model = readTenn('element "D" { item "A" { pos 10 -20; title "Tiny" } }')!;
  const item = model.elements[0]!.items[0]!;
  const session = new EditorSession(model.elements[0]!, {});
  const port = (session as unknown as { tenn: { boxes: { name: string; x: number; y: number; width: number; height: number }[] } }).tenn;
  const box = port.boxes.find((b) => b.name === "A")!;
  const drawn = drawnBox(session);
  // The port lays its boxes out y-down; the drawables are y-up, so the picture is the box flipped at the origin.
  assert.deepEqual(drawn, { x: box.x, y: -box.y - box.height, width: box.width, height: box.height });

  // The selection box is the drawable's, so the two share the top left corner.
  const d = session.scene.drawables.get(item)!.getSelectorBounds();
  assert.equal(drawn.x, d.x);
  assert.equal(drawn.y + drawn.height, d.y + d.height);

  // Dragging moves the drawable; the picture is previewed at the same delta, so the box stays on the item.
  const centre = { x: d.x + d.width / 2, y: d.y + d.height / 2 };
  session.down(centre);
  session.move({ x: centre.x + 30, y: centre.y - 7 });
  const moved = drawnBox(session);
  assert.deepEqual({ x: moved.x - drawn.x, y: moved.y - drawn.y }, { x: 30, y: -7 });
  const at = session.scene.drawables.get(item)!.getSelectorBounds();
  assert.deepEqual({ x: at.x - d.x, y: at.y - d.y }, { x: 30, y: -7 });
});
