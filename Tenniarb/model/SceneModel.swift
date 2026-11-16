//
//  SceneModel.swift
//  Tenniarb
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//  WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.  See the
//  License for the specific language governing permissions and limitations under
//  the License.
//

import Tenn

/// Which renderers a document needs: go/model/scenekind.go, the rule of spec 5a.17.
enum DocKind {
    case diagram  // only classic elements
    case scene  // scene elements and mounted files, no classic items
    case mixed
}

/// Reports whether e is drawn by go/scene: its own content uses a scene feature.
func sceneElement(_ e: Element) -> Bool {
    for p in e.properties {
        switch commandOf(p) {
        case "use":
            if useWithPath(p) { return true }
        case "item":
            // The classic model keeps the items at the root of a file as properties.
            if let b = lastBlock(p), sceneBlock(b.children ?? []) { return true }
        default:
            break
        }
    }
    for it in e.items where it.kind != .Link && sceneBlock(Array(it.properties)) {
        return true
    }
    return false
}

/// The commands of an item that give it child nodes or auto-layout.
private func sceneBlock(_ cmds: [TennNode]) -> Bool {
    for p in cmds {
        switch commandOf(p) {
        case "item", "slot", "auto-layout":
            return true
        case "use":
            if useWithPath(p) { return true }
        default:
            break
        }
    }
    return false
}

private func commandOf(_ n: TennNode) -> String {
    n.kind == .Command ? (n.getIdent(0) ?? "") : ""
}

private func lastBlock(_ c: TennNode) -> TennNode? {
    let children = c.children ?? []
    guard children.count > 1, children[children.count - 1].kind == .BlockExpr else { return nil }
    return children[children.count - 1]
}

/// A bare use is a user property (spec 9.4) and has no arguments.
private func useWithPath(_ n: TennNode) -> Bool {
    let children = n.children ?? []
    return children.count > 1 && children[1].kind != .BlockExpr
}

/// Classifies a document by sceneElement over its elements: an element with items that is not a scene
/// element is classic, and a mounted file makes a document a scene when nothing else decides.
func kindOfModel(_ m: ElementModel) -> DocKind {
    var scene = false
    var classic = false
    var mounts = false
    func walk(_ e: Element) {
        if sceneElement(e) {
            scene = true
        } else if !e.items.isEmpty {
            classic = true
        }
        if !e.fromFile.isEmpty { mounts = true }
        for c in e.elements { walk(c) }
    }
    walk(m)
    switch (scene || mounts, classic) {
    case (true, true): return .mixed
    case (true, false): return .scene
    default: return .diagram
    }
}
