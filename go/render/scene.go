package render

import (
	"strings"
	"time"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/markdown"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/tenn"
)

// Options configure a scene. The zero value renders a light diagram with the default gfx measurer and
// no expression engine: tokens render as written.
type Options struct {
	DarkMode bool
	// Exec supplies the evaluated values of expressions. NewDrawableScene with nil renders tokens as written;
	// Build makes a context (NewExecContext) when nil.
	Exec model.IExecutionContext
	// NoEvaluate makes Build (and NewExecContext) leave expressions unevaluated.
	NoEvaluate bool
	// ExecLimit caps one evaluation round of a context Build makes; zero is exec.DefaultRoundLimit.
	ExecLimit time.Duration
	// Measurer measures text; gfx.DefaultMeasurer (the backend's, when one is linked in) when nil.
	Measurer gfx.TextMeasurer
	// FontFamily is the gfx font family for all text; empty is the backend's default sans face.
	FontFamily string
	// Items restricts the scene to these items (Swift: DrawableScene items:); nil builds element.Items.
	Items []*model.DiagramItem
	// SkipChildren builds the scene without items and links (a throwaway scene to size one item).
	SkipChildren bool
	// DecodeImage turns inline image data into pixels; StdImageDecoder (PNG, JPEG, GIF) when nil.
	DecodeImage ImageDecoder
	ScaleFactor float64
	// Padding is the margin around the scene when drawing, DefaultPadding when nil.
	Padding *float64
	// FirstID is the id of the first gfx node made; 1 when zero.
	FirstID int
	// ScreenShadows keeps item and link shadows constant in screen points while the view zooms, as
	// CGContext.setShadow does in Swift. For a live window only: an export wants them scaled with the scene.
	ScreenShadows bool
}

// DefaultPadding is the margin renderElement leaves around the scene.
const DefaultPadding = 15.0

func (o Options) padding() float64 {
	if o.Padding != nil {
		return *o.Padding
	}
	return DefaultPadding
}

func parseLayer(s *DrawableStyle) Layer {
	if s.Layer != nil {
		switch *s.Layer {
		case "background":
			return LayerBackground
		case "hover":
			return LayerHover
		}
	}
	return LayerDefault
}

func parseLayout(style *DrawableItemStyle, horizontal, vertical TextPosition) (TextPosition, TextPosition) {
	if style.Layout != nil {
		ls := map[string]bool{}
		for _, t := range strings.Split(*style.Layout, ",") {
			ls[jsTrim(t)] = true
		}
		if ls["center"] {
			horizontal = Center
		}
		if ls["left"] {
			horizontal = Left
		}
		if ls["right"] {
			horizontal = Right
		}
		if ls["middle"] {
			vertical = Middle
		}
		if ls["top"] {
			vertical = Top
		}
		if ls["fill"] {
			vertical = Fill
		}
		if ls["bottom"] {
			vertical = Bottom
		}
	}
	return horizontal, vertical
}

// DrawableScene builds drawables from an Element; evaluated values come from the execution context
// exactly as in Swift. It implements model.LayoutScene.
type DrawableScene struct {
	DrawableContainer

	// Drawables maps every built item or link to its drawable.
	Drawables map[*model.DiagramItem]Drawable
	// ItemToLink lists the links touching an item.
	ItemToLink map[*model.DiagramItem][]*model.DiagramItem
	// LineTo is the preview line of a link being drawn (Swift lineToDrawable), nil when none.
	LineTo *DrawableLine

	Style    *SceneStyle
	DarkMode bool

	opts    Options
	env     *textEnv
	decoder ImageDecoder
	decoded map[string]*DecodedImage
	// elements is the container the items and links were built into.
	elements *DrawableContainer
	// order lists the built items in build order: boxes first, then links.
	order []*model.DiagramItem
}

var _ model.LayoutScene = (*DrawableScene)(nil)

// NewDrawableScene builds the drawables of element. Call Layout to lay them out.
func NewDrawableScene(element *model.Element, o Options) *DrawableScene {
	s := &DrawableScene{
		DrawableContainer: *NewDrawableContainer(),
		Drawables:         map[*model.DiagramItem]Drawable{},
		ItemToLink:        map[*model.DiagramItem][]*model.DiagramItem{},
		DarkMode:          o.DarkMode,
		opts:              o,
		env:               newTextEnv(o.Measurer, o.FontFamily),
		decoder:           o.DecodeImage,
		decoded:           map[string]*DecodedImage{},
	}
	if s.opts.ScaleFactor == 0 {
		s.opts.ScaleFactor = 1
	}
	if s.decoder == nil {
		s.decoder = StdImageDecoder
	}
	items := o.Items
	if items == nil {
		items = element.Items
	}
	s.elements = s.buildElementScene(element, !o.SkipChildren, items)
	s.Append(s.elements)
	return s
}

// GetBounds implements model.LayoutScene: the bounds of the whole scene.
func (s *DrawableScene) GetBounds() model.Rect {
	b := s.Bounds()
	return model.Rect{X: b.X, Y: b.Y, Width: b.Width, Height: b.Height}
}

// GetItemBounds implements model.LayoutScene.
func (s *DrawableScene) GetItemBounds(node *model.DiagramItem) (model.Rect, bool) {
	d, ok := s.Drawables[node]
	if !ok {
		return model.Rect{}, false
	}
	b := d.Bounds()
	return model.Rect{X: b.X, Y: b.Y, Width: b.Width, Height: b.Height}, true
}

// Layout lays the scene out inside bounds; dirty is the area to draw.
func (s *DrawableScene) Layout(bounds, dirty Rect) {
	s.bounds = bounds
	s.DrawableContainer.Layout(bounds, dirty)
}

// Traverse visits the scene and its direct children only.
func (s *DrawableScene) Traverse(op func(Drawable) bool) { traverseWith(s, s.Children, op) }

func (s *DrawableScene) evaluated(target model.EvalTarget) Evaluated {
	if s.opts.Exec == nil {
		return nil
	}
	return Evaluated(s.opts.Exec.GetEvaluated(target, nil, nil))
}

func (s *DrawableScene) newImageProvider(item *model.DiagramItem) *ElementImageProvider {
	p := NewElementImageProvider(item, s.opts.ScaleFactor, s.decode)
	return p
}

// decode caches decoded payloads: the same image is usually referenced by several items.
func (s *DrawableScene) decode(b64 string) *DecodedImage {
	if img, ok := s.decoded[b64]; ok {
		return img
	}
	img := s.decoder(b64)
	s.decoded[b64] = img
	return img
}

func (s *DrawableScene) register(e *model.DiagramItem, box interface {
	Drawable
	Append(Drawable)
}, textBox *TextBox, parent *DrawableContainer) {
	box.Append(textBox)
	s.Drawables[e] = box
	s.order = append(s.order, e)
	parent.Append(box)
}

func (s *DrawableScene) buildRoundRect(bounds Rect, style *DrawableItemStyle, e *model.DiagramItem, tb *TextBox, parent *DrawableContainer, fill bool, stack int) {
	b := NewRoundBox(bounds, style, fill)
	b.Stack = stack
	b.Item, b.layer = e, parseLayer(&style.DrawableStyle)
	s.register(e, b, tb, parent)
}

func (s *DrawableScene) buildCircle(bounds Rect, style *DrawableStyle, e *model.DiagramItem, tb *TextBox, parent *DrawableContainer) {
	b := NewCircleBox(bounds, style, true)
	b.Item, b.layer = e, parseLayer(style)
	s.register(e, b, tb, parent)
}

func (s *DrawableScene) buildEmptyRect(bounds Rect, style *DrawableStyle, e *model.DiagramItem, tb *TextBox, parent *DrawableContainer) {
	b := NewEmptyBox(bounds, style)
	b.Item, b.layer = e, parseLayer(style)
	s.register(e, b, tb, parent)
}

func (s *DrawableScene) buildInvisibleRect(bounds Rect, style *DrawableStyle, e *model.DiagramItem, tb *TextBox, parent *DrawableContainer) {
	b := NewInvisibleBox(bounds, style)
	b.Item, b.layer = e, parseLayer(style)
	s.register(e, b, tb, parent)
}

// inheritedProperties is the properties of e after `inherit` and `use-style`: the parent item's
// properties overridden by ours.
func (s *DrawableScene) inheritedProperties(e *model.DiagramItem) *model.ModelProperties {
	properties := e.Properties
	parentEl := e.Parent
	include := e.Properties.Get(model.PersistenceStyleName(model.StyleInherit))
	if include == nil || include.Count() <= 1 {
		return properties
	}
	ident, ok := include.GetIdent(1)
	if !ok {
		return properties
	}
	name := ident
	for parentEl != nil && strings.HasPrefix(name, "../") {
		name = name[3:]
		parentEl = parentEl.Parent
	}
	index, _ := include.GetInt(2)
	if parentEl == nil {
		return properties
	}
	var parentItems []*model.DiagramItem
	for _, it := range parentEl.Items {
		if it.Name == name {
			parentItems = append(parentItems, it)
		}
	}
	if index < 0 || index >= len(parentItems) {
		return properties
	}
	properties = parentItems[index].Properties.Clone()
	// Smart overide of all paren properties with ours
	for _, node := range e.Properties.Nodes() {
		var override *tenn.Node
		if nodeName, ok := node.GetIdentText(); ok && node.IsNamedElement() {
			override = properties.Get(nodeName)
		}
		if override != nil {
			override.Replace(node) // Replace existing with new one.
		} else {
			properties.Append(node.Clone())
		}
	}
	return properties
}

func (s *DrawableScene) buildItemDrawable(e *model.DiagramItem, elementDrawable *DrawableContainer) {
	style := s.Style.DefaultItemStyle.Copy()
	style.ScreenShadow = s.opts.ScreenShadows
	evaluatedValues := s.evaluated(e)
	images := s.newImageProvider(e)

	// parse uses with list of styles.
	properties := s.inheritedProperties(e)

	parent := e.Parent
	styleNode := e.Properties.Get(model.PersistenceStyleName(model.StyleUseStyle))
	var stylesBlock *tenn.Node
	if parent != nil {
		if sn := parent.Properties.Get("styles"); sn != nil {
			stylesBlock = sn.GetChild(1)
		}
	}
	if styleNode != nil && styleNode.Count() > 1 && styleNode.Children != nil && parent != nil && stylesBlock != nil {
		elementEvaluated := s.evaluated(parent)
		for _, c := range styleNode.Children[1:] {
			ident, ok := c.GetIdentText()
			if !ok {
				continue
			}
			if named := stylesBlock.GetNamedElement(ident); named != nil {
				if data := named.GetChild(1); data != nil {
					style.ParseStyle(data, elementEvaluated)
				}
			}
		}
	}

	style.ParseStyle(properties.AsNode(), evaluatedValues)

	titleValue := e.Name
	if len(titleValue) == 0 {
		titleValue = " "
	}
	if style.Title != nil {
		titleValue = *style.Title
	}
	titleValue = prepareBodyText(titleValue)
	// Marker value
	if style.Marker != nil {
		titleValue = *style.Marker + " " + titleValue
	}
	shift := Point{}

	var bodyAttr *AttributedString
	var bodyTokens []markdown.Token
	bodyFontSize := 0.0
	if bodyNode := properties.Get("body"); bodyNode != nil {
		// Body could have custome properties like width, height, color, font-size, so we will parse it as is.
		bodyStyle := style.Copy()
		bodyStyle.FontSize -= 2 // Make a bit smaller for body
		textValue := ""
		if block := bodyNode.GetChild(1); block != nil {
			if block.Kind == tenn.KindBlockExpr {
				bodyStyle.ParseStyle(block, evaluatedValues)
				if tn := block.GetNamedElement("text"); tn != nil {
					if txt, ok := getString(tn.GetChild(1), evaluatedValues); ok {
						textValue = txt
					}
				}
			} else if txt, ok := getString(block, evaluatedValues); ok {
				textValue = txt
			}
		}
		horizontal, vertical := parseLayout(bodyStyle, Left, Middle)
		prefix := ""
		if len(titleValue) > 0 {
			prefix = "\n"
		}
		bodyTokens = markdown.GetTokens(prefix + prepareBodyText(textValue))
		bodyFontSize = bodyStyle.FontSize
		bodyAttr = toAttributedString(bodyTokens, FontSpec{Size: bodyStyle.FontSize}, bodyStyle.TextColor(), &shift, images, []TextPosition{vertical, horizontal}, style.LineSpacing)
	}

	vertical, horizontal := Middle, Center
	titleTokens := markdown.GetTokens(titleValue)
	if bodyAttr != nil {
		vertical, horizontal = Fill, Left
	}
	// If markdown has titles, bullets, we need to change horizontal layout to left one.
	for _, t := range titleTokens {
		if t.Type == markdown.TokBullet || t.Type == markdown.TokTitle || t.Type == markdown.TokCode {
			horizontal = Left
			break
		}
	}
	horizontal, vertical = parseLayout(style, horizontal, vertical)

	attr := toAttributedString(titleTokens, FontSpec{Size: style.FontSize}, style.TextColor(), &shift, images, []TextPosition{horizontal, vertical}, 0)
	if bodyAttr != nil {
		attr.AppendString(bodyAttr)
	}
	textBounds := s.env.calculateSize(attr)

	offx, offy := 4.0, 4.0
	wx, wy := offx*2, offy*2

	width := textBounds.Width
	if style.Width != nil && *style.Width >= 1 {
		width = *style.Width
		wx = 0
	}
	height := textBounds.Height
	if style.Height != nil && *style.Height >= 1 {
		height = *style.Height
		wy = 0
	}
	if width-offx >= textBounds.Width {
		wx = 0
	}
	if height-offy >= textBounds.Height {
		wy = 0
	}
	if width-offx > textBounds.Width && width <= 20 {
		wx = 0
		offx = (width - textBounds.Width) / 2
	}
	if height-offy > textBounds.Height && height <= 20 {
		wy = 0
		offy = (height - textBounds.Height) / 2
	}

	sz := Size{width + shift.X + wx, height + shift.Y + wy}
	bounds := Rect{e.X, e.Y - sz.Height, sz.Width, sz.Height}

	ftb := Rect{offx, offy, textBounds.Width + shift.X, textBounds.Height + shift.Y}
	if ftb.Width+offx*2 < width {
		ftb.Width = width - offx*2
	}
	if ftb.Height+offy*2 < height {
		ftb.Height = height - offy*2
	}

	switch vertical {
	case Middle:
		if ftb.Height > textBounds.Height {
			yshift := ftb.Height - textBounds.Height
			ftb.Y = offy + yshift/2
			ftb.Height -= yshift
		} else if bounds.Height < ftb.Height {
			yshift := ftb.Height - bounds.Height + offy
			ftb.Y = offy - yshift/2
		}
	case Top:
		if ftb.Height > textBounds.Height {
			yshift := ftb.Height - textBounds.Height
			ftb.Y = offy + yshift
			ftb.Height -= yshift
		} else if bounds.Height < ftb.Height {
			yshift := ftb.Height - bounds.Height + offy
			ftb.Y = offy - yshift/2
		}
	case Fill:
		if bounds.Height < ftb.Height {
			yshift := ftb.Height - bounds.Height
			ftb.Y = -1 * (offy + yshift)
		}
	case Bottom:
		if ftb.Height > textBounds.Height {
			yshift := ftb.Height - textBounds.Height
			ftb.Y = offy
			ftb.Height -= yshift
		} else if bounds.Height < ftb.Height {
			yshift := ftb.Height - bounds.Height + offy
			ftb.Y = offy - yshift/2
		}
	}

	textBox := &TextBox{frame: ftb, attr: attr, env: s.env}
	fontSize := style.FontSize
	textBox.HTML = func() string {
		html := markdown.ToHTML(titleTokens, fontSize, "", images)
		if bodyTokens != nil {
			html += markdown.ToHTML(bodyTokens, bodyFontSize, "", images)
		}
		return html
	}

	display := ""
	if style.Display != nil {
		display = *style.Display
	}
	switch display {
	case "text":
		s.buildEmptyRect(bounds, &style.DrawableStyle, e, textBox, elementDrawable)
	case "no-fill":
		s.buildRoundRect(bounds, style, e, textBox, elementDrawable, false, 0)
	case "stack":
		s.buildRoundRect(bounds, style, e, textBox, elementDrawable, true, 3)
	case "circle":
		s.buildCircle(bounds, &style.DrawableStyle, e, textBox, elementDrawable)
	case "none":
		s.buildInvisibleRect(bounds, &style.DrawableStyle, e, textBox, elementDrawable)
	default:
		s.buildRoundRect(bounds, style, e, textBox, elementDrawable, true, 0)
	}
}

func (s *DrawableScene) buildElementScene(element *model.Element, buildChildren bool, items []*model.DiagramItem) *DrawableContainer {
	elementDrawable := NewDrawableContainer()

	s.Style = NewSceneStyle(s.DarkMode)
	evaluated := s.evaluated(element)
	if element.Properties != nil {
		s.Style.ParseStyle(element.Properties.AsNode(), evaluated)
	}

	if !buildChildren {
		return elementDrawable
	}
	var links []*model.DiagramItem
	for _, e := range items {
		switch e.Kind {
		case model.ItemKindItem:
			s.buildItemDrawable(e, elementDrawable)
		case model.ItemKindLink:
			links = append(links, e)
		}
	}
	for _, e := range links {
		if !e.IsLink() {
			continue
		}
		linkStyle := s.Style.DefaultLineStyle.Copy()
		linkStyle.ScreenShadow = s.opts.ScreenShadows
		linkStyle.ParseStyle(e.Properties.AsNode(), evaluated)
		sr := Rect{0, 0, 5, 5}
		tr := Rect{0, 5, 5, 5}

		if e.Source != nil {
			if d, ok := s.Drawables[e.Source]; ok {
				s.addLink(e.Source, e)
				sr = d.SelectorBounds()
			}
		}
		if e.Target != nil {
			if d, ok := s.Drawables[e.Target]; ok {
				s.addLink(e.Target, e)
				tr = d.SelectorBounds()
			}
		}
		line := NewDrawableLine(sr, tr, linkStyle, Point{e.X, e.Y})
		if len(e.Name) > 0 {
			line.AddLabel(e.Name, s.newImageProvider(e), s.env)
		}
		line.Item = e
		s.Drawables[e] = line
		s.order = append(s.order, e)
		elementDrawable.Insert(line, 0)
	}
	return elementDrawable
}

func (s *DrawableScene) addLink(item, link *model.DiagramItem) {
	s.ItemToLink[item] = append(s.ItemToLink[item], link)
}

// BuildItemDrawable builds one item into container (and the scene's Drawables); it is how an item is
// sized for expressions that read their own width and height.
func (s *DrawableScene) BuildItemDrawable(e *model.DiagramItem, container *DrawableContainer) {
	s.buildItemDrawable(e, container)
}
