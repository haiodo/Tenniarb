#  Changelog

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
+ Compare with Git: a tree of changed nodes (added, removed, changed, moved) with the old and the new value,
  and Revert to HEAD as a single undo step.
+ `suggest` reports what could be tidied (components from repeats, named styles, dead commands) and applies a
  finding with a preview and one undo.
+ MCP for agents: `get_state` names the kind, parameters, styles, groups and use arguments of a node; `gen`
  prints the text a component template generates; `suggest` reports; a comment on a part a document shows from
  a mounted file belongs to that file.
+ Builds: unsigned packages for macOS (universal dmg and app zip), Windows (zip) and Linux (tar.gz); macOS
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

