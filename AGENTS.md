# AGENTS.md

## О проекте

Tenniarb - нативное macOS-приложение (AppKit) для моделирования диаграмм с вычислениями.
Ключевая идея: структура + вычисления в формате `.tenn`.

- Язык/платформа: Swift, Xcode project (`Tenniarb.xcodeproj`)
- UI: программно (без storyboard/xib)
- Вычисления: `JavaScriptCore` (`ExecutionContext`)
- Markdown: пакет `cmkdown` (SwiftPM, см. `Package.resolved`)

## Структура репозитория

- `Tenniarb/` - основной код приложения
- `Tenniarb/model/` - модель, операции, undo/redo, вычисления
- `Tenniarb/document/` - NSDocument-слой, парсинг/чтение/запись `.tenn`
- `Tenniarb/document/tenn/` - lexer/parser Tenn (`TennLexer`, `TennParser`)
- `Tenniarb/views/` + `ViewController.swift` + `SceneDrawView.swift` - UI и взаимодействие
- `Tenniarb/ElementScene.swift` - рендер диаграмм/текста
- `web/` - TypeScript-версия (план в `plans/plan.md`, раздел 3). npm workspaces `packages/*`,
  тесты `node:test` на `.ts` без компиляции. `web/bench/` - бенчмарки рендера, вне workspaces.
- `TenniarbTests/` - unit tests (518 тестов, ~70% покрытия); `PerformanceTests` вынесены
  в схему `Tenniarb-Performance`. Схема скипает все остальные suite поимённо - при
  добавлении нового тестового класса список надо дополнить, иначе он попадёт в perf-прогон.
- `TenniarbUITests/` - UI tests (таргет собирается, но в схеме `Tenniarb` отключен: `skipped = "YES"`)
- `.github/workflows/` - CI и release pipeline

## Локальная сборка и тесты

Все команды собраны в `Makefile`, `make help` показывает список:

```bash
make build          # сборка Debug
make test           # unit-тесты (без performance)
make test-only T=TenniarbTests/LexerTests
make perf           # только performance-тесты (схема Tenniarb-Performance)
make lint           # SwiftLint
make lint-fix       # автокоррекции SwiftLint
make format         # swift-format --in-place
make format-check   # падает если код не отформатирован
make ci             # lint + build + test, то же что в CI
make clean
make web            # сборка web/packages
make web-test       # typecheck + тесты web/
```

Инструменты:
- **Линтер** - SwiftLint (`.swiftlint.yml`), ставится через `brew install swiftlint`.
- **Форматтер** - `swift-format` из Xcode toolchain (`.swift-format`), ставить нечего.

Performance-тесты исключены из основного прогона (36 сек из 36.5) через `SkippedTests` в схеме
`Tenniarb`. `-only-testing` не перекрывает `SkippedTests`, поэтому для них заведена отдельная
схема `Tenniarb-Performance`, а не флаг.

Если `xcodebuild` падает из-за plugin/runtime окружения Xcode, сначала привести локальный Xcode в рабочее состояние (`xcodebuild -runFirstLaunch`) и повторить.

## Правила внесения изменений

1. Сохранять совместимость формата `.tenn`.
- Любые изменения в parser/lexer/persistence должны не ломать чтение существующих файлов.

2. Не обходить `ElementModelStore` при изменениях модели из UI.
- Операции должны идти через store, чтобы сохранялись undo/redo, уведомления и пересчет.

3. Учитывать потокобезопасность вычислительного движка.
- `ExecutionContext` синхронизирован через `syncQueue`; не добавлять доступ к JS-контексту в обход этой модели.

4. Сохранять текущий стиль построения UI.
- Окна/контроллеры создаются программно (`Document`, `Application`, `AppDelegate`), не добавлять storyboard/xib без явной необходимости.

5. Не вносить случайные изменения в `project.pbxproj`.
- Менять version/build settings только когда это часть задачи.

## Минимальная валидация перед сдачей

1. Для любых Swift-изменений:
- `swiftlint lint --config .swiftlint.yml`

2. Для parser/lexer/persistence:
- `TenniarbTests/TennTests.swift`
- `TenniarbTests/PersistenceTests.swift`

3. Для Markdown/рендера текста:
- `TenniarbTests/MarkDownTests.swift`
- релевантные участки в `ElementScene.swift`

4. Для рендера/экспорта:
- `TenniarbTests/svh-generate/TestSVGGenerate.swift` (если затронут экспорт/scene rendering)

5. Для изменений в логике модели:
- релевантные тесты + проверка, что undo/redo и refresh-события не регресснули.

## CI и релизы

- CI workflow (`.github/workflows/ci.yml`) запускает SwiftLint, затем собирает Debug и запускает tests на `macos-latest`.
- Release workflow (`.github/workflows/release.yml`) запускается по тегу:
  - собирает Release
  - архивирует `Tenniarb.app` в zip
  - публикует GitHub Release
  - обновляет `MARKETING_VERSION` в `Tenniarb.xcodeproj/project.pbxproj` на default branch

- CI (`ci.yml`, job `app`) собирает Tauri-приложение (`web/packages/app`) на macOS/Windows/Linux.
- Release web workflow (`.github/workflows/release-web.yml`) по тем же тегам собирает unsigned dmg/NSIS/AppImage/deb и дописывает `Tenniarb-app-<версия>-*` в релиз тега (версия из тега через `tauri build --config`, не коммитится). Оба workflow создают релиз только если его нет (`gh release view || gh release create`), порядок не важен.
- Не проверено в GitHub Actions до первого запуска.

Перед изменениями release-логики проверять, что шаг обновления версии в `project.pbxproj` останется консистентным.

## Что не является целью в обычной задаче

- Рефакторинг всей архитектуры UI
- Массовая смена формата `.tenn`
- Смена схемы релизов/тегов без явного запроса
