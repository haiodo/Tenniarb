// element --from-file (spec 3a, go/scene load.go) and template (spec 5c, go/scene template.go): the files
// are read through the loader the caller gives, as images are decoded through the decoder.
import assert from "node:assert/strict";
import { test } from "node:test";
import { readTenn } from "@tenniarb/core";
import type { Element } from "@tenniarb/core";
import { buildTennScene, renderTemplate } from "../src/scene-tenn.ts";
import type { TennScene } from "../src/scene-tenn.ts";
import { renderTenn } from "../src/node.ts";

const reader = (files: Record<string, string>): ((path: string) => string | null) => (path) => files[path] ?? null;

function elementOf(text: string, name: string): Element {
  const model = readTenn(text);
  assert.ok(model !== null, "the document parses");
  const element = model.elements.find((e) => e.name === name);
  assert.ok(element !== undefined, `element ${name}`);
  return element;
}

const scene = (text: string, name: string, files: Record<string, string>, file = "app/main.tenn"): TennScene =>
  buildTennScene(elementOf(text, name), { file, readFile: reader(files), source: text });

const box = (s: TennScene, name: string): TennScene["boxes"][number] => {
  const b = s.boxes.find((it) => it.name === name);
  assert.ok(b !== undefined, `box ${name}: ${s.boxes.map((it) => it.name).join(", ")}`);
  return b;
};

const kit = `element "Parts" {
  item Button { width 40; height 20; color #ff0000 }
}
element "Inner" {
  item box { auto-layout row; use Parts.Button as inner }
}
element "withIcon" {
  item box { auto-layout row; use Icon as dot }
}
item Card { width 10; height 10; color #00ff00 }
`;

const main = `element "ui_kit" --from-file "kit.lib.tenn"
element "Icon" {
  item Dot { width 8; height 8 }
}
element "Screen" {
  item col { auto-layout column; gap 4
    use ui_kit.Parts.Button as b
    use ui_kit.Card as c
    use ui_kit.Inner as i
    use ui_kit.withIcon as w
  }
}
`;

test("a mounted file gives its elements and content under the name of the mounting element", () => {
  const files = { "app/kit.lib.tenn": kit };
  const s = scene(main, "Screen", files);
  assert.deepEqual(s.diagnostics, []);
  assert.deepEqual([box(s, "b").width, box(s, "b").height], [40, 20]);
  assert.deepEqual([box(s, "c").width, box(s, "c").height], [10, 10]);
});

test("a component of a mounted file names its siblings by a short path", () => {
  const s = scene(main, "Screen", { "app/kit.lib.tenn": kit });
  assert.equal(box(s, "inner").width, 40);
});

test("a mounted file sees the names of the place it is mounted at", () => {
  const s = scene(main, "Screen", { "app/kit.lib.tenn": kit });
  assert.deepEqual([box(s, "dot").width, box(s, "dot").height], [8, 8]);
});

test("renderTenn reads the files it is given", async () => {
  const text = `element "ui_kit" --from-file "lib.tenn"
element "Screen" { item col { auto-layout column; use ui_kit.Parts.Button as b } }
`;
  const lib = `element "Parts" { item Button { title "Mounted"; width 40; height 20 } }\n`;
  const svg = await renderTenn(text, { format: "svg", file: "main.tenn", readFile: (p) => (p === "lib.tenn" ? lib : null) });
  assert.match(svg, /Mounted/);
});

test("a mount cycle is an error that names every --from-file on the way round", () => {
  const a = `element "x" --from-file "b.tenn"\nelement "S" { item i { width 10; height 10 } }\n`;
  const b = `element "y" --from-file "a.tenn"\n`;
  const s = scene(a, "S", { "b.tenn": b, "a.tenn": a }, "a.tenn");
  const messages = s.diagnostics.map((d) => d.message);
  assert.ok(
    messages.some((m) => m === "import cycle: a.tenn:1:1 imports b.tenn, b.tenn:1:1 imports a.tenn"),
    messages.join("\n"),
  );
  // The element with the bad file stays in the tree, empty: the rest of the screen draws.
  assert.equal(box(s, "i").width, 10);
});

test("a missing file and a path outside the workspace root are errors, not crashes", () => {
  const text = `element "out" --from-file "../outside.tenn"
element "gone" --from-file "nope.tenn"
element "S" { item i { width 10; height 10 } }
`;
  const s = scene(text, "S", {}, "c.tenn");
  assert.deepEqual(
    s.diagnostics.map((d) => d.message),
    ['element --from-file "../outside.tenn" leaves the workspace root', "cannot read nope.tenn"],
  );
  assert.equal(box(s, "i").width, 10);
});

test("an element with --from-file also names the elements of the mounted file", () => {
  const text = `element "kit" --from-file "kit.lib.tenn"
element "S" { item i { use kit.Parts.Button as b } }
`;
  const s = scene(text, "S", { "app/kit.lib.tenn": kit });
  assert.deepEqual(s.diagnostics, []);
  assert.equal(box(s, "b").width, 40);
});

const templates = `element "Chip" {
  kind Component
  export let label = "Chip"
  export let tone = plain
  template %{<span class="\${tone}">\${label}</span>%}
}
element "Card" {
  kind Component
  export let title = "Button"
  export let tone = secondary
  template %{
<button class="btn-\${tone}">\${title}<slot/></button>
%}
  item Box { auto-layout column
    slot { item Empty { title "empty" } }
  }
}
element "Panel" {
  kind Component
  export let width
  template %{<aside style="width: \${width}"></aside>%}
}
element "Plain" {
  kind Component
  template %{<b><slot/></b>%}
}
element "Screen" {
  item col { auto-layout column
    use Chip as one
    use Chip as two { label "Save"; tone primary }
    use Card as card { item Body { title "body" } }
  }
}
`;

test("a template renders the defaults of the parameters and the values of -set", () => {
  const chip = elementOf(templates, "Chip");
  assert.equal(renderTemplate(chip, { source: templates }).text, '<span class="plain">Chip</span>');
  assert.equal(
    renderTemplate(chip, { source: templates, values: { label: "Go", tone: "green" } }).text,
    '<span class="green">Go</span>',
  );
});

test("a template of a use takes the block of that use", () => {
  const screen = elementOf(templates, "Screen");
  assert.equal(renderTemplate(screen, { source: templates, path: "Screen.col.two" }).text, '<span class="primary">Save</span>');
});

test("a slot the use leaves empty gets the fallback of the component", () => {
  const card = elementOf(templates, "Card");
  assert.equal(renderTemplate(card, { source: templates }).text, '<button class="btn-secondary">Buttonempty</button>');
  const screen = elementOf(templates, "Screen");
  assert.equal(renderTemplate(screen, { source: templates, path: "Screen.col.card" }).text, '<button class="btn-secondary">Buttonbody</button>');
});

test("template errors are the text of Go's errors and no text", () => {
  const chip = elementOf(templates, "Chip");
  const unknown = renderTemplate(chip, { source: templates, values: { tone: "green", nope: "1" } });
  assert.equal(unknown.text, "");
  assert.deepEqual(unknown.errors.map((e) => e.message), ['unknown parameter "nope" (parameters: "label", "tone")']);

  const panel = elementOf(templates, "Panel");
  const missing = renderTemplate(panel, { source: templates });
  assert.equal(missing.text, "");
  assert.ok(missing.errors.some((e) => e.message === 'element "Panel" needs "width": set it in the block of the use or with -set'));

  const plain = elementOf(templates, "Plain");
  assert.deepEqual(
    renderTemplate(plain, { source: templates }).errors.map((e) => e.message),
    ["the default slot has no content: pass nodes in the block of the use"],
  );
});

test("template { ... } keeps the text between the braces, dedented", () => {
  const text = `element "B" {
  kind Component
  template {
    <i>hi</i>
  }
}
`;
  const b = elementOf(text, "B");
  assert.equal(renderTemplate(b, { source: text }).text, "<i>hi</i>");
});
