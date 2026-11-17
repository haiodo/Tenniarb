//
//  InspectorController.swift
//  Tenniarb
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.
//

import Cocoa
import Tenn

/// The right column of a document: the properties of the selected scene node or classic item, as
/// go/app builds them. A field writes its own command; the value in effect is the hint.
@MainActor
final class InspectorController: NSObject, NSTextFieldDelegate, NSTextViewDelegate {
    /// The width Go gives the inspector; wide enough for a pair of fields without cutting them.
    static let width: CGFloat = 272
    /// How far the divider may move: the column is fixed in spirit, not in the way of a wider path.
    static let minWidth: CGFloat = 240
    static let maxWidth: CGFloat = 320

    /// One row as the tests and the column see it: the caption, the value the node writes, the hint of an
    /// unset field, the kind of control, and what a popup offers.
    struct InspectorRow {
        enum Kind: String {
            case text
            case popup
            case toggle
            case checks
            case info
            case button
            case slider
            case body
        }

        let section: String
        let label: String
        var text: String
        var placeholder = ""
        let kind: Kind
        var choices: [String] = []
        /// The segments of a size row and the one that is written, nil when none is.
        var segments: [String] = []
        var chosen: Int?
        /// The title of a button beside the field of a shadow row, and the layer it drops.
        var action = ""
        var layer = 0
    }

    private(set) var rows: [InspectorRow] = []
    /// The header the section title shows: Item, Frame, Instance, Slot, or Screen with nothing selected.
    private(set) var title = ""

    private(set) var containerView: NSView!

    private weak var canvas: SceneCanvasView?
    private weak var document: Document?

    private var stack: NSStackView?
    private var rowControls: [RowControl] = []
    /// The selection path the controls were built for; a different one rebuilds them.
    private var shownSelection: [Int]?
    /// The classic selection: one item or link, and the element shown when no single one is selected.
    private weak var classicItem: DiagramItem?
    private weak var classicElement: Element?
    /// The classic selection the controls were built for.
    private var shownClassic: String?

    /// Where a body edit goes: the item's body, or the description of the element when nothing is
    /// selected.
    private enum ClassicBodyTarget {
        case item(DiagramItem)
        case element(Element)
    }

    private struct PendingBody {
        let target: ClassicBodyTarget
        let text: String
    }

    /// The body field is written after a pause, as Go's propsDelay; the counter drops a queued write
    /// when the selection moves or the edit is committed early.
    private var pendingBody: PendingBody?
    private var bodyChanges = 0

    // MARK: - Wiring

    /// Points the column at a canvas and the document its edits go through.
    func attach(canvas: SceneCanvasView?, document: Document?) {
        self.canvas = canvas
        self.document = document
        canvas?.onSelectionChange = { [weak self] in self?.refresh() }
        refresh()
    }

    /// A classic document is a diagram without a scene tree; the canvas may be nil until it is built.
    private var isClassic: Bool { document?.sceneTree == nil }

    /// Points the classic column at the selection: one item or link, or the element when none is.
    func showClassic(items: [DiagramItem], element: Element?) {
        guard isClassic else { return }
        commitBody()  // the pending text belongs to the selection that is about to move
        classicItem = items.count == 1 ? items[0] : nil
        classicElement = element ?? classicItem?.parent
        refresh()
    }

    /// The element the classic column falls back to: the one the outline names, else the owner of the
    /// item, else the first diagram of the document.
    private func classicShownElement() -> Element? {
        classicElement ?? classicItem?.parent ?? document?.store?.model.elements.first
    }

    private var classicKey: String {
        if let item = classicItem {
            return (item is LinkItem ? "link:" : "item:") + item.id.uuidString
        }
        return "element:" + (classicShownElement()?.id.uuidString ?? "")
    }

    /// Builds the column: a scroll view around the sections, no horizontal bar, so a row that cannot wrap is
    /// cut at the fixed width rather than widening the pane.
    func createView() -> NSView {
        let root = NSView()
        root.translatesAutoresizingMaskIntoConstraints = false

        let scroll = NSScrollView()
        scroll.translatesAutoresizingMaskIntoConstraints = false
        scroll.hasVerticalScroller = true
        scroll.hasHorizontalScroller = false
        scroll.autohidesScrollers = true
        scroll.borderType = .noBorder
        scroll.drawsBackground = false
        scroll.contentView.drawsBackground = false
        root.addSubview(scroll)
        NSLayoutConstraint.activate([
            scroll.leadingAnchor.constraint(equalTo: root.leadingAnchor),
            scroll.trailingAnchor.constraint(equalTo: root.trailingAnchor),
            scroll.topAnchor.constraint(equalTo: root.topAnchor),
            scroll.bottomAnchor.constraint(equalTo: root.bottomAnchor),
        ])

        let content = NSView()
        content.translatesAutoresizingMaskIntoConstraints = false
        let stack = NSStackView()
        stack.translatesAutoresizingMaskIntoConstraints = false
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 10
        stack.edgeInsets = NSEdgeInsets(top: 10, left: 8, bottom: 10, right: 8)
        content.addSubview(stack)
        scroll.documentView = content
        NSLayoutConstraint.activate([
            stack.leadingAnchor.constraint(equalTo: content.leadingAnchor),
            stack.trailingAnchor.constraint(equalTo: content.trailingAnchor),
            stack.topAnchor.constraint(equalTo: content.topAnchor),
            stack.bottomAnchor.constraint(equalTo: content.bottomAnchor),
            content.widthAnchor.constraint(equalTo: scroll.contentView.widthAnchor),
        ])
        self.containerView = root
        self.stack = stack
        reloadUI()
        return root
    }

    // MARK: - Reading the selection

    /// The drawn node the rows describe: the selected item or instance, or the part of an instance the use
    /// around it edits through an at override.
    private struct Selection {
        let pl: Scene.Placed
        let node: Scene.Node?
        let part: SceneCanvasView.Part?
    }

    private func selection() -> Selection? {
        guard let canvas, let path = canvas.selection, let pl = canvas.placed(at: path) else { return nil }
        return Selection(pl: pl, node: pl.src, part: canvas.part(at: path))
    }

    /// Where an edit of the selection is written: the whole text of the file the command is in.
    private struct Context {
        let file: String
        let text: String
    }

    private func context(_ sel: Selection) -> Context? {
        guard let document, let tree = document.sceneTree else { return nil }
        let file: String
        if let part = sel.part {
            file = part.use.pos.file
        } else if let node = sel.node {
            file = node.pos.file
        } else {
            return nil
        }
        guard let text = tree.source(file) else { return nil }
        return Context(file: file, text: text)
    }

    /// The command the selected node writes itself; a part holds its own in the at override of the use.
    private func ownField(_ sel: Selection, _ name: String) -> Scene.Prop? {
        if let part = sel.part {
            guard !part.at.isEmpty, let at = atOf(part.use, part.at) else { return nil }
            return atProp(at, name)
        }
        guard let node = sel.node else { return nil }
        return ownProp(node, name)
    }

    private static func texts(_ p: Scene.Prop) -> [String] {
        (0..<p.args.count).map { p.text($0) ?? "" }
    }

    private func args(_ sel: Selection, _ name: String) -> [String] {
        if let own = ownField(sel, name) {
            return InspectorController.texts(own)
        }
        guard let eff = sel.pl.prop(name) else { return [] }
        return InspectorController.texts(eff)
    }

    // MARK: - Rows

    /// One editable field of the column: the command it writes, which argument, and what a popup offers.
    private struct Field {
        let section: String
        let label: String
        let kind: InspectorRow.Kind
        var property = ""
        var index = 0
        var choices: [String] = []
        var unset = ""
        var numeric = false
        var list = false
        var modes = false
    }

    private static let fields: [Field] = [
        Field(
            section: "Layout", label: "Auto layout", kind: .popup, property: "auto-layout",
            choices: ["none", "row", "column", "abs"], unset: "none"),
        Field(section: "Layout", label: "Gap", kind: .text, property: "gap", numeric: true),
        Field(
            section: "Layout", label: "Align", kind: .popup, property: "align",
            choices: ["none", "start", "center", "end"], unset: "none"),
        Field(
            section: "Layout", label: "Justify", kind: .popup, property: "justify",
            choices: ["none", "start", "center", "end", "space-between"], unset: "none"),
        Field(section: "Layout", label: "Clip", kind: .toggle, property: "clip"),
        Field(section: "Layout", label: "Position X", kind: .text, property: "pos", index: 0, numeric: true),
        Field(section: "Layout", label: "Position Y", kind: .text, property: "pos", index: 1, numeric: true),
        Field(section: "Layout", label: "Size W", kind: .text, property: "width", modes: true),
        Field(section: "Layout", label: "Size H", kind: .text, property: "height", modes: true),
        Field(section: "Layout", label: "Padding", kind: .text, property: "padding", list: true),

        Field(section: "Fill", label: "Color", kind: .text, property: "color"),

        Field(section: "Text", label: "Title", kind: .text, property: "title"),
        Field(section: "Text", label: "Color", kind: .text, property: "text-color"),
        Field(section: "Text", label: "Size", kind: .text, property: "font-size", numeric: true),
        Field(section: "Text", label: "Font", kind: .text, property: "font-family"),
        Field(section: "Text", label: "Weight", kind: .text, property: "font-weight"),

        Field(section: "Border", label: "Color", kind: .text, property: "border-color"),
        Field(section: "Border", label: "Width", kind: .text, property: "line-width", numeric: true),
        Field(
            section: "Border", label: "Style", kind: .popup, property: "line-style",
            choices: ["none", "solid", "dashed", "dotted"], unset: "none"),
        Field(section: "Border", label: "Radius", kind: .text, property: "corner-radius"),

        Field(
            section: "Shape", label: "Display", kind: .popup, property: "display",
            choices: ["rect", "no-fill", "circle", "stack", "text"], unset: "rect"),
        Field(
            section: "Shape", label: "Layer", kind: .popup, property: "layer",
            choices: ["default", "background", "hover"], unset: "default"),

        Field(section: "Effects", label: "Opacity", kind: .text, property: "opacity", numeric: true),
        Field(
            section: "Effects", label: "Shadow", kind: .popup, property: "shadow",
            choices: ["none", "soft", "floating", "custom"], unset: "none"),
        Field(section: "Effects", label: "Hidden", kind: .toggle, property: "hidden"),
    ]

    private func field(for row: InspectorRow) -> Field? {
        InspectorController.fields.first { $0.section == row.section && $0.label == row.label }
    }

    /// The shadows the preset popup writes, by the name it shows: a tight contact layer and a wide
    /// faint one, which is what reads as a soft shadow on a card or a plate.
    private static let shadowPresets: [String: [[String]]] = [
        "soft": [["0", "-4", "12", "#00000014"], ["0", "-14", "32", "#00000010"]],
        "floating": [["0", "-12", "20", "#00000012"], ["0", "-32", "64", "#0000000f"]],
    ]

    /// The layer of a shadow as the file writes it: an expression stays text, not its value.
    private static func shadowTexts(_ p: Scene.Prop) -> [String] {
        (0..<p.args.count).map { p.args[$0].getIdentText() ?? "" }
    }

    /// Reports whether the commands of the node are the layers of a preset, as written.
    private static func sameShadows(_ sh: [Scene.Prop], _ layers: [[String]]) -> Bool {
        sh.count == layers.count && zip(sh, layers).allSatisfy { shadowTexts($0.0) == $0.1 }
    }

    /// The shadow commands the selected node writes itself; a part holds its own in an at override,
    /// which the list does not edit.
    private func ownShadows(_ sel: Selection) -> [Scene.Prop] {
        guard sel.part == nil, let node = sel.node else { return [] }
        return shadowsOf(node)
    }

    /// The Shadow row of the Effects section: the preset the node's own layers spell out, custom for
    /// any others, none when it writes no layer.
    private func shadowPresetRow(_ sel: Selection) -> InspectorRow {
        let sh = ownShadows(sel)
        var text = "none"
        if !sh.isEmpty {
            text = "custom"
            for (name, layers) in InspectorController.shadowPresets where InspectorController.sameShadows(sh, layers) {
                text = name
                break
            }
        }
        return InspectorRow(
            section: "Effects", label: "Shadow", text: text, kind: .popup,
            choices: ["none", "soft", "floating", "custom"])
    }

    /// The rows of the shadow list: X, Y, blur and colour of every layer the node writes, a button that
    /// drops it, and the button that adds one.
    private func shadowRows(_ sel: Selection) -> [InspectorRow] {
        guard sel.part == nil, let node = sel.node else { return [] }
        var rows: [InspectorRow] = []
        for (i, p) in shadowsOf(node).enumerated() {
            let args = InspectorController.shadowTexts(p)
            func arg(_ k: Int) -> String { k < args.count ? args[k] : "" }
            let n = i + 1
            rows.append(InspectorRow(section: "Effects", label: "Shadow \(n) X", text: arg(0), kind: .text))
            rows.append(InspectorRow(section: "Effects", label: "Shadow \(n) Y", text: arg(1), kind: .text))
            rows.append(InspectorRow(section: "Effects", label: "Shadow \(n) Blur", text: arg(2), kind: .text))
            rows.append(
                InspectorRow(
                    section: "Effects", label: "Shadow \(n) Color", text: arg(3), kind: .text,
                    action: "−", layer: i))
        }
        rows.append(InspectorRow(section: "Effects", label: "+ Shadow", text: "", kind: .button))
        return rows
    }

    /// Which segment a size row shows: Fixed for a number, Hug or Fill for the word, none for an
    /// expression, and Fixed when the node writes no size.
    private func sizeMode(_ sel: Selection, _ name: String) -> Int? {
        guard let own = ownField(sel, name), let first = own.args.first else { return 0 }
        if first.kind == .Expression || first.kind == .ExpressionBlock {
            return nil
        }
        switch own.text(0) {
        case "hug": return 1
        case "fill": return 2
        default: return 0
        }
    }

    /// The layer and argument a shadow row of the list edits, nil for any other row.
    private static func shadowField(_ row: InspectorRow) -> (layer: Int, arg: Int)? {
        guard row.section == "Effects" else { return nil }
        let parts = row.label.split(separator: " ")
        guard parts.count == 3, parts[0] == "Shadow", let n = Int(parts[1]), n > 0 else { return nil }
        return ["X": 0, "Y": 1, "Blur": 2, "Color": 3][String(parts[2])].map { (n - 1, $0) }
    }

    /// One row as the current tree writes it.
    private func fieldRow(_ f: Field, _ sel: Selection) -> InspectorRow {
        if f.property == "shadow" {
            return shadowPresetRow(sel)
        }
        let own = ownField(sel, f.property)
        let eff = sel.pl.prop(f.property)
        var text = ""
        var placeholder = ""
        switch f.kind {
        case .text:
            if f.list {
                text = own.map(InspectorController.texts)?.joined(separator: " ") ?? ""
            } else if let own, f.index < own.args.count {
                text = own.text(f.index) ?? ""
            }
            if own == nil, f.index == 0, let eff {
                placeholder = InspectorController.texts(eff).joined(separator: " ")
            }
        case .popup:
            if let own {
                text = own.text(0) ?? ""
            } else if let eff {
                let value = InspectorController.texts(eff).joined(separator: " ")
                text = value.isEmpty ? f.unset : value
            } else {
                text = f.unset
            }
        case .toggle:
            text = own == nil ? "off" : "yes"
        case .info, .checks, .button, .slider, .body:
            break
        }
        var row = InspectorRow(
            section: f.section, label: f.label, text: text, placeholder: placeholder, kind: f.kind,
            choices: f.choices)
        if f.modes {
            row.segments = ["Fixed", "Hug", "Fill"]
            row.chosen = sizeMode(sel, f.property)
        }
        return row
    }

    /// Item, Frame, Instance, Slot: what the header calls the shown node, as go/app sceneSections.
    private func kindText(_ n: Scene.Node?) -> String {
        guard let n else { return "Screen" }
        switch n.kind {
        case .use:
            return "Instance"
        case .slot:
            return "Slot"
        case .item:
            return sceneKids(n).isEmpty ? "Item" : "Frame"
        default:
            return "Item"
        }
    }

    /// What go/editor SceneKids lists under a node: items and uses, and the slots of an item.
    private func sceneKids(_ n: Scene.Node) -> [Scene.Node] {
        n.content.filter { c in
            c.kind == .item || c.kind == .use || (n.kind == .item && c.kind == .slot)
        }
    }

    private func sizeText(_ w: Double, _ h: Double) -> String {
        func f(_ v: Double) -> String {
            if v.rounded() == v, abs(v) < 1e15 { return String(Int(v)) }
            return String(v)
        }
        return "\(f(w)) x \(f(h))"
    }

    // MARK: - Parameters and styles

    /// The use the Parameters section describes: the selected instance, or the use around a selected part.
    private func paramUse(_ sel: Selection) -> Scene.Node? {
        if let part = sel.part { return part.use }
        guard let node = sel.node, node.kind == .use else { return nil }
        return node
    }

    /// The component the use draws, nil when it declares no parameter.
    private func paramNode(_ sel: Selection) -> Scene.Node? {
        guard let document, let tree = document.sceneTree, let use = paramUse(sel),
            let scope = use.parent, let target = try? tree.resolve(scope, use.path),
            !target.params.isEmpty
        else { return nil }
        return target
    }

    private func param(_ sel: Selection, _ name: String) -> Scene.Param? {
        paramNode(sel)?.params.first { $0.name == name }
    }

    /// The group of the component's styles the default of prm names, as go/app paramGroup: the component's own
    /// groups, else those of the styles block above it.
    private func paramGroup(_ sel: Selection, _ prm: Scene.Param) -> Scene.StyleGroup? {
        guard let component = paramNode(sel), let first = prm.defaultValue?.text(0) else { return nil }
        let name = normalizeName(first)
        var scope = component
        if component.styleGroups.isEmpty {
            guard let above = styleScope(component) else { return nil }
            scope = above
        }
        return scope.styleGroups.first { $0.styles.contains(name) }
    }

    /// The values of the parameter as the file writes them: what the use sets, else the default.
    private func paramCurrent(_ sel: Selection, _ prm: Scene.Param) -> [String] {
        if let use = paramUse(sel), let own = ownProp(use, prm.name) {
            return InspectorController.texts(own)
        }
        return prm.defaultValue.map(InspectorController.texts) ?? []
    }

    /// The values the parameter holds in effect, as style names.
    private func paramValues(_ sel: Selection, _ prm: Scene.Param) -> [String] {
        paramCurrent(sel, prm).map(normalizeName)
    }

    /// The rows of the Parameters section: one per parameter of the component the selected use draws.
    private func parameterRows(_ sel: Selection) -> [InspectorRow] {
        guard let component = paramNode(sel) else { return [] }
        return component.params.map { prm in
            if let gr = paramGroup(sel, prm) {
                if gr.anyOf {
                    return InspectorRow(
                        section: "Parameters", label: prm.name,
                        text: checkedText(gr.styles, paramValues(sel, prm)), kind: .checks, choices: gr.styles)
                }
                let chosen = gr.styles.last(where: paramValues(sel, prm).contains) ?? ""
                return InspectorRow(
                    section: "Parameters", label: prm.name, text: chosen, kind: .popup, choices: gr.styles)
            }
            return InspectorRow(
                section: "Parameters", label: prm.name,
                text: paramCurrent(sel, prm).joined(separator: " "), kind: .text)
        }
    }

    // MARK: - Animation

    /// The rows of the Animation section: what plays on the selected item, a Play switch and a slider that
    /// walks through the frames.
    private func animationRows(_ sel: Selection) -> [InspectorRow] {
        guard let canvas, let a = canvas.screen?.animation(of: sel.pl) else { return [] }
        return [
            InspectorRow(section: "Animation", label: "Animation", text: InspectorController.animTitle(a), kind: .info),
            InspectorRow(
                section: "Animation", label: "Play", text: canvas.animationsPlaying ? "yes" : "off", kind: .toggle),
            InspectorRow(section: "Animation", label: "Frame", text: "", kind: .slider),
        ]
    }

    /// The animation as Go's animTitle shows it: the name, the duration and the easing.
    private static func animTitle(_ a: Scene.Animation) -> String {
        let d = a.duration
        if d.truncatingRemainder(dividingBy: 1) == 0 {
            return "\(a.name), \(Int(d))s \(a.easing)"
        }
        return "\(a.name), \(Int(d * 1000))ms \(a.easing)"
    }

    /// Stops the playback at frame percent 0..100, as dragging the Frame slider does.
    func scrub(_ frame: Int) {
        canvas?.scrubAnimations(Double(min(100, max(0, frame))) / 100)
        refresh()
    }

    /// The styles the selected node names in effect: the use-style values drawn, the overrides of the uses
    /// around a part already applied.
    private func nodeStyles(_ sel: Selection) -> [String] {
        styleRefs(sel.pl.props())
    }

    /// The groups of the nearest styles block above the selected node; a node that names no style gets none.
    private func styleGroups(_ sel: Selection) -> [Scene.StyleGroup] {
        guard !nodeStyles(sel).isEmpty, let node = sel.node, let scope = styleOwner(node) else { return [] }
        return scope.styleGroups.filter { !$0.styles.isEmpty }
    }

    /// The rows of the Styles section: a popup for one-of, a check box per style for any-of.
    private func styleRows(_ sel: Selection) -> [InspectorRow] {
        let values = nodeStyles(sel)
        return styleGroups(sel).map { gr in
            if gr.anyOf {
                return InspectorRow(
                    section: "Styles", label: gr.name, text: checkedText(gr.styles, values),
                    kind: .checks, choices: gr.styles)
            }
            let chosen = gr.styles.last(where: values.contains) ?? ""
            return InspectorRow(
                section: "Styles", label: gr.name, text: chosen, kind: .popup, choices: gr.styles)
        }
    }

    /// The styles of a group that are in effect, in the order of the group.
    private func checkedText(_ styles: [String], _ values: [String]) -> String {
        styles.filter(values.contains).joined(separator: " ")
    }

    /// Writes the values as the command of parameter prm in the block of the use: one edit, one undo step.
    private func setParam(_ sel: Selection, _ prm: Scene.Param, _ values: [String]) {
        guard let use = paramUse(sel), let ctx = context(sel) else { return }
        let line = ([prm.name] + values.map { paramValue(prm, $0) }).joined(separator: " ")
        if let own = ownProp(use, prm.name) {
            apply(Scene.TextEdit(start: own.range.start, end: own.range.end, new: line), in: ctx.file)
        } else {
            apply(insertEdit(ctx.text, use, line), in: ctx.file)
        }
    }

    /// Drops the value of parameter prm of the use, so the default of the declaration comes back.
    private func removeParam(_ sel: Selection, _ prm: Scene.Param) {
        guard let use = paramUse(sel), let ctx = context(sel),
            let edit = removeParamEdit(ctx.text, use, prm)
        else { return }
        apply(edit, in: ctx.file)
    }

    /// Adds a style to an any-of parameter, or drops it.
    private func toggleParam(_ sel: Selection, _ prm: Scene.Param, _ style: String) {
        var want = paramValues(sel, prm)
        if want.contains(style) {
            want.removeAll { $0 == style }
        } else {
            want.append(style)
        }
        setParam(sel, prm, want)
    }

    /// Makes style the choice of group gr: one-of replaces the styles of the group in place, any-of adds or
    /// drops it.
    private func setGroupStyle(_ sel: Selection, _ gr: Scene.StyleGroup, _ style: String) {
        var want = nodeStyles(sel)
        if gr.anyOf, want.contains(style) {
            want.removeAll { $0 == style }
        } else {
            want = withStyle(want, gr, style)
        }
        setStyles(sel, want)
    }

    /// Puts style where the styles of the group are in want, so a choice takes the place of the default it
    /// replaces; at the end when the group holds none of them.
    private func withStyle(_ want: [String], _ gr: Scene.StyleGroup, _ style: String) -> [String] {
        var out: [String] = []
        var done = false
        for name in want {
            if gr.styles.contains(name) {
                if !done {
                    out.append(style)
                    done = true
                }
                continue
            }
            out.append(name)
        }
        if !done {
            out.append(style)
        }
        return out
    }

    /// Writes the styles the node names as one edit: the list itself on a node of the file; on a part of an
    /// instance what differs from the defaults of its component, a dropped default by name (-medium).
    private func setStyles(_ sel: Selection, _ want: [String]) {
        guard let ctx = context(sel) else { return }
        if let part = sel.part {
            let base = nodeStyles(sel)
            if base == want {
                return
            }
            guard !part.at.isEmpty else {
                canvas?.setMessage(
                    "Not applied: the item has no unique name inside the instance, edit it in the use")
                return
            }
            let args = removals(base, want) + additions(base, want)
            guard let edit = atEdit(ctx.text, part.use, part.at, "use-style", args) else { return }
            apply(edit, in: ctx.file)
            return
        }
        guard let node = sel.node else { return }
        var args = want
        if ownUseStyle(node) > 1 {
            args = removals(nodeStyles(sel), want) + want
        }
        guard let edit = propEdit(ctx.text, node, "use-style", args) else { return }
        apply(edit, in: ctx.file)
    }

    /// The use-style commands written on n itself.
    private func ownUseStyle(_ n: Scene.Node) -> Int {
        if n.kind == .use {
            return n.block.filter { $0.kind == .prop && $0.prop?.name == "use-style" }.count
        }
        return n.props.filter { $0.name == "use-style" }.count
    }

    private func removals(_ have: [String], _ want: [String]) -> [String] {
        have.filter { !want.contains($0) }.map { "-" + $0 }
    }

    private func additions(_ have: [String], _ want: [String]) -> [String] {
        want.filter { !have.contains($0) }
    }

    private func build() -> (title: String, rows: [InspectorRow]) {
        if isClassic { return buildClassic() }
        guard let canvas, canvas.screen != nil else { return ("", []) }
        var head = "Screen"
        var name = canvas.screenNode?.name ?? ""
        var size = ""
        var component: String?
        let sel = selection()
        if let sel, let node = sel.node {
            head = kindText(node)
            name = node.name
            size = sizeText(sel.pl.w, sel.pl.h)
            if sel.part == nil, node.kind == .use {
                component = node.path
            }
        } else if let root = canvas.placed {
            size = sizeText(root.w, root.h)
        }
        var rows: [InspectorRow] = [
            InspectorRow(section: "Header", label: "Name", text: name, kind: .info)
        ]
        if let component {
            rows.append(InspectorRow(section: "Header", label: "Component", text: component, kind: .info))
        }
        rows.append(InspectorRow(section: "Header", label: "Size", text: size, kind: .info))
        guard let sel, sel.node != nil else { return (head, rows) }
        rows += parameterRows(sel)
        rows += animationRows(sel)
        rows += styleRows(sel)
        for f in InspectorController.fields where !(f.property == "shadow" && sel.part != nil) {
            rows.append(fieldRow(f, sel))
            if f.property == "shadow" {
                rows += shadowRows(sel)
            }
        }
        return (head, rows)
    }

    /// Rebuilds the values from the tree behind the canvas, and the controls when the rows themselves
    /// changed; a value the user is typing into is not overwritten.
    func refresh() {
        if isClassic {
            refreshClassic()
            return
        }
        let built = build()
        let path = canvas?.selection
        let selectionChanged = path != shownSelection
        shownSelection = path
        let keysChanged = built.rows.map(InspectorController.rowKey) != rows.map(InspectorController.rowKey)
        let titleChanged = built.title != title
        title = built.title
        rows = built.rows
        if selectionChanged || keysChanged || titleChanged {
            reloadUI()
        }
        for (i, row) in rows.enumerated() where rowControls.indices.contains(i) {
            rowControls[i].apply(row)
        }
    }

    private static func rowKey(_ r: InspectorRow) -> String {
        "\(r.section)|\(r.label)|\(r.kind.rawValue)|\(r.choices.joined(separator: ","))|\(r.segments.joined(separator: ","))"
    }

    /// The row with this caption: "Text.Color" and "Text Color" pick the Color of the Text section, a plain
    /// caption the first row that holds it.
    func row(_ label: String) -> InspectorRow? {
        let key = label.lowercased()
        if let exact = rows.first(where: { $0.label.lowercased() == key }) {
            return exact
        }
        let parts = key.replacingOccurrences(of: ".", with: " ").split(separator: " ", maxSplits: 1)
        guard parts.count == 2 else { return nil }
        let section = String(parts[0])
        let caption = String(parts[1])
        return rows.first { $0.section.lowercased() == section && $0.label.lowercased() == caption }
    }

    // MARK: - Writing

    /// Sets a text row, as typing into it and pressing Return does.
    func setText(_ value: String, for label: String) {
        guard let row = row(label) else { return }
        commitText(row, value)
    }

    /// Picks a choice of a popup row; the choice that stands for unset removes the command.
    func setChoice(_ choice: String, for label: String) {
        guard let row = row(label) else { return }
        applyChoice(row, choice)
    }

    /// Flips a toggle row: on writes the command with its words, off removes it.
    func toggle(_ label: String) {
        guard let row = row(label), row.kind == .toggle else { return }
        applyToggle(row, on: row.text != "yes")
    }

    /// Checks or unchecks a style of a check box group row, as a click on its box does.
    func setCheck(_ style: String, on: Bool, for label: String) {
        guard let row = row(label) else { return }
        applyCheck(row, style, on: on)
    }

    /// Picks Fixed, Hug or Fill of a size row, as a click on its segment does. Fixed takes the size the
    /// item is drawn with.
    func setSizeMode(_ mode: String, for label: String) {
        guard let row = row(label), let f = field(for: row), f.modes, let sel = selection() else { return }
        switch mode {
        case "Hug":
            write(f.property, ["hug"])
        case "Fill":
            write(f.property, ["fill"])
        case "Fixed":
            let measured = f.property == "height" ? sel.pl.h : sel.pl.w
            write(f.property, [String(format: "%.0f", measured)])
        default:
            return
        }
        refresh()
    }

    /// Adds a layer with the default contact shadow to the selected node's own shadows.
    func addShadow() {
        if isClassic {
            guard let item = classicItem, !(item is LinkItem) else { return }
            let layers = InspectorController.classicShadows(item) + [["0", "-2", "4", "#00000020"]]
            writeClassicShadows(layers, item)
            refresh()
            return
        }
        guard let sel = selection(), sel.part == nil, let node = sel.node else { return }
        let layers = shadowsOf(node).map(InspectorController.shadowTexts) + [["0", "-2", "4", "#00000020"]]
        writeShadowLayers(layers)
        refresh()
    }

    /// Drops the own shadow layer at index, as the − button of its row does.
    func removeShadow(at index: Int) {
        if isClassic {
            guard let item = classicItem, !(item is LinkItem) else { return }
            var layers = InspectorController.classicShadows(item)
            guard layers.indices.contains(index) else { return }
            layers.remove(at: index)
            writeClassicShadows(layers, item)
            refresh()
            return
        }
        guard let sel = selection(), sel.part == nil, let node = sel.node else { return }
        var layers = shadowsOf(node).map(InspectorController.shadowTexts)
        guard layers.indices.contains(index) else { return }
        layers.remove(at: index)
        writeShadowLayers(layers)
        refresh()
    }

    private func commitText(_ row: InspectorRow, _ raw: String) {
        guard row.kind == .text else { return }
        let value = raw.trimmingCharacters(in: .whitespacesAndNewlines)
        guard value != row.text else {
            refresh()
            return
        }
        if isClassic {
            commitClassicText(row, value)
            return
        }
        if row.section == "Parameters" {
            guard let sel = selection(), let prm = param(sel, row.label) else { return }
            if value.isEmpty {
                removeParam(sel, prm)  // an empty field drops the command: the default comes back
            } else {
                setParam(sel, prm, [value])
            }
            refresh()
            return
        }
        if let layer = InspectorController.shadowField(row) {
            guard layer.arg == 3 || value.isEmpty || isNumberOrExpr(value) else {
                refresh()  // an offset and a blur take a number or the source of an expression, as go/app
                return
            }
            setShadowText(value, layer: layer.layer, arg: layer.arg)
            return
        }
        guard let f = field(for: row) else { return }
        guard !f.numeric || value.isEmpty || isNumberOrExpr(value) else {
            refresh()  // a number field takes a number or the source of an expression, as go/app
            return
        }
        guard let sel = selection(), context(sel) != nil else {
            refresh()
            return
        }
        if f.list {
            let words = value.split(whereSeparator: { $0 == " " || $0 == "\t" }).map(String.init)
            write(f.property, words)
        } else if value.isEmpty && f.index == 0 {
            write(f.property, [])  // an empty field removes the command
        } else {
            var values = args(sel, f.property)
            while values.count <= f.index {
                values.append("0")
            }
            values[f.index] = value
            write(f.property, values)
        }
        refresh()
    }

    private func applyChoice(_ row: InspectorRow, _ choice: String) {
        guard row.kind == .popup, row.choices.contains(choice) else { return }
        if isClassic {
            applyClassicChoice(row, choice)
            return
        }
        guard let sel = selection() else { return }
        switch row.section {
        case "Parameters":
            guard let prm = param(sel, row.label) else { return }
            setParam(sel, prm, [choice])
            refresh()
        case "Styles":
            guard let gr = styleGroups(sel).first(where: { $0.name == row.label }) else { return }
            setGroupStyle(sel, gr, choice)
            refresh()
        case "Effects" where row.label == "Shadow":
            applyShadowPreset(choice)
            refresh()
        default:
            guard let f = field(for: row), f.choices.contains(choice) else { return }
            if choice == f.unset {
                write(f.property, [])
            } else {
                write(f.property, [choice])
            }
            refresh()
        }
    }

    private func applyCheck(_ row: InspectorRow, _ style: String, on: Bool) {
        guard row.kind == .checks, row.choices.contains(style), let sel = selection() else { return }
        let checked = row.text.split(whereSeparator: \.isWhitespace).map(String.init)
        guard checked.contains(style) != on else { return }
        switch row.section {
        case "Parameters":
            guard let prm = param(sel, row.label) else { return }
            toggleParam(sel, prm, style)
        case "Styles":
            guard let gr = styleGroups(sel).first(where: { $0.name == row.label }) else { return }
            setGroupStyle(sel, gr, style)
        default:
            return
        }
        refresh()
    }

    private func applyToggle(_ row: InspectorRow, on: Bool) {
        guard row.kind == .toggle else { return }
        if row.section == "Animation" {
            canvas?.setAnimationsPlaying(on)  // the switch of the preview writes nothing to the file
            refresh()
            return
        }
        guard let f = field(for: row) else { return }
        write(f.property, on ? ["yes"] : [])
        refresh()
    }

    /// Picks a preset of the Shadow row; custom is the state of layers written by hand and writes
    /// nothing, none drops every layer.
    private func applyShadowPreset(_ preset: String) {
        guard preset != "custom" else { return }
        writeShadowLayers(InspectorController.shadowPresets[preset] ?? [])
    }

    /// Writes layers as the shadow commands of the selected node, all of them as one undo step.
    @discardableResult
    private func writeShadowLayers(_ layers: [[String]]) -> Bool {
        writeShadows(layers.map { commandText("shadow", $0) })
    }

    @discardableResult
    private func writeShadows(_ cmds: [String]) -> Bool {
        guard let sel = selection(), sel.part == nil, let node = sel.node, let ctx = context(sel) else {
            return false
        }
        let edits = setShadowsEdit(ctx.text, node, cmds)
        guard !edits.isEmpty else { return false }
        return apply(edits, in: ctx.file)
    }

    /// Sets one argument of an own shadow layer: the whole list is written again as one edit.
    private func setShadowText(_ value: String, layer: Int, arg: Int) {
        guard let sel = selection(), sel.part == nil, let node = sel.node else { return }
        var layers = shadowsOf(node).map(InspectorController.shadowTexts)
        guard !value.isEmpty, layers.indices.contains(layer) else {
            refresh()
            return
        }
        while layers[layer].count <= arg {
            layers[layer].append("0")
        }
        layers[layer][arg] = value
        writeShadowLayers(layers)
        refresh()
    }

    @discardableResult
    private func write(_ name: String, _ args: [String]) -> Bool {
        guard let sel = selection(), let ctx = context(sel) else {
            refresh()
            return false
        }
        if let part = sel.part {
            guard !part.at.isEmpty else {
                canvas?.setMessage(
                    "Not applied: the item has no unique name inside the instance, edit it in the use")
                return false
            }
            guard let edit = atEdit(ctx.text, part.use, part.at, name, args) else { return false }
            return apply(edit, in: ctx.file)
        }
        guard let node = sel.node, let edit = propEdit(ctx.text, node, name, args) else { return false }
        return apply(edit, in: ctx.file)
    }

    /// setPropEdit for the array a field holds: the printer takes the values as one command, as the variadic
    /// producer does with its arguments.
    private func propEdit(_ text: String, _ node: Scene.Node, _ name: String, _ args: [String]) -> Scene.TextEdit? {
        guard !args.isEmpty else { return setPropEdit(text, node, name) }
        let line = commandText(name, args)
        if let own = ownProp(node, name) {
            return Scene.TextEdit(start: own.range.start, end: own.range.end, new: line)
        }
        return insertEdit(text, node, line)
    }

    /// atPropEdit for the array a field holds: the override goes into the at of the part, or a new at is
    /// written in the block of the use.
    private func atEdit(
        _ text: String, _ use: Scene.Node, _ path: String, _ name: String, _ args: [String]
    ) -> Scene.TextEdit? {
        guard !args.isEmpty else { return atPropEdit(text, use, path, name) }
        let line = commandText(name, args)
        let at = atOf(use, path)
        if let own = at.flatMap({ atProp($0, name) }) {
            return Scene.TextEdit(start: own.range.start, end: own.range.end, new: line)
        }
        if let at {
            let holder = Scene.Node(kind: .item, range: at.range, body: at.body)
            return insertEdit(text, holder, line)
        }
        return insertEdit(text, use, commandText("at", path) + " { " + line + " }")
    }

    /// Applies one text edit through the document; a refusal stays over the canvas and the old text stays.
    @discardableResult
    private func apply(_ edit: Scene.TextEdit, in file: String) -> Bool {
        apply([edit], in: file)
    }

    /// Applies a list of edits of one file as one undo step, as setShadowsEdit hands them over.
    @discardableResult
    private func apply(_ edits: [Scene.TextEdit], in file: String) -> Bool {
        guard let document else { return false }
        do {
            try document.applySceneEdits(edits, in: file)
            canvas?.setMessage(nil)
            if let next = document.sceneTree, let canvas, next !== canvas.shownTree {
                canvas.show(next)
            }
            return true
        } catch {
            canvas?.setMessage(error.localizedDescription)
            refresh()
            return false
        }
    }

    private func isNumberOrExpr(_ s: String) -> Bool {
        if Double(s) != nil { return true }
        return s.hasPrefix("${") || s.hasPrefix("$(")
    }

    // MARK: - Classic documents

    /// One row of the classic column: the value the selection writes and the edit that writes it back.
    private struct ClassicField {
        let section: String
        let label: String
        var kind: InspectorRow.Kind = .text
        var text = ""
        var choices: [String] = []
        var unset: String?
        var numeric = false
        var keep = false
        var action = ""
        var layer = 0
        var write: ((String) -> Void)?
    }

    /// The fields of the current classic selection; a write finds its row here by section and caption.
    private var classicFields: [ClassicField] = []

    private static let horizontalLayouts = ["left", "center", "right"]
    private static let verticalLayouts = ["top", "middle", "bottom", "fill"]
    /// The commands whose argument is text even when it looks like a number, as go/editor inspector.
    private static let stringProps: Set<String> = ["name", "label", "title", "marker"]

    /// The rows of a classic document: one item or link, or the element when nothing is selected.
    private func buildClassic() -> (title: String, rows: [InspectorRow]) {
        let item = classicItem
        let link = item as? LinkItem
        let element = classicShownElement()
        var fields: [ClassicField] = []

        if let link {
            fields.append(
                ClassicField(
                    section: "Header", label: "Label", text: link.name,
                    write: { [weak self] in self?.renameClassic($0) }))
        } else if let item {
            fields.append(
                ClassicField(
                    section: "Header", label: "Name", text: item.name,
                    write: { [weak self] in self?.renameClassic($0) }))
            fields.append(itemField("Header", "Title", "title", item))
        } else {
            fields.append(
                ClassicField(
                    section: "Header", label: "Name", text: element?.name ?? "",
                    write: { [weak self] in self?.renameClassic($0) }))
        }
        if let item {
            fields.append(
                ClassicField(
                    section: "Header", label: "Kind", kind: .info, text: link == nil ? "Item" : "Link"))
            if link == nil {
                fields.append(
                    ClassicField(
                        section: "Header", label: "Body", kind: .info,
                        text: InspectorController.bodyKind(item.properties)))
            }
        } else if let element {
            fields.append(
                ClassicField(
                    section: "Header", label: "Body", kind: .info,
                    text: InspectorController.bodyKind(element.properties)))
            fields.append(elementField("Header", "Grid X", "grid", 0, element))
            fields.append(elementField("Header", "Grid Y", "grid", 1, element))
        }

        // The body of the item, or the description of the element when nothing is selected.
        if link == nil {
            let text =
                item.map(InspectorController.classicBody)
                ?? element.map(InspectorController.elementDescription) ?? ""
            fields.append(
                ClassicField(
                    section: item == nil ? "Description" : "Body", label: "Body", kind: .body, text: text))
        }

        if let item, link == nil {
            fields.append(positionField("Position X", 0, item))
            fields.append(positionField("Position Y", 1, item))
            fields.append(itemField("Layout", "Size W", "width", item, numeric: true))
            fields.append(itemField("Layout", "Size H", "height", item, numeric: true))
            fields.append(itemField("Layout", "Padding", "padding", item, list: true))
            fields.append(itemField("Fill", "Color", "color", item))
        }

        if let item {
            fields.append(itemField("Text", "Color", "text-color", item))
            fields.append(itemField("Text", "Size", "font-size", item, numeric: true))
            if link == nil {
                fields.append(
                    layoutField(
                        "Text", "Align", InspectorController.horizontalLayouts,
                        InspectorController.verticalLayouts, item))
                fields.append(
                    layoutField(
                        "Text", "Vertical", InspectorController.verticalLayouts,
                        InspectorController.horizontalLayouts, item))
                fields.append(itemField("Text", "Marker", "marker", item))
            }
        }

        if let item, link == nil {
            fields.append(itemField("Border", "Color", "border-color", item))
            fields.append(itemField("Border", "Width", "line-width", item, numeric: true))
            fields.append(
                popupField(
                    "Border", "Style", "line-style", item,
                    ["none", "solid", "dashed", "dotted"], "none"))
            fields.append(itemField("Border", "Radius", "corner-radius", item, numeric: true))
        }

        if let item {
            if link == nil {
                fields.append(
                    popupField(
                        "Shape", "Display", "display", item,
                        ["rect", "no-fill", "circle", "stack", "text"], "rect"))
            }
            fields.append(
                popupField("Shape", "Layer", "layer", item, ["default", "background", "hover"], "default"))
        }

        if let item, link == nil {
            fields += shadowFields(item)
        }

        if let link {
            fields.append(itemField("Line", "Color", "color", link))
            fields.append(itemField("Line", "Width", "line-width", link, numeric: true))
            fields.append(
                popupField("Line", "Style", "line-style", link, ["solid", "dashed", "dotted"], "solid"))
            fields.append(
                popupField("Line", "Arrows", "display", link, ["solid", "arrow", "arrows", "arrow-source"], "solid"))
        }

        if item == nil, let element {
            let styles = InspectorController.classicStyles(element)
            fields.append(ClassicField(section: "Styles", label: "Named", kind: .info, text: styles.names))
            fields.append(ClassicField(section: "Styles", label: "Groups", kind: .info, text: styles.groups))
        }

        classicFields = fields
        let rows = fields.map {
            InspectorRow(
                section: $0.section, label: $0.label, text: $0.text,
                kind: $0.kind, choices: $0.choices, action: $0.action, layer: $0.layer)
        }
        let title = item == nil ? "Element" : (link == nil ? "Item" : "Link")
        return (title, rows)
    }

    private func refreshClassic() {
        let built = buildClassic()
        let key = classicKey
        let selectionChanged = key != shownClassic
        shownClassic = key
        let keysChanged = built.rows.map(InspectorController.rowKey) != rows.map(InspectorController.rowKey)
        let titleChanged = built.title != title
        title = built.title
        rows = built.rows
        if selectionChanged || keysChanged || titleChanged {
            reloadUI()
        }
        for (i, row) in rows.enumerated() where rowControls.indices.contains(i) {
            rowControls[i].apply(row)
        }
    }

    // MARK: - Reading the classic commands

    private static func args(_ node: TennNode?) -> [String] {
        (node?.children ?? []).dropFirst().map { $0.getIdentText() ?? "" }
    }

    private static func text(_ node: TennNode?, _ index: Int = 0) -> String {
        let values = args(node)
        return index < values.count ? values[index] : ""
    }

    /// None, Markdown or Image of a `body` command, as go/editor bodyKind.
    private static func bodyKind(_ props: ModelProperties) -> String {
        guard let body = props.get("body") else { return "None" }
        return args(body).joined(separator: " ").contains("image") ? "Image" : "Markdown"
    }

    private static func numberText(_ value: CGFloat) -> String {
        let rounded = (Double(value) * 100).rounded() / 100
        if rounded.rounded() == rounded, abs(rounded) < 1e15 { return String(Int(rounded)) }
        return String(rounded)
    }

    private static func layoutWords(_ item: DiagramItem) -> [String] {
        args(item.properties.get("layout")).flatMap {
            $0.split(whereSeparator: { $0 == "," || $0.isWhitespace })
        }.map(String.init)
    }

    private func itemField(
        _ section: String, _ label: String, _ name: String, _ item: DiagramItem,
        index: Int = 0, numeric: Bool = false, list: Bool = false
    ) -> ClassicField {
        let values = InspectorController.args(item.properties.get(name))
        let text: String
        if list {
            text = values.joined(separator: " ")
        } else {
            text = index < values.count ? values[index] : ""
        }
        return ClassicField(
            section: section, label: label, text: text, numeric: numeric,
            write: { [weak self] in self?.writeItem(item, name, $0, index: index, list: list) })
    }

    private func popupField(
        _ section: String, _ label: String, _ name: String, _ item: DiagramItem,
        _ choices: [String], _ unset: String
    ) -> ClassicField {
        let own = InspectorController.text(item.properties.get(name))
        return ClassicField(
            section: section, label: label, kind: .popup, text: own.isEmpty ? unset : own,
            choices: choices, unset: unset,
            write: { [weak self] in self?.writeItem(item, name, $0) })
    }

    private func layoutField(
        _ section: String, _ label: String, _ mine: [String], _ other: [String], _ item: DiagramItem
    ) -> ClassicField {
        let word = InspectorController.layoutWords(item).first(where: mine.contains) ?? ""
        return ClassicField(
            section: section, label: label, kind: .popup, text: word.isEmpty ? "none" : word,
            choices: ["none"] + mine, unset: "none",
            write: { [weak self] value in
                let keep = InspectorController.layoutWords(item).first(where: other.contains) ?? ""
                let out = (value.isEmpty ? [] : [value]) + (keep.isEmpty ? [] : [keep])
                self?.writeItem(item, "layout", out.joined(separator: " "), list: true)
            })
    }

    private func positionField(_ label: String, _ axis: Int, _ item: DiagramItem) -> ClassicField {
        ClassicField(
            section: "Layout", label: label,
            text: InspectorController.numberText(axis == 0 ? item.x : item.y),
            write: { [weak self] in self?.moveItem(item, axis, $0) })
    }

    private func elementField(
        _ section: String, _ label: String, _ name: String, _ index: Int, _ element: Element
    ) -> ClassicField {
        let values = InspectorController.args(element.properties.get(name))
        return ClassicField(
            section: section, label: label, text: index < values.count ? values[index] : "",
            numeric: true, keep: true,
            write: { [weak self] in self?.writeElement(element, name, $0, index: index) })
    }

    /// The shadow commands the item writes, each as the arguments of `shadow`.
    private static func classicShadows(_ item: DiagramItem) -> [[String]] {
        (item.properties.node.children ?? [])
            .filter { $0.kind == .Command && $0.getIdent(0) == "shadow" }
            .map { (node: TennNode) in (node.children ?? []).dropFirst().map { $0.getIdentText() ?? "" } }
    }

    /// The preset the layers spell out, custom for layers no preset names, none for no layer.
    private static func shadowPreset(_ layers: [[String]]) -> String {
        guard !layers.isEmpty else { return "none" }
        for (name, want) in shadowPresets where want == layers {
            return name
        }
        return "custom"
    }

    /// The rows of the Effects section: the preset popup and one field per layer, as the scene list.
    private func shadowFields(_ item: DiagramItem) -> [ClassicField] {
        let layers = InspectorController.classicShadows(item)
        var fields: [ClassicField] = [
            ClassicField(
                section: "Effects", label: "Shadow", kind: .popup,
                text: InspectorController.shadowPreset(layers),
                choices: ["none", "soft", "floating", "custom"], unset: "none",
                write: { [weak self] in self?.setClassicShadowPreset($0, item) })
        ]
        for (i, layer) in layers.enumerated() {
            func arg(_ k: Int) -> String { k < layer.count ? layer[k] : "" }
            let n = i + 1
            for (label, k) in [("X", 0), ("Y", 1), ("Blur", 2), ("Color", 3)] {
                fields.append(
                    ClassicField(
                        section: "Effects", label: "Shadow \(n) \(label)", text: arg(k),
                        numeric: k < 3, action: k == 3 ? "−" : "", layer: i,
                        write: { [weak self] in
                            self?.setClassicShadowText($0, layer: i, arg: k, item: item)
                        }))
            }
        }
        fields.append(ClassicField(section: "Effects", label: "+ Shadow", kind: .button))
        return fields
    }

    /// The body of the item as the inline editor shows it: `body "text"` or the text of a body block.
    private static func classicBody(_ item: DiagramItem) -> String {
        guard let body = item.properties.get("body") else { return "" }
        var node = body.getChild(1)
        if let block = node, block.kind == .BlockExpr {
            node = block.getNamedElement("text")?.getChild(1)
        }
        return prepareBodyText(node?.getIdentText() ?? "")
    }

    /// The description the element writes, if any.
    private static func elementDescription(_ element: Element) -> String {
        text(element.toTennAsProps(.BlockExpr).getNamedElement("description"))
    }

    /// A string for one line of text, markdown when the text carries a line break.
    private static func bodyNode(_ text: String) -> TennNode {
        if text.contains("\n") || text.contains("\\n") {
            return TennNode.newMarkdownNode(text)
        }
        return TennNode.newStrNode(text)
    }

    /// The element's styles block read the way Go's StyleNames and StyleGroups read it: named styles
    /// without the item and line defaults, and the group declarations as `name (one-of a b)`.
    private static func classicStyles(_ element: Element) -> (names: String, groups: String) {
        let block = element.properties.get("styles")?.getChild(1)
        var names: [String] = []
        var groups: [String] = []
        for cmd in block?.children ?? [] where cmd.kind == .Command {
            let name = cmd.getIdent(0) ?? ""
            guard !name.isEmpty else { continue }
            let args = (cmd.children ?? []).dropFirst().map { $0.getIdentText() ?? "" }
            if cmd.getChild(1)?.kind != .BlockExpr {
                let mode = args.first ?? ""
                guard mode == "one-of" || mode == "any-of" else { continue }
                let styles = args.dropFirst().map(normalizeName)
                groups.append("\(normalizeName(name)) (\(mode) \(styles.joined(separator: " ")))")
                continue
            }
            if name != "item" && name != "line" {
                names.append(name)
            }
        }
        return (
            names.isEmpty ? "None" : names.joined(separator: ", "),
            groups.isEmpty ? "None" : groups.joined(separator: ", ")
        )
    }

    // MARK: - Writing the classic commands

    private func commitClassicText(_ row: InspectorRow, _ value: String) {
        guard
            let f = classicFields.first(where: { $0.section == row.section && $0.label == row.label })
        else {
            refresh()
            return
        }
        guard !f.numeric || value.isEmpty || isNumberOrExpr(value) else {
            refresh()
            return
        }
        if value.isEmpty && f.keep {
            refresh()
            return
        }
        f.write?(value)
        refresh()
    }

    private func applyClassicChoice(_ row: InspectorRow, _ choice: String) {
        guard let f = classicFields.first(where: { $0.section == row.section && $0.label == row.label })
        else {
            refresh()
            return
        }
        f.write?(choice == f.unset ? "" : choice)
        refresh()
    }

    /// Writes one argument of a command of the item; empty drops it and the arguments after it, and a
    /// command left with no arguments is removed.
    private func writeItem(
        _ item: DiagramItem, _ name: String, _ value: String, index: Int = 0, list: Bool = false
    ) {
        guard let element = item.parent, let store = document?.store else { return }
        let args: [TennNode]
        if list {
            args = value.split(whereSeparator: \.isWhitespace).map {
                InspectorController.valueNode(String($0), string: false)
            }
        } else if let next = InspectorController.commandArgs(
            item.properties.get(name), value, index: index, keep: false,
            string: InspectorController.stringProps.contains(name))
        {
            args = next
        } else {
            return
        }
        let props = item.toTennAsProps(.BlockExpr)
        InspectorController.setCommand(props, name, args)
        store.setProperties(
            element, item, props, undoManager: document?.undoManager,
            refresh: { [weak self] in self?.refresh() })
    }

    private func writeElement(_ element: Element, _ name: String, _ value: String, index: Int) {
        guard !value.isEmpty, let store = document?.store else { return }
        var args = (element.properties.get(name)?.children ?? []).dropFirst().map { $0.clone() }
        while args.count <= index {
            args.append(TennNode.newIntNode(0))
        }
        args[index] = InspectorController.valueNode(value, string: false)
        let props = element.toTennAsProps(.BlockExpr)
        InspectorController.setCommand(props, name, args)
        store.setProperties(
            element, props, undoManager: document?.undoManager,
            refresh: { [weak self] in self?.refresh() })
    }

    /// Replaces every shadow the item writes with layers, as one undo step; none leaves it without.
    private func writeClassicShadows(_ layers: [[String]], _ item: DiagramItem) {
        guard let element = item.parent, let store = document?.store else { return }
        let props = item.toTennAsProps(.BlockExpr)
        props.removeNamed("shadow")
        for layer in layers {
            let cmd = TennNode.newCommand("shadow")
            cmd.add(layer.map { InspectorController.valueNode($0, string: false) })
            props.add(cmd)
        }
        store.setProperties(
            element, item, props, undoManager: document?.undoManager,
            refresh: { [weak self] in self?.refresh() })
    }

    /// Picks a preset of the classic Shadow row; custom is the state of layers written by hand.
    private func setClassicShadowPreset(_ preset: String, _ item: DiagramItem) {
        guard preset != "custom" else { return }
        writeClassicShadows(InspectorController.shadowPresets[preset] ?? [], item)
        refresh()
    }

    /// Sets one argument of a classic shadow layer; the whole list is written again as one edit.
    private func setClassicShadowText(_ value: String, layer: Int, arg: Int, item: DiagramItem) {
        var layers = InspectorController.classicShadows(item)
        guard !value.isEmpty, layers.indices.contains(layer) else {
            refresh()
            return
        }
        while layers[layer].count <= arg {
            layers[layer].append("0")
        }
        layers[layer][arg] = value
        writeClassicShadows(layers, item)
        refresh()
    }

    /// The text the body field holds: the pending edit, else what the selection writes.
    var bodyText: String {
        guard isClassic else { return "" }
        if let pending = pendingBody { return pending.text }
        guard let target = classicBodyTarget() else { return "" }
        return InspectorController.bodyText(of: target)
    }

    /// Commits a body edit at once, as leaving the field does.
    func setBody(_ text: String, for label: String) {
        guard rows.contains(where: { $0.kind == .body && $0.label == label }),
            let target = classicBodyTarget()
        else { return }
        pendingBody = PendingBody(target: target, text: text)
        commitBody()
    }

    private func classicBodyTarget() -> ClassicBodyTarget? {
        if let item = classicItem {
            return item is LinkItem ? nil : .item(item)
        }
        return classicShownElement().map(ClassicBodyTarget.element)
    }

    private static func bodyText(of target: ClassicBodyTarget) -> String {
        switch target {
        case .item(let item):
            return classicBody(item)
        case .element(let element):
            return elementDescription(element)
        }
    }

    /// Writes the body after the pause Go gives it, so a burst of typing is one undo step.
    private func scheduleBody(_ text: String, target: ClassicBodyTarget) {
        bodyChanges += 1
        let n = bodyChanges
        pendingBody = PendingBody(target: target, text: text)
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) { [weak self] in
            MainActor.assumeIsolated {
                guard let self, self.bodyChanges == n else { return }
                self.commitBody()
            }
        }
    }

    private func commitBody() {
        bodyChanges += 1
        guard let pending = pendingBody else { return }
        pendingBody = nil
        guard pending.text != InspectorController.bodyText(of: pending.target) else {
            refresh()
            return
        }
        writeBody(pending.text, to: pending.target)
        refresh()
    }

    /// Writes the body of the item, or the description of the element, as one model operation.
    private func writeBody(_ text: String, to target: ClassicBodyTarget) {
        guard let store = document?.store else { return }
        switch target {
        case .item(let item):
            guard let element = item.parent else { return }
            let props = item.toTennAsProps(.BlockExpr)
            if text.isEmpty {
                props.removeNamed("body")
            } else if let body = props.getNamedElement("body") {
                guard let block = body.getChild(1) else { return }
                let node = InspectorController.bodyNode(text)
                if block.kind == .BlockExpr {
                    guard let slot = block.getNamedElement("text") else { return }  // no text entry: left alone
                    slot.children?.removeAll()
                    slot.add(TennNode.newIdent("text"), node)
                } else {
                    body.children?.removeAll()
                    body.add(TennNode.newIdent("body"), node)
                }
            } else {
                props.add(TennNode.newCommand("body", InspectorController.bodyNode(text)))
            }
            store.setProperties(
                element, item, props, undoManager: document?.undoManager,
                refresh: { [weak self] in self?.refresh() })
        case .element(let element):
            let props = element.toTennAsProps(.BlockExpr)
            props.removeNamed("description")
            if !text.isEmpty {
                props.add(TennNode.newCommand("description", InspectorController.bodyNode(text)))
            }
            store.setProperties(
                element, props, undoManager: document?.undoManager,
                refresh: { [weak self] in self?.refresh() })
        }
    }

    /// Renames the item, link or element the column shows; the model keeps the item name, the file calls
    /// it label for a link.
    private func renameClassic(_ value: String) {
        guard !value.isEmpty, let store = document?.store else { return }
        if let item = classicItem, item.parent != nil {
            store.updateName(
                item: item, value, undoManager: document?.undoManager,
                refresh: { [weak self] in self?.refresh() })
        } else if let element = classicShownElement() {
            store.updateName(
                element: element, value, undoManager: document?.undoManager,
                refresh: { [weak self] in self?.refresh() })
        }
    }

    private func moveItem(_ item: DiagramItem, _ axis: Int, _ value: String) {
        guard !value.isEmpty, let store = document?.store, let number = Double(value),
            item.parent != nil
        else { return }
        var x = item.x
        var y = item.y
        if axis == 0 {
            x = CGFloat(number)
        } else {
            y = CGFloat(number)
        }
        store.updatePosition(
            item: item, newPos: CGPoint(x: x, y: y), undoManager: document?.undoManager,
            refresh: { [weak self] in self?.refresh() })
    }

    private static func commandArgs(
        _ node: TennNode?, _ value: String, index: Int, keep: Bool, string: Bool
    ) -> [TennNode]? {
        var args = (node?.children ?? []).dropFirst().map { $0.clone() }
        if value.isEmpty {
            if keep { return nil }
            if index < args.count {
                args.removeSubrange(index..<args.count)
            }
            return args
        }
        while args.count <= index {
            args.append(TennNode.newIntNode(0))
        }
        args[index] = valueNode(value, string: string)
        return args
    }

    private static func setCommand(_ props: TennNode, _ name: String, _ args: [TennNode]) {
        if args.isEmpty {
            props.removeNamed(name)
            return
        }
        if let cmd = props.getNamedElement(name) {
            cmd.children?.removeAll()
            cmd.add(TennNode.newIdent(name))
            cmd.add(args)
            return
        }
        let cmd = TennNode.newCommand(name)
        cmd.add(args)
        props.add(cmd)
    }

    /// The node for a written value: text for the string commands, a number for a number, a symbol for a
    /// word, a string for anything with blanks.
    private static func valueNode(_ text: String, string: Bool) -> TennNode {
        if string { return TennNode.newStrNode(text) }
        if text.hasPrefix("${") || text.hasPrefix("$(") { return TennNode.newIdent(text) }
        var body = Substring(text)
        if body.hasPrefix("-") { body = body.dropFirst() }
        var digits = 0
        var dot = false
        for c in body {
            if c == "." {
                if digits == 0 || dot { return TennNode.newIdent(text) }
                dot = true
            } else if c.isNumber {
                digits += 1
            } else if c.isWhitespace {
                return TennNode.newStrNode(text)
            } else {
                return TennNode.newIdent(text)
            }
        }
        guard digits > 0 else { return TennNode.newIdent(text) }
        let type: TennTokenType = dot ? .floatLit : .intLit
        return TennNode(kind: dot ? .FloatLit : .IntLit, tok: TennToken(type: type, literal: text))
    }

    // MARK: - The column

    private func reloadUI() {
        guard let stack else { return }
        for control in rowControls {
            control.detach()  // a field of the old rows must not write into the new selection
        }
        for view in stack.arrangedSubviews {
            stack.removeArrangedSubview(view)
            view.removeFromSuperview()
        }
        rowControls = []
        var section = ""
        var body: NSStackView?
        for (i, row) in rows.enumerated() {
            if row.section != section {
                section = row.section
                let header = NSTextField(labelWithString: row.section == "Header" ? title : row.section)
                header.font = NSFont.systemFont(ofSize: 11, weight: .semibold)
                header.textColor = .secondaryLabelColor
                header.lineBreakMode = .byTruncatingTail
                stack.addArrangedSubview(header)
                header.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -16).isActive = true
                let next = NSStackView()
                next.orientation = .vertical
                next.alignment = .leading
                next.spacing = 6
                stack.addArrangedSubview(next)
                next.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -16).isActive = true
                body = next
            }
            let control = RowControl(row: row, tag: i, target: self)
            body?.addArrangedSubview(control.view)
            control.view.widthAnchor.constraint(equalTo: body!.widthAnchor).isActive = true
            rowControls.append(control)
        }
    }

    // MARK: - Controls

    @objc fileprivate func textCommitted(_ field: NSTextField) {
        guard rows.indices.contains(field.tag) else { return }
        commitText(rows[field.tag], field.stringValue)
    }

    @objc fileprivate func popupChanged(_ popup: NSPopUpButton) {
        guard rows.indices.contains(popup.tag), let choice = popup.titleOfSelectedItem else { return }
        applyChoice(rows[popup.tag], choice)
    }

    @objc fileprivate func toggleChanged(_ button: NSButton) {
        guard rows.indices.contains(button.tag) else { return }
        applyToggle(rows[button.tag], on: button.state == .on)
    }

    @objc fileprivate func checkChanged(_ button: NSButton) {
        guard rows.indices.contains(button.tag), let style = button.identifier?.rawValue else { return }
        applyCheck(rows[button.tag], style, on: button.state == .on)
    }

    @objc fileprivate func sizeSegmentChanged(_ control: NSSegmentedControl) {
        guard rows.indices.contains(control.tag) else { return }
        let row = rows[control.tag]
        guard row.segments.indices.contains(control.selectedSegment) else { return }
        setSizeMode(row.segments[control.selectedSegment], for: row.label)
    }

    @objc fileprivate func animationFrameChanged(_ slider: NSSlider) {
        guard rows.indices.contains(slider.tag) else { return }
        scrub(Int(slider.doubleValue.rounded()))
    }

    @objc fileprivate func shadowButtonPressed(_ button: NSButton) {
        guard rows.indices.contains(button.tag) else { return }
        if rows[button.tag].kind == .button {
            addShadow()
        } else {
            removeShadow(at: rows[button.tag].layer)
        }
    }

    func controlTextDidEndEditing(_ obj: Notification) {
        guard let field = obj.object as? NSTextField else { return }
        textCommitted(field)
    }

    func textDidChange(_ notification: Notification) {
        guard let editor = notification.object as? NSTextView,
            let index = rowControls.firstIndex(where: { $0.bodyView === editor }),
            rows.indices.contains(index), rows[index].kind == .body,
            editor.string != rows[index].text,
            let target = classicBodyTarget()
        else { return }
        scheduleBody(editor.string, target: target)
    }

    /// Leaving the field writes the pending text at once, as Go's focus-lost commit does.
    func textDidEndEditing(_ notification: Notification) {
        guard notification.object is NSTextView else { return }
        commitBody()
    }
}

/// One row of the column: the caption on the left and the control on the right, built once and refilled
/// when the tree changes.
@MainActor
private final class RowControl {
    let view: NSView

    private let kind: InspectorController.InspectorRow.Kind
    private let info: NSTextField?
    private let text: NSTextField?
    private let popup: NSPopUpButton?
    private let toggleButton: NSButton?
    private let checkBoxes: [NSButton]
    private let segmented: NSSegmentedControl?
    private let action: NSButton?
    private var slider: NSSlider?
    var bodyView: NSTextView?
    private var bodyScroll: NSScrollView?

    init(row: InspectorController.InspectorRow, tag: Int, target: InspectorController) {
        kind = row.kind
        let caption = NSTextField(labelWithString: row.kind == .button ? "" : row.label)
        caption.font = NSFont.systemFont(ofSize: 11)
        caption.textColor = .secondaryLabelColor
        caption.lineBreakMode = .byTruncatingTail
        caption.widthAnchor.constraint(equalToConstant: 68).isActive = true

        var ctl: NSView
        switch row.kind {
        case .info:
            let label = NSTextField(labelWithString: row.text)
            label.font = NSFont.systemFont(ofSize: 11)
            label.textColor = .secondaryLabelColor
            label.lineBreakMode = .byTruncatingMiddle
            label.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
            info = label
            text = nil
            popup = nil
            toggleButton = nil
            checkBoxes = []
            segmented = nil
            action = nil
            ctl = label
        case .text:
            let field = NSTextField(frame: .zero)
            field.font = NSFont.systemFont(ofSize: 11)
            field.controlSize = .small
            field.isEditable = true
            field.isBordered = true
            field.bezelStyle = .roundedBezel
            field.lineBreakMode = .byTruncatingTail
            field.usesSingleLineMode = true
            field.placeholderString = row.placeholder
            field.tag = tag
            field.target = target
            field.action = #selector(InspectorController.textCommitted(_:))
            field.delegate = target
            field.setContentHuggingPriority(.defaultLow, for: .horizontal)
            field.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
            field.widthAnchor.constraint(greaterThanOrEqualToConstant: 40).isActive = true
            var views: [NSView] = [field]
            var seg: NSSegmentedControl?
            if !row.segments.isEmpty {
                let control = NSSegmentedControl(
                    labels: row.segments, trackingMode: .selectOne, target: target,
                    action: #selector(InspectorController.sizeSegmentChanged(_:)))
                control.controlSize = .small
                control.tag = tag
                control.setContentHuggingPriority(.required, for: .horizontal)
                control.setContentCompressionResistancePriority(.required, for: .horizontal)
                seg = control
                views.append(control)
            }
            var more: NSButton?
            if !row.action.isEmpty {
                let button = NSButton(
                    title: row.action, target: target,
                    action: #selector(InspectorController.shadowButtonPressed(_:)))
                button.controlSize = .small
                button.bezelStyle = .rounded
                button.tag = tag
                button.widthAnchor.constraint(equalToConstant: 24).isActive = true
                more = button
                views.append(button)
            }
            info = nil
            text = field
            popup = nil
            toggleButton = nil
            checkBoxes = []
            segmented = seg
            action = more
            let group = NSStackView(views: views)
            group.orientation = .horizontal
            group.alignment = .centerY
            group.spacing = 4
            ctl = group
        case .popup:
            let button = NSPopUpButton(frame: .zero, pullsDown: false)
            button.font = NSFont.systemFont(ofSize: 11)
            button.controlSize = .small
            button.addItems(withTitles: row.choices)
            button.tag = tag
            button.target = target
            button.action = #selector(InspectorController.popupChanged(_:))
            button.setContentHuggingPriority(.defaultLow, for: .horizontal)
            info = nil
            text = nil
            popup = button
            toggleButton = nil
            checkBoxes = []
            segmented = nil
            action = nil
            ctl = button
        case .toggle:
            let button = NSButton(checkboxWithTitle: "", target: target, action: #selector(InspectorController.toggleChanged(_:)))
            button.controlSize = .small
            button.tag = tag
            info = nil
            text = nil
            popup = nil
            toggleButton = button
            checkBoxes = []
            segmented = nil
            action = nil
            ctl = button
        case .slider:
            let control = NSSlider(
                value: 0, minValue: 0, maxValue: 100, target: target,
                action: #selector(InspectorController.animationFrameChanged(_:)))
            control.controlSize = .small
            control.isContinuous = true
            control.tag = tag
            control.setContentHuggingPriority(.defaultLow, for: .horizontal)
            control.widthAnchor.constraint(greaterThanOrEqualToConstant: 40).isActive = true
            info = nil
            text = nil
            popup = nil
            toggleButton = nil
            checkBoxes = []
            segmented = nil
            action = nil
            slider = control
            ctl = control
        case .body:
            let editor = NSTextView(frame: NSRect(x: 0, y: 0, width: 240, height: 96))
            editor.font = NSFont.systemFont(ofSize: 11)
            editor.isRichText = false
            editor.isAutomaticQuoteSubstitutionEnabled = false
            editor.isAutomaticDashSubstitutionEnabled = false
            editor.isAutomaticTextReplacementEnabled = false
            editor.isVerticallyResizable = true
            editor.isHorizontallyResizable = false
            editor.autoresizingMask = [.width]
            editor.minSize = NSSize(width: 0, height: 96)
            editor.maxSize = NSSize(width: CGFloat.greatestFiniteMagnitude, height: CGFloat.greatestFiniteMagnitude)
            editor.textContainer?.widthTracksTextView = true
            editor.textContainer?.containerSize = NSSize(width: 240, height: CGFloat.greatestFiniteMagnitude)
            editor.delegate = target
            let scroll = NSScrollView()
            scroll.hasVerticalScroller = true
            scroll.autohidesScrollers = true
            scroll.borderType = .bezelBorder
            scroll.documentView = editor
            scroll.heightAnchor.constraint(equalToConstant: 96).isActive = true
            info = nil
            text = nil
            popup = nil
            toggleButton = nil
            checkBoxes = []
            segmented = nil
            action = nil
            bodyView = editor
            bodyScroll = scroll
            ctl = scroll
        case .checks:
            let group = NSStackView()
            group.orientation = .vertical
            group.alignment = .leading
            group.spacing = 2
            var boxes: [NSButton] = []
            for choice in row.choices {
                let button = NSButton(
                    checkboxWithTitle: choice, target: target, action: #selector(InspectorController.checkChanged(_:)))
                button.font = NSFont.systemFont(ofSize: 11)
                button.controlSize = .small
                button.tag = tag
                button.identifier = NSUserInterfaceItemIdentifier(choice)
                group.addArrangedSubview(button)
                boxes.append(button)
            }
            info = nil
            text = nil
            popup = nil
            toggleButton = nil
            checkBoxes = boxes
            segmented = nil
            action = nil
            ctl = group
        case .button:
            let button = NSButton(
                title: row.label, target: target,
                action: #selector(InspectorController.shadowButtonPressed(_:)))
            button.controlSize = .small
            button.bezelStyle = .rounded
            button.tag = tag
            info = nil
            text = nil
            popup = nil
            toggleButton = nil
            checkBoxes = []
            segmented = nil
            action = nil
            ctl = button
        }

        if let scroll = bodyScroll {
            // The body field takes the whole row: a caption on the left would leave it too narrow.
            let holder = NSView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            scroll.translatesAutoresizingMaskIntoConstraints = false
            holder.addSubview(scroll)
            NSLayoutConstraint.activate([
                scroll.leadingAnchor.constraint(equalTo: holder.leadingAnchor),
                scroll.trailingAnchor.constraint(equalTo: holder.trailingAnchor),
                scroll.topAnchor.constraint(equalTo: holder.topAnchor),
                scroll.bottomAnchor.constraint(equalTo: holder.bottomAnchor),
            ])
            view = holder
        } else {
            let line = NSStackView(views: [caption, ctl])
            line.orientation = .horizontal
            line.alignment = .centerY
            line.spacing = 8
            line.translatesAutoresizingMaskIntoConstraints = false
            view = line
        }
        apply(row)
    }

    func apply(_ row: InspectorController.InspectorRow) {
        switch kind {
        case .info:
            info?.stringValue = row.text
            info?.toolTip = row.text
        case .text:
            text?.placeholderString = row.placeholder
            if text?.currentEditor() == nil {
                text?.stringValue = row.text
            }
            segmented?.selectedSegment = row.chosen ?? -1
        case .button:
            break
        case .popup:
            if let popup, popup.titleOfSelectedItem != row.text {
                if let i = row.choices.firstIndex(of: row.text) {
                    popup.selectItem(at: i)
                } else {
                    popup.select(nil)
                }
            }
        case .toggle:
            toggleButton?.state = row.text == "yes" ? .on : .off
        case .checks:
            let checked = Set(row.text.split(whereSeparator: \.isWhitespace).map(String.init))
            for button in checkBoxes {
                button.state = checked.contains(button.identifier?.rawValue ?? "") ? .on : .off
            }
        case .slider:
            break  // the thumb stays where the scrub put it
        case .body:
            // A refresh must not replace what the user is typing.
            if let bodyView, bodyView.window?.firstResponder !== bodyView, bodyView.string != row.text {
                bodyView.string = row.text
            }
        }
    }

    /// Takes the row out of the responder chain before its view is dropped with a rebuilt column.
    func detach() {
        text?.target = nil
        text?.delegate = nil
        popup?.target = nil
        toggleButton?.target = nil
        segmented?.target = nil
        action?.target = nil
        slider?.target = nil
        bodyView?.delegate = nil
        for button in checkBoxes {
            button.target = nil
        }
    }
}
