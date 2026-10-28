# Components and libraries

Any item is a component: there is no marker. `use <path> as <name> { overrides }` makes an instance;
the name after `as` is required. Write a part once and use it, instead of copying its text.

## A library

A library is an ordinary `.tenn` file. Slots are the places where an instance puts its own nodes:
`slot` without a name is the default one, `slot name` a named one, and the block of a slot is the
fallback shown while nobody fills it.

```tenn
// file: lib/kit.tenn
element "Kit" {
  item Button {
    auto-layout row
    padding 8 14
    corner-radius 6
    color #2563eb
    item Label { title Button; font-size 14; text-color #ffffff }
  }
  item "Button secondary" {
    auto-layout row
    padding 8 14
    corner-radius 6
    border-color #d1d5db
    item Label { title Button; font-size 14 }
  }
  item Card {
    auto-layout column
    width 360
    padding 16
    gap 12
    corner-radius 10
    color #ffffff
    border-color #e5e7eb
    item Title { title Card; font-size 16; font-weight 700 }
    slot {
      item Empty { title "Nothing here"; font-size 13; text-color #9ca3af }
    }
    item Footer {
      auto-layout row
      width fill
      gap 8
      justify end
      slot actions
    }
  }
}
```

## Using it

```tenn
// file: invite.tenn
element "kit" --from-file "lib/kit.tenn"
element "Invite" {
  use kit.Kit.Card as invite {
    width 400
    at Title { title "Invite people" }
    item Email {
      auto-layout row
      width fill
      height 36
      padding 0 10
      align center
      corner-radius 6
      border-color #d1d5db
      item v { title "name@example.com"; font-size 14; text-color #9ca3af }
    }
    slot actions {
      use kit.Kit.Button_secondary as cancel { at Label { title Cancel } }
      use kit.Kit.Button as send { at Label { title "Send invite" } }
    }
  }
}
```

Inside the block of a `use`:

- a property (`width 400`, `color red`, `pos 0 -300`) overrides the root of the instance;
- `at <path> { ... }` overrides a part; the path starts at the children of the component root;
- `replace <path> { nodes }` puts other nodes in place of a part;
- `slot <name> { nodes }` fills a named slot; nodes written directly (`item`, `use`) go to the default
  slot. Filling a slot drops its fallback; an empty `slot {}` (default) or `slot name {}` drops it and
  puts nothing.
- `element "<name>" --from-file "<file>"` takes the whole content of the element from another file. The
  path is relative to the file with the line and must stay in the workspace. The path of a use through
  it starts with the element name: `kit.Kit.Card`.
- To change a component, `edit` the library file itself, not the line that mounts it: the mounting file
  never holds the library text.

Paths to remember:

- An instance inside a component is a segment too: `at Toolbar.save.Label { title Save }` changes
  the label of the instance `save` inside `Toolbar`.
- A slot name is a segment only to reach its fallback (`at default.Empty { title "No one yet" }`; the
  unnamed slot is `default`). A path that ends on a slot is an error: fill it with `slot <name> { }`.
- A path that matches two nodes is an error with the candidates listed: add a segment.

## Variants

A variant is a separate component: `Button`, `Button secondary`, `Button/primary/large`. Pick the one
you need by its path, then override the rest. Do not rebuild a variant by hand from another one.

## Finding components in a large library

- `get_state {file: "lib/kit.tenn", find: "Button"}` lists every node whose name has the text, with its
  full path inside that file; prefix the alias to use it.
- `get_state {file: "screen.tenn", path: "kit", find: "Button"}` searches through the mounted file and prints
  paths ready for `use`.
- `get_state {file, find: "Button", depth: 3}` adds the parts of each match, as `at` and `replace`
  address them, and its slots (`slot` lines). That is usually enough to write the overrides without
  `read`. `get_state {file, path: <component>, depth: 4}` lists one component deeper.

A library converted from pen.dev (`pen2tenn`) keeps the pen.dev catalog: each component sits in
`<Page>.<Page_Component>.components.<row>.<cell>.<Component_variant>`, next to an item `label` with
its pen.dev name. `header`, `components`, `row`, `cell` and `label` are the catalog, not components:
use the deepest node, such as `ui.Forms.Forms_Input.components.cell.Forms_Input`. Its text leaves carry
`layout left` from the converter; leave it. Other habits of such
libraries:

- Large components (a window, a settings layout) use `auto-layout abs`: their parts sit at `pos`, and
  `at <part> { pos X Y; height H }` moves one.
- Optional parts are there with `hidden yes`: `at <part> { hidden no }` shows one, `hidden yes` drops
  an icon you do not want.
- A slot often holds sample content as its fallback (a list of nav items, a sample row): fill the slot
  to replace it, or leave it to keep the sample.
