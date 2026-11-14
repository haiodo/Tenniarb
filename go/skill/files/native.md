# Native desktop screens: one library, six looks

`libraries/native.tenn` is one component library for macOS, Windows 11 and Linux, each in light and
dark. The look is chosen by a `Theme` wrapper; there are no per-platform copies of the library:

```tenn
element "native" --from-file "tenniarb-library://native"

element "Preferences" {
    use native.Theme as skin {
        osName Macos
        theme Light
        slot content {
            use native.Window as window { winWidth 560; winHeight 470; slot content { /* … */ } }
        }
    }
}
```

`osName` is `Macos`, `Windows` or `Linux`; `theme` is `Light` or `Dark`; both default to `Macos` and
`Light`. The words name the palette items of the library, so the capital letter matters: `theme dark`
is an error, `theme Dark` is the skin. Everything inside the slot inherits the look, including
components used deep in the tree. A nested `Theme` switches a subtree: the demo's `Skins` element wraps
six cards in six skins.

Demo: `examples/native-demo.tenn`. `Skins` shows all six side by side; `Preferences`, `Workbench` and
`MenuDemo` are full screens on `Macos` / `Light`.

## How a theme works

Each skin is an **item** of the library: a theme item holds what the three systems share, and a system
item only what it changes:

```tenn
item Light {
    field #ffffff; text #000000; onAccent #ffffff; font 13; /* the colours all three share */
    item Macos { surface #ececec; accent #0a84ff; radius 6; /* … */ }
    item Windows { surface #f3f3f3; accent #0067c0; radius 4; /* … */ }
    item Linux { surface #f6f5f4; accent #3584e4; radius 6; /* … */ }
}
item Dark { /* the colours all three share, and the same three items */ }
```

`Theme` turns those items into context in two layers:

```tenn
context --from-item ${theme}
context --from-item ${theme + '.' + osName}
```

`--from-item <path>` reads every property of that item as a context key. The first line brings what the
systems share, the second what this one changes: a later line wins, and a key it does not name keeps the
value from the line above. The palette has no expressions of its own, so the components read plain
`ctx.surface`, `ctx.accent`, `ctx.radius` and the rest. The same mechanism is available to any library.
Two consequences:

- A screen must sit inside a `Theme`; outside one the tokens are undefined and `render` reports the
  warnings. The library's own file root repeats the two lines for its elements, so opening
  `libraries/native.tenn` previews the macOS look.
- Do not write surface colors in a screen. A raw `color #ffffff` stays white in the dark theme; use
  `Window`, `Page` and `Panel`, which paint `ctx.surface`, `ctx.cardBg` and the rest.

The tokens are the properties of the palette items at the top of `libraries/native.tenn`: `surface`,
`panel`, `field`, `button`, `border`, `borderDisabled`, `divider`, `text`, `textSecondary`,
`textDisabled`, `accent`, `selection`, `selectionText`, `onAccent`, `headerBg`, `group`, `tabOff`,
`shadowColor`, `radius`, `radiusSmall`, `radiusPanel`, `windowRadius`, `font`, `fontSmall`, `cardBg`,
`pageBg`, `sidebarBg`, `tabRule`. To retune a skin, edit its item; every component follows. To add a
system, add the item to `Light` and `Dark` and name it in `osName`.

## Palette at a glance

| | macOS light | macOS dark | Windows light | Windows dark | Linux light | Linux dark |
|---|---|---|---|---|---|---|
| surface | `#ececec` | `#1e1e1e` | `#f3f3f3` | `#202020` | `#f6f5f4` | `#242424` |
| field | `#ffffff` | `#1e1e1e` | `#ffffff` | `#2d2d2d` | `#ffffff` | `#363636` |
| text | `#000000` | `#ffffff` | `#000000` | `#ffffff` | `#000000` | `#ffffff` |
| accent | `#0a84ff` | `#0a84ff` | `#0067c0` | `#60cdff` | `#3584e4` | `#3584e4` |
| selection | `#cfe4ff` | `#244a73` | `#cce4f7` | `#264f78` | `#3584e4` | `#3584e4` |
| selection text | black | white | black | white | white | white |

Windows dark puts black on its light accent: the default button and the check mark are black there.
Radii: 6 controls and 8 panels on mac and linux, 4 and 4 on Windows, window 10 / 8 / 0.

## Type, sizes, spacing

| Value | Where |
|---|---|
| 13 | labels, controls, menu items, table cells |
| 11 | status bar |
| 24 | button, text field, combo, menu bar item |
| 20 | list and tree row, checkbox, radio |
| 27 | tab |
| 22 | menu item, table header, status bar |
| 28 | toolbar |
| 6–8 | gap between controls, group padding |
| 12 | page padding |
| 10 | page margin |

## Capitalization

The rules are platform-neutral; they come from the Eclipse UI Guidelines.

| Title Case | Sentence case |
|---|---|
| menus and menu items | check box labels |
| tooltips | radio button labels |
| window, dialog and tab titles | group labels |
| column headings | labels of text fields |
| push buttons (`OK`, `Cancel`, `Apply and Close`) | descriptive text |

No ending punctuation. `Show whitespace characters`, not `Show Whitespace Characters`; `Text font`,
not `Text Font`; button `OK`, tab `General`, column `Updated`.

- A dialog focuses its first input; with no input, the default button.
- Footer order: `OK` `Cancel` for a simple dialog; `Back` `Next` `Finish` `Cancel` for a wizard.
- An error that needs an answer is a modal dialog, not a red line inside the page.
- No mnemonics in a mockup: the underline is a keyboard affordance the drawing does not carry.

## Components

| Component | Draws | Parameters |
|---|---|---|
| `Button` | push button, 24 px | `text`, `kind` = `normal` `default` `disabled` |
| `TextField` | field with a border | `value`, `kind` = `normal` `readonly` `disabled`, `fieldWidth` |
| `Checkbox` | 16 px box, accent when checked | `text`, `checked`, `enabled` |
| `Radio` | 16 px circle, accent dot | `text`, `checked`, `enabled` |
| `Combo` | field with a chevron | `value`, `comboWidth`, `enabled` |
| `Tab` | tab; a blue rule only on Linux | `text`, `active`, `tabWidth` |
| `ToolItem` | 26 x 24 icon button | `icon` (SVG path, view-box 14), `enabled` |
| `MenuBarItem` | top-level menu label, 24 px | `text`, `active` |
| `MenuItem` | 22 px row: check, label, shortcut | `text`, `shortcut`, `checked`, `enabled`, `itemWidth` |
| `Menu` | popup panel with a shadow | `menuWidth`, `slot content` |
| `MenuSeparator` | 1 px line | `itemWidth` |
| `TreeItem` | 20 px row with a twisty | `text`, `depth`, `selected`, `expanded`, `sidebar`, `rowWidth` |
| `TableHeader` | 22 px header, Name / Status / Updated | `rowWidth` |
| `TableRow` | 20 px row, same columns | `name`, `status`, `updated`, `selected`, `rowWidth` |
| `Group` | bordered box with a caption | `title`, `groupWidth`, `slot content` |
| `Separator` | 1 px line, horizontal or vertical | `lineWidth`, `vertical` |
| `StatusBar` | message left, size right | `message`, `barWidth` |
| `Dialog` | title strip, body, right-aligned footer | `title`, `dialogWidth`, `slot content`, `slot footer` |
| `Window` | outer frame | `winWidth`, `winHeight`, `slot content` |
| `Page` | content area | `pageWidth`, `pageHeight`, `kind` = `card` `flat` `sidebar`, `slot content` |
| `Panel` | toolbar strip | `panelWidth`, `panelHeight`, `slot content` |

`yes` / `no` are the boolean values.

## Widths are explicit

Rows, tables, groups, pages and bars take their width as a parameter. A `width fill` inside a component
whose parent hugs collapses to the content, so pass the same width down a table:
`TableHeader { rowWidth 506 }` and every `TableRow { rowWidth 506 }`. The value is the page width minus
its padding and borders.

`TableHeader` and `TableRow` hard-code three columns (200, fill, 110). For another table, build rows as
items with the same widths; do not stretch this component past a few more columns.

## A preferences page

```tenn
element "native" --from-file "tenniarb-library://native"

use native.Theme as skin {
    osName Macos
    theme Light
    slot content {
        use native.Window as window {
            winWidth 560
            winHeight 470
            slot content {
                item Tabs {
                    auto-layout row; padding 6 8 0 8; width fill; align end
                    use native.Tab as general { text General; active yes }
                    use native.Tab as editors { text Editors }
                    item Filler { width fill }
                }
                use native.Page as page {
                    pageWidth 534
                    pageHeight 390
                    slot content {
                        item Body {
                            auto-layout column; gap 10; padding 12; width fill
                            use native.Group as appearance {
                                title Appearance
                                groupWidth 508
                                slot content {
                                    use native.Checkbox as c1 { text "Show text editors"; checked yes }
                                    use native.Checkbox as c2 { text "Show whitespace characters" }
                                }
                            }
                            item Table {
                                auto-layout column; width 508
                                use native.TableHeader as head { rowWidth 506 }
                                use native.TableRow as r1 { rowWidth 506; name Platform; status Active; updated Today; selected yes }
                            }
                        }
                    }
                }
                item Buttons {
                    auto-layout row; align center; justify end; gap 6; width fill; height 40; padding 0 10
                    use native.Button as ok { text OK; kind default }
                    use native.Button as cancel { text Cancel }
                }
            }
        }
    }
}
```

## A workbench window

The sidebar and the editor are `Page` kinds; a vertical `Separator` marks the boundary.

```tenn
element "native" --from-file "tenniarb-library://native"

use native.Theme as skin {
    osName Macos
    theme Light
    slot content {
        use native.Window as window {
            winWidth 900
            winHeight 600
            slot content {
                item MenuBar {
                    auto-layout row; align center; padding 0 2; width fill; height 24
                    use native.MenuBarItem as file { text File }
                    use native.MenuBarItem as edit { text Edit }
                }
                use native.Panel as toolbar {
                    panelWidth 898
                    panelHeight 28
                    slot content {
                        item Left {
                            auto-layout row; align center; padding 0 4
                            use native.ToolItem as newFile { icon "M3 2 h5 l3 3 v9 h-8 Z" }
                            use native.ToolItem as run { icon "M4 2 L11 7 L4 12 Z" }
                        }
                    }
                }
                item Main {
                    auto-layout row; width 898; height 524
                    use native.Page as treePage {
                        pageWidth 280
                        pageHeight 524
                        kind sidebar
                        slot content {
                            item Tree { auto-layout column; padding 2 0
                                use native.TreeItem as root { text Platform; expanded yes; sidebar yes; rowWidth 276 }
                                use native.TreeItem as file { text Workbench.tenn; depth 2; selected yes; sidebar yes; rowWidth 276 }
                            }
                        }
                    }
                    use native.Separator as divider { lineWidth 524; vertical yes }
                    use native.Page as editorPage {
                        pageWidth 617
                        pageHeight 524
                        kind flat
                        slot content { /* editor */ }
                    }
                }
                use native.StatusBar as bar { message Ready; barWidth 898 }
            }
        }
    }
}
```

## A dialog, a menu, a theme switch

`Dialog` carries the title strip, the body and the footer; fill both slots.

```tenn
element "native" --from-file "tenniarb-library://native"

use native.Dialog as rename {
    title "Rename Resource"
    dialogWidth 420
    slot content {
        item Form {
            auto-layout column; gap 6; width fill
            item Prompt { title "Enter the new name:"; font-size 13 }
            use native.TextField as name { value "Workbench.tenn"; fieldWidth 380 }
        }
    }
    slot footer {
        use native.Button as ok { text OK; kind default }
        use native.Button as cancel { text Cancel }
    }
}
```

A menu floats over the content, so it needs an `abs` parent. In a column it lands at the bottom.

```tenn
element "native" --from-file "tenniarb-library://native"

item Stage {
    auto-layout abs; width 478; height 318
    use native.Page as page { pageWidth 476; pageHeight 316; kind flat; slot content { } }
    item MenuBox {
        pos 16 -52
        use native.Menu as menu {
            menuWidth 220
            slot content {
                use native.MenuItem as mi1 { text "New Project"; shortcut "Ctrl+N"; itemWidth 216 }
                use native.MenuSeparator as sep { itemWidth 216 }
                use native.MenuItem as mi2 { text Delete; enabled no; itemWidth 216 }
            }
        }
    }
}
```

To show one screen in two skins, wrap it twice with different `osName` and `theme` values; do not copy
the screen. The `Skins` element in the demo is the worked example: one card per skin, each a `Theme`
wrapper around the same small window.

## What the library does not draw

- Window chrome: no title bar, close button or resize grip. A screen shows the client area; the title
  belongs in the element name and the document around it.
- Focus rings, hover and pressed states, scrollbars, cursors. Draw the selected row and the default
  button; leave the rest to the real UI.
- Icons are `path` strings with `view-box`, not images. The samples are 14 x 14; keep that box.
- The group caption sits inside the top border. Drawing text over a border is not something the scene
  does.

## Checks before the render

- `check` is empty for the screen and for the mounted library.
- The screen sits inside `Theme`; a nested `Theme` only where a subtree must differ.
- Every `use` has `as <name>`; the name is unique in its parent.
- A header and its rows share `rowWidth`.
- A menu sits in an `abs` parent.
- No raw surface colors in the screen: `Window`, `Page` and `Panel` do that job, so the dark theme works.
- Button and menu titles are Title Case; check box, radio and group labels are sentence case.
