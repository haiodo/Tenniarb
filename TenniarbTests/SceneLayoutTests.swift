//
//  SceneLayoutTests.swift
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

/// A diagram with `count` items laid out in a column, plus links chaining them.
private func makeDiagram(items count: Int, linked: Bool = false) -> (ElementModelStore, Element, [DiagramItem]) {
    let model = ElementModel()
    let diagram = Element(name: "Diagram")
    model.add(diagram)

    var items: [DiagramItem] = []
    for i in 0..<count {
        let item = DiagramItem(kind: .Item, name: "Item\(i)")
        item.x = Double(i) * 10
        item.y = Double(i) * 10
        diagram.add(item)
        items.append(item)
    }
    if linked {
        for i in 1..<max(count, 1) {
            diagram.add(source: items[i - 1], target: items[i])
        }
    }
    return (ElementModelStore(model), diagram, items)
}

private func makeScene(_ element: Element) -> DrawableScene {
    let context = ExecutionContext()
    context.setElement(element)
    return DrawableScene(element, darkMode: false, executionContext: context)
}

private func makeContext(_ store: ElementModelStore, _ element: Element, bounds: CGRect = CGRect(x: 0, y: 0, width: 800, height: 600))
    -> LayoutContext
{
    return LayoutContext(element, scene: makeScene(element), store: store, bounds: bounds)
}

// MARK: - LayoutContext

class LayoutContextTests: XCTestCase {

    func testFindNodesSkipsLinks() {
        let (store, diagram, items) = makeDiagram(items: 3, linked: true)
        let context = makeContext(store, diagram)

        XCTAssertEqual(context.nodes.count, items.count, "Only .Item entries count as nodes")
        XCTAssertTrue(context.nodes.allSatisfy { $0.kind == .Item })
    }

    func testFindEdgesReturnsOnlyLinks() {
        let (store, diagram, _) = makeDiagram(items: 3, linked: true)
        let context = makeContext(store, diagram)

        XCTAssertEqual(context.edges.count, 2, "Three chained items produce two links")
        XCTAssertTrue(context.edges.allSatisfy { $0.kind == .Link })
    }

    func testViewBoundsAreTheOnesPassedIn() {
        let (store, diagram, _) = makeDiagram(items: 1)
        let bounds = CGRect(x: 5, y: 6, width: 100, height: 200)
        let context = makeContext(store, diagram, bounds: bounds)

        XCTAssertEqual(context.getViewBounds(), bounds)
    }

    func testBoundsOfKnownNodeIsNonEmpty() {
        let (store, diagram, items) = makeDiagram(items: 1)
        let context = makeContext(store, diagram)

        let bounds = context.getBounds(node: items[0])
        XCTAssertGreaterThan(bounds.width, 0, "A drawn item has a real size")
        XCTAssertGreaterThan(bounds.height, 0)
    }

    func testBoundsOfUnknownNodeIsZero() {
        let (store, diagram, _) = makeDiagram(items: 1)
        let context = makeContext(store, diagram)

        let stranger = DiagramItem(kind: .Item, name: "NotInScene")
        XCTAssertEqual(context.getBounds(node: stranger), CGRect(), "Items without a drawable report an empty rect")
    }

    func testEveryNodeIsMovable() {
        let (store, diagram, items) = makeDiagram(items: 2)
        let context = makeContext(store, diagram)
        XCTAssertTrue(items.allSatisfy { context.isMovable($0) })
    }

    func testPrePostLayoutPassesRunInOrder() {
        let (store, diagram, _) = makeDiagram(items: 1)
        let context = makeContext(store, diagram)

        var order: [String] = []
        context.preLayoutPass = [{ order.append("pre1") }, { order.append("pre2") }]
        context.postLayoutPass = [{ order.append("post") }]

        context.preLayout()
        context.postLayout()
        XCTAssertEqual(order, ["pre1", "pre2", "post"])
    }

    func testApplyWithoutAlgorithmReturnsNoOperations() {
        let (store, diagram, _) = makeDiagram(items: 2)
        let context = makeContext(store, diagram)
        XCTAssertTrue(context.apply(true).isEmpty, "No layout set means nothing to do")
    }

    func testApplyRunsPassesAroundTheAlgorithm() {
        let (store, diagram, _) = makeDiagram(items: 2)
        let context = makeContext(store, diagram)

        var order: [String] = []
        context.preLayoutPass = [{ order.append("pre") }]
        context.postLayoutPass = [{ order.append("post") }]
        context.layout = GridLayout()

        _ = context.apply(true)
        XCTAssertEqual(order, ["pre", "post"], "Passes bracket the algorithm run")
    }
}

// MARK: - Layout algorithms

class LayoutAlgorithmTests: XCTestCase {

    func testGridLayoutProducesOnePositionOperationPerNode() {
        let (store, diagram, items) = makeDiagram(items: 4)
        let context = makeContext(store, diagram)

        let ops = GridLayout().apply(context: context, clean: true)
        XCTAssertEqual(ops.count, items.count)
    }

    func testGridLayoutIsNoOpWhenNotClean() {
        let (store, diagram, _) = makeDiagram(items: 4)
        let context = makeContext(store, diagram)

        XCTAssertTrue(GridLayout().apply(context: context, clean: false).isEmpty)
    }

    func testGridLayoutMovesItemsApart() {
        let (store, diagram, items) = makeDiagram(items: 4)
        let context = makeContext(store, diagram)

        for op in GridLayout().apply(context: context, clean: true) {
            op.apply()
        }

        let positions = Set(items.map { "\($0.x),\($0.y)" })
        XCTAssertEqual(positions.count, items.count, "Grid layout gives every item a distinct cell")
    }

    func testGridLayoutOnEmptyDiagramProducesNothing() {
        let (store, diagram, _) = makeDiagram(items: 0)
        let context = makeContext(store, diagram)
        XCTAssertTrue(GridLayout().apply(context: context, clean: true).isEmpty)
    }

    func testSpringLayoutProducesPositionOperations() {
        let (store, diagram, items) = makeDiagram(items: 5, linked: true)
        let context = makeContext(store, diagram)

        let ops = SpringLayout().apply(context: context, clean: true)
        XCTAssertEqual(ops.count, items.count, "Every node gets a new position")
    }

    func testSpringLayoutIsNoOpWhenNotClean() {
        let (store, diagram, _) = makeDiagram(items: 5, linked: true)
        let context = makeContext(store, diagram)

        XCTAssertTrue(SpringLayout().apply(context: context, clean: false).isEmpty)
    }

    func testSpringLayoutKeepsItemsWithinFiniteCoordinates() {
        let (store, diagram, items) = makeDiagram(items: 6, linked: true)
        let context = makeContext(store, diagram)

        for op in SpringLayout().apply(context: context, clean: true) {
            op.apply()
        }

        for item in items {
            XCTAssertTrue(item.x.isFinite, "\(item.name) must not drift to NaN/infinity")
            XCTAssertTrue(item.y.isFinite, "\(item.name) must not drift to NaN/infinity")
        }
    }

    func testSpringLayoutOnSingleNode() {
        let (store, diagram, _) = makeDiagram(items: 1)
        let context = makeContext(store, diagram)

        let ops = SpringLayout().apply(context: context, clean: true)
        XCTAssertEqual(ops.count, 1, "A lone node still gets placed")
    }

    func testTreeLayoutIsStillAStub() {
        let (store, diagram, _) = makeDiagram(items: 3, linked: true)
        let context = makeContext(store, diagram)
        XCTAssertTrue(TreeLayout().apply(context: context, clean: true).isEmpty, "TreeLayout is not implemented yet")
    }
}

// MARK: - DrawableScene

class DrawableSceneTests: XCTestCase {

    func testSceneBuildsADrawablePerItem() {
        let (_, diagram, items) = makeDiagram(items: 3)
        let scene = makeScene(diagram)

        for item in items {
            XCTAssertNotNil(scene.drawables[item], "\(item.name) must have a drawable")
        }
    }

    func testSceneBoundsCoverAllItems() {
        let (_, diagram, _) = makeDiagram(items: 3)
        let scene = makeScene(diagram)

        let bounds = scene.getBounds()
        XCTAssertGreaterThan(bounds.width, 0)
        XCTAssertGreaterThan(bounds.height, 0)
    }

    func testEmptySceneHasEmptyBounds() {
        let (_, diagram, _) = makeDiagram(items: 0)
        let scene = makeScene(diagram)
        XCTAssertEqual(scene.getBounds().width, 0)
    }

    func testFindLocatesTheItemUnderAPoint() {
        let (_, diagram, items) = makeDiagram(items: 1)
        let scene = makeScene(diagram)

        guard let drawable = scene.drawables[items[0]] else {
            return XCTFail("Expected a drawable for the only item")
        }
        let bounds = drawable.getBounds()
        let found = scene.find(CGPoint(x: bounds.midX, y: bounds.midY), allowAll: true)
        XCTAssertTrue(found.contains(where: { $0.item == items[0] }), "The item under the point must be found")
    }

    func testFindReturnsNothingFarAwayFromItems() {
        let (_, diagram, _) = makeDiagram(items: 1)
        let scene = makeScene(diagram)
        XCTAssertTrue(scene.find(CGPoint(x: 100_000, y: 100_000)).isEmpty)
    }

    func testTraverseVisitsSelfAndDirectChildren() {
        let (_, diagram, _) = makeDiagram(items: 3, linked: true)
        let scene = makeScene(diagram)

        var visited = 0
        scene.traverse { _ in
            visited += 1
            return true
        }
        // DrawableContainer.traverse is one level deep: it visits itself and its direct
        // children, without recursing into nested containers.
        XCTAssertEqual(visited, 2)
    }

    func testTraverseStopsWhenVisitorReturnsFalse() {
        let (_, diagram, _) = makeDiagram(items: 3, linked: true)
        let scene = makeScene(diagram)

        var visited = 0
        scene.traverse { _ in
            visited += 1
            return false
        }
        XCTAssertEqual(visited, 1, "Returning false from the visitor stops the walk")
    }

    func testSvgRenderIsStillAStub() {
        let (_, diagram, _) = makeDiagram(items: 2)
        let scene = makeScene(diagram)
        let bounds = scene.getBounds()
        scene.layout(bounds, bounds)

        // Every render(type:) in ElementScene.swift returns "". Plan item 3 (interactive HTML
        // export) assumes an SVG renderer exists - it does not yet. Flip this test when it lands.
        XCTAssertEqual(scene.render(type: .svg), "", "SVG rendering is not implemented yet")
    }

    func testLinkDrawableFollowsItsEndpoints() {
        let (_, diagram, items) = makeDiagram(items: 2, linked: true)
        let scene = makeScene(diagram)
        let bounds = scene.getBounds()
        scene.layout(bounds, bounds)

        guard let link = diagram.items.compactMap({ $0 as? LinkItem }).first,
            let linkDrawable = scene.drawables[link]
        else {
            return XCTFail("Expected a drawable for the link")
        }

        let before = linkDrawable.getBounds()

        items[1].x += 500
        let moved = makeScene(diagram)
        let movedBounds = moved.getBounds()
        moved.layout(movedBounds, movedBounds)

        guard let movedLink = moved.drawables[link] else {
            return XCTFail("Expected a drawable for the link after the move")
        }
        XCTAssertNotEqual(movedLink.getBounds(), before, "Moving an endpoint must move the link")
    }
}
