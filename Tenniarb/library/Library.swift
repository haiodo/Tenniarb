//
//  Library.swift
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

/// Names and folders of the libraries a document mounts with `--from-file`: the part go/library plays.
enum Library {
    /// The prefix a document writes in place of a path.
    static let scheme = "tenniarb-library://"

    /// A folder of libraries looked at before the shipped ones, so a checkout shadows the shipped copy.
    static let envLibraries = "TENNIARB_LIBRARIES"

    /// The file that lists installed libraries.
    static let indexName = "libraries.json"

    /// Bounds the walk up from a workspace, short enough never to climb out of the user's tree.
    static let findDepth = 12

    /// One segment of a library name.
    static let namePattern = "^[a-z0-9][a-z0-9-]*$"

    /// A library name or path that cannot be used; the message is the one go/library gives.
    struct Failure: Error, CustomStringConvertible {
        let message: String

        var description: String { message }
    }

    /// What a document writes after `tenniarb-library://`: a name, or owner/name for an installed library.
    struct Path: Equatable {
        var owner = ""
        var name = ""

        /// The form a document writes.
        var string: String {
            owner.isEmpty ? Library.scheme + name : Library.scheme + owner + "/" + name
        }

        /// The file below a root.
        var rel: String {
            owner.isEmpty ? name + ".tenn" : owner + "/" + name + ".tenn"
        }

        /// The name a reader writes in a document, without the scheme.
        var label: String {
            owner.isEmpty ? name : owner + "/" + name
        }

        /// Also guards a Path built by hand, so no name can leave its root.
        func valid() throws {
            var messages: [String] = []
            if let problem = Library.nameProblem(name) {
                messages.append(problem)
            }
            if !owner.isEmpty, let problem = Library.nameProblem(owner) {
                messages.append(problem)
            }
            if !messages.isEmpty {
                throw Failure(message: messages.joined(separator: "\n"))
            }
        }
    }

    static func isScheme(_ s: String) -> Bool {
        s.hasPrefix(scheme)
    }

    /// Reads "tenniarb-library://haiodo/c4"; a trailing ".tenn" is accepted and dropped.
    static func parse(_ s: String) throws -> Path {
        let lowered = s.lowercased()
        guard lowered.hasPrefix(scheme) else {
            throw Failure(message: "\"\(s)\" is not a \(scheme) name")
        }
        var rest = String(lowered.dropFirst(scheme.count))
        if rest.hasSuffix(".tenn") {
            rest = String(rest.dropLast(5))
        }
        let parts = rest.components(separatedBy: "/")
        if parts.count > 2 {
            throw Failure(
                message: "library \"\(rest)\": want name or owner/name, each matching \(namePattern)")
        }
        for seg in parts {  // an empty owner or name fails here, so no "/x" is taken for a name
            if let problem = nameProblem(seg) {
                throw Failure(message: problem)
            }
        }
        return parts.count == 2 ? Path(owner: parts[0], name: parts[1]) : Path(name: parts[0])
    }

    /// Why seg is not one segment of a library name; nil when it is.
    private static func nameProblem(_ seg: String) -> String? {
        let scalars = seg.unicodeScalars
        let ok =
            scalars.first.map(isNameStart) == true
            && scalars.dropFirst().allSatisfy(isNameBody)
        guard ok else {
            return
                "library name \"\(seg)\": must match \(namePattern), lowercase letters digits and dashes only"
        }
        return nil
    }

    private static func isNameStart(_ c: Unicode.Scalar) -> Bool {
        (97...122).contains(c.value) || (48...57).contains(c.value)
    }

    private static func isNameBody(_ c: Unicode.Scalar) -> Bool {
        isNameStart(c) || c == "-"
    }

    // MARK: - Folders

    /// Where a scheme name is looked up: the installed folders first, an installed copy shadows a
    /// shipped one, then the shipped folders in order.
    struct Finder {
        var installed: [String] = []
        var shipped: [String] = []

        /// The folders of the finder, installed first, each once.
        var dirs: [String] {
            var out: [String] = []
            for dir in installed + shipped where !dir.isEmpty && !out.contains(dir) {
                out.append(dir)
            }
            return out
        }

        /// The file of p, or an error that names what was looked for and where.
        func resolve(_ p: Path) throws -> URL {
            try p.valid()
            let dirs = self.dirs
            let rel = p.rel
            for dir in dirs {
                let file = URL(fileURLWithPath: dir).appendingPathComponent(rel)
                if isRegularFile(file) {
                    return file.standardizedFileURL
                }
            }
            if dirs.isEmpty {
                throw Failure(
                    message:
                        "library \(p.label) not found: no library folder to look in (set \(envLibraries) or install the application)"
                )
            }
            throw Failure(
                message: "library \(p.label) not found: looked for \(rel) in \(dirs.joined(separator: ", "))")
        }
    }

    /// Where a document at workspace finds the libraries it mounts by name.
    static func `default`(workspace: URL) -> Finder {
        Finder(
            installed: [Comments.configDir().appendingPathComponent("libraries", isDirectory: true).path],
            shipped: shipped(workspace: workspace))
    }

    /// The shipped folders, in order: $TENNIARB_LIBRARIES, beside the executable, the resources of an
    /// .app, and a libraries folder of the repository above the workspace or the working directory.
    static func shipped(workspace: URL?) -> [String] {
        var out: [String] = []
        func add(_ dir: String?) {
            guard let dir, !dir.isEmpty else { return }
            let path = URL(fileURLWithPath: dir).standardizedFileURL.path
            if !out.contains(path) {
                out.append(path)
            }
        }
        add(ProcessInfo.processInfo.environment[envLibraries])
        if let exe = Bundle.main.executableURL?.resolvingSymlinksInPath() {
            let dir = exe.deletingLastPathComponent().path
            add(dir + "/libraries")
            // The resources of an .app: Contents/MacOS/../Resources/libraries.
            add(dir + "/../Resources/libraries")
        }
        add(repoLibraries(workspace ?? URL(fileURLWithPath: FileManager.default.currentDirectoryPath)))
        add(repoLibraries(URL(fileURLWithPath: FileManager.default.currentDirectoryPath)))
        return out
    }

    /// The closest libraries folder with a .tenn file above workspace, or nil when there is none. The
    /// walk stops at the home folder and never runs longer than findDepth steps.
    static func repoLibraries(_ workspace: URL) -> String? {
        var dir = workspace.standardizedFileURL.path
        if dir.isEmpty {
            dir = FileManager.default.currentDirectoryPath
        }
        let home = FileManager.default.homeDirectoryForCurrentUser.path
        for _ in 0..<findDepth {
            let candidate = dir + "/libraries"
            if hasTenn(candidate) {
                return candidate
            }
            if dir == home {
                return nil
            }
            let parent = (dir as NSString).deletingLastPathComponent
            if parent == dir || parent.isEmpty {
                return nil
            }
            dir = parent
        }
        return nil
    }

    /// Whether dir holds at least one .tenn file, so an unrelated folder named libraries is not taken
    /// for the one of the format.
    private static func hasTenn(_ dir: String) -> Bool {
        guard let names = try? FileManager.default.contentsOfDirectory(atPath: dir) else { return false }
        return names.contains { name in
            name.hasSuffix(".tenn") && !isDirectory(URL(fileURLWithPath: dir).appendingPathComponent(name))
        }
    }

    private static func isDirectory(_ url: URL) -> Bool {
        var isDir: ObjCBool = false
        return FileManager.default.fileExists(atPath: url.path, isDirectory: &isDir) && isDir.boolValue
    }

    private static func isRegularFile(_ url: URL) -> Bool {
        var isDir: ObjCBool = false
        guard FileManager.default.fileExists(atPath: url.path, isDirectory: &isDir) else { return false }
        return !isDir.boolValue
    }

    // MARK: - Index

    /// One line of the index of installed libraries.
    struct Entry: Codable, Equatable {
        var owner: String
        var name: String
        var url: String
        var ref: String
    }

    /// Reads the index of dir; a missing file is an empty list, not an error.
    static func loadIndex(dir: URL) throws -> [Entry] {
        let file = dir.appendingPathComponent(indexName)
        guard FileManager.default.fileExists(atPath: file.path) else { return [] }
        do {
            return try JSONDecoder().decode([Entry].self, from: Data(contentsOf: file))
        } catch {
            throw Failure(message: "\(file.path): \(error)")
        }
    }

    /// Writes the index of dir sorted by owner then name.
    static func saveIndex(dir: URL, _ list: [Entry]) throws {
        let sorted = list.sorted { a, b in
            a.owner == b.owner ? a.name < b.name : a.owner < b.owner
        }
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        do {
            try FileManager.default.createDirectory(
                at: dir, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
            try encoder.encode(sorted).write(to: dir.appendingPathComponent(indexName), options: .atomic)
        } catch {
            throw Failure(message: "\(dir.path): \(error)")
        }
    }
}
