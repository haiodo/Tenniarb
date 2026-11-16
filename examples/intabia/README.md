# Intabia Platform design

The Intabia Platform UI kit and screens as Tenniarb `.tenn` files. Open `PlatformDesign.tenn`: it mounts every
library and application, application > category > screens. The files are hand-edited in Tenniarb. They came
from the screen scripts of the `foundation` repository, whose Storybook remains as the component catalogue;
the generator that produced the first version is gone.

`tenniarb check PlatformDesign.tenn` validates the whole tree, and `tenniarb check <file>` a part in the
context of the entry document.

## Layout

| File | Content |
|---|---|
| `PlatformDesign.tenn` | the entry document: the Light and Dark palettes, the `Theme` component, `Libraries`, one element per application, and `Tasks` with the design task files |
| `ui-kit.lib.tenn` | the `ui_kit` library: mounts one file per category, `element Actions --from-file "lib/actions.tenn"` |
| `lib/<category>.tenn` | the components of a category as `item`s; for a library of one category (`workbench`, `settings_kit`, `tracker_kit`, `chat_kit`, `login_kit`, `profile_kit`) this is the mounted library file itself |
| `apps/<app>.tenn` | an element per category of the application, with its screens on a grid |
| `fusio-1613.tenn`, `fusio-1621.tenn` | design tasks as they were drawn: an element per page, the screens on `pos` |
| `fusio-1620.tenn`, `cmp-1620-*.tenn`, `final-1620-flow.tenn` | design task files maintained by hand, mounted under `Tasks` |
| `assets/` | pictures the components place with `image`; the path is relative to the file that uses it |

Every file but `PlatformDesign.tenn` starts with `entry "<path to PlatformDesign.tenn>"` and mounts no library:
a name is looked up in the file, then up the chain of mounts to the entry document (Tenniarb spec 5a.1, 5a.19),
so a part opens on its own and `tenniarb check <file>` resolves it in the context of the entry.

## Libraries

| Library (name under `Libraries` in PlatformDesign) | Categories | Path of a component |
|---|---|---|
| `ui_kit` | `Actions`, `Forms`, `People`, `Status`, `TagsAndFilters`, `Icon`: base components and icons | `Libraries.ui_kit.Actions.Button`, `Libraries.ui_kit.Icon.tick` |
| `workbench` | `Workbench`: shell, top bar, app rail, navigator, nav items, space and panel headers | `Libraries.workbench.TopBar`, `Libraries.workbench.Workbench` |
| `settings_kit` | `Settings`: section title, form and grid rows, table, settings layout | `Libraries.settings_kit.FormRow` |
| `tracker_kit` | `Tracker`: issue cards, kanban column, issue rows and groups, issue page | `Libraries.tracker_kit.Cards.IssueCard` |
| `chat_kit` | `Chat`: messages, date separator, input | `Libraries.chat_kit.Message` |
| `login_kit` | `Login`: login card, form parts, pages | `Libraries.login_kit.LoginCard` |
| `profile_kit` | `Profile`: the account and workspace screens of the profile task | `Libraries.profile_kit.ProfilePage` |

Dependencies point down the list: the base categories are used by every application and by the other libraries;
`Tracker` also uses `Status` and `TagsAndFilters`; `Settings` and `Login` use `Workbench`. A library of one
category has no category level in the path. Inside a library file a component of the same category is named by
its item, one of another category of the same library by `Category.item`, one of another library by its full path
with the `Libraries.` prefix.

## Add a component

The library is the source: add or change a component in `lib/<category>.tenn` (an `item` with `export let`
parameters and `styles`, optional parts `hidden`, containers with variable content as `slot`), and use it in
screens with `use <library>.<path> as ...`, passing parameters by name.

A `§` item is a section title of the catalogue, not a component: `Insert Component` lists any named node, skip
those.

## Notes

- Parameter values reach JavaScript as strings, except numbers: `"false"` is truthy. Write boolean parameter
  conditions as `flag === "true"`.
- An expression on a nested `use` sees the parameters of that component, not the parent. For such a part use a
  `styles` rule of the parent with literal `hidden yes/no`.
- Keyframes are looked up in the component the `animation name ...` is written in; a file-level block is not
  visible to a nested `item`.
- A `style` block of a part adds properties before its own; remove the part's literal if the style overrides it.
- MCP `render` evaluates parameters in an `element`, not in the root catalogue `item` of a library. Check
  parameter changes on an element catalogue or an instance on a screen.
- `tenn-render` runs from the root `PlatformDesign.tenn` with the dotted path of a screen. Passing a part with
  `entry` gives no context of the mounted libraries.
