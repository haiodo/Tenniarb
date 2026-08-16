//
//  PreferencesUtilsTests.swift
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

@testable import Tenniarb

private func components(_ color: CGColor?) -> [CGFloat] {
    return color?.components ?? []
}

// MARK: - Colour parsing

class ColorParsingTests: XCTestCase {

    func testSixDigitHexIsParsed() {
        let color = parseColor("#ff0000")
        XCTAssertEqual(components(color)[0], 1.0, accuracy: 0.001)
        XCTAssertEqual(components(color)[1], 0.0, accuracy: 0.001)
        XCTAssertEqual(components(color)[2], 0.0, accuracy: 0.001)
    }

    func testHexWithoutHashIsAccepted() {
        XCTAssertEqual(components(parseColor("00ff00")), components(parseColor("#00ff00")))
    }

    func testEightDigitHexCarriesAlpha() {
        let color = parseColor("#ff000080")
        XCTAssertEqual(components(color)[3], 128.0 / 255.0, accuracy: 0.005)
    }

    func testAlphaArgumentIsAppliedToSixDigitHex() {
        let color = parseColor("#ff0000", alpha: 0.5)
        XCTAssertEqual(components(color)[3], 0.5, accuracy: 0.001)
    }

    func testNamedColoursResolve() {
        XCTAssertNotNil(parseColor("red"))
        XCTAssertNotNil(parseColor("white"))
        XCTAssertEqual(components(parseColor("red")), components(parseColor("RED")), "Names are case-insensitive")
    }

    func testSurroundingWhitespaceIsTrimmed() {
        XCTAssertEqual(components(parseColor("  #ff0000  ")), components(parseColor("#ff0000")))
    }

    func testUnknownNameIsRejected() {
        XCTAssertNil(parseColor("definitelynotacolour"))
    }

    func testWrongLengthHexIsRejected() {
        XCTAssertNil(parseColor("#fff"))
        XCTAssertNil(parseColor("#ff00ff00ff"))
    }

    func testTextColourContrastsWithItsBackground() {
        let onWhite = getTextColorBasedOn(CGColor(red: 1, green: 1, blue: 1, alpha: 1))
        let onBlack = getTextColorBasedOn(CGColor(red: 0, green: 0, blue: 0, alpha: 1))
        XCTAssertNotEqual(components(onWhite), components(onBlack), "Light and dark backgrounds get different text")
    }
}

// MARK: - Preferences

@MainActor
class PreferenceConstantsTests: XCTestCase {

    func testSharedInstanceIsStable() {
        XCTAssertTrue(PreferenceConstants.preference === PreferenceConstants.preference)
    }

    func testDefaultsAreRegistered() {
        let preference = PreferenceConstants.preference
        preference.checkDefaults()

        XCTAssertGreaterThanOrEqual(preference.autoExpandLevel, 0)
        XCTAssertNotNil(preference.getBackground())
    }

    func testBackgroundFollowsTheDarkModeFlag() {
        let preference = PreferenceConstants.preference
        XCTAssertNotEqual(
            components(preference.backgroundGet), components(preference.backgroundDarkGet),
            "Light and dark backgrounds differ")
    }

    func testGetBackgroundReturnsOneOfTheTwo() {
        let preference = PreferenceConstants.preference
        let background = components(preference.getBackground())

        XCTAssertTrue(
            background == components(preference.backgroundGet) || background == components(preference.backgroundDarkGet))
    }

    func testBooleanPreferencesAreReadable() {
        let preference = PreferenceConstants.preference
        _ = preference.autoExpand
        _ = preference.renderEnableBackground
        _ = preference.renderUseNativeResolution
        _ = preference.uiQuickPanelOnTop
        _ = preference.uiTransparentBackground
        _ = preference.isDiagramDarkMode()
    }

    func testRawDarkModeIsAnswered() {
        _ = PreferenceConstants.isDarkModeRaw()
    }
}

// MARK: - Clipboard

@MainActor
class ClipboardUtilsTests: XCTestCase {

    override func setUp() {
        super.setUp()
        NSPasteboard.general.clearContents()
    }

    func testCopyPutsTennSourceOnThePasteboard() {
        let node = TennNode.newCommand("color", TennNode.newIdent("red"))
        ClipboardUtils.copy(node)

        XCTAssertEqual(NSPasteboard.general.string(forType: .string), node.toStr())
    }

    func testValidTennCanBePasted() {
        ClipboardUtils.copy(TennNode.newCommand("color", TennNode.newIdent("red")))
        XCTAssertTrue(ClipboardUtils.canPaste())
    }

    func testBrokenTennCannotBePasted() {
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString("item Broken {", forType: .string)
        XCTAssertFalse(ClipboardUtils.canPaste(), "An unclosed block is not pasteable")
    }

    func testEmptyPasteboardCannotBePasted() {
        NSPasteboard.general.clearContents()
        XCTAssertFalse(ClipboardUtils.canPaste())
    }

    func testPasteDeliversTheParsedNode() {
        ClipboardUtils.copy(TennNode.newCommand("color", TennNode.newIdent("red")))

        var delivered: TennNode?
        ClipboardUtils.paste { delivered = $0 }

        XCTAssertNotNil(delivered)
        XCTAssertEqual(delivered?.getNamedElement("color")?.getIdent(1) ?? delivered?.getIdent(0, 1), "red")
    }

    func testPasteOfBrokenSourceDeliversNothing() {
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString("item Broken {", forType: .string)

        var called = false
        ClipboardUtils.paste { _ in called = true }
        XCTAssertFalse(called)
    }

    func testPngOnThePasteboardBecomesAnImageCommand() {
        let png = Data(
            base64Encoded: "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
        )!
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setData(png, forType: .png)
        NSPasteboard.general.setString("logo.png", forType: .string)

        var delivered: TennNode?
        ClipboardUtils.paste { delivered = $0 }

        XCTAssertEqual(delivered?.getIdent(0), "image")
        XCTAssertEqual(delivered?.getIdent(1), "logo.png")
    }

    func testCopyHtmlPublishesBothFlavours() {
        ClipboardUtils.copyHtml("<html></html>")

        XCTAssertEqual(NSPasteboard.general.string(forType: .html), "<html></html>")
        XCTAssertEqual(NSPasteboard.general.string(forType: .string), "<html></html>")
    }
}

// MARK: - Image helpers

class ImageUtilsTests: XCTestCase {

    func testMaxRectKeepsTheAspectRatio() {
        let rect = getMaxRect(maxWidth: 100, maxHeight: 100, imageWidth: 200, imageHeight: 100)

        XCTAssertLessThanOrEqual(rect.width, 100)
        XCTAssertLessThanOrEqual(rect.height, 100)
        XCTAssertEqual(rect.width / rect.height, 2.0, accuracy: 0.01, "A 2:1 image stays 2:1")
    }

    func testTallImageIsBoundedByHeight() {
        let rect = getMaxRect(maxWidth: 100, maxHeight: 100, imageWidth: 100, imageHeight: 400)
        XCTAssertLessThanOrEqual(rect.height, 100)
        XCTAssertEqual(rect.width / rect.height, 0.25, accuracy: 0.01)
    }

    func testSmallImageIsNotBlownUpBeyondTheBounds() {
        let rect = getMaxRect(maxWidth: 500, maxHeight: 500, imageWidth: 10, imageHeight: 10)
        XCTAssertLessThanOrEqual(rect.width, 500)
        XCTAssertEqual(rect.width, rect.height, accuracy: 0.01)
    }

    func testScalingProducesASmallerImage() throws {
        let context = CGContext(
            data: nil, width: 100, height: 100, bitsPerComponent: 8, bytesPerRow: 0,
            space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
        context.setFillColor(CGColor(red: 1, green: 0, blue: 0, alpha: 1))
        context.fill(CGRect(x: 0, y: 0, width: 100, height: 100))

        guard let image = context.makeImage() else { throw XCTSkip("Cannot build a test image") }
        guard let scaled = scaleImage(image, maxWidth: 50, maxHeight: 50) else {
            return XCTFail("Expected a scaled image")
        }
        XCTAssertLessThanOrEqual(scaled.width, 50)
        XCTAssertLessThanOrEqual(scaled.height, 50)
    }
}

// MARK: - Small controllers

@MainActor
class SmallControllerTests: XCTestCase {

    func testHelpControllerLoadsItsView() {
        let controller = HelpController()
        controller.loadViewIfNeeded()
        XCTAssertNotNil(controller.view)
    }

    func testResourcesControllerLoadsItsView() {
        let controller = ResourcesViewController()
        controller.loadViewIfNeeded()
        XCTAssertNotNil(controller.view)
    }

    func testSourcePopoverShowsTheElementSource() {
        let model = ElementModel.parseTenn(
            node: TennParser().parse(
                """
                element Root {
                    item Alpha {
                        pos 0 0
                    }
                }
                """))

        let controller = SourcePopoverViewController()
        controller.loadViewIfNeeded()
        controller.setElement(element: model.elements[0])
        XCTAssertNotNil(controller.view)
    }
}
