//
//  SceneResolve.swift
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
    /// A path a lookup cannot follow; the message is the one go/scene/Resolve gives.
    struct ResolveError: Error, CustomStringConvertible {
        let message: String

        var description: String { message }
    }

    /// The chain of mount points a file is reached through: at is the element with --from-file in the mounting
    /// file, up the context of that file. A file mounted in several places has the first chain.
    final class MountCtx {
        let at: Node
        let up: MountCtx?

        init(_ at: Node, _ up: MountCtx?) {
            self.at = at
            self.up = up
        }

        func push(_ at: Node) -> MountCtx { MountCtx(at, self) }
    }
}

/// A node a path names so far, with the context of the file it lives in.
private struct Hit {
    let n: Scene.Node
    let c: Scene.MountCtx?
}

private func pick(_ nodes: [Scene.Node], _ seg: Scene.Segment) -> [Scene.Node] {
    nodes.filter { seg.matches($0) }
}

/// Lists nodes as the quoted names a path writes, each with its position.
private func whereText(_ nodes: [Scene.Node]) -> String {
    nodes.map { quoteName($0.name) + " (\($0.pos))" }.joined(separator: ", ")
}

/// The scope after a: its parent, or from the root of a mounted file the place it is mounted at (spec 5a.1).
private func outward(_ a: Scene.Node, _ c: Scene.MountCtx?) -> (Scene.Node?, Scene.MountCtx?) {
    if let p = a.parent {
        return (p, c)
    }
    guard let c else { return (nil, nil) }
    return (c.at.parent, c.up)
}

/// The node and the context of its file, or nil and the message of the problem.
private func lookup(
    _ scope: Scene.Node,
    _ ctx: Scene.MountCtx?,
    _ path: String
) -> (node: Scene.Node?, ctx: Scene.MountCtx?, message: String) {
    guard let segs = splitPath(path) else {
        return (nil, nil, "bad path \"\(path)\"")
    }
    var a: Scene.Node? = scope
    var c = ctx
    while let cur = a {
        var found = pick(cur.named(), segs[0]).map { Hit(n: $0, c: c) }
        if !found.isEmpty {
            for seg in segs.dropFirst() {
                var next: [Hit] = []
                for h in found {
                    var cc = h.c
                    if h.n.mount != nil {
                        cc = cc?.push(h.n) ?? Scene.MountCtx(h.n, nil)
                    }
                    next += pick(h.n.named(), seg).map { Hit(n: $0, c: cc) }
                }
                if next.isEmpty {
                    return (nil, nil, "path \(path): nothing named \"\(seg.name)\" under \(whereText(found.map(\.n)))")
                }
                found = next
            }
            if found.count > 1 {
                return (nil, nil, "ambiguous path \(path): candidates \(whereText(found.map(\.n)))")
            }
            return (found[0].n, found[0].c, "")
        }
        (a, c) = outward(cur, c)
    }
    return (nil, nil, "path \(path): \"\(segs[0].name)\" is not defined here or above")
}

extension Scene.Tree {
    /// Records, for each file below the entry, the chain of mounts that reaches it first.
    func mapMounts() {
        ctxs = [:]
        guard let top else { return }
        walkMounts(top, nil)
    }

    private func walkMounts(_ n: Scene.Node, _ c: Scene.MountCtx?) {
        for e in n.elems {
            guard let m = e.mount else {
                walkMounts(e, c)
                continue
            }
            guard ctxs[ObjectIdentifier(m)] == nil else { continue }
            let ctx = Scene.MountCtx(e, c)
            ctxs[ObjectIdentifier(m)] = ctx
            walkMounts(m, ctx)
        }
    }

    /// The mount context of the file n lives in; a file mounted in several places has the first.
    func ctxOf(_ n: Scene.Node) -> Scene.MountCtx? {
        var n = n
        while let p = n.parent { n = p }
        return ctxs[ObjectIdentifier(n)]
    }

    /// Finds the node a path names from scope (spec 4.2/4.3); nothing is added to the diagnostics.
    func resolve(_ scope: Scene.Node, _ path: String) throws -> Scene.Node {
        let (node, _, message) = lookup(scope, ctxOf(scope), path)
        if let node, message.isEmpty {
            return node
        }
        throw Scene.ResolveError(message: message)
    }

    /// resolve that reports a failure at pos; the context of the found node comes back.
    func resolve(
        _ scope: Scene.Node,
        _ ctx: Scene.MountCtx?,
        _ path: String,
        _ pos: Scene.Pos
    ) -> (node: Scene.Node?, ctx: Scene.MountCtx?) {
        let (node, c, message) = lookup(scope, ctx, path)
        if !message.isEmpty {
            fail(pos, message)
        }
        return (node, c)
    }
}
