---
name: tenniarb-design
description: Draw UI screens (app windows, settings pages, lists, dialogs, mobile screens) and diagrams (brain maps, flows) in Tenniarb .tenn files through the tenniarb MCP tools, reusing components of a library. Use when asked to design, mock up, sketch or change a screen, a component or a diagram in a .tenn file.
---

# Tenniarb design

A `.tenn` file is text. You draw by editing it and you check by rendering it. Every tool works on
paths relative to the workspace root. More files of this skill, `read_skill {path}`:

| File | Read it when |
|---|---|
| `format.md` | before the first screen: commands, properties, text, colors, the Y axis |
| `components.md` | the workspace has a library, or you repeat a part: --from-file, use, slots, at/replace |
| `layout.md` | building a screen: auto-layout and recipes (app window, settings, list, dialog, mobile) |
| `diagrams.md` | drawing a brain map, flow or other diagram with links |
| `review.md` | after every render: what to check, typical mistakes and their fixes |

## The loop

1. `get_state` with no arguments: the files, their top two levels, the renderer of each element,
   errors. Lines end with `+N` when N nodes are cut off below.
2. Library, if there is one (a file with components, often `lib/*.tenn`):
   - `get_state {file, find: "FormRow", depth: 3}` - every node whose name has the word, with its full
     path for `use` and, below it, its parts as `at`/`replace` take them and its slots. One call per
     component you need; `find` without `depth` only lists the paths.
   - `read {file, path: <component>}` - its text, only when you need its size, colors or a layout
     that is not obvious from the parts.
3. Write the screen in one `edit` (compact form, `format.md`): a new file is one edit with empty
   `find` and the whole text in `replace`; in an existing file insert a new element after a line
   you know.
4. Read the result of `edit`: it refuses any change that adds an error and lists the warnings left.
   No separate `validate` is needed after an edit.
   Without MCP (you edit the file directly) run `tenniarb check FILE` after every edit instead: it
   prints `path:line:col: error|warning: message` and exits 1 on errors; fix them before you render.
5. `render {file, element}` and look at the image with `review.md` in mind.
6. Fix with small edits: `find` is one unique line or a few words, `replace` the new text. Render again.
   Stop when the render matches the request; two or three rounds are normal.

## Cheap habits

- Never `read` a whole library or a large file: list it with `get_state` and read one node.
- Use components, do not copy their text: `use lib.Kit.Button as ok { at Label { title OK } }`.
- One `edit` call can carry several find/replace pairs for one file.
- `find` must match exactly once: take the shortest unique text, a name in quotes with its line.
- After a successful edit do not read the file again to check it: the edit result says what you need.
  Read only the lines you are going to change, with `read {file, path}`.
- Render one element. Put screens in their own elements, so a render shows one screen. A desktop
  screen of 1920x1080 reads well at `scale 0.5`, which costs a quarter of the pixels.
- The same override on many instances: one edit with `all: true` (`find` a text all of them share).
- Compact form: two-space indent, short blocks on one line `item a { title Hi; font-size 14 }`,
  bare words without quotes.

## Two things to remember

- **Y axis points up.** `pos 0 -700` is 700 units *below* `pos 0 0`. Screens side by side: `pos 0 0`,
  `pos 1500 0`; a second row: `pos 0 -1000`.
- **Scene or diagram.** An element where an item has children, `auto-layout` or a `use` is a UI scene;
  any other element is a classic diagram (items at `pos`, links, styles). `get_state` shows which.
  Keep screens and diagrams in separate elements.

## When the app is open

If the user has the file open in Tenniarb, `edit` changes the document in the app as one undo step
and the file on disk stays untouched until the user saves. If the user typed since you read, the edit
fails: read the lines again and repeat. `selection` shows what the user selected (dotted paths);
`select {file, nodes, reveal: true}` points at nodes you are talking about. The user sees that an agent
works on the file and can follow your reads, edits and selects in the window, so `select` also shows what you
work on.

## Comments from the user

The user leaves comments on nodes of a design in the app. After you finish a task call `comments {file}`: each
open comment has an `id`, the `text`, the anchor (`link_text` goes to `select`) and the replies. Fix what it asks,
then `reply {file, id, text}` or `resolve {file, id, text}`; the user sees it in the thread at once. To stay on
duty loop `wait_comment {timeout_s}`: it returns the new comments and replies, or `[]` after the timeout, and you
call it again.
