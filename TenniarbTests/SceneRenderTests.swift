//
//  SceneRenderTests.swift
//  TenniarbTests
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//
//  See the License for the specific language governing permissions and
//  limitations under the License.

import XCTest
import cmkdown

@testable import Tenniarb

// MARK: - Helpers

/// An offscreen bitmap context, so the drawing code can run without a window.
private func makeContext(width: Int = 400, height: Int = 400) -> CGContext {
    let context = CGContext(
        data: nil, width: width, height: height, bitsPerComponent: 8, bytesPerRow: 0,
        space: CGColorSpaceCreateDeviceRGB(),
        bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
    NSGraphicsContext.current = NSGraphicsContext(cgContext: context, flipped: false)
    return context
}

/// Parse a diagram from tenn source and return its first nested element.
private func parseDiagram(_ source: String) -> Element {
    let parser = TennParser()
    let node = parser.parse(source)
    XCTAssertFalse(parser.errors.hasErrors(), "Fixture must parse cleanly")
    return ElementModel.parseTenn(node: node)
}

private func buildScene(_ element: Element, darkMode: Bool = false) -> DrawableScene {
    let context = ExecutionContext()
    context.setElement(element)
    return DrawableScene(element, darkMode: darkMode, executionContext: context)
}

private func drawAndLayout(_ scene: DrawableScene) {
    let bounds = scene.getBounds()
    scene.layout(bounds, bounds)
    scene.draw(context: makeContext())
}

// MARK: - Rendering a real diagram offscreen

class SceneRenderTests: XCTestCase {

    private let source = """
        element Root {
            item First {
                pos 10 20
                color red
                font-size 20
            }
            item Second {
                pos 200 40
                display text
                text-color blue
            }
            item Circle {
                pos 100 200
                display circle
            }
            link First Second {
            }
        }
        """

    private func diagram() -> Element {
        return parseDiagram(source).elements[0]
    }

    func testSceneBuildsDrawablesForEveryItem() {
        let element = diagram()
        let scene = buildScene(element)

        for item in element.items {
            XCTAssertNotNil(scene.drawables[item], "\(item.name) needs a drawable")
        }
    }

    func testDrawingTheWholeSceneSucceeds() {
        let scene = buildScene(diagram())
        drawAndLayout(scene)
        XCTAssertGreaterThan(scene.getBounds().width, 0, "The scene still reports its size after drawing")
    }

    func testDrawingInDarkModeSucceeds() {
        let scene = buildScene(diagram(), darkMode: true)
        drawAndLayout(scene)
        XCTAssertTrue(scene.darkMode)
    }

    func testDrawBoxPassRuns() {
        let scene = buildScene(diagram())
        let bounds = scene.getBounds()
        scene.layout(bounds, bounds)
        scene.drawBox(context: makeContext())
    }

    func testDrawingProducesANonBlankImage() {
        let scene = buildScene(diagram())
        let context = makeContext()
        let bounds = scene.getBounds()

        scene.offset = CGPoint(x: -bounds.origin.x + 10, y: -bounds.origin.y + 10)
        scene.layout(bounds, bounds)
        scene.draw(context: context)

        guard let data = context.data else { return XCTFail("Expected a bitmap backing store") }
        let pixels = data.bindMemory(to: UInt8.self, capacity: context.bytesPerRow * context.height)
        var painted = 0
        for i in stride(from: 3, to: context.bytesPerRow * context.height, by: 4) where pixels[i] != 0 {
            painted += 1
        }
        XCTAssertGreaterThan(painted, 0, "Something must actually be drawn")
    }

    func testShadowAndBorderStylesRenderWithoutTrouble() {
        let element = parseDiagram(
            """
            element Root {
                item Boxed {
                    pos 0 0
                    shadow 2 2 4 gray
                    border-color green
                    line-width 3
                    corner-radius 6
                }
            }
            """
        ).elements[0]

        drawAndLayout(buildScene(element))
    }

    func testItemWithExplicitSizeRendersAtThatSize() {
        let element = parseDiagram(
            """
            element Root {
                item Wide {
                    pos 0 0
                    width 300
                    height 120
                }
            }
            """
        ).elements[0]

        let scene = buildScene(element)
        guard let item = element.items.first, let drawable = scene.drawables[item] else {
            return XCTFail("Expected a drawable")
        }
        XCTAssertEqual(drawable.getBounds().width, 300, accuracy: 2)
        XCTAssertEqual(drawable.getBounds().height, 120, accuracy: 2)
    }

    func testBackgroundLayerItemIsNotSelectable() {
        let element = parseDiagram(
            """
            element Root {
                item Back {
                    pos 0 0
                    layer background
                }
            }
            """
        ).elements[0]

        let scene = buildScene(element)
        guard let item = element.items.first, let drawable = scene.drawables[item] else {
            return XCTFail("Expected a drawable")
        }
        XCTAssertFalse(drawable.isSelectable(), "Background items stay out of the way of selection")
    }

    func testMarkdownBodyIsRendered() {
        let element = parseDiagram(
            """
            element Root {
                item Doc {
                    pos 0 0
                    body %{**bold** and _italic_}
                }
            }
            """
        ).elements[0]

        let scene = buildScene(element)
        guard let item = element.items.first, let drawable = scene.drawables[item] else {
            return XCTFail("Expected a drawable")
        }
        XCTAssertGreaterThan(drawable.getBounds().width, 0)
        drawAndLayout(scene)
    }

    func testLinkBetweenItemsGetsALineDrawable() {
        let element = diagram()
        let scene = buildScene(element)

        guard let link = element.items.first(where: { $0.kind == .Link }) else {
            return XCTFail("Fixture must contain a link")
        }
        XCTAssertTrue(scene.drawables[link] is DrawableLine)
    }

    func testUpdateActiveElementsMarksThemActive() {
        let element = diagram()
        let scene = buildScene(element)
        let items = element.items.filter { $0.kind == .Item }

        scene.updateActiveElements([items[0]])
        XCTAssertEqual(scene.activeElements, [items[0]])

        scene.updateActiveElements([])
        XCTAssertTrue(scene.activeElements.isEmpty)
    }

    func testUpdateActiveElementsAppliesTemporaryPositions() {
        let element = diagram()
        let scene = buildScene(element)
        guard let item = element.items.first(where: { $0.kind == .Item }),
            let drawable = scene.drawables[item]
        else {
            return XCTFail("Expected a drawable")
        }
        let before = drawable.getBounds()

        // Positions passed here are drag offsets applied during draw, not a rebuild of the
        // drawable, so the cached bounds stay put until the scene is rebuilt.
        scene.updateActiveElements([item], [item: CGPoint(x: 500, y: 500)])
        XCTAssertEqual(scene.activeElements, [item])
        XCTAssertEqual(scene.drawables[item]?.getBounds(), before)
    }

    func testUpdateLineToFindsTheItemUnderThePoint() {
        let element = diagram()
        let scene = buildScene(element)
        let items = element.items.filter { $0.kind == .Item }

        guard let targetDrawable = scene.drawables[items[1]] else { return XCTFail("Expected a drawable") }
        let center = CGPoint(x: targetDrawable.getBounds().midX, y: targetDrawable.getBounds().midY)

        XCTAssertEqual(scene.updateLineTo(items[0], center), items[1], "Dragging onto an item selects it as the target")
    }

    func testUpdateLineToReturnsNilOverEmptySpace() {
        let element = diagram()
        let scene = buildScene(element)
        let items = element.items.filter { $0.kind == .Item }

        XCTAssertNil(scene.updateLineTo(items[0], CGPoint(x: 100_000, y: 100_000)))
    }

    func testEditingModeIsRemembered() {
        let scene = buildScene(diagram())
        XCTAssertFalse(scene.editingMode)
        scene.editingMode = true
        XCTAssertTrue(scene.editingMode)
    }

    func testSceneWithoutChildElementsStillBuilds() {
        let element = diagram()
        let context = ExecutionContext()
        context.setElement(element)

        let scene = DrawableScene(element, darkMode: false, executionContext: context, buildChildren: false)
        XCTAssertEqual(scene.getBounds().width, 0, "Skipping children leaves nothing to measure")
    }

    func testSceneRestrictedToASubsetOfItems() {
        let element = diagram()
        let context = ExecutionContext()
        context.setElement(element)
        let subset = Array(element.items.filter { $0.kind == .Item }.prefix(1))

        let scene = DrawableScene(element, darkMode: false, executionContext: context, items: subset)
        XCTAssertNotNil(scene.drawables[subset[0]])
    }
}

// MARK: - Text measurement

class SceneTextTests: XCTestCase {

    func testCalculateSizeGrowsWithTheText() {
        let short = DrawableScene.calculateSize(attrStr: NSAttributedString(string: "a"))
        let long = DrawableScene.calculateSize(attrStr: NSAttributedString(string: "a much longer piece of text"))
        XCTAssertGreaterThan(long.width, short.width)
    }

    func testCalculateSizeOfEmptyTextIsNotNegative() {
        let size = DrawableScene.calculateSize(attrStr: NSAttributedString(string: ""))
        XCTAssertGreaterThanOrEqual(size.width, 0)
        XCTAssertGreaterThanOrEqual(size.height, 0)
    }

    func testAttributedStringFromMarkdownKeepsTheText() {
        var shift = CGPoint.zero
        let item = DiagramItem(kind: .Item, name: "Host")
        let attributed = DrawableScene.toAttributedString(
            tokens: MarkdownLexer.getTokens(code: "**bold** text"),
            font: NSFont.systemFont(ofSize: 12), color: CGColor.black, shift: &shift,
            imageProvider: ElementImageProvider(item, 1), layout: [])

        XCTAssertTrue(attributed.string.contains("bold"))
        XCTAssertTrue(attributed.string.contains("text"))
        XCTAssertGreaterThan(attributed.length, 0)
    }

    func testMultilineMarkdownIsTallerThanASingleLine() {
        let item = DiagramItem(kind: .Item, name: "Host")
        func measure(_ code: String) -> CGSize {
            var shift = CGPoint.zero
            let attributed = DrawableScene.toAttributedString(
                tokens: MarkdownLexer.getTokens(code: code),
                font: NSFont.systemFont(ofSize: 12), color: CGColor.black, shift: &shift,
                imageProvider: ElementImageProvider(item, 1), layout: [])
            return DrawableScene.calculateSize(attrStr: attributed)
        }
        XCTAssertGreaterThan(measure("one\ntwo").height, measure("one").height)
    }
}
