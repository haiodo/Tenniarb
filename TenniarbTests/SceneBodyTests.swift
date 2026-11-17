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

/// The body of an item: the inline markup, the blocks, heading anchors and a section read from a
/// markdown file, as go/scene/body.go and toc.go.
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

    // MARK: - Anchors and the table of contents

    func testBodyHeadingAnchorsAreNumberedInDocumentOrder() throws {
        let tree = try tree([
            "board.tenn": """
            element "Screen" {
                item "I" {
                    body %{
                        # Цель

                        ## Раздел 2: Итоги!

                        ## Цель

                        ### цель
                    }
                }
            }
            """
        ])
        let screen = try screen(tree)
        let anchors = try body(screen).blocks.filter { $0.kind == .heading }.map(\.anchor)
        XCTAssertEqual(anchors, ["цель", "раздел-2-итоги", "цель-2", "цель-3"])
        XCTAssertEqual(anchorOf("  Пробелы   и   тире  "), "пробелы-и-тире")
        XCTAssertEqual(anchorOf("Ёлка, ёж"), "ёлка-ёж")
        XCTAssertEqual(anchorOf("**Жирный** `код`"), "жирный-код")
        XCTAssertEqual(anchorOf("—"), "")
        var seen: [String: Int] = [:]
        XCTAssertEqual(uniqueAnchor(&seen, ""), "")
        XCTAssertEqual(uniqueAnchor(&seen, "цель"), "цель")
        XCTAssertEqual(uniqueAnchor(&seen, "цель"), "цель-2")
        XCTAssertEqual(uniqueAnchor(&seen, "цель"), "цель-3")
    }

    func testHeadingsOfListsNamesAndAnchors() {
        let (names, anchors) = headingsOf("# План\n\n## Цель ##\n\n## Цель\n\n#### Глубоко")
        XCTAssertEqual(names, ["План", "Цель", "Цель", "Глубоко"])
        XCTAssertEqual(anchors, ["план", "цель", "цель-2", "глубоко"])
    }

    func testBodyTOCListsTheHeadingsBeforeTheBody() throws {
        let board = """
            element "Screen" {
                item "I" {
                    body %{
                        # План

                        текст

                        ## Цель

                        ## Цель

                        ### Дальше
                    }
                    toc 2
                }
            }
            """
        let loaded = try tree(["board.tenn": board])
        let pageScreen = try screen(loaded)
        let page = try body(pageScreen)
        let first = try XCTUnwrap(page.blocks.first)
        XCTAssertEqual(first.kind, .toc)
        XCTAssertEqual(first.toc.map(\.level), [1, 2, 2])
        XCTAssertEqual(first.toc.map { runsText($0.runs) }, ["План", "Цель", "Цель"])
        let lines = page.lines(1000)
        XCTAssertEqual(lines.prefix(3).map(\.x), [0, Scene.bodyListIndent, Scene.bodyListIndent])
        XCTAssertEqual(lines[1].runs[0].font.size, 14)  // body 16 - 2
        let plainTree = try tree(["board.tenn": board.replacingOccurrences(of: "toc 2", with: "")])
        let plainScreen = try screen(plainTree)
        XCTAssertGreaterThan(page.height(1000), try body(plainScreen).height(1000))
        let noHeadings = try tree(["board.tenn": "element \"Screen\" { item \"I\" { body \"текст\"; toc 3 } }"])
        let noHeadingsScreen = try screen(noHeadings)
        XCTAssertNotEqual(try body(noHeadingsScreen).blocks.first?.kind, .toc)
    }

    func testTOCDepthMustBeAHeadingLevel() throws {
        let range = try tree([
            "board.tenn": """
            element "Screen" {
                item "I" { body "# Цель"; toc 9 }
            }
            """
        ])
        _ = range.build("Screen", measurer: Scene.TextMeasurer(estimated: true))
        XCTAssertEqual(range.diagnostics.map(\.message), ["toc takes a depth from 1 to 6, got \"9\""])
        let word = try tree([
            "board.tenn": """
            element "Screen" {
                item "I" { body "# Цель"; toc deep }
            }
            """
        ])
        _ = word.build("Screen", measurer: Scene.TextMeasurer(estimated: true))
        XCTAssertEqual(word.diagnostics.map(\.message), ["toc takes a depth from 1 to 6, got \"deep\""])
    }

    // MARK: - Links

    func testBodyLinksSkipCodeAndImages() {
        let text = "[a](one.md#A) `[b](two.md#B)`\n\n```\n[c](three.md#C)\n```\n\n![pic](four.md#F) [d](https://x#D) [e](#E)"
        XCTAssertEqual(bodyLinks(text), ["one.md#A", "https://x#D", "#E"])
    }

    func testBodyLinkToAMissingHeadingIsReported() throws {
        let tree = try tree([
            "board.tenn": """
            element "Screen" {
                item "I" {
                    body %{
                        # Цель

                        [ok](#Цель) [bad](#Дальше) [doc](doc.md#Раздел) [gone](nope.md#A)
                    }
                }
            }
            """,
            "doc.md": "# Год\n\n## Раздел\n\nтекст\n",
        ])
        _ = tree.build("Screen", measurer: Scene.TextMeasurer(estimated: true))
        XCTAssertEqual(
            tree.diagnostics.map(\.message),
            [
                "link \"#Дальше\": no section \"Дальше\" in board.tenn (headings: Цель)",
                "link \"nope.md#A\": no such file nope.md",
            ])
    }
}
