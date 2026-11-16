//
//  SceneImageTests.swift
//  TenniarbTests
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//  WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.  See the
//  License for the specific language governing permissions and limitations under
//  the License.
//

import XCTest

@testable import Tenniarb

/// Pictures of items and the inline `@(name)` of a title and a body, as go/scene/image.go.
final class SceneImageTests: XCTestCase {

    /// The picture of web/fixtures/scene/classic-image.tenn: a PNG of 8 by 4 pixels.
    private let logo = """
        iVBORw0KGgoAAAANSUhEUgAAAAgAAAAECAYAAACzzX7wAAAABHNCSVQICAgIfAhkiAAAAAFzUkdCAK7OHOkAAAAVSURBVAiZ\
        YzROO/OfAQ9gwidJHQUANtQCbKJNLnwAAAAASUVORK5CYII=
        """

    /// A workspace on disk: the loader reads mounts and pictures from it, as go/scene does.
    private func tree(_ text: String) throws -> Scene.Tree {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-scene-image-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        try text.write(to: root.appendingPathComponent("board.tenn"), atomically: true, encoding: .utf8)
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        return try SceneLoader(root: root).load("board.tenn")
    }

    func testAnInlineImageInATitleGrowsTheBoxByItsSize() throws {
        let tree = try tree(
            """
            element "Screen" {
                item "Card" { title "@(logo)"; image "logo" @(\(logo)) }
            }
            """)
        XCTAssertTrue(tree.diagnostics.isEmpty, "\(tree.diagnostics)")
        let screen = try XCTUnwrap(tree.build("Screen", measurer: Scene.TextMeasurer(estimated: true)))
        let card = try XCTUnwrap(screen.placed().kids.first)
        let ti = try XCTUnwrap(screen.p.texts[ObjectIdentifier(screen.tree.kids[0])])

        // EstimateText at 18 points: the picture 8 wide and the space go/render keeps beside it 10.8,
        // inset by the 4 points of a classic leaf; the 4-point picture stays under the 22-point line.
        XCTAssertTrue(ti.parts?.contains { $0.img != nil } ?? false, "the title draws the picture")
        XCTAssertEqual(card.w, 26.8, accuracy: 1e-9)
        XCTAssertEqual(card.h, 30, accuracy: 1e-9)
    }

    func testAnUnnamedPictureIsTheSizeOfTheBox() throws {
        let tree = try tree(
            """
            element "Screen" {
                item "Pic" { title ""; image "logo" @(\(logo)) }
            }
            """)
        XCTAssertTrue(tree.diagnostics.isEmpty, "\(tree.diagnostics)")
        let screen = try XCTUnwrap(tree.build("Screen", measurer: Scene.TextMeasurer(estimated: true)))
        let pic = try XCTUnwrap(screen.placed().kids.first)
        XCTAssertEqual(pic.w, 8, accuracy: 1e-9)
        XCTAssertEqual(pic.h, 4, accuracy: 1e-9)
    }

    func testAMissingFileAndBrokenDataAreWarnings() throws {
        let tree = try tree(
            """
            element "Screen" {
                item "A" { title ""; image "nope.png" }
                item "B" { title ""; image "logo" @(AAAA) }
            }
            """)
        let screen = try XCTUnwrap(tree.build("Screen", measurer: Scene.TextMeasurer(estimated: true)))
        let messages = tree.diagnostics.map(\.message)
        XCTAssertTrue(
            messages.contains("image \"nope.png\": no such file, the item is drawn empty"), "\(messages)")
        XCTAssertTrue(
            messages.contains("image \"logo\": the file is not a PNG, JPEG, GIF, WebP or TIFF image, the item is drawn empty"),
            "\(messages)")
        XCTAssertTrue(tree.diagnostics.allSatisfy(\.warn), "a picture that is not drawn is not an error")
        // A box without a picture and without text keeps the classic 8-point dot.
        XCTAssertEqual(try XCTUnwrap(screen.placed().kids.first).w, 8, accuracy: 1e-9)
    }
}
