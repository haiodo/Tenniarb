//
//  CompareTests.swift
//  TenniarbTests
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.  See the
//  License for the specific language governing permissions and limitations under
//  the License.
//

import XCTest

@testable import Tenniarb

/// Compare mode of a scene document: the last commit against the text the window holds. The reports are
/// built without windows; a document is closed unedited by the shared support.
@MainActor
final class CompareTests: SceneDocumentTestCase {
    private let screen = """
        element "Screen" {
          width 240
          item Window { auto-layout column }
        }

        """
    private let editedScreen = """
        element "Screen" {
          width 200
          item Window { auto-layout column }
        }

        """

    func testStartBuildsTheReportOfTheLastCommit() throws {
        let root = try workspace(["Screen.tenn": screen])
        try commit(root, "Screen.tenn")
        try editedScreen.write(to: root.appendingPathComponent("Screen.tenn"), atomically: true, encoding: .utf8)
        let document = try openTestDocument(root.appendingPathComponent("Screen.tenn"))

        let compare = CompareController(document: document)
        XCTAssertTrue(compare.start())
        let report = try XCTUnwrap(compare.report)
        XCTAssertEqual(report.count(.changed), 1)
        XCTAssertEqual(compare.changedPaths(), ["Screen"])
        XCTAssertEqual(compare.summary(), "Against HEAD: 1 changed")
        XCTAssertTrue(compare.frames().contains { $0.path == "Screen" && $0.color == CompareController.color(.changed) })

        let labels = labels(compare.rows())
        XCTAssertTrue(labels.contains("Screen  changed  1"), "\(labels)")
        XCTAssertTrue(labels.contains("width  240 -> 200"), "\(labels)")

        compare.close()
        XCTAssertFalse(compare.isOn)
        XCTAssertTrue(compare.rows().isEmpty)
        XCTAssertEqual(compare.summary(), "")
    }

    func testADocumentOutsideARepositoryHasNoComparison() throws {
        let root = try workspace(["Screen.tenn": screen])
        guard Diff.repo(root) == nil else {
            throw XCTSkip("the scratch folder is inside a repository")
        }
        let document = try openTestDocument(root.appendingPathComponent("Screen.tenn"))

        let compare = CompareController(document: document)
        XCTAssertFalse(compare.start())
        XCTAssertFalse(compare.isOn)
        XCTAssertEqual(compare.message, "The file is not in a git repository: there is no version to compare with")
        XCTAssertTrue(compare.rows().isEmpty)
    }

    func testAnUntrackedFileHasNoVersionAtHead() throws {
        let root = try workspace(["Screen.tenn": screen])
        try git(root, ["init"])
        try git(root, ["config", "user.email", "test@example.com"])
        try git(root, ["config", "user.name", "Test"])
        let document = try openTestDocument(root.appendingPathComponent("Screen.tenn"))

        let compare = CompareController(document: document)
        XCTAssertFalse(compare.start())
        XCTAssertFalse(compare.isOn)
        XCTAssertTrue(try XCTUnwrap(compare.message).contains("has no version at HEAD"), compare.message ?? "")
    }

    func testRefreshComparesTheEditedTextAgain() throws {
        let root = try workspace(["Screen.tenn": screen])
        try commit(root, "Screen.tenn")
        let url = root.appendingPathComponent("Screen.tenn")
        let document = try openTestDocument(url)

        let compare = CompareController(document: document)
        XCTAssertTrue(compare.start())
        XCTAssertEqual(compare.summary(), "No changes against HEAD")

        let text = try XCTUnwrap(document.sceneText)
        let range = try XCTUnwrap(text.range(of: "240"))
        let start = text.utf8.distance(from: text.startIndex, to: range.lowerBound)
        try document.applySceneEdit(
            Scene.TextEdit(start: start, end: start + "240".utf8.count, new: "200"), in: "Screen.tenn")

        compare.refresh()
        XCTAssertEqual(try XCTUnwrap(compare.report).count(.changed), 1)
        XCTAssertEqual(compare.summary(), "Against HEAD: 1 changed")
        XCTAssertEqual(compare.changedPaths(), ["Screen"])
    }

    func testAChangedItemIsNamedFromItsElement() throws {
        let root = try workspace(["Screen.tenn": screen])
        try commit(root, "Screen.tenn")
        let edited = try replace(screen, "item Window { auto-layout column }", "item Window { auto-layout column; gap 10 }")
        try edited.write(to: root.appendingPathComponent("Screen.tenn"), atomically: true, encoding: .utf8)
        let document = try openTestDocument(root.appendingPathComponent("Screen.tenn"))

        let compare = CompareController(document: document)
        XCTAssertTrue(compare.start())
        XCTAssertEqual(compare.changedPaths(), ["Screen.Window"])
        let node = try XCTUnwrap(try XCTUnwrap(compare.report).node("Screen.Window"))
        XCTAssertEqual(node.element, "Screen")
        XCTAssertEqual(node.sel, "Window")
        XCTAssertTrue(compare.frames().contains { $0.path == "Screen.Window" })
        XCTAssertNotNil(node.new, "the node of the current version is where a row selects it")
    }

    // MARK: - Helpers

    /// A scratch workspace with the files written; the suite removes it after the documents are closed.
    private func workspace(_ files: [String: String]) throws -> URL {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent("tenniarb-compare-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        for (path, text) in files {
            let url = root.appendingPathComponent(path)
            try FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
            try text.write(to: url, atomically: true, encoding: .utf8)
        }
        return root
    }

    /// Makes root a repository with the committed file name; a machine without git skips the test.
    private func commit(_ root: URL, _ name: String) throws {
        try git(root, ["init"])
        try git(root, ["config", "user.email", "test@example.com"])
        try git(root, ["config", "user.name", "Test"])
        try git(root, ["config", "commit.gpgsign", "false"])
        try git(root, ["add", name])
        try git(root, ["commit", "-m", "the first version"])
    }

    private func git(_ dir: URL, _ args: [String]) throws {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/env")
        process.arguments = ["git", "-C", dir.path] + args
        process.standardOutput = FileHandle.nullDevice
        let errors = Pipe()
        process.standardError = errors
        do {
            try process.run()
        } catch {
            throw XCTSkip("git is not installed")
        }
        process.waitUntilExit()
        if process.terminationStatus != 0 {
            let text = String(decoding: errors.fileHandleForReading.readDataToEndOfFile(), as: UTF8.self)
            throw XCTSkip("git \(args.joined(separator: " ")) failed: \(text)")
        }
    }

    /// The labels of the rows, the nested ones after their parent.
    private func labels(_ rows: [CompareController.Row]) -> [String] {
        rows.flatMap { [$0.label] + labels($0.kids) }
    }

    /// Swaps the first occurrence of old in text.
    private func replace(_ text: String, _ old: String, _ new: String) throws -> String {
        let range = try XCTUnwrap(text.range(of: old), "\(old) is not in the text")
        return text.replacingCharacters(in: range, with: new)
    }
}
