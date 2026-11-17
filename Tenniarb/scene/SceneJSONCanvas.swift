//
//  SceneJSONCanvas.swift
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

import Foundation

/// go/jsoncanvas: reads and writes JSON Canvas 1.0 and converts it to and from the .tenn scene text.
enum JSONCanvas {
    /// One .canvas file: a flat list of nodes and the edges between them.
    struct Canvas: Codable, Equatable {
        var nodes: [Node]
        var edges: [Edge]

        init(nodes: [Node] = [], edges: [Edge] = []) {
            self.nodes = nodes
            self.edges = edges
        }

        private enum CodingKeys: String, CodingKey {
            case nodes, edges
        }

        init(from decoder: Decoder) throws {
            let c = try decoder.container(keyedBy: CodingKeys.self)
            nodes = try c.decodeIfPresent([Node].self, forKey: .nodes) ?? []
            edges = try c.decodeIfPresent([Edge].self, forKey: .edges) ?? []
        }

        func encode(to encoder: Encoder) throws {
            var c = encoder.container(keyedBy: CodingKeys.self)
            try c.encode(nodes, forKey: .nodes)
            try c.encode(edges, forKey: .edges)
        }
    }

    /// A canvas node; the fields of one type are empty for the others.
    struct Node: Codable, Equatable {
        var id = ""
        var type = ""
        var x = 0.0
        var y = 0.0
        var width = 0.0
        var height = 0.0
        var color = ""  // "1".."6"
        var text = ""
        var file = ""
        var subpath = ""
        var url = ""
        var label = ""
        var background = ""
        var backgroundStyle = ""

        init() {}

        init(
            id: String, type: String, x: Double, y: Double, width: Double, height: Double,
            color: String = "", text: String = "", file: String = "", subpath: String = "", url: String = "",
            label: String = "", background: String = "", backgroundStyle: String = ""
        ) {
            self.id = id
            self.type = type
            self.x = x
            self.y = y
            self.width = width
            self.height = height
            self.color = color
            self.text = text
            self.file = file
            self.subpath = subpath
            self.url = url
            self.label = label
            self.background = background
            self.backgroundStyle = backgroundStyle
        }

        private enum CodingKeys: String, CodingKey {
            case id, type, x, y, width, height, color, text, file, subpath, url, label, background,
                backgroundStyle
        }

        init(from decoder: Decoder) throws {
            let c = try decoder.container(keyedBy: CodingKeys.self)
            id = try c.decodeIfPresent(String.self, forKey: .id) ?? ""
            type = try c.decodeIfPresent(String.self, forKey: .type) ?? ""
            x = try c.decodeIfPresent(Double.self, forKey: .x) ?? 0
            y = try c.decodeIfPresent(Double.self, forKey: .y) ?? 0
            width = try c.decodeIfPresent(Double.self, forKey: .width) ?? 0
            height = try c.decodeIfPresent(Double.self, forKey: .height) ?? 0
            color = try c.decodeIfPresent(String.self, forKey: .color) ?? ""
            text = try c.decodeIfPresent(String.self, forKey: .text) ?? ""
            file = try c.decodeIfPresent(String.self, forKey: .file) ?? ""
            subpath = try c.decodeIfPresent(String.self, forKey: .subpath) ?? ""
            url = try c.decodeIfPresent(String.self, forKey: .url) ?? ""
            label = try c.decodeIfPresent(String.self, forKey: .label) ?? ""
            background = try c.decodeIfPresent(String.self, forKey: .background) ?? ""
            backgroundStyle = try c.decodeIfPresent(String.self, forKey: .backgroundStyle) ?? ""
        }

        func encode(to encoder: Encoder) throws {
            var c = encoder.container(keyedBy: CodingKeys.self)
            try c.encode(id, forKey: .id)
            try c.encode(type, forKey: .type)
            try c.encode(x, forKey: .x)
            try c.encode(y, forKey: .y)
            try c.encode(width, forKey: .width)
            try c.encode(height, forKey: .height)
            if !color.isEmpty { try c.encode(color, forKey: .color) }
            if !text.isEmpty { try c.encode(text, forKey: .text) }
            if !file.isEmpty { try c.encode(file, forKey: .file) }
            if !subpath.isEmpty { try c.encode(subpath, forKey: .subpath) }
            if !url.isEmpty { try c.encode(url, forKey: .url) }
            if !label.isEmpty { try c.encode(label, forKey: .label) }
            if !background.isEmpty { try c.encode(background, forKey: .background) }
            if !backgroundStyle.isEmpty { try c.encode(backgroundStyle, forKey: .backgroundStyle) }
        }
    }

    /// An edge between two nodes; sides and ends are optional in the format.
    struct Edge: Codable, Equatable {
        var id = ""
        var fromNode = ""
        var fromSide = ""
        var fromEnd = ""
        var toNode = ""
        var toSide = ""
        var toEnd = ""
        var color = ""
        var label = ""

        init() {}

        init(
            id: String, fromNode: String, fromSide: String = "", fromEnd: String = "", toNode: String,
            toSide: String = "", toEnd: String = "", color: String = "", label: String = ""
        ) {
            self.id = id
            self.fromNode = fromNode
            self.fromSide = fromSide
            self.fromEnd = fromEnd
            self.toNode = toNode
            self.toSide = toSide
            self.toEnd = toEnd
            self.color = color
            self.label = label
        }

        private enum CodingKeys: String, CodingKey {
            case id, fromNode, fromSide, fromEnd, toNode, toSide, toEnd, color, label
        }

        init(from decoder: Decoder) throws {
            let c = try decoder.container(keyedBy: CodingKeys.self)
            id = try c.decodeIfPresent(String.self, forKey: .id) ?? ""
            fromNode = try c.decodeIfPresent(String.self, forKey: .fromNode) ?? ""
            fromSide = try c.decodeIfPresent(String.self, forKey: .fromSide) ?? ""
            fromEnd = try c.decodeIfPresent(String.self, forKey: .fromEnd) ?? ""
            toNode = try c.decodeIfPresent(String.self, forKey: .toNode) ?? ""
            toSide = try c.decodeIfPresent(String.self, forKey: .toSide) ?? ""
            toEnd = try c.decodeIfPresent(String.self, forKey: .toEnd) ?? ""
            color = try c.decodeIfPresent(String.self, forKey: .color) ?? ""
            label = try c.decodeIfPresent(String.self, forKey: .label) ?? ""
        }

        func encode(to encoder: Encoder) throws {
            var c = encoder.container(keyedBy: CodingKeys.self)
            try c.encode(id, forKey: .id)
            try c.encode(fromNode, forKey: .fromNode)
            if !fromSide.isEmpty { try c.encode(fromSide, forKey: .fromSide) }
            if !fromEnd.isEmpty { try c.encode(fromEnd, forKey: .fromEnd) }
            try c.encode(toNode, forKey: .toNode)
            if !toSide.isEmpty { try c.encode(toSide, forKey: .toSide) }
            if !toEnd.isEmpty { try c.encode(toEnd, forKey: .toEnd) }
            if !color.isEmpty { try c.encode(color, forKey: .color) }
            if !label.isEmpty { try c.encode(label, forKey: .label) }
        }
    }

    struct CanvasError: Swift.Error, LocalizedError, CustomStringConvertible {
        let message: String

        var errorDescription: String? { message }
        var description: String { message }
    }

    /// Whether path is a JSON Canvas file, as app.IsCanvas.
    static func isCanvas(_ path: String) -> Bool {
        (path as NSString).pathExtension.lowercased() == "canvas"
    }

    // MARK: - Reading and writing

    /// Reads a .canvas file. Unknown fields are ignored; a broken structure names the node or edge
    /// it is in.
    static func parse(_ data: Data) throws -> Canvas {
        let canvas: Canvas
        do {
            canvas = try JSONDecoder().decode(Canvas.self, from: data)
        } catch {
            throw CanvasError(message: "json canvas: \(error)")
        }
        var ids = Set<String>()
        for (i, n) in canvas.nodes.enumerated() {
            if n.id.isEmpty {
                throw CanvasError(message: "json canvas: node \(i + 1) has no id")
            }
            if ids.contains(n.id) {
                throw CanvasError(message: "json canvas: node \(quote(n.id)) is repeated")
            }
            ids.insert(n.id)
            switch n.type {
            case "text", "group":
                break
            case "file":
                if n.file.isEmpty {
                    throw CanvasError(message: "json canvas: file node \(quote(n.id)) has no file")
                }
            case "link":
                if n.url.isEmpty {
                    throw CanvasError(message: "json canvas: link node \(quote(n.id)) has no url")
                }
            default:
                throw CanvasError(message: "json canvas: node \(quote(n.id)) has unknown type \(quote(n.type))")
            }
        }
        var edges = Set<String>()
        for (i, e) in canvas.edges.enumerated() {
            if e.id.isEmpty {
                throw CanvasError(message: "json canvas: edge \(i + 1) has no id")
            }
            if edges.contains(e.id) {
                throw CanvasError(message: "json canvas: edge \(quote(e.id)) is repeated")
            }
            edges.insert(e.id)
            if !ids.contains(e.fromNode) {
                throw CanvasError(message: "json canvas: edge \(quote(e.id)): fromNode \(quote(e.fromNode)) is not a node")
            }
            if !ids.contains(e.toNode) {
                throw CanvasError(message: "json canvas: edge \(quote(e.id)): toNode \(quote(e.toNode)) is not a node")
            }
        }
        return canvas
    }

    /// Writes a .canvas file: two spaces of indentation, empty arrays for missing lists.
    static func marshal(_ canvas: Canvas) throws -> Data {
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys, .withoutEscapingSlashes]
        var data: Data
        do {
            data = try encoder.encode(canvas)
        } catch {
            throw CanvasError(message: "json canvas: \(error)")
        }
        data.append(0x0A)
        return data
    }

    /// Reads a .canvas file as the text of a .tenn document named after the file; notes are what the
    /// conversion left out.
    static func canvasText(path: String, data: Data) throws -> (text: String, notes: [String]) {
        let canvas = try parse(data)
        let base = (path as NSString).lastPathComponent
        let name = (base as NSString).deletingPathExtension
        return toTenn(canvas, name: name)
    }

    /// Writes a scene tree as JSON Canvas: the tree is the source, not the text.
    static func canvasData(tree: Scene.Tree?) throws -> Data {
        guard let tree else {
            throw CanvasError(message: "the document has no scene to write as JSON Canvas")
        }
        let converted = try fromTree(tree)
        return try marshal(converted.canvas)
    }

    /// The warning before an explicit save to .canvas: what fromTree had to group or drop. The message is
    /// empty when the document converts without loss.
    static func canvasNotes(tree: Scene.Tree?) -> (message: String, warning: Bool) {
        guard let tree else { return ("", false) }
        do {
            let converted = try fromTree(tree)
            return (converted.notes.joined(separator: "\n"), !converted.notes.isEmpty)
        } catch {
            return ("\(error)", true)
        }
    }
}

// MARK: - Import: canvas to .tenn

extension JSONCanvas {
    /// Converts a canvas to the text of a .tenn file: one element named name holds an item per node,
    /// groups become containers and edges become links.
    static func toTenn(_ canvas: Canvas, name: String) -> (text: String, notes: [String]) {
        let report = Report()
        let trimmed = name.trimmingCharacters(in: .whitespacesAndNewlines)
        let title = trimmed.isEmpty ? "Canvas" : name
        let converter = ImportConverter(canvas: canvas, report: report)
        converter.build()
        var body = converter.roots.map { converter.item($0) }
        body += converter.rootLinks
        let element = Cmd(name: "element", args: [.string(title)], block: body)
        return (CompactPrinter().render(element) + "\n", report.list)
    }

    /// A canvas node in the group tree the import builds by containment.
    private final class InNode {
        let node: Node
        var name = ""
        weak var parent: InNode?
        var kids: [InNode] = []
        var links: [Cmd] = []

        init(_ node: Node) {
            self.node = node
        }

        var parentX: Double { parent?.node.x ?? 0 }
        var parentY: Double { parent?.node.y ?? 0 }
    }

    private final class ImportConverter {
        let canvas: Canvas
        let report: Report
        var byID: [String: InNode] = [:]
        var roots: [InNode] = []
        var rootLinks: [Cmd] = []

        init(canvas: Canvas, report: Report) {
            self.canvas = canvas
            self.report = report
        }

        /// Makes the containment tree: the smallest group that contains a node holds it. A group inside a
        /// group must be strictly larger, so equal boxes cannot form a cycle.
        func build() {
            var all: [InNode] = []
            var groups: [InNode] = []
            for n in canvas.nodes {
                let node = InNode(n)
                byID[n.id] = node
                all.append(node)
                if n.type == "group" {
                    groups.append(node)
                }
            }
            for n in all {
                var best: InNode?
                for g in groups where g !== n {
                    guard contains(g.node, n.node) else { continue }
                    if n.node.type == "group" && area(g.node) <= area(n.node) {
                        continue
                    }
                    if best == nil || area(g.node) < area(best!.node) || (area(g.node) == area(best!.node) && g.node.id < best!.node.id) {
                        best = g
                    }
                }
                n.parent = best
            }
            for n in all {
                if let parent = n.parent {
                    parent.kids.append(n)
                } else {
                    roots.append(n)
                }
            }
            name(roots)
            for e in canvas.edges {
                edge(e)
            }
        }

        private func contains(_ group: Node, _ n: Node) -> Bool {
            let eps = 1e-6
            return n.x >= group.x - eps && n.y >= group.y - eps
                && n.x + n.width <= group.x + group.width + eps
                && n.y + n.height <= group.y + group.height + eps
        }

        private func area(_ n: Node) -> Double { n.width * n.height }

        /// Gives every sibling set unique item names; edges refer to the ends by these names.
        private func name(_ siblings: [InNode]) {
            var used: [String: Int] = [:]
            for n in siblings {
                let base = baseName(n.node)
                let k = used[base] ?? 0
                n.name = k > 0 ? "\(base) \(k + 1)" : base
                used[base] = k + 1
                name(n.kids)
            }
        }

        private func baseName(_ n: Node) -> String {
            var base = ""
            switch n.type {
            case "group": base = n.label
            case "text": base = splitText(n.text).title
            case "file": if !external(n.file) { base = pathBase(n.file) }
            case "link": base = n.url
            default: break
            }
            base = base.trimmingCharacters(in: .whitespacesAndNewlines)
            return base.isEmpty ? n.type : base
        }

        private func splitText(_ text: String) -> (title: String, body: String) {
            let text = text.replacingOccurrences(of: "\r\n", with: "\n")
            guard let i = text.firstIndex(of: "\n") else { return (text, "") }
            return (String(text[..<i]), String(text[text.index(after: i)...]))
        }

        private func pathBase(_ p: String) -> String {
            if p == "/" { return "/" }
            let parts = p.split(separator: "/", omittingEmptySubsequences: true)
            return parts.last.map(String.init) ?? "."
        }

        func item(_ n: InNode) -> Cmd {
            let node = n.node
            var body: [Cmd] = [
                cmd("pos", num(node.x - n.parentX), num(-(node.y - n.parentY))),
                cmd("width", num(node.width)),
                cmd("height", num(node.height)),
            ]
            switch node.type {
            case "text":
                let (title, rest) = splitText(node.text)
                body.append(cmd("title", .string(title)))
                if !rest.isEmpty {
                    body.append(cmd("body", .string(rest)))
                }
            case "file":
                if external(node.file) {
                    body.append(cmd("title", .string(node.file)))
                    body.append(cmd("body", .string(linkText(node.file))))
                    report.add(
                        "file node \(quote(node.id)): \(quote(node.file)) is outside the workspace and became a link item")
                } else {
                    body.append(cmd("image", .string(node.file)))
                    if !node.subpath.isEmpty {
                        report.add(
                            "file node \(quote(node.id)): subpath \(quote(node.subpath)) is lost, an image has no anchor")
                    }
                }
            case "link":
                body.append(cmd("title", .string(node.url)))
                body.append(cmd("body", .string(linkText(node.url))))
            case "group":
                if !node.label.isEmpty {
                    body.append(cmd("title", .string(node.label)))
                }
                if !node.background.isEmpty || !node.backgroundStyle.isEmpty {
                    report.add(
                        "group \(quote(node.id)): background \(quote(node.background)) is lost, .tenn groups have no background image"
                    )
                }
            default:
                break
            }
            if let c = color(node) {
                body.append(c)
            } else if node.type == "group" {
                // A white default box would cover the canvas; a frame keeps the children visible.
                body.append(cmd("color", .string("none")))
                body.append(cmd("border-color", .string("grey-400")))
            }
            for k in n.kids {
                body.append(item(k))
            }
            body += n.links
            return Cmd(name: "item", args: [.string(n.name)], block: body)
        }

        private func color(_ n: Node) -> Cmd? {
            guard !n.color.isEmpty else { return nil }
            guard let name = presetName(n.color) else {
                report.add("node \(quote(n.id)): colour \(quote(n.color)) is not 1..6, the fill is left to the default")
                return nil
            }
            if !sameColor(name, n.color) {
                report.add("colour \(quote(n.color)) has no exact .tenn shade, using \(name)")
            }
            return cmd("color", .string(name))
        }

        private func edge(_ e: Edge) {
            guard let from = byID[e.fromNode] else {
                report.add("edge \(quote(e.id)): fromNode \(quote(e.fromNode)) is not a node")
                return
            }
            guard let to = byID[e.toNode] else {
                report.add("edge \(quote(e.id)): toNode \(quote(e.toNode)) is not a node")
                return
            }
            guard from.parent === to.parent else {
                report.add(
                    "edge \(quote(e.id)): .tenn links join siblings only, \(quote(from.name)) and \(quote(to.name)) are in different groups"
                )
                return
            }
            var body: [Cmd] = []
            if !e.label.isEmpty {
                body.append(cmd("label", .string(e.label)))
            }
            if e.fromEnd == "arrow" {
                report.add("edge \(quote(e.id)): an arrow at the source has no .tenn form, only the arrow at the target is kept")
            }
            switch e.toEnd {
            case "", "none": break
            case "arrow": body.append(cmd("display", .string("arrow")))
            default: report.add("edge \(quote(e.id)): end \(quote(e.toEnd)) is unknown, the line has no arrow")
            }
            if !e.color.isEmpty {
                if let name = presetName(e.color) {
                    if !sameColor(name, e.color) {
                        report.add("colour \(quote(e.color)) has no exact .tenn shade, using \(name)")
                    }
                    body.append(cmd("color", .string(name)))
                } else {
                    report.add("edge \(quote(e.id)): colour \(quote(e.color)) is not 1..6, the line keeps the default")
                }
            }
            switch (e.fromSide.isEmpty, e.toSide.isEmpty) {
            case (false, false):
                body.append(cmd("route", .string("elbow"), .string(e.fromSide), .string(e.toSide)))
            case (false, true):
                body.append(cmd("route", .string("elbow"), .string(e.fromSide)))
            case (true, false):
                report.add(
                    "edge \(quote(e.id)): only the target side is given, .tenn writes the source side first; the line is straight")
            case (true, true): break
            }
            let link = Cmd(name: "link", args: [.string(from.name), .string(to.name)], block: body.isEmpty ? nil : body)
            if from.parent == nil {
                rootLinks.append(link)
            } else {
                from.parent?.links.append(link)
            }
        }
    }

    /// Whether a file node points outside the workspace.
    private static func external(_ s: String) -> Bool { s.contains("://") }

    private static func linkText(_ url: String) -> String {
        let label = url.replacingOccurrences(of: "[", with: "\\[").replacingOccurrences(of: "]", with: "\\]")
        return "[" + label + "](" + url + ")"
    }
}

// MARK: - Export: .tenn to canvas

extension JSONCanvas {
    /// Converts a loaded .tenn file: the items of the file root, then every top level element as a group.
    /// A file of one element and no root content is that element's content, as fromElement.
    static func fromTree(_ tree: Scene.Tree?) throws -> (canvas: Canvas, notes: [String]) {
        guard let tree else {
            throw CanvasError(message: "json canvas: nil tree")
        }
        guard let main = tree.main else {
            throw CanvasError(message: "json canvas: the tree has no files")
        }
        if main.elems.count == 1 && main.content.isEmpty {
            return try fromElement(tree, main.elems[0])
        }
        let exporter = Exporter(tree)
        let screen = try exporter.content(main, 0, 0)
        let kids = screen.placed().kids
        var bottom = 0.0
        if !kids.isEmpty {
            let b = bounds(kids)
            bottom = b.y + b.h
        }
        if !main.elems.isEmpty {
            exporter.report.add("elements are not part of JSON Canvas; each top level element becomes a group")
        }
        var offset = bottom + 80
        for el in main.elems {
            let screen: Scene.Screen
            do {
                screen = try exporter.build(el)
            } catch {
                exporter.report.add("element \(quote(el.name)) is skipped: \(error)")
                continue
            }
            var b = bounds(screen.placed().kids)
            if b.w == 0 && b.h == 0 {
                b = Rect(x: 0, y: 0, w: screen.w, h: screen.h)
            }
            let dy = offset - b.y
            exporter.addGroup(el.name, b.x, offset, b.w, b.h)
            for pl in screen.placed().kids {
                exporter.walk(pl, 0, dy)
            }
            exporter.links(el)
            exporter.elementLoss(el)
            offset += b.h + 80
        }
        return (exporter.canvas, exporter.report.list)
    }

    /// Converts the content of one element, without the group around it: a node of the canvas can stand
    /// for a group, and an extra group would hide the element's own items.
    static func fromElement(_ tree: Scene.Tree?, _ el: Scene.Node?) throws -> (canvas: Canvas, notes: [String]) {
        guard let tree else {
            throw CanvasError(message: "json canvas: nil tree")
        }
        guard let el else {
            throw CanvasError(message: "json canvas: nil element")
        }
        let exporter = Exporter(tree)
        _ = try exporter.content(el, 0, 0)
        exporter.elementLoss(el)
        return (exporter.canvas, exporter.report.list)
    }

    private struct Rect {
        var x: Double
        var y: Double
        var w: Double
        var h: Double
    }

    /// Go bounds keeps the first item's size as the starting extent, then takes the minimum corner and the
    /// maximum edge; it is not max-min when the first item is wider than the rest.
    private static func bounds(_ items: [Scene.Placed]) -> Rect {
        guard let first = items.first else { return Rect(x: 0, y: 0, w: 0, h: 0) }
        var b = Rect(x: first.x, y: first.y, w: first.w, h: first.h)
        func walk(_ ps: [Scene.Placed]) {
            for p in ps {
                b.x = min(b.x, p.x)
                b.y = min(b.y, p.y)
                b.w = max(b.w, p.x + p.w)
                b.h = max(b.h, p.y + p.h)
                walk(p.kids)
            }
        }
        walk(items)
        b.w -= b.x
        b.h -= b.y
        return b
    }

    private final class Exporter {
        let tree: Scene.Tree
        let report = Report()
        var canvas = Canvas()
        var ids: [ObjectIdentifier: String] = [:]
        var nodes = 0
        var edges = 0

        init(_ tree: Scene.Tree) {
            self.tree = tree
        }

        func build(_ el: Scene.Node) throws -> Scene.Screen {
            let before = tree.diagnostics.count
            // go/jsoncanvas builds with scene.Options{}: gfx.EstimateText, the metrics of the stand, not
            // the CoreText the window draws with.
            if let screen = tree.buildElement(el, measurer: Scene.TextMeasurer(estimated: true)) {
                return screen
            }
            let added = Array(tree.diagnostics.dropFirst(before))
            let message = added.map { d in
                "\(d.pos): \(d.warn ? "warning: " : "")\(d.message)"
            }.joined(separator: "\n")
            throw CanvasError(message: message.isEmpty ? "the element does not build" : message)
        }

        /// Adds the placed items of an element and its links.
        func content(_ el: Scene.Node, _ x: Double, _ y: Double) throws -> Scene.Screen {
            let screen = try build(el)
            for pl in screen.placed().kids {
                walk(pl, x, y)
            }
            links(el)
            return screen
        }

        func addGroup(_ label: String, _ x: Double, _ y: Double, _ w: Double, _ h: Double) -> String {
            let id = nextID()
            canvas.nodes.append(Node(id: id, type: "group", x: x, y: y, width: w, height: h, label: label))
            return id
        }

        private func nextID() -> String {
            nodes += 1
            return "n\(nodes)"
        }

        /// Adds one placed item and its children. x and y shift the coordinates of an element that is
        /// placed below the previous one.
        @discardableResult
        func walk(_ pl: Scene.Placed, _ x: Double, _ y: Double) -> String {
            guard let src = pl.src else { return "" }
            if src.kind == .use {
                report.add("use \(quote(src.name)) is not exported: JSON Canvas has no components")
                return ""
            }
            loss(pl)
            let id = nextID()
            var node = Node(id: id, type: "", x: x + pl.x, y: y + pl.y, width: pl.w, height: pl.h)
            node.color = color(pl.prop("color"))
            ids[ObjectIdentifier(src)] = id
            if !pl.kids.isEmpty {
                node.type = "group"
                node.label = propText(pl, "title")
                if node.label.isEmpty {
                    node.label = src.name
                }
                canvas.nodes.append(node)
                for k in pl.kids {
                    walk(k, x, y)
                }
            } else if fileImage(pl.prop("image")) {
                node.type = "file"
                node.file = propText(pl, "image")
                if let pr = pl.prop("image"), inline(pr) {
                    report.add("item \(quote(src.name)): a base64 image is not written into the canvas, only its name \(quote(node.file))")
                }
                canvas.nodes.append(node)
            } else {
                var title = propText(pl, "title")
                let body = propText(pl, "body")
                if title.isEmpty && body.isEmpty {
                    title = src.name
                }
                if let url = singleLink(body), title.isEmpty || title == url {
                    node.type = "link"
                    node.url = url
                } else {
                    node.type = "text"
                    node.text = title
                    if !body.isEmpty && !title.isEmpty {
                        node.text += "\n"
                    }
                    node.text += body
                }
                canvas.nodes.append(node)
            }
            return id
        }

        func links(_ parent: Scene.Node) {
            for c in parent.content {
                switch c.kind {
                case .link:
                    edge(c)
                case .item:
                    if ids[ObjectIdentifier(c)] != nil {
                        links(c)
                    }
                default:
                    break
                }
            }
        }

        private func edge(_ lk: Scene.Node) {
            let from = end(lk, lk.from, "source-index")
            let to = end(lk, lk.to, "target-index")
            if from.isEmpty || to.isEmpty {
                report.add("link \(quote(lk.from)) \(quote(lk.to)) is not exported: an end is a use or is not drawn")
                return
            }
            edges += 1
            var edge = Edge(id: "e\(edges)", fromNode: from, toNode: to)
            if let pr = findProp(lk, "label") {
                edge.label = rawText(pr)
            }
            if let pr = findProp(lk, "display"), rawText(pr) == "arrow" {
                edge.toEnd = "arrow"
            }
            if let pr = findProp(lk, "color") {
                edge.color = color(pr)
            }
            if let pr = findProp(lk, "route") {
                let vals = rawValues(pr)
                if vals.first == "elbow" {
                    if vals.count > 1 {
                        edge.fromSide = vals[1]
                    }
                    if vals.count > 2 {
                        edge.toSide = vals[2]
                    }
                    if !side(edge.fromSide) || !side(edge.toSide) {
                        report.add(
                            "link \(quote(lk.from)) \(quote(lk.to)): side \(quote(edge.fromSide)) \(quote(edge.toSide)) is not a JSON Canvas side"
                        )
                        edge.fromSide = ""
                        edge.toSide = ""
                    }
                }
            }
            canvas.edges.append(edge)
        }

        /// The canvas id of the n-th sibling item with the name of a link end.
        private func end(_ lk: Scene.Node, _ name: String, _ indexProp: String) -> String {
            var idx = 0
            if let pr = findProp(lk, indexProp), let v = pr.num(0) {
                idx = Int(v)
            }
            var n = 0
            for c in lk.parent?.content ?? [] where c.kind == .item && c.name == name {
                if n == idx {
                    return ids[ObjectIdentifier(c)] ?? ""
                }
                n += 1
            }
            return ""
        }

        private func color(_ pr: Scene.Prop?) -> String {
            guard let pr else { return "" }
            let word = rawText(pr).trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
            switch word {
            case "", "none", "transparent": return ""
            default: break
            }
            if word == "linear" {
                report.add("a gradient is not a JSON Canvas colour, the fill is left out")
                return ""
            }
            guard let key = presetKey(word) else {
                report.add("colour \(quote(word)) has no JSON Canvas form, the fill is left out")
                return ""
            }
            if let name = presetName(key), name != word {
                report.add("colour \(quote(word)) is approximated by JSON Canvas colour \(quote(key))")
            }
            return key
        }

        /// Reports the properties JSON Canvas has no field for.
        private func loss(_ pl: Scene.Placed) {
            guard let src = pl.src else { return }
            for name in ["auto-layout", "use-style", "layer", "rotate", "hidden", "animation", "opacity"] where pl.prop(name) != nil {
                report.add("item \(quote(src.name)): \(name) is lost in JSON Canvas")
            }
            if let pr = pl.prop("display") {
                let word = rawText(pr)
                if !word.isEmpty && word != "rect" {
                    report.add("item \(quote(src.name)): display \(word) is lost in JSON Canvas")
                }
            }
            for pr in pl.props() where hasExprRaw(pr) {
                report.add("item \(quote(src.name)): expressions are not evaluated, the canvas holds the written value")
                break
            }
        }

        func elementLoss(_ el: Scene.Node) {
            if let mount = el.mount {
                report.add("element \(quote(el.name)): the mount of \(quote(mount.pos.file)) is flattened into a group")
            }
            if let styles = el.styles, !styles.isEmpty {
                report.add("element \(quote(el.name)): styles are lost in JSON Canvas")
            }
            if !el.params.isEmpty {
                report.add("element \(quote(el.name)): parameters are lost in JSON Canvas")
            }
            if el.props.contains(where: { $0.name == "template" }) {
                report.add("element \(quote(el.name)): the template is lost in JSON Canvas")
            }
            if !el.elems.isEmpty {
                report.add("element \(quote(el.name)): nested elements are not exported")
            }
            for pr in el.props where hasExprRaw(pr) {
                report.add("element \(quote(el.name)): expressions are not evaluated, the canvas holds the written value")
                break
            }
        }
    }

    private static func propText(_ pl: Scene.Placed, _ name: String) -> String {
        guard let pr = pl.prop(name) else { return "" }
        return rawText(pr)
    }

    /// The first argument of a property as text, as Go Prop.Values reads it: the written value, not the
    /// evaluated one.
    private static func rawText(_ pr: Scene.Prop) -> String {
        pr.args.first?.getIdentText() ?? ""
    }

    private static func rawValues(_ pr: Scene.Prop) -> [String] {
        pr.args.map { $0.getIdentText() ?? "" }
    }

    private static func fileImage(_ pr: Scene.Prop?) -> Bool {
        guard let pr else { return false }
        return !rawText(pr).isEmpty && !inline(pr)
    }

    private static func inline(_ pr: Scene.Prop?) -> Bool {
        guard let pr else { return false }
        return pr.args.count >= 2 && pr.args[1].kind == .Image
    }

    private static func singleLink(_ body: String) -> String? {
        let text = body.trimmingCharacters(in: .whitespacesAndNewlines)
        guard text.hasPrefix("["), text.hasSuffix(")"), let close = text.firstIndex(of: "]") else { return nil }
        let after = text.index(after: close)
        guard after < text.endIndex, text[after] == "(" else { return nil }
        let inner = text[text.index(after: after)..<text.index(before: text.endIndex)]
        guard !inner.isEmpty, !inner.contains(")") else { return nil }
        if inner.unicodeScalars.contains(where: { " \t\n\u{0C}\r".unicodeScalars.contains($0) }) {
            return nil
        }
        return String(inner)
    }

    private static func side(_ s: String) -> Bool {
        s == "top" || s == "right" || s == "bottom" || s == "left"
    }

    private static func findProp(_ n: Scene.Node, _ name: String) -> Scene.Prop? {
        n.props.first { $0.name == name }
    }

    private static func hasExprRaw(_ pr: Scene.Prop) -> Bool {
        for a in pr.args {
            switch a.kind {
            case .Expression, .ExpressionBlock:
                return true
            case .StringLit:
                if let s = a.getIdentText(), s.contains("${") {
                    return true
                }
            default:
                break
            }
        }
        return false
    }
}

// MARK: - Palette

extension JSONCanvas {
    private struct Preset {
        let key: String
        let name: String
        let hex: String
    }

    private static let presets = [
        Preset(key: "1", name: "red-500", hex: "#fb464c"),
        Preset(key: "2", name: "orange-500", hex: "#e9973f"),
        Preset(key: "3", name: "yellow-500", hex: "#e0de71"),
        Preset(key: "4", name: "green-500", hex: "#44cf6e"),
        Preset(key: "5", name: "cyan-500", hex: "#53dfdd"),
        Preset(key: "6", name: "purple-500", hex: "#a882ff"),
    ]

    private static func presetName(_ key: String) -> String? {
        presets.first { $0.key == key }?.name
    }

    /// The canvas colour nearest to a .tenn colour name or hex value.
    private static func presetKey(_ color: String) -> String? {
        guard let rgb = parseRGB(color) else { return nil }
        var best = ""
        var bestDistance = -1.0
        for p in presets {
            guard let other = parseRGB(p.hex) else { continue }
            let d =
                (rgb.0 - other.0) * (rgb.0 - other.0) + (rgb.1 - other.1) * (rgb.1 - other.1)
                + (rgb.2 - other.2) * (rgb.2 - other.2)
            if bestDistance < 0 || d < bestDistance {
                best = p.key
                bestDistance = d
            }
        }
        return best.isEmpty ? nil : best
    }

    private static func sameColor(_ name: String, _ key: String) -> Bool {
        guard let p = presets.first(where: { $0.key == key }), let a = parseRGB(name), let b = parseRGB(p.hex) else {
            return false
        }
        return a.0 == b.0 && a.1 == b.1 && a.2 == b.2
    }

    private static func parseRGB(_ color: String) -> (Double, Double, Double)? {
        // go/jsoncanvas takes a named colour or hex only; a colour array of an expression is no colour here.
        guard !color.contains(","), let c = Scene.parseColor(color) else { return nil }
        return ((c.r * 255).rounded(), (c.g * 255).rounded(), (c.b * 255).rounded())
    }
}

// MARK: - Shared helpers

/// The conversion notes in the order Go reports them, each message once.
private final class Report {
    var list: [String] = []
    private var seen = Set<String>()

    func add(_ message: String) {
        guard seen.insert(message).inserted else { return }
        list.append(message)
    }
}

private func quote(_ s: String) -> String {
    var out = "\""
    for u in s.unicodeScalars {
        switch u {
        case "\\": out += "\\\\"
        case "\"": out += "\\\""
        case "\n": out += "\\n"
        case "\t": out += "\\t"
        case "\r": out += "\\r"
        default:
            if u.value < 0x20 {
                out += String(format: "\\u%04x", u.value)
            } else {
                out.unicodeScalars.append(u)
            }
        }
    }
    return out + "\""
}

private func round2(_ v: Double) -> Double { (v * 100).rounded() / 100 }

// MARK: - Compact printer

private enum Arg {
    case string(String)
    case number(Double)
}

private struct Cmd {
    var name: String
    var args: [Arg]
    var block: [Cmd]?
}

private func cmd(_ name: String, _ args: Arg...) -> Cmd {
    Cmd(name: name, args: args, block: nil)
}

private func num(_ v: Double) -> Arg {
    .number(round2(v))
}

/// The compact form of go/tenn's printer with tab indentation and packed commands: the text
/// canvas2tenn writes.
private struct CompactPrinter {
    private let indent = "\t"
    private let limit = 160

    func render(_ command: Cmd) -> String {
        var out = ""
        line(&out, command, 0)
        return out
    }

    private func line(_ out: inout String, _ c: Cmd, _ ind: Int) {
        out += String(repeating: indent, count: ind)
        out += c.name
        for a in c.args {
            out += " "
            out += atom(a)
        }
        guard let block = c.block else { return }
        out += " "
        let width = lastLineWidth(out)
        if let one = oneLine(block), width + one.utf8.count <= limit {
            out += one
            return
        }
        out += "{\n"
        lines(&out, block, ind + 1)
        out += "\n" + String(repeating: indent, count: ind) + "}"
    }

    private func lines(_ out: inout String, _ cmds: [Cmd], _ ind: Int) {
        var j = 0
        while j < cmds.count {
            if j > 0 {
                out += "\n"
            }
            let run = packRun(Array(cmds[j...]), ind)
            if run.count > 1 {
                out += String(repeating: indent, count: ind) + run.joined(separator: "; ")
                j += run.count
                continue
            }
            line(&out, cmds[j], ind)
            j += 1
        }
    }

    private func packRun(_ cmds: [Cmd], _ ind: Int) -> [String] {
        var width = ind * indent.utf8.count
        var run: [String] = []
        for c in cmds {
            guard c.block == nil, let text = plain(c), !text.contains("\n") else { break }
            if !run.isEmpty {
                if width + 2 + text.utf8.count > limit {
                    break
                }
                width += 2
            }
            width += text.utf8.count
            run.append(text)
        }
        return run
    }

    private func plain(_ c: Cmd) -> String? {
        ([c.name] + c.args.map(atom)).joined(separator: " ")
    }

    private func oneLine(_ block: [Cmd]) -> String? {
        if block.isEmpty {
            return "{}"
        }
        var parts: [String] = []
        for c in block {
            guard c.block == nil, let text = plain(c), !text.contains("\n") else { return nil }
            parts.append(text)
        }
        return "{ " + parts.joined(separator: "; ") + " }"
    }

    private func atom(_ a: Arg) -> String {
        switch a {
        case .number(let v): return String(v)
        case .string(let s): return bareSafe(s) ? s : "\"" + s.replacingOccurrences(of: "\"", with: "\\\"") + "\""
        }
    }

    private func lastLineWidth(_ s: String) -> Int {
        guard let i = s.lastIndex(of: "\n") else { return s.utf8.count }
        return s[s.index(after: i)...].utf8.count
    }
}

/// bareSafe of go/tenn: the word lexes as one symbol token, never a number, so the parser reads the
/// same text back.
private func bareSafe(_ s: String) -> Bool {
    if s.isEmpty {
        return false
    }
    for u in s.unicodeScalars {
        if CharacterSet.letters.contains(u) || CharacterSet.decimalDigits.contains(u)
            || u == "_" || u == "-" || u == "." || u == "#"
        {
            continue
        }
        return false
    }
    return detectSymbolType(s) == .symbol
}

private enum SymbolType {
    case symbol, int, float
}

/// DetectSymbolType of go/tenn: a run of digits is a number, anything else a symbol; a leading - is the
/// sign of the number, not part of it.
private func detectSymbolType(_ s: String) -> SymbolType {
    var skipFirst = s.first == "-"
    var dot = false
    var digits = 0
    for u in s.unicodeScalars {
        if skipFirst {
            skipFirst = false
            continue
        }
        if u == "." {
            if digits == 0 || dot {
                return .symbol
            }
            dot = true
            continue
        }
        guard CharacterSet.decimalDigits.contains(u) else {
            return .symbol
        }
        digits += 1
    }
    return dot ? .float : .int
}
