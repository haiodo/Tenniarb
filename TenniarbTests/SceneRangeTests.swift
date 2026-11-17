//
//  SceneRangeTests.swift
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

import Foundation
import XCTest

@testable import Tenniarb

/// The Swift side of go/cmd/tennranges: the byte range of every scene node and property, against what
/// that tool prints for every fixture on this run, so a range off by a byte is caught before it eats text.
final class SceneRangeTests: XCTestCase {

    private let repo = URL(fileURLWithPath: #filePath)
        .deletingLastPathComponent().deletingLastPathComponent()
    private var fixtures: URL { repo.appendingPathComponent("web/fixtures/scene") }
    /// The expected ranges are what go/cmd/tennranges prints now, so nothing expected is kept in the
    /// repository.

    // MARK: - Stand

    /// Every fixture in web/fixtures/scene is compared, so a new .tenn cannot slip in unread.
    func testEveryFixtureMatchesGo() throws {
        let names = try FileManager.default.contentsOfDirectory(atPath: fixtures.path)
            .filter { $0.hasSuffix(".tenn") }.map { String($0.dropLast(".tenn".count)) }.sorted()
        XCTAssertFalse(names.isEmpty, "no fixtures under \(fixtures.path)")
        // One process for the whole stand: the tool prints every fixture as one array.
        let data = try run(try rangesTool(), ["-root", fixtures.path, "-dir", fixtures.path])
        let goldens = try JSONDecoder().decode([RangeGolden].self, from: data)
        let byName = Dictionary(
            uniqueKeysWithValues: goldens.map { (String($0.file.dropLast(".tenn".count)), $0) })
        XCTAssertEqual(Set(byName.keys), Set(names), "the tool has to list every fixture")
        for name in names {
            try compare(name, try XCTUnwrap(byName[name]))
        }
    }

    private func compare(_ name: String, _ golden: RangeGolden) throws {
        let tree = try SceneLoader(root: fixtures).load(name + ".tenn")
        let items = dump(tree)
        XCTAssertEqual(golden.file, name + ".tenn", "\(name): the tool names another file")
        for i in 0..<max(golden.items.count, items.count) {
            guard i < golden.items.count else {
                return XCTFail("\(name): Swift lists \(items[i]) at \(i), Go lists nothing")
            }
            guard i < items.count else {
                return XCTFail("\(name): Go lists \(golden.items[i]) at \(i), Swift lists nothing")
            }
            guard golden.items[i] == items[i] else {
                return XCTFail(
                    "\(name): item \(i) \(items[i].path): Go \(golden.items[i]), Swift \(items[i])")
            }
        }
    }

    /// go/cmd/tennranges built into the temp directory; the Go build cache keeps this to a moment.
    private func rangesTool() throws -> URL {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("tenniarb-tennranges", isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let exe = dir.appendingPathComponent("tennranges")
        _ = try run(
            try goExecutable(), ["build", "-o", exe.path, "./cmd/tennranges"],
            cwd: repo.appendingPathComponent("go"))
        return exe
    }

    /// The Go toolchain: the test host starts with a bare PATH, so the usual install places are tried.
    private func goExecutable() throws -> URL {
        for path in ["/opt/homebrew/bin/go", "/usr/local/bin/go", "/usr/bin/go"] where FileManager.default.isExecutableFile(atPath: path) {
            return URL(fileURLWithPath: path)
        }
        throw NSError(
            domain: "SceneRangeTests", code: 1,
            userInfo: [NSLocalizedDescriptionKey: "no go toolchain: the range stand builds go/cmd/tennranges"])
    }

    /// Runs a tool and returns its stdout; a non-zero status fails the test with the tool's stderr.
    @discardableResult
    private func run(_ exe: URL, _ args: [String], cwd: URL? = nil) throws -> Data {
        let process = Process()
        process.executableURL = exe
        process.arguments = args
        if let cwd { process.currentDirectoryURL = cwd }
        let out = Pipe()
        let err = Pipe()
        process.standardOutput = out
        process.standardError = err
        try process.run()
        let data = out.fileHandleForReading.readDataToEndOfFile()
        let message = err.fileHandleForReading.readDataToEndOfFile()
        process.waitUntilExit()
        guard process.terminationStatus == 0 else {
            throw NSError(
                domain: "SceneRangeTests", code: Int(process.terminationStatus),
                userInfo: [
                    NSLocalizedDescriptionKey:
                        "\(exe.lastPathComponent) \(args.joined(separator: " ")): \(String(decoding: message, as: UTF8.self))"
                ])
        }
        return data
    }

    // MARK: - Edge cases

    func testCommandWithANestedBlockEndsAtItsClosingBrace() throws {
        let text = """
            element "E" {
                item A {
                    auto-layout column
                    item B { width 10 }
                }
            }
            """
        let tree = try load(text)
        let element = try XCTUnwrap(tree.main?.elems.first)
        let a = try XCTUnwrap(element.content.first)
        let b = try XCTUnwrap(a.content.first)
        XCTAssertEqual(
            slice(text, element.range),
            "element \"E\" {\n    item A {\n        auto-layout column\n        item B { width 10 }\n    }\n}")
        XCTAssertEqual(slice(text, a.range), "item A {\n        auto-layout column\n        item B { width 10 }\n    }")
        XCTAssertEqual(slice(text, b.range), "item B { width 10 }")
        let layout = try XCTUnwrap(a.prop("auto-layout"))
        XCTAssertEqual(slice(text, layout.range), "auto-layout column")
    }

    func testTemplateHoldsBracesAndQuotes() throws {
        let text = """
            element "E" {
                item A {
                    body %{if (x) { y("a b") } else { z } }
                }
            }
            """
        let tree = try load(text)
        let a = try XCTUnwrap(tree.main?.elems.first?.content.first)
        XCTAssertEqual(slice(text, a.range), "item A {\n        body %{if (x) { y(\"a b\") } else { z } }\n    }")
        let body = try XCTUnwrap(a.prop("body"))
        XCTAssertEqual(slice(text, body.range), "body %{if (x) { y(\"a b\") } else { z } }")
    }

    func testQuotedStringHoldsABrace() throws {
        let text = """
            element "E" {
                item A { title "close } here"; width 10 }
            }
            """
        let tree = try load(text)
        let a = try XCTUnwrap(tree.main?.elems.first?.content.first)
        XCTAssertEqual(slice(text, a.range), "item A { title \"close } here\"; width 10 }")
        XCTAssertEqual(slice(text, try XCTUnwrap(a.prop("title")).range), "title \"close } here\"")
        XCTAssertEqual(slice(text, try XCTUnwrap(a.prop("width")).range), "width 10")
    }

    func testExpressionArgument() throws {
        let text = """
            element "E" {
                item A { width $(10 + 2) }
            }
            """
        let tree = try load(text)
        let a = try XCTUnwrap(tree.main?.elems.first?.content.first)
        XCTAssertEqual(slice(text, try XCTUnwrap(a.prop("width")).range), "width $(10 + 2)")
        XCTAssertEqual(slice(text, a.range), "item A { width $(10 + 2) }")
    }

    func testPropertyEndsAtTheBraceOfItsBlock() throws {
        let text = """
            element "E" {
                item A {
                    animation spin {
                        from { rotate 0 }
                        to { rotate 360 }
                    }
                }
            }
            """
        let tree = try load(text)
        let a = try XCTUnwrap(tree.main?.elems.first?.content.first)
        let animation = try XCTUnwrap(a.prop("animation"))
        XCTAssertEqual(
            slice(text, animation.range),
            "animation spin {\n            from { rotate 0 }\n            to { rotate 360 }\n        }")
        XCTAssertEqual(
            slice(text, a.range),
            "item A {\n        animation spin {\n            from { rotate 0 }\n            to { rotate 360 }\n        }\n    }")
    }

    func testRangesAreBytesNotCharacters() throws {
        let text = """
            element "Кнопка" {
                item A { title "Привет } мир"; width 10 }
            }
            """
        let tree = try load(text)
        let a = try XCTUnwrap(tree.main?.elems.first?.content.first)
        let bytes = Data(text.utf8)
        let start = try XCTUnwrap(bytes.range(of: Data("item A".utf8))).lowerBound
        XCTAssertEqual(a.range.start, start)
        XCTAssertNotEqual(text.count, bytes.count, "the fixture has to be non-ASCII for this test to mean anything")
        XCTAssertEqual(slice(text, a.range), "item A { title \"Привет } мир\"; width 10 }")
        XCTAssertEqual(slice(text, try XCTUnwrap(a.prop("title")).range), "title \"Привет } мир\"")
    }

    // MARK: - Helpers

    /// A workspace with one file, as the loader reads it from disk.
    private func load(_ text: String) throws -> Scene.Tree {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-ranges-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        try text.write(to: root.appendingPathComponent("a.tenn"), atomically: true, encoding: .utf8)
        return try SceneLoader(root: root).load("a.tenn")
    }

    /// The text in a byte range: the ranges are UTF-8 bytes, and the files hold non-ASCII text.
    private func slice(_ text: String, _ range: Scene.Range) -> String {
        String(decoding: Array(text.utf8)[range.start..<range.end], as: UTF8.self)
    }
}

// MARK: - tennranges dump

/// One command of the tree as go/cmd/tennranges writes it.
private struct RangeItem: Codable, Equatable, CustomStringConvertible {
    var kind: String
    var path: String
    var file: String
    var start: Int
    var end: Int
    var line: Int
    var col: Int

    var description: String {
        "\(kind) \(file):\(line):\(col) [\(start),\(end))"
    }
}

private struct RangeGolden: Codable {
    var file: String
    var items: [RangeItem]
}

/// The dump of go/cmd/tennranges: every node and property of the tree, mounted files included, in file
/// name order and then in file order. A sort of Swift is not stable, so the index goes into the key.
private func dump(_ tree: Scene.Tree) -> [RangeItem] {
    var out: [RangeItem] = []
    func add(_ kind: String, _ path: String, _ pos: Scene.Pos, _ range: Scene.Range) {
        out.append(
            RangeItem(
                kind: kind, path: path, file: pos.file, start: range.start, end: range.end, line: pos.line, col: pos.col))
    }
    func overrides(_ ovs: [Scene.Override], _ path: String) {
        for ov in ovs {
            if let p = ov.prop {
                add("prop", propPath(path, p.name), p.pos, p.range)
            }
            overrides(ov.block, path)
        }
    }
    func node(_ n: Scene.Node, _ path: String) {
        add(n.kind.description, path, n.pos, n.range)
        // The lists of a mounting element are the lists of the file it mounts, which is listed on its own.
        if n.mount != nil {
            return
        }
        for p in n.props {
            add("prop", propPath(path, p.name), p.pos, p.range)
        }
        // The properties of a use are overrides; its nodes are in content, like every other content node.
        overrides(n.block, path)
        for c in n.elems {
            node(c, joinPath(path, c.name))
        }
        for c in n.content {
            node(c, joinPath(path, c.name))
        }
    }
    var files: [Scene.Node] = []
    if let main = tree.main {
        files.append(main)
    }
    files += tree.order.filter { $0 !== tree.main }
    for f in files {
        node(f, "")
    }
    return out.enumerated().sorted { a, b in
        if a.element.file != b.element.file {
            return a.element.file < b.element.file
        }
        if a.element.start != b.element.start {
            return a.element.start < b.element.start
        }
        return a.offset < b.offset
    }.map { $0.element }
}

/// Adds a segment the way a path writes it, and skips the empty name of an unnamed node.
private func joinPath(_ parent: String, _ name: String) -> String {
    let seg = segText(name)
    if parent.isEmpty {
        return seg
    }
    if seg.isEmpty {
        return parent
    }
    return parent + "." + seg
}

/// SegText of go/scene/names.go: the segment a path writes for a name, quoted when it is not a key.
private func segText(_ name: String) -> String {
    if name.isEmpty {
        return ""
    }
    return normalizeName(name) == name ? name : quoteName(name)
}

/// A property below a node, without the dot a file root has no use for.
private func propPath(_ parent: String, _ name: String) -> String {
    parent.isEmpty ? name : parent + "." + name
}

/// The Swift listing of a library's components against what go/cmd/tenncomponents prints for every
/// fixture on this run: the picker offers what Go offers, in the same order and groups.
final class SceneComponentTests: XCTestCase {

    private let repo = URL(fileURLWithPath: #filePath)
        .deletingLastPathComponent().deletingLastPathComponent()
    private var fixtures: URL { repo.appendingPathComponent("web/fixtures/scene") }

    struct Golden: Decodable {
        struct Mount: Decodable {
            struct Item: Decodable {
                let path: String
                let name: String
                let group: String
            }
            let alias: String
            let file: String
            let components: [Item]
        }
        let file: String
        let mounts: [Mount]
    }

    func testEveryFixtureMountListsLikeGo() throws {
        try compare(args: [])
        try compare(args: ["-q", "a"])
        try compare(args: ["-limit", "1"])
    }

    private func compare(args: [String]) throws {
        let tool = try componentsTool()
        let data = try run(tool, ["-root", fixtures.path, "-dir", "."] + args)
        let goldens = try JSONDecoder().decode([Golden].self, from: data)
        let names = try FileManager.default.contentsOfDirectory(atPath: fixtures.path)
            .filter { $0.hasSuffix(".tenn") }.sorted()
        XCTAssertEqual(goldens.map(\.file), names, "the tool has to list every fixture")

        for golden in goldens {
            let tree = try SceneLoader(root: fixtures).loadAuto(golden.file)
            var mounts: [(alias: String, file: String)] = []
            var seen = Set<ObjectIdentifier>()
            func walk(_ n: Scene.Node) {
                for c in n.elems {
                    if let m = c.mount, seen.insert(ObjectIdentifier(m)).inserted {
                        mounts.append((c.key, m.pos.file))
                    }
                    walk(c)
                }
            }
            walk(try XCTUnwrap(tree.main))
            XCTAssertEqual(mounts.map(\.alias), golden.mounts.map(\.alias), "\(golden.file): mounts")
            for (i, m) in golden.mounts.enumerated() {
                XCTAssertEqual(mounts[i].file, m.file, "\(golden.file): the file of \(m.alias)")
                let root = try XCTUnwrap(tree.file(m.file), "\(golden.file): \(m.file) is loaded")
                let comps = sceneComponents(alias: m.alias, root: root, q: query(args), limit: limit(args))
                XCTAssertEqual(comps.map(\.path), m.components.map(\.path), "\(golden.file) \(m.alias): paths")
                XCTAssertEqual(comps.map(\.name), m.components.map(\.name), "\(golden.file) \(m.alias): names")
                XCTAssertEqual(comps.map(\.group), m.components.map(\.group), "\(golden.file) \(m.alias): groups")
            }
        }
    }

    private func query(_ args: [String]) -> String {
        guard let i = args.firstIndex(of: "-q"), i + 1 < args.count else { return "" }
        return args[i + 1]
    }

    private func limit(_ args: [String]) -> Int {
        guard let i = args.firstIndex(of: "-limit"), i + 1 < args.count, let n = Int(args[i + 1]) else { return 200 }
        return n
    }

    /// go/cmd/tenncomponents built into the temp directory; the Go build cache keeps this to a moment.
    private func componentsTool() throws -> URL {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(
            "tenniarb-tenncomponents", isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let exe = dir.appendingPathComponent("tenncomponents")
        _ = try run(
            try goExecutable(), ["build", "-o", exe.path, "./cmd/tenncomponents"],
            cwd: repo.appendingPathComponent("go"))
        return exe
    }

    private func goExecutable() throws -> URL {
        for path in ["/opt/homebrew/bin/go", "/usr/local/bin/go", "/usr/bin/go"] where FileManager.default.isExecutableFile(atPath: path) {
            return URL(fileURLWithPath: path)
        }
        throw NSError(
            domain: "SceneComponentTests", code: 1,
            userInfo: [NSLocalizedDescriptionKey: "no go toolchain: the component stand builds go/cmd/tenncomponents"])
    }

    @discardableResult
    private func run(_ exe: URL, _ args: [String], cwd: URL? = nil) throws -> Data {
        let process = Process()
        process.executableURL = exe
        process.arguments = args
        if let cwd { process.currentDirectoryURL = cwd }
        let out = Pipe()
        let err = Pipe()
        process.standardOutput = out
        process.standardError = err
        try process.run()
        let data = out.fileHandleForReading.readDataToEndOfFile()
        let message = err.fileHandleForReading.readDataToEndOfFile()
        process.waitUntilExit()
        guard process.terminationStatus == 0 else {
            throw NSError(
                domain: "SceneComponentTests", code: Int(process.terminationStatus),
                userInfo: [
                    NSLocalizedDescriptionKey:
                        "\(exe.lastPathComponent) \(args.joined(separator: " ")): \(String(decoding: message, as: UTF8.self))"
                ])
        }
        return data
    }
}
