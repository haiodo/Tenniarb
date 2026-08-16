//
//  DrawableLineTests.swift
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

// MARK: - Helpers

private func lineStyle(_ source: String = "") -> DrawableLineStyle {
    let style = DrawableLineStyle(false)
    if !source.isEmpty {
        style.parseStyle(TennParser().parse(source), [:])
    }
    return style
}

private func makeLine(
    from: CGRect, to: CGRect, style: DrawableLineStyle = DrawableLineStyle(false)
) -> DrawableLine {
    return DrawableLine(source: from, target: to, style: style)
}

// MARK: - Geometry

class DrawableLineTests: XCTestCase {

    private let left = CGRect(x: 0, y: 0, width: 40, height: 20)
    private let right = CGRect(x: 200, y: 0, width: 40, height: 20)
    private let above = CGRect(x: 0, y: 200, width: 40, height: 20)

    func testEndpointsSitBetweenTheTwoBoxes() {
        let line = makeLine(from: left, to: right)

        XCTAssertGreaterThanOrEqual(line.source.x, left.minX)
        XCTAssertLessThanOrEqual(line.target.x, right.maxX)
        XCTAssertLessThan(line.source.x, line.target.x, "The line runs left to right")
    }

    func testEndpointsFollowAVerticalArrangement() {
        let line = makeLine(from: left, to: above)
        XCTAssertLessThan(line.source.y, line.target.y, "The line runs bottom to top")
    }

    func testMovingTheTargetMovesTheEndpoint() {
        let line = makeLine(from: left, to: right)
        let before = line.target

        line.updateLayout(source: left, target: right.offsetBy(dx: 300, dy: 0))
        XCTAssertNotEqual(line.target, before, "Re-layout must track the moved box")
    }

    func testPlainLineHasNoExtraPoints() {
        let line = makeLine(from: left, to: right)
        XCTAssertTrue(line.extraPoints.isEmpty, "A direct line needs no waypoints")
    }

    func testPointConstructorKeepsExactCoordinates() {
        let line = DrawableLine(source: CGPoint(x: 1, y: 2), target: CGPoint(x: 3, y: 4), style: lineStyle())
        XCTAssertEqual(line.source, CGPoint(x: 1, y: 2))
        XCTAssertEqual(line.target, CGPoint(x: 3, y: 4))
        XCTAssertTrue(line.extraPoints.isEmpty)
    }

    func testUpdateLayoutClearsPreviousWaypoints() {
        let style = lineStyle("layout middle")
        let overlappingX = CGRect(x: 0, y: 0, width: 40, height: 20)
        let belowSameX = CGRect(x: 10, y: 200, width: 40, height: 20)

        let line = makeLine(from: overlappingX, to: belowSameX, style: style)
        XCTAssertFalse(line.extraPoints.isEmpty, "The middle layout routes around the boxes")
        let before = line.extraPoints

        line.updateLayout(source: left, target: right)
        XCTAssertEqual(line.extraPoints.count, before.count, "Waypoints are rebuilt, not appended to")
        XCTAssertNotEqual(line.extraPoints, before, "The new positions replace the old ones")
    }

    func testMiddleLayoutRoutesAroundVerticallyStackedBoxes() {
        let style = lineStyle("layout middle")
        let lower = CGRect(x: 0, y: 0, width: 40, height: 20)
        let upper = CGRect(x: 10, y: 300, width: 40, height: 20)

        let line = makeLine(from: lower, to: upper, style: style)
        XCTAssertEqual(line.extraPoints.count, 2, "Two waypoints make the detour")
        XCTAssertEqual(line.extraPoints[0].x, line.extraPoints[1].x, accuracy: 0.001, "The detour runs along one vertical")
    }

    func testMiddleLayoutRoutesAroundHorizontallyPlacedBoxes() {
        let style = lineStyle("layout middle")
        let a = CGRect(x: 0, y: 0, width: 40, height: 20)
        let b = CGRect(x: 300, y: 5, width: 40, height: 20)

        let line = makeLine(from: a, to: b, style: style)
        XCTAssertEqual(line.extraPoints.count, 2)
        XCTAssertEqual(line.extraPoints[0].y, line.extraPoints[1].y, accuracy: 0.001, "The detour runs along one horizontal")
    }

    func testFindHitsAPointOnTheLine() {
        let line = DrawableLine(source: CGPoint(x: 0, y: 0), target: CGPoint(x: 100, y: 0), style: lineStyle())
        XCTAssertTrue(line.find(CGPoint(x: 50, y: 0)))
    }

    func testFindMissesAPointOffTheLine() {
        let line = DrawableLine(source: CGPoint(x: 0, y: 0), target: CGPoint(x: 100, y: 0), style: lineStyle())
        XCTAssertFalse(line.find(CGPoint(x: 50, y: 80)))
    }

    func testFindFollowsTheWaypointsOfARoutedLine() {
        let style = lineStyle("layout middle")
        let lower = CGRect(x: 0, y: 0, width: 40, height: 20)
        let upper = CGRect(x: 10, y: 300, width: 40, height: 20)
        let line = makeLine(from: lower, to: upper, style: style)

        let waypoint = line.extraPoints[0]
        XCTAssertTrue(line.find(waypoint), "A point on the detour still belongs to the line")
    }

    func testLabelIsAttachedWithANonEmptyBox() {
        let item = DiagramItem(kind: .Item, name: "Host")
        let line = makeLine(from: left, to: right)
        line.addLabel("edge label", imageProvider: ElementImageProvider(item, 1))

        XCTAssertNotNil(line.label)
        XCTAssertGreaterThan(line.label!.getBounds().width, 0)
    }

    func testLabelHonoursEscapedNewlines() {
        let item = DiagramItem(kind: .Item, name: "Host")
        let single = makeLine(from: left, to: right)
        single.addLabel("one", imageProvider: ElementImageProvider(item, 1))

        let double = makeLine(from: left, to: right)
        double.addLabel("one\\ntwo", imageProvider: ElementImageProvider(item, 1))

        XCTAssertGreaterThan(
            double.label!.getBounds().height, single.label!.getBounds().height, "A two-line label is taller")
    }

    func testBoundsCoverBothEndpoints() {
        let line = makeLine(from: left, to: right)
        let bounds = line.getBounds()

        XCTAssertLessThanOrEqual(bounds.minX, min(line.source.x, line.target.x) + 0.001)
        XCTAssertGreaterThanOrEqual(bounds.maxX, max(line.source.x, line.target.x) - 0.001)
    }

    func testRenderIsAStub() {
        XCTAssertEqual(makeLine(from: left, to: right).render(type: .svg), "")
    }

    func testIdenticalBoxesDoNotProduceInvalidCoordinates() {
        let box = CGRect(x: 10, y: 10, width: 40, height: 20)
        let line = makeLine(from: box, to: box)

        XCTAssertTrue(line.source.x.isFinite)
        XCTAssertTrue(line.source.y.isFinite)
        XCTAssertTrue(line.target.x.isFinite)
        XCTAssertTrue(line.target.y.isFinite)
    }
}

// MARK: - Image provider

class ElementImageProviderTests: XCTestCase {

    func testUnknownImageNameResolvesToNil() {
        let provider = ElementImageProvider(DiagramItem(kind: .Item, name: "Host"), 1)
        XCTAssertNil(provider.resolveImage(name: "missing"))
    }

    func testImageDeclaredOnTheElementIsResolved() {
        // 1x1 transparent PNG.
        let png =
            "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
        let item = DiagramItem(kind: .Item, name: "Host")
        item.properties.append(TennNode.newCommand("image", TennNode.newIdent("logo"), TennNode.newIdent(png)))

        let provider = ElementImageProvider(item, 1)
        let image = provider.resolveImage(name: "logo")
        XCTAssertNotNil(image, "A base64 image declared on the element must resolve")
    }

    func testBrokenImageDataResolvesToNil() {
        let item = DiagramItem(kind: .Item, name: "Host")
        item.properties.append(TennNode.newCommand("image", TennNode.newIdent("broken"), TennNode.newIdent("not-base64!!")))

        XCTAssertNil(ElementImageProvider(item, 1).resolveImage(name: "broken"))
    }

    func testRescaleProducesTheRequestedSize() {
        let original = NSImage(size: CGSize(width: 10, height: 10))
        let scaled = rescaleImage(original, 40, 20)
        XCTAssertEqual(scaled.size, CGSize(width: 40, height: 20))
    }
}
