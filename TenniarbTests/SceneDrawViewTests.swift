//
//  SceneDrawViewTests.swift
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

/// A view wired up to a real model, without ever attaching it to a window.
@MainActor
private func makeView(source: String? = nil) -> (SceneDrawView, ElementModelStore, Element) {
    let text =
        source
            ?? """
            element Root {
                item First {
                    pos 10 20
                }
                item Second {
                    pos 200 40
                }
                link First Second {
                }
            }
            """
    let parser = TennParser()
    let model = ElementModel.parseTenn(node: parser.parse(text))
    let store = ElementModelStore(model)

    let view = SceneDrawView(frame: NSRect(x: 0, y: 0, width: 800, height: 600))
    view.setModel(store: store)
    let diagram = model.elements[0]
    view.setActiveElement(diagram)
    return (view, store, diagram)
}

private func offscreenContext(width: Int = 800, height: Int = 600) -> CGContext {
    let context = CGContext(
        data: nil, width: width, height: height, bitsPerComponent: 8, bytesPerRow: 0,
        space: CGColorSpaceCreateDeviceRGB(),
        bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
    NSGraphicsContext.current = NSGraphicsContext(cgContext: context, flipped: false)
    return context
}

@MainActor
private func items(_ element: Element) -> [DiagramItem] {
    return element.items.filter { $0.kind == .Item }
}

// MARK: - Model wiring

@MainActor
class SceneDrawViewModelTests: XCTestCase {

    func testSettingAModelBuildsAScene() {
        let (view, _, diagram) = makeView()

        XCTAssertNotNil(view.scene)
        for item in diagram.items {
            XCTAssertNotNil(view.scene?.drawables[item], "\(item.name) needs a drawable")
        }
    }

    func testViewRegistersItselfAsAModelListener() {
        let (view, store, _) = makeView()
        XCTAssertTrue(store.onUpdate.contains(where: { ($0 as AnyObject) === (view as AnyObject) }))
    }

    func testSettingTheSameModelTwiceRegistersOnlyOneListener() {
        let (view, store, _) = makeView()
        view.setModel(store: store)
        let count = store.onUpdate.filter { ($0 as AnyObject) === (view as AnyObject) }.count
        XCTAssertEqual(count, 1, "Re-setting the same store must not double-subscribe")
    }

    func testActivatingADiagramCentresTheView() {
        let (view, _, _) = makeView()
        guard let bounds = view.scene?.getBounds() else { return XCTFail("Expected a scene") }

        XCTAssertEqual(view.ox, -bounds.midX, accuracy: 0.001)
        XCTAssertEqual(view.oy, -bounds.midY, accuracy: 0.001)
        XCTAssertEqual(view.zoomLevel, 1)
    }

    func testOffsetsAreStoredOnTheElement() {
        let (view, _, diagram) = makeView()
        view.ox = 42
        view.oy = 24

        XCTAssertEqual(diagram.ox, 42)
        XCTAssertEqual(diagram.oy, 24)
        XCTAssertEqual(view.ox, 42)
    }

    func testOffsetsAreZeroWithoutAnActiveElement() {
        let view = SceneDrawView(frame: NSRect(x: 0, y: 0, width: 100, height: 100))
        XCTAssertEqual(view.ox, 0)
        XCTAssertEqual(view.oy, 0)
    }

    func testReactivatingTheSameElementIsANoOp() {
        let (view, _, diagram) = makeView()
        view.ox = 123
        view.setActiveElement(diagram)
        XCTAssertEqual(view.ox, 123, "Re-activating the current element must not recentre it")
    }

    func testStructureEventRebuildsTheScene() {
        let (view, _, diagram) = makeView()
        let before = view.scene

        let event = ModelEvent(kind: .Structure, element: diagram)
        view.notifyChanges(event)
        XCTAssertFalse(view.scene === before, "A structural change replaces the scene")
    }

    func testLayoutEventKeepsTheSameScene() {
        let (view, _, diagram) = makeView()
        let before = view.scene

        view.notifyChanges(ModelEvent(kind: .Layout, element: diagram))
        XCTAssertTrue(view.scene === before, "A layout change only redraws")
    }

    func testRemovingASelectedItemDropsItFromTheSelection() {
        let (view, _, diagram) = makeView()
        let all = items(diagram)
        view.setActiveItems(all)
        XCTAssertEqual(view.activeItems.count, all.count)

        let event = ModelEvent(kind: .Structure, element: diagram)
        event.items[all[0]] = .Remove
        view.notifyChanges(event)

        XCTAssertFalse(view.activeItems.contains(all[0]), "A deleted item cannot stay selected")
    }
}

// MARK: - Selection

@MainActor
class SceneDrawViewSelectionTests: XCTestCase {

    func testSelectingASingleItem() {
        let (view, _, diagram) = makeView()
        let first = items(diagram)[0]

        view.setActiveItem(first)
        XCTAssertEqual(view.activeItems, [first])
        XCTAssertEqual(view.scene?.activeElements, [first])
    }

    func testClearingTheSelection() {
        let (view, _, diagram) = makeView()
        view.setActiveItem(items(diagram)[0])

        view.setActiveItem(nil)
        XCTAssertTrue(view.activeItems.isEmpty)
    }

    func testSelectionCallbacksFire() {
        let (view, _, diagram) = makeView()
        var reported: [[DiagramItem]] = []
        view.onSelection = [{ reported.append($0) }]

        let first = items(diagram)[0]
        view.setActiveItem(first)

        XCTAssertEqual(reported.count, 1)
        XCTAssertEqual(reported[0], [first])
    }

    func testSelectingTheSameItemAgainDoesNotNotify() {
        let (view, _, diagram) = makeView()
        let first = items(diagram)[0]
        view.setActiveItem(first)

        var calls = 0
        view.onSelection = [{ _ in calls += 1 }]
        view.setActiveItem(first)
        XCTAssertEqual(calls, 0, "Re-selecting the same item is skipped")
    }

    func testForceReselectNotifiesAnyway() {
        let (view, _, diagram) = makeView()
        let first = items(diagram)[0]
        view.setActiveItems([first])

        var calls = 0
        view.onSelection = [{ _ in calls += 1 }]
        view.setActiveItems([first], force: true)
        XCTAssertEqual(calls, 1)
    }

    func testSelectAllPicksEveryItem() {
        let (view, _, diagram) = makeView()
        view.selectAllItems()
        XCTAssertEqual(Set(view.activeItems), Set(diagram.items))
    }

    func testSelectAllByKindPicksOnlyThatKind() {
        let (view, _, diagram) = makeView()

        view.selectAllByKind(kind: .Item)
        XCTAssertEqual(Set(view.activeItems), Set(items(diagram)))

        view.selectAllByKind(kind: .Link)
        XCTAssertTrue(view.activeItems.allSatisfy { $0.kind == .Link })
        XCTAssertFalse(view.activeItems.isEmpty)
    }

    func testSelectNoneClearsEverything() {
        let (view, _, _) = makeView()
        view.selectAllItems()
        view.selectNoneItems()
        XCTAssertTrue(view.activeItems.isEmpty)
    }

    func testSelectionMovesThePivotPoint() {
        let (view, _, diagram) = makeView()
        let before = view.pivotPoint
        view.setActiveItem(items(diagram)[1])
        XCTAssertNotEqual(view.pivotPoint, before, "The pivot follows the selection")
    }
}

// MARK: - Hit testing and zoom

@MainActor
class SceneDrawViewInteractionTests: XCTestCase {

    func testFindElementLocatesAnItemUnderThePoint() {
        let (view, _, diagram) = makeView()
        let first = items(diagram)[0]
        guard let drawable = view.scene?.drawables[first] else { return XCTFail("Expected a drawable") }
        let bounds = drawable.getBounds()

        let found = view.findElement(x: bounds.midX, y: bounds.midY, allowAll: true)
        XCTAssertTrue(found.contains(where: { $0.item == first }))
    }

    func testFindElementReturnsNothingOverEmptySpace() {
        let (view, _, _) = makeView()
        XCTAssertTrue(view.findElement(x: 100_000, y: 100_000).isEmpty)
    }

    func testZoomInAndOutChangeTheLevelInOppositeDirections() {
        let (view, _, _) = makeView()
        let start = view.zoomLevel

        view.zoomIn(nil)
        let zoomedIn = view.zoomLevel
        XCTAssertGreaterThan(zoomedIn, start)

        view.zoomOut(nil)
        XCTAssertLessThan(view.zoomLevel, zoomedIn)
    }

    func testZoomChangesAreReportedAsPercent() {
        let (view, _, _) = makeView()
        var reported: [Int] = []
        view.onZoomChanged = { reported.append($0) }

        view.zoomLevel = 2
        XCTAssertEqual(reported, [200])
    }

    func testZoomBoundsCoverTheDiagram() {
        let (view, _, _) = makeView()
        let bounds = view.zoomBounds()
        XCTAssertGreaterThan(bounds.width, 0)
        XCTAssertGreaterThan(bounds.height, 0)
    }

    func testCentreItemMovesTheOffsetsOntoIt() {
        let (view, _, diagram) = makeView()
        let second = items(diagram)[1]
        guard let drawable = view.scene?.drawables[second] else { return XCTFail("Expected a drawable") }

        view.centerItem(second, 0)
        XCTAssertEqual(view.ox, -drawable.getBounds().midX, accuracy: 0.001)
    }

    func testDrawRunsAgainstAnOffscreenContext() {
        let (view, _, _) = makeView()
        _ = offscreenContext()
        view.draw(view.bounds)
    }

    func testDrawWithoutAModelIsHarmless() {
        let view = SceneDrawView(frame: NSRect(x: 0, y: 0, width: 100, height: 100))
        _ = offscreenContext(width: 100, height: 100)
        view.draw(view.bounds)
    }

    func testScheduleRedrawIsIdempotent() {
        let (view, _, _) = makeView()
        view.scheduleRedraw()
        XCTAssertTrue(view.drawScheduled)
        view.scheduleRedraw()
        XCTAssertTrue(view.drawScheduled, "A second request folds into the pending one")
    }
}

// MARK: - Property editing

@MainActor
class SceneDrawViewPropertyTests: XCTestCase {

    func testChangingAPropertyOnTheSelectedItem() {
        let (view, _, diagram) = makeView()
        let first = items(diagram)[0]
        view.setActiveItem(first)

        view.changeItemProps("color", TennNode.newIdent("red"))
        XCTAssertEqual(first.properties.node.getNamedElement("color")?.getIdent(1), "red")
    }

    func testChangingAPropertyReplacesAnExistingValue() {
        let (view, _, diagram) = makeView()
        let first = items(diagram)[0]
        view.setActiveItem(first)

        view.changeItemProps("color", TennNode.newIdent("red"))
        view.changeItemProps("color", TennNode.newIdent("blue"))

        XCTAssertEqual(first.properties.node.getNamedElement("color")?.getIdent(1), "blue")
    }

    func testChangingAPropertyNeedsExactlyOneSelectedItem() {
        let (view, _, diagram) = makeView()
        let all = items(diagram)

        view.setActiveItems(all)
        view.changeItemProps("color", TennNode.newIdent("red"))
        for item in all {
            XCTAssertNil(item.properties.node.getNamedElement("color"), "Multi-selection edits are not applied here")
        }
    }

    func testChangingAPropertyWithNothingSelectedIsANoOp() {
        let (view, _, diagram) = makeView()
        view.setActiveItem(nil)
        view.changeItemProps("color", TennNode.newIdent("red"))

        for item in diagram.items {
            XCTAssertNil(item.properties.node.getNamedElement("color"))
        }
    }

    func testMenuActionsAreBuiltFromTheDisplayVariants() {
        let (view, _, _) = makeView()
        let menu = view.createMenu(selector: #selector(SceneDrawView.displayMenuAction(_:)), items: view.itemDisplayVariants)
        XCTAssertEqual(menu.items.count, view.itemDisplayVariants.count)
        XCTAssertEqual(menu.items.first?.title, view.itemDisplayVariants.first)
    }
}
