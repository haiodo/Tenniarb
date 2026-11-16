# Карта паритета: Go → Swift

Рабочий документ, 2026-11-16. Один рядок - один портированный юнит: что в Go, что ему отвечает в Swift,
насколько это сделано и что сверять. План работ - `../my-tasks/tenniarb/swift-parity.md`, здесь только
учёт. Ссылки `file:line` даны по текущему дереву; строки сдвигаются, сверять по имени символа.

## Как держать карту актуальной

- Любая правка порта (Go или Swift) правит свои рядки в том же коммите: добавился юнит в Go - рядок
  получает `нет` и инвариант в «Что сверять» до того, как появится код Swift.
- `сверено` ставится только после прогона сверки против Go, а не после «код похож». Для сцены это
  голдены `web/fixtures/scene/*.layout.json`, снятые `go/cmd/tennlayout` (сверяет
  `web/packages/render/test/scene-parity.test.ts`), и `TestGoldensAreCurrent`
  (`go/cmd/tennlayout/main_test.go:13`). Для классики пиксельного равенства нет, поэтому её рядки
  остаются `есть`.
- Статусы: `нет` - кода в Swift нет; `начат` - типы/чтение есть, поведение ещё не то; `есть` - работает,
  но с Go не сверялось; `сверено` - сверка прошла, дата и стенд названы в «Что сверять».
- Факт без проверки пишется как «не проверено», догадка не пишется. Строка «Что сверять» называет
  инвариант или файл-фикстуру, а не «протестировать».

## 1. Сценовый движок: `go/scene` → `Tenniarb/scene`

| Go | Swift | Статус | Что сверять |
|---|---|---|---|
| `go/scene/tree.go:14` `Kind` | `Tenniarb/scene/SceneNode.swift` `Kind` | начат | значения `file/element/item/link/use/slot` и их порядок (`tree.go:16-23`) |
| `go/scene/errors.go:9` `Pos`, `:17` `Error`, `:31` `Errors` | `SceneNode.swift` `Pos`, `SceneLoader.swift` `Diagnostic` | начат | 1-based строка/колонка по формуле `load.go:324`; файл - слэш-путь от корня; дедупликация и сортировка диагностик (`load.go:104`) - в Swift нет |
| `go/scene/tree.go:26` `Range` | `SceneNode.swift` `Range` | начат | байтовые `[start,end)`; `End` команды в Swift-узле `Tenn` не записан, конец берётся по последнему токену - точность не проверена |
| `go/scene/tree.go:105` `Node` | `SceneNode.swift` `Node` | начат | поля `Name/Key/Pos/Range/Body/Parent/Props/Content/Elems/Mount/From/To/Path/Block/Styles/StyleGroups/Type/Params`; `NameArg/PathArg/FromArg/ToArg` не портированы |
| `go/scene/tree.go:33` `Prop` | `SceneNode.swift` `Prop` | начат | `text`/`num` по аргументам; `Val` (вычисленное) не портирован |
| `go/scene/tree.go:160` `Param` | `SceneNode.swift` `Param` | начат | имя как написано, `Default == nil` - обязательный; `ctx` и повтор - предупреждения (`load.go:482-503`) |
| `go/scene/tree.go:178` `Style`, `:184` `StyleKid`, `:191` `StyleBlock`, `:199` `StyleGroup` | `SceneNode.swift` `Style`, `StyleKid`, `StyleBlock`, `StyleGroup` | начат | нормализация имён, порядок объявления, `one-of`/`any-of` (`load.go:714`) |
| `go/scene/tree.go:91` `Override`, `:81` `OvKind` | `SceneNode.swift` `Override`, `OvKind` | начат | `prop/node/at/replace/slot` и вложенность блока (`load.go:843`) |
| `go/scene/tree.go:205` `IsPlate` | - | нет | имя с `§`; без `Nodes()`/путей не нужно |
| `go/scene/names.go:14` `NormalizeName` | `SceneNode.swift` `normalizeName` | начат | буквы/метки/цифры сохраняются, всё прочее `_`; пустое имя пустое |
| `go/scene/names.go:26` `Segment`, `:82` `splitPath` | `SceneLoader.swift` `validPath` | начат | синтаксис пути (точки, кавычки, `\"`); `Segment` и разбор на сегменты не портированы - нужны для `resolve` |
| `go/scene/load.go:21` `Tree` | `SceneLoader.swift` `Tree` | начат | `root/main/files/order/src/diags`; `ctxs/overlay/top` не портированы |
| `go/scene/load.go:46` `Load`, `:56` `LoadOverlay` | `SceneLoader.swift` `load(file:)` | начат | корень воркспейса, выход за корень - ошибка (`:228`); главный файл нечитаем - ошибка, остальное - диагностики |
| `go/scene/load.go:64` `LoadIn` | - | нет | контекст `entry` (спека 5a.1) |
| `go/scene/entry.go:218` `LoadAuto`, `:149` `FindEntry` | - | нет | поиск entry вверх по папкам; без него доска-часть не видит имена мастера |
| `go/scene/load.go:253` `loadFile` | `SceneLoader.swift` `loadFile` | начат | кэш по абсолютному пути, стек монтирований, цикл - ошибка с позициями (`:292`); `joinedBlocks` - нет |
| `go/scene/load.go:405` `fill` | `SceneLoader.swift` `fill` | начат | порядок разбора `element/content/styles/kind/export/template/else` и `template` не портирован |
| `go/scene/load.go:440` `addElement` | `SceneLoader.swift` `addElement` | начат | вложенный `element`, `kind` в аргументах - предупреждение, блок вместе с `--from-file` - ошибка |
| `go/scene/load.go:482` `addParam` | `SceneLoader.swift` `addParam` | начат | `export let <имя>` и `export let <имя> = <значение>`; `ctx` и повтор - предупреждения |
| `go/scene/load.go:516` `addTemplate`, `:549` `dedent` | - | нет | текст шаблона и снятие отступа |
| `go/scene/load.go:592` `mountFile` | `SceneLoader.swift` `mountFile` | начат | списки общие с корнем файла, `Type` наследуется; схемы `tenniarb-library://` (`:627`) нет |
| `go/scene/load.go:652` `resolveFile`, `:662` `resolveIn` | `SceneLoader.swift` `resolveIn` | начат | путь от папки монтирующего файла, а не от корня; выход за корень - ошибка |
| `go/scene/load.go:670` `addStyles`, `:692` `parseStyles` | `SceneLoader.swift` `addStyles` | начат | имя стиля нормализуется, блоки детей, `animation` в стиле - предупреждение (`:737`) |
| `go/scene/load.go:757` `addContent` | `SceneLoader.swift` `addContent` | начат | `item/slot/link/use`; голый `use` и `slot` вне `item` остаются свойствами |
| `go/scene/load.go:792` `newUse` | `SceneLoader.swift` `newUse` | начат | форма `use <путь> as <имя>`, путь из слов и кавычек, блок - overrides |
| `go/scene/load.go:304` `joinedBlocks` | - | нет | два блока в одной команде - предупреждение |
| `go/scene/exprcheck.go:14` `checkExpressions` | - | нет | синтаксис выражений при загрузке |
| `go/scene/resolve.go:86` `Resolve`, `:106` `lookup` | - | нет | путь по нормализованным именам, подъём по `Parent` и через точку монтирования; `ambiguous path` |
| `go/scene/expand.go:20` `RNode`, `:58` `expander` | - | нет | разворот `use`: экземпляр, слоты, `at`/`replace`, параметры |
| `go/scene/params.go:61` `params`, `:92` `defaults` | - | нет | значение параметра из `use` выше умолчания; обязательный без значения - ошибка |
| `go/scene/styles.go:15` `applyStyles`, `:77` `inheritProps` | - | нет | свойства стиля после последнего `use-style`; наследование шрифта от родителя |
| `go/scene/context.go:48` `context`, `:23` `ctxDecl` | - | нет | `context`/`ctx` (спека 5f) |
| `go/scene/layout.go:106` `prep`, `:192` `root`, `:400` `item` | `Tenniarb/scene/SceneLayout.swift` `LayoutEngine` | начат | корень `Abs` и его размер; `pos` - верхний левый угол, экранный `y = -pos.y` (`:405`) |
| `go/scene/layout.go:494` `container` | `SceneLayout.swift` `container` | начат | `auto-layout row/column/abs`, `gap`, `align`, `justify`; `wrap`, `line-gap` не портированы |
| `go/scene/layout.go:510` `padding` | `SceneLayout.swift` `padding` | начат | 1 / 2 (вертикаль, горизонталь) / 4 (T R B L) значения |
| `go/scene/layout.go:583` `size`, `:609` `sizeWord` | `SceneLayout.swift` `size` | начат | число, `hug`, `fill`; `fill` без auto-layout родителя - hug; проценты - предупреждение |
| `go/scene/layout.go:486` `limits` | `SceneLayout.swift` `limits` | начат | `min/max-width/height`, `0` в max - без предела |
| `gowt gfx/layout/layout.go:97` `layout.Node`, `:171` `Layout` | `SceneLayout.swift` `LayoutNode`, `layout()` | начат | row/column/abs, fixed/hug/fill, gap/pad/align/justify, absolute, min/max; `Wrap`/`LineGap` нет |
| `go/scene/layout.go:629` `leaf` | `SceneLayout.swift` `leaf` | начат | размер листа из текста/тела; текста ещё нет, лист без `width`/`height` получает 0 - не проверено |
| `go/scene/layout.go:734` `textOf` | - | нет | имя/`title`, `font-*`, `line-height`, `layout left/right`, `marker` |
| `go/scene/layout.go:96` `lineBox`, `:1066` `natural`, `:1082` `height`, `:1094` `layout`, `:1187` `wrap` | - | нет | переносы, базовые линии, метрики шрифта, классические коэффициенты `:47-63` |
| `go/scene/measure.go:13` `MeasureText` | - | нет | оценка ширины/высоты текста для раскладки |
| `go/scene/body.go` (1183) `bodyText` | - | нет | тело item: markdown, блоки, ширина, высота |
| `go/scene/bodylink.go:10` `BodyLink` | - | нет | ссылки и якоря внутри тела |
| `go/scene/paint.go:11` `gradient`, `:36` `fill`, `:103` `shadows` | - | нет | заливка, градиент, тени |
| `go/scene/color.go:14` `parseColor`, `:63` `contrast` | - | нет | синтаксис цвета и контраст текста на заливке |
| `go/scene/icon.go:62` `icon` | - | нет | пути иконки внутри item |
| `go/scene/image.go:30` `itemImage`, `:54` `rasterCache` | - | нет | картинки item, base64/файл, кэш растров |
| `go/scene/build.go:139` `Build`, `:155` `build` | - | нет | построение `gfx`-сцены, порядок рисования, `layer` |
| `go/scene/build.go:362` `picture`, `:64` `Box`, `:76` `Boxes` | - | нет | бокс, путь, клип, тень; боксы для попадания и стенда |
| `go/scene/placed.go:12` `Placed`, `:36` `Placed()` | - | нет | дерево нарисованных item в координатах экрана |
| `go/scene/linkroute.go:23` `routeWord`, `:82` `elbowCorner` | - | нет | маршрут связи: прямой, elbow, quad |
| `go/scene/anim.go:47` `Animation`, `:39` `Frame` | - | нет | анимации и keyframes |
| `go/scene/svg.go:19` `WriteSVG` | - | нет | экспорт SVG |
| `go/scene/toc.go:9` `anchorOf` | - | нет | якоря заголовков и оглавление |
| `go/scene/template.go:9` `Template`, `:26` `Render` | - | нет | генерация текста компонента |
| `go/scene/itemhtml.go:14` `ItemHTML` | - | нет | HTML одного item |
| `go/scene/classic.go:8` `ClassicRef`, `:18` `canEval` | - | нет | вычисление классической диаграммы в контексте сцены |
| `go/scene/refs.go:12` `RefKind` | - | нет | сбор ссылок путей для переименования |
| `go/scene/build.go:582` `MoveItem` | - | нет | перетаскивание абсолютного item и перерисовка связей |

## 2. Документ, модель, формат

| Go | Swift | Статус | Что сверять |
|---|---|---|---|
| `github.com/haiodo/tenn/go` (lexer/parser/printer, v1.0.1) | SPM `Tenn` v1.0.3, `Tenniarb/document/Document.swift:141` `TennParser` | есть | один и тот же код формата; номера расходятся только на публичную видимость Swift-API (1.0.2, 1.0.3), поведение парсинга общее - `TenniarbTests/TennTests.swift`, `TenniarbTests/PersistenceTests.swift` |
| `go/app/document.go:43` чтение/запись документа | `Tenniarb/document/Document.swift:134` `read(from:)`, `:180` `write(to:)` | есть | сохранение не теряет блоки, `--from-file` пишется строкой (`docs/spec/tenn-scene.md:69`) |
| `go/model/persistence.go:143` `roundPos` | `Tenniarb/ElementPersistence.swift:134` `roundPos` | сверено 2026-11-16 | координата пишется до 0.01, как в Go и web; стенд - общий голден `web/fixtures/Example.saved.tenn` в `TenniarbTests/TennGoldenTests.swift` |
| `go/app/doccore.go:28` печать текста | `Tenniarb/document/tenn/AttributedPrinter.swift:1` | есть | отступы/кавычки при записи; `TenniarbTests/PersistenceTests.swift` |
| `go/model` (3034) классическая модель | `Tenniarb/model/ElementModel.swift:89` `Element`, `:335` `DiagramItem`, `:370` `LinkItem` | есть | классика читается и рисуется; набор свойств - `Tenniarb/ElementPersistence.swift:53` |
| `go/model/scenekind.go:6` `SceneElement` | - | нет | выбор рисователя элемента по содержимому (спека 5a.17) |
| `go/model/mount.go` монтирование в модели | - | нет | `--from-file` как часть модели, не только чтения |
| Сценовая модель (auto-layout, use, slots, params, styles) | `Tenniarb/scene/SceneNode.swift` | начат | только дерево файла; модели правки узла нет |
| `go/model/undo_manager.go` | `Tenniarb/model/ElementOperations.swift:165` `ElementModelStore`, `:178` `execute` | есть | undo/redo одной правки; `TenniarbTests/OperationsTests.swift` |
| `go/exec/execution.go:151` goja | `Tenniarb/model/ExecutionModel.swift:566` `JSContext`, `:716` `ExecutionContext` | есть | `${...}` и `$(...)` дают тот же текст; `TenniarbTests/ExecutionContextTests.swift` |

## 3. Окна, меню, редактор

| Go | Swift | Статус | Что сверять |
|---|---|---|---|
| `go/app/shell.go:184` окно на документ | `Tenniarb/document/Document.swift:57`, `Tenniarb/WindowController.swift:1` | есть | открытие второго файла даёт второе окно |
| `go/app/menus.go:20` меню | `Tenniarb/Application.swift:92` | есть | состав меню: `../my-tasks/tenniarb/swift-parity.md` §1.1; визуально не проверено |
| `go/app/shell.go:24` опрос внешних изменений 500 мс | - | нет | файл на диске изменился - приложение подхватывает |
| `go/app/debounce.go:5` автосохранение 2 с | `Tenniarb/document/Document.swift:27` `autosavesInPlace` | есть | пауза и момент записи |
| `go/app/recovery.go:1` восстановление после kill | - | нет | папка `recovery/<дата-время>` и предложение при старте |
| `go/app/window.go:133` рамка окна | `Tenniarb/document/Document.swift:112` `saveWindowPosition` | есть | рамка переживает перезапуск |
| `go/app/sceneview.go:1` камера/зум/сцена | `Tenniarb/SceneDrawView.swift:2169` `draw`, `:1958` `zoomBounds`, `:2243` `zoomIn` | есть | классика; сценового ввода нет |
| `go/app/sceneedit.go:1` правка сцены | - | нет | выделение, перетаскивание, рисование связи |
| `go/app/sceneview.go:53` кэш до 4 деревьев на окно | - | нет | память на окно при переключении элементов |
| `go/app/indicators.go:13` точки за краем | `Tenniarb/SceneDrawView.swift:2017` `drawRulers` | есть | положение и вид точек |
| `go/app/menus.go:246` контекстные меню | `Tenniarb/SceneDrawView.swift:709` `createColorMenu`, `:2531` | есть | классический быстрый стиль; сценового меню нет |

## 4. Outline, поиск, Items, инспектор

| Go | Swift | Статус | Что сверять |
|---|---|---|---|
| `go/app/outline.go:20` outline | `Tenniarb/controllers/OutlineSidebarController.swift:22`, `Tenniarb/Outline-ViewController-Extension.swift:1` | есть | дерево элементов; строк сцены и монтирований нет |
| `go/app/items.go:59` панель Items | - | нет | item'ы элемента, hover «+», свёрнутость |
| `go/app/search.go` Goto Item | `Tenniarb/views/SearchBoxController.swift:22` | есть | поиск по имени и переход |
| `go/app/navhist.go` история навигации | - | нет | Back/Forward |
| `go/app/inspector.go:429` структурный инспектор | `Tenniarb/controllers/PropertiesPanelController.swift:23` | начат | у Swift текст выделенного элемента, полей нет |
| `go/app/shadowlist.go` редактор теней | - | нет | список теней |
| `go/app/colorpicker.go:1` пипетка | `Tenniarb/SceneDrawView.swift:709` | есть | выбор цвета; не проверено визуально |
| `go/app/prefs.go`, `go/config.go:46` 12 настроек | `Tenniarb/views/PreferencesController.swift:516` | есть | набор ключей: 12 против 8 (`swift-parity.md` §1.4) |

## 5. Библиотеки и компоненты

| Go | Swift | Статус | Что сверять |
|---|---|---|---|
| `go/library/find.go:22` поиск библиотеки | - | нет | встроенные и пользовательские библиотеки |
| `go/app/mountdoc.go:90` окно библиотеки | - | нет | правка библиотеки открывает её окно |
| `go/app/outline_scene.go:206` Add Library | - | нет | добавление библиотеки в документ |
| `go/app/component_preview.go` превью компонента | - | нет | превью и drag в холст |
| `libraries/*.tenn` встроенные библиотеки | - | нет | те же 5 файлов доступны |
| `go/scene/load.go:627` `tenniarb-library://` | - | нет | монтирование по схеме через overlay |

## 6. Форматы и экспорт

| Go | Swift | Статус | Что сверять |
|---|---|---|---|
| `go/app/export_png.go:20` PNG элемента | `Tenniarb/views/ExportManager.swift:153` `exportPng` | есть | тот же элемент и масштаб дают тот же размер PNG; `TenniarbTests/DocumentExportTests.swift` |
| `go/app/export_png.go:131` PNG выделения и 1..3 | - | нет | масштаб и границы выделения |
| `go/app/export_pages.go:1` PNG/PDF страниц | - | нет | страница на элемент и прогресс |
| `go/scene/svg.go:19` SVG | - | нет | SVG элемента |
| `go/app/exports.go:22` HTML с PNG | `Tenniarb/views/ExportManager.swift:298` `exportHtml` | есть | вложенная картинка и разметка |
| `go/app/export.go:57` интерактивный HTML | `Tenniarb/views/ExportManager.swift:244` `exportInteractiveHtml` | есть | тот же бандл `web/packages/embed` |
| `go/jsoncanvas` (934) | - | нет | чтение `.canvas` с потерями |
| `go/penimport` (3199) | - | нет | импорт pen |
| Экспорт PDF/JSON/`.tenn` одного элемента | `Tenniarb/views/ExportManager.swift:310`, `:340`, `:432` | есть | в Go нет; решить судьбу (`swift-parity.md` §6.5) |

## 7. Ссылки, комментарии, агент, CLI

| Go | Swift | Статус | Что сверять |
|---|---|---|---|
| `go/link` (348), схема `tenniarb://` | - | нет | Open Link/Copy Link, `CFBundleURLTypes` |
| `go/comments` (313), `go/app/comments.go` | - | нет | пины, треды, список |
| `go/mcpserver` (4851), `cmd/tenniarb-mcp`, `cmd/tenniarb-service` | - | нет | решение: не портировать (`swift-parity.md` §5) |
| `go/app/agent.go:1` присутствие агента | - | нет | Follow и метки правок |
| `go/app/debounce.go:8` автосохранение правки агента 10 с | - | нет | момент записи после правки |
| `go/cmd/tenniarb/main.go:27` `check`/`suggest`/`bundle`/`gen`/`install` | - | нет | headless CLI, решение: не портировать |
| `go/app/compare.go:1` Compare with Git | - | нет | сравнение с git |
| `go/app/qa.go:24` хуки QA | - | нет | `state/text/scene/doc/reset` для тестов |

## 8. Стенды сверки

| Go | Swift | Статус | Что сверять |
|---|---|---|---|
| `go/cmd/tennlayout/main.go:1` печать дерева | - | нет | вывод `layout` до и после порта; голдены `web/fixtures/scene/*.layout.json` |
| `go/cmd/tennlayout/main_test.go:13` `TestGoldensAreCurrent` | - | нет | голден отстал от фикстуры - тест падает |
| `web/packages/render/test/scene-parity.test.ts:1` сверка web | - | нет | эталон для Swift-сверки, сам тест не трогается |
| `go/scene/*_test.go` (86 файлов пакета) | `TenniarbTests/SceneLoaderTests.swift`, `TenniarbTests/SceneAutoLayoutTests.swift` | начат | новые тесты не запускались (GUI-тест требует разрешения владельца); переносить по одному сценарию из `go/scene/layout_test.go` и `fromfile_test.go` |
| `go/tenncorpus` golden/round-trip на `web/fixtures` | `TenniarbTests/TennGoldenTests.swift:1` | есть | тот же текст разбирается и печатается одинаково |
