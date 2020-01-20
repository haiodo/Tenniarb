#  Changelog

## Version 1.1

Fixed bugs:
* Element deletion was not used Undo/Redo.
* Support Copy/Paste on element tree outline.
* Auto selection of items after addition, and other operations.
* Fix floating point number rounding.
* Fix link doubling when do duplicate.

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

Breaking changes:
* Positioning of items changes to grow down instead of grow up, so some diagrams could move up/down.

