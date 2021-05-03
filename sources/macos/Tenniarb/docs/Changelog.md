#  Changelog
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

