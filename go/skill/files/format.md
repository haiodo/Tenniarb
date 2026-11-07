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
| `body` | markdown: a string, `%{ ... }` or `{ text ... }` | text under the title; wraps and hugs (see below) |
| `font-size` | number | default 18; inherited by the parts below, so set it once on the box |
| `font-weight` | 100..900, `normal`, `bold` | |
| `font-family` | name | default Helvetica |
| `font-style` | `italic`, `normal` | |
| `letter-spacing`, `line-height`, `line-spacing` | number | |
| `text-color` | color | inherited down the tree; default black, or the contrast of the box (`white` on a dark `color`) |
| `layout` | `left`, `right` | aligns the lines of a leaf with a width; default centered |
| `color` | color, or `linear ANGLE COLOR [POS] COLOR [POS] ...`; `none` is transparent | fill; the gradient angle is CSS: 0 up, 90 right, 180 down; positions 0..1 on every stop or on none |
| `border-color`, `line-width` | color, number | frame; `line-width` alone draws nothing |
| `line-style` | `solid`, `dashed`, `dotted` | the frame of an item and the line of a `link`; default solid |
| `corner-radius` | `N` or `TL TR BR BL` | |
| `shadow` | `DX DY [BLUR [COLOR]]` | drop shadow of the box (of the border without `color`); Y points up, so `shadow 0 -4 8` falls down; blur 5 and black at 1/3 by default. Write it twice for a soft shadow: a wide faint pass and a tight one |
| `clip` | `yes` | children are cut at the box |
| `opacity` | 0..1 | multiplies down the subtree |
| `hidden` | `yes` | not laid out, not drawn |
| `path`, `view-box`, `fill-rule` | SVG path data, 4 numbers, `evenodd` | an icon leaf: `color` fills, `border-color` strokes |
| `image` | path `"assets/photo.png"` | a picture file as the content of the box; `corner-radius` and `clip` cut it |
| `fit` | `fill` (default), `cover`, `contain` | `cover` fills the box and crops the middle; `contain` fits it all inside |
| `rotate` | degrees | static turn clockwise about the center of the box |
| `pos` | X Y | top left corner of a root item; Y points up |
| `width`, `height`, `gap`, `padding`, ... | | auto-layout, see `layout.md` |

**Fonts.** `font-family` names a face of the machine. The files of `<config>/fonts` (on macOS
`~/Library/Application Support/Tenniarb/fonts`) and of every path in `TENNIARB_FONTS` - a file or a
folder, separated as `PATH` is - are registered for the process at start, so a document can ship its
own `.ttf`/`.otf` instead of asking the user to install anything. Web fonts (`.woff2`) are not read.

**Inheritance.** `text-color` and the font (`font-family`, `font-size`, `font-weight`, `font-style`,
`letter-spacing`) come from the nearest box above that sets them; a text colour with no such box is the
contrast to the box's own fill, and an icon with no `color`/`border-color` is filled with the text colour
around it. Everything else stays where it is written: `color`, `border-color`, `line-width`, sizes,
`padding`, `opacity` and the layout do not inherit. `primary { text-color #ffffff }` on a box colours
every text and icon below it; a label or an icon sets its own colour only when it differs.

Colors: `#rrggbb`, `#rrggbbaa`, or a name from the palette (`red`, `blue`, `gray-400`, `green-200`).
Three-digit `#fff` is not a color; for a transparent fill leave `color` out.

An expression `${...}` may stand for any value, a word included: `width ${icon ? 44 : "hug"}` is 44 or
`hug`, and `${...}` works in a `styles` row and in a keyframe too (`components.md`).

## Links

- `link A B { ... }` joins two items of the same element by name; in a screen they are the aliases of
  the items around it (`use kit.Card as card`).
- `display arrow` puts the head at the target; `line-style dashed|dotted`, `line-width` and `color`
  work as on a frame.
- `route elbow [exit-side [entry-side]]` draws the line orthogonally: it leaves and enters the named
  sides (`left`, `right`, `top`, `bottom`), with the corners rounded by `corner-radius` (12 by default).
  Without the sides it uses the ones that face each other, so a straight pair of boxes still reads.
  Links that share a side spread along it, in the order of the boxes they lead to - a hub with six
  satellites is one `link` per tile, not a hand-placed path.
- Without `route` the line is straight between the boxes' edges.

## Text and icons

`path` belongs to the leaf that draws it: the leaf has no box of its own and the path fills it. A wrapper
of path leaves has no path, so `color` or `border-color` on the wrapper paints its box — a filled square
where a shape should be. Colour the leaf that has the `path`, or give the wrapper none.

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

## Pictures

A photo, a screenshot or a logo is an item with `image`: the file is the content of the box, and
`corner-radius`, `clip`, `opacity` and `hidden` work as for any item.

```tenn
item Hero {
  width 1800; height 1335
  image "assets/video-bg.png" fit cover
  corner-radius 24
}
item Row {
  auto-layout row
  gap 16
  item Avatar { width 70; height 70; image "assets/ul.jpg" fit cover; corner-radius 35 }
  item Shot { width 300; height 190; image "shots/tracker.png" fit contain; color #f4f4f5 }
  item Disc { width 120; height 120; image "img/disc.webp" fit contain; rotate -20 }
}
```

- The path is relative to the `.tenn` file that writes the property, as with `element --from-file`, and
  may not leave the workspace root. PNG, JPEG, GIF and WebP are read; a missing file or another format
  (AVIF) is a `check` warning and the item is drawn empty - it is not an error, so build the rest.
- `fit fill` (default) stretches into the box, `cover` fills it and cuts the middle (photos, avatars),
  `contain` fits all of it inside (logos, screenshots with an edge), with empty bands.
- Write it as `image "a.png" fit cover` or as two commands `image "a.png"; fit cover`.
- No `width`/`height` gives the item the size of the file in pixels.
- An item with `image` has no default fill and no border: `color` and `border-color` turn them back on.
- The whole screen can be a picture: `element "Landing" { width 1440; height 900; image "bg.png" fit cover }`.
- `rotate` is degrees clockwise about the middle of the box; it draws only, the layout keeps the upright
  box. Do not use it to fake a layout rotation.

## Text blocks: `body`

A paragraph, a list or a whole piece of documentation goes into **one** item's `body`, not into an
item per line. The body is markdown, it wraps by the item's width and the item's height hugs it.

```tenn
item Doc {
  auto-layout column
  width 640
  gap 12
  item Title { title "Architecture"; font-size 24; font-weight 700 }
  body %{
## Goal
Take the call nobody picked up and **bring it to payment**.

- a hold by an idempotent key
- `booking-gateway` keeps no state
- the queue is PostgreSQL: `SKIP LOCKED`

> the booking system does not change
  }
}
```

- Forms: `body "one line"`, `body %{ ... }` for markdown, `body { text "..." ; font-size 13 }` to
  change the font of one block.
- Text from a markdown file, whole or one section: `body --from-file "docs/plan.md"` and
  `body --from-file "docs/plan.md#Goal"`. The section starts at that heading and ends before the next
  heading of the same or a higher level; the heading itself is kept. The path is relative to the
  `.tenn` file, as with `element --from-file`. A missing file or heading is an error with the list of
  headings the file has. Text edited in the `.md` comes back on Reload, so one section serves several
  screens.
- A table of contents is a property of the item that has the body: `toc 2` (headings down to level 2,
  `toc` alone is 3) draws the list above the text, indented by level, two points smaller than the body.
  A `toc` on an item without its own `body` is a warning: it lists the body of that same item only.
- Every heading gets an anchor from its text: lower case, spaces to `-`, only letters, digits and `-`
  (Cyrillic kept), repeats get `-2`, `-3`. A link `[Goal](plan.md#Goal)` or `[Goal](#Goal)` draws its
  label; `check` and `validate` report a link whose file or heading is not there, with the headings the
  file has.
- Markdown: `#`/`##`/`###` headings, paragraphs, `- `/`* `/`+ ` lists, fenced code, `> ` quotes, and
  inline `**bold**`, `*italic*`, code in backticks and `[label](url)` - a link draws its label, it
  does not navigate.
- The body is drawn under the `title`, 2 points smaller than the item's font; headings are +6/+4/+2
  and bold, code is monospaced on a light background, a quote has a bar on the left.
- The width wraps the text and the height hugs it. **Never set `height` on a text block** and never
  split one piece of text into items with hand-picked heights: one edited sentence then overflows the
  box, and every neighbour moves by hand.
- Unknown markdown (a table, an HTML block) is drawn as written: keep to the list above.

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
- `rotate` turns about the middle of what the item draws. A shape that looks the same turned — a full ring,
  a square — shows no motion at all: a spinner is an open arc (about 270°) of a ring.

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
