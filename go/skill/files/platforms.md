# Numbers for a desktop screen

Values in points, for a macOS app at 2x: 1512 x 982 is a 14" MacBook screen, Apple's default scaled
resolution. On Windows and Linux: `body 14`, `caption 12`, `title 28`, page margin 12, the rest of these
tables unchanged.

Set `font-size` on every text leaf: the format's default 18 is not one of these sizes.

## Type

| Value | Where it is used |
|---|---|
| 26 / 600 | page title of a welcome or an empty screen; one per screen |
| 22 / 600 | window title, the name of the screen |
| 15 / 600 | section title over a group of rows |
| 13 / 600 | row title, toolbar label, selected sidebar item |
| 13 / 400 | body, field value, list cell, dialog text |
| 12 / 400 | table header, tab bar label, the line under a body |
| 11 / 400 | caption, count, time, hint under a field |

26, 22, 13 and 11 are Apple's Large Title, Title 1, body and caption. Two weights carry a screen: 400 and
600. 700 only on a 22 or 26 title.

## Spacing

| Value | Where it is used |
|---|---|
| 4 | the step: every `gap` and `padding` is a multiple of it |
| 2, 6 | a nudge inside a control - two icon buttons, the space under a caption; nowhere else |
| 4-6 | icon to its label |
| 8 | between two controls of one row, between two buttons of a footer |
| 12 | between the blocks inside a card, the gap of a sidebar |
| 16 | `padding` of a card, the page margin of a window |
| 24 | between the sections of a page, `padding` of a wide dialog |
| 20 | `padding` of a narrow dialog |

Space inside a group is smaller than space between groups, or the groups read as one.

## Controls

| Value | Where it is used |
|---|---|
| 28 | row of a list, a table or a sidebar |
| 28 | height of a button, a text field, a combo |
| 16 | icon of a row, of a toolbar item |
| 12 | the small icon of a chrome button |
| 68 | label column of an inspector or settings row, before the control |
| 6 | `corner-radius` of a button, an input, a row |
| 10 | `corner-radius` of a window, of a dialog |
| 12 | `corner-radius` of a card |
| 44 x 44 | the smallest touch target - Apple's iOS minimum, phone screens only |

The row, control and panel numbers are the app's own (`go/app/window.go`, `go/app/inspector.go`). `layout.md`
draws its recipes roomier, 14 body and 44 rows; for a screen that must look like the app, this table wins,
and a form page may use 32 for its controls - one height per screen, not both.

## Windows and panels

| Value | Where it is used |
|---|---|
| 1180 x 760 | the usual app window |
| 820 x 440 | the smallest window that still works - the app's minimum |
| 1512 x 982 | the widest screen worth drawing |
| 220 | sidebar and inspector column - the app gives both the sidebar width |
| 200 | height of the bottom panel |
| 720 | width of a settings page |
| 400-560 | width of a dialog |

## What breaks the look

- two sizes of text in one row: hierarchy is `font-weight` and `text-color`, not a point.
- an icon at 21: 12 or 16 only, a stray size looks blurred beside the text.
- 11 for a row title: 11 is a caption, a row is 13.
- a 44 row in a list of 28 rows.
- `padding 16` on a card and `padding 12` on its child: one padding per card.
- the 18 default left on a leaf.
- `corner-radius 12` on a divider: a 1 point line takes `corner-radius 0`.

Numbers from this repo's app (`go/app`) and Apple's Human Interface Guidelines, paraphrased; text written
for Tenniarb.
