//
//  OutlineAndTextTests.swift
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

private let treeSource = """
    element Root {
        element Branch {
            element Leaf {
                item Deep {
                    pos 0 0
                }
            }
            item Middle {
                pos 0 0
            }
        }
        element Sibling {
            item Other {
                pos 0 0
            }
        }
        item Alpha {
            pos 10 20
            color red
        }
        item Beta {
            pos 200 40
        }
        link Alpha Beta {
        }
    }
    """

@MainActor
private func makeController(_ source: String = treeSource) -> (ViewController, ElementModelStore, Element) {
    let model = ElementModel.parseTenn(node: TennParser().parse(source))
    let store = ElementModelStore(model)

    let controller = ViewController()
    let window = NSWindow(
        contentRect: NSRect(x: 0, y: 0, width: 944, height: 764),
        styleMask: [.titled, .resizable], backing: .buffered, defer: false)
    window.contentViewController = controller
    controller.loadViewIfNeeded()
    controller.viewDidLoad()
    controller.setElementModel(elementStore: store)

    let diagram = model.elements[0]
    controller.onElementSelected(diagram)
    window.makeFirstResponder(controller.scene)
    return (controller, store, diagram)
}

@MainActor
private func items(_ element: Element) -> [DiagramItem] {
    return element.items.filter { $0.kind == .Item }
}

@MainActor
private func drainMainQueue(_ test: XCTestCase, after delay: TimeInterval = 0) {
    let done = test.expectation(description: "main queue drained")
    DispatchQueue.main.asyncAfter(deadline: .now() + delay) { done.fulfill() }
    test.wait(for: [done], timeout: 5)
}

// MARK: - Outline tree

@MainActor
class OutlineDelegateTests: XCTestCase {

    private func delegate() -> (OutlineViewControllerDelegate, ViewController, Element) {
        let (controller, _, diagram) = makeController()
        return (OutlineViewControllerDelegate(controller), controller, diagram)
    }

    func testRootLevelCountsTheTopElements() {
        let (outlineDelegate, controller, _) = delegate()
        let outline = controller.worldTree!

        let rootCount = outlineDelegate.outlineView(outline, numberOfChildrenOfItem: nil)
        XCTAssertGreaterThan(rootCount, 0)
    }

    func testChildCountFollowsTheNestedElements() {
        let (outlineDelegate, controller, diagram) = delegate()
        let outline = controller.worldTree!

        XCTAssertEqual(outlineDelegate.outlineView(outline, numberOfChildrenOfItem: diagram), diagram.elements.count)
    }

    func testLeafElementsHaveNoChildren() {
        let (outlineDelegate, controller, diagram) = delegate()
        let outline = controller.worldTree!
        let leaf = diagram.elements.first(where: { $0.elements.isEmpty })

        XCTAssertNotNil(leaf)
        XCTAssertEqual(outlineDelegate.outlineView(outline, numberOfChildrenOfItem: leaf!), 0)
    }

    func testChildLookupReturnsTheNestedElement() {
        let (outlineDelegate, controller, diagram) = delegate()
        let outline = controller.worldTree!

        let child = outlineDelegate.outlineView(outline, child: 0, ofItem: diagram) as? Element
        XCTAssertEqual(child, diagram.elements[0])
    }

    func testExpandabilityMatchesHavingChildren() {
        let (outlineDelegate, controller, diagram) = delegate()
        let outline = controller.worldTree!

        let branch = diagram.elements.first(where: { !$0.elements.isEmpty })!
        let leaf = diagram.elements.first(where: { $0.elements.isEmpty })!

        XCTAssertTrue(outlineDelegate.outlineView(outline, isItemExpandable: branch))
        XCTAssertFalse(outlineDelegate.outlineView(outline, isItemExpandable: leaf))
    }

    func testObjectValueIsTheElementName() {
        let (outlineDelegate, controller, diagram) = delegate()
        let outline = controller.worldTree!

        let value = outlineDelegate.outlineView(outline, objectValueFor: nil, byItem: diagram)
        XCTAssertEqual(value as? String, diagram.name)
    }

    func testEditingIsAllowedForElements() {
        let (outlineDelegate, controller, diagram) = delegate()
        let outline = controller.worldTree!
        _ = outlineDelegate.outlineView(outline, shouldEdit: nil, item: diagram)
    }

    func testMenuIsBuiltForTheOutline() {
        let (outlineDelegate, _, _) = delegate()
        _ = outlineDelegate.createMenu()
    }

    func testMenuValidationIsAnswered() {
        let (outlineDelegate, _, _) = delegate()
        let item = NSMenuItem()
        item.action = #selector(OutlineViewControllerDelegate.addElementAction(_:))
        _ = outlineDelegate.validateMenuItem(item)
    }

    func testAddElementActionGrowsTheTree() {
        let (controller, _, diagram) = makeController()
        let outlineDelegate = OutlineViewControllerDelegate(controller)
        controller.onElementSelected(diagram)
        let before = diagram.elements.count

        outlineDelegate.addElementAction(NSMenuItem())
        XCTAssertGreaterThan(diagram.elements.count, before)
    }

    func testDuplicateElementAddsACopy() {
        let (controller, _, diagram) = makeController()
        let outlineDelegate = OutlineViewControllerDelegate(controller)
        controller.onElementSelected(diagram.elements[0])
        let before = diagram.elements.count

        outlineDelegate.duplicateElementAction(NSMenuItem())
        XCTAssertGreaterThanOrEqual(diagram.elements.count, before)
    }

    func testRowViewIsProvided() {
        let (outlineDelegate, controller, diagram) = delegate()
        let outline = controller.worldTree!
        XCTAssertNotNil(outlineDelegate.outlineView(outline, rowViewForItem: diagram))
    }

    func testPasteboardWriterIsOfferedForElements() {
        let (outlineDelegate, controller, diagram) = delegate()
        let outline = controller.worldTree!
        _ = outlineDelegate.outlineView(outline, pasteboardWriterForItem: diagram)
    }
}

// MARK: - Outline view commands

@MainActor
class OutlineViewCommandTests: XCTestCase {

    func testCopyPutsTheSelectedElementOnThePasteboard() {
        let (controller, _, _) = makeController()
        guard let outline = controller.worldTree as? OutlineNSOutlineView else {
            return XCTFail("Expected the custom outline view")
        }
        outline.selectRowIndexes(IndexSet(integer: 0), byExtendingSelection: false)
        NSPasteboard.general.clearContents()

        outline.copy(NSMenuItem())
        XCTAssertNotNil(NSPasteboard.general.string(forType: .string))
    }

    func testMenuValidationOnTheOutlineView() {
        let (controller, _, _) = makeController()
        guard let outline = controller.worldTree as? OutlineNSOutlineView else {
            return XCTFail("Expected the custom outline view")
        }
        for selector in [#selector(OutlineNSOutlineView.copy(_:)), #selector(OutlineNSOutlineView.paste(_:))] {
            let item = NSMenuItem()
            item.action = selector
            _ = outline.validateMenuItem(item)
        }
    }

    func testTextFieldCellIsUsable() {
        let cell = OutlineTextFieldCell(textCell: "Name")
        XCTAssertEqual(cell.stringValue, "Name")
        _ = cell.drawingRect(forBounds: NSRect(x: 0, y: 0, width: 100, height: 20))
    }
}

// MARK: - Text properties pane

@MainActor
class TextPropertiesDelegateTests: XCTestCase {

    /// TextPropertiesDelegate force-casts its text view to TennTextView, so reuse the
    /// one the controller already built.
    private func delegate() -> (TextPropertiesDelegate, ViewController, Element) {
        let (controller, _, diagram) = makeController()
        return (TextPropertiesDelegate(controller, controller.textView), controller, diagram)
    }

    func testShowingAnItemFillsTheTextView() {
        let (textDelegate, _, diagram) = delegate()
        let first = items(diagram)[0]

        textDelegate.setTextValue(diagram, first)
        XCTAssertTrue(textDelegate.view.string.contains("Alpha"))
        XCTAssertTrue(textDelegate.view.string.contains("color"))
    }

    func testShowingAnElementFillsTheTextView() {
        let (textDelegate, _, diagram) = delegate()

        textDelegate.setTextValue(diagram, nil)
        XCTAssertTrue(textDelegate.view.string.contains(diagram.name))
    }

    func testShowingNothingLeavesTheViewAlone() {
        let (textDelegate, _, _) = delegate()
        let before = textDelegate.view.string
        textDelegate.setTextValue(nil, nil)
        XCTAssertEqual(textDelegate.view.string, before, "Nothing to show means nothing changes")
    }

    func testSwitchingBetweenItemsReplacesTheContent() {
        let (textDelegate, _, diagram) = delegate()
        let all = items(diagram)

        textDelegate.setTextValue(diagram, all[0])
        let firstText = textDelegate.view.string

        textDelegate.setTextValue(diagram, all[1])
        XCTAssertNotEqual(textDelegate.view.string, firstText)
    }

    func testDelegateSubscribesToModelChangesOnce() {
        let (textDelegate, controller, diagram) = delegate()
        textDelegate.setTextValue(diagram, items(diagram)[0])
        textDelegate.setTextValue(diagram, items(diagram)[1])

        let count = controller.elementStore!.onUpdate.filter { $0 is TextPropertiesDelegate }.count
        XCTAssertEqual(count, 1, "Repeated updates must not subscribe again")
    }

    func testGeneratedContentRoundTripsThroughTheParser() {
        let (textDelegate, _, diagram) = delegate()
        textDelegate.setTextValue(diagram, items(diagram)[0])

        let parser = TennParser()
        let node = textDelegate.generateTextContent(parser)
        XCTAssertFalse(parser.errors.hasErrors())
        XCTAssertTrue(node.toStr().contains("Alpha"))
    }

    func testHighlightingKeepsTheText() {
        let (textDelegate, _, diagram) = delegate()
        textDelegate.setTextValue(diagram, items(diagram)[0])
        let before = textDelegate.view.string

        textDelegate.highlight()
        XCTAssertEqual(textDelegate.view.string, before, "Syntax colouring must not alter the source")
    }

    func testModelChangesAreAccepted() {
        let (textDelegate, _, diagram) = delegate()
        textDelegate.setTextValue(diagram, items(diagram)[0])
        textDelegate.notifyChanges(ModelEvent(kind: .Layout, element: diagram))
        drainMainQueue(self, after: 0.1)
    }

    func testTextChangeIsCounted() {
        let (textDelegate, _, diagram) = delegate()
        textDelegate.setTextValue(diagram, items(diagram)[0])
        let before = textDelegate.changes

        // Only the bookkeeping is checked here: the deferred half of sheduleUpdate ends in
        // ViewController.mergeProperties, which force-unwraps undoManager and so needs a
        // key window to run.
        textDelegate.textDidChange(Notification(name: NSText.didChangeNotification))
        XCTAssertEqual(textDelegate.changes, before + 1)
    }

    func testEndEditingIsHandled() {
        let (textDelegate, _, diagram) = delegate()
        textDelegate.setTextValue(diagram, items(diagram)[0])
        textDelegate.textDidEndEditing(Notification(name: NSText.didEndEditingNotification))
        drainMainQueue(self, after: 0.1)
    }
}

// MARK: - Execution engine, remaining paths

class ExecutionModelExtraTests: XCTestCase {

    private func makeElement(_ source: String) -> Element {
        return ElementModel.parseTenn(node: TennParser().parse(source)).elements[0]
    }

    func testBuiltInNowReturnsATimestamp() {
        let element = makeElement(
            """
            element Root {
                item A {
                    pos 0 0
                    stamp $(utils.now())
                }
            }
            """)
        let ctx = ExecutionContext()
        ctx.setElement(element)

        let values = ctx.getEvaluated(element.items[0]).values.map { $0.toDouble() }
        XCTAssertTrue(values.contains(where: { $0 > 1_000_000_000 }), "utils.now() is a unix timestamp")
    }

    func testTextWidthAndTextSizeAreAvailable() {
        let element = makeElement(
            """
            element Root {
                item A {
                    pos 0 0
                    w $(utils.textWidth("hello", 12))
                    s $(utils.textSize("hello", 12)[0])
                }
            }
            """)
        let ctx = ExecutionContext()
        ctx.setElement(element)

        let values = ctx.getEvaluated(element.items[0]).values.map { $0.toDouble() }
        XCTAssertTrue(values.contains(where: { $0 > 0 }), "Text measurement returns a positive width")
    }

    func testScaleFactorIsAccepted() {
        let ctx = ExecutionContext()
        ctx.setElement(makeElement("element Root {\n  item A {\n    pos 0 0\n  }\n}"))
        ctx.setScaleFactor(2)
    }

    func testElementLevelExpressionsAreEvaluated() {
        let element = makeElement(
            """
            element Root {
                total $(1 + 2)
                item A {
                    pos 0 0
                }
            }
            """)
        let ctx = ExecutionContext()
        ctx.setElement(element)

        let values = ctx.getEvaluated(element).values.map { $0.toInt32() }
        XCTAssertTrue(values.contains(3))
    }

    func testUpdateAllCallsBackWhenDone() {
        let ctx = ExecutionContext()
        ctx.setElement(makeElement("element Root {\n  item A {\n    pos 0 0\n  }\n}"))

        let done = expectation(description: "recalculation finished")
        ctx.updateAll { done.fulfill() }
        wait(for: [done], timeout: 5)
    }

    func testBrokenExpressionDoesNotBringDownTheEngine() {
        let element = makeElement(
            """
            element Root {
                item A {
                    pos 0 0
                    bad $(this is not valid javascript)
                    good $(1 + 1)
                }
            }
            """)
        let ctx = ExecutionContext()
        ctx.setElement(element)

        let values = ctx.getEvaluated(element.items[0]).values.map { $0.toInt32() }
        XCTAssertTrue(values.contains(2), "A broken neighbour must not stop the valid expression")
    }

    func testEvaluatingAnItemWithAnExplicitNode() {
        let element = makeElement(
            """
            element Root {
                item A {
                    pos 0 0
                }
            }
            """)
        let ctx = ExecutionContext()
        ctx.setElement(element)

        let node = TennParser().parse("value $(6 * 7)")
        let evaluated = ctx.getEvaluated(element.items[0], node, nil)
        XCTAssertTrue(evaluated.values.map { $0.toInt32() }.contains(42))
    }

    func testEvaluatingAnElementWithAnExplicitNode() {
        let element = makeElement("element Root {\n  item A {\n    pos 0 0\n  }\n}")
        let ctx = ExecutionContext()
        ctx.setElement(element)

        let node = TennParser().parse("value $(2 + 3)")
        let evaluated = ctx.getEvaluated(element, node)
        XCTAssertTrue(evaluated.values.map { $0.toInt32() }.contains(5))
    }

    func testRelatedItemsAreReachableFromAnExpression() {
        let element = makeElement(
            """
            element Root {
                item A {
                    pos 0 0
                    count $(sources.length + targets.length)
                }
                item B {
                    pos 100 0
                }
                link A B {
                }
            }
            """)
        let ctx = ExecutionContext()
        ctx.setElement(element)

        let evaluated = ctx.getEvaluated(element.items.first(where: { $0.name == "A" })!)
        XCTAssertFalse(evaluated.isEmpty)
    }
}
