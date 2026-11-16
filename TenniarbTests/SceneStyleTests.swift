//
//  SceneStyleTests.swift
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

final class SceneStylesTests: XCTestCase {

    /// A workspace on disk, one file.
    private func tree(_ text: String) throws -> Scene.Tree {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-scene-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        try text.write(to: root.appendingPathComponent("s.tenn"), atomically: true, encoding: .utf8)
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        return try SceneLoader(root: root).load("s.tenn")
    }

    func testStyleNamedInOneOfGroupApplies() throws {
        let tree = try tree(
            """
            element "Screen" {
                styles {
                    primary { color #6b3ad3 }
                    secondary { color #ffffff }
                    look one-of primary secondary
                }
                item "A" { use-style primary }
                item "B" { use-style secondary }
            }
            """)
        let element = try XCTUnwrap(tree.main?.elems.first)
        XCTAssertNil(element.styles?["look"], "a group is no style")
        XCTAssertEqual(element.styleGroups.first?.name, "look")
        XCTAssertEqual(element.styleGroups.first?.anyOf, false)
        XCTAssertEqual(element.styleGroups.first?.styles, ["primary", "secondary"])

        let screen = try XCTUnwrap(tree.element("Screen"))
        applyStyles(screen)
        let a = try XCTUnwrap(screen.kids.first)
        let b = try XCTUnwrap(screen.kids.last)
        XCTAssertEqual(a.prop("color")?.text(0), "#6b3ad3", "the style of the group the node names wins")
        XCTAssertEqual(b.prop("color")?.text(0), "#ffffff")
        XCTAssertEqual(a.props.filter { $0.name == "color" }.count, 1)
    }

    func testStyleOrderAroundUseStyle() throws {
        let tree = try tree(
            """
            element "E" {
                styles { wide { width 48 } }
                item "A" { width 20; use-style wide }
                item "B" { use-style wide; width 20 }
            }
            """)
        let screen = try XCTUnwrap(tree.element("E"))
        applyStyles(screen)
        let a = try XCTUnwrap(screen.kids.first)
        let b = try XCTUnwrap(screen.kids.last)
        XCTAssertEqual(a.prop("width")?.num(0), 48, "a property above use-style is a default the style overrides")
        XCTAssertEqual(b.prop("width")?.num(0), 20, "a property below use-style wins over the style")
    }

    func testKidBlockAndFontInheritance() throws {
        let tree = try tree(
            """
            element "E" {
                styles {
                    wide_label { Label { width 200 } }
                    big { font-size 20 }
                }
                item "F" { use-style wide_label; item "Label" { title "x" } }
                item "Box" { use-style big; item "Text" { title "y" } }
            }
            """)
        let screen = try XCTUnwrap(tree.element("E"))
        applyStyles(screen)
        let f = try XCTUnwrap(screen.kids.first)
        XCTAssertEqual(f.kids.first?.prop("width")?.num(0), 200, "a kid block of a style reaches the kid")

        let box = try XCTUnwrap(screen.kids.last)
        XCTAssertEqual(box.prop("font-size")?.num(0), 20)
        XCTAssertEqual(box.kids.first?.prop("font-size")?.num(0), 20, "an item takes the font of the box it is in")
    }
}
