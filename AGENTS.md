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
- `TenniarbTests/` - unit/performance tests
- `TenniarbUITests/` - UI tests (в shared scheme таргет подключен, но `TenniarbUITests` отмечен как skipped)
- `.github/workflows/` - CI и release pipeline

## Локальная сборка и тесты

Основные команды (как в CI):

```bash
swiftlint lint --config .swiftlint.yml
```

```bash
xcodebuild -project Tenniarb.xcodeproj -scheme Tenniarb -configuration Debug \
  -derivedDataPath build clean build CODE_SIGNING_ALLOWED=NO
```

```bash
xcodebuild test -project Tenniarb.xcodeproj -scheme Tenniarb \
  -destination 'platform=macOS' -configuration Debug -derivedDataPath build \
  CODE_SIGNING_ALLOWED=NO CODE_SIGNING_REQUIRED=NO CODE_SIGN_IDENTITY="" DEVELOPMENT_TEAM=""
```

Точечный запуск тестов:

```bash
xcodebuild test -project Tenniarb.xcodeproj -scheme Tenniarb \
  -destination 'platform=macOS' \
  -only-testing:TenniarbTests/LexerTests
```

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

Перед изменениями release-логики проверять, что шаг обновления версии в `project.pbxproj` останется консистентным.

## Что не является целью в обычной задаче

- Рефакторинг всей архитектуры UI
- Массовая смена формата `.tenn`
- Смена схемы релизов/тегов без явного запроса
