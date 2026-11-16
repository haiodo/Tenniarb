//
//  SceneStyles.swift
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

/// The text properties an item takes from its parent when it sets none of its own, as font properties inherit
/// in CSS: a component sets the font once on its root (spec 5b).
private let inheritFont = ["font-family", "font-size", "font-weight", "font-style", "letter-spacing"]

/// Gives every node the properties of the styles it names with use-style: a style comes where its use-style is
/// written, so properties above it are defaults and properties below it win; kid blocks reach the kids as their
/// defaults.
func applyStyles(_ r: Scene.RNode) {
    if let src = r.src {
        let def = r.def ?? src
        if let scope = styleOwner(def) {
            let names = styleRefs(r.props)
            var props: [Scene.Prop] = []
            for name in names {  // the last style named wins per property
                if let st = scope.styles?[name] {
                    props += st.props
                }
            }
            r.props = afterUseStyle(r.props, props)
            // Kid blocks prepend, so the styles go in reverse: the one named last stays nearest the end.
            for name in names.reversed() {
                if let st = scope.styles?[name] {
                    applyKidStyles(r, st)
                }
            }
        }
    }
    for k in r.kids {
        inheritProps(k, r)
        applyStyles(k)
    }
}

/// Whether an argument is an expression, or a string with one inside.
func hasExpr(_ pr: Scene.Prop) -> Bool {
    if pr.val != nil {  // already evaluated
        return false
    }
    for a in pr.args {
        if a.kind == .Expression || a.kind == .ExpressionBlock {
            return true
        }
        if a.kind == .StringLit, let s = a.getIdentText(), s.contains("${") {
            return true
        }
    }
    return false
}

/// The styles props name with use-style, in order; "-name" drops the one named before. An expression names its
/// styles only once evaluated: before that its words are not style names.
private func styleRefs(_ props: [Scene.Prop]) -> [String] {
    var names: [String] = []
    for p in props where p.name == "use-style" {
        if p.val == nil && hasExpr(p) {
            continue
        }
        for i in 0..<p.args.count {
            guard let text = p.text(i) else { continue }
            // A parameter may hold several styles: "extras outlined rounded" names both.
            for word in text.split(whereSeparator: { $0.isWhitespace }) {
                if word.hasPrefix("-") {
                    dropName(&names, String(word.dropFirst()))
                    continue
                }
                names.append(normalizeName(String(word)))
            }
        }
    }
    return names
}

private func dropName(_ names: inout [String], _ name: String) {
    let key = normalizeName(name)
    names.removeAll { $0 == key }
}

/// Puts the properties of the styles right after the last use-style of the node: what is written before is a
/// default the style overrides, what is written after overrides the style.
private func afterUseStyle(_ props: [Scene.Prop], _ add: [Scene.Prop]) -> [Scene.Prop] {
    if add.isEmpty {
        return props
    }
    var at = 0
    for (i, p) in props.enumerated() where p.name == "use-style" {
        at = i + 1
    }
    return Array(props[..<at]) + add + Array(props[at...])
}

/// Gives the kids a style names by name the properties of that block, as a CSS descendant selector does; they go
/// first, so a kid's own and style properties win.
private func applyKidStyles(_ r: Scene.RNode, _ st: Scene.Style) {
    for sk in st.kids {
        for k in r.kids where k.key == sk.key {
            k.props = sk.style.props + k.props
            applyKidStyles(k, sk.style)
        }
    }
}

/// Gives an item the text properties of its parent when it sets none itself; they come first, so own and style
/// properties win over them.
private func inheritProps(_ k: Scene.RNode, _ parent: Scene.RNode) {
    if k.kind != .item {
        return
    }
    for name in inheritFont {
        if k.prop(name) != nil {
            continue
        }
        if let pr = parent.prop(name) {
            k.props.insert(pr, at: 0)
        }
    }
}

/// The node whose styles a node may name: the node itself when it holds a styles block (an item that is a
/// component), else the nearest element above it.
private func styleOwner(_ n: Scene.Node) -> Scene.Node? {
    if hasStyles(n) {
        return n
    }
    return styleScope(n)
}

/// The element whose styles a node may name: the nearest one above it in its own file.
private func styleScope(_ n: Scene.Node) -> Scene.Node? {
    var p = n.parent
    while let cur = p {
        if hasStyles(cur) {
            return cur
        }
        p = cur.parent
    }
    return nil
}

private func hasStyles(_ n: Scene.Node) -> Bool {
    !(n.styles?.isEmpty ?? true) || !n.styleGroups.isEmpty
}

/// Gives the nodes of a classic diagram the default look its element declares with styles { item { … } } and
/// styles { line { … } }: go/render reads those two names as the look of every box and every link of the element,
/// not as styles a node has to name.
func classicDefaults(_ r: Scene.RNode) {
    guard let src = r.src else { return }
    let item = src.styles?["item"]
    let line = src.styles?["line"]
    if item == nil && line == nil {
        return
    }
    func walk(_ n: Scene.RNode) {
        switch n.kind {
        case .item:
            n.props = defaultProps("item", item, n) + n.props
        case .link:
            n.props = defaultProps("line", line, n) + n.props
        case .slot:
            break
        }
        for k in n.kids {
            walk(k)
        }
    }
    for k in r.kids {
        walk(k)
    }
}

/// The properties the default style name gives n. A node that names the same style takes it through use-style
/// alone, and a shadow of its own replaces the default one: go/render parsed the default first and the node's
/// properties over it.
private func defaultProps(_ name: String, _ st: Scene.Style?, _ n: Scene.RNode) -> [Scene.Prop] {
    guard let st, !styleRefs(n.props).contains(normalizeName(name)) else { return [] }
    let ownShadow = n.props.contains { $0.name == "shadow" }
    return st.props.filter { !(ownShadow && $0.name == "shadow") }
}
