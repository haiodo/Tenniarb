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

        link center tl { route elbow top right; display arrow; line-style dotted; line-width 2; color #E0684B }
    }
}
```

## Компоненты

| Компонент | Параметры | Что рисует |
|---|---|---|
| `Tile` | `icon` (SVG-путь 24×24), `iconColor`, `rule` (`nonzero`/`evenodd`) | белая плитка 170×170 со скруглением и тенью |
| `Hub` | `icon`, `title` | карточка 420×310: внешняя плита с тенью, внутри вложенная с иконкой и моноширинной подписью |

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
