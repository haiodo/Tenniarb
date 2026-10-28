# Auto-layout and screen recipes

## The model

`auto-layout row` lays children left to right, `column` top to bottom, `abs` leaves them at their `pos`.
The properties of the container:

| Property | Values | Meaning |
|---|---|---|
| `gap` | number | space between children |
| `padding` | `A`, `V H` or `T R B L` | inner space; three numbers are an error |
| `align` | `start`, `center`, `end` | cross axis: vertical in a row, horizontal in a column |
| `justify` | `start`, `center`, `end`, `space-between` | main axis; there is no `space-around` |
| `wrap` | `yes` | a row breaks into lines by its width; `line-gap` between lines |

Sizes of any child of auto-layout (and of the container itself):

| `width` / `height` | Meaning |
|---|---|
| (none) or `hug` | as big as the content |
| a number | fixed |
| `fill` | the rest of the parent's main axis, or all of its cross axis |

`min-width`, `max-width`, `min-height`, `max-height` bound a size; `absolute yes` takes a child out of
the flow, placed by `pos X Y` from the top left of the parent (Y up: `pos 8 -8` is 8 right, 8 down).

Rules that save a round of fixes:

- `fill` needs a parent that knows its size: a `fill` child in a `hug` parent shrinks to nothing.
  Give the screen root a fixed `width` and `height`, then `fill` downwards.
- A spacer pushes the rest to the end: `item grow { width fill }` in a row.
- Lines and dividers are leaves: `item line { width fill; height 1; color #e5e7eb }`.
- A border is a box around the item: there are no one-sided borders, use a divider leaf.
- Children paint over their parent's border; put the border on the child or leave a `padding` of 1.
- `hug`, `fill`, `gap`, `padding` mean nothing outside auto-layout: there they are an error or a plain
  user property.
- `padding` works only with `auto-layout` on the same item. On a text leaf (`title`, no children) without its
  own `auto-layout` it does nothing and `check` warns. Text has no padding of its own: `width 80` is the text box.
  To inset text, use `item Pill { auto-layout row; padding 4 8; item t { title Text } }` or give the leaf
  `auto-layout row` and `padding`.
- Spaces at the start and end of `title` are kept and widen a `hug` leaf. Browsers collapse them in SVG unless the
  document has `xml:space="preserve"` (`tenn-render` writes it).

## App window: sidebar, content, inspector

```tenn
element "App window" {
  item Window {
    auto-layout row
    width 1200
    height 760
    color #ffffff
    border-color #d1d5db
    corner-radius 10
    clip yes
    item Sidebar {
      auto-layout column
      width 232
      height fill
      padding 12
      gap 2
      color #f3f4f6
      item Brand { title Acme; font-size 15; font-weight 700 }
      item Gap { height 12 }
      item Active {
        auto-layout row
        width fill
        padding 6 10
        corner-radius 6
        color #e0e7ff
        item t { title Inbox; font-size 14; font-weight 600 }
      }
      item Other {
        auto-layout row
        width fill
        padding 6 10
        item t { title Projects; font-size 14 }
      }
    }
    item Content {
      auto-layout column
      width fill
      height fill
      padding 20 24
      gap 16
      item Toolbar {
        auto-layout row
        width fill
        align center
        gap 8
        item Heading { title Inbox; font-size 20; font-weight 700 }
        item grow { width fill }
        item New {
          auto-layout row
          padding 6 12
          corner-radius 6
          color #2563eb
          item t { title "New issue"; font-size 13; text-color #ffffff }
        }
      }
      item Body { title "Content goes here"; width fill; font-size 14; layout left; text-color #6b7280 }
    }
    item Inspector {
      auto-layout column
      width 280
      height fill
      padding 16
      gap 12
      border-color #e5e7eb
      item Heading { title Properties; font-size 13; font-weight 600 }
      item Field { title "Status: Open"; font-size 13 }
    }
  }
}
```

## Settings page

Label column of fixed width, control column that fills; sections divided by a line.

```tenn
element "Settings page" {
  item Page {
    auto-layout column
    width 720
    padding 32
    gap 20
    color #ffffff
    item Title { title Settings; font-size 24; font-weight 700 }
    item Section { title Profile; font-size 15; font-weight 600 }
    item NameRow {
      auto-layout row
      width fill
      gap 16
      align center
      item Label { title "Display name"; width 200; font-size 14; layout left }
      item Input {
        auto-layout row
        width fill
        height 36
        padding 0 10
        align center
        corner-radius 6
        border-color #d1d5db
        item v { title "Ada Lovelace"; font-size 14 }
      }
    }
    item ToggleRow {
      auto-layout row
      width fill
      gap 16
      align center
      item Label { title "Email notifications"; width 200; font-size 14; layout left }
      item Toggle {
        auto-layout row
        width 36
        height 20
        padding 2
        justify end
        corner-radius 10
        color #2563eb
        item knob { width 16; height 16; corner-radius 8; color #ffffff }
      }
    }
    item line { width fill; height 1; color #e5e7eb }
    item Actions {
      auto-layout row
      width fill
      gap 8
      justify end
      item Cancel {
        auto-layout row
        padding 8 14
        corner-radius 6
        border-color #d1d5db
        item t { title Cancel; font-size 14 }
      }
      item Save {
        auto-layout row
        padding 8 14
        corner-radius 6
        color #2563eb
        item t { title Save; font-size 14; text-color #ffffff }
      }
    }
  }
}
```

## List and table

A table is a column of rows with the same fixed cell widths; one cell may `fill`. Write the row once as
a component and `use` it per line (see `components.md`) when there are more than three rows.

```tenn
element "Issue table" {
  item Row {
    auto-layout row
    width 640
    height 36
    padding 0 12
    gap 12
    align center
    item Id { title TSK-1; width 64; font-size 13; layout left; text-color #6b7280 }
    item Name { title "Row title"; width fill; font-size 13; layout left }
    item Status { title Open; width 80; font-size 13; layout left }
  }
  item Table {
    pos 0 -60
    auto-layout column
    width 640
    color #ffffff
    border-color #e5e7eb
    corner-radius 8
    clip yes
    item Head {
      auto-layout row
      width fill
      height 32
      padding 0 12
      gap 12
      align center
      color #f9fafb
      item Id { title ID; width 64; font-size 12; font-weight 600; layout left }
      item Name { title Title; width fill; font-size 12; font-weight 600; layout left }
      item Status { title Status; width 80; font-size 12; font-weight 600; layout left }
    }
    use Row as r1 { width fill; at Id { title TSK-1 }; at Name { title "Fix login redirect" } }
    item l1 { width fill; height 1; color #f3f4f6 }
    use Row as r2 { width fill; at Id { title TSK-2 }; at Name { title "Dark mode for settings" }; at Status { title Done } }
  }
}
```

The `Row` component sits in the same element here, so the render shows it too; in a real file it
lives in a library element (`components.md`).

## Dialog over a screen

A scrim the size of the screen centers the dialog.

```tenn
element "Delete dialog" {
  item Scrim {
    auto-layout column
    width 800
    height 500
    align center
    justify center
    color #0f172a66
    item Dialog {
      auto-layout column
      width 400
      padding 20
      gap 12
      corner-radius 10
      color #ffffff
      item Title { title "Delete project?"; font-size 17; font-weight 700 }
      item Text { title "The project and its 24 issues will be deleted. This cannot be undone."; width fill; font-size 14; layout left; text-color #4b5563 }
      item Buttons {
        auto-layout row
        width fill
        gap 8
        justify end
        item Cancel {
          auto-layout row
          padding 8 14
          corner-radius 6
          border-color #d1d5db
          item t { title Cancel; font-size 14 }
        }
        item Delete {
          auto-layout row
          padding 8 14
          corner-radius 6
          color #dc2626
          item t { title Delete; font-size 14; text-color #ffffff }
        }
      }
    }
  }
}
```

## Mobile screen

390 by 844, a header, a list that takes the rest, a tab bar at the bottom.

```tenn
element "Mobile inbox" {
  item Phone {
    auto-layout column
    width 390
    height 844
    color #ffffff
    corner-radius 40
    clip yes
    item Status { auto-layout row; width fill; height 44; padding 0 24; align center; item time { title "9:41"; font-size 15; font-weight 600 } }
    item Header { auto-layout row; width fill; padding 8 20; item h { title Inbox; font-size 28; font-weight 700 } }
    item List {
      auto-layout column
      width fill
      height fill
      padding 0 20
      item m1 {
        auto-layout column
        width fill
        padding 12 0
        gap 2
        item from { title "Grace Hopper"; font-size 16; font-weight 600 }
        item text { title "The build is green again"; font-size 14; text-color #6b7280 }
      }
      item l1 { width fill; height 1; color #e5e7eb }
      item m2 {
        auto-layout column
        width fill
        padding 12 0
        gap 2
        item from { title "Alan Turing"; font-size 16; font-weight 600 }
        item text { title "Review of the parser"; font-size 14; text-color #6b7280 }
      }
    }
    item Tabs {
      auto-layout row
      width fill
      height 83
      padding 10 0 0 0
      justify space-between
      color #f9fafb
      item t1 { title Inbox; width fill; font-size 11; font-weight 600; text-color #2563eb }
      item t2 { title Search; width fill; font-size 11; text-color #6b7280 }
      item t3 { title Profile; width fill; font-size 11; text-color #6b7280 }
    }
  }
}
```
