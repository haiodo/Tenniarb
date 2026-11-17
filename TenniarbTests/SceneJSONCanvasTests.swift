//
//  SceneJSONCanvasTests.swift
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

/// The JSON Canvas port, against the Go CLIs built on this run: no generated golden is kept in the
/// repo.
final class SceneJSONCanvasTests: XCTestCase {

    private let repo = URL(fileURLWithPath: #filePath)
        .deletingLastPathComponent().deletingLastPathComponent()
    private var fixtures: URL { repo.appendingPathComponent("web/fixtures/scene") }
    private var example: URL { repo.appendingPathComponent("go/jsoncanvas/testdata/example.canvas") }

    // MARK: - Canvas to .tenn

    func testCanvasToTennMatchesGo() throws {
        let data = try Data(contentsOf: example)
        let canvas = try JSONCanvas.parse(data)
        let swift = JSONCanvas.toTenn(canvas, name: "example")

        let out = try run(try tool("canvas2tenn"), [example.path])
        XCTAssertEqual(String(decoding: out.stdout, as: UTF8.self), swift.text, "example.canvas: .tenn text differs from canvas2tenn")

        let warnings = String(decoding: out.stderr, as: UTF8.self)
            .split(separator: "\n").compactMap { line -> String? in
                guard line.hasPrefix("warning: ") else { return nil }
                return String(line.dropFirst("warning: ".count))
            }
        XCTAssertEqual(warnings, swift.notes, "example.canvas: conversion notes differ from canvas2tenn")
    }

    func testTrickyCanvasToTennMatchesGo() throws {
        let json = #"""
            {
              "nodes": [
                {"id": "t1", "type": "text", "x": -10.555, "y": 0, "width": 300, "height": 50,
                 "color": "7", "text": " A title with spaces \r\nsecond line with a \"quote\" and a \\ backslash"},
                {"id": "t2", "type": "text", "x": 0, "y": 200, "width": 150, "height": 50, "color": "1", "text": "same"},
                {"id": "t3", "type": "text", "x": 0, "y": 260, "width": 150, "height": 50, "color": "2", "text": "same"},
                {"id": "f1", "type": "file", "x": 400, "y": 0, "width": 100, "height": 100,
                 "file": "https://example.com/a.png", "subpath": "#H"},
                {"id": "f2", "type": "file", "x": 400, "y": 150, "width": 100, "height": 100,
                 "file": "docs/a b.md", "subpath": "#H", "color": "3"},
                {"id": "l1", "type": "link", "x": 400, "y": 300, "width": 100, "height": 40,
                 "url": "https://x.test/a[b]", "color": "4"},
                {"id": "g1", "type": "group", "x": 600, "y": 0, "width": 300, "height": 300, "label": "Outer",
                 "color": "5", "background": "bg.png", "backgroundStyle": "cover"},
                {"id": "g2", "type": "group", "x": 650, "y": 50, "width": 100, "height": 100, "label": "Outer"},
                {"id": "c1", "type": "text", "x": 660, "y": 60, "width": 50, "height": 20, "text": "child"}
              ],
              "edges": [
                {"id": "e1", "fromNode": "t1", "fromSide": "top", "toNode": "t2", "toEnd": "square",
                 "color": "9", "label": "x\"y"},
                {"id": "e2", "fromNode": "t2", "fromEnd": "arrow", "toNode": "t3"},
                {"id": "e3", "fromNode": "t2", "toNode": "c1"},
                {"id": "e4", "fromNode": "t2", "fromSide": "left", "toSide": "right", "toNode": "t3", "toEnd": "none"}
              ]
            }
            """#
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-canvas-tricky-\(UUID().uuidString)", isDirectory: true)
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        let file = root.appendingPathComponent("tricky.canvas")
        try Data(json.utf8).write(to: file)

        let canvas = try JSONCanvas.parse(Data(json.utf8))
        let swift = JSONCanvas.toTenn(canvas, name: "tricky")
        let out = try run(try tool("canvas2tenn"), [file.path])
        XCTAssertEqual(String(decoding: out.stdout, as: UTF8.self), swift.text, "tricky.canvas: .tenn text differs from canvas2tenn")
        let warnings = String(decoding: out.stderr, as: UTF8.self)
            .split(separator: "\n").compactMap { line -> String? in
                guard line.hasPrefix("warning: ") else { return nil }
                return String(line.dropFirst("warning: ".count))
            }
        XCTAssertEqual(warnings, swift.notes, "tricky.canvas: conversion notes differ from canvas2tenn")
    }

    // MARK: - .tenn to canvas

    func testTennToCanvasMatchesGo() throws {
        let tool = try tool("tenn2canvas")
        let names = try FileManager.default.contentsOfDirectory(atPath: fixtures.path)
            .filter { $0.hasSuffix(".tenn") && !$0.hasSuffix(".lib.tenn") && $0 != "design-kit.tenn" }
            .sorted()
        XCTAssertGreaterThanOrEqual(names.count, 4, "the stand needs the scene fixtures")
        for name in names {
            let tree = try SceneLoader(root: fixtures).loadAuto(name)
            let swift = try JSONCanvas.fromTree(tree)
            let swiftData = try JSONCanvas.marshal(swift.canvas)
            let goData = try run(tool, ["-root", fixtures.path, name]).stdout
            let go = try JSONSerialization.jsonObject(with: goData)
            let mine = try JSONSerialization.jsonObject(with: swiftData)
            if let difference = firstDifference(go, mine, name) {
                XCTFail(difference)
            }
        }
    }

    /// The first place two decoded JSON values differ, as Go then Swift; nil when they are the same.
    private func firstDifference(_ go: Any, _ swift: Any, _ path: String) -> String? {
        if let go = go as? [String: Any] {
            guard let swift = swift as? [String: Any] else {
                return "\(path): Go is an object, Swift is \(swift)"
            }
            if go.keys.sorted() != swift.keys.sorted() {
                return "\(path): Go keys \(go.keys.sorted()), Swift keys \(swift.keys.sorted())"
            }
            for key in go.keys.sorted() {
                if let d = firstDifference(go[key]!, swift[key]!, "\(path).\(key)") {
                    return d
                }
            }
            return nil
        }
        if let go = go as? [Any] {
            guard let swift = swift as? [Any] else {
                return "\(path): Go is a list, Swift is \(swift)"
            }
            if go.count != swift.count {
                return "\(path): Go has \(go.count) entries, Swift has \(swift.count)"
            }
            for i in 0..<go.count {
                if let d = firstDifference(go[i], swift[i], "\(path)[\(i)]") {
                    return d
                }
            }
            return nil
        }
        if let go = go as? NSNumber {
            guard let swift = swift as? NSNumber else {
                return "\(path): Go is \(go), Swift is \(swift)"
            }
            if go.doubleValue != swift.doubleValue {
                return "\(path): Go is \(go), Swift is \(swift)"
            }
            return nil
        }
        if let go = go as? String {
            guard let swift = swift as? String else {
                return "\(path): Go is \"\(go)\", Swift is \(swift)"
            }
            if go != swift {
                return "\(path): Go is \"\(go)\", Swift is \"\(swift)\""
            }
            return nil
        }
        return go as? NSNull != nil && swift as? NSNull != nil ? nil : "\(path): Go is \(go), Swift is \(swift)"
    }

    // MARK: - Parse

    func testParseRejectsBrokenCanvas() throws {
        try expect(
            #"{"nodes":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1},{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1}]}"#,
            "json canvas: node \"a\" is repeated")
        try expect(
            #"{"nodes":[{"id":"a","type":"shape","x":0,"y":0,"width":1,"height":1}]}"#,
            "json canvas: node \"a\" has unknown type \"shape\"")
        try expect(
            #"{"nodes":[{"id":"a","type":"file","x":0,"y":0,"width":1,"height":1}]}"#,
            "json canvas: file node \"a\" has no file")
        try expect(
            #"{"nodes":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1}],"edges":[{"id":"e1","fromNode":"a","toNode":"b"}]}"#,
            "json canvas: edge \"e1\": toNode \"b\" is not a node")
    }

    private func expect(_ json: String, _ message: String) throws {
        XCTAssertThrowsError(try JSONCanvas.parse(Data(json.utf8))) { error in
            XCTAssertEqual((error as? JSONCanvas.CanvasError)?.message, message)
        }
    }

    // MARK: - Round trip

    func testExampleRoundTripKeepsNodes() throws {
        let canvas = try JSONCanvas.parse(try Data(contentsOf: example))
        let text = JSONCanvas.toTenn(canvas, name: "example").text
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-canvas-roundtrip-\(UUID().uuidString)", isDirectory: true)
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        try text.write(to: root.appendingPathComponent("example.tenn"), atomically: true, encoding: .utf8)

        let tree = try SceneLoader(root: root).loadAuto("example.tenn")
        let back = try JSONCanvas.fromTree(tree)
        XCTAssertEqual(back.canvas.nodes.count, canvas.nodes.count)
        XCTAssertEqual(Set(back.canvas.nodes.map(\.id)).count, canvas.nodes.count)
        XCTAssertEqual(back.canvas.nodes.map(\.type), canvas.nodes.map(\.type))
        XCTAssertEqual(back.canvas.edges.count, canvas.edges.count)
        for type in ["text", "file", "link", "group"] {
            XCTAssertEqual(back.canvas.nodes.filter { $0.type == type }.count, canvas.nodes.filter { $0.type == type }.count)
        }
    }

    // MARK: - Stand

    /// go/cmd/<name> built into the temp directory; the Go build cache keeps this to a moment.
    private func tool(_ name: String) throws -> URL {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("tenniarb-\(name)", isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let exe = dir.appendingPathComponent(name)
        _ = try run(try goExecutable(), ["build", "-o", exe.path, "./cmd/\(name)"], cwd: repo.appendingPathComponent("go"))
        return exe
    }

    /// The Go toolchain: the test host starts with a bare PATH, so the usual install places are tried.
    private func goExecutable() throws -> URL {
        for path in ["/opt/homebrew/bin/go", "/usr/local/bin/go", "/usr/bin/go"] where FileManager.default.isExecutableFile(atPath: path) {
            return URL(fileURLWithPath: path)
        }
        throw NSError(
            domain: "SceneJSONCanvasTests", code: 1,
            userInfo: [NSLocalizedDescriptionKey: "no go toolchain: the JSON Canvas stand builds the Go CLIs"])
    }

    @discardableResult
    private func run(_ exe: URL, _ args: [String], cwd: URL? = nil) throws -> (stdout: Data, stderr: Data) {
        let process = Process()
        process.executableURL = exe
        process.arguments = args
        if let cwd { process.currentDirectoryURL = cwd }
        let out = Pipe()
        let err = Pipe()
        process.standardOutput = out
        process.standardError = err
        try process.run()
        let stdout = out.fileHandleForReading.readDataToEndOfFile()
        let stderr = err.fileHandleForReading.readDataToEndOfFile()
        process.waitUntilExit()
        guard process.terminationStatus == 0 else {
            throw NSError(
                domain: "SceneJSONCanvasTests", code: Int(process.terminationStatus),
                userInfo: [
                    NSLocalizedDescriptionKey:
                        "\(exe.lastPathComponent) failed: \(String(decoding: stderr, as: UTF8.self))"
                ])
        }
        return (stdout, stderr)
    }
}

/// Document read and write of .canvas through the same paths a window uses.
@MainActor
final class SceneJSONCanvasDocumentTests: SceneDocumentTestCase {

    private let repo = URL(fileURLWithPath: #filePath)
        .deletingLastPathComponent().deletingLastPathComponent()
    private var example: URL { repo.appendingPathComponent("go/jsoncanvas/testdata/example.canvas") }
    private var fixtures: URL { repo.appendingPathComponent("web/fixtures/scene") }

    private func workspace() throws -> URL {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-canvas-doc-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        return root
    }

    func testOpenCanvasAndWriteItBack() throws {
        let root = try workspace()
        let source = root.appendingPathComponent("example.canvas")
        try FileManager.default.copyItem(at: example, to: source)

        let document = try openTestDocument(source)
        XCTAssertEqual(document.fileURL, source)
        let text = document.sceneText ?? document.store?.model.toTennStr() ?? ""
        XCTAssertTrue(text.hasPrefix("element example"), "the canvas did not convert to the example element:\n\(text)")
        let scene = document.sceneTree
        let classic = document.store?.model.elements.contains { $0.name == "example" } == true
        XCTAssertTrue(scene != nil || classic, "the converted example is in neither the scene tree nor the classic model")
        XCTAssertFalse(document.sceneCanvasNotes.isEmpty, "the imported colours have no exact .tenn shade")

        let out = root.appendingPathComponent("back.canvas")
        try document.write(to: out, ofType: "canvas")
        XCTAssertEqual(document.fileURL, out)
        let canvas = try JSONCanvas.parse(try Data(contentsOf: out))
        XCTAssertEqual(canvas.nodes.count, 10)
        XCTAssertEqual(Set(canvas.nodes.map(\.id)).count, 10)
        XCTAssertEqual(canvas.edges.count, 4)
    }

    func testClassicCanvasStaysClassic() throws {
        let root = try workspace()
        let source = root.appendingPathComponent("classic.canvas")
        let json = #"{"nodes":[{"id":"a","type":"text","x":10,"y":20,"width":100,"height":40,"text":"Card"}]}"#
        try Data(json.utf8).write(to: source)

        let document = try openTestDocument(source)
        XCTAssertNil(document.sceneTree, "a classic diagram must not be opened as a scene")
        XCTAssertNil(document.sceneText)
        XCTAssertEqual(document.store?.model.elements.first?.name, "classic")
        XCTAssertTrue(document.sceneCanvasNotes.isEmpty, "a colourless text node converts without loss")
    }

    func testWriteSceneFixtureMatchesGo() throws {
        try XCTSkipUnless(
            FileManager.default.isExecutableFile(atPath: "/opt/homebrew/bin/go")
                || FileManager.default.isExecutableFile(atPath: "/usr/local/bin/go")
                || FileManager.default.isExecutableFile(atPath: "/usr/bin/go"),
            "no go toolchain: the JSON Canvas stand builds tenn2canvas")
        let document = try openTestDocument(fixtures.appendingPathComponent("auto-layout-tour.tenn"))
        XCTAssertNotNil(document.sceneTree, "auto-layout-tour.tenn has to load as a scene")
        let root = try workspace()
        let out = root.appendingPathComponent("auto-layout-tour.canvas")
        try document.write(to: out, ofType: "canvas")
        let swift = try JSONSerialization.jsonObject(with: try Data(contentsOf: out))

        let tool = try builtTool("tenn2canvas")
        let goData = try runTool(tool, ["-root", fixtures.path, "auto-layout-tour.tenn"])
        let go = try JSONSerialization.jsonObject(with: goData)
        if let difference = firstDifference(go, swift, "auto-layout-tour.canvas") {
            XCTFail(difference)
        }
        XCTAssertFalse(document.sceneCanvasNotes.isEmpty, "auto-layout is not part of JSON Canvas")
    }

    private func firstDifference(_ go: Any, _ swift: Any, _ path: String) -> String? {
        if let go = go as? [String: Any] {
            guard let swift = swift as? [String: Any] else { return "\(path): Go is an object, Swift is \(swift)" }
            if go.keys.sorted() != swift.keys.sorted() {
                return "\(path): Go keys \(go.keys.sorted()), Swift keys \(swift.keys.sorted())"
            }
            for key in go.keys.sorted() {
                if let d = firstDifference(go[key]!, swift[key]!, "\(path).\(key)") { return d }
            }
            return nil
        }
        if let go = go as? [Any] {
            guard let swift = swift as? [Any] else { return "\(path): Go is a list, Swift is \(swift)" }
            if go.count != swift.count { return "\(path): Go has \(go.count) entries, Swift has \(swift.count)" }
            for i in 0..<go.count {
                if let d = firstDifference(go[i], swift[i], "\(path)[\(i)]") { return d }
            }
            return nil
        }
        if let go = go as? NSNumber {
            guard let swift = swift as? NSNumber else { return "\(path): Go is \(go), Swift is \(swift)" }
            return go.doubleValue == swift.doubleValue ? nil : "\(path): Go is \(go), Swift is \(swift)"
        }
        if let go = go as? String {
            guard let swift = swift as? String else { return "\(path): Go is \"\(go)\", Swift is \(swift)" }
            return go == swift ? nil : "\(path): Go is \"\(go)\", Swift is \"\(swift)\""
        }
        return go as? NSNull != nil && swift as? NSNull != nil ? nil : "\(path): Go is \(go), Swift is \(swift)"
    }

    private func builtTool(_ name: String) throws -> URL {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("tenniarb-\(name)", isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let exe = dir.appendingPathComponent(name)
        _ = try runTool(
            goExecutable(), ["build", "-o", exe.path, "./cmd/\(name)"], cwd: repo.appendingPathComponent("go"))
        return exe
    }

    private func goExecutable() throws -> URL {
        for path in ["/opt/homebrew/bin/go", "/usr/local/bin/go", "/usr/bin/go"] where FileManager.default.isExecutableFile(atPath: path) {
            return URL(fileURLWithPath: path)
        }
        throw NSError(domain: "SceneJSONCanvasDocumentTests", code: 1, userInfo: [NSLocalizedDescriptionKey: "no go toolchain"])
    }

    @discardableResult
    private func runTool(_ exe: URL, _ args: [String], cwd: URL? = nil) throws -> Data {
        let process = Process()
        process.executableURL = exe
        process.arguments = args
        if let cwd { process.currentDirectoryURL = cwd }
        let out = Pipe()
        let err = Pipe()
        process.standardOutput = out
        process.standardError = err
        try process.run()
        let stdout = out.fileHandleForReading.readDataToEndOfFile()
        let stderr = err.fileHandleForReading.readDataToEndOfFile()
        process.waitUntilExit()
        guard process.terminationStatus == 0 else {
            throw NSError(
                domain: "SceneJSONCanvasDocumentTests", code: Int(process.terminationStatus),
                userInfo: [
                    NSLocalizedDescriptionKey:
                        "\(exe.lastPathComponent) failed: \(String(decoding: stderr, as: UTF8.self))"
                ])
        }
        return stdout
    }
}
