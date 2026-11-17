//
//  PenNames.swift
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

extension PenImport {
    static func key(_ n: PenNode) -> String { normalize(n.name) }

    /// A node as a segment of an at/replace path: slots are segments too.
    static func pathSeg(_ n: PenNode) -> String? {
        if n.kind == .slot { return n.slot }
        return n.isNamed ? key(n) : nil
    }

    /// Visits every node of the file, the ones inside blocks included.
    static func eachNode(_ n: PenNode, _ fn: (PenNode) -> Void) {
        fn(n)
        for k in n.kids { eachNode(k, fn) }
        for k in n.content { eachNode(k, fn) }
    }

    /// Flags the nodes some path goes through: those must have names that tell siblings apart.
    static func markNeeded(_ f: PenFile) {
        eachNode(f.root) { n in
            if n.parent === f.root { n.needed = true }
            guard n.kind == .use, let target = n.target else { return }
            var p: PenNode? = target
            while let node = p, node.kind != .file {
                node.needed = true
                p = node.parent
            }
            for ov in n.block {
                for c in ov.chain { c.needed = true }
            }
        }
    }

    static func digitLead(_ s: String) -> Bool {
        guard let c = s.first else { return false }
        return isDigit(c)
    }

    /// Renames the later of the siblings with one key where a path needs to tell them apart.
    static func dedupe(_ list: [PenNode]) {
        var need: [String: Bool] = [:]
        var all: [String: Bool] = [:]
        for n in list where n.isNamed {
            if n.needed && digitLead(key(n)) {
                n.name = "_" + n.name
            }
            need[key(n)] = (need[key(n)] ?? false) || n.needed
            all[key(n)] = true
        }
        var seen: Set<String> = []
        for n in list where n.isNamed {
            let k = key(n)
            if seen.contains(k) && (need[k] ?? false) {
                var i = 2
                while true {
                    let cand = n.name + " " + String(i)
                    if all[normalize(cand)] == nil {
                        n.name = cand
                        all[normalize(cand)] = true
                        break
                    }
                    i += 1
                }
            }
            seen.insert(k)
        }
    }

    static func dedupeAll(_ f: PenFile) {
        dedupe(f.root.kids)
        eachNode(f.root) { n in
            dedupe(n.kids)
            dedupe(n.content)
        }
    }

    /// The path a use in file `from` writes to name t.
    static func pathTo(_ from: PenFile, _ t: PenNode) -> String {
        var segs: [String] = []
        var p: PenNode? = t
        while let node = p, node.kind != .file {
            if node.isNamed { segs.append(key(node)) }
            p = node.parent
        }
        segs.reverse()
        if t.file !== from {
            segs.insert(from.mountAlias(t.file), at: 0)
        }
        return segs.joined(separator: ".")
    }

    static func named(_ a: PenNode, _ seg: String) -> [PenNode] {
        var out: [PenNode] = []
        func pick(_ list: [PenNode]) {
            for k in list where k.isNamed && key(k) == seg {
                out.append(k)
            }
        }
        switch a.kind {
        case .file:
            pick(a.kids)
            for alias in sortedKeys(a.file.imports) {
                if let lib = a.file.imports[alias], a.file.mountAlias(lib) == seg {
                    out.append(lib.root)
                }
            }
        case .item, .slot:
            pick(a.kids)
        case .use:
            pick(a.content)
        }
        return out
    }

    /// Follows go/scene's rules: the first segment is looked up from the scope of the use upwards, the
    /// rest below what it found.
    static func resolveSim(_ u: PenNode, _ path: String) -> [PenNode] {
        let segs = path.components(separatedBy: ".")
        var a = u.parent
        while let scope = a {
            var cur = named(scope, segs[0])
            if !cur.isEmpty {
                for s in segs.dropFirst() {
                    var next: [PenNode] = []
                    for n in cur {
                        next += named(n, s)
                    }
                    cur = next
                }
                return cur
            }
            a = scope.parent
        }
        return []
    }

    /// Lists the uses whose path does not reach their target because a node nearer to the use has the
    /// name of the first segment.
    static func shadowed(_ files: [PenFile]) -> [PenNode] {
        var bad: [PenNode] = []
        for f in files {
            eachNode(f.root) { n in
                guard n.kind == .use, let target = n.target else { return }
                let got = resolveSim(n, pathTo(f, target))
                if got.count != 1 || got[0] !== target {
                    bad.append(n)
                }
            }
        }
        return bad
    }

    /// Renames the first segment of the path of u's target.
    static func bump(_ u: PenNode) {
        guard let t = u.target else { return }
        if t.file !== u.file {
            t.file.alias += "_lib"
            return
        }
        var p: PenNode? = t
        while let node = p {
            if let parent = node.parent, parent.kind == .file {
                node.name += " top"
                dedupe(parent.kids)
                return
            }
            p = node.parent
        }
    }
}

extension PenConvert {
    /// Makes the names unique where a path needs it and checks the paths.
    func finalizeNames() {
        for f in order {
            PenImport.markNeeded(f)
        }
        for f in order {
            PenImport.dedupeAll(f)
        }
        for _ in 0..<8 {
            let bad = PenImport.shadowed(order)
            if bad.isEmpty { return }
            for u in bad {
                PenImport.bump(u)
            }
        }
        for u in PenImport.shadowed(order) {
            if let target = u.target {
                u.file.warn("path \(PenImport.pathTo(u.file, target)) does not reach its target")
            }
        }
    }
}
