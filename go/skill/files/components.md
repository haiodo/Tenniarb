# Components and libraries

`use <path> as <name> { overrides }` makes an instance of any element or item; the name after `as` is
required. Write a part once and use it, instead of copying its text.

An element written for reuse says so and declares what an instance may set:

```tenn
element "Button" {
  kind Component                 // a label for the app's lists, changes no layout
  export let title = "Button"    // a parameter with a default; without "=" it is required
  export let kind = primary      // a value that picks a style, see "Styles" below
  export let size = medium
  styles {
    primary { color #6b3ad3 }
    secondary { color #ffffff; border-color #d1d5db; text-color #111827 }
    large { height 48; padding 0 16 }
    medium { height 40; padding 0 12 }
    kind one-of primary secondary
    size one-of large medium
  }
  item Btn {
    auto-layout row
    corner-radius 8
    use-style ${kind} ${size}
    item Label { title ${title}; font-size 14 }
  }
}

use Button as save { title "Save"; kind secondary; size large }
```

`${...}` is evaluated: a parameter is an ordinary variable inside the element, so `width ${size + 12}`
and a style picked by `${kind}` work. A parameter may hold a list of words (`extras outlined rounded`).

## A library

A library is an ordinary `.tenn` file. Slots are the places where an instance puts its own nodes:
`slot` without a name is the default one, `slot name` a named one, and the block of a slot is the
fallback shown while nobody fills it.

```tenn
// file: lib/kit.tenn
element "Kit" {
  element "Button" {
    kind Component
    export let title = "Button"
    export let kind = primary
    styles {
      primary { color #2563eb; text-color #ffffff }
      secondary { color #ffffff; border-color #d1d5db }
      kind one-of primary secondary
    }
    item Btn {
      auto-layout row
      padding 8 14
      corner-radius 6
      use-style ${kind}
      item Label { title ${title}; font-size 14 }
    }
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
      use kit.Kit.Button as cancel { title Cancel; kind secondary }
      use kit.Kit.Button as send { title "Send invite" }
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
- Mount a library once, in the entry document (`PlatformDesign.tenn`), and mount the app files there
  too. A mounted file does not repeat the library mount: its names are looked up in the file, then where
  it is mounted, up to the entry. The same file mounted under two entries uses each entry's library.
  A part opened alone is checked in the context of its entry (found by searching up the folders, or set
  with a first line `entry "../PlatformDesign.tenn"`); `validate` and `tenniarb check` name it.

Paths to remember:

- An instance inside a component is a segment too: `at Toolbar.save.Label { title Save }` changes
  the label of the instance `save` inside `Toolbar`.
- A slot name is a segment only to reach its fallback (`at default.Empty { title "No one yet" }`; the
  unnamed slot is `default`). A path that ends on a slot is an error: fill it with `slot <name> { }`.
- A path that matches two nodes is an error with the candidates listed: add a segment.

## Styles: one component, many looks

A variant is a named style of the same component, not a second copy: `styles { primary { … } large { … } }`
in the element, `use-style primary large` on the part that looks different. Rules that matter when writing
one:

- a style comes where its `use-style` is written: properties above it are defaults, properties below it win,
  and `at` from the instance wins over both;
- the last style named wins per property; `use-style -large` drops one;
- a block inside a style is a selector for a kid: `wide { Label { width 200 } }`;
- `one-of` and `any-of` lines in `styles` list what is valid: the app shows them as a menu and check boxes,
  and a parameter whose default is a member of the group becomes that menu on the instance;
- `text-color` and the font are inherited by the parts below, the text colour defaults to the contrast of the
  box; `none` is the transparent colour (`border-color none`).

A catalogue of copies (`Button`, `Button secondary`, `Button/primary/large`) is the old form: keep using it
when a library already has it (a pen.dev import does), but a new part is one component with styles.

## Templates: code out of a component

A component can carry the code it generates: `template %{ … }` with `${param}` and `<slot/>`, and
`tenniarb gen [-set name=value] FILE ELEMENT` prints it, and the MCP tool `gen` does the same. The text inside `%{ … }` is not parsed, so it may be
Svelte, JSX or anything else.

```tenn
element "Chip" {
  kind Component
  export let label = "Chip"
  template %{<span class="chip">${label}<slot/></span>%}
}
```

## Finding components in a large library

- A component line of `get_state` also names its `kind`, `params`, `styles` and `groups`
  (`Kit.Button element 2 scene kind=component params=title,kind styles=primary,secondary groups=kind:one-of(primary secondary)`),
  so the overrides are known without reading the component; `!` after a parameter marks it required.
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
