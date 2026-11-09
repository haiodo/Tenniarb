#  Changelog

## Unreleased

* A classic diagram drawn through `go/scene` is read from the picture: the pointer, the quick panel,
  Goto Item and the size of the selection take an item's place from the offset the scene laid it out
  with, and a press, a band or a right click where the scene draws nothing no longer grabs the item the
  old renderer put there. The two renderers measure text differently, so the box `go/render` gave a
  text-sized item can be hundreds of points wider than the drawn one; the input of a classic diagram
  still runs through the session, which now asks the picture before it takes a press.
* gowt 0.15.6: the app builds with `CGO_ENABLED=1 -tags gowtcgo` by default, the native
  AppKit/CoreGraphics calls gowt measures about eight times faster than the purego backends; `CGO=0`
  keeps purego, and the cross-compiled packages stay on it.
* The canvas always draws through `go/scene`, a classic diagram included: the
  `TENNIARB_SCENE_CANVAS=0` rollback and the old `go/render` canvas are gone. Editing a classic diagram
  still runs on the session, which the scene follows.
* `go/jsoncanvas` reads and writes JSON Canvas 1.0 and converts both ways: `text` becomes an item
  with the first line as `title` and the rest as markdown `body`, `file` an `image` or a link item
  outside the workspace, `link` an item with a markdown link, `group` a container, and edges become
  `link` with `label`, `display arrow` and `route elbow` sides. Colours `1`..`6` take the nearest
  `.tenn` shade, and everything the format cannot hold is a warning. `canvas2tenn` and
  `tenn2canvas` in `go/cmd` run the conversion from the shell. The app opens a `.canvas` as a
  document and saves it back from the scene, listing what the format cannot hold before it is lost.

* The inspector is one panel for a classic item and a scene node: Layout, Fill, Text, Border and Shape
  are built from the same rows for both, the scene node gains the `Display`, `Layer` and border `Style`
  rows the classic item always had, and `Padding` reads the same field on either side.
* The Effects section of a scene node lists its `shadow` layers like the classic inspector does: each
  layer's X, Y, blur and colour, a button to drop one and one to add a layer, each write one undo step
  that leaves the file with exactly the layers shown; the `Shadow` row beside it keeps the `soft`,
  `floating` and `custom` choices.
* A classic diagram drawn through `go/scene` draws the `!(colour)` and `&(size)` markup of a title and
  a body instead of its syntax: `!(red|IN)!(green|tegrations)` is two coloured runs, `&(30|x)` makes
  its line taller and wider, `!()` and `&()` return to the item's colour and size, and the width and
  height of the item follow the runs. An unknown colour is a warning once and falls back to the text
  colour (`examples/anti-crm.tenn`, item "Architecture.Integrations").
* The web scene port lays out a classic diagram: `$(...)`/`${...}` come from the element model through
  the same `ExecutionContext` the old item renderer uses, the text inset is 4 and an item with no text
  is the eight-point dot, so the new classic fixtures of the parity stand match the Go golden.
* The web scene port draws the inline pictures of a classic diagram: `image "name" @(...)` is decoded
  and cached, an item nothing names takes the pixels of the picture, and `@(name|width)` in a `title`
  or a `body` draws it in the line, so the new `classic-image` fixture matches the Go golden.
* A classic diagram drawn through `go/scene` keeps its inline pictures: `image "name" @(...)` is
  decoded and cached like a picture file, `@(name)` and `@(name|width)` in a `title` or a `body` draw
  it inline, and an image the text does not name fills the box and sizes it by its pixels
  (`docs/Example.tenn`, item "Main Window").
* The `render` and `layout` MCP tools draw and lay out a classic diagram through `go/scene`, with the
  `$(...)`/`${...}` values of the element model: `layout` no longer refuses it, and its text line says
  "as a scene" like a scene element.
* Every classic diagram of the repository now loads and lays out through `go/scene`: a value the old
  renderer could not read is a warning instead of an error, a size below one falls back to the text,
  an item with no text keeps the eight-point dot `go/render` drew, and a link whose end is missing
  leaves the rest of the file drawn. `TestClassicFilesDrawThroughScene` guards it over
  `examples/`, `docs/` and `web/fixtures/`.
* A classic diagram drawn through `go/scene` keeps the text inset of the old renderer (4 on every
  side unless `padding` says otherwise), and a `title` with more than one argument draws the first, as
  it always has, instead of failing the whole file: the first steps of drawing classic elements with
  the scene engine (`docs/Example.tenn`, `examples/anti-crm.tenn` load and lay out).
* The classic inspector has a `Padding` row, and its Effects section lists every `shadow` layer of the
  item: the layers it writes, or those of the element's `styles { item }` when it writes none, with a
  button to add or drop a layer. The item defaults of the element get the same list, so a soft shadow of
  two layers is editable on either side.
* Choosing a style on a part of an instance replaces the style its `use-style` expression produces, so
  the row shows the choice afterwards; before, the words of the expression were written into the `at`
  block as style names and the row kept reading the value the expression still gave.
* The Shadow row of a scene shows `custom` when the node writes layers of its own, instead of `none`.
* The Code pane keeps the caret while the user types: the session redraws inside the edit it applies,
  before the edit is recorded, so the pane took the draft for a change of the code and put the caret
  back at the start after every keystroke.
* `padding` insets the text of an ordinary item, with or without `auto-layout`: one value for all sides,
  two for vertical and horizontal, four as top right bottom left. A hugging item grows by the padding.
* A drag, a resize, an align and the inspector write coordinates to 1/100 of a point, and saving keeps
  two decimals, so a file stops collecting float noise like `-549.527720208378` from a pointer or from
  the float32 positions the format reads.
* `layout quad` in a scene draws a link as a curve through its `pos` control point, and a link with
  `pos` but no `quad` bends as two straight segments, as a classic diagram does.
* `display` and `layer` work in scenes as they do on a classic diagram: `display stack|circle|no-fill|text|none`
  picks the shape of the box, and `layer background|hover` moves the picture under or over its siblings
  without touching the layout.
* An item that holds items is a box for them, not a label: it draws no name and no `body`, only a `title`
  if one is written, so `title ""` is no longer needed to keep a card unlabelled. A childless item
  outside a layout still shows its name, as a diagram does.
* The inspector writes a soft shadow in one choice: `Shadow` offers `soft` and `floating`, each putting
  two `shadow` commands on the selected scene node as one undo step, and `none` takes them away.
* A `shadow` whose offset is an expression the evaluator cannot resolve is skipped with a warning
  instead of crashing the build.
* "Attach image..." is on the canvas menu of a scene: the picture goes in as `image "path"`, a file
  from the workspace is linked, one from elsewhere is copied into `assets/` beside the document, and
  with nothing selected it becomes a new item.
* `shadow` may be written more than once on an item: every command is its own layer, so a soft shadow
  is a wide faint pass under a tight one instead of a single blur that only gets bigger.
* `link ... { route elbow top right }` draws a connector orthogonally: it leaves and enters the named
  sides with rounded corners, and the links that share a side spread along it, so a hub with satellites
  needs one link per tile instead of hand-placed paths.
* `line-style dashed|dotted` draws the border of a scene item and the line of a `link` dashed, as it does
  on a classic diagram; before, the scene ignored it and every frame was solid.
* Font files of `<config>/fonts` and of every `TENNIARB_FONTS` path are registered for the process at
  start, so a document names its own family (`font-family Manrope`) without a system install.
* MCP `design` and `refactor` refuse changes made from an outdated document snapshot instead of
  overwriting edits made by the user while the operation was being prepared. `refactor` also refuses
  files whose application has moved to a newer service.
* `design` reads unsaved changes in open libraries when resolving components.
* Closing a document stops when its mounted library cannot be saved, preserving the unsaved changes.
* Recovery keeps backup files when restoring a document fails and finds older backups when the newest
  recovery folder is incomplete.
* Save As reloads the scene when the document path changes within the same workspace.

A scene item can draw a raster picture from a file and be turned still: the two things a landing page
rebuilt in `.tenn` needs for its 55 screenshots and its tilted disc.

* `image "assets/photo.png"` on an item or on the root of an element puts a PNG, JPEG, GIF or WebP file
  into the box: `fit fill|cover|contain`, `corner-radius` and `clip` cut it, `opacity` and `hidden` work
  as before. `fit` reads on the same command after the path and as a command of its own. The path is
  relative to the file that writes it and may not leave the workspace; a missing file or an unreadable
  format (AVIF) is a warning and an empty item, so `check` and `validate` still pass. Decoded pictures
  are cached by path, size and mtime in a 512 MiB LRU, so a rebuild does not read them again.
* `rotate -20` on an item turns it by degrees clockwise about the middle of its box, picture or shape
  alike; the scene bounds follow the turned box, the auto-layout does not.
* An element root honours `width` and `height` now: they size the screen, and a root with no size takes
  the size of its picture.
* The classic `image <name> <base64>` of a diagram is unchanged.
* A bare `title` with spaces is an error that names the fix (`title "Sign in"`) instead of drawing only the
  first word.
* `color linear` on a `path` leaf fills the shape with the gradient, as on any other item, instead of the
  `"linear" is not a color` warning.

An agent's edit of a library comes into view where the library is shown, a click stops on the component
instead of falling into its parts, and there is a way to the place a component is defined.

* An edit of a file a window shows as a mounted element - `Libraries.ui_kit`, nested mounts included - is
  revealed in that window with the mounting element selected, instead of opening the file in a second window.
* Go to Component, in the Items menu and on the Component button of the inspector, shows the element that
  defines the component and selects the node it is drawn from.
* A click on an instance selects the component whole; a double click goes one level in, and a click never
  falls into the parts of a component on its own.
* Insert Component inserts into a screen that lives inside a mounted file: the window used to jump to the
  document's own root, as if every mounted ancestor made the screen a library.
* The context menus of an item hold their commands in Add and Move submenus instead of one long list.
* The banner is selectable and has a Copy button, so an error can be pasted elsewhere whole.
* Insert Component lists the components of a library: plates, the caption groups over them and the parts of
  a component are left out, and `kind Component` takes precedence when a library says which nodes are
  components.
* The inspector no longer panics when its scheduled refresh runs after the panel was rebuilt: sections and
  rows that are gone are skipped.
* The Align and Vertical choices of the inspector show none for an axis the item does not write, and
  choosing none clears that word instead of leaving the value the renderer would default to; the scene
  Align, Justify and border Style choices work the same way.
* An agent's edits are written to disk by the autosave within ten seconds of its last edit, instead of
  waiting for the human's Save: a run that dies, or an app that is replaced mid-session, leaves nothing
  only in memory.
* `TENNIARB_PPROF=127.0.0.1:6060` starts the Go pprof HTTP endpoint for CPU, heap and goroutine profiles.
  The listener is off by default. `tenniarb-service` takes the same variable (`127.0.0.1:6061` in
  `make install`), so the process that holds the loaded trees can be asked where its memory went.
* The MCP call log records what a call was asked, not only how big it was: the arguments (first 2000
  bytes), whether the call failed, the client (`opencode`, `codex`) and the workspace root. `make install`
  points it at `~/Library/Logs/Tenniarb/mcp-calls.log` and probes the profiler as well.
* `make install` registers the tenniarb MCP in codex (`make codex-mcp`) and Claude Code (`make claude-mcp`)
as it did in opencode, and writes the design skill into `~/.codex/skills`, where codex reads its own copy.

An agent can move and rename nodes of the whole workspace now, and the picker shows the components of the
libraries rather than the boxes they are drawn on.

* `refactor` renames or moves a node and rewrites every `use`, `at` and `replace` path that names it or a part
  of it, in every file that mounts it: a rename of the library button touched 236 places in 11 files. A move
  refuses where an instance-relative path names the node, and where the node ends a link.
* The component picker previews the chosen component, and the diagram canvas menu offers Insert Component...
  `edit` and `design` take `render` and `scale`, so the picture of what changed comes with the change, and
  `render` draws several elements in one call.
* Insert Component... lists the components of the libraries only, not the plates they sit on or the parts
  inside a component, and Up/Down, PageUp/PageDown and Home/End move the selection while you type the
  filter; the canvas menu calls it Insert Component... too.
* The inspector keeps its width - it cannot be dragged sideways any more - and it is wide enough for its
  widest row: the Fixed/Hug/Fill segments, a pair of fields and a colour value are whole, not cut at the edge.
  A long name, path, list of styles or animation wraps inside the panel instead of scrolling it sideways.
* A node a component can be made from - `kind Component`, or one that declares parameters - has a puzzle
  icon in the Items tree, so a component is not taken for a plain group.
* A design op that reads a node of a mounted file reads it in the context of its entry, so a part of a board
  finds the names of the master document; properties in a `use` block are replaced, not duplicated.
* A node is listed once when two documents mount its file.
* The design skill has the numbers of a desktop screen (type scale, spacing grid, control heights, window and
  panel widths) and the direction step before drawing: audit, direction, then passes in an order that stops
  a screen looking generated.
* The QA driver's reset closes every window and reopens the document the application was started on, and the
  helper windows before the documents, so each test starts from one clean window.

## Version 2.3.0

The tools speak in numbers now: a layout report without a picture, reads bounded by bytes, and edits by node
path. A scene without a rasterizer measures its text, so the same report works where no CoreGraphics is.

* `layout` reports the size of a scene, the box of every item a few levels deep, the items that leave their
  parent or their plate, and overlapping siblings - once each with a count, and `ok:` when nothing leaves.
* `read` is bounded by 12 KB rather than 400 lines and points at the node paths to read by, several nodes in
  one call; a workspace search is bounded by 6 KB, and the whole-file read is capped the same way.
* `design` changes a file by node path: set a property, move by a delta, insert, delete, replace_all. A node
  written in a mounted file is refused there, and the file that writes it is named.
* Reads follow `--from-file` and give the render the computed geometry; an edit of a mounted element writes to
  its own file, props and new items alike.
* The listing of a workspace skips `node_modules` and what `.gitignore` skips, and is cut where an agent stops
  reading, with the name of the call that lists the rest.
* Every tool call is logged by default (`$TMPDIR/tenniarb-mcp-calls.log`, `TENNIARB_MCP_LOG` for another path,
  `off` to disable), so a run is measured by the bytes it returns, not by the agent's report.
* `guide` lists its sections instead of printing the whole reference, and the skill carries the syntax of a
  screen, so a first drawing needs no separate lookup.
* The new-board recipe is written in the order that passes first: an empty file takes the body, then the
  board is mounted, then the body names its entry.
* Group and ungroup the items of a screen: one item the canvas moves, copies and deletes as a whole, and a
  `use` may join a group. Edit > Group (Cmd+G) and Ungroup (Cmd+Shift+G), one undo step each.
* Unsaved work survives the app being told to stop: the documents go to recovery and are offered at the next
  start.
* A mounted library gets a window before an agent edits it, so the work is never out of sight.

## Version 2.2.0

* The skill says what a node inherits - font size, weight and text colour go down the tree, colour, size and
  padding do not - and when a path or a style block paints, so an agent stops painting a wrapper that has
  nothing to paint.
* A press inside the selected item that lands on its child moves the item, as before, instead of selecting
  the child.
* A new item lands where the empty canvas was clicked, and the view keeps its zoom.

## Version 2.1.0

An agent's edit waits for the human now: the window shows it as edited and the file is written only when you
save, so nothing changes on disk behind your back. Switching elements in the outline keeps the zoom, and the
browser editor zooms with two fingers on a touch screen.

+ An edit an agent makes through MCP stays unsaved: one undo step in the window, the title says "Edited", and
  the file on disk is written by Save (or by closing the window with the edits kept), never by the autosave.
+ The outline keeps the zoom as it switches elements, and each element returns to the viewport it was left in;
  only the first element of a document is fitted to the window.
+ Group and ungroup in a screen: the selected items become one item, which the canvas moves, copies and
  deletes as a whole; Ungroup puts them back where they were. Edit > Group (Cmd+G) and Ungroup
  (Cmd+Shift+G), and the canvas menu. One undo step each.
+ A file an agent edits opens in the window: an edit of a file no window shows opens it with Follow on, so
  the work is never out of sight. Settings turns this off with "Open files an agent edits".
+ `get_state` follows `.gitignore` instead of a hard-coded list: the files from the file system root
  down to the workspace say what the walk leaves out, so a project root that also builds a front-end
  still gives its design files, and a `packages/` directory is still searched.
+ `layout` says `ok: nothing leaves its parent or its plate` when it finds nothing: an empty report read
  as "not checked" and cost a run a dozen extra calls.
+ The skill's recipe for a new board is the one that passes on the first try: a minimal file with no
  library names, the mount in the entry file, then the body with `entry` and the library names - both
  of those resolve only through a mount that exists.
+ A file of zero length takes the body in one `edit` with an empty `find`, as a new file does: the
  recipe for a board creates the file, mounts it, then adds the `entry` line.
+ A search across the workspace (`get_state {find}` without a file) stops at 6 KB with the hint to make
  the word longer or to name one file: a word as common as "Button" found a hundred nodes in eight files.
+ `layout` answers with estimated text metrics where the platform has no CoreGraphics, as on the Linux
  and Windows CI, instead of refusing to lay anything out: a scene without a window still has boxes.
+ The QA driver's reset closes every window and reopens the document the application was started on, so
  each test starts from one clean window instead of the pile the tests before it left.
+ The component picker previews what is chosen: the component as the canvas draws it, beside the list,
  and its filter field spans the popover.
+ A component can be inserted from the canvas menu of a diagram: the element becomes a scene, as it does
  for a new screen; an element with links is refused, a scene drawing none.
+ `design` reads a file in the context of its entry document: the names a library gets from the document
  that mounts it resolve, as they do in the canvas, so a `use` path of a part works.
+ A property written in the block of a `use` is replaced where it stands: `design {op: "move"}` moved a
  `use` and left two `pos` lines.
+ `refactor` renames or moves a node and every `use`, `at` and `replace` path that names it, in every
  file that mounts it, as one change: renaming `Libraries.ui_kit.Actions.Button` in the foundation
  library updated 236 places in 11 files, and moving it into `Forms` kept 0 errors.
+ `render` draws several elements in one call (`elements`), and `edit` and `design` take `render` to draw
  an element after the change, so checking a change costs one call instead of two.
+ Renaming an item in the tree shows the field above the row instead of under it: gowt 0.15.3 raises
  the inline editor over the native outline rows on macOS.
+ A search lists a node once even when several documents mount its file: the trees of two documents
  parse that file separately, so the same library was printed under each - 31 repeats for "Avatar".
+ A search of the whole workspace lists a mounted file once, in the document that mounts it, and leaves
  out the files that have no match: 27 lines of "nothing named like" and every duplicate went away.
+ `layout` and `render {inspect}` no longer report a plate as an overlap with what is drawn on it, nor a
  status marker hanging 2 units over the icon it sits on as a problem: both are on purpose, and a run
  spent its calls on them. A component leaving its "§ ..." plate is still reported.
+ `guide` without a topic returns the examples and the list of sections, not the whole 16 KB reference:
  a run asks for the section it needs by number or heading word.
+ `read` stops at 12 KB as well as at 400 lines: a board file can be 700 KB with long lines, and a
  whole-file read of it used to come back as 50 KB of text. Node and mounted-element reads are cut the
  same way, with the hint to read one node.
+ The first call of a run is cheap: `get_state` without a file stops after about 12 KB and points at
  `get_state {file}` and `find`, instead of listing every node of the workspace (107 KB on this tree).
+ Every tool call is logged to a file in the temporary directory (1 MB, then rolled to `.1`;
  `TENNIARB_MCP_LOG` picks another path, `off` silences it), so a run can be measured after the fact.
+ Stopping the app with a signal (a `make install`, a restart, a kill) keeps the unsaved work: every
  window with changes, and every file it mounts that it holds unsaved, goes to
  `~/Library/Application Support/Tenniarb/recovery/<run>/` with a manifest. The next start offers it
  back once and opens each file with the kept text as unsaved changes.
+ An edit of a library a board mounts opens that library in a window first, so the change belongs to a
  document: it has an undo and a save there, and every board that mounts the file shows the same text.
  Before, an agent edit of a library nobody had open went straight to disk, with no draft and no undo.
+ Agents change a file by operations on nodes: `design` takes `set`, `move`, `insert`, `delete` and
  `replace_all`, addresses nodes by the paths `get_state` prints, and applies the whole call as one change
  (one undo step in the window), with nothing written if the result adds an error.
+ The design skill carries the syntax of a screen itself: one file to read before drawing, the reference
  files only for a property it does not list, the library, a full recipe or a diagram. `SKILL.md` was
  6.8 KB and a run read 38.5 KB of skill; now the first screen costs one read of about 11 KB.
+ `layout` answers with a summary by default: the scene, one line per top-level item and the problems
  (`left its parent`, sits on a plate and leaves it, overlaps), each once with a count; `depth 3` asks
  for the box of every item. A 1920×1080 board went from 5 862 bytes to 682.
+ Agents get numbers instead of guesses: `read` of an element with `--from-file` returns the content of the
  mounted file (a path into it returns the node), and `render {inspect: true}` adds the geometry — sizes,
  boxes, the items that leave the plate they sit on, overlapping siblings. `layout` gives the same numbers
  without drawing a picture, and `read` takes several paths in one call.
+ The design skill tells an agent to change a file with `edit`, not with its own file tools: a change made
  behind the app has no undo step and the window cannot show it to you.
* The browser editor, the live demo of the site, zooms with a pinch: two fingers over the canvas, one finger
  still pans it.
* A press inside the item selected in a screen moves it, even where a child covers all of its picture; a
  second click goes into the child, as before.
* Check for Updates: a build from a source checkout no longer compares as a version older than a release, and
  a tag of digits only is not taken for a version.

## Version 2.0.0

The application is written in Go now: one binary for macOS, Windows and Linux, with the same `.tenn` format
and the same calculation engine. The Swift app stays in the repository as history and is no longer shipped.

+ Components and parameters: `kind Component`, `export let`, values set on a `use`, and `${…}` expressions
  evaluated in the component instance.
+ Named styles with `use-style`, groups of styles (`one-of`/`any-of`), and the cascade in declaration order.
+ Slots: a slot declared inside `replace` is filled from the same `use`; a named slot of a component a
  wrapper uses is passed through by its name.
+ Animations: `animation <name> <duration> [easing] [repeat]` with frames; the editor plays them, the SVG
  export keeps `repeat`, a PNG is the first frame.
+ Path segments in double quotes, so a name with punctuation is addressed as written: `at "2 Пространства - пусто"`.
+ `width`/`height` accept an expression that gives a number or `hug`/`fill`; animation frames accept expressions.
+ Export pages: every element of a file as a PNG file, or as a page of one PDF.
+ Interactive HTML export inlines the files a document mounts with `--from-file`: the page draws anywhere,
  not only next to the files it mounts. The browser bundle draws an element whose content is a `use`.
+ Compare with Git: a tree of changed nodes (added, removed, changed, moved) with the old and the new value,
  and Revert to HEAD as a single undo step.
+ `suggest` reports what could be tidied (components from repeats, named styles, dead commands) and applies a
  finding with a preview and one undo.
+ MCP for agents: `get_state` names the kind, parameters, styles, groups and use arguments of a node; `gen`
  prints the text a component template generates; `suggest` reports; a comment on a part a document shows from
  a mounted file belongs to that file.
+ Builds: unsigned packages for macOS on Apple silicon (dmg and app zip), Windows (zip) and Linux (tar.gz); macOS
  asks to allow the app in Security, Windows shows a SmartScreen warning.

### Cross-platform application

+ Native on macOS, Windows and Linux through the `gowt` toolkit.
+ The same outline, inspector, canvas, scene renderer and export as the macOS app.
+ Editor with the calculation engine, styles, components, animations, undo/redo, quick styles, settings,
  Open Recent, autosave, print, and MCP for agents.

## Version 1.4

+ Resize the selected item with the keyboard: Shift+Arrows changes width/height by the grid step, Cmd+Arrows resizes around the center.
+ Export as interactive HTML.
+ Closed hand cursor while panning the diagram, open hand when Space is held over an empty place.
* Smoother resize with the keyboard: only the changed dimension is written, size is taken from the visible bounds.
* Code is moved to Swift 6 with strict concurrency checking, controllers are split into separate files.


### Cross-platform app (experimental)

Tauri 2 app built from the TypeScript port in `web/`, released with the same version number as the macOS app. Reads and writes the same `.tenn` files as the macOS app.

+ Builds for macOS (universal dmg), Windows (NSIS installer) and Linux (AppImage, deb). Builds are unsigned.
+ Editor with the calculation engine, outline, properties panel, quick styles, undo/redo, zoom.
+ Menus and shortcuts of the macOS app, Settings window, Open Recent, autosave, Print.
+ Export to HTML, interactive HTML, PNG, JSON, and PDF through the print dialog.
+ About dialog with name, version, copyright (Help menu on Windows/Linux).
* Experimental: the macOS app stays the reference implementation. Pixel-exact rendering is not a goal, JavaScript in documents runs in the app page (no sandbox), no file versions browser (macOS only).


## Version 1.3.4

* New file icon.


## Version 1.3.3

+ Pinch to zoom on the trackpad.
+ Zoom controls (-, 100%, +) in the title bar, View > Zoom In/Zoom Out/Reset Zoom.
* New application icon.
* Standard system selection in the element outline.
* Quick edit field has a borderless look.
* Window UI is created in code instead of a storyboard.


## Version 1.3.2

* Markdown rendering moved to the cmkdown package.
* Fix a few listener leaks.

## Version 1.3

+ Support Quick Style context menu
+ Allow to define line shadows
- Disable by default popup menu


## Version 1.2

+ Support for corner-radius properly - to control from round corners to flat ones.
+ Support for line-spacing properly - to control text display spacing 
+ Support for Markdown font sizing - &(size|?Text)
+ Ignore links on align operations.
+ Paste as Item/ Paste as Item set actions.
+ Select all items/ Select all link actions.
+ Move only right on duplicate and copy/paste
+ Support apple mouse scroll to drag canvas.
* Use #colors for background in preferences.
+ Support Zooming of diagram canvas.



## Version 1.1.1

* Stack item now looks better with shadows.
* Fix few minor font size glitches.
+ Global styles context menu with some usefull operations.
+ Various size calculation fixes
+ Fix popup display

## Version 1.1

Fixed bugs:
* Element deletion was not used Undo/Redo.
* Support Copy/Paste on element tree outline.
* Auto selection of items after addition, and other operations.
* Fix floating point number rounding.
* Fix link doubling when do duplicate.
* Fix concurency issues with calculation engine operations.

Improvements:
* Improve parsing performance, up to 2x for some files.
* Support of embedded images.
* Support for Markdown highlighting.
    * Bold/Italic/Strike
    * Code style
    * Color enhancements.
* Links now could be drawn with quad curved style.
* Advanced text positioning.
* Context menu enhancements:
    * Context menu always on selected item.
    * Context menu for selection.
    * Move Front/Back.
    * Align modes for edges.

Selection improvements:
* Option + Click, select all items with outgoing relatives.

Editing improvements:
* Cmd+Enter -> edit value field for item.

Breaking changes:
* Positioning of items changes to grow down instead of grow up, so some diagrams could move up/down.

