//
//  ModelCoreTests.swift
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

// MARK: - TennNode

class TennNodeTests: XCTestCase {

    func testCountReflectsChildren() {
        let node = TennNode.newNode(kind: .Statements)
        XCTAssertEqual(node.count, 0)
        node.add(TennNode.newIdent("a"))
        node.add(TennNode.newIdent("b"))
        XCTAssertEqual(node.count, 2)
    }

    func testCloneIsDeepAndIndependent() {
        let root = TennNode.newBlockExpr(TennNode.newCommand("pos", TennNode.newIntNode(1), TennNode.newIntNode(2)))
        let copy = root.clone()

        XCTAssertEqual(copy.toStr(), root.toStr())

        copy.add(TennNode.newCommand("extra", TennNode.newIdent("x")))
        XCTAssertNotEqual(copy.count, root.count, "Mutating the clone must not touch the original")
    }

    func testReplaceSwapsAllChildren() {
        let target = TennNode.newBlockExpr(TennNode.newCommand("a", TennNode.newIntNode(1)))
        let source = TennNode.newBlockExpr(
            TennNode.newCommand("b", TennNode.newIntNode(2)), TennNode.newCommand("c", TennNode.newIntNode(3)))

        target.replace(source)
        XCTAssertEqual(target.count, 2)
        XCTAssertEqual(target.getIdent(0, 0), "b")
    }

    func testNamedElementLookupInBlock() {
        let block = TennNode.newBlockExpr(
            TennNode.newCommand("color", TennNode.newIdent("red")), TennNode.newCommand("size", TennNode.newIntNode(10)))

        XCTAssertNotNil(block.getNamedElement("color"))
        XCTAssertNil(block.getNamedElement("missing"))
    }

    func testNamedLookupOnlyWorksForBlocks() {
        let statements = TennNode.newNode(kind: .Statements)
        statements.add(TennNode.newCommand("color", TennNode.newIdent("red")))
        XCTAssertNil(statements.getNamedElement("color"), "Only BlockExpr keeps a named index")
    }

    func testRemoveNamedReportsWhetherItRemovedAnything() {
        let block = TennNode.newBlockExpr(TennNode.newCommand("color", TennNode.newIdent("red")))
        XCTAssertTrue(block.removeNamed("color"))
        XCTAssertFalse(block.removeNamed("color"), "Second removal has nothing left to drop")
    }

    func testGetIdentTextForEachLiteralKind() {
        XCTAssertEqual(TennNode.newIdent("sym").getIdentText(), "sym")
        XCTAssertEqual(TennNode.newStrNode("text").getIdentText(), "text")
        XCTAssertEqual(TennNode.newIntNode(7).getIdentText(), "7")
        XCTAssertEqual(TennNode.newMarkdownNode("**b**").getIdentText(), "**b**")
        XCTAssertNil(TennNode.newNode(kind: .Statements).getIdentText(), "Containers carry no literal")
    }

    func testGetIntAndGetFloat() {
        let cmd = TennNode.newCommand("pos", TennNode.newIntNode(42), TennNode.newFloatNode(1.5))
        XCTAssertEqual(cmd.getInt(1), 42)
        XCTAssertEqual(cmd.getFloat(2), 1.5)
        XCTAssertNil(cmd.getInt(0), "The command name is not a number")
    }

    func testGetChildOutOfRangeIsNil() {
        let cmd = TennNode.newCommand("a", TennNode.newIdent("b"))
        XCTAssertNil(cmd.getChild(99))
        XCTAssertNil(cmd.getChild([0, 0, 0]), "Walking past a leaf yields nil")
    }

    func testGetBlockReturnsChildrenOfNestedBlock() {
        let inner = TennNode.newBlockExpr(TennNode.newCommand("x", TennNode.newIntNode(1)))
        let cmd = TennNode.newCommand("item", inner)
        XCTAssertEqual(cmd.getBlock(1).count, 1)
        XCTAssertTrue(cmd.getBlock(99).isEmpty, "Missing block reads as empty")
    }

    func testIsNamedElement() {
        XCTAssertTrue(TennNode.newCommand("color", TennNode.newIdent("red")).isNamedElement())
        XCTAssertFalse(TennNode.newNode(kind: .Statements).isNamedElement())
    }

    func testTypedGetValueWithDefaults() {
        let block = TennNode.newBlockExpr(
            TennNode.newCommand("title", TennNode.newStrNode("Hello")),
            TennNode.newCommand("size", TennNode.newIntNode(12)),
            TennNode.newCommand("visible", TennNode.newIdent("true")))

        XCTAssertEqual(block.getValue(name: "title", defaultValue: ""), "Hello")
        XCTAssertEqual(block.getValue(name: "size", defaultValue: 0), 12)
        XCTAssertEqual(block.getValue(name: "visible", defaultValue: false), true)

        XCTAssertEqual(block.getValue(name: "missing", defaultValue: "fallback"), "fallback")
        XCTAssertEqual(block.getValue(name: "missing", defaultValue: 99), 99)
        XCTAssertEqual(block.getValue(name: "missing", defaultValue: true), true)
    }

    func testTraverseVisitsEveryNodeAndCanStopEarly() {
        let root = TennNode.newBlockExpr(
            TennNode.newCommand("a", TennNode.newIntNode(1)), TennNode.newCommand("b", TennNode.newIntNode(2)))

        var all = 0
        root.traverse { _ in
            all += 1
            return true
        }
        XCTAssertGreaterThan(all, 3)

        var stopped = 0
        root.traverse { _ in
            stopped += 1
            return false
        }
        XCTAssertEqual(stopped, 1)
    }
}

// MARK: - SceneMath

class SceneMathTests: XCTestCase {

    func testCrossingLinesMeetInTheMiddle() {
        let cross = crossLine(
            CGPoint(x: -10, y: 0), CGPoint(x: 10, y: 0),
            CGPoint(x: 0, y: -10), CGPoint(x: 0, y: 10))

        XCTAssertNotNil(cross)
        XCTAssertEqual(cross!.x, 0, accuracy: 0.0001)
        XCTAssertEqual(cross!.y, 0, accuracy: 0.0001)
    }

    func testParallelSegmentsDoNotCross() {
        let cross = crossLine(
            CGPoint(x: 0, y: 0), CGPoint(x: 10, y: 0),
            CGPoint(x: 0, y: 5), CGPoint(x: 10, y: 5))
        XCTAssertNil(cross)
    }

    func testSegmentsThatWouldCrossOnlyIfExtendedDoNot() {
        let cross = crossLine(
            CGPoint(x: 0, y: 0), CGPoint(x: 1, y: 0),
            CGPoint(x: 5, y: -5), CGPoint(x: 5, y: 5))
        XCTAssertNil(cross, "Intersection lies outside both segments")
    }

    func testLineEnteringABoxCrossesIt() {
        let box = CGRect(x: 0, y: 0, width: 10, height: 10)
        let cross = crossBox(CGPoint(x: -5, y: 5), CGPoint(x: 5, y: 5), box)

        XCTAssertNotNil(cross)
        XCTAssertEqual(cross!.x, 0, accuracy: 0.0001, "It enters through the left edge")
    }

    func testLineMissingTheBoxDoesNotCross() {
        let box = CGRect(x: 0, y: 0, width: 10, height: 10)
        XCTAssertNil(crossBox(CGPoint(x: -5, y: 50), CGPoint(x: 5, y: 50), box))
    }

    func testPointOnSegmentIsDetected() {
        XCTAssertTrue(crossPointLine(CGPoint(x: 0, y: 0), CGPoint(x: 10, y: 0), CGPoint(x: 5, y: 0)))
    }

    func testPointOffSegmentIsRejected() {
        XCTAssertFalse(crossPointLine(CGPoint(x: 0, y: 0), CGPoint(x: 10, y: 0), CGPoint(x: 5, y: 40)))
    }
}

// MARK: - Property round trips

class ElementPropertiesTests: XCTestCase {

    func testItemPropertiesRoundTrip() {
        let item = DiagramItem(kind: .Item, name: "Node")
        item.x = 11
        item.y = 22
        item.properties.append(TennNode.newCommand("color", TennNode.newIdent("red")))

        let props = item.toTennAsProps()

        let restored = DiagramItem(kind: .Item, name: "")
        restored.fromTennProps(props)

        XCTAssertEqual(restored.name, "Node")
        XCTAssertEqual(restored.x, 11)
        XCTAssertEqual(restored.y, 22)
    }

    func testItemPropertiesRoundTripThroughText() {
        let item = DiagramItem(kind: .Item, name: "Node")
        item.x = 5
        item.y = 6

        let reparsed = item.toTennAsProps(.Statements, reparse: true)
        let restored = DiagramItem(kind: .Item, name: "")
        restored.fromTennProps(reparsed)

        XCTAssertEqual(restored.name, "Node")
        XCTAssertEqual(restored.x, 5)
    }

    func testDroppingPositionResetsItToZero() {
        let item = DiagramItem(kind: .Item, name: "Node")
        item.x = 100
        item.y = 200

        item.fromTennProps(TennParser().parse("name Node"))
        XCTAssertEqual(item.x, 0, "A property set without pos clears the position")
        XCTAssertEqual(item.y, 0)
    }

    func testElementPropertiesRoundTrip() {
        let element = Element(name: "Diagram")
        element.properties.append(TennNode.newCommand("color", TennNode.newIdent("blue")))

        let restored = Element(name: "")
        restored.fromTennProps(element.toTennAsProps())
        XCTAssertEqual(restored.name, "Diagram")
    }

    func testLinkPropertiesKeepTheirLabel() {
        let source = DiagramItem(kind: .Item, name: "A")
        let target = DiagramItem(kind: .Item, name: "B")
        let link = LinkItem(kind: .Link, name: "edge", source: source, target: target)

        let restored = LinkItem(kind: .Link, name: "", source: source, target: target)
        restored.fromTennProps(link.toTennAsProps())
        XCTAssertEqual(restored.name, "edge")
    }
}

// MARK: - Sync / JSON serialization

class SyncSerializationTests: XCTestCase {

    private func makeDiagram() -> Element {
        let element = Element(name: "Diagram")
        element.description = "a description"

        let a = DiagramItem(kind: .Item, name: "A")
        a.x = 1
        a.y = 2
        a.properties.append(TennNode.newCommand("color", TennNode.newIdent("red")))

        let b = DiagramItem(kind: .Item, name: "B")
        b.x = 3
        b.y = 4

        element.add(a)
        element.add(b)
        element.add(source: a, target: b)
        return element
    }

    func testSyncSplitsItemsFromEdges() {
        let sync = makeDiagram().toSync()

        XCTAssertEqual(sync.name, "Diagram")
        XCTAssertEqual(sync.description, "a description")
        XCTAssertEqual(sync.items?.count, 2)
        XCTAssertEqual(sync.edges?.count, 1)
    }

    func testSyncEdgeCarriesSourceAndTargetIds() {
        let element = makeDiagram()
        let sync = element.toSync()

        guard let edge = sync.edges?.first else { return XCTFail("Expected one edge") }
        let ids = Set(element.items.map { $0.id.uuidString })
        XCTAssertTrue(ids.contains(edge.source ?? ""))
        XCTAssertTrue(ids.contains(edge.target ?? ""))
    }

    func testSyncKeepsItemPositionsAndProperties() {
        let sync = makeDiagram().toSync()
        guard let first = sync.items?.first(where: { $0.name == "A" }) else { return XCTFail("Expected item A") }

        XCTAssertEqual(first.pos.x, 1)
        XCTAssertEqual(first.pos.y, 2)
        XCTAssertNotNil(first.properties, "Item A carries a color property")
    }

    func testEmptyDiagramHasNoItemsOrEdges() {
        let sync = Element(name: "Empty").toSync()
        XCTAssertNil(sync.items, "Empty collections are dropped rather than serialized as []")
        XCTAssertNil(sync.edges)
    }

    func testJsonDecodesBackIntoTheSameShape() throws {
        let json = makeDiagram().toSyncJson()
        let decoded = try JSONDecoder().decode(SyncElement.self, from: Data(json.utf8))

        XCTAssertEqual(decoded.name, "Diagram")
        XCTAssertEqual(decoded.items?.count, 2)
        XCTAssertEqual(decoded.edges?.count, 1)
    }

    func testNewlinesAreEscapedForTransport() {
        let element = Element(name: "Diagram")
        let item = DiagramItem(kind: .Item, name: "two\nlines")
        element.add(item)

        guard let synced = element.toSync().items?.first else { return XCTFail("Expected an item") }
        XCTAssertFalse(synced.name.contains("\n"), "Raw newlines must not reach the JSON payload")
        XCTAssertTrue(synced.name.contains("\\n"))
    }

    func testNodeToSyncFlattensChildren() {
        let block = TennNode.newBlockExpr(
            TennNode.newCommand("color", TennNode.newIdent("red")), TennNode.newCommand("size", TennNode.newIntNode(3)))

        let lines = block.toSync()
        XCTAssertEqual(lines.count, 2)
        XCTAssertFalse(lines.contains(where: { $0.contains("\n") }))
    }

    func testLeafNodeToSyncIsEmpty() {
        XCTAssertTrue(TennNode.newIdent("leaf").toSync().isEmpty)
    }
}

// MARK: - Attributed (syntax-highlighted) printing

class AttributedPrinterTests: XCTestCase {

    private let font = NSFont.systemFont(ofSize: 12)

    func testAttributedOutputMatchesPlainText() {
        let node = TennParser().parse("element Root {\n  item A {\n    pos 1 2\n  }\n}")
        let attributed = node.toAttributedStr(font, NSColor.black)
        XCTAssertEqual(attributed.string, node.toStr(), "Highlighting must not change the text itself")
    }

    func testLiteralsGetTheirOwnColor() {
        let node = TennParser().parse("color red\nsize 10")
        let attributed = node.toAttributedStr(font, NSColor.black)

        var colors: Set<NSColor> = []
        attributed.enumerateAttribute(.foregroundColor, in: NSRange(location: 0, length: attributed.length)) { value, _, _ in
            if let color = value as? NSColor { colors.insert(color) }
        }
        XCTAssertGreaterThan(colors.count, 1, "Symbols and numbers are not painted the same")
    }

    func testEveryCharacterCarriesTheRequestedFont() {
        let node = TennParser().parse("name Value")
        let attributed = node.toAttributedStr(font, NSColor.black)

        var fontRanges = 0
        attributed.enumerateAttribute(.font, in: NSRange(location: 0, length: attributed.length)) { value, _, _ in
            if value != nil { fontRanges += 1 }
        }
        XCTAssertGreaterThan(fontRanges, 0)
    }

    func testIndentationIsApplied() {
        let node = TennParser().parse("name Value")
        let plain = node.toAttributedStr(font, NSColor.black, 0)
        let indented = node.toAttributedStr(font, NSColor.black, 2)
        XCTAssertGreaterThan(indented.length, plain.length, "Indenting adds leading whitespace")
    }

    func testCleanModeStillRendersTheDocument() {
        let node = TennParser().parse("element Root {\n  name Value\n}")
        let clean = node.toAttributedStr(font, NSColor.black, 0, true)
        XCTAssertTrue(clean.string.contains("Root"))
        XCTAssertTrue(clean.string.contains("Value"))
    }

    func testEmptyDocumentProducesEmptyOutput() {
        let node = TennParser().parse("")
        XCTAssertEqual(node.toAttributedStr(font, NSColor.black).length, 0)
    }
}
