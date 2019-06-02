#  Tenniarb - General information

Tenniarb is a diagram/modeling tool, capable of doing simple and complex diagraming, easy organize them into categories. Easy create and style them, and as benetit it has a powerfull JavaScript calculation engine embedded, so styling and diagraming become a real powerfull solution for any kind of fast modeling and prototyping.

Every item on diagram is an object with set of properties, some properties have influence on how item is displayed on diagram. Some properties are user defined data, properties could depend on calculations and other items and properties.

In general every item on diagram could be treated as a cell in electronic table, but it allow not just one formala to be used but to define any set of formal and properties to be used.


## First steps

Tenniarb is multi window application, every window manage it own file on file system. Every file has plain text structure.

Simple diagram content will look like:

```
element "Simple" {
    item "Central" {
        font-size 30
        marker "🎁"
    }
}
```

Where `element` is describing a logic structure of diagram layers embedded one into another and `item` show individual items on diagram. Syntax is pretty easy to read and write by hands, it will be described in details in Syntax Section of this document. Language used to manage content of all element, items inside file are named Tenn. It was based on well known and old Tool Command Language(TCL) and have a clean and easy to use syntax.

### Main screen

Main window:
![](./Images/main_screen.png)

Main window has following structural components:

* Document logical structure:

    It allows to organize diagrams, and perform logical structuration with basic operations of drag & drop, duplication and naming.

* Diagram with items.

    Support organization of items with links, styles for visual representation of ideas, structures and any kind of information required.

* Properties pane.

    A textual representation of selected diagram, item with proeprties managed by Tenn language.

### Creating and managing elements.

There is few ways to create elements.

![](./Images/elements_menu.png)

* Using (+), (-) buttons oon top of logical structure, will add child item to selected top level item or delete child item. All this operations are support undo & redo.

* Drag & Drop - could be used to organize items.

* Duplicate - could be used for duplicate selected layer of diageam for perform some changes and see differences.


### Creating and managing items on diagram layer.

On every selected layer items could be managed using items main panel. After layer is selected we could use selection, modification and addition of new items.

Adding new items to diagram layer:

* Pressing 'Tab' key on keyboard will add new item and link it with selected item.
  ![](./Images/add_new_item.png)
* Pressing 'Option + Tab' key on keyboard will add new item, link it with selected item and copy styles from selected item.
  ![](./Images/add_item_copy_style.png)
* Pressing 'Command + D' key on keyboard will duplicate selected item and it incoming links, it could be used to easy create more linked items for brain maps.
  ![](./Images/duplicate_item.png)
* Clicking '(+)' button on top of items layer, will add item item and link it with selected item.
* Using context menu:
    ![](./Images/context_menu.png)
    * New item - Add new top level item.
    * New linked item - Add new linked item to selected one.
    * Linked styled item - Add new linked item with copy of styles to selected one.
    * Style - use or define new style.
    * Duplicate - dupliate selected item with incoming links.
    * Delete - delete current selected items.

* Styling items:
    ![](./Images/styling_items.png)
    * Popup toolbar could be used for fast apply for basic display, color, font size, line width and marker fields.
    ![](./Images/diagram_quick_styles.png)
    * All styles could be editing using textual representation in Tell language format. More details will be in Styling secrtion of this document.
    ![](./Images/textual_styles.png)


# Tenniarb - Detailed specifications.

## Tenn language reference.

Language is based on well known and easy to use TCL(Tool command language) it has concept of command and arguments. Argument in '{' '}' are also treated as commands.

```
item "my item" {
    cmd1 1
    cmd2 1.1
}
```

So item is command with 2 arguments, String argument "my item" and block of commands argument.

Types of arguemnts:

* Identifier - a word without spaces containing characters/digits and set of special characters '-_.'. Command names are identifiers.
* String - a word between string start/end symbopls " or ' could be used. String support ${...} JavaScript embeddings. String could be joined with "abc" + "bce" and be on different lines. String support \c escaping.
* Number - a decimal values `123` not containing . and other characters and symbols.
* Float - a decimal . decimal value.
* Comment - a sequence starting with // and ending of end of line.
* CComment - /* ... */
* Expressions - ${...} $(...) - any expression in JavaScript language.
* Markdown block - %{...} is interpreted as text block with multi line structure, could be highlighted and interpreterd as markdown markup.
* Command separator - ';' could be used as command separator.

(!) All comments are fully removed at this versions of application.
