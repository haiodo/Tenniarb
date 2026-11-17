//
//  LinksTests.swift
//  TenniarbTests
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

import XCTest

@testable import Tenniarb

/// The link format, the remotes of local clones and the document key: go/link without windows.
@MainActor
final class LinksTests: XCTestCase {

    /// 2026-10-28T10:00:00Z, the time of the Go tests.
    private let t0 = Date(timeIntervalSince1970: 1_793_181_600)

    private func scratch(_ prefix: String) throws -> URL {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("\(prefix)-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        return root
    }

    // MARK: - Remotes and roots

    func testNormalizeRemote() {
        let want = "github.com/haiodo/foundation"
        for raw in [
            "git@github.com:haiodo/foundation.git",
            "https://github.com/haiodo/foundation.git",
            "https://github.com/haiodo/foundation",
            "https://github.com/haiodo/foundation/",
            "https://user:token@github.com/haiodo/foundation.git",
            "ssh://git@github.com:22/haiodo/foundation.git",
            "git://github.com/haiodo/foundation.git\n",
            "GitHub.com:Haiodo/Foundation",
        ] {
            XCTAssertEqual(Links.normalizeRemote(raw), want, raw)
        }
        for raw in ["", "foundation", "/srv/git/foundation.git", "https://github.com"] {
            XCTAssertEqual(Links.normalizeRemote(raw), "", raw)
        }
    }

    // MARK: - The two forms

    func testRoundTrip() throws {
        let links = [
            Links.Link(
                repo: "github.com/haiodo/foundation", path: "design/PlatformDesign.tenn",
                element: ["Tracker", "Boards"], node: "Kanban"),
            Links.Link(repo: "github.com/a/b", path: "File.tenn"),
            Links.Link(repo: "github.com/a/b", path: "File.tenn", node: "x"),
            Links.Link(
                repo: "github.com/a/b", path: "My docs/Дизайн.tenn",
                element: ["Трекер досок", "Boards#1"], node: "Card#2"),
            Links.Link(
                repo: "github.com/a/b", path: "a.tenn", element: ["Screen", "v1.2 a/b 100%"],
                node: "login/Header.Title"),
            Links.Link(repo: "github.com/a/b", path: "a.tenn", element: ["A&B=C?"], node: "x y&z"),
            Links.Link(home: true, path: "notes/a.tenn", element: ["E"]),
        ]
        for l in links {
            let got = try Links.parse(l.url())
            XCTAssertEqual(got, l, l.url())

            let plain = try Links.parse(l.text())
            XCTAssertEqual(plain.path, l.path, l.text())
            XCTAssertEqual(plain.element, l.element, l.text())
            XCTAssertEqual(plain.node, l.node, l.text())
            XCTAssertEqual(plain.repo, "", l.text())
        }
    }

    func testURLShape() {
        let l = Links.Link(
            repo: "github.com/haiodo/foundation", path: "a/B c.tenn", element: ["Tracker", "Boards"],
            node: "Kanban")
        XCTAssertEqual(
            l.url(),
            "tenniarb://open?repo=github.com/haiodo/foundation&path=a/B%20c.tenn&el=Tracker.Boards&node=Kanban")
        XCTAssertEqual(l.text(), "a/B c.tenn#Tracker.Boards/Kanban")
    }

    func testParsePlain() throws {
        var l = try Links.parse("  PlatformDesign.tenn#Tracker.Boards/Kanban\n")
        XCTAssertEqual(l.path, "PlatformDesign.tenn")
        XCTAssertEqual(l.element, ["Tracker", "Boards"])
        XCTAssertEqual(l.node, "Kanban")

        // The first "/" ends the element; the rest is the node, an instance path included.
        l = try Links.parse("d/F.tenn#Screen/login/Header.Title")
        XCTAssertEqual(l.node, "login/Header.Title")
        XCTAssertEqual(l.element, ["Screen"])

        l = try Links.parse("F.tenn#/Kanban")
        XCTAssertEqual(l.element, [])
        XCTAssertEqual(l.node, "Kanban")

        l = try Links.parse("F.tenn")
        XCTAssertEqual(l.path, "F.tenn")
        XCTAssertEqual(l.element, [])
        XCTAssertEqual(l.node, "")

        XCTAssertEqual(try Links.parse("F.TENN").path, "F.TENN", "the extension is compared without case")
        XCTAssertEqual(try Links.parse("tenniarb://OPEN?path=x.tenn").path, "x.tenn")
    }

    func testParseRefuses() {
        for s in [
            "", "hello", "notes.txt#a", "foo#1",
            "tenniarb://open?path=../x.tenn",
            "tenniarb://open?path=/etc/x.tenn",
            "tenniarb://open?path=a/../../x.tenn",
            "tenniarb://open?path=x.txt",
            "tenniarb://open?repo=a/b",
            "tenniarb://close?path=x.tenn",
            "../x.tenn#a",
            "/abs/x.tenn#a",
            "x.tenn#a..b",
        ] {
            XCTAssertThrowsError(try Links.parse(s), s)
        }

        XCTAssertThrowsError(try Links.parse("hello")) {
            XCTAssertEqual($0 as? Links.Error, Links.Error.notLink)
            XCTAssertEqual($0.localizedDescription, "not a tenniarb link")
        }
        XCTAssertThrowsError(try Links.parse("tenniarb://close?path=x.tenn")) {
            XCTAssertEqual($0 as? Links.Error, Links.Error.unknownAction)
            XCTAssertEqual($0.localizedDescription, "link: unknown action, only open is known")
        }
        XCTAssertThrowsError(try Links.parse("x.tenn#a..b")) {
            XCTAssertEqual($0 as? Links.Error, Links.Error.emptyElement)
            XCTAssertEqual($0.localizedDescription, "link: empty element name")
        }
        XCTAssertThrowsError(try Links.parse("tenniarb://open?path=x.txt")) {
            XCTAssertEqual($0.localizedDescription, "link: path must name a .tenn file")
        }
        XCTAssertThrowsError(try Links.parse("tenniarb://open?path=../x.tenn")) {
            XCTAssertEqual($0.localizedDescription, "link: path must stay inside the repository")
        }
    }

    func testIsLinkAndIsURL() {
        XCTAssertTrue(Links.isLink("a.tenn"))
        XCTAssertTrue(Links.isLink("dir/a.tenn#E"))
        XCTAssertTrue(Links.isLink("  tenniarb://open?path=a.tenn\n"))
        XCTAssertFalse(Links.isLink("a.txt"))
        XCTAssertFalse(Links.isLink("tenniarb://close?path=a.tenn"))

        XCTAssertTrue(Links.isURL("  TENNIARB://open?path=a.tenn"))
        XCTAssertTrue(Links.isURL("tenniarb://close"), "a link that does not parse is still a URL")
        XCTAssertFalse(Links.isURL("a.tenn"))
        XCTAssertFalse(Links.isURL("notes about tenniarb: links"))
    }

    // MARK: - Resolution

    func testResolve() {
        let files = Set([
            "/work/foundation/design/P.tenn",
            "/work/other/design/P.tenn",
            "/home/me/notes/n.tenn",
        ])
        let exists: (String) -> Bool = { files.contains($0) }
        let roots = [
            Links.Root(dir: "/work/other", remote: "github.com/x/other"),
            Links.Root(dir: "/work/foundation", remote: "github.com/haiodo/foundation"),
        ]
        let repos = Links.Repos()

        var l = Links.Link(repo: "github.com/haiodo/foundation", path: "design/P.tenn")
        XCTAssertEqual(repos.resolve(l, roots: roots, home: "/home/me", exists: exists), "/work/foundation/design/P.tenn")

        l.repo = "GitHub.com/Haiodo/Foundation"
        XCTAssertNotNil(repos.resolve(l, roots: roots, home: "/home/me", exists: exists), "the remote is compared without case")

        l.repo = "github.com/x/unknown"
        XCTAssertNil(repos.resolve(l, roots: roots, home: "/home/me", exists: exists), "a clone of another repository")

        l = Links.Link(repo: "github.com/haiodo/foundation", path: "design/missing.tenn")
        XCTAssertNil(repos.resolve(l, roots: roots, home: "/home/me", exists: exists), "the clone has no such file")

        // A plain link takes the first clone that has the file.
        XCTAssertEqual(
            repos.resolve(Links.Link(path: "design/P.tenn"), roots: roots, home: "/home/me", exists: exists),
            "/work/other/design/P.tenn")
        XCTAssertEqual(
            repos.resolve(Links.Link(home: true, path: "notes/n.tenn"), roots: [], home: "/home/me", exists: exists),
            "/home/me/notes/n.tenn")
        XCTAssertNil(
            repos.resolve(Links.Link(home: true, path: "notes/n.tenn"), roots: [], home: "", exists: exists),
            "no home folder")
    }

    // MARK: - Git

    func testOf() throws {
        try requireGit()
        let dir = try scratch("tenniarb-links-of")
        let repo = dir.appendingPathComponent("clone", isDirectory: true)
        let bare = dir.appendingPathComponent("norem", isDirectory: true)
        try FileManager.default.createDirectory(
            at: repo.appendingPathComponent("design"), withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: bare, withIntermediateDirectories: true)
        try git(repo, ["init", "-q"])
        try git(repo, ["remote", "add", "origin", "git@github.com:haiodo/foundation.git"])
        try git(bare, ["init", "-q"])
        let file = repo.appendingPathComponent("design/P.tenn")
        try Data().write(to: file)

        let repos = Links.Repos()
        let (l, note) = try repos.of(file: file.path, home: dir.path, element: ["A"], node: "n")
        XCTAssertEqual(note, "")
        XCTAssertEqual(l.repo, "github.com/haiodo/foundation")
        XCTAssertEqual(l.path, "design/P.tenn")
        XCTAssertFalse(l.home)

        // A repository without a remote, and a folder without a repository, fall back to home.
        let (home, homeNote) = try repos.of(file: bare.appendingPathComponent("q.tenn").path, home: dir.path)
        XCTAssertTrue(home.home)
        XCTAssertEqual(home.path, "norem/q.tenn")
        XCTAssertEqual(
            homeNote,
            "The file is not in a git repository with a remote: the link works only for files at the same place under home."
        )

        let loose = try scratch("tenniarb-links-loose").appendingPathComponent("q.tenn")
        XCTAssertThrowsError(try repos.of(file: loose.path, home: dir.path)) {
            XCTAssertEqual($0 as? Links.Error, Links.Error.noBase)
            XCTAssertEqual(
                $0.localizedDescription,
                "the file is not in a git repository with a remote and not under the home folder")
        }
    }

    func testOfWithoutARepositoryNamesTheReason() throws {
        let home = try scratch("tenniarb-links-home")
        let (l, note) = try Links.Repos().of(file: home.appendingPathComponent("q.tenn").path, home: home.path)
        XCTAssertTrue(l.home)
        XCTAssertEqual(l.path, "q.tenn")
        XCTAssertEqual(
            note, "The file is not in a git repository: the link works only for files at the same place under home.")
    }

    /// Two clones of one repository name a document alike; another remote or no repository does not.
    func testDocKeyAcrossClones() throws {
        try requireGit()
        let dir = try scratch("tenniarb-links-key")
        let a = try clone(dir, "a", "git@github.com:haiodo/foundation.git")
        let b = try clone(dir, "b", "https://github.com/haiodo/foundation")
        let other = try clone(dir, "c", "https://github.com/haiodo/other.git")

        let repos = Links.Repos()
        let ka = repos.docKey(a.appendingPathComponent("design/P.tenn").path)
        XCTAssertEqual(ka, "repo:github.com/haiodo/foundation#design/P.tenn")
        XCTAssertEqual(repos.docKey(b.appendingPathComponent("design/P.tenn").path), ka)
        XCTAssertNotEqual(repos.docKey(other.appendingPathComponent("design/P.tenn").path), ka)
        XCTAssertNotEqual(repos.docKey(a.appendingPathComponent("design/Q.tenn").path), ka)

        let loose = dir.appendingPathComponent("loose/P.tenn")
        XCTAssertEqual(repos.docKey(loose.path), "path:" + loose.standardizedFileURL.path)
    }

    func testControllerKeysTheCommentsByRepository() throws {
        try requireGit()
        let dir = try scratch("tenniarb-links-comments")
        let repo = try clone(dir, "clone", "git@github.com:haiodo/foundation.git")
        let file = repo.appendingPathComponent("design/P.tenn")
        try Data().write(to: file)
        let cfg = dir.appendingPathComponent("cfg", isDirectory: true)
        let store = Comments.Store(dir: Comments.dirIn(cfg))

        let key = "repo:github.com/haiodo/foundation#design/P.tenn"
        let comments = CommentsController(url: file, configDir: cfg)
        XCTAssertEqual(comments.key, key)
        _ = try comments.addComment(text: "tight", anchor: Comments.Anchor(), now: t0)
        XCTAssertTrue(FileManager.default.fileExists(atPath: store.file(for: key).path))

        let loose = try scratch("tenniarb-links-comments-loose").appendingPathComponent("P.tenn")
        let outside = CommentsController(url: loose, configDir: cfg)
        XCTAssertEqual(outside.key, "path:" + loose.standardizedFileURL.path)
        _ = try outside.addComment(text: "here", anchor: Comments.Anchor(), now: t0)
        XCTAssertTrue(
            FileManager.default.fileExists(atPath: store.file(for: "path:" + loose.standardizedFileURL.path).path))

        XCTAssertNil(CommentsController(url: nil, configDir: cfg).key, "an untitled document has no key")
    }

    // MARK: - Git in a scratch folder

    private struct GitFailure: Error {
        let output: String
    }

    private func requireGit() throws {
        guard (try? git(nil, ["--version"])) != nil else { throw XCTSkip("no git") }
    }

    private func git(_ dir: URL?, _ args: [String]) throws -> String {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/env")
        process.arguments = ["git"] + (dir.map { ["-C", $0.path] } ?? []) + args
        let pipe = Pipe()
        process.standardOutput = pipe
        process.standardError = pipe
        try process.run()
        let data = pipe.fileHandleForReading.readDataToEndOfFile()
        process.waitUntilExit()
        let out = String(decoding: data, as: UTF8.self)
        guard process.terminationStatus == 0 else { throw GitFailure(output: out) }
        return out
    }

    private func clone(_ dir: URL, _ name: String, _ remote: String) throws -> URL {
        let d = dir.appendingPathComponent(name, isDirectory: true)
        try FileManager.default.createDirectory(
            at: d.appendingPathComponent("design"), withIntermediateDirectories: true)
        try git(d, ["init", "-q"])
        try git(d, ["remote", "add", "origin", remote])
        return d
    }
}
