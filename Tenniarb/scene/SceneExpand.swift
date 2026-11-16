//
//  SceneExpand.swift
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
    /// The kind of a resolved node.
    enum RKind: UInt8 {
        case item
        case link
        case slot  // a slot of a component not yet filled; gone after finalize
    }

    /// A node after use and slots are expanded: what is drawn.
    final class RNode {
        let kind: RKind
        var name: String
        var key: String
        var pos: Pos  // where it is written; for an instance, the use
        var props: [Prop]
        var kids: [RNode] = []  // in file order, links and items mixed (z order)
        var slot = ""  // RSlot: normalized name, "" for the default slot; kids are the fallback
        var inst = false  // the root of an instance: slots below it belong to it, not to the use around
        var touched = false  // RSlot: at or replace went through it into the fallback
        var replaced = false  // RSlot: declared by the nodes of a replace, which win over a slot of the same name
        var from = ""
        var to = ""
        var vars: [String: String]?  // the parameters of its instance and the context as ctx.* keys (5c, 5f)
        var def: Node?  // the item or element the root of an instance stands for
        var src: Node?  // the command written for the node

        init(
            kind: RKind,
            name: String = "",
            key: String = "",
            pos: Pos = Pos(),
            props: [Prop] = [],
            vars: [String: String]? = nil,
            src: Node? = nil
        ) {
            self.kind = kind
            self.name = name
            self.key = key
            self.pos = pos
            self.props = props
            self.vars = vars
            self.src = src
        }

        /// The property in effect: the last one written.
        func prop(_ name: String) -> Prop? {
            for pr in props.reversed() where pr.name == name {
                return pr
            }
            return nil
        }
    }

    /// Expands use instances and slots into the tree that is drawn.
    final class Expander {
        let t: Tree
        var stack: [(n: Node, label: String)] = []
        var depth = 0  // uses being expanded; slots stay open until the outermost one is done
        var ctx: MountCtx?  // mount context of the file the node being expanded lives in
        /// Evaluates the value of a parameter (spec 5c); nil: no expression runs.
        let eval: EvalFunc?
        var vars: [String: String]?  // parameters of the instance being expanded
        var env: [String: String]?  // context declarations in scope, key -> value (spec 5f)

        init(t: Tree, ctx: MountCtx?, eval: EvalFunc? = nil) {
            self.t = t
            self.ctx = ctx
            self.eval = eval
        }
    }
}

private func label(_ n: Scene.Node) -> String {
    switch n.kind {
    case .use: return "use \(n.path) as \(n.name) (\(n.pos))"
    case .item: return "item \"\(n.name)\" (\(n.pos))"
    case .element: return "element \"\(n.name)\" (\(n.pos))"
    default: return "file (\(n.pos))"
    }
}

/// Replaces the slots still open by their fallback content.
private func finalize(_ r: Scene.RNode) {
    var kids: [Scene.RNode] = []
    for k in r.kids {
        finalize(k)
        if k.kind == .slot {
            kids += k.kids
        } else {
            kids.append(k)
        }
    }
    r.kids = kids
}

private struct SlotFill {
    let name: String
    let pos: Scene.Pos
    var nodes: [Scene.RNode]
    var explicit: Bool  // written as slot [name] { ... }: an empty block still replaces the fallback
}

/// Marks the slots the nodes of a replace declare: a fill reaches them before a slot of the same name the
/// component declares elsewhere (spec 4.7a).
private func markReplaced(_ nodes: [Scene.RNode]) {
    for n in nodes {
        if n.kind == .slot {
            n.replaced = true
        }
        markReplaced(n.kids)
    }
}

private func splice(_ kids: [Scene.RNode], _ i: Int, _ with: [Scene.RNode]) -> [Scene.RNode] {
    kids[..<i] + with + kids[(i + 1)...]
}

private func withoutProp(_ props: [Scene.Prop], _ name: String) -> [Scene.Prop] {
    props.filter { $0.name != name }
}

private struct Loc {
    let parent: Scene.RNode
    let i: Int

    var node: Scene.RNode { parent.kids[i] }
}

/// Whether a segment of an at or replace path names the drawn node k; a slot is named "default" when it has no name.
private func matchesSeg(_ k: Scene.RNode, _ seg: Scene.Segment) -> Bool {
    switch k.kind {
    case .item:
        if seg.quoted {
            return seg.name.precomposedStringWithCanonicalMapping == k.name.precomposedStringWithCanonicalMapping
        }
        return k.key == seg.key
    case .slot:
        if seg.quoted {
            return normalizeName(seg.name) == k.slot
        }
        return slotSegment(k) == seg.key
    default:
        return false
    }
}

/// The name a path or a message gives a drawn node; the unnamed slot is "default".
private func rName(_ n: Scene.RNode) -> String {
    n.kind == .slot && n.slot.isEmpty ? "default" : n.name
}

private func slotSegment(_ r: Scene.RNode) -> String {
    r.slot.isEmpty ? "default" : r.slot
}

/// The slot a fill of name goes into: one the nodes of a replace declared wins over one of the component.
private func findSlot(_ r: Scene.RNode, _ name: String) -> Loc? {
    if let l = findSlotIn(r, name, true) {
        return l
    }
    return findSlotIn(r, name, false)
}

private func findSlotIn(_ r: Scene.RNode, _ name: String, _ replaced: Bool) -> Loc? {
    for (i, k) in r.kids.enumerated() {
        if k.kind == .slot && k.slot == name && k.replaced == replaced {
            return Loc(parent: r, i: i)
        }
        if k.inst {
            continue
        }
        if let l = findSlotIn(k, name, replaced) {
            return l
        }
    }
    return nil
}

/// A slot of a component nested in a wrapper, with the path a reader writes it with.
private struct ForwardedSlot {
    let loc: Loc
    let path: String
}

/// Lists the slots named name that live in the components r uses, in file order: a wrapper passes a slot of a
/// component it uses through by its name (spec 4.7a). The slots of r itself are findSlot's.
private func forwardedSlots(_ r: Scene.RNode, _ name: String) -> [ForwardedSlot] {
    let replaced = collectSlots(r, name, "", false, true)
    return replaced.isEmpty ? collectSlots(r, name, "", false, false) : replaced
}

/// Walks the children of r; inInstance tells that they belong to an instance, whose slots a fill of the wrapper may reach.
private func collectSlots(
    _ r: Scene.RNode,
    _ name: String,
    _ prefix: String,
    _ inInstance: Bool,
    _ replaced: Bool
) -> [ForwardedSlot] {
    var out: [ForwardedSlot] = []
    for (i, k) in r.kids.enumerated() {
        let p = joinPath(prefix, nodeSegment(k))
        if k.kind == .slot {
            if inInstance && k.slot == name && k.replaced == replaced {
                out.append(ForwardedSlot(loc: Loc(parent: r, i: i), path: joinPath(p, slotSegment(k))))
            }
        } else if k.inst {
            out += collectSlots(k, name, p, true, replaced)
        } else {
            out += collectSlots(k, name, p, inInstance, replaced)
        }
    }
    return out
}

/// The path segment of a node: its name, else the key the format writes.
private func nodeSegment(_ n: Scene.RNode) -> String {
    n.name.isEmpty ? n.key : n.name
}

private func joinPath(_ prefix: String, _ seg: String) -> String {
    if prefix.isEmpty { return seg }
    if seg.isEmpty { return prefix }
    return prefix + "." + seg
}

private func slotNames(_ r: Scene.RNode) -> [String] {
    var out: [String] = []
    for k in r.kids {
        if k.inst {
            continue
        }
        if k.kind == .slot {
            out.append(k.slot.isEmpty ? "(default)" : k.slot)
        }
        out += slotNames(k)
    }
    return out
}

extension Scene.Expander {
    /// Pushes n on the stack of nodes being expanded; a node already there is a cycle.
    func enter(_ n: Scene.Node, _ pos: Scene.Pos) -> Bool {
        for (i, f) in stack.enumerated() where f.n === n {
            let chain = stack[i...].map(\.label) + [label(n)]
            t.fail(pos, "cycle: \(chain.joined(separator: " -> "))")
            return false
        }
        stack.append((n, label(n)))
        return true
    }

    func leave() { stack.removeLast() }

    func newR(_ kind: Scene.RKind, _ n: Scene.Node) -> Scene.RNode {
        Scene.RNode(kind: kind, name: n.name, key: n.key, pos: n.pos, props: n.props, vars: scope(), src: n)
    }

    /// Expands an element or a file: its content is the scene. env is the context of an instance, nil when the
    /// node's own context commands declare it.
    func container(_ n: Scene.Node, _ env: [String: String]?) -> Scene.RNode? {
        guard enter(n, n.pos) else { return nil }
        defer { leave() }
        let saved = ctx
        if n.mount != nil {
            ctx = saved?.push(n) ?? Scene.MountCtx(n, nil)
        }
        defer { ctx = saved }
        let savedEnv = self.env
        if let env {
            self.env = env
        } else {
            self.env = context(n, n.props, self.env)
        }
        defer { self.env = savedEnv }
        let r = newR(.item, n)
        r.kids = contents(n.content)
        return r
    }

    func contents(_ nodes: [Scene.Node]) -> [Scene.RNode] {
        nodes.compactMap { content($0) }
    }

    func content(_ n: Scene.Node) -> Scene.RNode? {
        switch n.kind {
        case .item:
            return item(n)
        case .use:
            return use(n)
        case .link:
            let r = newR(.link, n)
            r.name = ""
            r.key = ""
            r.from = n.from
            r.to = n.to
            return r
        case .slot:
            let r = newR(.slot, n)
            r.slot = n.key
            r.kids = contents(n.content)
            return r
        default:
            return nil
        }
    }

    /// Expands an item written in place: a component opened by itself is an instance that sets nothing, so its
    /// body sees the defaults of its parameters (spec 5c).
    func item(_ n: Scene.Node) -> Scene.RNode? {
        let caller = vars
        if !n.params.isEmpty {
            vars = defaults(n)
        }
        defer { vars = caller }
        return itemBody(n, nil)
    }

    /// Expands the content of an item in the scope vars already holds: the use of a component item has set the
    /// values there, and they must not fall back to the defaults. env is the context of an instance, nil for an
    /// item written in place: then its own context commands declare it.
    func itemBody(_ n: Scene.Node, _ env: [String: String]?) -> Scene.RNode? {
        guard enter(n, n.pos) else { return nil }
        defer { leave() }
        let saved = self.env
        if let env {
            self.env = env
        } else {
            self.env = context(n, n.props, self.env)
        }
        defer { self.env = saved }
        let r = newR(.item, n)
        r.kids = contents(n.content)
        return r
    }

    /// Expands an instance. As the target of another use its slots stay open for that use. env is the context
    /// of the use site (nil: the current one), overlay the block declarations of the uses whose target it is,
    /// weakest first (spec 5f).
    func use(
        _ n: Scene.Node, asTarget: Bool = false, env: [String: String]? = nil, overlay: [Scene.CtxDecl] = []
    ) -> Scene.RNode? {
        guard let parent = n.parent else { return nil }
        guard enter(n, n.pos) else { return nil }
        defer { leave() }
        depth += 1
        defer { depth -= 1 }
        let outer = ctx
        let (target, tctx) = t.resolve(parent, outer, n.path, n.pos)
        guard let target else { return nil }
        // The values of the parameters are the scope of the block of the use, a default the scope of the
        // declaration; without parameters of its own the target reads the scope it is written in.
        let caller = vars
        let callerEnv = env ?? self.env
        var scope = scopeOf(caller, callerEnv)
        let (values, used) = params(target, n, scope)
        if let values {
            scope = scopeOf(values, callerEnv)  // the block reads the parameters of the instance it makes
        }
        // The order of the context is: the place of the use, the definition root, the block of the use
        // (spec 5f). A use as target keeps its own block below the block that inserts it, so the nested use
        // applies the two itself.
        let block = ctxDecls(n, blockProps(n.block), scope)
        var inst: Scene.RNode?
        var instEnv: [String: String]?
        ctx = tctx  // the component's names resolve in its own file, the block's in this one
        if let values {
            vars = values
        }
        if target.kind == .use {
            inst = instance(target, n, callerEnv, block + overlay)
        } else {
            instEnv = applyCtx(callerEnv, ctxDecls(target, target.props, scope))
            instEnv = applyCtx(instEnv, block)
            instEnv = applyCtx(instEnv, overlay)
            instEnv = applyCtx(instEnv, fileDefaults(target, callerEnv))
            inst = instance(target, n, instEnv, [])
        }
        ctx = outer
        vars = caller
        guard let inst else { return nil }
        inst.def = target
        inst.name = n.name
        inst.key = n.key
        inst.pos = n.pos
        inst.inst = true
        inst.src = n
        inst.props = withoutProp(inst.props, "pos")  // the component's pos is its place on the library canvas
        if instEnv == nil {
            instEnv = ctxEnv(inst.vars)
        }
        let savedEnv = self.env  // the content of the slots and at reads the context of the instance it lands in
        self.env = instEnv
        apply(inst, n.block, used)
        self.env = savedEnv
        if !asTarget && depth == 1 {
            finalize(inst)
        }
        return inst
    }

    func instance(
        _ target: Scene.Node, _ use: Scene.Node, _ env: [String: String]?, _ overlay: [Scene.CtxDecl]
    ) -> Scene.RNode? {
        switch target.kind {
        case .item: return itemBody(target, env)
        case .link: return content(target)
        case .use: return self.use(target, asTarget: true, env: env, overlay: overlay)
        case .element, .file: return container(target, env)
        default:
            t.fail(use.pos, "path \(use.path) names a \(target.kind), not a node")
            return nil
        }
    }

    /// Puts the block of a use onto the instance root: properties override, nodes go to the default slot, at and
    /// replace reach into the instance, slot fills a named slot. used holds the overrides that are parameter values.
    func apply(_ root: Scene.RNode, _ block: [Scene.Override], _ used: Set<Int>) {
        var fills: [SlotFill] = []
        func add(_ name: String, _ pos: Scene.Pos, _ nodes: [Scene.RNode], _ explicit: Bool) {
            for i in fills.indices where fills[i].name == name {
                fills[i].nodes += nodes
                fills[i].explicit = fills[i].explicit || explicit
                return
            }
            fills.append(SlotFill(name: name, pos: pos, nodes: nodes, explicit: explicit))
        }
        for (i, ov) in block.enumerated() {
            switch ov.kind {
            case .prop:
                if used.contains(i) {
                    continue
                }
                guard let p = ov.prop else { continue }
                // After the definition's properties: what the use site writes wins over the component's styles (spec 5b).
                root.props.append(p)
            case .node:
                guard let n = ov.node else { continue }
                add("", ov.pos, contents([n]), false)
            case .slot:
                add(normalizeName(ov.slot), ov.pos, nodes(ov.block, slots: false), true)
            case .at:
                guard let at = find(root, ov.path, ov.pos) else { continue }
                apply(at.node, ov.block, [])
            case .replace:
                guard let at = find(root, ov.path, ov.pos) else { continue }
                let replacement = nodes(ov.block, slots: true)
                markReplaced(replacement)
                at.parent.kids = splice(at.parent.kids, at.i, replacement)
            }
        }
        for f in fills {
            fillSlot(root, f)
        }
    }

    /// Expands the block of a slot or replace, which takes nodes only; slots too in a replace block, because a
    /// slot the replacement declares is filled from the outside as any other (spec 4.7a).
    func nodes(_ block: [Scene.Override], slots: Bool) -> [Scene.RNode] {
        var out: [Scene.RNode] = []
        for ov in block {
            if ov.kind == .node {
                if let n = ov.node, let r = content(n) {
                    out.append(r)
                }
            } else if ov.kind == .slot && slots {
                let r = Scene.RNode(kind: .slot, name: ov.slot, pos: ov.pos)
                r.slot = normalizeName(ov.slot)
                r.kids = nodes(ov.block, slots: false)
                out.append(r)
            } else {
                t.fail(ov.pos, "only item, link and use are allowed here")
            }
        }
        return out
    }

    /// Follows a path of names from the instance root: items, instances and slots, no links. A slot segment is
    /// the slot name ("default" for the unnamed one) and leads into its fallback.
    fileprivate func find(_ root: Scene.RNode, _ path: String, _ pos: Scene.Pos) -> Loc? {
        guard let segs = splitPath(path) else {
            t.fail(pos, "bad path \"\(path)\"")
            return nil
        }
        var nodes = [root]
        var locs: [Loc] = []
        for seg in segs {
            locs = []
            for p in nodes {
                for (i, k) in p.kids.enumerated() where matchesSeg(k, seg) {
                    locs.append(Loc(parent: p, i: i))
                }
            }
            if locs.isEmpty {
                t.fail(pos, "path \(path): nothing named \"\(seg.name)\" inside the instance")
                return nil
            }
            nodes = locs.map { l in
                let n = l.node
                if n.kind == .slot {
                    n.touched = true
                }
                return n
            }
        }
        if nodes.count > 1 {
            let parts = nodes.map { quoteName(rName($0)) + " (\($0.pos))" }
            t.fail(pos, "ambiguous path \(path): candidates \(parts.joined(separator: ", "))")
            return nil
        }
        if nodes[0].kind == .slot {
            t.fail(pos, "path \(path) names a slot, not a node; fill it with slot \(segs[segs.count - 1].text) { ... }")
            return nil
        }
        return locs[0]
    }

    fileprivate func fillSlot(_ root: Scene.RNode, _ f: SlotFill) {
        // Nodes of the block that failed leave no fill; only slot {} says the fallback goes.
        if f.name.isEmpty && f.nodes.isEmpty && !f.explicit {
            return
        }
        var at = findSlot(root, f.name)
        if at == nil && !f.name.isEmpty {
            // A wrapper passes a slot of a component it uses through by its name (spec 4.7a).
            let hits = forwardedSlots(root, f.name)
            switch hits.count {
            case 1:
                at = hits[0].loc
            case 0:
                break
            default:
                let paths = hits.map(\.path).joined(separator: ", ")
                t.fail(
                    f.pos,
                    "slot \"\(f.name)\" is in \(hits.count) nested components: \(paths); give the wrapper a slot of its own")
                return
            }
        }
        if let at {
            if at.node.touched {
                t.warn(f.pos, "slot \(slotSegment(at.node)) is filled here, the at/replace changes to its fallback are dropped")
            }
            at.parent.kids = splice(at.parent.kids, at.i, f.nodes)
            return
        }
        var have = slotNames(root).joined(separator: ", ")
        if have.isEmpty {
            have = "none"
        }
        if f.name.isEmpty {
            t.fail(f.pos, "nodes passed, but the component has no default slot (slots: \(have))")
            return
        }
        t.fail(f.pos, "unknown slot \"\(f.name)\" (slots: \(have))")
    }
}

extension Scene.Tree {
    /// Expands the element at path ("" is the content of the file itself) into resolved nodes, slots filled.
    func element(_ path: String, eval: Scene.EvalFunc? = nil) -> Scene.RNode? {
        guard let main else { return nil }
        var target = main
        var ctx = ctxOf(main)
        if !path.isEmpty {
            let found = resolve(main, ctx, path, main.pos)
            guard let node = found.node else { return nil }
            target = node
            ctx = found.ctx
        }
        guard target.isContainer() else {
            guard target.kind == .item else {
                fail(main.pos, "path \(path) names a \(target.kind), not an element or an item")
                return nil
            }
            return Scene.Expander(t: self, ctx: ctx, eval: eval).item(target)
        }
        return expandElement(target, ctx, eval: eval)
    }

    /// Expands a container in the context of its file, slots filled; a component on its own shows its defaults.
    func expandElement(_ target: Scene.Node, _ ctx: Scene.MountCtx?, eval: Scene.EvalFunc? = nil) -> Scene.RNode? {
        let x = Scene.Expander(t: self, ctx: ctx, eval: eval)
        // The root of the file is a node above every element in it, so its context is inherited (spec 5f).
        let file = fileRoot(target)
        if file !== target {
            x.env = x.context(file, file.props, nil)
        }
        x.vars = x.defaults(target)
        let rn = x.container(target, nil)
        if let rn {
            finalize(rn)
        }
        return rn
    }
}
