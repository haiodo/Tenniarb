//
//  SceneComponents.swift
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

extension Scene {
    /// A node of a library one can insert: its name, the use path and the branch it is defined in.
    struct Component {
        var path: String
        var name: String
        var group: String
        var node: Node
    }
}

/// The elements, items and uses written directly in n, in file order: what a path can name.
func sceneNodes(_ n: Scene.Node) -> [Scene.Node] {
    if n.kind == .use {
        return []
    }
    var out = n.elems
    for c in n.content where c.kind == .item || c.kind == .use {
        out.append(c)
    }
    return out.enumerated()
        .sorted { a, b in
            a.element.range.start == b.element.range.start
                ? a.offset < b.offset : a.element.range.start < b.element.range.start
        }
        .map(\.element)
}

func isPlate(_ name: String) -> Bool { name.hasPrefix("§") }

/// The path a use of n writes from the file that mounts it as alias; nil when n cannot be used:
/// not an element, an item or a use, or inside an instance or a slot.
func usePath(_ alias: String, _ n: Scene.Node?) -> String? {
    guard let n, n.kind == .element || n.kind == .item || n.kind == .use else { return nil }
    var segs = [n.key]
    for p in sequence(first: n.parent, next: { $0?.parent }) {
        guard let p, p.kind != .file else { break }
        guard p.kind == .item || p.kind == .element else { return nil }
        segs.append(p.key)
    }
    let path = segs.reversed().joined(separator: ".")
    return alias.isEmpty ? path : alias + "." + path
}

/// What a library offers in file order, depth first, those whose name or key holds q, at most limit.
/// A marked component or one with parameters is one; plates, caption groups and parts are not.
func sceneComponents(alias: String, root: Scene.Node, q: String, limit: Int) -> [Scene.Component] {
    let query = q.lowercased()
    let marked = declaresComponents(root)
    func isComponent(_ k: Scene.Node) -> Bool {
        if isPlate(k.name) || sceneNodes(k).isEmpty {
            return false
        }
        if k.type == "component" || !k.params.isEmpty {
            return true
        }
        if marked || captionOnly(k) {
            return false
        }
        return true
    }
    var out: [Scene.Component] = []
    func walk(_ n: Scene.Node, _ where_: String) {
        for k in sceneNodes(n) {
            if out.count >= limit {
                return
            }
            if k.kind == .element {
                // An element that says it is a component is one, not a folder of them; one that says
                // nothing is a group, walked as Go does.
                if k.type == "component" || !k.params.isEmpty {
                    if let comp = componentOf(alias, k, where_, query) {
                        out.append(comp)
                    }
                    continue
                }
                walk(k, k.name)
                continue
            }
            if !isComponent(k) {
                continue
            }
            if let comp = componentOf(alias, k, where_, query) {
                out.append(comp)
            }
        }
    }
    walk(root, "")
    return out
}

/// k as a component of the library alias, when its name holds q and its path is one a use can name.
private func componentOf(_ alias: String, _ k: Scene.Node, _ where_: String, _ q: String) -> Scene.Component? {
    let hit = q.isEmpty || k.name.lowercased().contains(q) || k.key.lowercased().contains(q)
    guard let p = usePath(alias, k), !k.name.isEmpty, hit else { return nil }
    return Scene.Component(path: p, name: k.name, group: where_, node: k)
}

/// Whether a library says which nodes are components: "kind component". A library that says nothing
/// keeps the older rule.
func declaresComponents(_ root: Scene.Node) -> Bool {
    for k in sceneNodes(root) {
        if k.type == "component" || declaresComponents(k) {
            return true
        }
    }
    return false
}

/// The box a group sits on: its only content is the title a plate carries.
func captionOnly(_ k: Scene.Node) -> Bool {
    let kids = sceneNodes(k)
    return kids.count == 1 && kids[0].kind == .item && kids[0].name == "title" && sceneNodes(kids[0]).isEmpty
}

/// Puts the components of one group side by side, the groups in the order they first appear.
func byGroup(_ input: [Scene.Component]) -> [Scene.Component] {
    var order: [String] = []
    var members: [String: [Scene.Component]] = [:]
    for c in input {
        if members[c.group] == nil {
            order.append(c.group)
        }
        members[c.group, default: []].append(c)
    }
    return order.flatMap { members[$0] ?? [] }
}

/// Where Insert Component puts a component while sel is selected: an item, an instance (slots to
/// choose from, "" the default one), or nil for a free spot on the canvas.
func insertTarget(_ tree: Scene.Tree, _ sel: Scene.Node?) -> (into: Scene.Node?, slots: [String]) {
    guard let sel else { return (nil, []) }
    switch sel.kind {
    case .item:
        return (sel, [])
    case .use:
        guard let parent = sel.parent, let comp = try? tree.resolve(parent, sel.path) else {
            return (nil, [])
        }
        let (def, named) = componentSlots(comp)
        if !named.isEmpty && def {
            return (sel, [""] + named)
        }
        if !named.isEmpty {
            return (sel, named)
        }
        return def ? (sel, []) : (nil, [])
    default:
        return (nil, [])
    }
}

/// The slots a component defines, not those of the instances inside it. A component that is itself a
/// use reports none; its open slots need the expanded instance.
func componentSlots(_ comp: Scene.Node) -> (def: Bool, named: [String]) {
    var def = false
    var named: [String] = []
    func walk(_ n: Scene.Node) {
        for c in n.content {
            switch c.kind {
            case .slot:
                if c.name.isEmpty {
                    def = true
                } else {
                    named.append(c.name)
                }
                walk(c)
            case .item:
                walk(c)
            default:
                break
            }
        }
    }
    if comp.kind == .item {
        walk(comp)
    }
    return (def, named)
}

/// Wraps cmd into the fill of a named slot; the default slot takes cmd as it is.
func slotCommand(_ slot: String, _ cmd: String) -> String {
    slot.isEmpty ? cmd : commandText("slot", slot) + " { " + cmd + " }"
}

/// A name no node under n uses, as base, base_2, base_3.
func freeName(_ n: Scene.Node, _ base: String) -> String {
    var taken = Set<String>()
    for c in sceneNodes(n) { taken.insert(c.key) }
    for c in n.content { taken.insert(c.key) }
    var name = base
    var i = 2
    while taken.contains(normalizeName(name)) {
        name = base + "_" + String(i)
        i += 1
    }
    return name
}

/// The deepest node that holds every one of nodes; nil for none.
func commonAncestor(_ nodes: [Scene.Node]) -> Scene.Node? {
    guard let first = nodes.first else { return nil }
    for a in sequence(first: first.parent, next: { $0?.parent }) {
        guard let a else { break }
        if nodes.dropFirst().allSatisfy({ n in
            sequence(first: n.parent, next: { $0?.parent }).contains { $0 === a }
        }) {
            return a
        }
    }
    return nil
}

/// The name of the child of root that n sits in: the top branch under the shared root.
func branchOf(_ root: Scene.Node?, _ n: Scene.Node) -> String {
    guard let root else { return "" }
    for p in sequence(first: n, next: { $0.parent }) {
        if p.parent === root {
            return p === n ? "" : p.name
        }
    }
    return ""
}
