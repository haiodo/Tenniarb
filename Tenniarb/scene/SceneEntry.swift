//
//  SceneEntry.swift
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

import Foundation
import Tenn

/// Entry discovery of go/scene/entry.go: the part file that is loaded in the context of the document
/// that mounts it (spec 5a.1).
extension SceneLoader {
    /// The top-level property that names the entry document of a part file: entry "../Design.tenn".
    static let entryProperty = "entry"

    /// How many folders up findEntry looks, the workspace root included.
    static let maxEntryDepth = 8

    /// What entry discovery needs of a file: the files it mounts and its entry property, absolute.
    private struct FileInfo {
        var mounts: [String] = []
        var entry = ""
    }

    /// Loads a file that may be a part of a bigger document. With an entry property the file is loaded
    /// in that entry's context. Without one, a file that loads with errors is tried in the context of
    /// the entry findEntry finds, and that tree comes back when it has fewer errors.
    ///
    /// Go decides on the errors validate() reports at load; validate is not ported, so a use that does
    /// not resolve where it is written stands in for them.
    // shortcut: a name an at block or a slot brings in is not counted - port validate() when the app
    // shows a part file opened in the wrong context.
    func loadAuto(_ file: String) throws -> Scene.Tree {
        if let property = Self.explicitEntry(root: root, file: file) {
            do {
                return try loadIn(file, entry: property.path.path)
            } catch {
                let plain = try load(file)
                plain.fail(property.pos, "entry: \(error)")
                return plain
            }
        }
        let plain = try load(file)
        let plainErrors = plain.fatalCount + unresolvedUses(plain)
        guard plainErrors > 0, let entry = Self.findEntry(root: root, file: file),
            let tree = try? loadIn(file, entry: entry.path),
            tree.fatalCount + unresolvedUses(tree) < plainErrors
        else {
            return plain
        }
        return tree
    }

    /// The absolute path a file names with its entry property, with the position of the property.
    static func explicitEntry(root: URL, file: String) -> (path: URL, pos: Scene.Pos)? {
        let absRoot = realPath(root)
        let abs = realPath(absolute(root: absRoot, file: file))
        guard let src = try? String(contentsOf: abs, encoding: .utf8) else { return nil }
        let ast = TennParser().parse(src)
        for c in ast.children ?? [] where cmdName(c) == entryProperty {
            let written = argText(split(c).args, 0)
            guard !written.isEmpty else { continue }
            let path = abs.deletingLastPathComponent().appendingPathComponent(written).standardizedFileURL
            return (path, nodePos(slashPath(absRoot, abs), c))
        }
        return nil
    }

    /// The entry document of file: the .tenn files in its folder and the folders up to the workspace
    /// root that mount it, directly or through other files. The nearest wins, and the search goes on
    /// from it, so the top of a chain of mounts comes back.
    static func findEntry(root: URL, file: String) -> URL? {
        let absRoot = realPath(root)
        let abs = realPath(absolute(root: absRoot, file: file))
        return findEntry(root: absRoot, file: abs, seen: [abs.path: true])
    }

    private static func findEntry(root: URL, file: URL, seen: [String: Bool]) -> URL? {
        if let info = readInfo(file), !info.entry.isEmpty {
            return URL(fileURLWithPath: info.entry)
        }
        guard let cand = nearestMounter(root: root, file: file) else { return nil }
        if seen[cand.path] == true { return cand }
        var seen = seen
        seen[cand.path] = true
        return findEntry(root: root, file: cand, seen: seen) ?? cand
    }

    /// The first file, folder by folder up from file, that reaches file by mounts.
    private static func nearestMounter(root: URL, file: URL) -> URL? {
        var dir = file.deletingLastPathComponent()
        for _ in 0..<maxEntryDepth {
            let names = ((try? FileManager.default.contentsOfDirectory(atPath: dir.path)) ?? []).sorted()
            for name in names where name.hasSuffix(".tenn") {
                let p = realPath(dir.appendingPathComponent(name))
                if p.path == file.path || isDirectory(p) { continue }
                if reaches(from: p, target: file, seen: [:]) { return p }
            }
            let parent = dir.deletingLastPathComponent()
            if dir.path == root.path || parent.path == dir.path { break }
            dir = parent
        }
        return nil
    }

    /// Reports whether from is target or mounts it through some chain of files.
    private static func reaches(from: URL, target: URL, seen: [String: Bool]) -> Bool {
        if from.path == target.path { return true }
        if seen[from.path] == true { return false }
        var seen = seen
        seen[from.path] = true
        guard let info = readInfo(from) else { return false }
        return info.mounts.contains { reaches(from: URL(fileURLWithPath: $0), target: target, seen: seen) }
    }

    /// Reads file (absolute) for its mounts and entry; nil when the file cannot be read.
    // shortcut: a file is read again on every lookup, cache its info by size and mtime (as infos in
    // entry.go does) when entry discovery shows up in a profile.
    private static func readInfo(_ file: URL) -> FileInfo? {
        guard let src = try? String(contentsOf: file, encoding: .utf8) else { return nil }
        return infoOf(file, src)
    }

    private static func infoOf(_ file: URL, _ src: String) -> FileInfo {
        let dir = file.deletingLastPathComponent()
        var info = FileInfo()
        let ast = TennParser().parse(src)
        for c in ast.children ?? [] where cmdName(c) == entryProperty {
            let rel = argText(split(c).args, 0)
            if !rel.isEmpty && info.entry.isEmpty {
                info.entry = realPath(dir.appendingPathComponent(rel)).path
            }
        }
        info.mounts = mountsOf(ast.children ?? [], dir)
        return info
    }

    /// The files the --from-file elements of cmds name, nested elements included.
    private static func mountsOf(_ cmds: [TennNode], _ dir: URL) -> [String] {
        var out: [String] = []
        for c in cmds where cmdName(c) == "element" {
            let (args, block) = split(c)
            if let rel = fromFile(args) {
                out.append(realPath(dir.appendingPathComponent(rel)).path)
                continue
            }
            out += mountsOf(block, dir)
        }
        return out
    }

    private static func isDirectory(_ url: URL) -> Bool {
        var isDir: ObjCBool = false
        return FileManager.default.fileExists(atPath: url.path, isDirectory: &isDir) && isDir.boolValue
    }
}

/// How many uses of the tree name nothing where they are written; the tree a plain load of a part file
/// gives has more of them than the same file in its entry's context.
private func unresolvedUses(_ tree: Scene.Tree) -> Int {
    var count = 0
    var pending: [Scene.Node] = tree.main.map { [$0] } ?? []
    while let n = pending.popLast() {
        if n.kind == .use, (try? tree.resolve(n, n.path)) == nil {
            count += 1
        }
        pending += n.elems
        pending += n.content
        var overrides = n.block
        while let ov = overrides.popLast() {
            if let node = ov.node { pending.append(node) }
            overrides += ov.block
        }
    }
    return count
}

/// file is absolute, or relative to root.
private func absolute(root: URL, file: String) -> URL {
    file.hasPrefix("/") ? URL(fileURLWithPath: file) : root.appendingPathComponent(file)
}

/// Cleans url and resolves symlinks when it exists, as the loader does.
private func realPath(_ url: URL) -> URL {
    url.standardizedFileURL.resolvingSymlinksInPath()
}

/// The path of abs relative to root, with slash separators.
private func slashPath(_ root: URL, _ abs: URL) -> String {
    let prefix = root.path + "/"
    guard abs.path.hasPrefix(prefix) else { return abs.path }
    return String(abs.path.dropFirst(prefix.count))
}
