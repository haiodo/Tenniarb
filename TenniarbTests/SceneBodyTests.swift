//
//  SceneBodyTests.swift
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

/// The body of an item: the inline markup, the blocks and a section read from a markdown file, as
/// go/scene/body.go.
final class SceneBodyTests: XCTestCase {

    /// A workspace on disk: the loader reads mounts and bodies from it, as go/scene does.
    private func tree(_ files: [String: String], _ main: String = "board.tenn") throws -> Scene.Tree {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-scene-body-\(UUID().uuidString)", isDirectory: true)
        for (path, text) in files {
            let url = root.appendingPathComponent(path)
            try FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
            try text.write(to: url, atomically: true, encoding: .utf8)
        }
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        return try SceneLoader(root: root).load(main)
    }

    /// The screen of the element; it keeps the build, and so the body, alive.
    private func screen(_ tree: Scene.Tree, _ element: String = "Screen") throws -> Scene.Screen {
        try XCTUnwrap(tree.build(element, measurer: Scene.TextMeasurer(estimated: true)))
    }

    /// The body of the first item of the screen.
    private func body(_ screen: Scene.Screen) throws -> Scene.BodyText {
        let item = try XCTUnwrap(screen.tree.kids.first)
        return try XCTUnwrap(screen.p.bodies[ObjectIdentifier(item)])
    }

    func testBodyInlineCarriesTheLookOfItsMarkup() throws {
        let tree = try tree([
            "board.tenn": """
            element "Screen" {
                item "I" { body "A **bold** *it* `code` [link](http://x)" }
            }
            """
        ])
        let screen = try screen(tree)
        let line = try XCTUnwrap(try body(screen).lines(.infinity).first)
        XCTAssertEqual(line.runs.map(\.text), ["A ", "bold", " ", "it", " ", "code", " ", "link"])
        XCTAssertEqual(line.runs[1].font.weight, 700)
        XCTAssertTrue(line.runs[3].font.italic)
        XCTAssertTrue(line.runs[5].code)
        XCTAssertEqual(line.runs[5].font.family, "Menlo")
        XCTAssertEqual(line.runs[7].link, "http://x")
    }

    func testBodyFromFileReadsTheSectionUnderItsHeading() throws {
        let tree = try tree([
            "board.tenn": """
            element "Screen" {
                item "Page" { body --from-file "doc.md#Goal" { font-size 14 } }
            }
            """,
            "doc.md": """
            # Title

            Intro.

            ## Goal

            The text of the section.

            ## Next

            Other.
            """,
        ])
        XCTAssertTrue(tree.diagnostics.isEmpty, "\(tree.diagnostics)")
        let screen = try screen(tree)
        let body = try body(screen)
        XCTAssertEqual(body.font.size, 14)
        // The section is the heading line and what is below it, as a reader of the file sees it.
        XCTAssertEqual(body.lines(.infinity).map { $0.runs.map(\.text).joined() }, ["Goal", "The text of the section."])
    }

    func testAClassicBodyBreaksItsLinesAsWritten() throws {
        let tree = try tree([
            "board.tenn": """
            element "Screen" {
                item "I" { title "T"; body "one\\ntwo" }
            }
            """
        ])
        let screen = try screen(tree)
        XCTAssertTrue(screen.classic, "a file with no auto-layout is a classic diagram")
        XCTAssertEqual(
            try body(screen).lines(.infinity).map { $0.runs.map(\.text).joined() }, ["one", "two"])
    }
}
