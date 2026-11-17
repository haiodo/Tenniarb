//
//  LinkOpen.swift
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

import Cocoa

/// The place a link names in an open document; go/app's gotoLink without a window.
struct LinkTarget {
    /// The element row of a scene file: its index path among the elements of the file; nil for a classic
    /// document.
    var elementPath: [Int]?
    /// The element of a classic document, whose selection is by object.
    var element: Element?
    /// The item of a classic document the link names.
    var item: String?
    /// The byte offset of the node in the scene file; nil when the link names no node or the node is gone.
    var nodeOffset: Int?
}

/// Finding the file of a link and the place in a document: go/app's linkFile and gotoLink.
@MainActor
enum LinkOpen {

    /// One lookup for every open: git runs at most once per repository root.
    private static let repos = Links.Repos()

    /// Whether path names a file, as go/app's isFile.
    static func isFile(_ path: String) -> Bool {
        var isDir: ObjCBool = false
        return FileManager.default.fileExists(atPath: path, isDirectory: &isDir) && !isDir.boolValue
    }

    /// The file a link names on this machine, or nil. The clones are those of the active document and of
    /// the open ones, deduplicated; a plain link also takes the file beside the active document.
    static func resolve(
        _ link: Links.Link, active: URL?, openDocuments: [URL], home: URL, exists: (String) -> Bool
    ) -> String? {
        if link.home {
            return repos.resolve(link, roots: [], home: home.path, exists: exists)
        }
        var roots: [Links.Root] = []
        var seen = Set<String>()
        for url in [active].compactMap({ $0 }) + openDocuments {
            guard let dir = Links.findRoot(url.path), seen.insert(dir).inserted else { continue }
            // A plain link takes the first clone that has the file, so its remote is not asked for.
            roots.append(Links.Root(dir: dir, remote: link.repo.isEmpty ? "" : repos.remote(of: dir)))
        }
        if let path = repos.resolve(link, roots: roots, home: home.path, exists: exists) {
            return path
        }
        // A plain link is often the name of a file beside the open one, not a path from the clone root.
        guard link.repo.isEmpty, let folder = active?.deletingLastPathComponent() else { return nil }
        let path = folder.appendingPathComponent(link.path).standardizedFileURL.path
        return exists(path) ? path : nil
    }

    /// The place a link names in the document, or nil when no such element is there. A node that is gone
    /// still gives the element.
    static func target(in document: Document, element: [String], node: String) -> LinkTarget? {
        if let tree = document.sceneTree {
            return sceneTarget(in: tree, element: element, node: node)
        }
        guard let model = document.store?.model else { return nil }
        return classicTarget(in: model, element: element, node: node)
    }

    private static func sceneTarget(in tree: Scene.Tree, element: [String], node: String) -> LinkTarget? {
        var nodes = tree.main?.elems ?? []
        var current = tree.main
        var path: [Int] = []
        for seg in element {
            guard let hit = sceneChild(nodes, seg) else { return nil }
            path.append(nodes.firstIndex { $0 === hit } ?? 0)
            current = hit
            nodes = hit.elems
        }
        guard let current else { return nil }
        if node.isEmpty { return LinkTarget(elementPath: path) }
        return LinkTarget(elementPath: path, nodeOffset: CommentPaths.node(at: node, from: current)?.range.start)
    }

    /// The element a path segment names among siblings: its name or key, "#n" for the n-th repeat.
    private static func sceneChild(_ nodes: [Scene.Node], _ seg: String) -> Scene.Node? {
        let (name, index) = CommentPaths.splitIndex(seg)
        var left = index
        for n in nodes where n.kind == .element && (n.name == name || n.key == name) {
            if left == 0 { return n }
            left -= 1
        }
        return nil
    }

    private static func classicTarget(in model: ElementModel, element: [String], node: String) -> LinkTarget? {
        var current: Element = model
        for seg in element {
            guard let hit = classicChild(current.elements, seg) else { return nil }
            current = hit
        }
        if node.isEmpty { return LinkTarget(element: current) }
        return LinkTarget(element: current, item: classicItem(current, node))
    }

    /// The element a path segment names among siblings, by written name or as go/scene normalises it.
    private static func classicChild(_ elements: [Element], _ seg: String) -> Element? {
        let (name, index) = CommentPaths.splitIndex(seg)
        let key = normalizeName(name)
        var left = index
        for e in elements where e.name == name || normalizeName(e.name) == key {
            if left == 0 { return e }
            left -= 1
        }
        return nil
    }

    /// The name of the item a node names, exactly or as a scene path writes it, as go/app's linkItem.
    private static func classicItem(_ element: Element, _ node: String) -> String? {
        let items = element.items.filter { !($0 is LinkItem) }
        if let item = items.first(where: { $0.name == node }) { return item.name }
        let key = normalizeName(node)
        return items.first { normalizeName($0.name) == key }?.name
    }
}
