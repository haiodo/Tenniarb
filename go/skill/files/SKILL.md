---
name: tenniarb-design
description: Draw UI screens (app windows, settings pages, lists, dialogs, mobile screens) and diagrams (brain maps, flows) in Tenniarb .tenn files through the tenniarb MCP tools, reusing components of a library. Use when asked to design, mock up, sketch or change a screen, a component or a diagram in a .tenn file.
---

# Tenniarb design

A `.tenn` file is text. You draw by editing it and you check by rendering it. Every tool works on
paths relative to the workspace root.

This file is enough to draw a screen: the loop, the syntax you will use and the traps. The rest is
reference, read it only when the task needs that part:

| File | Read it when |
|---|---|
| `format.md` | you need a property this file does not list: gradients, shadows, `body` markdown, `path` icons, animations, compact form, `context`, or what the scene cannot do |
| `platforms.md` | you are picking a font size, gap, padding, row or control height, or a window and panel width, and want the number instead of a guess |
| `direction.md` | before drawing a screen: the audit, the direction and the passes that stop it looking generated |
| `components.md` | the library has the screen you need and you fill its slots; or the file is a library itself |
| `native.md` | the screen should look like a native desktop app: one library, a `Theme` wrapper for macOS/Windows/Linux and light/dark, surfaces, control sizes, capitalization and recipes |
| `layout.md` | you are drawing an app window, a settings page, a list or a dialog and want the full recipe |
| `tools.md` | `design`, `read`, `layout` or `refactor` does something you did not expect |
| `diagrams.md` | drawing a brain map, flow or other diagram with links |
| `review.md` | after the first render, to check what you drew against the usual mistakes |

`guide {topic}` is the command reference the engine runs; ask for one section (`styles`, `3`), not the
whole thing.

## When this skill is for

Drawing or changing a screen, a component or a diagram **in a `.tenn` file**, through these tools.
Not for prose, notes or reports - those are markdown files, not screens - and not for changing the
editor or any code: a `.tenn` is data.

## If / then

- The same card, row or dialog appears twice → make a component in a library and `use` it
  (`components.md`); two copies drift apart on the first change.
- A feature or a flow, not one screen → list its steps and only the states where the screen differs (empty,
  loading, no results, error, success); one element per screen, side by side from `pos 0 0`. The frame
  stays: a state changes the content area, not the window, toolbar and sidebar. A loading state is a
  skeleton of the content or a local indicator, never an invented percentage.
- The content flows along one axis → `auto-layout row|column`; it overlaps or sits at a point → `pos`
  under a parent without auto-layout.
- Two boxes are joined → `link` with `display arrow`; a hub or a C4 picture → `route elbow` (see
  `examples/hub-demo.tenn`), not hand-placed `path` items.
- A raster is content → `image "assets/x.png" fit cover`; an icon → `path` with `view-box`.
- A control height, gap or window width exists → take the number from `platforms.md`, do not invent it.
- The screen should read as a native desktop app → mount `tenniarb-library://native`, wrap the screen
  in `Theme` with `osName` and `theme`, and follow `native.md`; do not mix it with the modern numbers.

## The loop

1. `get_state` with no arguments: the files, their top two levels, the renderer of each element,
   errors. Lines end with `+N` when N nodes are cut off below.
2. Library, if there is one (a file with components, often `lib/*.tenn`):
   - `get_state {find: "Avatar", depth: 3}` without a file searches every file of the workspace: one
     call finds the component wherever it lives. With a file it searches that file only; without
     `depth` it lists just the paths.
   - `read {file, path: <component>}` - its text, only when you need its size, colors or a layout
     that is not obvious from the parts.
3. Write the screen in one `edit` (the syntax below): a new file is one edit with empty `find` and the
   whole text in `replace`; in an existing file insert a new element after a line you know.
4. Read the result of `edit`: it refuses any change that adds an error and lists the warnings left.
   No separate `validate` is needed after an edit.
5. `layout {file, element}` - the scene size, one line per top-level item and the problems (an item
   that leaves its parent, one that sits on a plate and leaves it, overlapping siblings), each once
   with a count. `depth 3` lists the box of every item, when a number is wrong and you need to see why.
6. `render {file, element}` only when how it looks matters: colors, type, spacing. Look at the image
   with `review.md` in mind, fix with small edits, render again. Two or three rounds are normal.

## The syntax of a screen

```tenn
element "Screens" {
  element "Login" {
    item Card {
      auto-layout column
      gap 12
      padding 24
      width 360
      color #ffffff
      corner-radius 12
      item Heading { title "Sign in"; font-size 20; font-weight 700 }
      item Hint { title "Use your work email"; font-size 13; text-color #6b7280 }
      item Go { title "Continue"; font-size 14; text-color #6b3ad3 }
    }
  }
}
```

- `element` is one canvas: put each screen in its own element, then `render` it alone.
- Inside an element: `item` (a box, a text leaf, an icon), `use` (an instance of a component) and
  `link` in diagrams.
- Commands end at a new line or `;`. Bare words for letters, digits and `_ - . #`; quote everything
  else: `title "Sign in"`, `title ""`, `title "2024"`.
- A name becomes a path segment: `item "Primary button"` is `Primary_button`. Paths are case
  sensitive; `get_state` prints a name with punctuation in quotes.

**Boxes and text.** `color`, `border-color` + `line-width`, `corner-radius`, `clip yes`, `opacity`,
`shadow DX DY BLUR`. Text: `title`, `font-size` (default 18), `font-weight`, `text-color`, `layout left`
(align lines inside a width). `font-size`, `font-weight` and `text-color` inherit down the tree;
`color`, sizes, padding and layout do not.

**Auto-layout.** An item with children is a container. `auto-layout row|column` (+ `abs` for free
positions), `gap N`, `padding N` or `padding V H` / `T R B L`, `align start|center|end|stretch`,
`justify start|center|end|space-between`. `width`/`height` take a number, `fill` (take the parent's
room) or `hug` (the content's size, the default). `pos X Y` places a top-level item of the element and
**Y points up**: `pos 0 -100` is 100 units below `pos 0 0`.

**A screen filling the canvas**: one item with `width 1200; height 760; auto-layout column` as the
root, then rows with `width fill` inside it, then your content. Screens are scene elements: an element
whose items have children, `auto-layout` or a `use`. Anything else is a classic diagram.

**Components.** `use ui.Button as ok { title "OK"; kind secondary }` - parameters by name. A component
declares them with `export let`, and a value you set on the instance wins over the default.
`at Label { title "OK" }` overrides a part the component gives no parameter for, `slot name { ... }`
fills a hole. A file that uses a library does not mount it: libraries are mounted once in the entry
document (`PlatformDesign.tenn`), and a file opened alone finds its entry by itself or by a first line
`entry "../../PlatformDesign.tenn"`.

**New file in a library workspace.** Two names in a board need the entry: `entry` itself, and every
`Libraries.…` it uses. A file that nothing mounts can have neither. So, in this order:

1. create the file with a minimal body and no library names: `element "Board" { }`;
2. add `element "Board" --from-file "dev/design/tenn/apps/board.tenn"` to the entry file;
3. write the real body, its first line `entry "../../../PlatformDesign.tenn"`.

Anything else costs a round on `does not mount`, on `"Libraries" is not defined here or above`, or on
`no such file`.

**Styles.** `styles { primary { color #6b3ad3 } kind one-of primary secondary }` inside an element,
then `use-style primary` on a node, or `use-style ${kind}` with a parameter. Any property goes in a
style; the ones a `styles` block lists as `one-of` or `any-of` become the inspector's choice.

**Colors**: `#rrggbb`, `#rrggbbaa` or a palette name (`gray-400`, `blue`, `red`). Three-digit `#fff` is
not a color.

## Cheap habits

- Never `read` a whole library or a large file: list it with `get_state` and read one node.
- One `edit` call can carry several find/replace pairs for one file.
- `find` must match exactly once: take the shortest unique text, a name in quotes with its line.
- After a successful edit do not read the file again: the edit result says what you need. `edit` and
  `design` take `render` (an element path), so the picture comes back with the answer.
- Several elements at once: `render {elements: ["A", "B"]}` draws each as its own image in one call.
- A file that the app has open is rewritten when it opens (indent, quotes, one command per line), so
  text you wrote may no longer match: take `find` from `read`, not from your own edit.
- The same override on many instances: one edit with `all: true` (`find` a text all of them share).
- Render one element. A desktop screen of 1920x1080 reads well at `scale 0.5`, which costs a quarter
  of the pixels. Ask for `layout` before the first render: it finds the overflow that the image would
  only show you.

## Tool quirks

`design` ops, repeated properties, mounted-file reads, inspect geometry and `refactor`: read
`tools.md` when one of them does something you did not expect.

## Two things to remember

- **Y axis points up.** `pos 0 -700` is 700 units *below* `pos 0 0`. Screens side by side: `pos 0 0`,
  `pos 1500 0`; a second row: `pos 0 -1000`.
- **Scene or diagram.** An element where an item has children, `auto-layout` or a `use` is a UI scene;
  any other element is a classic diagram (items at `pos`, links, styles). `get_state` shows which.
  Keep screens and diagrams in separate elements.

## When the app is open

Never write the file with your own tools - Write, Edit, apply_patch, a shell command. The app keeps the
document in memory: a change made behind it lands in no undo step and no follow, and a window with
unsaved edits asks the user to throw them away. Every change, a new file included, goes through `edit`:
it validates the result and writes the file itself when the app has it closed.

If the user has the file open in Tenniarb, `edit` changes the document in the app as one undo step, and
the autosave writes it to disk within about ten seconds of your last edit (the app keeps showing it as
edited until then), so a run of yours cannot stay in memory and be lost. If the user typed since you
read, the edit fails: read the lines again and repeat.
`selection` shows what the user selected (dotted paths) and fails when the app is not connected - it is
not a step of the loop; `select {file, nodes, reveal: true}` points at nodes you are talking about.

## Comments from the user

After a task call `comments {file}`: each open comment has an `id`, `text`, the anchor (`link_text` for
`select`) and replies. Fix it, then `reply {file, id, text}` or `resolve {file, id, text}`; the thread
updates at once. Loop `wait_comment {timeout_s}` to stay on duty: new events, or `[]` after the timeout.

A comment with a `file` field is on a file the document mounts (`element X --from-file`): reply and edit
that file, not this one. Comments live in the app, not in the `.tenn` text: a `//` line is a code
comment, not a note from the user.
