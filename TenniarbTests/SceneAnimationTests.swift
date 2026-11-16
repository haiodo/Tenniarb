//
//  SceneAnimationTests.swift
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

/// Keyframes and the first frame they give the picture, as go/scene/anim.go.
final class SceneAnimationTests: XCTestCase {

    /// A workspace on disk: the loader resolves the files a scene mounts from it.
    private func tree(_ text: String) throws -> Scene.Tree {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-scene-animation-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        try text.write(to: root.appendingPathComponent("board.tenn"), atomically: true, encoding: .utf8)
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        return try SceneLoader(root: root).load("board.tenn")
    }

    func testTheFirstFrameIsTheOpacityOfTheNode() throws {
        let tree = try tree(
            """
            element "Screen" {
                animation fade {
                    0% { opacity 0 }
                    100% { opacity 1 }
                }
                item Card { width 80; height 32; animation fade 600ms ease-out }
            }
            """)
        XCTAssertTrue(tree.diagnostics.isEmpty, "\(tree.diagnostics)")
        let screen = try XCTUnwrap(tree.build("Screen", measurer: Scene.TextMeasurer(estimated: true)))
        let card = try XCTUnwrap(screen.tree.kids.first)

        XCTAssertEqual(screen.p.styles[ObjectIdentifier(card)]?.opacity, 0)
        let a = try XCTUnwrap(screen.p.anim[ObjectIdentifier(card)])
        XCTAssertEqual(a.duration, 0.6)
        XCTAssertEqual(a.easing, "ease-out")
        XCTAssertEqual(a.frames.map(\.offset), [0, 1])
    }

    func testABadKeyframeIsWarned() throws {
        let tree = try tree(
            """
            element "Screen" {
                animation broken {
                    middle { rotate 10 }
                    from { width 40 }
                    to { opacity 0.5 }
                }
                item A { width 10; height 10; animation broken 1s }
            }
            """)
        _ = try XCTUnwrap(tree.build("Screen", measurer: Scene.TextMeasurer(estimated: true)))
        let messages = tree.diagnostics.map(\.message)
        XCTAssertTrue(messages.contains("animation broken: frame \"middle\" is not from, to or a percentage"), "\(messages)")
        XCTAssertTrue(
            messages.contains("animation broken: width is a layout property and is not animated, the frame is ignored"),
            "\(messages)")
        XCTAssertTrue(tree.diagnostics.allSatisfy(\.warn), "a frame that does not read is not an error")
    }
}
