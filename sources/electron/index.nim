# A entry point for my application.

import karax, karaxdsl, vdom, kdom, components

import vstyles

converter toCStr*(str: string): cstring =
  str.cstring

let sMainLayout = style(
  (StyleAttr.display, "grid"),
  (StyleAttr.height, "100%"),
  (StyleAttr.margin, "0 0 0 0"),
  (StyleAttr.padding, "0 0 0 0"),
  (StyleAttr.gridTemplateColumns, "220px auto"),
  (StyleAttr.gridTemplateRows, "40px auto 180px 20px"),
  (StyleAttr.gridTemplateAreas, """title title
                                   nav main
                                   nav props
                                   footer footer""")
)


proc createDom(): VNode =
  result = buildHtml(tdiv):
    tdiv:
      text "Hello world"


setRenderer createDom
