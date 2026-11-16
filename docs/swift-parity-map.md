# Карта паритета: Go → Swift

Рабочий документ, 2026-11-16. Один рядок - один портированный юнит: что в Go, что ему отвечает в Swift,
насколько это сделано и что сверять. План работ - `../my-tasks/tenniarb/swift-parity.md`, здесь только
учёт. Ссылки `file:line` даны по текущему дереву; строки сдвигаются, сверять по имени символа.

## Как держать карту актуальной

- Любая правка порта (Go или Swift) правит свои рядки в том же коммите: добавился юнит в Go - рядок
  получает `нет` и инвариант в «Что сверять» до того, как появится код Swift.
- `сверено` ставится только после прогона сверки против Go, а не после «код похож». Для сцены это
  голдены `web/fixtures/scene/*.layout.json`, снятые `go/cmd/tennlayout` (сверяет
  `web/packages/render/test/scene-parity.test.ts`), `TestGoldensAreCurrent`
  (`go/cmd/tennlayout/main_test.go:13`), а со стороны Swift - `TenniarbTests/SceneParityTests.swift`
  (`dump` против тех же голденов с портом `gfx.EstimateText`). Для классики пиксельного равенства нет,
  поэтому её рядки остаются `есть`.
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
| `go/scene/names.go:14` `NormalizeName` | `SceneNode.swift` `normalizeName` | есть | буквы/метки/цифры сохраняются, всё прочее `_`; ключ сегмента - `Segment.key`, сравнение имён - NFC (`Segment.matches`); фикстура - кавычки и совпадающие ключи в `TenniarbTests/SceneResolveTests.swift` |
| `go/scene/names.go:26` `Segment`, `:82` `splitPath` | `SceneNode.swift` `Segment`, `splitPath`, `quoteName` | есть | точки, кавычки, `\"`; пустой сегмент, незакрытая кавычка и текст после кавычки - nil; тот же разбор у загрузчика (`pathText`) и у `at`/`replace` (`SceneExpand.swift` `find`); `TenniarbTests/SceneResolveTests.swift` `testSplitPathReadsQuotedSegments` |
| `go/scene/load.go:21` `Tree` | `SceneLoader.swift` `Tree` | начат | `root/main/files/order/src/diags`; `ctxs` (цепочка монтирований) есть, `overlay` не портирован |
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
| `go/scene/exprcheck.go:14` `checkExpressions` | - | нет | синтаксис выражений при загрузке; сообщение движка (`goja.CompilerSyntaxError`) в Swift было бы своё, и ни одна фикстура на него не опирается, поэтому порт отложен |
| `go/scene/resolve.go:86` `Resolve`, `:106` `lookup` | `SceneResolve.swift` `Tree.resolve`, `lookup`, `MountCtx`, `Tree.mapMounts` | сверено 2026-11-16 | первый сегмент ищется от scope вверх через точки монтирования (корень смонтированного файла продолжается в месте монтирования); сегмент в кавычках - по имени, голый - по ключу; ошибки `nothing named`/`ambiguous path` (кандидаты с позициями)/`is not defined here or above` как в Go; стенд - `TenniarbTests/SceneParityTests.swift` против `web/fixtures/scene/*.layout.json`, фикстура scene-name-classic (путь элемента и `use`); монтирование сверено на `library-screen` (мастер `design-kit.tenn`); `context-file` пропущен (`context.go`) |
| `go/scene/expand.go:20` `RNode`, `:58` `expander` | `SceneExpand.swift` `RNode`, `RKind`, `Expander`, `Tree.element` | сверено 2026-11-16 | `use` даёт экземпляр с именем из `as`, детей из компонента; слот по имени и по умолчанию заменяются содержимым, незаполненный остаётся fallback, `slot {}` его снимает; `at`/`replace`; `pos` компонента с корня убирается; цикл - `cycle: ...`; контекст (5f) не портирован; значения параметров вычисляются вычислителем (`Expander.eval`); слоты и `at` сверены на `library-screen` (три экземпляра `kit.Card`, `at Foot`), `replace` - на `slots-dialog` (`replace Box.Content` со слотом `fields`); стенд - `TenniarbTests/SceneParityTests.swift`, фикстуры scene-name-classic, library-screen, slots-dialog |
| `go/exec/scene_eval.go:15` `SceneEvalVars`, `:36` `sceneValue` | `Tenniarb/scene/SceneEval.swift` `sceneEvalVars`, `sceneValue`, `canonicalNumber` | сверено 2026-11-16 | `ctx.*` собираются в один объект `ctx`, переменная - число только в канонической записи (`007`, `1.50`, `1e5` остаются текстом, как `FormatFloat(f, 'f', -1, 64)`), сбой вычисления - `false` и ничего не применяется, контекст JS свой на каждый вызов; стенд - `TenniarbTests/SceneParityTests.swift`, фикстуры param-metrics, theme-styles, design-kit, library-screen, hero, button-styles (значения параметров, шаблоны, `use-style ${...}`); арифметики над числами в них нет - её держит `TenniarbTests/SceneEvalTests.swift`, повтор `go/exec/scene_eval_test.go`; `SceneEval` без переменных не портирован - это `sceneEvalVars(code, nil)` |
| `go/scene/params.go:61` `params`, `:92` `defaults`, `:32` `evalArg`, `:49` `evalValue`, `:119` `evalWords` | `SceneParams.swift` `Expander.params`, `defaults`, `value`, `evalArg`, `evalValue`, `evalWords` | сверено 2026-11-16 | значение из блока `use` выше умолчания, остальные - умолчания (каждое вычисляется в своём scope); обязательный без значения - `element "C" needs "size": set it in the block of the use`; значение-шаблон `"${...}"` идёт как template string, `undefined`/`NaN` - не значение; стенд - `TenniarbTests/SceneParityTests.swift`, фикстуры param-metrics (четыре экземпляра `Metric`), theme-styles, design-kit, library-screen; `TenniarbTests/SceneExpandTests.swift` |
| `go/scene/styles.go:15` `applyStyles`, `:77` `inheritProps`, `:113` `StyleRefs` | `SceneStyles.swift` `applyStyles`, `inheritProps`, `styleRefs` | сверено 2026-11-16 | свойства стиля встают сразу после последнего `use-style` (выше - умолчания, ниже - свои), блоки детей как descendant-селектор в обратном порядке, шрифт наследуется item'ом; корень экземпляра берёт стили `Def`, остальные - ближайшего элемента; `use-style` с выражением берётся только после `evalTree` (`SceneBuild.swift`), несколько слов и `-имя`; стенд - `TenniarbTests/SceneParityTests.swift`, фикстуры theme-styles, design-kit, param-metrics, button-styles, library-screen (`use-style ${tone}`); `TenniarbTests/SceneStyleTests.swift` |
| `go/scene/styles.go:152` `classicDefaults`, `:180` `defaultProps` | `SceneStyles.swift` `classicDefaults`, `defaultProps` | сверено 2026-11-16 | `styles { item }` и `styles { line }` классического элемента - вид каждого item'а и каждой связи, а не стиль по имени; тень узла заменяет умолчание, `use-style item` снимает умолчание целиком; стенд - `TenniarbTests/SceneParityTests.swift`, фикстура classic-styles (заливка `#eef4ff`, тень `-5 -5 5`, связь `#cc3333` шириной 2 пунктиром) |
| `go/scene/context.go:48` `context`, `:23` `ctxDecl` | - | нет | `context`/`ctx` (спека 5f); блокирует context, context-classic, context-file (`go/scene/context.go`) |
| `go/scene/layout.go:106` `prep`, `:192` `root`, `:400` `item` | `Tenniarb/scene/SceneBuild.swift` `Prep`, `root`, `item` | сверено 2026-11-16 | узел `RNode` с детьми, `pos` - верхний левый угол, экранный `y = -pos.y`; `Scene.LayoutNode.node`/`text` - то, что Go держит в `frames`/`texts`; `prep.eval`/`evalProp`/`evalTree` вычисляют свойства до стилей, без вычислителя - предупреждение `expressions are not evaluated`; `prop` возвращает свойство без выражения как написано; тело - `SceneBody.swift` (`bodies`, `bodyAt`: тело flow-узла идёт блоком после детей); картинка корня и иконка листа не портированы (`image.go`, `icon.go`); стенд - `TenniarbTests/SceneParityTests.swift` против `web/fixtures/scene/*.layout.json`, все 24 сравниваемые фикстуры |
| `go/scene/layout.go:494` `container` | `SceneBuild.swift` `container` | сверено 2026-11-16 | `auto-layout row/column/abs`, `gap`, `line-gap`, `wrap`, `align`, `justify`; `line-gap`/`wrap` читаются через `num`/`flag`, как в Go (`line-gap takes a number`, `wrap takes yes or no`), и раскладываются `SceneLayout.swift` `layoutWrap`; стенд - `TenniarbTests/SceneParityTests.swift`, фикстуры auto-layout-tour (`width 320; wrap yes; gap 8; line-gap 8`, пятый тег уходит на вторую линию), slots-dialog (canvas 980 переносит три диалога 460), root-size, rotate, display-layer, gradient-shadow |
| `go/scene/layout.go:510` `padding` | `SceneBuild.swift` `padding`, `leaf` | сверено 2026-11-16 | 1 / 2 (вертикаль, горизонталь) / 4 (T R B L) значения; в листе `padX = left`, `padY = top`, как `layout.go:646`; стенд - `TenniarbTests/SceneParityTests.swift`, фикстура padding (классическая, дробная ширина `44.4`); `SceneBuildTests.swift` `testPaddingInsetsTheTextOfALeafWithinOnePoint` |
| `go/scene/layout.go:583` `size`, `:609` `sizeWord` | `SceneBuild.swift` `size`, `sizeWord` | сверено 2026-11-16 | число, `hug`, `fill`; `fill` без auto-layout родителя - hug; проценты - предупреждение; `min/max` только в layout-контексте, как в Go; стенд - `TenniarbTests/SceneParityTests.swift`, фикстуры root-size, gradient-shadow (hug/fill/fixed) |
| `go/scene/layout.go:486` `limits` | `SceneBuild.swift` `limits` | сверено 2026-11-16 | `min/max-width/height`, `0` в max - без предела; стенд - `TenniarbTests/SceneParityTests.swift`, фикстура button-styles (`min-width` на четырёх размерах); `SceneBuildTests.swift` `testMinAndMaxWidthClampAHugWithinOnePoint` |
| `gowt gfx/layout/layout.go:97` `layout.Node`, `:171` `Layout` | `SceneLayout.swift` `LayoutNode`, `layout()` | сверено 2026-11-16 | row/column/abs, fixed/hug/fill, gap/pad/align/justify, absolute, min/max, лист по `leafW`/`leafH`, `Wrap`/`LineGap` и `layoutWrap` (`:415`) с `wrappedColumnsW` (`:245`, ширина переносящейся колонки с фиксированной высотой) сверены на всех 24 фикстурах стенда `TenniarbTests/SceneParityTests.swift`; перенос - `SceneAutoLayoutTests.swift` `testWrapBreaksARowIntoLinesWithLineGap`, `testWrappingColumnHugsTheWidthOfItsColumns`; в param-metrics `wrap yes` не переносит строку: четыре карточки влезают в 990 |
| `go/scene/layout.go:629` `leaf` | `SceneBuild.swift` `leaf` | сверено 2026-11-16 | размер листа из текста и тела (имя/`title`, перенос, округление вверх; классика без округления - padding, Hug `44.4`): ширина - большее из `ti.natural()`/`body.natural()`, высота - `ti.height` + `body.height` + `2*padY`; тело в листе рисуется под заголовком, поэтому заголовок встаёт в левый верх, а не по центру; иконка не портирована (`icon.go`); стенд - `TenniarbTests/SceneParityTests.swift`, фикстуры padding, gradient-shadow, scene-name-classic, doc-body (тело из файла), onboarding-flow (`body "optional"` в блоке `at`), scene-text |
| `go/scene/layout.go:734` `textOf` | `SceneText.swift` `Prep.textOf`, `TextInfo` | сверено 2026-11-16 | имя/`title`, `font-*`, `line-height`, `line-spacing`, `layout left/right`, `marker` (из стиля); `evalText` заменяет каждый `${...}` значением, без вычислителя текст остаётся как написан; в классике заголовок режется разметкой (`titleMarkupParts`: `!(цвет)`, `&(размер)`, `**bold**`/`*italic*`/`` `code` `` через `titleEmphasisParts`), `@(картинка)` остаётся текстом до `image.go`; стенд - `TenniarbTests/SceneParityTests.swift`, все 24 фикстуры (наследование цвета и `font-size` - gradient-shadow, `title ${label}` - design-kit, param-metrics; разметка заголовка - classic-body, classic-styles) |
| `go/scene/layout.go:96` `lineBox`, `:1066` `natural`, `:1082` `height`, `:1094` `layout`, `:1187` `wrap` | `SceneText.swift` `lineBox`, `natural`, `height`, `layout`, `wrap`, `TitleLine`, `TitlePart` | сверено 2026-11-16 | `lineBox` (метрики `Hg`, округление, классические коэффициенты `:47-63`), `natural`, `height`, `wrap` сверены на всех 24 фикстурах стенда `TenniarbTests/SceneParityTests.swift`; `layout` (строки из частей заголовка, слова частями, перенос слишком длинного слова в классике) сверен на classic-styles и classic-body; `@(картинка)` в заголовке - `image.go` |
| `go/scene/measure.go:13` `MeasureText` | `SceneText.swift` `measureText` | есть | `Hg` и строки через CoreText, ширина/высота вверх; `SceneBuildTests.swift` `testMeasureTextRoundsItsLinesUpWithinOnePoint`; стенд паритета мерит не им, а портом `gfx.EstimateText` (`Scene.TextMeasurer(estimated: true)`), так что CoreText с Go не сверялся |
| `go/scene/body.go` (1183) `bodyText` | `Tenniarb/scene/SceneBody.swift` `BodyText`, `Prep.bodyInfo`, `titleSize`, `markdownArg`, `bodyFileText`, `bodySection`, `parse`, `inline`, `wrap`, `lines`, `height`, `natural` | сверено 2026-11-16 | тело item: `parse` блоками (заголовок 1-3 с размером `+6/+4/+2`, абзац, список, цитата с полосой, блок кода), `inline` разметкой (`**bold**`, `*italic*`, `` `code` `` моно, `!(цвет)`, `&(размер)`, `[label](url)`), `dedent`, `\n` классики, `classicBreaks` (переносы, которые go/render читает жёстко), `sep` (пробел в сцене, перевод строки в классике), список классики без отступа и с переносом слова по буквам, `body --from-file "page.md#Раздел"` (секция от заголовка до следующего того же уровня); ширина/высота тела идут в hug листа; стенд - `TenniarbTests/SceneParityTests.swift`, фикстуры doc-body (`--from-file`), scene-text (`body` у листа), onboarding-flow (`body` из блока `at`), classic-styles (разметка тела); `TenniarbTests/SceneBodyTests.swift`; не портировано из body.go: оглавление (`toc.go`), якоря заголовков (`toc.go`), ссылки тела (`bodylink.go`), картинки `@(name)` (`image.go`), `${...}` классики (`classic.go`) |
| `go/scene/bodylink.go:10` `BodyLink` | - | нет | ссылки и якоря внутри тела (`checkBodyLinks`, `BodyLines`-якоря); `bodyInfo` их не проверяет; ни одна фикстура ссылок в теле не имеет |
| `go/scene/paint.go:11` `gradient`, `:36` `fill`, `:103` `shadows` | `Tenniarb/scene/ScenePaint.swift` `Gradient`, `Prep.fill`, `Prep.shadows`, `Look` | сверено 2026-11-16 | заливка, градиент (угол, стопы, база - первый видимый стоп), тени (`DY = -dy`, blur 5 и `#00000055` по умолчанию, порядок в файле); стенд - `TenniarbTests/SceneParityTests.swift` против `web/fixtures/scene/*.layout.json`, фикстуры gradient-shadow, shadows-pair, display-layer (stack/text) |
| `go/scene/color.go:14` `parseColor`, `:63` `contrast` | `Tenniarb/scene/SceneColor.swift` `Scene.parseColor`, `Scene.contrast` | сверено 2026-11-16 | синтаксис цвета (имя cmkdown, 6/8 hex, `none`, каналы) и контраст текста на заливке; стенд - `TenniarbTests/SceneParityTests.swift`, все 24 фикстуры (`fill`/`border`/`textColor` в каждом узле) |
| `go/scene/icon.go:62` `icon` | - | нет | пути иконки внутри item (`path`, `view-box`), рисуются вместо рамки листа; дамп стенда путей не читает, поэтому button-styles и fusio-1620 сходятся и без порта, а картинка иконки - нет |
| `go/scene/image.go:30` `itemImage`, `:54` `rasterCache`, `:438` `inlineProvider`, `:453` `resolveInline` | - | нет | картинки item, base64/файл, кэш растров; блокирует classic-image (`go/scene/image.go`); в `style` пока `shortcut`: item с `image` берёт белые умолчания сцены; `@(name)` в теле и заголовке остаётся текстом - `SceneBody.swift` `inline` и `SceneText.swift` `titleMarkupParts` ждут его |
| `go/scene/build.go:139` `Build`, `:155` `build` | `Tenniarb/scene/ScenePlaced.swift` `Tree.build`, `buildElement`, `Tree.layout` | сверено 2026-11-16 | построение экрана: умолчания классики, разворот, выражения, стили, раскладка, `Placed`, предупреждения `warnOutside` и `warnBodyFit`; `Screen.W/H` и порядок рисования; картинки, иконки и анимации вне порта (свои рядки ниже), связи - `SceneLink.swift`, тела - `SceneBody.swift`; стенд - `TenniarbTests/SceneParityTests.swift`, все 24 фикстуры |
| `go/scene/build.go:362` `picture`, `:64` `Box`, `:76` `Boxes` | `ScenePlaced.swift` `Box`, `boxes()`, `Screen.placed`; `ScenePaint.swift` `Look` | есть | `Box`/`Boxes` (путь, имя, rect, глубина, строки текста) сверены на всех 24 фикстурах стенда; `picture` - запись `gfx.Picture` - не портирован, в Swift нет gfx, дамп читает данные стиля (`Look`) и тексты тела (`BodyText.lines`), а не декодирует картинку |
| `go/scene/placed.go:12` `Placed`, `:36` `Placed()` | `Tenniarb/scene/ScenePlaced.swift` `Placed`, `Screen.placed()` | сверено 2026-11-16 | дерево нарисованных item в координатах экрана (x/y родителя плюс rx/ry, `free`, `display none` без детей); стенд - `TenniarbTests/SceneParityTests.swift`, все 24 фикстуры |
| `go/scene/linkroute.go:23` `routeWord`, `:32` `isElbow`, `:35` `sideWord`, `:51` `routeSides`, `:73` `checkRoute`, `:89` `elbow`, `:123` `elbowSides`, `:165` `facingSides`, `:181` `sideVertical`, `:184` `sideAnchor`, `:197` `along`, `:205` `elbowPath` | `Tenniarb/scene/SceneLink.swift` `routeWord`, `isElbow`, `sideWord`, `routeSides`, `checkRoute`, `elbow`, `elbowSides`, `facingSides`, `sideVertical`, `sideAnchor`, `along`, `elbowPath`, `Scene.Point` | сверено 2026-11-16 | стороны `right/left/top/bottom`, обращённые - по центрам (при `abs(dx) >= abs(dy)` правый/левый); elbow: одно колено, когда горизонтальна одна сторона, середина отрезка, когда обе; связи одной стороны делят её в порядке концов, `f = (i+1)/(n+1)`; скругление угла `min(r, полсегмента)`, `corner-radius` связи перекрывает 12; диагностики `a route side is ...` и `route takes straight or elbow ...`; стенд - `TenniarbTests/SceneParityTests.swift`, фикстуры link-routes (elbow по именованным и обращённым сторонам, `corner-radius 4`), link-quad (`pos` контрольной точки, `layout quad`) |
| `go/scene/build.go:623` `link`, `:637` `linkPicture`, `:721` `linkControl`, `:734` `isQuad`, `:744` `edgePoint`; `go/scene/layout.go:360` `links` | `Tenniarb/scene/SceneLink.swift` `linkPicture`, `linkStyle`, `links`, `linkControl`, `isQuad`, `edgePoint`; `ScenePlaced.swift` `Screen.links` | сверено 2026-11-16 | конец связи - n-й item того же родителя с тем же именем (`source-index`/`target-index`, NFC), скрытый конец уводит связь с собой; прямая выходит из края коробки к центру другой, `pos` - контрольная точка (середина коробок плюс смещение, y вверх), `layout quad` - одна кривая; цвет, `line-width`, `line-style`; связь с `label` в `links` не попадает - она рисует текст, а `links` - только линии; `parent` в дампе - путь item'а, в чьих координатах линия; стенд - `TenniarbTests/SceneParityTests.swift`, фикстуры link-quad, link-routes, classic-link-index (`source-index`/`target-index`), classic-shapes (`display arrow`, dashed, `color`) |
| `go/scene/layout.go:345` `linkBoxes`, `go/scene/build.go:953` `LinkBox` | - | нет | коробка связи (объединение коробок концов) для выбора и наведения в холсте; потребителя в Swift нет, пока холст не рисует связи |

| `go/scene/anim.go:47` `Animation`, `:39` `Frame` | - | нет | анимации и keyframes; блокирует animations (`go/scene/anim.go`) |
| `go/scene/svg.go:19` `WriteSVG` | - | нет | экспорт SVG |
| `go/scene/toc.go:9` `anchorOf` | - | нет | якоря заголовков и оглавление; `bodyInfo` портирован без них (`toc` предупреждает, что ему нужно тело), ни одна фикстура `toc` не пишет |
| `go/scene/template.go:9` `Template`, `:26` `Render` | - | нет | генерация текста компонента |
| `go/scene/itemhtml.go:14` `ItemHTML` | - | нет | HTML одного item |
| `go/scene/classic.go:8` `ClassicRef`, `:18` `canEval` | - | нет | вычисление классической диаграммы в контексте сцены (`modelVals` читает scope модели: `size`, `base`, `cols`); `go/exec` уже есть, нужен сам `classic.go`; блокирует classic-values |
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
| `go/cmd/tennlayout/main.go:1` печать дерева | `TenniarbTests/SceneParityTests.swift` `dump` | сверено 2026-11-16 | тот же JSON, что у Go: `element`/`width`/`height`/`nodes`/`links`, ключи по одному, числа 3 знака, цвета `#rrggbb`/`#rrggbbaa`; меритель - порт `gfx.EstimateText` (`Scene.TextMeasurer(estimated: true)`), как у самого `tennlayout` (nil `Measurer`), а не CoreText; 24 фикстуры сходятся: root-size, padding, rotate, display-layer, gradient-shadow, shadows-pair, scene-name-classic, design-kit, hero, library-screen, param-metrics, theme-styles, button-styles, fusio-1620, link-quad, link-routes, classic-link-index, classic-shapes, classic-styles, doc-body, onboarding-flow, scene-text, auto-layout-tour, slots-dialog; 7 пропущены поимённо, причина и Go-файл - таблица `skipped` в `TenniarbTests/SceneParityTests.swift` (`classic.go`, анимации, картинки, контекст); вычислитель - `sceneEvalVars` (`SceneEval.swift`), как `EvalVars` у самого `tennlayout`; `links` пишутся из `Screen.links` - прямая, elbow и quad с цветом, `line-width`, `line-style` (`dash`/`phase`) и стрелкой, `parent` - путь item'а, в чьих координатах линия |
| `go/cmd/tennlayout/main_test.go:13` `TestGoldensAreCurrent` | - | нет | голден отстал от фикстуры - тест падает; Swift-стенд голдены только читает и не перегенерирует |
| `web/packages/render/test/scene-parity.test.ts:1` сверка web | - | нет | эталон для Swift-сверки, сам тест не трогается |
| `go/scene/*_test.go` (86 файлов пакета) | `TenniarbTests/SceneLoaderTests.swift`, `SceneAutoLayoutTests.swift`, `SceneResolveTests.swift`, `SceneExpandTests.swift`, `SceneStyleTests.swift`, `SceneBuildTests.swift`, `SceneBodyTests.swift`, `SceneParityTests.swift` | начат | наборы проходят (564 теста, 2026-11-16); сценарии переносятся по одному из `go/scene/layout_test.go`, `pathquote_test.go`, `scene_test.go`, `body_test.go`; картинка сверяется стендом `SceneParityTests` на 24 фикстурах |
| `go/tenncorpus` golden/round-trip на `web/fixtures` | `TenniarbTests/TennGoldenTests.swift:1` | есть | тот же текст разбирается и печатается одинаково |
