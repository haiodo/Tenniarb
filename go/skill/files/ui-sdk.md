---
name: tenniarb-ui-sdk
description: Compose a screen from the UI SDK wireframe kit in this repository, libraries/ui-sdk.tenn, and print it as Svelte 5 markup with `tenniarb generate`. Use when a screen must read as real UI SDK components (Button, Select, Dialog, Listbox) instead of hand-drawn boxes, or when the drawn screen has to become @ui-sdk/svelte code.
---

# The UI SDK wireframe kit

`libraries/ui-sdk.tenn` holds one element per part of [ui-sdk.dev](https://ui-sdk.dev/md/svelte/components):
256 elements, exactly the part names of its Svelte library, grayscale, no behavior. The parts sit under
seven category elements - `Actions`, `Forms`, `Display`, `Overlays`, `Navigation`, `Feedback`, `Media` -
which are groups for the component browser, not components: a use names the category, `use ui.Forms.Select`.
A screen composes the parts the way the ui-sdk docs nest them, and `tenniarb generate` prints the code
their `template`s record.

This page is local to this repository: the kit is a checked-in `.tenn` file, not a package, and ui-sdk.dev
has no skill to install - read its docs for prop names and composition, never for a copy of this kit.

## Mount it

```tenn
element "ui" --from-file "tenniarb-library://ui-sdk"

element "Settings" {
    item Page {
        auto-layout column; gap 16; padding 24; width 720
        use ui.Actions.Button as save { text "Save changes" }
    }
}
```

The path in `--from-file` is relative to the file that mounts; the scheme `tenniarb-library://ui-sdk`
finds `libraries/ui-sdk.tenn` from any workspace. Mount it once, in the entry document or in the screen
itself, never twice.

## Find a part

- The name in the ui-sdk docs is the element name: `SelectContent`, `DialogTitle`, `ToolbarButton`.
- `get_state {file: "libraries/ui-sdk.tenn", find: "Select"}` lists the parts that match.
- `grep 'element "' libraries/ui-sdk.tenn` lists all 256; the category element above a part is the group
  the browser shows it under.
- The composition of a part is in the docs (`Composition` block). Dialog, Menu, Listbox and the other
  roots draw their first level when used empty - see "What a part draws" below.
- Which `@ui-sdk/svelte` entry a part comes from is written in the `template` of that part; the part name
  is the same everywhere, so you rarely need the slug by hand.

## Compose a screen

- A part that holds others declares the slot `content`: fill it with `slot content { ... }`. Nested
  parts follow the docs: `use ui.Overlays.Dialog { slot content { use ui.Overlays.DialogContent { ... } } }`.
- Place parts with ordinary `item` boxes (`auto-layout column`, `gap`, `padding`): the boxes are the
  layout of the wireframe and stay out of the markup. Only the parts become code.
- Every part is grayscale and self-contained: it reads the palette of the file itself, so it looks the
  same anywhere and needs no wrapper.
- Take widths from the kit, not from the docs screenshots: `Select` is 240, `DialogContent` 360,
  `Listbox` 240. They are fixed inside the part: `width` on the instance sizes the group the part hangs
  in, not the part's own box, so a wider part is a change of `libraries/ui-sdk.tenn`.

## The naming contract

The generator maps kit to code by these names, so keep them:

| In the kit | On the instance | In the markup |
|---|---|---|
| `text` | what the part shows | children: `<Button>Save</Button>` |
| `value` | text of a field (`Input`, `Textarea`, `…Input`) | `defaultValue="Acme"` |
| `checked` | `yes` or `no` | `defaultChecked` |
| `disabled`, `open`, `multiple`, `required` | `yes` or `no` | bare attribute, `no` drops it |
| any other `export let` | its value | quoted attribute of the same name |

`value` on an item or a trigger is the option's identity, not a field text, and stays `value="…"`.
Expressions are not markup: a prop written `${...}` is dropped, so pass literal text to the parts you
want in the generated file. Live state is the app's job - the wireframe sets `defaultValue` and
`defaultChecked`, the app adds `bind:value` or `onValueChange` on top.

## Print the code

```bash
cd go && go run ./cmd/tenniarb generate -root .. ../examples/ui-sdk.tenn Settings
```

`FILE ELEMENT` - the element is a dotted path from the file root; `-template` wraps the code in
something else. The `<script>` block with the imports comes from the `template` of the screen element:

```svelte
<script lang="ts">
  import { Button } from "@ui-sdk/svelte/button"
  import { Dialog, DialogContent, DialogTitle } from "@ui-sdk/svelte/dialog"
</script>

<Button>Save changes</Button>
<Dialog>
  <DialogContent>
    <DialogTitle>Delete it?</DialogTitle>
    <Button disabled>Delete</Button>
  </DialogContent>
</Dialog>
```

Paste it into a Svelte 5 app that has `npm install @ui-sdk/svelte @ui-sdk/core`; keep the imports, drop
or restyle the wireframe fields that only exist as a placeholder. The generated markup is a starting
point, not a finished screen: nothing is bound and no event handler is written.

## What a part draws

- Containers (`DialogContent`, `MenuContent`, `Listbox`, `Toolbar`, `TreeItem`, …) draw a box and keep
  a `slot content`; an empty one shows a faint hint line.
- A composition root (`Dialog`, `Menu`, `Combobox`, `Autocomplete`, `Command`, `Tooltip`, …) fills its
  slot with one level of its own composition, so `use ui.Overlays.Dialog { }` already reads as a dialog. Your own
  slot replaces that fallback entirely.
- Parts drawn whole, no slot: `Select`, `Tabs`, `Field`, `FileInput`, `Calendar`, `ColorPicker`,
  `ImageCropper`, `Waterfall`, `Tree`, `Timeline`, `Carousel`, `ScrollArea`, `Resizable`, `Pagination`,
  `SegmentedControl`, `OTPInput`, `NumberInput`, `PasswordInput`, `TokenInput`, `DateInput`,
  `TimeInput`, `Slider`, `Meter`, `Progress`, `Rating`, `Checkbox`, `Switch`, `Radio`, `Avatar`,
  `Menubar`, `Preferences`, `Input`, `Textarea`.
- `FieldLabel`, `FieldDescription`, `FieldError`, `*Title`, `*Description` are one line of text: they
  take `text` and nothing else.

## Limits

- No behavior: no focus, hover, press, scroll, open/close. A dialog is a picture of an open dialog.
- No colors beyond the gray scale and one red for `FieldError`; a themed screen belongs to
  `libraries/native.tenn` or to your own components, not here.
- The kit has no table part (ui-sdk composes tables from app markup): use `Listbox`, `Command` or
  `Waterfall` for tabular content.
- `tenniarb generate` prints the templates of the uses, so a part drawn in a plain item, an icon `path`
  or a `link` is not in the code.
