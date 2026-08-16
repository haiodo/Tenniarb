//
//  DrawableStyleTests.swift
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

import JavaScriptCore
import XCTest

@testable import Tenniarb

// MARK: - Helpers

/// Parse `source` as style properties and apply them to `style`.
private func apply(_ source: String, to style: DrawableStyle, evaluations: [TennToken: JSValue] = [:]) {
    style.parseStyle(TennParser().parse(source), evaluations)
}

private func components(_ color: CGColor) -> [CGFloat] {
    return color.components ?? []
}

// MARK: - DrawableStyle

class DrawableStyleTests: XCTestCase {

    func testDefaultsDifferBetweenLightAndDarkMode() {
        let light = DrawableStyle(false)
        let dark = DrawableStyle(true)

        XCTAssertNotEqual(components(light.color), components(dark.color))
        XCTAssertNotEqual(components(light.borderColor), components(dark.borderColor))
        XCTAssertEqual(light.fontSize, 18)
        XCTAssertNil(light.width)
        XCTAssertNil(light.height)
    }

    func testColorIsParsedByName() {
        let style = DrawableStyle(false)
        apply("color red", to: style)
        XCTAssertNotEqual(components(style.color), components(DrawableStyle(false).color))
    }

    func testUnknownColorNameLeavesTheDefault() {
        let style = DrawableStyle(false)
        let before = components(style.color)
        apply("color notacoloratall", to: style)
        XCTAssertEqual(components(style.color), before)
    }

    func testTextColorFallsBackToContrastWithBackground() {
        let style = DrawableStyle(false)
        XCTAssertNil(style.textColorValue, "Nothing explicit yet")
        let derived = style.textColor

        apply("text-color red", to: style)
        XCTAssertNotNil(style.textColorValue)
        XCTAssertNotEqual(components(style.textColor), components(derived), "An explicit value wins over the derived one")
    }

    func testFontSizeIsClampedToTheAllowedRange() {
        let tooBig = DrawableStyle(false)
        apply("font-size 99", to: tooBig)
        XCTAssertEqual(tooBig.fontSize, 36)

        let tooSmall = DrawableStyle(false)
        apply("font-size 1", to: tooSmall)
        XCTAssertEqual(tooSmall.fontSize, 4)

        let fine = DrawableStyle(false)
        apply("font-size 20", to: fine)
        XCTAssertEqual(fine.fontSize, 20)
    }

    func testWidthAndHeightAreCappedAtTenThousand() {
        let style = DrawableStyle(false)
        apply("width 99999\nheight 99999", to: style)
        XCTAssertEqual(style.width, 10000)
        XCTAssertEqual(style.height, 10000)
    }

    func testWidthAndHeightPassThroughWhenReasonable() {
        let style = DrawableStyle(false)
        apply("width 120\nheight 40", to: style)
        XCTAssertEqual(style.width, 120)
        XCTAssertEqual(style.height, 40)
    }

    func testDisplayLayerAndLineStyleAreStoredVerbatim() {
        let style = DrawableStyle(false)
        apply("display text\nlayer background\nline-style dashed", to: style)
        XCTAssertEqual(style.display, "text")
        XCTAssertEqual(style.layer, "background")
        XCTAssertEqual(style.lineStyle, "dashed")
    }

    func testLayoutJoinsAllItsArguments() {
        let style = DrawableStyle(false)
        apply("layout auto center", to: style)
        XCTAssertEqual(style.layout, "auto, center")
    }

    func testLayoutWithNoArgumentsIsEmpty() {
        let style = DrawableStyle(false)
        apply("layout", to: style)
        XCTAssertEqual(style.layout, "")
    }

    func testLineWidthIsParsed() {
        let style = DrawableStyle(false)
        apply("line-width 2.5", to: style)
        XCTAssertEqual(style.lineWidth, 2.5)
    }

    func testShadowReadsOffsetBlurAndColor() {
        let style = DrawableStyle(false)
        apply("shadow 2 3 7 red", to: style)

        XCTAssertEqual(style.shadow, CGSize(width: 2, height: 3))
        XCTAssertEqual(style.shadowBlur, 7)
        XCTAssertNotNil(style.shadowColor)
    }

    func testShadowWithOnlyOffsetKeepsDefaultBlur() {
        let style = DrawableStyle(false)
        let defaultBlur = style.shadowBlur
        apply("shadow 1 1", to: style)

        XCTAssertEqual(style.shadow, CGSize(width: 1, height: 1))
        XCTAssertEqual(style.shadowBlur, defaultBlur)
        XCTAssertNil(style.shadowColor)
    }

    func testInheritReadsNameAndIndex() {
        let style = DrawableStyle(false)
        apply("inherit 3", to: style)
        XCTAssertEqual(style.inherit, "inherit")
        XCTAssertEqual(style.inheritIndex, 3)
    }

    func testUnknownCommandsAreIgnored() {
        let style = DrawableStyle(false)
        apply("totally-unknown 1 2 3", to: style)
        XCTAssertEqual(style.fontSize, 18, "An unrecognised command must not disturb anything")
    }

    func testResetRestoresDefaults() {
        let style = DrawableStyle(false)
        apply("font-size 30\nwidth 100\ndisplay text\nline-style dashed", to: style)
        style.reset()

        XCTAssertEqual(style.fontSize, 18)
        XCTAssertNil(style.width)
        XCTAssertNil(style.display)
        XCTAssertNil(style.lineStyle)
    }

    func testCopyCarriesEveryField() {
        let style = DrawableStyle(false)
        apply("color red\nfont-size 22\nwidth 120\nheight 40\ndisplay text\nlayer hover\nline-style dashed\nline-width 3", to: style)
        apply("shadow 1 2 3 blue", to: style)

        let copy = style.copy()
        XCTAssertEqual(copy.fontSize, style.fontSize)
        XCTAssertEqual(copy.width, style.width)
        XCTAssertEqual(copy.height, style.height)
        XCTAssertEqual(copy.display, style.display)
        XCTAssertEqual(copy.layer, style.layer)
        XCTAssertEqual(copy.lineStyle, style.lineStyle)
        XCTAssertEqual(copy.lineWidth, style.lineWidth)
        XCTAssertEqual(copy.shadow, style.shadow)
        XCTAssertEqual(copy.shadowBlur, style.shadowBlur)
        XCTAssertEqual(components(copy.color), components(style.color))
    }

    func testCopyIsIndependentOfTheOriginal() {
        let style = DrawableStyle(false)
        apply("font-size 20", to: style)
        let copy = style.copy()

        apply("font-size 30", to: style)
        XCTAssertEqual(copy.fontSize, 20, "The copy keeps the value it was made with")
    }

    func testGetComponentValueAcceptsEveryNumericType() {
        let style = DrawableStyle(false)
        XCTAssertEqual(style.getComponentValue(Int(10)), 10)
        XCTAssertEqual(style.getComponentValue(Double(20)), 20)
        XCTAssertEqual(style.getComponentValue(Float(30)), 30)
        XCTAssertEqual(style.getComponentValue("not a number"), 255, "Anything else reads as full intensity")
    }
}

// MARK: - Style values driven by evaluated expressions

class DrawableStyleEvaluationTests: XCTestCase {

    /// Evaluate `expression` in a throwaway JS context so it can stand in for a computed property.
    private func evaluate(_ expression: String) -> JSValue {
        let context = JSContext()!
        return context.evaluateScript(expression)!
    }

    /// Build a `color`-style command whose value token can be mapped to an evaluated result.
    private func commandWithToken(_ name: String, _ literal: String) -> (TennNode, TennToken) {
        let token = TennToken(type: .expression, literal: literal)
        let value = TennNode.newNode(kind: .Expression, token)
        return (TennNode.newCommand(name, value), token)
    }

    func testExpressionSuppliesAFloatValue() {
        let (node, token) = commandWithToken("font-size", "10 + 5")
        let style = DrawableStyle(false)
        let block = TennNode.newBlockExpr(node)

        style.parseStyle(block, [token: evaluate("10 + 5")])
        XCTAssertEqual(style.fontSize, 15)
    }

    func testExpressionSuppliesAnRgbArrayColor() {
        let (node, token) = commandWithToken("color", "[255, 0, 0]")
        let style = DrawableStyle(false)
        let block = TennNode.newBlockExpr(node)

        style.parseStyle(block, [token: evaluate("[255, 0, 0]")])

        let parts = components(style.color)
        XCTAssertEqual(parts.count, 4)
        XCTAssertEqual(parts[0], 1.0, accuracy: 0.001, "Red channel is full")
        XCTAssertEqual(parts[1], 0.0, accuracy: 0.001)
    }

    func testExpressionRgbaArrayCarriesAlpha() {
        let (node, token) = commandWithToken("color", "[255, 0, 0, 128]")
        let style = DrawableStyle(false)

        style.parseStyle(TennNode.newBlockExpr(node), [token: evaluate("[255, 0, 0, 128]")])
        let parts = components(style.color)
        XCTAssertEqual(parts[3], 128.0 / 255.0, accuracy: 0.001)
    }

    func testExpressionSuppliesAColorName() {
        let (node, token) = commandWithToken("color", "'red'")
        let style = DrawableStyle(false)
        let before = components(style.color)

        style.parseStyle(TennNode.newBlockExpr(node), [token: evaluate("'red'")])
        XCTAssertNotEqual(components(style.color), before)
    }
}

// MARK: - DrawableItemStyle

class DrawableItemStyleTests: XCTestCase {

    func testTitleAndMarkerAreParsed() {
        let style = DrawableItemStyle(false)
        apply("title Header\nmarker dot", to: style)
        XCTAssertEqual(style.title, "Header")
        XCTAssertEqual(style.marker, "dot")
    }

    func testCornerRadiusIsCappedAtFifteen() {
        let style = DrawableItemStyle(false)
        apply("corner-radius 4 8 99 2", to: style)
        XCTAssertEqual(style.cornerRadius, [4, 8, 15, 2])
    }

    func testCornerRadiusReplacesAnyPreviousValue() {
        let style = DrawableItemStyle(false)
        apply("corner-radius 1 2 3 4", to: style)
        apply("corner-radius 5", to: style)
        XCTAssertEqual(style.cornerRadius, [5], "Each declaration starts from scratch")
    }

    func testLineSpacingIsParsed() {
        let style = DrawableItemStyle(false)
        apply("line-spacing 3", to: style)
        XCTAssertEqual(style.lineSpacing, 3)
    }

    func testItemStyleStillHandlesBaseCommands() {
        let style = DrawableItemStyle(false)
        apply("font-size 24\ndisplay text", to: style)
        XCTAssertEqual(style.fontSize, 24)
        XCTAssertEqual(style.display, "text")
    }

    func testCopyCarriesItemSpecificFields() {
        let style = DrawableItemStyle(false)
        apply("title Header\nmarker dot\ncorner-radius 6\nline-spacing 2", to: style)

        let copy = style.copy()
        XCTAssertEqual(copy.title, "Header")
        XCTAssertEqual(copy.marker, "dot")
        XCTAssertEqual(copy.cornerRadius, [6])
        XCTAssertEqual(copy.lineSpacing, 2)
    }
}

// MARK: - SceneStyle

class SceneStyleTests: XCTestCase {

    func testDefaultGridSpan() {
        XCTAssertEqual(SceneStyle(false).gridSpan, CGPoint(x: 5, y: 5))
    }

    func testGridCommandOverridesSpan() {
        let style = SceneStyle(false)
        apply("grid 20 30", to: style)
        XCTAssertEqual(style.gridSpan, CGPoint(x: 20, y: 30))
    }

    func testStylesBlockConfiguresItemDefaults() {
        let style = SceneStyle(false)
        apply("styles {\n  item {\n    font-size 24\n  }\n}", to: style)
        XCTAssertEqual(style.defaultItemStyle.fontSize, 24)
    }

    func testStylesBlockConfiguresLineDefaults() {
        let style = SceneStyle(false)
        apply("styles {\n  line {\n    line-width 4\n  }\n}", to: style)
        XCTAssertEqual(style.defaultLineStyle.lineWidth, 4)
    }

    func testItemAndLineDefaultsAreSeparate() {
        let style = SceneStyle(false)
        apply("styles {\n  item {\n    font-size 30\n  }\n}", to: style)
        XCTAssertNotEqual(style.defaultLineStyle.fontSize, 30, "Line defaults must not pick up item settings")
    }
}

// MARK: - Body text preparation

class BodyTextTests: XCTestCase {

    func testCommonIndentIsStripped() {
        XCTAssertEqual(prepareBodyText("\n    one\n    two\n"), "one\ntwo")
    }

    func testDeeperIndentIsKeptRelative() {
        XCTAssertEqual(prepareBodyText("\n  one\n    two\n"), "one\n  two")
    }

    func testEscapedNewlinesBecomeRealOnes() {
        XCTAssertEqual(prepareBodyText("one\\ntwo"), "one\ntwo")
    }

    func testTabsBecomeFourSpaces() {
        XCTAssertFalse(prepareBodyText("\tone").contains("\t"))
    }

    func testLeadingAndTrailingBlankLinesAreDropped() {
        XCTAssertEqual(prepareBodyText("\nbody\n"), "body")
    }

    func testSingleLineIsUntouched() {
        XCTAssertEqual(prepareBodyText("plain"), "plain")
    }

    func testEmptyInputStaysEmpty() {
        XCTAssertEqual(prepareBodyText(""), "")
    }
}

// MARK: - Drawable primitives

class DrawablePrimitiveTests: XCTestCase {

    func testRoundBoxBoundsCoverTheRequestedRectPlusItsStroke() {
        let bounds = CGRect(x: 1, y: 2, width: 30, height: 40)
        let box = RoundBox(bounds: bounds, DrawableItemStyle(false), fill: true)

        // getBounds() grows the rect by the stroke width so the border is not clipped.
        XCTAssertTrue(box.getBounds().contains(bounds))
        XCTAssertEqual(box.getBounds().width, bounds.width, accuracy: 1)
        XCTAssertNotNil(box.path, "A round box builds its rounded path up front")
    }

    func testRoundBoxSetPathMovesIt() {
        let box = RoundBox(bounds: CGRect(x: 0, y: 0, width: 10, height: 10), DrawableItemStyle(false), fill: true)
        let moved = CGRect(x: 5, y: 6, width: 20, height: 20)
        box.setPath(moved)
        XCTAssertTrue(box.getBounds().contains(moved))
        XCTAssertEqual(box.getBounds().midX, moved.midX, accuracy: 0.001)
    }

    func testRoundBoxHonoursCustomCornerRadius() {
        let style = DrawableItemStyle(false)
        apply("corner-radius 2 4 6 8", to: style)
        let box = RoundBox(bounds: CGRect(x: 0, y: 0, width: 50, height: 50), style, fill: true)
        XCTAssertNotNil(box.path)
    }

    func testEmptyBoxKeepsItsBounds() {
        let bounds = CGRect(x: 3, y: 4, width: 15, height: 25)
        let box = EmptyBox(bounds: bounds, DrawableStyle(false))
        XCTAssertEqual(box.getBounds(), bounds)

        let moved = CGRect(x: 0, y: 0, width: 1, height: 1)
        box.setPath(moved)
        XCTAssertEqual(box.getBounds(), moved)
    }

    func testCircleBoxKeepsItsBounds() {
        let bounds = CGRect(x: 0, y: 0, width: 40, height: 40)
        let box = CircleBox(bounds: bounds, DrawableItemStyle(false), fill: true)
        XCTAssertEqual(box.getBounds(), bounds)
    }

    func testTextBoxReportsItsFrame() {
        let text = NSAttributedString(string: "hello")
        let box = TextBox(text: text, bounds: CGRect(x: 1, y: 1, width: 50, height: 20))

        XCTAssertEqual(box.getBounds().width, 50)
        XCTAssertTrue(box.isVisible())
        XCTAssertTrue(box.isSelectable())

        box.setFrame(CGRect(x: 0, y: 0, width: 80, height: 30))
        XCTAssertEqual(box.getBounds().width, 80)
    }

    func testTextBoxTraverseVisitsItself() {
        let box = TextBox(text: NSAttributedString(string: "x"), bounds: .zero)
        var visited = 0
        box.traverse { _ in
            visited += 1
            return true
        }
        XCTAssertEqual(visited, 1)
    }

    func testTextBoxRenderIsAStub() {
        let box = TextBox(text: NSAttributedString(string: "x"), bounds: .zero)
        XCTAssertEqual(box.render(type: .svg), "")
    }

    func testItemDrawableDefaultsToSelectable() {
        let drawable = ItemDrawable()
        XCTAssertTrue(drawable.isSelectable(), "The default layer is selectable")
        XCTAssertEqual(drawable.render(type: .svg), "")
    }

    func testContainerReportsUnionOfChildren() {
        let a = EmptyBox(bounds: CGRect(x: 0, y: 0, width: 10, height: 10), DrawableStyle(false))
        let b = EmptyBox(bounds: CGRect(x: 100, y: 100, width: 10, height: 10), DrawableStyle(false))
        let container = DrawableContainer([a, b])

        let bounds = container.getBounds()
        XCTAssertGreaterThanOrEqual(bounds.width, 110)
        XCTAssertGreaterThanOrEqual(bounds.height, 110)
    }

    func testEmptyContainerHasNoChildren() {
        XCTAssertNil(DrawableContainer([]).children)
    }
}
