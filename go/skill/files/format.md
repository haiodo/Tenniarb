# The .tenn format for screens

The reference is `guide {topic}` (a section such as `3` or `styles`); this is the part you need to draw.

## Commands

A command is a name, its arguments and an optional block: `item Card { color #ffffff }`.
Commands are separated by a new line or `;`. The `{` stays on the line of its command.

- Bare words are fine for letters, digits and `_ - . #`: `title Settings`, `color #2f6bff`.
- Quote everything else: spaces, an empty string, a word that looks like a number:
  `title "Sign in"`, `title ""`, `title "2024"`.
- `//` comments are allowed, but the app drops them when it rewrites a file: keep nothing in them.

## Structure

```tenn
element "Screens" {
  element "Login" {
    item Card {
      auto-layout column
      gap 8
      padding 16
      color #ffffff
      corner-radius 8
      item Heading { title "Sign in"; font-size 20; font-weight 700 }
      item Hint { title "Use your work email"; font-size 13; text-color #6b7280 }
    }
  }
}
```

- `element` is the tree of the document: one element is one canvas. Put each screen in its own element.
- Inside an element: `item`, `use` (an instance of a component) and `link`.
- An `item` with children or `auto-layout` is a container; an item without children is a leaf:
  text, a box or an icon.
- Names: `item "Primary button"` has the path segment `Primary_button` (every character that is not a
  letter or a digit becomes `_`). Paths are case-sensitive: `Screens.Login.Card.Heading`.
  A name can also be written in double quotes, as it is: `Screens.Login."2 Пространства - пусто"`;
  `get_state` prints such names that way. Names may repeat, but a path that matches two nodes is an
  error.

An element can take its content from another file: `element "kit" --from-file "lib/kit.tenn"`. The line
has no block; the elements and items of that file become the content of `kit`, and `use kit.Card as c`
reaches them (`components.md`).

Libraries are mounted once, in the entry document of the workspace (for example `PlatformDesign.tenn`).
A name inside a mounted file is looked up in that file first, then at the place where it is mounted, up
to the entry, as with `#include`: app and category files (`apps/tracker.tenn`) hold no library mounts
and just write `use ui.Button as b`. A file opened alone finds its entry by itself, or by a first line
`entry "../PlatformDesign.tenn"`.

## Inside auto-layout

An item whose parent or itself has `auto-layout` is in a layout context. There:

- the name is **not** shown; text is drawn only with `title`;
- there is **no fill and no border** until you set `color` or `border-color`;
- text has no padding and no hidden inset: an explicit `width` is the text box, so `width 80` wraps text wider than 80;
  the container's `padding` gives the space. `padding` on a leaf without its own `auto-layout` does nothing and
  `check` warns; wrap the text in an item with `auto-layout` and `padding`;
- spaces at the start and end of a `title` are kept and count in the `hug` width (`title "    indented"`).

Outside auto-layout (classic diagrams) an item shows its name and has a white box with a black frame.

## Properties

| Property | Values | Notes |
|---|---|---|
| `title` | text | the text of a leaf |
| `font-size` | number | default 18; inherited by the parts below, so set it once on the box |
| `font-weight` | 100..900, `normal`, `bold` | |
| `font-family` | name | default Helvetica |
| `font-style` | `italic`, `normal` | |
| `letter-spacing`, `line-height`, `line-spacing` | number | |
| `text-color` | color | inherited down the tree; default black, or the contrast of the box (`white` on a dark `color`) |
| `layout` | `left`, `right` | aligns the lines of a leaf with a width; default centered |
| `color` | color, or `linear ANGLE COLOR [POS] COLOR [POS] ...`; `none` is transparent | fill; the gradient angle is CSS: 0 up, 90 right, 180 down; positions 0..1 on every stop or on none |
| `border-color`, `line-width` | color, number | frame; `line-width` alone draws nothing |
| `corner-radius` | `N` or `TL TR BR BL` | |
| `shadow` | `DX DY [BLUR [COLOR]]` | drop shadow of the box (of the border without `color`); Y points up, so `shadow 0 -4 8` falls down; blur 5 and black at 1/3 by default |
| `clip` | `yes` | children are cut at the box |
| `opacity` | 0..1 | multiplies down the subtree |
| `hidden` | `yes` | not laid out, not drawn |
| `path`, `view-box`, `fill-rule` | SVG path data, 4 numbers, `evenodd` | an icon leaf: `color` fills, `border-color` strokes |
| `pos` | X Y | top left corner of a root item; Y points up |
| `width`, `height`, `gap`, `padding`, ... | | auto-layout, see `layout.md` |

Colors: `#rrggbb`, `#rrggbbaa`, or a name from the palette (`red`, `blue`, `gray-400`, `green-200`).
Three-digit `#fff` is not a color; for a transparent fill leave `color` out.

## Text and icons

```tenn
element "Bits" {
  item Row {
    auto-layout row
    gap 8
    align center
    padding 8
    item Star { width 16; height 16; view-box 0 0 24 24; path "M12 2l3 7h7l-6 5 2 8-6-4-6 4 2-8-6-5h7z"; color #f59e0b }
    item Label { title "Starred"; font-size 14; font-weight 600 }
    item Note { title "A longer note wraps inside its width"; width 120; font-size 12; layout left; text-color #6b7280 }
  }
}
```

A leaf with a fixed or `fill` width wraps its text at spaces; without a width it is one line.

## The Y axis

`pos X Y` places the top left corner of a root item of an element, and Y grows **upward**: `pos 0 -400`
is 400 units below `pos 0 0`. Children of auto-layout need no `pos`; an `absolute yes` child takes
`pos X Y` from the top left of its parent, so 8 units down is `pos 8 -8`.

## Animations

An item can spin, pulse or fade: the keyframes live in the element that uses them, the item names one.

```tenn
animation spin {
  from { rotate 0 }
  to { rotate 360 }
}
item Spinner { width 28; height 28; path "M14 2a12 12 0 1 0 12 12"; border-color #6b3ad3; animation spin 1.2s linear infinite }
```

- `animation <name> <duration> [easing] [repeat]` on an item or in a style line; the duration is `300ms` or
  `1s`, the easing `linear|ease|ease-in|ease-out|ease-in-out`, the repeat a count or `infinite`.
- A frame is `from`/`to` or `0%`/`50%`/`100%`, and sets `rotate`, `scale`, `opacity`, `color`,
  `border-color`, `text-color` or `pos`. Sizes, gaps and padding cannot animate: they warn and stay.
- One block per line. A nested element (a component inside another) does not see the keyframes of its
  parent: give it its own copy.
- The editor plays every animation in a loop; an exported SVG keeps the repeat as written; a PNG is the
  first frame.

## Compact form

Files made for agents use two spaces per level and one-line blocks of plain commands:

```tenn
element "Compact" {
  item Button {
    auto-layout row
    padding 8 16
    corner-radius 6
    color #2f6bff
    item Label { title Save; font-size 14; text-color #ffffff }
  }
}
```

A block with child blocks has one command per line; a block of plain commands goes on one line
while it stays under 160 characters.
