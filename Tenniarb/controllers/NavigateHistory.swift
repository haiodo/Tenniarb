//
//  NavigateHistory.swift
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

/// An entry holds names, not indices: the element's names from the root, node paths, or a classic
/// element's items. A move keeps an entry; a rename or a delete decides whether it still resolves.
struct NavigationEntry: Equatable {
    var element: [String] = []
    var nodes: [String] = []
    var items: [String] = []
}

/// The selections Back and Forward walk, oldest first, as go/app/navhist.go: at is the entry on screen.
struct NavigateHistory {
    /// The cap of the entries kept, as go/app/navhist.go navMax.
    static let limit = 100

    private(set) var entries: [NavigationEntry] = []
    private(set) var at = 0

    /// Makes entry the one on screen and drops what was ahead of it; the entry already on screen is not
    /// repeated.
    mutating func push(_ entry: NavigationEntry) {
        if !entries.isEmpty {
            if entries[at] == entry { return }
            entries.removeSubrange((at + 1)...)
        }
        entries.append(entry)
        if entries.count > Self.limit {
            entries.removeFirst(entries.count - Self.limit)
        }
        at = entries.count - 1
    }

    /// The index of the nearest entry in direction (-1 back, +1 forward) that valid accepts; -1 when there
    /// is none. An entry valid refuses stays in the list.
    func find(_ direction: Int, valid: (NavigationEntry) -> Bool) -> Int {
        var i = at + direction
        while i >= 0 && i < entries.count {
            if valid(entries[i]) { return i }
            i += direction
        }
        return -1
    }

    func can(_ direction: Int, valid: (NavigationEntry) -> Bool) -> Bool {
        find(direction, valid: valid) >= 0
    }

    /// Makes the nearest entry in direction that valid accepts the one on screen and returns it.
    mutating func move(_ direction: Int, valid: (NavigationEntry) -> Bool) -> NavigationEntry? {
        let i = find(direction, valid: valid)
        guard i >= 0 else { return nil }
        at = i
        return entries[i]
    }

    /// The entries once the element at path old has the path now, as go/app's afterElementRename.
    mutating func elementRenamed(from old: [String], to now: [String]) {
        guard !old.isEmpty else { return }
        for i in entries.indices where entries[i].element.starts(with: old) {
            entries[i].element = now + entries[i].element.dropFirst(old.count)
        }
    }

    /// The entries once the node at old of the element has the path now, as go/app's afterNodeRename: a
    /// path of the node or below it follows.
    mutating func nodeRenamed(in element: [String], from old: String, to now: String) {
        guard !old.isEmpty else { return }
        for i in entries.indices where entries[i].element == element {
            entries[i].nodes = entries[i].nodes.map { path in
                if path == old { return now }
                guard path.hasPrefix(old) else { return path }
                let rest = path.dropFirst(old.count)
                guard rest.first == "." || rest.first == "/" else { return path }
                return now + String(rest)
            }
        }
    }
}

/// The names from the model root to an element, as go/model PathOf writes a path: "#n" for a name a
/// sibling before it holds too.
func classicElementPath(_ element: Element) -> [String] {
    var segments: [String] = []
    var current: Element? = element
    while let e = current, let parent = e.parent {
        var repeatIndex = 0
        for sibling in parent.elements {
            if sibling === e { break }
            if sibling.name == e.name { repeatIndex += 1 }
        }
        segments.append(repeatIndex > 0 ? "\(e.name)#\(repeatIndex)" : e.name)
        current = parent
    }
    return segments.reversed()
}

/// The place an entry names in the document, or nil when one of its names no longer resolves.
@MainActor
enum NavigateResolve {

    struct Place {
        /// The index path of the element among the elements of the file, as the outline writes it.
        var path: [Int]
        var nodes: [Scene.Node]
    }

    /// The element of a scene and the nodes of the entry inside it; as go/app's resolveSelection.
    static func scene(_ entry: NavigationEntry, in tree: Scene.Tree) -> Place? {
        guard !entry.element.isEmpty, let root = tree.main else { return nil }
        var element = root
        var path: [Int] = []
        for seg in entry.element {
            let (name, index) = CommentPaths.splitIndex(seg)
            var repeatIndex = index
            var found: (node: Scene.Node, at: Int)?
            for (i, c) in element.elems.enumerated() where c.kind == .element && c.name == name {
                if repeatIndex == 0 {
                    found = (c, i)
                    break
                }
                repeatIndex -= 1
            }
            guard let hit = found else { return nil }
            element = hit.node
            path.append(hit.at)
        }
        var nodes: [Scene.Node] = []
        for nodePath in entry.nodes {
            guard let node = CommentPaths.node(at: nodePath, from: element) else { return nil }
            nodes.append(node)
        }
        return Place(path: path, nodes: nodes)
    }

    struct ClassicPlace {
        var element: Element
        var items: [DiagramItem]
    }

    /// The element of a classic document and the items of the entry in it; a link has no name to find it
    /// by, as go/app's navSelection skips it.
    static func classic(_ entry: NavigationEntry, in model: ElementModel) -> ClassicPlace? {
        guard !entry.element.isEmpty else { return nil }
        var element: Element = model
        for seg in entry.element {
            let (name, index) = CommentPaths.splitIndex(seg)
            var repeatIndex = index
            var found: Element?
            for c in element.elements where c.name == name {
                if repeatIndex == 0 {
                    found = c
                    break
                }
                repeatIndex -= 1
            }
            guard let hit = found else { return nil }
            element = hit
        }
        var items: [DiagramItem] = []
        for name in entry.items {
            guard let item = element.items.first(where: { !($0 is LinkItem) && $0.name == name }) else {
                return nil
            }
            items.append(item)
        }
        return ClassicPlace(element: element, items: items)
    }
}
