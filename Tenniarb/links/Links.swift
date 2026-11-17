//
//  Links.swift
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

/// Links to a place in a .tenn file that work on another machine: the file is named by its git
/// repository and its path inside it, never by an absolute path; docs/links.md has the format.
enum Links {

    /// The URL scheme the app registers with the system.
    static let scheme = "tenniarb"

    /// A place in a .tenn file.
    struct Link: Equatable, Sendable {
        /// The normalised git remote of the repository that holds the file; empty for a plain link and
        /// for a home link.
        var repo = ""
        /// The file is in no repository: path is then relative to the home folder of the user.
        var home = false
        /// Slash path of the file from the repository root (or from home).
        var path = ""
        /// The elements from the root down, as CommentPaths.elementPath writes them: "#n" for a repeat.
        var element: [String] = []
        /// The selected node of the element, as CommentPaths.nodePath writes it; empty selects none.
        var node = ""

        /// The plain form for chats and agents: "dir/File.tenn#Element.Sub/node". Repo and home are not in it.
        func text() -> String {
            let f = fragment()
            return f.isEmpty ? path : path + "#" + f
        }

        /// The tenniarb://open link; the slash stays readable, the keys are go/link's.
        func url() -> String {
            var pairs: [String] = []
            func add(_ key: String, _ value: String) {
                if !value.isEmpty { pairs.append(key + "=" + Links.esc(value)) }
            }
            add("repo", repo)
            if home { add("home", "1") }
            add("path", path)
            add("el", elementText())
            add("node", node)
            var comps = URLComponents()
            comps.scheme = Links.scheme
            comps.host = "open"
            comps.percentEncodedQuery = pairs.joined(separator: "&")
            let base = comps.string ?? Links.scheme + "://open"
            return pairs.isEmpty ? base + "?" : base
        }

        fileprivate func check() throws {
            guard !path.isEmpty, path.lowercased().hasSuffix(".tenn") else { throw Error.pathNotTenn }
            guard !path.hasPrefix("/"), !path.contains("\\"), Links.isLocal(path) else {
                throw Error.pathOutside
            }
        }

        fileprivate mutating func setFragment(_ frag: String) throws {
            if let slash = frag.firstIndex(of: "/") {
                node = String(frag[frag.index(after: slash)...])
                try setElement(String(frag[..<slash]))
            } else {
                node = ""
                try setElement(frag)
            }
        }

        fileprivate mutating func setElement(_ el: String) throws {
            element = []
            if el.isEmpty { return }
            for seg in el.split(separator: ".", omittingEmptySubsequences: false) {
                if seg.isEmpty { throw Error.emptyElement }
                element.append(String(seg).removingPercentEncoding ?? String(seg))
            }
        }

        /// The element names and the node as one path segment: the first "/" after it ends the element.
        private func fragment() -> String {
            var f = elementText()
            if !node.isEmpty { f += "/" + node }
            return f
        }

        /// A name cannot be cut at the separators of the path: "%", "." and "/".
        private func elementText() -> String {
            element.map {
                $0.replacingOccurrences(of: "%", with: "%25")
                    .replacingOccurrences(of: ".", with: "%2E")
                    .replacingOccurrences(of: "/", with: "%2F")
            }.joined(separator: ".")
        }
    }

    /// The refusals of both forms; the messages are the ones go/link returns.
    enum Error: LocalizedError, Equatable, Sendable {
        /// Text that is neither form.
        case notLink
        case pathNotTenn
        case pathOutside
        case emptyElement
        case unknownAction
        /// No repository with a remote holds the file and it is not under the home folder.
        case noBase

        var errorDescription: String? {
            switch self {
            case .notLink: return "not a tenniarb link"
            case .pathNotTenn: return "link: path must name a .tenn file"
            case .pathOutside: return "link: path must stay inside the repository"
            case .emptyElement: return "link: empty element name"
            case .unknownAction: return "link: unknown action, only open is known"
            case .noBase: return "the file is not in a git repository with a remote and not under the home folder"
            }
        }
    }

    /// A local clone: its folder and its normalised remote.
    struct Root: Equatable, Sendable {
        let dir: String
        let remote: String
    }

    /// Makes "host/owner/name" of a git remote, so that the ssh and the https remotes of one repository are
    /// equal: no scheme, user, port or ".git", lower case. "" when raw is no remote.
    static func normalizeRemote(_ raw: String) -> String {
        let raw = raw.trimmingCharacters(in: .whitespacesAndNewlines)
        var host = ""
        var path = ""
        if let schemeEnd = raw.range(of: "://") {
            let rest = raw[schemeEnd.upperBound...]
            let cut = rest.split(separator: "/", maxSplits: 1, omittingEmptySubsequences: false)
            host = String(cut[0])
            path = cut.count > 1 ? String(cut[1]) : ""
            if let at = host.lastIndex(of: "@") { host = String(host[host.index(after: at)...]) }
            if let colon = host.firstIndex(of: ":") { host = String(host[..<colon]) }
        } else if let (h, p) = scpRemote(raw) {
            host = h
            path = p
        }
        path = path.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
        if path.hasSuffix(".git") { path.removeLast(4) }
        guard !host.isEmpty, !path.isEmpty else { return "" }
        return (host + "/" + path).lowercased()
    }

    /// The "user@host:path" form of an ssh remote, as Go's scpRemote regexp reads it.
    private static func scpRemote(_ raw: String) -> (host: String, path: String)? {
        guard let colon = raw.firstIndex(of: ":") else { return nil }
        let head = String(raw[..<colon])
        let path = String(raw[raw.index(after: colon)...])
        guard !path.isEmpty else { return nil }
        var host = head
        if let at = head.lastIndex(of: "@") {
            let user = head[..<at]
            guard !user.isEmpty, !user.contains("/"), !user.contains("@") else { return nil }
            host = String(head[head.index(after: at)...])
        }
        guard !host.isEmpty, !host.contains("/"), !host.contains(":") else { return nil }
        return (host, path)
    }

    /// The nearest folder above file (or file itself, for a folder) that holds a .git entry; a worktree or
    /// a submodule has a .git file. nil when there is none.
    static func findRoot(_ file: String) -> String? {
        var dir = URL(fileURLWithPath: file).deletingLastPathComponent().standardizedFileURL.path
        var isDir: ObjCBool = false
        if FileManager.default.fileExists(atPath: file, isDirectory: &isDir), isDir.boolValue {
            dir = URL(fileURLWithPath: file).standardizedFileURL.path
        }
        while true {
            if FileManager.default.fileExists(atPath: dir + "/.git") { return dir }
            let up = (dir as NSString).deletingLastPathComponent
            if up == dir { return nil }
            dir = up
        }
    }

    /// Reads the URL form or the plain form; surrounding white space is ignored. A plain link has no repo:
    /// it names the file inside whichever clone holds it.
    static func parse(_ s: String) throws -> Link {
        let s = s.trimmingCharacters(in: .whitespacesAndNewlines)
        if s.lowercased().hasPrefix(scheme + ":") { return try parseURL(s) }
        let hash = s.firstIndex(of: "#")
        let file = hash.map { String(s[..<$0]) } ?? s
        let frag = hash.map { String(s[s.index(after: $0)...]) } ?? ""
        guard file.lowercased().hasSuffix(".tenn") else { throw Error.notLink }
        var l = Link()
        l.path = file
        try l.setFragment(frag)
        try l.check()
        return l
    }

    private static func parseURL(_ s: String) throws -> Link {
        guard let comps = URLComponents(string: s) else { throw Error.notLink }
        guard comps.host?.caseInsensitiveCompare("open") == .orderedSame, comps.port == nil else {
            throw Error.unknownAction
        }
        // Go's url.Values.Get takes the first of a repeated key.
        var query: [String: String] = [:]
        for item in comps.queryItems ?? [] where query[item.name] == nil {
            query[item.name] = item.value ?? ""
        }
        var l = Link()
        l.repo = query["repo"] ?? ""
        l.home = query["home"] == "1"
        l.path = query["path"] ?? ""
        l.node = query["node"] ?? ""
        try l.setElement(query["el"] ?? "")
        try l.check()
        return l
    }

    /// Reports whether s reads as a link in either form.
    static func isLink(_ s: String) -> Bool {
        (try? parse(s)) != nil
    }

    /// Reports whether s starts like a tenniarb:// link, valid or not.
    static func isURL(_ s: String) -> Bool {
        s.trimmingCharacters(in: .whitespacesAndNewlines).lowercased().hasPrefix(scheme + ":")
    }

    /// A query value with the slash left readable, as Go's url.QueryEscape plus the two swaps.
    fileprivate static func esc(_ s: String) -> String {
        var out = ""
        for b in s.utf8 {
            switch b {
            case 0x2D, 0x2E, 0x2F, 0x30...0x39, 0x41...0x5A, 0x5F, 0x61...0x7A, 0x7E:
                out.unicodeScalars.append(Unicode.Scalar(b))
            default:
                out += String(format: "%%%02X", Int(b))
            }
        }
        return out
    }

    /// The lexical part of Go's filepath.IsLocal: no empty, "." or ".." segment.
    fileprivate static func isLocal(_ path: String) -> Bool {
        guard !path.isEmpty, !path.hasPrefix("/") else { return false }
        return path.split(separator: "/", omittingEmptySubsequences: false).allSatisfy {
            !$0.isEmpty && $0 != "." && $0 != ".."
        }
    }

    /// file relative to dir with slashes; "" when file is not below dir.
    fileprivate static func relSlash(_ dir: String, _ file: String) -> String {
        let d = URL(fileURLWithPath: dir).standardizedFileURL.pathComponents
        let f = URL(fileURLWithPath: file).standardizedFileURL.pathComponents
        guard f.count > d.count, Array(f.prefix(d.count)) == d else { return "" }
        return f.dropFirst(d.count).joined(separator: "/")
    }

    /// dir and a slash path joined and cleaned, as filepath.Join.
    fileprivate static func join(_ dir: String, _ rel: String) -> String {
        URL(fileURLWithPath: dir, isDirectory: true).appendingPathComponent(rel).standardizedFileURL.path
    }

    /// The absolute slash path of file, cleaned as filepath.Clean does.
    fileprivate static func cleanSlash(_ file: String) -> String {
        URL(fileURLWithPath: file).standardizedFileURL.path
    }

    /// Asks git for the remote of a repository root and remembers the answer.
    final class Repos: @unchecked Sendable {
        private let lock = NSLock()
        private var remotes: [String: String] = [:]

        /// The normalised remote of the clone at root: origin, else the first remote; "" when git is missing
        /// or the clone has none. Git runs at most once per root.
        func remote(of root: String) -> String {
            lock.lock()
            defer { lock.unlock() }
            if let known = remotes[root] { return known }
            let found = gitRemote(root)
            remotes[root] = found
            return found
        }

        /// The clone that holds file; nil outside a repository and when it has no remote.
        func rootOf(_ file: String) -> Root? {
            guard let dir = Links.findRoot(file) else { return nil }
            let remote = remote(of: dir)
            return remote.isEmpty ? nil : Root(dir: dir, remote: remote)
        }

        /// The link to the place in file. A file in a repository with a remote gets repo and the path from
        /// its root; any other file under home gets home and the path from there, and note says so.
        func of(file: String, home: String, element: [String] = [], node: String = "") throws -> (link: Link, note: String) {
            var l = Link()
            l.element = element
            l.node = node
            if let root = rootOf(file) {
                l.repo = root.remote
                l.path = Links.relSlash(root.dir, file)
                return (l, "")
            }
            let rel = Links.relSlash(home, file)
            guard !home.isEmpty, !rel.isEmpty else { throw Error.noBase }
            l.home = true
            l.path = rel
            var note = "The file is not in a git repository with a remote"
            if Links.findRoot(file) == nil { note = "The file is not in a git repository" }
            return (l, note + ": the link works only for files at the same place under home.")
        }

        /// Finds the file of link among the local clones, in the order given; a plain link takes the first
        /// clone that has the file.
        func resolve(_ link: Link, roots: [Root], home: String, exists: (String) -> Bool) -> String? {
            if link.home {
                guard !home.isEmpty else { return nil }
                let p = Links.join(home, link.path)
                return exists(p) ? p : nil
            }
            for root in roots {
                if !link.repo.isEmpty, root.remote.caseInsensitiveCompare(link.repo) != .orderedSame {
                    continue
                }
                let p = Links.join(root.dir, link.path)
                if exists(p) { return p }
            }
            return nil
        }

        /// Names a document the same in every clone of its repository: the remote and the path inside it;
        /// outside a repository with a remote it is the absolute path, which only that file has.
        func docKey(_ file: String) -> String {
            if let root = rootOf(file) {
                let rel = Links.relSlash(root.dir, file)
                if !rel.isEmpty { return "repo:" + root.remote + "#" + rel }
            }
            return "path:" + Links.cleanSlash(file)
        }

        private func gitRemote(_ root: String) -> String {
            if let url = git(root, ["remote", "get-url", "origin"]) {
                return Links.normalizeRemote(url)
            }
            guard let names = git(root, ["remote"]) else { return "" }
            let first = String(names.prefix { $0 != "\n" }).trimmingCharacters(in: .whitespacesAndNewlines)
            guard !first.isEmpty, let url = git(root, ["remote", "get-url", first]) else { return "" }
            return Links.normalizeRemote(url)
        }

        /// Runs git in dir; nil when it cannot start, fails, or takes longer than three seconds.
        private func git(_ dir: String, _ args: [String]) -> String? {
            let process = Process()
            process.executableURL = URL(fileURLWithPath: "/usr/bin/env")
            process.arguments = ["git", "-C", dir] + args
            process.standardError = FileHandle.nullDevice
            let pipe = Pipe()
            process.standardOutput = pipe
            let done = DispatchSemaphore(value: 0)
            process.terminationHandler = { _ in done.signal() }
            do {
                try process.run()
            } catch {
                return nil
            }
            if done.wait(timeout: .now() + 3) == .timedOut {
                process.terminate()
                process.waitUntilExit()
                return nil
            }
            guard process.terminationStatus == 0 else { return nil }
            // shortcut: the output is read after the exit, so it must fit the pipe buffer; a remote URL does.
            let out = pipe.fileHandleForReading.readDataToEndOfFile()
            return String(decoding: out, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines)
        }
    }
}
