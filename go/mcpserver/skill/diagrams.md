# Diagrams: brain maps and flows

An element is drawn as a classic diagram when none of its items has children, `auto-layout` or a
`use`. Then it is drawn the way the Tenniarb canvas draws it: every item at its `pos`, a box with its
name, links between items, styles, and `${...}` expressions evaluated. Never add `auto-layout` or a child
item to a diagram: the element turns into a UI scene and loses its look.

```tenn
element "Release plan" {
  styles {
    item { font-size 14 }
    done { color green-200 }
    risk { color red-200 }
  }
  item "Release 2.0" {
    pos 0 0
    font-size 20
    color blue-200
  }
  item Design {
    pos -240 -120
    use-style done
  }
  item Backend {
    pos 0 -120
    body "API and storage"
  }
  item Mobile {
    pos 240 -120
    use-style risk
  }
  item QA {
    pos 0 -260
    display circle
    width 90
    height 90
  }
  link "Release 2.0" Design { display arrow }
  link "Release 2.0" Backend { display arrow }
  link "Release 2.0" Mobile { display arrow }
  link Backend QA { display arrow; line-style dashed }
  link Mobile QA { display arrow; layout quad }
}
```

## Items

- The name is the text; `title` replaces it, `body "..."` (or `body %{ multi-line }`) adds text under it.
- `pos X Y`: Y points up, so children of a map go to negative Y. Leave about 200 units between columns
  and 100 between rows; the box grows to fit its text.
- `color` (fill; text turns white or black to contrast), `text-color`, `font-size`, `width`, `height`,
  `corner-radius`, `line-width`, `line-style dashed|dotted`.
- `display`: `rect` (default), `no-fill`, `circle` (set `width` and `height`), `stack`, `text` (no box,
  for headings and notes).
- Colors: `#rrggbb` or a palette name with a shade, `red-200`, `gray-400`, `blue-500`.

## Links

- `link "From" "To" { ... }` joins two items of the same element by name.
- `display arrow` (to the target) or `arrows` (both ends); `line-style dashed|dotted`; `line-width 2`;
  `layout quad` for a curve; `color`.
- Two items with one name: the link picks the first; `source-index 1` / `target-index 1` pick the second.

## Styles

`styles { item { ... } line { ... } name { ... } }` in the element: `item` and `line` apply to every
item and link, a named style to the items with `use-style name`.

## Values

`${...}` is JavaScript evaluated per element: `value 3` and `title "Open: ${value}"`,
`title "Items: ${parent.items.length}"`. A render shows the result.

```tenn
element "Budget" {
  item Design {
    pos 0 0
    value 1200
    title "Design: ${value}"
  }
  item Build {
    pos 200 0
    value 3400
    title "Build: ${value}"
  }
  item Total {
    pos 100 -120
    display text
    font-size 18
    title "Total items: ${parent.items.length}"
  }
  link Design Total
  link Build Total
}
```
