#  Element structure

    * Element
        * describe a structure.
        * Contains other elements.
        * Contains DiagramItem
    * DiagramItem
        * has a name.
        * Could be a link to some element and in this case display this element.
        * could be a link between two DiagramItems.




# Drawings structure

Drawable ->
    Line
    RoundBox
    Text
    
each could draw a child in offset of base coords.

Component: Drawable ->
    render() -> produce a list of drawables.
    mapState(state) -> map a state value based on some key.
    
    

