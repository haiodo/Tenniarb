//
//  PenOverrides.swift
//  Tenniarb
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//
//  See the License for the specific language governing permissions and
//  limitations under the License.
//

import Foundation

/// Mirrors the expansion go/scene makes of use: the tree of an instance with its overrides and slots, so
/// that an override addressed by a pen id can be turned into a path of names.
final class PenENode {
    var o: PenNode
    var kids: [PenENode] = []
    weak var parent: PenENode?
    var open = false  // a slot nobody has filled yet

    init(o: PenNode, open: Bool = false) {
        self.o = o
        self.open = open
    }

    func add(_ k: PenENode) {
        k.parent = self
        kids.append(k)
    }

    /// The expansion of an instance. Its slots stay open, like in go/scene, where they are replaced by
    /// their fallback only when the outermost use is done: overrides reach into them.
    static func use(_ u: PenNode) -> PenENode? {
        guard let t = u.target else { return nil }
        let inst: PenENode?
        if t.kind == .use {
            inst = use(t)
        } else {
            inst = content(t)
        }
        guard let inst else { return nil }
        inst.o = u
        apply(inst, u.block)
        return inst
    }

    static func content(_ o: PenNode) -> PenENode? {
        switch o.kind {
        case .item, .slot:
            let e = PenENode(o: o, open: o.kind == .slot)
            for k in o.kids {
                if let ke = content(k) { e.add(ke) }
            }
            return e
        case .use:
            return use(o)
        case .file:
            return nil
        }
    }

    static func apply(_ root: PenENode, _ block: [PenOvr]) {
        var fills: [PenOvr] = []
        for ov in block {
            switch ov.kind {
            case .replace:
                if let (parent, i) = findChain(root, ov.chain) {
                    parent.splice(i, nodes(ov.nodes))
                }
            case .slot:
                fills.append(ov)
            case .at:
                break
            }
        }
        for ov in fills {
            if let slot = ov.slot, let (parent, i) = findSlot(root, slot) {
                parent.splice(i, nodes(ov.nodes))
            }
        }
    }

    static func nodes(_ list: [PenNode]) -> [PenENode] {
        list.compactMap { content($0) }
    }

    func splice(_ i: Int, _ with: [PenENode]) {
        var out = Array(kids[0..<i])
        for w in with {
            w.parent = self
            out.append(w)
        }
        out += kids[(i + 1)...]
        kids = out
    }

    static func findSlot(_ e: PenENode, _ slot: PenNode) -> (PenENode, Int)? {
        for (i, k) in e.kids.enumerated() {
            if k.open && k.o === slot { return (e, i) }
            if let found = findSlot(k, slot) { return found }
        }
        return nil
    }

    /// Follows origin nodes down from the instance root; returns the parent of the target and the index
    /// of the target.
    static func findChain(_ root: PenENode, _ chain: [PenNode]) -> (PenENode, Int)? {
        var cur = root
        var found: (PenENode, Int)?
        for n in chain {
            found = nil
            for (i, k) in cur.kids.enumerated() where k.o === n {
                found = (cur, i)
                break
            }
            guard let (parent, i) = found else { return nil }
            cur = parent.kids[i]
        }
        return found
    }

    /// Lists the nodes below scope that come from the pen node (f, id).
    static func locate(_ scope: PenENode, _ f: PenFile, _ id: String) -> [PenENode] {
        var out: [PenENode] = []
        func walk(_ e: PenENode) {
            for k in e.kids {
                if k.o.res === f && k.o.id == id && k.o.kind != .slot {
                    out.append(k)
                }
                walk(k)
            }
        }
        walk(scope)
        return out
    }

    /// The origin nodes from just below root down to e.
    static func chainOf(_ e: PenENode?, _ root: PenENode) -> [PenNode] {
        var rev: [PenNode] = []
        var e = e
        while let node = e, node !== root {
            rev.append(node.o)
            e = node.parent
        }
        return rev.reversed()
    }
}

/// What the descendants of one instance say, per expanded node.
final class PenOvAcc {
    var props: [ObjectIdentifier: PenMap] = [:]
    var repl: [ObjectIdentifier: PenMap] = [:]
}

extension PenImport {
    static let rootSkip: Set<String> = [
        "id", "type", "ref", "name", "descendants", "children", "x", "y", "reusable", "theme", "enabled",
        "layoutPosition",
    ]

    /// The pen keys that become properties on their own, whatever the other keys are. A property an
    /// override would only repeat from the component can be left out.
    static let sameKeys: Set<String> = [
        "width", "height", "gap", "padding", "cornerRadius", "clip", "opacity", "content", "fontSize",
        "lineHeight", "fontFamily", "fontWeight", "fontStyle", "letterSpacing", "textAlign", "alignItems",
        "justifyContent", "layout", "fill", "stroke", "strokeWidth",
    ]

    static func mergeMaps(_ lo: PenMap, _ hi: PenMap) -> PenMap {
        var out = PenMap(minimumCapacity: lo.count + hi.count)
        for (k, v) in lo { out[k] = v }
        for (k, v) in hi { out[k] = v }
        return out
    }

    static func without(_ m: PenMap, _ skip: Set<String>) -> PenMap {
        var out = PenMap(minimumCapacity: m.count)
        for (k, v) in m where !skip.contains(k) { out[k] = v }
        return out
    }

    /// The value names a variable, which may mean another color in the theme of the instance.
    static func hasVariable(_ v: Any?) -> Bool {
        switch v {
        case let s as String:
            return s.hasPrefix("$")
        case let list as [Any]:
            return list.contains { hasVariable($0) }
        case let m as PenMap:
            return m.values.contains { hasVariable($0) }
        default:
            return false
        }
    }

    /// over without the keys that hold the same value in base. A stroke width alone would turn into a
    /// stroke of width 1 where the stroke itself stays, so it goes only with the stroke.
    static func withoutSame(_ over: PenMap, _ base: PenMap) -> PenMap {
        var out = PenMap(minimumCapacity: over.count)
        for (k, v) in over {
            let bv = base[k]
            if sameKeys.contains(k), let bv, !hasVariable(v), penEqual(v, bv) { continue }
            out[k] = v
        }
        if out["stroke"] != nil, let sw = over["strokeWidth"] {
            out["strokeWidth"] = sw
        }
        return out
    }
}

extension PenFile {
    /// What a ref to id found instead of a component: a type this converter does not build, or nothing
    /// at all.
    func whyMissing(_ id: String) -> String {
        if let typ = unsupportedById[id] {
            return "the component is a \"\(typ)\", a type this converter does not build"
        }
        return "the component is not there"
    }

    /// The room the component with that id asks for in the .pen, and the size an icon takes when the file
    /// leaves it to the icon: twenty points, the default of the format.
    func componentSize(_ id: String) -> (w: Double, h: Double) {
        var w = 20.0
        var h = 20.0
        if let m = rawIdx[id] {
            if let v = m["width"] as? Double, v > 0 { w = v }
            if let v = m["height"] as? Double, v > 0 { h = v }
        }
        return (w, h)
    }
}

extension PenConvert {
    /// The pen map of a node with the root overrides of the instances on the way applied.
    func effPen(_ n: PenNode) -> PenMap {
        guard n.kind == .use, let t = n.target else { return n.pen }
        return PenImport.mergeMaps(effPen(t), PenImport.without(n.pen, PenImport.rootSkip))
    }

    /// Resolves every use below n.
    func finishTree(_ n: PenNode) {
        if n.kind == .use {
            finish(n)
            return
        }
        for k in n.kids {
            finishTree(k)
        }
    }

    /// Resolves the uses of a file; nodes made inside blocks are resolved when their block is.
    func finishAll(_ f: PenFile) {
        for n in f.root.kids {
            finishTree(n)
        }
    }

    func placeholder(_ u: PenNode, _ why: String) {
        let f = u.file
        f.unsup("unresolved ref")
        f.placeholders.append(u.ref)
        // The placeholder takes the room of the component it stands for: an icon of twenty points leaves
        // an empty box of twenty points, not a point, so the layout of the frame around it stays as it
        // was drawn.
        let (w, h) = (u.res ?? u.file).componentSize(u.ref)
        f.warn(
            String(
                format: "%@: %@ (ref %@), a %gx%g placeholder is written", u.name, why, u.ref, w, h))
        u.kind = .item
        u.target = nil
        u.block = []
        u.name = "missing: " + u.name
        u.missing = true
        u.type = "frame"
        // absolute keeps the placeholder out of the gaps of a flow, as the node that gfx/pen leaves out
        u.props =
            [
                PenImport.prop("width", PenImport.numArg(w)), PenImport.prop("height", PenImport.numArg(h)),
                PenImport.prop("absolute", PenImport.wordArg("yes")),
            ] + PenImport.posProps(u.pen, u.b)
    }

    func finish(_ u: PenNode) {
        switch u.state {
        case 2:
            return
        case 1:
            placeholder(u, "instance of itself")
            u.state = 2
            return
        default:
            break
        }
        u.state = 1
        defer { u.state = 2 }
        let res = u.res ?? u.file
        guard let t = res.resolve(u.ref) else {
            placeholder(u, res.whyMissing(u.ref))
            return
        }
        u.target = t
        finishTree(t)
        if u.kind != .use || t.missing {
            if t.missing { placeholder(u, "the component is a placeholder") }
            return
        }
        var rest = PenImport.without(u.pen, PenImport.rootSkip)
        if t.kind == .item {
            rest = PenImport.withoutSame(rest, t.pen)
        }
        u.props =
            PenImport.hiddenProp(u.pen, false)
            + props(
                res, rest, u.type,
                PenPctx(mode: u.b.mode, explicit: true, eff: effPen(u)))
        u.props += PenImport.posProps(u.pen, u.b)
        // the theme of an instance is a context declaration in its block
        u.props += PenImport.themeProps(u.pen)
        buildBlock(u)
    }

    func buildBlock(_ u: PenNode) {
        guard let base = PenENode.use(u) else { return }
        let acc = PenOvAcc()
        if let kids = u.pen["children"] as? [Any] {
            acc.props[ObjectIdentifier(base)] = ["children": kids]
        }
        let desc = u.pen["descendants"] as? PenMap
        collect(u, base, desc, acc)
        u.block = emitOps(u, base, acc)
    }

    func isReplacement(_ o: PenNode, _ val: PenMap) -> Bool { replaces(o.pen, o.id, val) }

    /// Tells whether an override replaces the node pen instead of changing its properties.
    func replaces(_ pen: PenMap, _ id: String, _ val: PenMap) -> Bool {
        guard let t = val["type"] else { return false }
        let oid = val["id"] as? String ?? ""
        return !PenImport.penEqual(t, pen["type"]) || !PenImport.penEqual(val["ref"], pen["ref"])
            || !oid.isEmpty && oid != id
    }

    /// Resolves a key of a descendants map written in f to the file and the ids of its path: pen names a
    /// node of the instance by its id, and one inside a nested instance by the instance's id and the id of
    /// the node, "outer/inner", because a bare id would not say which instance it belongs to.
    func keyTarget(_ f: PenFile?, _ key: String) -> (PenFile?, [String]) {
        let (alias, rest) = PenImport.splitRef(key)
        var g = f
        if !alias.isEmpty { g = f?.imports[alias] }
        return (g, rest.components(separatedBy: "/"))
    }

    /// Reads a descendants map of u. The nested maps of an instance come first, because the outer ones win
    /// where both name a node.
    func collect(_ u: PenNode, _ scope: PenENode, _ desc: PenMap?, _ acc: PenOvAcc) {
        let keys = PenImport.sortedKeys(desc ?? [:])
        for pass in 0..<2 {
            for key in keys {
                guard let val = desc?[key] as? PenMap else { continue }
                let ms = matches(u, scope, key)
                for e in ms {
                    collectOne(u, e, val, pass, acc)
                }
                if ms.isEmpty && pass == 0 {
                    missingTarget(u, key, val)
                }
            }
        }
    }

    func collectOne(_ u: PenNode, _ e: PenENode, _ val: PenMap, _ pass: Int, _ acc: PenOvAcc) {
        if isReplacement(e.o, val) {
            if pass == 1 { acc.repl[ObjectIdentifier(e)] = val }
            return
        }
        if let nd = val["descendants"] as? PenMap, pass == 0 {
            collect(u, e, nd, acc)
        }
        if pass == 1 {
            let skip: Set<String> = ["descendants", "id", "type", "ref"]
            acc.props[ObjectIdentifier(e)] = PenImport.mergeMaps(
                acc.props[ObjectIdentifier(e)] ?? [:], PenImport.without(val, skip))
        }
    }

    func matches(_ u: PenNode, _ scope: PenENode, _ key: String) -> [PenENode] {
        let (g, ids) = keyTarget(u.res, key)
        guard let g else { return [] }
        return locatePath(scope, g, ids)
    }

    /// Follows the ids of a descendants key from scope down: the first one is found anywhere in the
    /// instance, every next one inside the subtree of the one before it.
    func locatePath(_ scope: PenENode, _ f: PenFile, _ ids: [String]) -> [PenENode] {
        guard let first = ids.first else { return [] }
        var found = PenENode.locate(scope, f, first)
        for id in ids.dropFirst() {
            var next: [PenENode] = []
            for e in found {
                next += PenENode.locate(e, f, id)
            }
            found = next
        }
        return found
    }

    func missingTarget(_ u: PenNode, _ key: String, _ val: PenMap) {
        let (g, ids) = keyTarget(u.res, key)
        guard let g else {
            u.file.unsup("override of an id that is not in the file or library")
            return
        }
        for id in ids {
            if g.dropped.contains(id) {
                if (val["enabled"] as? Bool) == true {
                    u.file.unsup("enabling a disabled node inside a nested instance (not reachable)")
                }
                return
            }
            if g.rawIdx[id] == nil {
                u.file.unsup("override of an id that is not in the file or library")
                return
            }
        }
        u.file.unsup("override target not found in the instance")
    }

    /// Turns the collected overrides into the commands of the block, in tree order.
    func emitOps(_ u: PenNode, _ base: PenENode, _ acc: PenOvAcc) -> [PenOvr] {
        var ops: [PenOvr] = []
        func visit(_ e: PenENode) {
            if let p = acc.props[ObjectIdentifier(e)] {
                ops += propOps(u, base, e, p)
            }
            for k in e.kids {
                if let val = acc.repl[ObjectIdentifier(k)] {
                    ops.append(replaceOp(u, base, k, val))
                    continue
                }
                // go/scene drops the changes to the fallback of a slot that the block fills
                if k.open && slotFilled(ops, k.o) {
                    continue
                }
                visit(k)
            }
        }
        visit(base)
        return ops
    }

    func slotFilled(_ ops: [PenOvr], _ slot: PenNode) -> Bool {
        ops.contains { $0.kind == .slot && $0.slot === slot }
    }

    func replaceOp(_ u: PenNode, _ base: PenENode, _ e: PenENode, _ val: PenMap) -> PenOvr {
        var m = val
        for k in ["x", "y"] where m[k] == nil {
            if let v = e.o.pen[k] {
                m = PenImport.mergeMaps(m, [k: v])
            }
        }
        let b = PenBCtx(
            mode: mode, parentAbs: parentAbs(e), noIndex: true, layout: layoutParent(e), res: u.res)
        let op = PenOvr(kind: .replace, chain: PenENode.chainOf(e, base))
        if let n = buildNode(u.file, m, u, b) {
            finishTree(n)
            u.content.append(n)
            op.nodes = [n]
        }
        return op
    }

    /// Tells whether the node e sits in a frame, slots not counted.
    func layoutParent(_ e: PenENode) -> Bool {
        var p = e.parent
        while let node = p, node.o.kind == .slot {
            p = node.parent
        }
        return p?.o.type == "frame"
    }

    func parentAbs(_ e: PenENode) -> Bool {
        guard let p = e.parent else { return false }
        return (effPen(p.o)["layout"] as? String) == "none"
    }

    func propOps(_ u: PenNode, _ base: PenENode, _ e: PenENode, _ p: PenMap) -> [PenOvr] {
        var ops: [PenOvr] = []
        if let kids = p["children"] as? [Any] {
            ops += fillOps(u, base, e, kids)
        }
        let skip: Set<String> = ["children", "enabled", "x", "y", "layoutPosition"]
        var rest = PenImport.without(p, skip)
        let eff = PenImport.mergeMaps(effPen(e.o), p)
        var typ = e.o.type
        if typ.isEmpty { typ = "frame" }
        // a node inside a nested instance may have been changed by that instance: its base is not known here
        if e !== base && e.o.kind == .item && !insideUse(e, base) {
            rest = PenImport.withoutSame(rest, e.o.pen)
        }
        var props =
            PenImport.hiddenProp(p, true)
            + self.props(u.res ?? u.file, rest, typ, PenPctx(mode: mode, explicit: true, eff: eff))
        let hx = p["x"] != nil
        let hy = p["y"] != nil
        if hx || hy {
            let x = eff["x"] as? Double ?? 0
            let y = eff["y"] as? Double ?? 0
            props.append(PenImport.prop("pos", PenImport.numArg(x), PenImport.numArg(-y)))
        }
        if !props.isEmpty && e !== base {
            ops.append(PenOvr(kind: .at, chain: PenENode.chainOf(e, base), props: props))
        } else if !props.isEmpty {
            u.props = setProps(u.props, props)
        }
        return ops
    }

    /// Replaces the properties of the same name, or appends.
    func setProps(_ old: [PenProp], _ with: [PenProp]) -> [PenProp] {
        var out = old
        for w in with {
            var done = false
            for i in out.indices where out[i].name == w.name {
                out[i] = w
                done = true
            }
            if !done { out.append(w) }
        }
        return out
    }

    /// The item an instance chain ends in.
    func rootItem(_ n: PenNode?) -> PenNode? {
        var n = n
        while let node = n, node.kind == .use {
            n = node.target
        }
        return n
    }

    /// Puts new children into e: a slot when it has an open one, else in the place of the children it has.
    /// A frame with none gets a slot for them.
    func fillOps(_ u: PenNode, _ base: PenENode, _ e: PenENode, _ kids: [Any]) -> [PenOvr] {
        let b = PenBCtx(
            mode: mode, parentAbs: (effPen(e.o)["layout"] as? String) == "none", noIndex: true,
            layout: e.o.type == "frame", res: u.res)
        var nodes: [PenNode] = []
        for k in kids {
            guard let km = k as? PenMap else { continue }
            if let n = buildNode(u.file, km, u, b) {
                finishTree(n)
                u.content.append(n)
                nodes.append(n)
            }
        }
        for k in e.kids where k.open {
            return [slotFill(base, k, nodes)]
        }
        if e.kids.isEmpty {
            return slotFor(u, base, e, nodes)
        }
        if insideUse(e, base) {
            u.file.unsup("children replace the inlined content of a nested instance")
        }
        var ops: [PenOvr] = []
        for (i, k) in e.kids.enumerated() {
            let op = PenOvr(kind: .replace, chain: PenENode.chainOf(k, base))
            if i == 0 { op.nodes = nodes }
            ops.append(op)
        }
        return ops
    }

    /// Fills a slot. go/scene fills the slots of a nested instance only from inside it, so the fill goes
    /// into an at of that instance.
    func slotFill(_ base: PenENode, _ slot: PenENode, _ nodes: [PenNode]) -> PenOvr {
        var owner = slot.parent
        while let node = owner, node !== base, node.o.kind != .use {
            owner = node.parent
        }
        let op = PenOvr(kind: .slot, nodes: nodes, slot: slot.o)
        if let owner, owner !== base {
            op.chain = PenENode.chainOf(owner, base)
        }
        return op
    }

    /// Adds a slot to the component frame that e comes from and fills it.
    func slotFor(_ u: PenNode, _ base: PenENode, _ e: PenENode, _ nodes: [PenNode]) -> [PenOvr] {
        var item = e.o
        if e === base, let root = rootItem(u.target) {
            item = root
        }
        if nodes.isEmpty { return [] }
        if item.kind != .item || insideUse(e, base) {
            u.file.unsup("children given to an empty frame of a nested instance")
            return []
        }
        let s = PenNode(kind: .slot, file: item.file)
        s.res = item.res
        s.parent = item
        s.slot = slotName(item.file, item.name)
        item.kids.append(s)
        let slot = PenENode(o: s, open: true)
        e.add(slot)
        return [slotFill(base, slot, nodes)]
    }

    func insideUse(_ e: PenENode?, _ base: PenENode) -> Bool {
        var p = e
        while let node = p, node !== base {
            if node.o.kind == .use { return true }
            p = node.parent
        }
        return false
    }
}
