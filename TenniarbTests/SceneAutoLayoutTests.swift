//
//  SceneAutoLayoutTests.swift
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

final class SceneAutoLayoutTests: XCTestCase {

    private func layout(_ text: String) throws -> (tree: Scene.Tree, root: Scene.LayoutNode) {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-auto-layout-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        let file = root.appendingPathComponent("board.tenn")
        try text.write(to: file, atomically: true, encoding: .utf8)

        let tree = try SceneLoader(root: root).load("board.tenn")
        let screen = try XCTUnwrap(tree.main?.elems.first)
        let box = Scene.LayoutEngine(tree: tree).layout(screen)
        box.layout()
        return (tree, box)
    }

    func testRowFillSharesFreeWidthAndCrossAxis() throws {
        let (tree, root) = try layout(
            """
            element "Screen" {
                item "Row" {
                    auto-layout row
                    gap 4
                    padding 10 20
                    width 300
                    height 100
                    item "A" {
                        width fill
                        height fill
                    }
                    item "B" {
                        width 50
                        height fill
                    }
                }
            }
            """)

        XCTAssertTrue(tree.diagnostics.isEmpty, "\(tree.diagnostics)")
        let row = try XCTUnwrap(root.kids.first)
        XCTAssertEqual(row.rw, 300, accuracy: 1e-9)
        XCTAssertEqual(row.rh, 100, accuracy: 1e-9)
        XCTAssertEqual(row.padding.left, 20, accuracy: 1e-9)
        XCTAssertEqual(row.padding.right, 20, accuracy: 1e-9)
        XCTAssertEqual(row.padding.top, 10, accuracy: 1e-9)

        let a = try XCTUnwrap(row.kids.first)
        XCTAssertEqual(a.rw, 206, accuracy: 1e-9, "300 - 40 padding - 4 gap - 50 fixed")
        XCTAssertEqual(a.rh, 80, accuracy: 1e-9)
        XCTAssertEqual(a.rx, 20, accuracy: 1e-9)
        XCTAssertEqual(a.ry, 10, accuracy: 1e-9)

        let b = try XCTUnwrap(row.kids.last)
        XCTAssertEqual(b.rw, 50, accuracy: 1e-9)
        XCTAssertEqual(b.rx, 230, accuracy: 1e-9)
    }

    func testHugRowAndColumnSizeFromFixedChildren() throws {
        let (tree, root) = try layout(
            """
            element "Screen" {
                item "Row" {
                    auto-layout row
                    gap 5
                    padding 2
                    item "A" {
                        width 30
                        height 10
                    }
                    item "B" {
                        width 40
                        height 20
                    }
                }
                item "Col" {
                    auto-layout column
                    gap 5
                    padding 2
                    item "C" {
                        width 30
                        height 10
                    }
                    item "D" {
                        width 40
                        height 20
                    }
                }
            }
            """)

        XCTAssertTrue(tree.diagnostics.isEmpty, "\(tree.diagnostics)")
        let row = try XCTUnwrap(root.kids.first)
        XCTAssertEqual(row.rw, 79, accuracy: 1e-9, "2 + 30 + 5 + 40 + 2")
        XCTAssertEqual(row.rh, 24, accuracy: 1e-9, "2 + 20 + 2")
        let rowSecond = try XCTUnwrap(row.kids.last)
        XCTAssertEqual(rowSecond.rx, 37, accuracy: 1e-9)

        let column = try XCTUnwrap(root.kids.last)
        XCTAssertEqual(column.rw, 44, accuracy: 1e-9, "2 + 40 + 2")
        XCTAssertEqual(column.rh, 39, accuracy: 1e-9, "2 + 10 + 5 + 20 + 2")
        let columnSecond = try XCTUnwrap(column.kids.last)
        XCTAssertEqual(columnSecond.ry, 17, accuracy: 1e-9)
    }

    func testPosIsTopLeftWithYUpAndAlignJustifyPlaceChildren() throws {
        let (tree, root) = try layout(
            """
            element "Screen" {
                item "Box" {
                    item "Pin" {
                        absolute yes
                        pos 5 7
                        width 10
                        height 10
                    }
                }
                item "Row" {
                    auto-layout row
                    width 100
                    height 50
                    align center
                    justify end
                    item "A" {
                        width 20
                        height 10
                    }
                    item "B" {
                        width 10
                        height 10
                    }
                }
            }
            """)

        XCTAssertTrue(tree.diagnostics.isEmpty, "\(tree.diagnostics)")
        let box = try XCTUnwrap(root.kids.first)
        let pin = try XCTUnwrap(box.kids.first)
        XCTAssertEqual(pin.rx, 5, accuracy: 1e-9)
        XCTAssertEqual(pin.ry, -7, accuracy: 1e-9, ".tenn has y up")

        let row = try XCTUnwrap(root.kids.last)
        let a = try XCTUnwrap(row.kids.first)
        let b = try XCTUnwrap(row.kids.last)
        XCTAssertEqual(a.rx, 70, accuracy: 1e-9, "justify end: 100 - 30")
        XCTAssertEqual(b.rx, 90, accuracy: 1e-9)
        XCTAssertEqual(a.ry, 20, accuracy: 1e-9, "align center: (50 - 10) / 2")
    }
}
