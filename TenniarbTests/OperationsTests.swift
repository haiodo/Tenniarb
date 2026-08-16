//
//  OperationsTests.swift
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

/// Root model with one diagram element, plus a store over it.
private func makeStore() -> (ElementModelStore, Element) {
    let model = ElementModel()
    let diagram = Element(name: "Diagram")
    model.add(diagram)
    return (ElementModelStore(model), diagram)
}

private func makeItem(_ name: String, x: Double = 0, y: Double = 0) -> DiagramItem {
    let item = DiagramItem(kind: .Item, name: name)
    item.x = x
    item.y = y
    return item
}

/// Records every ModelEvent delivered to the store listeners.
@MainActor
private final class RecordingListener: IElementModelListener {
    var events: [ModelEvent] = []
    var onEvent: (() -> Void)?

    func notifyChanges(_ event: ModelEvent) {
        XCTAssertTrue(Thread.isMainThread, "Listeners must be notified on the main thread")
        events.append(event)
        onEvent?()
    }
}

// MARK: - Tests

@MainActor
class OperationsTests: XCTestCase {

    // MARK: 1. AddElement / RemoveElement

    func testAddElementApplyUndoIsSymmetric() {
        let (store, diagram) = makeStore()
        let child = Element(name: "Child")

        let op = AddElement(store, diagram, child)
        XCTAssertEqual(diagram.elements.count, 0)

        op.apply()
        XCTAssertEqual(diagram.elements.count, 1)
        XCTAssertTrue(diagram.elements.contains(where: { $0 === child }))
        XCTAssertTrue(child.parent === diagram)

        op.undo()
        XCTAssertEqual(diagram.elements.count, 0)
    }

    func testAddElementAtIndexRestoresPosition() {
        let (store, diagram) = makeStore()
        let a = Element(name: "A")
        let b = Element(name: "B")
        diagram.add(a)
        diagram.add(b)

        let inserted = Element(name: "Inserted")
        let op = AddElement(store, diagram, inserted, index: 1)

        op.apply()
        XCTAssertEqual(diagram.elements.map { $0.name }, ["A", "Inserted", "B"])

        op.undo()
        XCTAssertEqual(diagram.elements.map { $0.name }, ["A", "B"])
    }

    func testRemoveElementUndoRestoresOriginalIndex() {
        let (store, diagram) = makeStore()
        let a = Element(name: "A")
        let b = Element(name: "B")
        let c = Element(name: "C")
        diagram.add(a)
        diagram.add(b)
        diagram.add(c)

        let op = RemoveElement(store, diagram, b)
        op.apply()
        XCTAssertEqual(diagram.elements.map { $0.name }, ["A", "C"])

        op.undo()
        XCTAssertEqual(diagram.elements.map { $0.name }, ["A", "B", "C"], "Undo must restore B at its original index")
    }

    // MARK: 2. AddItem / RemoveItem and link preservation

    func testAddItemApplyUndoIsSymmetric() {
        let (store, diagram) = makeStore()
        let item = makeItem("Node")

        let op = AddItem(store, diagram, item)
        op.apply()
        XCTAssertEqual(diagram.items.count, 1)
        XCTAssertTrue(item.parent === diagram)

        op.undo()
        XCTAssertEqual(diagram.items.count, 0)
    }

    func testRemoveItemUndoRestoresOriginalIndex() {
        let (store, diagram) = makeStore()
        let a = makeItem("A")
        let b = makeItem("B")
        let c = makeItem("C")
        diagram.add(a)
        diagram.add(b)
        diagram.add(c)

        let op = RemoveItem(store, diagram, b)
        op.apply()
        XCTAssertEqual(diagram.items.map { $0.name }, ["A", "C"])

        op.undo()
        XCTAssertEqual(diagram.items.map { $0.name }, ["A", "B", "C"])
    }

    func testRemoveItemAlsoRemovesItsLinks() {
        let (store, diagram) = makeStore()
        let source = makeItem("Source")
        let target = makeItem("Target")
        diagram.add(source: source, target: target)

        XCTAssertEqual(diagram.items.count, 3, "source, target and the link between them")

        // Same set the store builds in remove(_:item:).
        let related = diagram.getRelatedItems(source)
        XCTAssertEqual(related.count, 2, "The item itself plus the link referencing it")

        let composite = CompositeOperation(store, diagram, related.map { RemoveItem(store, diagram, $0) })
        composite.apply()
        XCTAssertEqual(diagram.items.map { $0.name }, ["Target"])

        composite.undo()
        XCTAssertEqual(diagram.items.count, 3)
        let restoredLink = diagram.items.compactMap { $0 as? LinkItem }.first
        XCTAssertNotNil(restoredLink, "The link must come back with its endpoints intact")
        XCTAssertTrue(restoredLink?.source === source)
        XCTAssertTrue(restoredLink?.target === target)
    }

    // MARK: 3. Value updates

    func testUpdatePositionApplyUndo() {
        let (store, diagram) = makeStore()
        let item = makeItem("Node", x: 10, y: 20)
        diagram.add(item)

        let op = UpdatePosition(store, diagram, item, old: CGPoint(x: 10, y: 20), new: CGPoint(x: 100, y: 200))
        op.apply()
        XCTAssertEqual(item.x, 100)
        XCTAssertEqual(item.y, 200)

        op.undo()
        XCTAssertEqual(item.x, 10)
        XCTAssertEqual(item.y, 20)
    }

    func testUpdateNameApplyUndo() {
        let (store, diagram) = makeStore()
        let item = makeItem("Old")
        diagram.add(item)

        let op = UpdateName(store, diagram, item, old: "Old", new: "New")
        op.apply()
        XCTAssertEqual(item.name, "New")

        op.undo()
        XCTAssertEqual(item.name, "Old")
    }

    func testUpdateElementNameApplyUndo() {
        let (store, diagram) = makeStore()

        let op = UpdateElementName(store, diagram, old: "Diagram", new: "Renamed")
        op.apply()
        XCTAssertEqual(diagram.name, "Renamed")

        op.undo()
        XCTAssertEqual(diagram.name, "Diagram")
    }

    func testComplexUpdateItemRoundTripsProperties() {
        let (store, diagram) = makeStore()
        let item = makeItem("Node", x: 5, y: 7)
        diagram.add(item)

        let before = item.toTennAsProps()
        let after = TennParser().parse("name Node\npos 42 43\ncolor red")

        let op = ComplexUpdateItem(store, diagram, item, old: before, new: after)
        op.apply()
        XCTAssertEqual(item.x, 42)
        XCTAssertEqual(item.y, 43)

        op.undo()
        XCTAssertEqual(item.x, 5, "Undo must restore the property snapshot taken before apply")
        XCTAssertEqual(item.y, 7)
    }

    func testComplexUpdateElementRoundTripsProperties() {
        let (store, diagram) = makeStore()

        let before = diagram.toTennAsProps()
        let after = TennParser().parse("name Updated")

        let op = ComplexUpdateElement(store, diagram, old: before, new: after)
        op.apply()
        XCTAssertEqual(diagram.name, "Updated")

        op.undo()
        XCTAssertEqual(diagram.name, "Diagram")
    }

    // MARK: 4. CompositeOperation

    func testCompositeAppliesInOrderAndUndoesInReverse() {
        let (store, diagram) = makeStore()
        let item = makeItem("Node", x: 0, y: 0)
        diagram.add(item)

        let first = UpdatePosition(store, diagram, item, old: CGPoint(x: 0, y: 0), new: CGPoint(x: 1, y: 1))
        let second = UpdatePosition(store, diagram, item, old: CGPoint(x: 1, y: 1), new: CGPoint(x: 2, y: 2))
        let composite = CompositeOperation(store, diagram, first, second)

        composite.apply()
        XCTAssertEqual(item.x, 2, "The last operation in the list wins")

        composite.undo()
        XCTAssertEqual(item.x, 0, "Reverse order undo must walk back through both positions")
    }

    func testCompositeTracksIsUndoCalledOnChildren() {
        let (store, diagram) = makeStore()
        let a = makeItem("A")
        let b = makeItem("B")
        let composite = CompositeOperation(store, diagram, AddItem(store, diagram, a), AddItem(store, diagram, b))

        composite.apply()
        XCTAssertTrue(composite.operations.allSatisfy { !$0.isUndoCalled })

        composite.undo()
        XCTAssertTrue(composite.operations.allSatisfy { $0.isUndoCalled })
    }

    func testCompositeEventKindIsStructureWhenChildrenDisagree() {
        let (store, diagram) = makeStore()
        let empty = CompositeOperation(store, diagram, [])
        XCTAssertEqual(empty.getEventKind(), .Structure, "An empty composite falls back to the base kind")

        let item = makeItem("Node")
        let uniform = CompositeOperation(store, diagram, AddItem(store, diagram, item), RemoveItem(store, diagram, item))
        XCTAssertEqual(uniform.getEventKind(), .Structure)
    }

    func testCompositeNameJoinsChildNames() {
        let (store, diagram) = makeStore()
        let item = makeItem("Node")
        let composite = CompositeOperation(store, diagram, AddItem(store, diagram, item), RemoveItem(store, diagram, item))
        XCTAssertEqual(composite.name, "AddItem,RemoveItem")
    }

    func testCompositeNotifierIsTheGivenElement() {
        let (store, diagram) = makeStore()
        let other = Element(name: "Other")
        let composite = CompositeOperation(store, other, [])
        XCTAssertTrue(composite.getNotifier() === other)
    }

    // MARK: 5. ModelEvent collection

    func testAddItemCollectsAppendThenRemove() {
        let (store, diagram) = makeStore()
        let item = makeItem("Node")
        let op = AddItem(store, diagram, item)

        op.apply()
        op.isUndoCalled = false
        var applied: [DiagramItem: ModelEventOperation] = [:]
        op.collect(&applied)
        XCTAssertEqual(applied[item], .Append)

        op.undo()
        op.isUndoCalled = true
        var undone: [DiagramItem: ModelEventOperation] = [:]
        op.collect(&undone)
        XCTAssertEqual(undone[item], .Remove)
    }

    func testRemoveElementCollectsRemoveThenAppend() {
        let (store, diagram) = makeStore()
        let child = Element(name: "Child")
        diagram.add(child)
        let op = RemoveElement(store, diagram, child)

        op.apply()
        op.isUndoCalled = false
        var applied: [Element: ModelEventOperation] = [:]
        op.collect(&applied)
        XCTAssertEqual(applied[child], .Remove)

        op.undo()
        op.isUndoCalled = true
        var undone: [Element: ModelEventOperation] = [:]
        op.collect(&undone)
        XCTAssertEqual(undone[child], .Append)
    }

    func testCompositeCollectsFromEveryChild() {
        let (store, diagram) = makeStore()
        let a = makeItem("A")
        let b = makeItem("B")
        let composite = CompositeOperation(store, diagram, AddItem(store, diagram, a), AddItem(store, diagram, b))
        composite.apply()

        var items: [DiagramItem: ModelEventOperation] = [:]
        composite.collect(&items)
        XCTAssertEqual(items.count, 2)
        XCTAssertEqual(items[a], .Append)
        XCTAssertEqual(items[b], .Append)
    }

    func testUpdateOperationCollectsUpdate() {
        let (store, diagram) = makeStore()
        let item = makeItem("Node")
        diagram.add(item)

        let op = UpdateName(store, diagram, item, old: "Node", new: "Renamed")
        var items: [DiagramItem: ModelEventOperation] = [:]
        op.collect(&items)
        XCTAssertEqual(items[item], .Update, "Value updates report Update regardless of direction")
    }

    // MARK: 6. Store execute, undo manager and listeners

    func testExecuteAppliesAndNotifiesListenerOnMainThread() {
        let (store, diagram) = makeStore()
        let listener = RecordingListener()
        store.onUpdate = [listener]

        let item = makeItem("Node")
        let notified = expectation(description: "listener notified")
        listener.onEvent = { notified.fulfill() }

        store.add(diagram, item, undoManager: nil, refresh: {})

        // apply() runs synchronously inside execute; notification is dispatched after it.
        XCTAssertEqual(diagram.items.count, 1)

        wait(for: [notified], timeout: 5)
        XCTAssertEqual(listener.events.count, 1)
        let event = listener.events[0]
        XCTAssertTrue(event.element === diagram)
        XCTAssertEqual(event.items[item], .Append)
        XCTAssertTrue(store.modified)
    }

    func testExecuteCallsRefreshAfterApply() {
        let (store, diagram) = makeStore()
        let refreshed = expectation(description: "refresh called")

        store.add(diagram, makeItem("Node"), undoManager: nil, refresh: { refreshed.fulfill() })
        wait(for: [refreshed], timeout: 5)
    }

    func testUndoManagerRoundTripRestoresModel() {
        let (store, diagram) = makeStore()
        let undoManager = UndoManager()
        undoManager.groupsByEvent = false

        let item = makeItem("Node")

        undoManager.beginUndoGrouping()
        store.add(diagram, item, undoManager: undoManager, refresh: {})
        undoManager.endUndoGrouping()
        XCTAssertEqual(diagram.items.count, 1)
        XCTAssertTrue(undoManager.canUndo)

        undoManager.undo()
        XCTAssertEqual(diagram.items.count, 0, "Undo must take the item back out")
        XCTAssertTrue(undoManager.canRedo)

        undoManager.redo()
        XCTAssertEqual(diagram.items.count, 1, "Redo must put it back")
    }

    func testExecuteTogglesBetweenApplyAndUndo() {
        let (store, diagram) = makeStore()
        let item = makeItem("Node")
        let op = AddItem(store, diagram, item)

        store.execute(op, nil, {})
        XCTAssertEqual(diagram.items.count, 1)
        XCTAssertFalse(op.isUndoCalled)

        store.execute(op, nil, {})
        XCTAssertEqual(diagram.items.count, 0, "Re-executing the same operation undoes it")
        XCTAssertTrue(op.isUndoCalled)
    }

    func testAddLinkCreatesEndpointsAndLink() {
        let (store, diagram) = makeStore()
        let source = makeItem("Source")
        let target = makeItem("Target")

        store.add(diagram, source: source, target: target, undoManager: nil, refresh: {})

        XCTAssertEqual(diagram.items.count, 3, "Both endpoints and the link get added")
        let link = diagram.items.compactMap { $0 as? LinkItem }.first
        XCTAssertNotNil(link)
        XCTAssertTrue(link?.source === source)
        XCTAssertTrue(link?.target === target)
    }

    func testMoveElementBetweenParents() {
        let (store, diagram) = makeStore()
        let other = Element(name: "Other")
        diagram.model?.add(other)

        let child = Element(name: "Child")
        diagram.add(child)

        store.move(child, other, undoManager: nil, refresh: {}, index: 0)

        XCTAssertFalse(diagram.elements.contains(where: { $0 === child }))
        XCTAssertTrue(other.elements.contains(where: { $0 === child }))
        XCTAssertTrue(child.parent === other)
    }

    func testRemoveItemThroughStoreDropsRelatedLinks() {
        let (store, diagram) = makeStore()
        let source = makeItem("Source")
        let target = makeItem("Target")
        diagram.add(source: source, target: target)

        store.remove(diagram, item: source, undoManager: nil, refresh: {})

        XCTAssertEqual(diagram.items.map { $0.name }, ["Target"], "Removing an endpoint drops the link too")
    }

    // MARK: 7. Remaining store entry points

    func testStoreUpdateNameForItemAndElement() {
        let (store, diagram) = makeStore()
        let item = makeItem("Old")
        diagram.add(item)

        store.updateName(item: item, "New", undoManager: nil, refresh: {})
        XCTAssertEqual(item.name, "New")

        store.updateName(element: diagram, "Renamed", undoManager: nil, refresh: {})
        XCTAssertEqual(diagram.name, "Renamed")
    }

    func testStoreUpdatePosition() {
        let (store, diagram) = makeStore()
        let item = makeItem("Node", x: 1, y: 2)
        diagram.add(item)

        store.updatePosition(item: item, newPos: CGPoint(x: 30, y: 40), undoManager: nil, refresh: {})
        XCTAssertEqual(item.x, 30)
        XCTAssertEqual(item.y, 40)
    }

    func testStoreAddAndRemoveElements() {
        let (store, diagram) = makeStore()
        let a = Element(name: "A")
        let b = Element(name: "B")

        store.addElements(diagram, [a, b], undoManager: nil, refresh: {})
        XCTAssertEqual(diagram.elements.map { $0.name }, ["A", "B"])

        store.remove(diagram, a, undoManager: nil, refresh: {})
        XCTAssertEqual(diagram.elements.map { $0.name }, ["B"])
    }

    func testStoreAddAndRemoveMultipleItems() {
        let (store, diagram) = makeStore()
        let a = makeItem("A")
        let b = makeItem("B")

        store.add(diagram, [a, b], undoManager: nil, refresh: {})
        XCTAssertEqual(diagram.items.count, 2)

        store.remove(diagram, items: [a, b], undoManager: nil, refresh: {})
        XCTAssertEqual(diagram.items.count, 0)
    }

    func testStoreSetPropertiesOnItemAndElement() {
        let (store, diagram) = makeStore()
        let item = makeItem("Node", x: 1, y: 2)
        diagram.add(item)

        store.setProperties(diagram, item, TennParser().parse("name Node\npos 11 12"), undoManager: nil, refresh: {})
        XCTAssertEqual(item.x, 11)
        XCTAssertEqual(item.y, 12)

        store.setProperties(diagram, TennParser().parse("name Renamed"), undoManager: nil, refresh: {})
        XCTAssertEqual(diagram.name, "Renamed")
    }

    func testCreatePropertiesBuildsUndoableOperation() {
        let (store, diagram) = makeStore()
        let item = makeItem("Node", x: 3, y: 4)
        diagram.add(item)

        let op = store.createProperties(diagram, item, TennParser().parse("name Node\npos 55 66"))
        op.apply()
        XCTAssertEqual(item.x, 55)

        op.undo()
        XCTAssertEqual(item.x, 3)
    }

    func testMakeNonModifiedClearsDirtyFlag() {
        let (store, diagram) = makeStore()
        store.add(diagram, makeItem("Node"), undoManager: nil, refresh: {})

        store.modified = true
        store.makeNonModified()
        XCTAssertFalse(store.modified)
    }

    func testCompositeAddAppendsOperations() {
        let (store, diagram) = makeStore()
        let composite = CompositeOperation(store, diagram, [])
        XCTAssertEqual(composite.operations.count, 0)

        composite.add(AddItem(store, diagram, makeItem("A")), AddItem(store, diagram, makeItem("B")))
        XCTAssertEqual(composite.operations.count, 2)
    }

    func testCreateUpdatePositionCapturesCurrentPositionAsOldValue() {
        let (store, diagram) = makeStore()
        let item = makeItem("Node", x: 7, y: 8)
        diagram.add(item)

        let op = store.createUpdatePosition(item: item, newPos: CGPoint(x: 70, y: 80))
        op.apply()
        XCTAssertEqual(item.x, 70)

        op.undo()
        XCTAssertEqual(item.x, 7, "Old value is the position captured at creation time")
        XCTAssertEqual(item.y, 8)
    }

    func testCreateUpdateOrderMovesItemToNewIndex() {
        let (store, diagram) = makeStore()
        let a = makeItem("A")
        let b = makeItem("B")
        let c = makeItem("C")
        diagram.add(a)
        diagram.add(b)
        diagram.add(c)

        let ops = store.createUpdateOrder(item: c, newPos: 0)
        let composite = CompositeOperation(store, diagram, ops)
        composite.apply()

        XCTAssertEqual(diagram.items.map { $0.name }, ["C", "A", "B"])
    }
}
