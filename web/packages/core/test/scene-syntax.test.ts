// Round trip of the scene syntax (docs/spec/tenn-scene.md 4.2, 4.7a, 5b, 5c, 13): the web editor saves
// through this model and printer, so nothing the scene renderer reads may be lost or reshaped.
import assert from "node:assert/strict";
import { test } from "node:test";
import { readTenn, toTennStr } from "../src/index.ts";

const read = (text: string): string => {
  const model = readTenn(text);
  assert.ok(model !== null, "the document parses");
  return toTennStr(model);
};

const button = `element "Button" {
    kind Component
    export let title = "Button"
    export let kind = primary
    export let extras = outlined rounded

    animation spin {
        from { rotate 0 }
        to { rotate 360 }
    }
    animation fade {
        0% { opacity 1 }
        50% { opacity 0.25 }
        100% { opacity 1 }
    }

    styles {
        primary { color #6b3ad3; text-color #ffffff }
        secondary { color #ffffff; border-color #1725471a; text-color #172547 }
        large { height 48; padding 0 16 }
        outlined { border-color #1725471a }
        kind one-of primary secondary
        size one-of large
        extras any-of outlined
    }

    item Btn {
        auto-layout row; gap 8; align center; height 40
        use-style \${kind} \${size}
        item Label { title \${title}; font-size 14; layout left }
    }
}

element "Screen" {
    use Button as save { title "Save"; kind secondary; size large }
}
`;

test("styles, export let, kind and animation blocks survive a save", () => {
  const out = read(button);
  for (const line of [
    "kind Component",
    "export let title",
    "export let kind",
    "export let extras",
    "animation spin",
    "animation fade",
    "from",
    "rotate 0",
    "0%",
    "opacity 0.25",
    "100%",
    "primary",
    "text-color",
    "one-of primary secondary",
    "any-of outlined",
    "use-style ${kind} ${size}",
    "auto-layout row",
  ]) {
    assert.ok(out.includes(line), `the saved text keeps ${line}:\n${out}`);
  }
  // Saving again changes nothing: the model the editor holds and the text it writes agree.
  assert.equal(read(out), out);
});

test("parameters of a use are kept as written", () => {
  const out = read(button);
  assert.match(out, /use Button as save \{[^}]*title "Save"/s);
  assert.match(out, /kind secondary/);
  assert.match(out, /size large/);
});

const quoted = `element "2 Пространства - пусто" {
    item "My Button" { width 20; height 10 }
    item "Кнопка (primary)" { width 30; height 10 }
}

element "Kit" {
    item Dialog {
        item Footer { item Ok { title "OK" } }
    }
}

element "Screen" {
    use "2 Пространства - пусто"."My Button" as q
    use "2 Пространства - пусто".Кнопка__primary_ as k
    use Kit.Dialog as d {
        at "Footer.Ok" { title "Delete" }
        replace Footer {
            item "Later" { title "Later" }
        }
    }
}
`;

test("a segment in double quotes survives a save", () => {
  const out = read(quoted);
  // The printer separates tokens with a space; the path still reads as one quoted segment after a dot.
  assert.ok(out.includes('use "2 Пространства - пусто" . "My Button" as q'), out);
  assert.ok(out.includes('at "Footer.Ok"'), out);
  assert.ok(out.includes("replace Footer"), out);
  // The path still names the same nodes after the round trip.
  const model = readTenn(out);
  assert.ok(model !== null);
  assert.equal(toTennStr(model), out);
});

test("an element --from-file argument is not dropped", () => {
  const out = read(`element "ui_kit" --from-file "libs/ui-kit.tenn"\n\nelement "Screen" {\n    item A { pos 0 0 }\n}\n`);
  assert.ok(out.includes('element "ui_kit" --from-file "libs/ui-kit.tenn"'), out);
});

test("an expression in a property and in a title survives a save", () => {
  const out = read(`element "Screen" {\n    item A { width \${20 * 4}; title "Hi \${who}"; color \${tone === "warn" ? "red" : "none"} }\n}\n`);
  assert.ok(out.includes("width ${20 * 4}"), out);
  assert.ok(out.includes('title "Hi ${who}"'), out);
  assert.ok(out.includes('color ${tone === "warn" ? "red" : "none"}'), out);
});
