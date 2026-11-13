# The 100 scenarios

One row is one directory in `scenarios/`: `task.md` is the prompt, `check.json` the criteria. `done` means
both files exist; `todo` means the task is agreed and the files are next. The fixture is `workspace/`:
`lib/ui_kit.tenn` (Button, Card, Avatar, Badge, Panel), `lib/forms.tenn` (Field, Dialog), `apps/home.tenn`,
`apps/empty.tenn`, a classic `apps/flow.tenn` when a scenario makes it, `assets/logo.png`.

Tiers: **b** basic syntax (20), **l** layout (20), **c** components (15), **s** states (15), **d** diagrams
(10), **t** tool discipline (10), **r** real pages of the platform design (10), **v** a picture to reproduce
(8). 100 tasks plus the visual tier.

| id | task | check |
|---|---|---|
| b01-card | a 320x200 white card, radius 12, padding 16, bold 20 title | card/style/title/render | done |
| b02-buttons | a row of three ui_kit buttons: primary, two secondary | uses/labels/kinds/render | done |
| b03-icon | a 24x24 check drawn with `path`, view-box 24, purple | path/view-box/color/render | done |
| b04-flow | new apps/flow.tenn mounted, 3 items, 2 arrow links | mount/items/links/render | done |
| b05-text-hierarchy | a 360 column: 32/20/13 pt title, subtitle, body | sizes/colors/render | done |
| b06-swatches | three 64x64 swatches, radius 12, row gap 8 | sizes/colors/render | done |
| b07-shadow | a card with `shadow 0 -8 24` | shadow/render | done |
| b08-outline | a 200x120 transparent box, 2px grey border, radius 8 | color none/border/line-width | done |
| b09-image | assets/logo.png 160 wide, `fit contain` | image/fit/render | done |
| b10-avatar-badge | a row with Avatar `MC` and Badge `Owner` | uses/params/render | done |
| b11-badge-at | three badges; the last two recolour Label with `at` | uses/at/text-color | done |
| b12-divider | a 320x1 divider in #e5e7eb | size/color/render | done |
| b13-button-kinds | primary Save + secondary Cancel, 8 gap | kind/params/render | done |
| b14-expression | a box `width ${40 * 3}` | width 120/render | done |
| b15-style | a `styles { accent { color #6b3ad3 } }`, used by a box | style/use-style/color | done |
| b16-two-styles | a `one-of` style, two nodes with different values | style args/colors | todo |
| b17-context | element `context radius ?= 8`, box `corner-radius ${ctx.radius}` | context/radius | done |
| b18-compact | one compact one-line block, still valid | parse/check/compact | done |
| b19-markdown-body | a classic item with a markdown `body` | body/check/render | done |
| b20-palette | three boxes in palette names (gray-400, blue, red) | colors resolve/render | done |
| l01-window | 1200x760: 48 rail, 240 side, fill main, 280 inspector | widths/heights/render | done |
| l02-list | five 44-high rows, fill title + badge right | rows/badges/layout/render | done |
| l03-dialog | scrim + 480 Dialog over Home with footer buttons | scrim/dialog/text/buttons | done |
| l04-form | 360 settings card: title, two Fields, Save | card/fields/button/layout | done |
| l05-settings-nav | 900x700 settings page, 240 nav + content | sizes/nav/content | done |
| l06-table | header + 4 rows, columns 240/fill/120 | rows/widths/alignment | done |
| l07-tabs | 3 tabs 48 high + panel below | tabs/panel/selection | done |
| l08-grid | 2x3 cards 300 wide, gap 16 | positions/count/overlap | done |
| l09-mobile-detail | 390x844: 56 header, 220 hero, content, 72 footer | heights/sections | done |
| l10-two-column-form | 2-column form, labels above fields | columns/fields | done |
| l11-toolbar | 56 toolbar: title fill, two buttons right | height/justify/buttons | done |
| l12-empty-state | 1200x760 centered empty state | center/align/text | done |
| l13-sidebar-nav | 280 sidebar, 8 nav rows, active row highlighted | rows/highlight/width | done |
| l14-split | main fill + 280 panel inside 1200 | widths/no overflow | done |
| l15-hero | 1200x480 hero: 40 title, subtitle, CTA centered | sizes/center/button | done |
| l16-pricing | three 320 pricing cards, middle highlighted | widths/gap/border | done |
| l17-sticky-footer | dialog header/body fill/footer at bottom | body fill/footer bottom | done |
| l18-avatar-stack | four avatars overlapping by -8 | pos/overlap intended | done |
| l19-wrap-tags | a wrap row of 8 tags, width 320 | wrap/rows/overflow | done |
| l20-absolute-badge | card with a badge at top-right via abs pos | pos/inside parent | done |
| c01-params | two Cards: one param title, one `at Title` | param/at/component unchanged | done |
| c02-component | add Stat to ui_kit, use twice on Home | component/style/uses/render | done |
| c03-slot | a Panel use with two cards in `slot content` | use/slot/children/render | done |
| c04-at-override | override Button Label text in a use | at/title/render | done |
| c05-instance-width | two Card uses 280 and 360 wide | overrides/component unchanged | done |
| c06-find-component | find Badge with `get_state {find}`, report its width | answer/one find call | done |
| c07-new-library | new lib/stats.tenn with Stat, mounted and used | file/mount/use/check | done |
| c08-refactor | rename Badge to Tag through the workspace | refactor/uses/check | done |
| c09-nested | a component that uses Avatar and Badge, used on Home | nested use/render | done |
| c10-new-app | new apps/profile.tenn with entry, mounted in the entry | entry/mount/check | done |
| c11-read-mounted | read a mounted component and report its padding | answer/read path | done |
| c12-field-param | a Field with label Email and a value with @ | params/title/render | done |
| c13-instance-color | recolour one Button instance, not the component | instance prop/component unchanged | done |
| c14-clone-component | clone Button as IconButton with a symbol-sized label | both exist/check | done |
| c15-style-in-component | a component with `use-style`, overridden in a use | style/override/check | done |
| s01-states | three 390x844 screens: empty, loading, error | frames/content/layout/render | done |
| s02-loading-rows | list skeleton: six 48-high grey rows | rows/colors/no text | done |
| s03-error-banner | red banner + Retry above a list | banner/button/order | done |
| s04-empty-search | search field + empty state + Clear | field/empty/action | done |
| s05-toast | success toast bottom-right over Home | pos/colors/above | done |
| s06-confirm-delete | confirm dialog with Delete/Cancel over a list | dialog/buttons/scrim | done |
| s07-onboarding | 3-step wizard, step 2 active | steps/active/check | done |
| s08-permission | permission screen: icon, text, Allow/Deny | icon/text/buttons | done |
| s09-payment-error | card form with an error under the card field | field/error/border | done |
| s10-offline | offline banner + cached list | banner/list/order | done |
| s11-multi-select | list of 6, three selected rows | selected fills/count | done |
| s12-tabs-frames | the same screen twice: tab 1 and tab 2 | frames/tab states | done |
| s13-mobile-drawer | phone with an open drawer over content | drawer/overlay/scrim | done |
| s14-table-empty | table header + one empty message row | header/empty row | done |
| s15-dark-twin | light and dark twin of a card | two frames/colors | done |
| d01-brainmap | hub + 6 branches | items/links/render | done |
| d02-branch-merge | decision, two branches, merge | links/items/render | done |
| d03-legend | diagram with a legend plate | plate/items/labels | todo |
| d04-plates | two plates, items inside each | parent/plate/inside | todo |
| d05-link-labels | links with titles | link titles/render | done |
| d06-line-styles | solid, dashed, dotted links | line-style/render | done |
| d07-hub-elbow | `route elbow` hub | route/render | done |
| d08-c4 | three C4 boxes with body text and arrows | body/links | done |
| d09-tree | three-level mind map tree | levels/links | done |
| d10-extend-flow | add one node and link to the flow | node/link/others unchanged | done |
| t01-preserve | change one heading, nothing else | one-line diff/no native write | done |
| t02-design-ops | rename a card and set its width with `design` | props/trace/others unchanged | done |
| t03-invalid-prop | set a property that does not exist; report, keep the file | file unchanged/answer | done |
| t04-no-whole-read | find a value in a 500-line seeded library | trace: no whole read | done |
| t05-multi-edit | change two nodes in one edit call | trace: one edit, two pairs | done |
| t06-exact-find | change one of two equal values with context | one line changed | done |
| t07-render-in-edit | use the `render` argument of edit, no second call | trace: render inside edit | done |
| t08-layout-first | find and fix an overflow | trace: layout before render | done |
| t09-stale-find | the seeded text differs from the task; re-read and retry | final state/check | done |
| t10-new-file-entry | add a screen file to the workspace and mount it | file/entry/mount/check | done |
| r01-dark-section | a dark twin of one platform section | plate/components/validate | todo |
| r02-plates | every card of a library sits inside its plate | layout: no leaves-plate | todo |
| r03-rename-component | rename a component, keep every use valid | check/find uses | todo |
| r04-build-screen | a 1200x760 screen from three library components | size/components/warnings | todo |
| r05-settings-row | add one settings row in the house style | row/props/render | todo |
| r06-overflow | fix the overflow layout reports on a page | layout clean/render | todo |
| r07-storybook-page | rebuild a storybook page from its screenshot | render/sections | todo |
| r08-preserve-big | change one value on a 300 KB page | one-line diff/check | todo |
| r09-comment | answer an open comment and fix it | comment resolved/file | todo |
| r10-cross-file | rename a part used by two app files | refactor/check | todo |

| v01-login | reproduce a 360x420 login from a picture | 1:1 compare/mae/match | done |
| v02-settings | reproduce a 1584x432 pair of settings screens from a picture | compare/structure | done |
| v03-workbench | reproduce a 1920x1080 platform workbench | compare/structure | done |
| v04-button-catalog | reproduce a 608x448 button catalog | 1:1 compare | done |
| v05-chat | reproduce a 1920x1080 chat channel | compare/structure | done |
| v06-dark-login | reproduce the login in the dark theme | compare/colors | done |
| v07-mobile-profile | reproduce a 390x844 profile | 1:1 compare | done |
| v08-table | reproduce a 900x620 table page | compare/structure | done |

## Context budgets

A run's size is the peak of one request: the system prompt and skill (~19k), the tool calls and their
answers, the renders. The runner records `peak_tokens` and, against the scenario's `ctx`, `ctx_over`.
Measured on flash: b/t/d runs peak at 25-38k, the login reproduction at 102k.

| ctx | what fits | tiers |
|---|---|---|
| 50k | the fixture, a few reads and one render | b, d, t |
| 200k | library browsing, several renders, a picture to compare | l, c, s, v (simple), v04 |
| 500k | a 1920x1080 page rebuilt from a picture, many render rounds | r, v03 |

## Counts

| tier | total | done | todo |
|---|---|---|---|
| b | 20 | 20 | 0 |
| l | 20 | 20 | 0 |
| c | 15 | 15 | 0 |
| s | 15 | 15 | 0 |
| d | 10 | 8 | 2 |
| t | 10 | 10 | 0 |
| r | 10 | 0 | 10 |
| v | 8 | 8 | 0 |
| **all** | **108** | **96** | **12** |
