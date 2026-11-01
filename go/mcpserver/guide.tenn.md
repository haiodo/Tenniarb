# .tenn reference

The commands the scene engine reads today, checked against `go/scene`. A file is a tree of elements;
an element is a canvas drawn as a UI scene (auto-layout, components) or as a classic diagram.

## 1. Commands and names

A command is a name, its arguments and an optional block: `item Card { color #ffffff }`. Commands end
at a new line or `;`; the `{` stays on the line of its command.

- Bare words hold letters, digits, `_ - . #`: `title Settings`, `color #2f6bff`. Quote anything else:
  `title "Sign in"`, `title ""`, `title "2024"`.
- `//` starts a comment that runs to the end of the line.
- A name becomes a path segment: every character that is not a letter or digit turns into `_`, so
  `item "Primary button"` is `Primary_button`. Paths are dotted and case-sensitive:
  `Screens.Login.Card.Heading`. A path that matches two nodes is an error.
- `link "A" "B" { display arrow }` joins two items of the same parent by name;
  `source-index`/`target-index` pick among equal names. `display arrow` draws the head, `color` and
  `line-width` style the line.

An element holds `item`, `use`, `link` and nested `element`s; an item holds `item`, `use`, `link`,
`slot`; a `link` holds properties only.

## 2. Elements, kinds and files

- `element "Login" { ... }` is one canvas; elements nest and `use` reaches them by path.
- `kind Component`, `kind Screen`, `kind Library`, `kind Application`: inside the block, `element "Button" { kind
  Component`. A kind sorts elements in the app's lists and changes no layout; without one an element
  is a screen. At the file root `kind Library` marks the file; an unknown kind is a warning.
- `element "kit" --from-file "lib/kit.tenn"` takes the content of the element from that file and
  takes no block. The path is relative to the writing file and may not leave the workspace root. What
  the file holds becomes the content of `kit`, so `use kit.Card as c` reaches it; a name inside the
  mounted file resolves there first, then where it is mounted.

## 3. Items and auto-layout

`item Name { ... }` is a box, a text leaf or a container. An item with `auto-layout`, or inside a
parent that has one, is in a layout context:

- `auto-layout row`, `column` or `abs`; `gap N`; `line-gap N` with `wrap yes`;
- `padding N`, `padding V H`, `padding T R B L`;
- `align start|center|end` (cross axis); `justify start|center|end|space-between` (main axis);
- `width` and `height` take `N`, `hug` or `fill`; `min-width`, `max-width`, `min-height`,
  `max-height` take numbers only;
- `absolute yes` with `pos X Y` takes a child out of the flow.

Any item, in a layout or not, takes `corner-radius N` or `TL TR BR BL`, `shadow DX DY [BLUR
[COLOR]]` (Y points up: `shadow 0 -4 8` falls down), `opacity 0..1` and `hidden yes` (not laid out, not
drawn); `clip yes` cuts the children at the box in a layout.

In a layout context the name is an alias: text is drawn only from `title`. Outside it the name is the
text, and `title` overrides it. A leaf with a fixed or `fill` width wraps its text at spaces.

## 4. pos and the Y axis

Y points up everywhere: `pos 10 20` puts the top left corner 20 units above the origin. `pos` places
the top left corner of an item outside a layout; children of `auto-layout` need no `pos`; an
`absolute yes` child takes `pos` from the top left of its parent, so 8 units down is `pos 8 -8`.
Content at a negative position is kept in the scene bounds.

## 5. use: instances, at, replace, slot

- `use <path> as <name>`: the path is dotted and names an element, an item or another `use`; `as` is
  required. The name is an alias for paths.
- Properties in the block go to the instance root (a name that is not a parameter is one of them).
  Nodes (`item`, `use`, `link`) go to the component's default slot.
- `at <path> { ... }` reaches a node inside the instance and applies properties to it, under the same
  rules as the component's body: `at Btn.Icon { path "M3 3L9 9" }`.
- `replace <path> { item ... }` swaps that node for the nodes of the block; only `item`, `use` and
  `link` are allowed there.
- `slot <name> { ... }` fills a named slot; an empty `slot {}` drops the fallback. Nodes with no
  default slot, or an unknown slot name, are errors.
- The path follows the expanded tree: item, instance and slot names, with `default` for the unnamed
  slot. `at` cannot end at a slot. Several matches make the path an error.
- `use-style` in the use block names a style of the element the use is written in, not of the
  component; for the component's own styles write it in `at <node> { use-style ... }`.

## 6. Parameters

- `export let title = "Step"` declares a parameter with a default; `export let title` makes it
  required, and a use that does not set it is an error.
- The use sets it by name: `use Step as "Sign up" { title "Sign up"; tone warn }`.
- Inside the element the parameter is an ordinary variable: `title ${title}`,
  `width ${size + 12}`, `use-style ${tone}`. `${...}` and `$(...)` run on the classic engine before
  the styles, so a style can be picked by a value.

## 7. styles and use-style

- `styles { primary { color #6b3ad3 } large { height 48 } }` in an element: named sets of
  properties. A node names the styles of the nearest element above it that has a `styles` block.
- `use-style primary large` takes several styles; the last one named wins per property.
  `use-style -large` drops a style named before it.
- A style lands where its `use-style` is written: properties above it are defaults and the style wins
  over them, properties below it win over the style. `at` properties are written after the
  definition, so they beat the definition and its styles; a `use-style` in the same `at` block still
  lands on its own line, so what follows it wins.
- A block inside a style sets a property of the kid with that name, as a CSS descendant selector:
  `large { Icon { hidden yes } }`. The kid's own and style properties still win.
- Groups are a declaration for the app's inspector, not a check: `tone one-of step end` (one of them)
  and `shape any-of icon-only` (any number). A line without a block and without `one-of`/`any-of` is
  not a style.

## 8. Colors and text

- `color` fills, `border-color` draws the border (`line-width N` for its width), `text-color` colors
  the text. A color is `#rrggbb`, `#rrggbbaa` (with or without `#`) or a palette name such as `red`,
  `blue-200`, `gray-400`; three digits are not a color. `none` and `transparent` mean no fill and no
  border.
- `color linear ANGLE COLOR [POS] COLOR [POS] ...` is a gradient: the angle is CSS degrees (0 up, 90
  right), positions 0..1 on every stop or on none.
- `text-color` and the font inherit down: `font-family`, `font-size` (default 18), `font-weight`
  (100..900, `normal`, `bold`), `font-style` (`italic`), `letter-spacing`. Fill, border, size,
  `opacity` and layout do not inherit.
- Without `text-color` an item takes black or white, whichever contrasts with its own visible fill;
  without a fill it takes the text colour of its parent. So `text-color` on a box colors everything
  inside it.
- `layout left` or `right` aligns the lines of a leaf with a width; the default is centered.
- An icon leaf is `path "M..."` with `view-box X Y W H` and `fill-rule evenodd|nonzero`; `color`
  fills it and `border-color` strokes it, and with neither it takes the inherited text colour.

## 9. Traps

- Two blocks on one line are one command: `item A { ... } item B { ... }` merges them and the second
  name is lost. Put each block on its own line.
- The name of an instance is an alias, never text.
- `padding` does nothing on a leaf without its own `auto-layout`; wrap the text in an item with
  `auto-layout` and `padding`.
- `width fill` and `height fill` need the parent's `auto-layout`: with only the item's own they warn
  and hug, with neither it is an error. `min-width fill` is an error: limits take numbers.
- There is no `text` command: text is `title`.

## 10. Not supported

The engine reads none of these; they are ignored or warned about, so do not write them:
`frame`, `param`, `variant`, `native`, `detach`, `import` (mount a file with `element ...
--from-file`), percentages (`width 50%` becomes `hug` and warns), `justify space-around`,
`aspect-ratio`, `display none` and `line-style` (a scene link reads `display arrow` only), and the
other commands of classic diagrams.

## 11. Examples

A component with parameters, then instances that set values and an `at` override:

```tenn
element "Step" {
    kind Component
    export let title = "Step"
    export let tone = step
    styles {
        box { font-size 14 }
        step { color blue-200 }
        end { color green-200 }
        warn { color red-200 }
        tone one-of step end warn
    }
    item Box { title ${title}; use-style box ${tone} }
}

element "Onboarding" {
    use Step as "Sign up" { title "Sign up" }
    use Step as "Verify email" { title "Verify email"; pos 200 0 }
    use Step as "Create workspace" { title "Create workspace"; pos 413 10; tone end; at Box { height 40 } }
    use Step as "Invite team" { title "Invite team"; pos 420 -120; tone warn }
    link "Sign up" "Verify email" { display arrow }
}
```

Styles, groups and a kid block, with the variants taken through `at`:

```tenn
element "Button" {
    kind Component
    styles {
        primary { color #6b3ad3; text-color #ffffff }
        ghost { color none; border-color none; text-color #6b3ad3 }
        large { height 48; min-width 48; gap 8; padding 0 16; corner-radius 8 }
        small { height 32; min-width 32; gap 4; padding 0 8; corner-radius 6 }
        icon-only { padding 0; Label { hidden yes } }
        kind one-of primary ghost
        size one-of large small
        shape any-of icon-only
    }
    item Btn {
        auto-layout row; gap 8; align center; justify center; clip yes; corner-radius 8
        use-style primary large
        item Icon { width 16; height 16; path "M8 3v10M3 8h10"; view-box 0 0 16 16 }
        item Label { title Button; font-size 14; layout left }
    }
}

element "Catalogue" {
    item Row {
        auto-layout row; gap 16; align center; padding 16
        use Button as "primary large" { at Btn { use-style primary large } }
        use Button as "primary small" { at Btn { use-style primary small } }
        use Button as "icon only" { at Btn { use-style primary large icon-only } }
        use Button as "ghost" { at Btn { use-style ghost large } }
    }
}
```
