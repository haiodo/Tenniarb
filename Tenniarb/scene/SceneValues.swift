//
//  SceneValues.swift
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
import JavaScriptCore
import Tenn

/// The element model as a classic diagram reads it: go/classic, the part of go/render the scene kept so an
/// old file draws as go/render drew it.
extension Scene {
    /// classic.Context: the evaluator of el; the caller decides where the expressions run.
    typealias ClassicContext = (_ el: Element) -> ExecutionContext

    /// classic.Values: the values of classic diagram el for Options.Classic, the text of every argument of
    /// prop on the node at, evaluated by ctx. An argument without a value is its literal text; a node or a
    /// property the model does not hold gives nothing.
    static func classicValues(_ el: Element, _ ctx: ExecutionContext?) -> ClassicFunc {
        var targets: [ClassicRef: ClassicTarget] = [ClassicRef(kind: .item, name: el.name, root: true): .element(el)]
        var items: [String: Int] = [:]
        for it in el.items where it.kind == .Item {
            let key = it.name.precomposedStringWithCanonicalMapping
            targets[ClassicRef(kind: .item, name: it.name, nth: items[key] ?? 0)] = .item(it)
            items[key] = (items[key] ?? 0) + 1
        }
        var links: [[String]: Int] = [:]
        for it in el.items where it.kind == .Link {
            let (from, to) = endNames(it)
            let key = [from.precomposedStringWithCanonicalMapping, to.precomposedStringWithCanonicalMapping]
            targets[ClassicRef(kind: .link, from: from, to: to, nth: links[key] ?? 0)] = .item(it)
            links[key] = (links[key] ?? 0) + 1
        }
        return { at, prop in
            guard let target = targets[at], let cmd = target.properties.get(prop) else { return nil }
            var ev: [TennToken: JSValue] = [:]
            if let ctx {
                switch target {
                case let .element(el): ev = ctx.getEvaluated(el)
                case let .item(it): ev = ctx.getEvaluated(it)
                }
            }
            var out: [String] = []
            for a in argsOf(cmd) {
                if let token = a.token, let v = ev[token] {
                    out.append(jsValue(v))
                    continue
                }
                out.append(a.getIdentText() ?? "")
            }
            return out
        }
    }

    /// classic.Hook: Options.Classic for path when it names an element of the element model, so a classic
    /// diagram reads the values the model computes. nil for a file root, an element the file does not
    /// declare, or one whose content is in another file: those keep their own evaluator.
    static func classicHook(_ tree: Tree, file: String, path: String, context: ClassicContext?) -> ClassicFunc? {
        guard !path.isEmpty, let context, let src = fileSource(tree, file) else { return nil }
        // The file is parsed again, as classic.Hook does: the values come from the tokens of this parse.
        let parser = TennParser()
        let ast = parser.parse(src.hasPrefix("\u{FEFF}") ? String(src.dropFirst()) : src)
        guard !parser.errors.hasErrors() else { return nil }
        // shortcut: element --from-file (model.FromFile) is not read by the Swift model, so an element whose
        // content is mounted answers no values instead of no hook; both fall back to the scene evaluator.
        guard let el = findElement(ElementModel.parseTenn(node: ast), path) else { return nil }
        var vals: ClassicFunc?
        return { at, prop in
            if vals == nil {
                vals = classicValues(el, context(el))
            }
            return vals?(at, prop)
        }
    }

    /// classic.SceneContext: the evaluator of el as the element model runs it.
    ///
    /// shortcut: the item sizes of classic/sizes.go are not wired, so width, height and utils.textSize of an
    /// expression come from the app renderer of the item, not from the box the scene draws; port Sizes and
    /// ItemAt when a file reads them.
    static func classicContext(_ el: Element) -> ExecutionContext {
        let ctx = ExecutionContext()
        ctx.setElement(el)
        return ctx
    }
}

/// The element or item a ClassicRef names: only these two carry properties.
private enum ClassicTarget {
    case element(Element)
    case item(DiagramItem)

    var properties: ModelProperties {
        switch self {
        case let .element(el): return el.properties
        case let .item(it): return it.properties
        }
    }
}

/// fileSource: the text of file as the tree keyed it, relative to the workspace root with slashes.
private func fileSource(_ tree: Scene.Tree, _ file: String) -> String? {
    let abs = file.hasPrefix("/") ? URL(fileURLWithPath: file) : tree.root.appendingPathComponent(file)
    let root = tree.root.path
    let path = abs.standardizedFileURL.path
    guard path.hasPrefix(root + "/") else { return nil }
    return tree.source(String(path.dropFirst(root.count + 1)))
}

/// findElement follows a dotted scene path (spec 4.2) through the elements of the model: a quoted segment
/// is a name as written, a bare one its key.
private func findElement(_ top: Element, _ path: String) -> Element? {
    guard let segs = splitPath(path) else { return nil }
    var el = top
    for seg in segs {
        guard let next = el.elements.first(where: { seg.isName($0.name) }) else { return nil }
        el = next
    }
    return el
}

/// The names of the items a link joins, as the format writes them.
private func endNames(_ link: DiagramItem) -> (from: String, to: String) {
    let l = link as? LinkItem
    return (l?.source?.name ?? "", l?.target?.name ?? "")
}

/// The arguments of a command: its children after the name, a block excluded.
private func argsOf(_ cmd: TennNode) -> [TennNode] {
    guard let children = cmd.children, children.count > 1 else { return [] }
    var out = Array(children[1...])
    if out.last?.kind == .BlockExpr {
        out.removeLast()
    }
    return out
}

/// JSValue.toString() of an expression value, as go/classic writes it: an array joins with commas and an
/// undefined element leaves an empty place.
private func jsValue(_ v: JSValue) -> String { v.toString() ?? "" }
