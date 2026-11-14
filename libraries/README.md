# Библиотека C4

`c4.tenn` — компоненты для диаграмм C4: контекст (C1), контейнеры (C2), компоненты (C3).
Сцены `.tenn` считают вёрстку сами, поэтому уровни собираются из компонентов и `auto-layout`,
а не из абсолютных координат: фигуры растут по тексту, вкладываются друг в друга и не наезжают.

Демо — `examples/c4-demo.tenn` (элементы `Context`, `Containers`, `Components` и галлерея
компонентов `Gallery`). Все три уровня — одна и та же система CRM.

## Подключение

```tenn
element "c4" --from-file "../libraries/c4.tenn"

element "Context" {
    use c4.Person as rep { title "Sales rep" }
}
```

Путь в `--from-file` — от файла, который монтирует. Библиотека монтируется один раз: в документе
входа (`--from-file`) или прямо в файле экрана, как в демо. Внутри смонтированного файла повторного
`--from-file` не нужно, имена видны сверху вниз.

## Компоненты

| Компонент | Что рисует | Параметры | Варианты |
|---|---|---|---|
| `Person` | актор, `#08427B`, иконка человека | `title`, `technology`, `description`, `icon` | — |
| `SoftwareSystem` | система, `#1168BD` | `title`, `technology`, `description`, `icon` | — |
| `Container` | контейнер, `#438DD5` | `title`, `technology`, `description`, `icon` | — |
| `Component` | компонент, `#85BBF0` | `title`, `technology`, `description`, `icon` | — |
| `Database` | хранилище: `#438DD5`, крышка цилиндра, иконка | `title`, `technology`, `description`, `icon` | — |
| `Queue` | очередь: `#438DD5`, иконка из трёх полос | `title`, `technology`, `description`, `icon` | — |
| `ExternalSystem` | внешняя система, `#999999`, пунктирная рамка | `title`, `technology`, `description`, `icon` | — |
| `Boundary` | рамка с подписью | `title`, `type` | `enterprise`, `system`, `container`, `component` |
| `Relation` | подпись, техпротокол и стрелка | `label`, `technology`, `direction`, `stroke` | `right`, `left`, `down`, `up`; `solid`, `dashed` |
| `Legend` | строка легенды по шести цветам | `title` | — |
| `Title` | заголовок уровня и подзаголовок | `title`, `subtitle` | — |

Текст в фигурах белый по каноническим цветам C4. `technology` и `description` — необязательные:
пустые не занимают место. У каждой фигуры и у `Boundary` есть `slot body` — туда кладутся свои
узлы внизу рамки:

```tenn
use c4.SoftwareSystem as crm {
    title "CRM Platform"
    technology SaaS
    description "Deals, contacts and billing"
    width 360
    slot body {
        item Note { title "in review"; font-size 11 }
    }
}
```

`icon` — SVG-путь в `view-box 0 0 24 24`; пустая строка убирает иконку, свой путь её заменяет:

```tenn
use c4.SoftwareSystem as custom { title "Custom"; icon "M12 2l3 7h7l-6 5 2 8-6-4-6 4 2-8-6-5h7z" }
use c4.Person as plain { title "No icon"; icon "" }
```

Ширина по умолчанию — 220 (у `Component` 200), высота — по контенту. Ширину можно переопределить
на экземпляре (`width 360`); высоту не задавать.

## Связи

`Relation` — подпись сверху и стрелка снизу; направление задаёт `direction`, пунктир — `stroke`:

```tenn
use c4.Relation as uses { label uses; technology "HTTPS, web UI"; direction down }
use c4.Relation as reads { label "reads and writes"; technology SQL; direction down; stroke dashed }
```

«Использует [HTTP/JSON]» пишется как `label "uses [HTTP/JSON]"` — скобки просто часть подписи,
протокол можно вынести в `technology` (он рисуется строкой ниже, курсивом).

Стрелка не соединяет две фигуры сама: она живёт в потоке между ними. Приём — колонка шириной
фигуры, `align center`, сверху `Relation`, снизу адресат:

```tenn
item FromApi {
    auto-layout column
    width 220
    align center
    use c4.Relation as r { label "reads and writes"; technology SQL; direction down }
    use c4.Database as db { title "CRM DB"; technology "PostgreSQL 16" }
}
```

Так стрелка стоит ровно под своей фигурой и не наезжает подписями на соседей.

## Как собрать свой уровень

1. Канва на всю ширину, вертикальный поток: `item Canvas { auto-layout column; gap 22; padding 30; width 1080; color #F5F7FA }`.
2. Сверху `use c4.Title as head { title "…"; subtitle "C4 level N — …" }`.
3. Дальше `item Stage { auto-layout column; gap 12; align center }` — в нём ряды фигур.
4. Порядок C1: акторы в ряд → стрелки вниз → `Boundary` (`type enterprise`) с системой → стрелки → внешние системы.
5. C2: актор → `Boundary` (`type system`): контейнеры колонкой, хранилища — рядом внизу внутри рамки.
6. C3: `Boundary` (`type system`) → `Boundary` (`type container`) с компонентами в ряд → внутри той же
   системной рамки хранилища и стрелки к ним.
7. Внизу `use c4.Legend as legend { }`.

Ряды, которые должны стоять друг под другом, задаются колонками одной ширины (`width 220`): так
подписи, стрелки и фигуры выравниваются без координат.

## Ограничения движка

- В сцене `line-style` не влияет на рамку (`go/scene` его не читает), поэтому рамки `Boundary`
  сплошные. Пунктир у `ExternalSystem` собран из отдельных штрихов (`styles dash` и `tick`):
  они тянутся по ширине и высоте фигуры и выглядят пунктиром, но у `Boundary` так нельзя — его
  ширина равна содержимому.
- `Database` — это прямоугольник с крышкой-эллипсом и иконкой цилиндра, а не полный цилиндр:
  фигура с текстом внутри не умеет рисовать изогнутые боковины.
- Белый текст на `#85BBF0` (`Component`) по канону C4 даёт низкий контраст. Менять цвет текста
  только осознанно: `at Box { text-color #0B2C4E }`.
- Стрелка `Relation` — фиксированные 44×12 единиц сцены; длина не параметризована.
  Для длинных связей стрелка стоит по центру колонки, а не дотягивается до фигуры.

# Библиотека hub

`hub.tenn` — диаграмма «хаб и спутники»: карточка в центре, квадратные плитки вокруг, связи —
пунктирные ортогональные стрелки со скруглениями. Демо: `examples/hub-demo.tenn`.

## Подключение

```tenn
element "hub" --from-file "../libraries/hub.tenn"

element "Map" {
    item Canvas {
        title ""
        width 1800; height 900
        color #F1F2EF

        use hub.Hub as center { pos 700 -295 }
        use hub.Tile as tl { pos 150 -70; iconColor "#E0684B" }

        link center tl { route elbow top right; display arrow; line-style dashed; line-width 3; color #E0684B }
    }
}
```

## Компоненты

| Компонент | Параметры | Что рисует |
|---|---|---|
| `Tile` | `icon` (SVG-путь 24×24), `iconColor`, `rule` (`nonzero`/`evenodd`) | белая плитка 164×164 со скруглением и мягкой тенью в два слоя |
| `Hub` | `icon`, `title` | две плиты 390×390 и 290×286: внешняя со своей мягкой тенью, вложенная сдвинута вниз-вправо, у неё своя тень, иконка контуром и моноширинная подпись |

## Как это работает

- Связи — обычные `link` с `route elbow <выход> <вход>`: линия выходит из названной стороны и входит
  в названную, углы скругляются `corner-radius` (по умолчанию 12). Без сторон берутся обращённые
  друг к другу, так что пара плиток «напротив» читается и без подсказок.
- Несколько связей с одной стороны расходятся по ней в порядке плиток, к которым ведут: у хаба
  четыре стрелки сверху и снизу встают сами.
- `pos` у детей — вниз от верхнего края родителя (ось Y направлена вверх), поэтому координаты
  отрицательные.

## Ограничения

- Маршрутизации вокруг препятствий нет: линия идёт по одной оси и поворачивает один-два раза,
  но не обходит другие плитки.
- Иконки — заливка одним путём; контурные иконки требуют `border-color`, а не `color`.

# Библиотека native

`native.tenn` — одна библиотека нативных десктопных компонентов: macOS, Windows 11 и Linux, каждая в
светлой и тёмной теме. Внешний вид выбирается обёрткой `Theme`, а не отдельным файлом:

```tenn
element "native" --from-file "tenniarb-library://native"

element "Settings" {
    use native.Theme as skin {
        osName Macos
        theme Light
        slot content {
            use native.Window as window {
                winWidth 560
                winHeight 470
                slot content {
                    use native.Button as ok { text OK; kind default }
                }
            }
        }
    }
}
```

`osName` — `Macos`, `Windows` или `Linux`, `theme` — `Light` или `Dark`; по умолчанию `Macos` и `Light`.
Слова совпадают с именами item-палитр, поэтому регистр важен: `theme dark` — ошибка, `theme Dark` —
нужный вид. Слот наследует вид целиком, включая компоненты глубоко в дереве; вложенный `Theme`
переключает одну ветку. Демо: `examples/native-demo.tenn`. Элемент `Skins` показывает все шесть
вариантов рядом, а `Preferences`, `Workbench`, `MenuDemo` — полные экраны на `Macos` / `Light`.
Путь `tenniarb-library://native` работает в любом воркспейсе: библиотека ставится вместе с приложением.

## Как устроена тема

Каждый вид — это `item` библиотеки: в item темы лежит то, что у трёх систем общее, в item системы —
только отличия:

```tenn
item Light {
    field #ffffff; text #000000; onAccent #ffffff; font 13; /* общее для трёх */
    item Macos { surface #ececec; accent #0a84ff; radius 6; /* … */ }
    item Windows { surface #f3f3f3; accent #0067c0; radius 4; /* … */ }
    item Linux { surface #f6f5f4; accent #3584e4; radius 6; /* … */ }
}
item Dark { /* общее для трёх и те же три item */ }
```

`Theme` накладывает их на контекст двумя строками:

```tenn
context --from-item ${theme}
context --from-item ${theme + '.' + osName}
```

`--from-item <путь>` читает каждое свойство item как ключ контекста. Первая строка приносит общее,
вторая — отличия системы: последняя строка побеждает, а ключ, который она не называет, остаётся от
предыдущей. Выражений в палитре нет, поэтому компоненты читают обычные `ctx.surface`, `ctx.accent`,
`ctx.radius`. Отсюда два правила:

- Экран должен лежать внутри `Theme`; без неё ключей нет и `render` перечислит предупреждения. Корень
  самой библиотеки повторяет те же две строки для её элементов, поэтому открытый `libraries/native.tenn`
  показывает вид macOS.
- Поверхности не красят вручную: сырой `color #ffffff` останется белым в тёмной теме. Для этого есть
  `Window`, `Page` (`kind` = `card` `flat` `sidebar`) и `Panel`; они рисуют `ctx.surface`, `ctx.cardBg`
  и остальное.

Токены — свойства item в начале `libraries/native.tenn`: `surface`, `panel`, `field`, `button`,
`border`, `borderDisabled`, `divider`, `text`, `textSecondary`, `textDisabled`, `accent`, `selection`,
`selectionText`, `onAccent`, `headerBg`, `group`, `tabOff`, `shadowColor`, `radius`, `radiusSmall`,
`radiusPanel`, `windowRadius`, `font`, `fontSmall`, `cardBg`, `pageBg`, `sidebarBg`, `tabRule`.
Правка item меняет вид всех компонентов сразу. Новая система — это item в `Light` и `Dark` и её имя
в `osName`.

## Компоненты

| Компонент | Параметры | Что рисует |
|---|---|---|
| `Button` | `text`, `kind` = `normal` `default` `disabled` | кнопка 24 px; `default` залита акцентом |
| `TextField` | `value`, `kind`, `fieldWidth` | поле с рамкой 1 px |
| `Checkbox` | `text`, `checked`, `enabled` | квадрат 16 px, отмеченный залит акцентом |
| `Radio` | `text`, `checked`, `enabled` | круг 16 px с точкой акцента |
| `Combo` | `value`, `comboWidth`, `enabled` | поле с шевроном |
| `Tab` | `text`, `active`, `tabWidth` | вкладка 27 px; синяя линия только у Linux |
| `ToolItem` | `icon` (SVG-путь 14×14), `enabled` | кнопка тулбара 26×24 |
| `MenuBarItem` | `text`, `active` | пункт строки меню 24 px |
| `MenuItem` | `text`, `shortcut`, `checked`, `enabled`, `itemWidth` | строка меню 22 px: галочка, подпись, сочетание |
| `Menu` | `menuWidth`, слот `content` | всплывающая панель с тенью |
| `MenuSeparator` | `itemWidth` | линия 1 px |
| `TreeItem` | `text`, `depth`, `selected`, `expanded`, `sidebar`, `rowWidth` | строка дерева 20 px; `sidebar yes` — фон колонки |
| `TableHeader` | `rowWidth` | шапка 22 px: Name / Status / Updated |
| `TableRow` | `name`, `status`, `updated`, `selected`, `rowWidth` | строка 20 px с теми же колонками |
| `Group` | `title`, `groupWidth`, слот `content` | рамка с подписью |
| `Separator` | `lineWidth`, `vertical` | линия 1 px, горизонтальная или вертикальная |
| `StatusBar` | `message`, `barWidth` | полоса 22 px: сообщение слева, размер справа |
| `Dialog` | `title`, `dialogWidth`, слоты `content` и `footer` | заголовок, тело, футер с кнопками справа |
| `Window` | `winWidth`, `winHeight`, слот `content` | рамка экрана с фоном темы |
| `Page` | `pageWidth`, `pageHeight`, `kind`, слот `content` | область содержимого |
| `Panel` | `panelWidth`, `panelHeight`, слот `content` | полоса тулбара |

## Ширины

Строки, таблицы, группы, страницы и статус-бар принимают ширину параметром (`rowWidth`, `groupWidth`,
`pageWidth`, `barWidth`), а не тянутся `fill`: внутри компонента, у которого размер по содержимому,
`fill` схлопывается. Шапке и всем строкам одной таблицы передаётся одно и то же число — ширина страницы
минус её отступы и рамки.

## Ограничения

- Оконного хрома нет: заголовок окна, кнопки закрытия и ресайза — нативное окружение, не `.tenn`.
- Фокус, hover, нажатие, скроллбары не рисуются: видны только выбранная строка и кнопка по умолчанию.
- `TableHeader` и `TableRow` жёстко трёхколоночные (200, fill, 110); другая таблица собирается из item.
- Подпись `Group` стоит внутри верхней рамки: наложить текст на линию движок не умеет.

# Библиотека иконок

`icons.tenn` — набор lucide: **2350 иконок**, по одному `item` на иконку, форма та же, что у
`web/fixtures/scene/lib/icon.tenn`. Иконки контурные: вид задаёт `border-color` и `line-width`, а не
`color`.

```tenn
element "icons" --from-file "../libraries/icons.tenn"

use icons.zap as Lightning { width 32; height 32; border-color #0b1411; line-width 2.67 }
```

Имя иконки — то же, что в lucide (`zap`, `arrow-left`, `loader-circle`). Список:
`grep '^item' libraries/icons.tenn`.

## Обновление

```bash
make icons
```

Цель скачивает пакет `lucide-static` из npm, распаковывает, заново пишет `libraries/icons.tenn` и
обновляет `libraries/LICENSES/lucide.txt`. Результат детерминирован: повторный прогон на той же версии
даёт байт-в-байт тот же файл. После обновления — закоммитить оба файла.

Сам инструмент — `go/cmd/tennicons` (не генерированный код): у него есть режим `-format go` для таблицы
в бинарь, но в репозиторий таблица не кладётся — иконки живут данными в `.tenn`.

## Лицензия

Иконки — **ISC** (Lucide Icons and Contributors); часть производна от **Feather** — **MIT** (Cole Bemis).
Оба текста и список производных иконок: `LICENSES/lucide.txt`. Уведомления сохраняются при
распространении: файл лежит рядом с библиотекой и упомянут в её шапке.

## Ограничения

- Иконки с залитой точкой (`tag`, `key-round`, `vault` и ещё десять) рисуются контуром: один путь либо
  залит, либо штрихуется.
- Концы и стыки штриха — прямые (`butt`/`miter`), а lucide рисует скруглённые: на 20–32 pt угол чуть
  резче.
