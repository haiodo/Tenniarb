//
//  DiffTests.swift
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

/// The comparison of go/diff/diff_test.go on scratch texts, and its reading of a committed file.
final class DiffTests: XCTestCase {
    private let card = """
        element "Card" {
          width 240
          item Header {
            title "Header"
            item Mark { title "*" }
          }
          item Body { title "Body" }
        }

        """

    // MARK: - Comparison

    func testTheSameTextReportsNothing() throws {
        let report = try compare(card, card)
        XCTAssertFalse(report.changed)
        XCTAssertEqual(report.paths(.changed), [])
        XCTAssertEqual(report.paths(.added), [])
    }

    func testAChangedPropertyIsAChangedElement() throws {
        let report = try compare(card, try replace(card, "width 240", "width 200"))
        XCTAssertEqual(statuses(report), ["Card": .changed])
        XCTAssertEqual(try XCTUnwrap(report.node("Card")).props, [Diff.Prop(name: "width", old: "240", new: "200")])
        XCTAssertEqual(report.count(.changed), 1)
        XCTAssertFalse(report.paths(.changed).isEmpty)
    }

    func testANestedNodeAndAnAddedOne() throws {
        var newText = try replace(card, #"item Mark { title "*" }"#, #"item Mark { title "+" }"#)
        newText = try replace(
            newText, #"  item Body { title "Body" }"#, "  item Body { title \"Body\" }\n  item Footer { title \"Footer\" }")
        let report = try compare(card, newText)

        XCTAssertEqual(statuses(report), ["Card.Header.Mark": .changed, "Card.Footer": .added])
        let props = try XCTUnwrap(report.node("Card.Header.Mark")).props
        XCTAssertEqual(props, [Diff.Prop(name: "title", old: "*", new: "+")])
        XCTAssertEqual(report.paths(.added), ["Card.Footer"])

        // the same item taken out is removed, with the nested node under it
        let back = try compare(newText, card)
        XCTAssertEqual(statuses(back), ["Card.Footer": .removed, "Card.Header.Mark": .changed])
        XCTAssertEqual(back.paths(.removed), ["Card.Footer"])
    }

    func testANodeThatChangedParentMoved() throws {
        let oldText = """
            element "A" {
              item X { title "x" }
            }
            element "B" {
              item Y { title "y" }
            }

            """
        let newText = """
            element "A" {
            }
            element "B" {
              item Y { title "y" }
              item X { title "x" }
            }

            """
        let report = try compare(oldText, newText)
        XCTAssertEqual(statuses(report), ["B.X": .moved])
        XCTAssertEqual(try XCTUnwrap(report.node("B.X")).note, "from A")
        XCTAssertEqual(report.count(.moved), 1)
    }

    func testANodeThatChangedOrderAmongItsSiblingsMoved() throws {
        let oldText = """
            element "A" {
              item One { title "1" }
              item Two { title "2" }
              item Three { title "3" }
            }

            """
        let newText = """
            element "A" {
              item Three { title "3" }
              item One { title "1" }
              item Two { title "2" }
            }

            """
        let report = try compare(oldText, newText)
        XCTAssertEqual(statuses(report), ["A.Three": .moved])
    }

    func testAddingANodeDoesNotMoveTheOthers() throws {
        let oldText = """
            element "A" {
              item One { title "1" }
              item Two { title "2" }
            }

            """
        let newText = """
            element "A" {
              item Zero { title "0" }
              item One { title "1" }
              item Two { title "2" }
            }

            """
        let report = try compare(oldText, newText)
        XCTAssertEqual(statuses(report), ["A.Zero": .added])
    }

    func testAnAddedElementHoldsItsNodes() throws {
        let oldText = "element \"A\" {\n}\n"
        let newText = """
            element "A" {
            }
            element "B" {
              item X { title "x" }
            }

            """
        var report = try compare(oldText, newText)
        XCTAssertEqual(statuses(report), ["B": .added, "B.X": .added])

        report = try compare(newText, oldText)
        XCTAssertEqual(statuses(report), ["B": .removed, "B.X": .removed])
    }

    func testAUseAndAStyleAreProperties() throws {
        let oldText = """
            element "Button" {
              color #ffffff
              styles {
                primary { color #6b3ad3; text-color #ffffff }
                secondary { color #000000 }
              }
            }
            element "Screen" {
              item Box {
                use Button as save
              }
            }

            """
        var report = try compare(oldText, try replace(oldText, "    use Button as save", "    use Button as save { use-style secondary }"))
        XCTAssertEqual(statuses(report), ["Screen.Box.save": .changed])
        XCTAssertEqual(
            try XCTUnwrap(report.node("Screen.Box.save")).props,
            [Diff.Prop(name: "use-style", old: "", new: "secondary")])

        let styled = try replace(oldText, "primary { color #6b3ad3; text-color #ffffff }", "primary { color #111111; text-color #ffffff }")
        report = try compare(oldText, styled)
        XCTAssertEqual(statuses(report), ["Button": .changed])
        XCTAssertTrue(try XCTUnwrap(report.node("Button")).props.contains { $0.name == "style primary" })
    }

    func testTheRootIsTheFileNameWithoutExtension() throws {
        let root = try workspace()
        let report = try Diff.compare(root: root, rel: "Card.tenn", old: card, new: card)
        XCTAssertEqual(report.root.name, "Card")
    }

    // MARK: - Revert

    func testRevertOfAChangedNodeBringsTheOldTextBack() throws {
        let newText = try replace(card, "width 240", "width 200")
        let report = try compare(card, newText)
        XCTAssertEqual(try apply(report, "Card", to: newText), card)
    }

    func testRevertOfAnAddedNodeCutsItsLine() throws {
        let newText = try replace(
            card, #"  item Body { title "Body" }"#, "  item Body { title \"Body\" }\n  item Footer { title \"Footer\" }")
        let report = try compare(card, newText)
        XCTAssertEqual(statuses(report), ["Card.Footer": .added])
        XCTAssertEqual(try apply(report, "Card.Footer", to: newText), card)
    }

    func testRevertOfARemovedNodeWritesItAfterItsPreviousSibling() throws {
        let newText = try replace(card, "  item Body { title \"Body\" }\n", "")
        let report = try compare(card, newText)
        XCTAssertEqual(statuses(report), ["Card.Body": .removed])
        XCTAssertEqual(try apply(report, "Card.Body", to: newText), card)
    }

    func testRevertOfAnUnchangedNodeHasNoEdit() throws {
        let report = try compare(card, card)
        XCTAssertNil(report.revert("Card"))
        XCTAssertNil(report.revert("Card.Header"))
    }

    func testARemovedChildWaitsForItsRemovedParent() throws {
        let oldText = """
            element "A" {
            }
            element "B" {
              item X { title "x" }
            }

            """
        let newText = "element \"A\" {\n}\n"
        let report = try compare(oldText, newText)
        XCTAssertEqual(statuses(report), ["B": .removed, "B.X": .removed])
        XCTAssertNil(report.revert("B.X"), "the child has no place while its parent is gone")
        XCTAssertEqual(try apply(report, "B", to: newText), oldText)
    }

    // MARK: - git

    func testTheRepositoryAndTheCommittedText() throws {
        let root = try workspace()
        let text = "element \"Card\" {\n  width 240\n}\n"
        try git(root, ["init"])
        try git(root, ["config", "user.email", "test@example.com"])
        try git(root, ["config", "user.name", "Test"])
        try git(root, ["config", "commit.gpgsign", "false"])
        try text.write(to: root.appendingPathComponent("Card.tenn"), atomically: true, encoding: .utf8)
        try git(root, ["add", "Card.tenn"])
        try git(root, ["commit", "-m", "the first version"])

        let repo = try XCTUnwrap(Diff.repo(root), "Repo found nothing")
        XCTAssertEqual(try Diff.show(repo: repo, rev: Diff.revHead, rel: "Card.tenn"), text)

        // the working copy is the new text; the comparison is against the commit
        let edited = try replace(text, "240", "200")
        try edited.write(to: root.appendingPathComponent("Card.tenn"), atomically: true, encoding: .utf8)
        let report = try Diff.compare(
            root: root, rel: "Card.tenn", old: try XCTUnwrap(Diff.show(repo: repo, rel: "Card.tenn")), new: edited)
        XCTAssertEqual(try XCTUnwrap(report.node("Card")).props, [Diff.Prop(name: "width", old: "240", new: "200")])
    }

    func testAFolderOutsideARepositoryHasNoVersion() throws {
        let root = try workspace()
        XCTAssertNil(Diff.repo(root))
        XCTAssertNil(Diff.show(repo: root.path, rev: Diff.revHead, rel: "x.tenn"))
    }

    // MARK: - Helpers

    /// A scratch workspace: the texts load from an overlay, the mounts from the disk.
    private func workspace() throws -> URL {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent("tenniarb-diff-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        return root
    }

    private func compare(_ old: String, _ new: String) throws -> Diff.Report {
        try Diff.compare(root: try workspace(), rel: "x.tenn", old: old, new: new)
    }

    /// The nodes with a status, by path.
    private func statuses(_ report: Diff.Report) -> [String: Diff.Status] {
        var out: [String: Diff.Status] = [:]
        func walk(_ n: Diff.Node) {
            if let status = n.status {
                out[n.path] = status
            }
            for k in n.kids {
                walk(k)
            }
        }
        walk(report.root)
        return out
    }

    /// Swaps the first occurrence of old in text.
    private func replace(_ text: String, _ old: String, _ new: String) throws -> String {
        let range = try XCTUnwrap(text.range(of: old), "\(old) is not in the text")
        return text.replacingCharacters(in: range, with: new)
    }

    /// The text after the revert of path; a report without an edit fails the test.
    private func apply(_ report: Diff.Report, _ path: String, to text: String) throws -> String {
        let revert = try XCTUnwrap(report.revert(path), "the report has no edit for \(path)")
        XCTAssertEqual(revert.file, "x.tenn")
        return try applyEdits(text, [revert.edit])
    }

    /// Runs git in dir; a machine without git skips the test.
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
}
