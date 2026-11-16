//
//  SceneBuildTests.swift
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

/// The placed tree of an expanded element: text sizes, padding, fill, wrap and limits, as go/scene/layout.go.
final class SceneBuildTests: XCTestCase {

    /// A workspace on disk: the loader reads mounts from it, as go/scene does.
    private func tree(_ files: [String: String], _ main: String = "board.tenn") throws -> Scene.Tree {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-scene-build-\(UUID().uuidString)", isDirectory: true)
        for (path, text) in files {
            let url = root.appendingPathComponent(path)
            try FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
            try text.write(to: url, atomically: true, encoding: .utf8)
        }
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        return try SceneLoader(root: root).load(main)
    }

    func testRowOfTextItemsHugsTheirTextWithinOnePoint() throws {
        let tree = try tree([
            "board.tenn": """
            element "Screen" {
                item "Row" {
                    auto-layout row
                    gap 2
                    item "A" { title "A" }
                    item "B" { title "Hello" }
                }
            }
            """
        ])
        XCTAssertTrue(tree.diagnostics.isEmpty, "\(tree.diagnostics)")
        let row = try XCTUnwrap(tree.layout("Screen")?.kids.first)
        let a = try XCTUnwrap(row.kids.first)
        let b = try XCTUnwrap(row.kids.last)

        // Helvetica 18: "A" is 12.01 points wide, "Hello" 41.02; a scene rounds a leaf up to whole points.
        XCTAssertEqual(a.rw, 13, accuracy: 1)
        XCTAssertEqual(b.rw, 42, accuracy: 1)
        XCTAssertEqual(a.rh, 18, accuracy: 1)
        XCTAssertEqual(row.rw, 57, accuracy: 1, "13 + 2 gap + 42")
        XCTAssertEqual(b.rx, 15, accuracy: 1)
    }

    func testPaddingInsetsTheTextOfALeafWithinOnePoint() throws {
        let tree = try tree([
            "board.tenn": """
            element "Screen" {
                item "Box" {
                    item "A" {
                        padding 2 4
                        title "Hi"
                    }
                }
            }
            """
        ])
        XCTAssertTrue(tree.diagnostics.isEmpty, "\(tree.diagnostics)")
        let a = try XCTUnwrap(tree.layout("Screen")?.kids.first?.kids.first)

        XCTAssertEqual(a.text?.padX ?? -1, 4, accuracy: 1e-9, "padding 2 4: left and right")
        XCTAssertEqual(a.text?.padY ?? -1, 2, accuracy: 1e-9, "padding 2 4: top and bottom")
        XCTAssertEqual(a.rw, 25, accuracy: 1, "ceil(16.998) + 2 * 4")
        XCTAssertEqual(a.rh, 22, accuracy: 1, "line 18 + 2 * 2")
    }

    func testFillTextItemSharesFreeWidthWithinOnePoint() throws {
        let tree = try tree([
            "board.tenn": """
            element "Screen" {
                item "Row" {
                    auto-layout row
                    width 200
                    padding 5
                    gap 5
                    item "A" { title "A" }
                    item "B" { width fill }
                }
            }
            """
        ])
        XCTAssertTrue(tree.diagnostics.isEmpty, "\(tree.diagnostics)")
        let row = try XCTUnwrap(tree.layout("Screen")?.kids.first)
        let a = try XCTUnwrap(row.kids.first)
        let b = try XCTUnwrap(row.kids.last)

        XCTAssertEqual(a.rw, 13, accuracy: 1)
        XCTAssertEqual(b.rw, 172, accuracy: 1, "200 - 10 padding - 5 gap - 13 hug")
        XCTAssertEqual(b.rx, 23, accuracy: 1, "5 padding + 13 + 5 gap")
        XCTAssertEqual(row.rh, 28, accuracy: 1, "text line 18 + 2 * 5")
    }

    func testLongTitleWrapsInFixedWidthOf60PointsWithinOnePoint() throws {
        let tree = try tree([
            "board.tenn": """
            element "Screen" {
                item "Box" {
                    item "T" {
                        width 60
                        title "aaaa bbbb cccc"
                    }
                }
            }
            """
        ])
        XCTAssertTrue(tree.diagnostics.isEmpty, "\(tree.diagnostics)")
        let t = try XCTUnwrap(tree.layout("Screen")?.kids.first?.kids.first)

        XCTAssertEqual(t.rw, 60, accuracy: 1e-9)
        // 60 wide, 10 padding each side: "aaaa" alone is 40.04 and does not fit 40 with a space and "bbbb".
        XCTAssertEqual(t.text?.wrap(40).count, 3)
        XCTAssertEqual(t.rh, 66, accuracy: 1, "3 lines * 18 + 2 * 6")
    }

    func testMinAndMaxWidthClampAHugWithinOnePoint() throws {
        let tree = try tree([
            "board.tenn": """
            element "Screen" {
                item "Box" {
                    auto-layout column
                    item "Wide" {
                        max-width 50
                        title "Wide text here"
                    }
                    item "Narrow" {
                        min-width 100
                        title "A"
                    }
                }
            }
            """
        ])
        XCTAssertTrue(tree.diagnostics.isEmpty, "\(tree.diagnostics)")
        let box = try XCTUnwrap(tree.layout("Screen")?.kids.first)
        let wide = try XCTUnwrap(box.kids.first)
        let narrow = try XCTUnwrap(box.kids.last)

        XCTAssertEqual(wide.rw, 50, accuracy: 1e-9, "hug 117 clamped by max-width")
        XCTAssertEqual(wide.rh, 54, accuracy: 1, "three words wrapped at 50")
        XCTAssertEqual(narrow.rw, 100, accuracy: 1e-9, "hug 13 raised by min-width")
        XCTAssertEqual(narrow.rh, 18, accuracy: 1, "one line of 18")
    }

    func testUseInstanceIsPlacedAtTheUsePosWithItsChildrenInside() throws {
        let tree = try tree([
            "board.tenn": """
            element "Kit" --from-file "lib/kit.tenn"
            element "Screen" {
                use Kit.Card as card {
                    pos 30 40
                }
            }
            """,
            "lib/kit.tenn": """
            item "Card" {
                pos 5 5
                width 100
                height 40
                item "In" {
                    width 20
                    height 20
                }
            }
            """,
        ])
        XCTAssertTrue(tree.diagnostics.isEmpty, "\(tree.diagnostics)")
        let card = try XCTUnwrap(tree.layout("Screen")?.kids.first)
        let inside = try XCTUnwrap(card.kids.first)

        XCTAssertEqual(card.rx, 30, accuracy: 1e-9, "the pos of the use, not the 5 of the component")
        XCTAssertEqual(card.ry, -40, accuracy: 1e-9, "y is up: screen y = -pos.y")
        XCTAssertEqual(card.rw, 100, accuracy: 1e-9)
        XCTAssertEqual(card.rh, 40, accuracy: 1e-9)
        XCTAssertEqual(inside.rx, 0, accuracy: 1e-9)
        XCTAssertEqual(inside.ry, 0, accuracy: 1e-9)
        XCTAssertEqual(inside.rw, 20, accuracy: 1e-9)
        XCTAssertEqual(inside.rh, 20, accuracy: 1e-9)
    }

    func testMeasureTextRoundsItsLinesUpWithinOnePoint() {
        let m = Scene.TextMeasurer()
        let one = measureText(m, "Hello", 18)
        XCTAssertEqual(one.w, 42, accuracy: 1, "ceil(41.019)")
        XCTAssertEqual(one.h, 18, accuracy: 1, "ascent 13.86 + descent 4.14")

        let two = measureText(m, "Hi\\nHo", 18)
        XCTAssertEqual(two.w, 24, accuracy: 1, "ceil(23.010)")
        XCTAssertEqual(two.h, 36, accuracy: 1)
    }

    func testAnElementWithAnErrorHasNoPlacedTree() throws {
        let tree = try tree([
            "board.tenn": """
            element "Screen" {
                item "Row" {
                    auto-layout row
                    item "A" { padding 1 2 3 }
                }
            }
            """
        ])
        XCTAssertNil(tree.layout("Screen"))
        XCTAssertTrue(tree.diagnostics.contains { !$0.warn }, "\(tree.diagnostics)")
    }
}
