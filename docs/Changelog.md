#  Changelog

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
+ The first call of a run is cheap: `get_state` without a file stops after about 12 KB and points at
  `get_state {file}` and `find`, instead of listing every node of the workspace (107 KB on this tree).
+ Every tool call is logged to a file in the temporary directory (1 MB, then rolled to `.1`;
  `TENNIARB_MCP_LOG` picks another path, `off` silences it), so a run can be measured after the fact.
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

