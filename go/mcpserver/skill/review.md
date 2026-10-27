# Reviewing a render

`render {file, element}` returns a PNG and a text line `rendered ... as a scene: W x H units` plus the
warnings of that element. Check the text first: a size you did not expect (a screen of 1200x760 that
renders 1200x31) already tells what is wrong.

## What to look at

1. **Size and frame.** The image is the bounding box of the element: one screen per element, or the
   image shows everything at once.
2. **Every text visible.** A box without text in a layout context needs `title` on a leaf: the name is
   not drawn there.
3. **Nothing collapsed.** A missing bar, input or divider is usually `fill` inside a `hug` parent.
4. **Alignment.** Rows: `align center` for icon + label; columns of a table: the same fixed widths in
   the header and in every row.
5. **Spacing.** One `gap` per container instead of spacer items; `padding` on the container, not on
   the text.
6. **Overflow.** Text that runs out of its box needs a width (it wraps then) or `clip yes` on the box.
7. **Instances.** Overrides reached the right part: if a label still shows the component's text, the
   `at` path names another node; `get_state {file, path: <component>, depth: 3}` shows the parts.

## Typical mistakes

| You see | Cause | Fix |
|---|---|---|
| a screen drawn upside down, items above instead of below | Y points up | `pos 0 -400` for below |
| the image is smaller than the screen, its top cut | the root has no fill: the image covers only what is drawn | `color #ffffff` on the screen root or in its `use` block |
| text cut at the end of a cell | the cell of the component has a fixed width and `clip` | `at <cell> { width 60 }` |
| an empty box where text should be | no `title`, or only a name | `item t { title Save; font-size 14 }` |
| an invisible box | no fill by default in layout | `color #ffffff` or `border-color` |
| huge text | default `font-size` is 18 | set `font-size` on every text |
| a bar of zero width | `width fill` in a `hug` parent | give the parent a width or `width fill` |
| the frame of a container is hidden | filled children paint over it | `padding 1` or a frame on the child |
| `"#fff" is not a color` | three hex digits | `#ffffff` |
| `color: "#00000000" is not a color` | fully transparent fill written as color | leave `color` out |
| `nothing named "X" inside the instance` | `at` path is not a part of the component | list its parts with `get_state` |
| `names a slot, not a node; fill it with slot ...` | `at` used to fill a slot | `slot name { ... }` |
| `nodes passed, but the component has no default slot` | items written in a `use` block | fill a named slot or use `at`/`replace` |
| `unknown slot "x" (slots: ...)` | the slot name is wrong | take one from the list |
| `use needs the form: use <path> as <name>` | `as` missing | add `as name` |
| `padding takes 1, 2 or 4 numbers` | three numbers | one, two or four numbers |
| `width fill needs auto-layout on the item or on its parent` | `hug`/`fill` outside auto-layout | add `auto-layout` to the item or its parent |
| a diagram suddenly looks like a mock-up | an item with children or `auto-layout` in a diagram element | move it to its own element |

## Fixing

Fix one cause at a time with a small `edit` and render again. When the same mistake repeats in many
places, change the component or the style once, not every instance.
