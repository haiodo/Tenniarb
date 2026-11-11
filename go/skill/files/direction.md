# Direction, and the AI tells

A screen assembled from "what components exist" comes out grey and could belong to any product. This is the
step before drawing: audit what is there, write the direction, then fix in passes. The order is the same for
a new screen and for one already drawn.

## 1. Audit

Name every tell before touching the file: one line each, what and where, with the number you measured.

```
Content.Toolbar gap 2, buttons 8
Card corner-radius 12, button 12, rail 12 - one radius on everything
Empty state 320 x 240 box, no action
```

A line without a number is an opinion; `render` then `layout {depth: 3}` give you the numbers. The tell
table at the end is the list to go through.

## 2. Direction

Five lines in your reply, before the first edit. Take them from the product and its library, never from a
trend.

- the product: what this screen does, for whom, one sentence;
- two neutrals: the surface and the line, from the library's palette or from the product's own colour;
- one accent: the colour of the one action or of the selection, and of nothing else;
- character: two or three concrete words, `calm, exact` or `fast, technical`; clean, modern and premium fit
  every template and say nothing;
- the signature detail: one small thing this product would have and another would not - a keycap beside a
  shortcut, a status dot on a live row, a mono figure in a table.

The direction lives in your reply, not in the file: `//` comments are dropped when the app rewrites a file.
When the palette has to stay in the file, it is one `styles { item { ... } }` block of the element.

## 3. Passes, in this order

Finish a pass across the whole screen before the next one. Spacing first: everything else reads differently
once the rhythm is right. Copy before type and layout, because the length of a label decides its size and
where it sits. Motion last.

| # | Pass | Rule | Property that carries it |
|---|---|---|---|
| 1 | Spacing | one grid, equal things equal, more space between groups than inside one | `gap`, `padding` |
| 2 | Colour | neutrals for surfaces, one accent for the one action; no hue gradient, no coloured glow | `color`, `text-color`, `border-color` |
| 3 | Icons | one set, one size, the colour of their text, one icon column | `path`, `view-box`, `width`, `height` |
| 4 | Copy | the title names the context, labels are short, names and numbers are real | `title`, `body` |
| 5 | Type | a few sizes, two weights, hierarchy from size and colour | `font-size`, `font-weight` |
| 6 | Layout | real window proportions, one primary action, nothing floating in the middle of a column | `auto-layout`, `width`, `align`, `justify` |
| 7 | Details | nested radius, hairline borders, quiet counts | `corner-radius`, `line-width`, `opacity` |
| 8 | Motion | only motion that explains a change | `animation` |

- Text with a width aligns `layout left`; centred text is for one or two short lines.
- A button names its action and its object: `Delete 3 files`, not `OK` or `Submit`.
- Accent under white text reaches 4.5 to 1, or the text is dark.
- An inner `corner-radius` is the outer one minus the padding between them.
- Sizes, gaps and padding cannot animate: a spinner is `rotate` on a path, a pulse is `opacity`.

## 4. When it is already fine

Say so in one line and offer three to five refinements with exact values, then stop. No redesign, no new
palette:

- `gap 10` -> `8`;
- `corner-radius 16` -> `12`;
- a second grey `#9ca3af` -> the muted `#6b7280` the rest of the screen uses;
- one 12 icon in a row of 16 -> `16`;
- a 20 title over a 13 body -> `15 / 600`.

An empty audit is an answer: the file needs no edit. Never change a good screen to make it different.

## 5. The tells

| You see | The tell | Fix in .tenn |
|---|---|---|
| `color linear 90 #6b3ad3 #2563eb` on a header or a card | the default indigo-to-violet gradient | one neutral `color`; the accent on the one action |
| `shadow 0 0 24 #7c3aed55` | a coloured glow standing in for hierarchy | `shadow 0 -2 6 #00000014` on a card at most; depth from `border-color` and a surface step |
| `title "Save ✨"` | an emoji used as an icon | a `path` icon at 16 and a plain `title` |
| every text centred | no `layout left` - a leaf centres by default | `layout left` on text with a width, `align start` in the column |
| `corner-radius` 6, 8, 10, 14, 18 in one screen | a radius per mood | 6 control, 10 window and dialog, 12 card |
| `font-size` 11, 12, 13, 14, 15, 16, 17, 20 | one size per element | the sizes of `platforms.md` |
| title, body and footer of a card all 13 / 400 | a card with no hierarchy | 15 / 600 title, 13 body, 11 caption in `text-color` |
| a toolbar of `title` leaves, no icons | an icon-less row that reads as a paragraph | 16 `path` icons, `gap 8` to the label |
| "Lorem ipsum", "Text", "Item 1" | filler copy | the words the product really shows |
| `title "Welcome to Acme"` | a greeting as a heading | the context: `Inbox`, `October 2026`, `Settings` |
| a 320 x 240 empty box, or one CTA for every cause | an empty state with no action, or the wrong one | a 13 line naming the cause and its own action: create, clear the filter, retry |
| three cards of equal width in a row | the generated feature grid | different widths, or one card and a list |
| "Supercharge", "Seamless", "All in one" | stock copy | what the feature does, in the product's words |

Practices after `heydamir/unslop` (CC BY 4.0, Damir Sokolovsky) and `Manavarya09/design-extract` (MIT); text
written for Tenniarb.
