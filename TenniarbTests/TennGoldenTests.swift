//
//  TennGoldenTests.swift
//  TenniarbTests
//
//  Golden fixtures shared with the TypeScript parser in web/packages/core.
//  Regenerate: make golden (TENN_GOLDEN_UPDATE=1).
//

import JavaScriptCore
import XCTest

@testable import Tenniarb

class TennGoldenTests: XCTestCase {

    private let fixtures = URL(fileURLWithPath: #filePath)
        .deletingLastPathComponent().deletingLastPathComponent()
        .appendingPathComponent("web/fixtures")

    private func tokenJSON(_ t: TennToken) -> [String: Any] {
        return [
            "type": "\(t.type)", "literal": t.literal,
            "line": t.line, "col": t.col, "pos": t.pos, "size": t.size,
        ]
    }

    private func nodeJSON(_ n: TennNode) -> [String: Any] {
        var r: [String: Any] = ["kind": "\(n.kind)"]
        if let t = n.token { r["token"] = tokenJSON(t) }
        if let c = n.children { r["children"] = c.map(nodeJSON) }
        return r
    }

    private func golden(_ source: String) -> String? {
        var tokens: [[String: Any]] = []
        let lexer = TennLexer(source)
        while let t = lexer.getToken() {
            tokens.append(tokenJSON(t))
            if t.type == .eof || tokens.count > 1_000_000 { break }
        }
        let parser = TennParser()
        let tree = parser.parse(source)
        let errors: [[String: Any]] = parser.errors.errors.map {
            ["code": "\($0.errorCode)", "message": $0.message, "line": $0.line, "col": $0.col]
        }
        let doc: [String: Any] = [
            "tokens": tokens, "tree": nodeJSON(tree),
            "printed": tree.toStr(), "printedClean": tree.toStr(0, true), "errors": errors,
        ]
        return jsonText(doc)
    }

    private func jsonText(_ doc: [String: Any]) -> String? {
        guard
            let data = try? JSONSerialization.data(
                withJSONObject: doc, options: [.prettyPrinted, .sortedKeys, .withoutEscapingSlashes])
        else { return nil }
        return String(decoding: data, as: UTF8.self) + "\n"
    }

    /// Model golden (<name>.model.json), made from ElementModel.parseTenn(node:) as Document.read does.
    /// Only for documents without parse errors (Document.read ignores files with errors).
    ///
    /// Root object is an Element. Fields:
    /// - Element: {"kind": "Root"|"Element", "name", "description"?, "properties": [Node], "items": [Item], "elements": [Element]}
    ///   - the root is the ElementModel that parseTenn returns (kind "Root", name "Root"); top-level `element`/`model`
    ///     commands become its "elements"; any other top-level statement goes to the root "properties".
    ///   - "description" is omitted when nil. The loader never sets it for elements (a `description` command
    ///     inside an element lands in "properties"), kept for completeness.
    ///   - "properties" are the TennNodes the loader did not consume (styles, unknown commands, bad `pos`, ...),
    ///     in source order. Node = {"kind", "token"?, "children"?} exactly as in .parse.json (same token shape).
    ///   - "items" holds both items and links in source order (as in element.items).
    /// - Item: {"kind": "Item"|"Link", "name", "x", "y", "description"?, "properties": [Node]}
    ///   - x, y come from the `pos` command (0 when absent); "name" of a Link is its `label` (empty when absent).
    ///   - description comes from `description` (also the misspelt `desription`); "properties" are the other commands.
    /// - Link also has "source" and "target": index into the SAME element's "items" array of the
    ///   referenced Item, or null when the endpoint was not resolved (name not found / index out of range).
    ///   Resolution rule (processLink): endpoint name + optional `source-index`/`target-index` (default 0) selects the
    ///   n-th Item (kind Item only, in order) with that name; links are not counted. Only resolved at load time.
    /// - Ids are random UUIDs in the app and are not serialized; items are referenced by array index, elements by path.
    /// - Transient/derived state (parent, model, ox, oy, images, modelName) is not serialized.
    private func modelJSON(_ e: Element) -> [String: Any] {
        var r: [String: Any] = [
            "kind": e.kind == .Root ? "Root" : "Element", "name": e.name,
            "properties": e.properties.map(nodeJSON),
            "items": e.items.map { itemJSON($0, e) }, "elements": e.elements.map(modelJSON),
        ]
        if let d = e.description { r["description"] = d }
        return r
    }

    private func itemJSON(_ i: DiagramItem, _ owner: Element) -> [String: Any] {
        var r: [String: Any] = [
            "kind": i.kind == .Link ? "Link" : "Item", "name": i.name, "x": Double(i.x), "y": Double(i.y),
            "properties": i.properties.map(nodeJSON),
        ]
        if let d = i.description { r["description"] = d }
        if let l = i as? LinkItem {
            r["source"] = l.source.flatMap { owner.items.firstIndex(of: $0) } ?? NSNull()
            r["target"] = l.target.flatMap { owner.items.firstIndex(of: $0) } ?? NSNull()
        }
        return r
    }

    /// Returns (model json, saved tenn) or nil when the document has parse errors.
    private func persisted(_ source: String) -> (String, String)? {
        let parser = TennParser()
        let tree = parser.parse(source)
        if parser.errors.hasErrors() { return nil }
        let model = ElementModel.parseTenn(node: tree)
        guard let json = jsonText(modelJSON(model)) else { return nil }
        return (json, model.toTennStr())
    }

    /// Evaluation golden (<name>.eval.json): what the app's ExecutionContext computes for a document.
    /// Written only when at least one expression was evaluated. Same documents as .model.json (no parse errors).
    ///
    /// How it is made: for every non-Root Element of the parsed model (pre-order, same order as .model.json), a fresh
    /// ExecutionContext gets setElement(element) - this is all the app runs on load (SceneDrawView.setElementModel);
    /// updateAll only runs for diagrams with an `animation` property. Values are then read the way the scene reads them
    /// (ElementScene.buildItemDrawable): ctx.getEvaluated(element) and ctx.getEvaluated(item), a [TennToken: JSValue].
    /// The Root element is not evaluated (the app never displays it).
    ///
    /// Schema: {"elements": [ElementEval]}, only elements with at least one evaluated value are listed.
    /// - ElementEval: {"path": [Int], "name", "element": [Entry], "items": [ItemEval]}
    ///   - path: indices through "elements" arrays of .model.json, starting at the Root (so [0] is the first top-level element).
    ///   - "element": entries from the element's own properties, "items" only those items that have entries.
    /// - ItemEval: {"index": Int, "kind": "Item"|"Link", "name", "entries": [Entry]} - index into "items" of the same element
    ///   in .model.json (links included).
    /// - Entry: {"path": [Int], "kind": TennNode kind, "literal": token literal, "value": Value}, in source order.
    ///   - path: first index selects a node of the "properties" array of that element/item in .model.json, the rest are
    ///     "children" indices. The node at the path is the one carrying the token the app keys the value by:
    ///     Expression (`$(...)`), ExpressionBlock (`${...}`), or a string/ident literal containing `${` (template, evaluated
    ///     as a JS template literal).
    ///   - Only evaluated nodes get entries; plain literals (`val 42`) have none.
    /// - Value (JSValue, as a tagged object; the "t" field is the tag):
    ///   - {"t": "undefined"} | {"t": "null"} | {"t": "boolean", "v": Bool}
    ///   - {"t": "number", "v": Double}; non-finite numbers use a string "v": "NaN" | "Infinity" | "-Infinity"
    ///   - {"t": "string", "v": String}
    ///   - {"t": "array", "v": [Value]}
    ///   - {"t": "object", "v": {key: Value}} - keys from JS `for in` (own + inherited enumerable), so the Swift-bridged
    ///     `pos` object shows x/y. Nesting deeper than 6 gives {"t": "truncated"}.
    ///   - {"t": "function"}
    ///   - {"t": "error", "name", "message"}: a JS exception caught while evaluating (the app stores the exception object).
    ///     name/message come from JavaScriptCore; message text is engine specific ("Can't find variable: x"), so a
    ///     port on another JS engine should compare only "t" and "name" for errors. A thrown non-Error value (`throw "s"`)
    ///     is stored as is, e.g. a plain string.
    /// Not covered: the item scope (width/height of drawables depend on text measuring), utils.now() value.
    private func evalJSON(_ source: String) -> String? {
        let parser = TennParser()
        let tree = parser.parse(source)
        if parser.errors.hasErrors() { return nil }
        let model = ElementModel.parseTenn(node: tree)

        var out: [[String: Any]] = []
        func visit(_ e: Element, _ path: [Int]) {
            let ctx = ExecutionContext()
            ctx.setElement(e)
            var r: [String: Any] = ["path": path, "name": e.name]
            let own = evalEntries(e.properties, ctx.getEvaluated(e))
            r["element"] = own
            var items: [[String: Any]] = []
            for (idx, itm) in e.items.enumerated() {
                let entries = evalEntries(itm.properties, ctx.getEvaluated(itm))
                if entries.isEmpty { continue }
                items.append([
                    "index": idx, "kind": itm.kind == .Link ? "Link" : "Item", "name": itm.name, "entries": entries,
                ])
            }
            r["items"] = items
            if !own.isEmpty || !items.isEmpty { out.append(r) }
            for (i, c) in e.elements.enumerated() { visit(c, path + [i]) }
        }
        for (i, c) in model.elements.enumerated() { visit(c, [i]) }
        if out.isEmpty { return nil }
        return jsonText(["elements": out])
    }

    private func evalEntries(_ props: ModelProperties, _ ev: [TennToken: JSValue]) -> [[String: Any]] {
        var entries: [[String: Any]] = []
        func walk(_ n: TennNode, _ path: [Int]) {
            if let t = n.token, let v = ev[t] {
                entries.append(["path": path, "kind": "\(n.kind)", "literal": t.literal, "value": evalValue(v, 0)])
            }
            for (i, c) in (n.children ?? []).enumerated() { walk(c, path + [i]) }
        }
        for (i, p) in props.node.children?.enumerated() ?? [TennNode]().enumerated() { walk(p, [i]) }
        XCTAssertEqual(entries.count, ev.count, "evaluated tokens not reachable from properties")
        return entries
    }

    private func evalValue(_ v: JSValue, _ depth: Int) -> [String: Any] {
        if v.isUndefined { return ["t": "undefined"] }
        if v.isNull { return ["t": "null"] }
        if v.isBoolean { return ["t": "boolean", "v": v.toBool()] }
        if v.isNumber {
            let d = v.toDouble()
            if d.isFinite { return ["t": "number", "v": d] }
            return ["t": "number", "v": d.isNaN ? "NaN" : (d > 0 ? "Infinity" : "-Infinity")]
        }
        if v.isString { return ["t": "string", "v": v.toString() ?? ""] }
        let ctx = v.context!
        if v.isInstance(of: ctx.objectForKeyedSubscript("Error")) {
            return ["t": "error", "name": v.forProperty("name")?.toString() ?? "", "message": v.forProperty("message")?.toString() ?? ""]
        }
        if depth > 6 { return ["t": "truncated"] }
        if v.isArray {
            let n = Int(v.forProperty("length")?.toInt32() ?? 0)
            return ["t": "array", "v": (0..<n).map { evalValue(v.atIndex($0), depth + 1) }]
        }
        if v.isObject {
            if v.forProperty("call")?.isObject == true && v.isInstance(of: ctx.objectForKeyedSubscript("Function")) {
                return ["t": "function"]
            }
            let keys =
                ctx.evaluateScript("(function(o){var r=[];for(var k in o)r.push(k);return r})")!
                .call(withArguments: [v])?.toArray() as? [String] ?? []
            var obj: [String: Any] = [:]
            for k in keys { obj[k] = evalValue(v.forProperty(k), depth + 1) }
            return ["t": "object", "v": obj]
        }
        return ["t": "undefined"]
    }

    private func rectJSON(_ r: CGRect) -> [String: Any] {
        return ["x": Double(r.origin.x), "y": Double(r.origin.y), "width": Double(r.width), "height": Double(r.height)]
    }

    /// Layout golden (<name>.layout.json): the layouts the app offers, run on every Element that has Items.
    /// Written only when some element has at least one Item. Only documents without parse errors.
    ///
    /// Offered layouts: GridLayout (SceneDrawView.performGridLayout) and SpringLayout (performSpringLayout). TreeLayout is a
    /// stub (apply returns []) with no callers, so it is not recorded.
    ///
    /// Setup per element (as the app does): ExecutionContext.setElement(element); DrawableScene(element, darkMode: false,
    /// executionContext: ctx, scaleFactor: 1); LayoutContext(element, scene, store, bounds: viewBounds); algorithm.apply(clean: true).
    /// The resulting UpdatePosition operations are read (newValue) and NOT applied, so both algorithms see the same input.
    ///
    /// Schema: {"elements": [LayoutEval]}, elements without Items are not listed.
    /// - LayoutEval: {"path": [Int] (as in .eval.json), "name", "sceneBounds": Rect, "nodes": [Node], "edges": [Edge],
    ///   "grid": {"viewBounds": Rect, "positions": [Pos]}, "spring": {"viewBounds": Rect, "params": {...}, "positions": [Pos]}}
    /// - Rect: {"x", "y", "width", "height"} (CGRect, origin + size, doubles).
    /// - INPUT. Node: {"item": index into "items" of the element in .model.json, "name", "x", "y" (item.x/y from `pos`),
    ///   "bounds": Rect} - bounds is scene.drawables[item].getBounds(), i.e. LayoutContext.getBounds(node:), measured by the
    ///   scene (text measuring with AppKit fonts). The port can feed these sizes instead of a renderer.
    ///   Nodes are the Items (not Links) in element order; this is the order the algorithms use.
    ///   Edge: {"item": index, "source": node item index | null, "target": node item index | null} (all Links, in order).
    ///   "sceneBounds" is scene.getBounds() = LayoutContext.getBounds() (GridLayout reads it; it ignores viewBounds).
    /// - "grid".viewBounds: what the app passes (view.bounds); fixed here to (0, 0, 800, 600).
    /// - "spring".viewBounds: fixed (-400, -300, 800, 600), the app passes a rect of the view size centered at 0.
    ///   params: SpringLayout defaults, with maxTimeMS = 0 (see below).
    /// - OUTPUT. Pos: {"item": index, "x", "y"} in operation order (= node order), the `newValue` of UpdatePosition.
    ///   Grid positions are the item origin (bottom-left based on scene bounds); spring positions are the item's
    ///   center-based location as saved by the algorithm (stored directly into item.x/y by the app).
    /// SpringLayout determinism: sprRandom is false by default (no drand48), so placement starts from the item positions.
    /// The only nondeterminism is time: with maxTimeMS > 0 the iteration counter skips ahead with wall-clock time, so the
    /// app result depends on machine speed. Here setSpringTimeout(0) is used, which disables that and runs exactly
    /// sprIterations (1000) iterations. Floating point results of 1000 iterations should be compared with a tolerance.
    private func layoutJSON(_ source: String) -> String? {
        let parser = TennParser()
        let tree = parser.parse(source)
        if parser.errors.hasErrors() { return nil }
        let model = ElementModel.parseTenn(node: tree)
        let store = ElementModelStore(model)

        var out: [[String: Any]] = []
        func positions(_ ops: [ElementOperation], _ e: Element) -> [[String: Any]] {
            return ops.compactMap { op in
                guard let up = op as? UpdatePosition, let idx = e.items.firstIndex(of: up.item) else { return nil }
                return ["item": idx, "x": Double(up.newValue.x), "y": Double(up.newValue.y)]
            }
        }
        func visit(_ e: Element, _ path: [Int]) {
            if e.items.contains(where: { $0.kind == .Item }) {
                let ctx = ExecutionContext()
                ctx.setElement(e)
                let scene = DrawableScene(e, darkMode: false, executionContext: ctx, scaleFactor: 1)
                let gridView = CGRect(x: 0, y: 0, width: 800, height: 600)
                let springView = CGRect(x: -400, y: -300, width: 800, height: 600)
                let lc = LayoutContext(e, scene: scene, store: store, bounds: gridView)
                let nodes: [[String: Any]] = lc.nodes.map {
                    [
                        "item": e.items.firstIndex(of: $0)!, "name": $0.name, "x": Double($0.x), "y": Double($0.y),
                        "bounds": rectJSON(lc.getBounds(node: $0)),
                    ]
                }
                let edges: [[String: Any]] = lc.edges.map {
                    [
                        "item": e.items.firstIndex(of: $0)!,
                        "source": $0.source.flatMap { e.items.firstIndex(of: $0) } ?? NSNull(),
                        "target": $0.target.flatMap { e.items.firstIndex(of: $0) } ?? NSNull(),
                    ]
                }
                let gridOps = GridLayout().apply(context: lc, clean: true)

                let sl = SpringLayout()
                sl.setSpringTimeout(0)
                let slc = LayoutContext(e, scene: scene, store: store, bounds: springView)
                let springOps = sl.apply(context: slc, clean: true)
                let params: [String: Any] = [
                    "iterations": sl.getIterations(), "maxTimeMS": sl.getSpringTimeout(), "random": sl.getRandom(),
                    "move": Double(sl.getSpringMove()), "strain": Double(sl.getSpringStrain()),
                    "length": Double(sl.getSpringLength()), "gravitation": Double(sl.getSpringGravitation()),
                ]
                out.append([
                    "path": path, "name": e.name, "sceneBounds": rectJSON(lc.getBounds()), "nodes": nodes, "edges": edges,
                    "grid": ["viewBounds": rectJSON(gridView), "positions": positions(gridOps, e)],
                    "spring": ["viewBounds": rectJSON(springView), "params": params, "positions": positions(springOps, e)],
                ])
            }
            for (i, c) in e.elements.enumerated() { visit(c, path + [i]) }
        }
        for (i, c) in model.elements.enumerated() { visit(c, [i]) }
        if out.isEmpty { return nil }
        return jsonText(["elements": out])
    }

    private func firstDiff(_ actual: String, _ expected: String) -> String {
        let a = actual.split(separator: "\n", omittingEmptySubsequences: false)
        let e = expected.split(separator: "\n", omittingEmptySubsequences: false)
        let i = zip(a, e).enumerated().first { $0.element.0 != $0.element.1 }?.offset ?? min(a.count, e.count)
        return "line \(i + 1)\n  expected: \(i < e.count ? e[i] : "<eof>")\n  actual:   \(i < a.count ? a[i] : "<eof>")"
    }

    private func check(_ base: String, _ ext: String, _ actual: String, update: Bool) throws {
        let url = fixtures.appendingPathComponent("\(base).\(ext)")
        if update {
            try actual.write(to: url, atomically: true, encoding: .utf8)
            return
        }
        guard let expected = try? String(contentsOf: url, encoding: .utf8) else {
            XCTFail("\(base): missing \(base).\(ext) (run make golden)")
            return
        }
        if expected != actual { XCTFail("\(base).\(ext): golden mismatch at \(firstDiff(actual, expected))") }
    }

    /// Compare mode for .eval.json: error values are compared by tag and name only, the JSC message text may change per OS.
    private func checkEval(_ base: String, _ actual: String, update: Bool) throws {
        let url = fixtures.appendingPathComponent("\(base).eval.json")
        if update { return try check(base, "eval.json", actual, update: true) }
        guard let expected = try? String(contentsOf: url, encoding: .utf8) else {
            XCTFail("\(base): missing \(base).eval.json (run make golden)")
            return
        }
        if expected == actual { return }
        func strip(_ v: Any) -> Any {
            if let a = v as? [Any] { return a.map(strip) }
            guard var d = v as? [String: Any] else { return v }
            if d["t"] as? String == "error" { d["message"] = nil }
            return d.mapValues(strip)
        }
        func norm(_ t: String) -> NSObject? {
            (try? JSONSerialization.jsonObject(with: Data(t.utf8))).flatMap { strip($0) as? NSObject }
        }
        if norm(expected) != norm(actual) {
            XCTFail("\(base).eval.json: golden mismatch at \(firstDiff(actual, expected))")
        }
    }

    /// Compare mode for .layout.json: sizes come from AppKit text measuring and may differ per OS/font stack. When the
    /// recorded inputs (nodes, sceneBounds) differ from the fresh ones the fixture is skipped with a message; otherwise
    /// grid positions are compared exactly and spring positions with a 1e-6 tolerance.
    private func checkLayout(_ base: String, _ actual: String, update: Bool) throws {
        if update { return try check(base, "layout.json", actual, update: true) }
        let url = fixtures.appendingPathComponent("\(base).layout.json")
        guard let expected = try? String(contentsOf: url, encoding: .utf8) else {
            XCTFail("\(base): missing \(base).layout.json (run make golden)")
            return
        }
        if expected == actual { return }
        func elements(_ t: String) -> [[String: Any]] {
            let obj = try? JSONSerialization.jsonObject(with: Data(t.utf8)) as? [String: Any]
            return obj?["elements"] as? [[String: Any]] ?? []
        }
        func same(_ a: Any?, _ b: Any?, _ tol: Double) -> Bool {
            if let x = a as? [Any], let y = b as? [Any] {
                return x.count == y.count && zip(x, y).allSatisfy { same($0, $1, tol) }
            }
            if let x = a as? [String: Any], let y = b as? [String: Any] {
                return Set(x.keys) == Set(y.keys) && x.keys.allSatisfy { same(x[$0], y[$0], tol) }
            }
            if let x = a as? NSNumber, let y = b as? NSNumber { return abs(x.doubleValue - y.doubleValue) <= tol }
            return (a as? String) == (b as? String)
        }
        let exp = elements(expected)
        let act = elements(actual)
        guard exp.count == act.count else {
            XCTFail("\(base).layout.json: element count \(act.count), expected \(exp.count)")
            return
        }
        for (e, a) in zip(exp, act) {
            let name = "\(e["name"] ?? "?")"
            if !same(e["nodes"], a["nodes"], 1e-9) || !same(e["sceneBounds"], a["sceneBounds"], 1e-9)
                || !same(e["edges"], a["edges"], 0) || !same(e["path"], a["path"], 0)
            {
                print("SKIPPED \(base).layout.json [\(name)]: measured sizes differ from the golden (font/OS), layout not compared")
                continue
            }
            let eg = e["grid"] as? [String: Any]
            let ag = a["grid"] as? [String: Any]
            if !same(eg, ag, 0) { XCTFail("\(base).layout.json [\(name)]: grid positions differ") }
            let es = e["spring"] as? [String: Any]
            let asp = a["spring"] as? [String: Any]
            if !same(es, asp, 1e-6) { XCTFail("\(base).layout.json [\(name)]: spring positions differ") }
        }
    }

    func testGoldenFixtures() throws {
        let update = ProcessInfo.processInfo.environment["TENN_GOLDEN_UPDATE"] == "1"
        let names = try FileManager.default.contentsOfDirectory(atPath: fixtures.path)
            .filter { $0.hasSuffix(".tenn") && !$0.hasSuffix(".saved.tenn") }.sorted()
        XCTAssertFalse(names.isEmpty, "no fixtures in \(fixtures.path)")

        for file in names {
            let base = String(file.dropLast(".tenn".count))
            let source = try String(contentsOf: fixtures.appendingPathComponent(file), encoding: .utf8)
            guard let actual = golden(source) else {
                XCTFail("\(base): JSON serialization failed")
                continue
            }
            try check(base, "parse.json", actual, update: update)
            if let (model, saved) = persisted(source) {
                try check(base, "model.json", model, update: update)
                try check(base, "saved.tenn", saved, update: update)
                if let ev = evalJSON(source) { try checkEval(base, ev, update: update) }
                if let lay = layoutJSON(source) { try checkLayout(base, lay, update: update) }
            }
        }
    }
}
