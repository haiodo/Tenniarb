//
//  SceneResolveTests.swift
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

final class SceneResolveTests: XCTestCase {

    /// A workspace on disk; the loader reads mounts from it, as go/scene does.
    private func workspace(_ files: [String: String]) throws -> URL {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-scene-\(UUID().uuidString)", isDirectory: true)
        for (path, text) in files {
            let url = root.appendingPathComponent(path)
            try FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
            try text.write(to: url, atomically: true, encoding: .utf8)
        }
        return root
    }

    func testSplitPathReadsQuotedSegments() throws {
        let segs = try XCTUnwrap(splitPath(#"Kit."a b".x"#))
        XCTAssertEqual(segs.map(\.name), ["Kit", "a b", "x"])
        XCTAssertEqual(segs.map(\.quoted), [false, true, false])
        XCTAssertEqual(segs[1].key, "a_b", "a quoted segment is named as written and keyed normalized")
        XCTAssertEqual(segs[1].text, #""a b""#)

        let escaped = try XCTUnwrap(splitPath(#""say \"hi\"""#))
        XCTAssertEqual(escaped.count, 1)
        XCTAssertEqual(escaped[0].name, #"say "hi""#)
        XCTAssertTrue(escaped[0].quoted)

        for bad in ["", "a.", ".a", "a..b", #""a"b"#, #""open"#, #""a"."#] {
            XCTAssertNil(splitPath(bad), "path \(bad) must not split")
        }
    }

    func testQuotedSegmentNamesTheNodeAsWritten() throws {
        let root = try workspace([
            "q.tenn": """
            element "Kit" {
                item "a b" { width 1 }
                item "a_b" { width 2 }
            }
            element "Screen" {
                use Kit."a b" as one
                use Kit."a_b" as two
            }
            """
        ])
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }

        let tree = try SceneLoader(root: root).load("q.tenn")
        let file = try XCTUnwrap(tree.main)
        XCTAssertEqual(try tree.resolve(file, #"Kit."a b""#).name, "a b")
        XCTAssertEqual(try tree.resolve(file, #"Kit."a_b""#).name, "a_b")
        XCTAssertEqual(try tree.resolve(file, "Kit").name, "Kit", "a bare segment names the key")
    }

    func testAmbiguousKeyIsTheGoDiagnostic() throws {
        let root = try workspace([
            "d.tenn": """
            element "Kit" {
                item "a b" { width 1 }
                item "a_b" { width 2 }
            }
            element "Screen" { use Kit.a_b as x }
            """
        ])
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }

        let tree = try SceneLoader(root: root).load("d.tenn")
        let file = try XCTUnwrap(tree.main)
        XCTAssertThrowsError(try tree.resolve(file, "Kit.a_b")) { error in
            XCTAssertEqual(
                "\(error)", #"ambiguous path Kit.a_b: candidates "a b" (d.tenn:2:5), "a_b" (d.tenn:3:5)"#)
        }
    }

    func testNameThatDoesNotResolveIsTheGoDiagnostic() throws {
        let root = try workspace([
            "m.tenn": """
            element "Screen" {
                use Missing as x
            }
            """
        ])
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }

        let tree = try SceneLoader(root: root).load("m.tenn")
        let screen = try XCTUnwrap(tree.element("Screen"))
        XCTAssertTrue(screen.kids.isEmpty)
        let diag = try XCTUnwrap(tree.diagnostics.first)
        XCTAssertEqual(diag.message, #"path Missing: "Missing" is not defined here or above"#)
        XCTAssertEqual(diag.pos, Scene.Pos(file: "m.tenn", line: 2, col: 5))
        XCTAssertFalse(diag.warn)
    }

    func testLookupClimbsOutOfAMountedFileToTheMountPoint() throws {
        let root = try workspace([
            "main.tenn": """
            element "Kit" --from-file "lib/kit.tenn"
            element "Screen" {
                use Kit.Card as card
            }
            item "Accent" { width 3; height 3 }
            """,
            "lib/kit.tenn": """
            item "Card" {
                use Accent as accent
            }
            """,
        ])
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }

        let tree = try SceneLoader(root: root).load("main.tenn")
        let screen = try XCTUnwrap(tree.element("Screen"))
        let card = try XCTUnwrap(screen.kids.first)
        XCTAssertEqual(card.name, "card")
        XCTAssertTrue(card.inst)

        let accent = try XCTUnwrap(card.kids.first)
        XCTAssertEqual(accent.name, "accent", "a name missing in the mounted file is looked up where it is mounted")
        XCTAssertEqual(accent.def?.name, "Accent")
        XCTAssertTrue(tree.diagnostics.isEmpty, "\(tree.diagnostics)")
    }
}
