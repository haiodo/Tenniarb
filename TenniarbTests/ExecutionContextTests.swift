//
//  ExecutionContextTests.swift
//  TenniarbTests
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0

import XCTest

@testable import Tenniarb

// MARK: - Helpers

private func makeExprToken(_ literal: String) -> TennToken {
    return TennToken(type: .expression, literal: literal)
}

private func makeExprNode(_ literal: String) -> TennNode {
    return TennNode.newNode(kind: .Expression, makeExprToken(literal))
}

private func makeExprBlockToken(_ literal: String) -> TennToken {
    return TennToken(type: .expressionBlock, literal: literal)
}

private func makeExprBlockNode(_ literal: String) -> TennNode {
    return TennNode.newNode(kind: .ExpressionBlock, makeExprBlockToken(literal))
}

/// Build a minimal Element with one item whose properties contain the given nodes.
private func makeElement(itemName: String = "A", propertyNodes: [TennNode]) -> (Element, DiagramItem) {
    let element = Element(name: "TestDiagram")
    let item = DiagramItem(kind: .Item, name: itemName)
    for node in propertyNodes {
        item.properties.append(node)
    }
    element.add(item)
    return (element, item)
}

// MARK: - Tests

class ExecutionContextTests: XCTestCase {

    // MARK: 1. Simple expressions on an item

    func testSimpleIntAssignment() {
        // x = 5  -> ExpressionBlock "var x = 5; x"
        // Easier: use Expression node for "x = 5, return x"
        // The engine evaluates Expression nodes as JS and stores result in evaluated dict.
        let exprToken = makeExprToken("5")
        let exprNode = TennNode.newNode(kind: .Expression, exprToken)
        let cmd = TennNode.newCommand("x", exprNode)
        // item.properties.node is the BlockExpr; we add cmd into it directly.
        let item = DiagramItem(kind: .Item, name: "A")
        item.properties.append(cmd)
        let element = Element(name: "Diag")
        element.add(item)

        let ctx = ExecutionContext()
        ctx.setElement(element)

        let evaluated = ctx.getEvaluated(item)
        // Find token matching exprToken
        let value = evaluated[exprToken]
        XCTAssertNotNil(value, "Expression token should have an evaluated value")
        XCTAssertEqual(value?.toInt32(), 5)
    }

    func testChainedExpression() {
        // y = x + 3 where x is set by prior expression
        // We use ExpressionBlock to run multi-statement JS.
        let blockToken = makeExprBlockToken("var x = 5; x")
        let blockNode = TennNode.newNode(kind: .ExpressionBlock, blockToken)
        let xCmd = TennNode.newCommand("x", blockNode)

        let exprToken = makeExprToken("x + 3")
        let exprNode = TennNode.newNode(kind: .Expression, exprToken)
        let yCmd = TennNode.newCommand("y", exprNode)

        let item = DiagramItem(kind: .Item, name: "A")
        item.properties.append(xCmd)
        item.properties.append(yCmd)
        let element = Element(name: "Diag")
        element.add(item)

        let ctx = ExecutionContext()
        ctx.setElement(element)

        let evaluated = ctx.getEvaluated(item)
        // x should be 5
        let xVal = evaluated[blockToken]
        XCTAssertNotNil(xVal)
        XCTAssertEqual(xVal?.toInt32(), 5)
        // y should be 8
        let yVal = evaluated[exprToken]
        XCTAssertNotNil(yVal, "y expression should be evaluated")
        XCTAssertEqual(yVal?.toInt32(), 8)
    }

    func testExpressionWithParens() {
        // z = (x * 2) where x = 5
        let xToken = makeExprBlockToken("var x = 5; x")
        let xNode = TennNode.newNode(kind: .ExpressionBlock, xToken)
        let xCmd = TennNode.newCommand("x", xNode)

        let zToken = makeExprToken("(x * 2)")
        let zNode = TennNode.newNode(kind: .Expression, zToken)
        let zCmd = TennNode.newCommand("z", zNode)

        let item = DiagramItem(kind: .Item, name: "A")
        item.properties.append(xCmd)
        item.properties.append(zCmd)
        let element = Element(name: "Diag")
        element.add(item)

        let ctx = ExecutionContext()
        ctx.setElement(element)

        let evaluated = ctx.getEvaluated(item)
        let zVal = evaluated[zToken]
        XCTAssertNotNil(zVal, "z expression should be evaluated")
        XCTAssertEqual(zVal?.toInt32(), 10)
    }

    // MARK: 2. References to sibling items via `items` proxy

    func testSiblingItemReference() {
        // Item B has a plain int prop "val 42".
        // Item A reads it via JS: items.B.val
        let bValToken = TennToken(type: .intLit, literal: "42")
        let bValNode = TennNode(kind: .IntLit, tok: bValToken)
        let bCmd = TennNode.newCommand("val", bValNode)

        let itemB = DiagramItem(kind: .Item, name: "B")
        itemB.properties.append(bCmd)

        let refToken = makeExprToken("items.B.val")
        let refNode = TennNode.newNode(kind: .Expression, refToken)
        let aCmd = TennNode.newCommand("bval", refNode)

        let itemA = DiagramItem(kind: .Item, name: "A")
        itemA.properties.append(aCmd)

        let element = Element(name: "Diag")
        element.add(itemB)
        element.add(itemA)

        let ctx = ExecutionContext()
        ctx.setElement(element)

        let evaluated = ctx.getEvaluated(itemA)
        let bval = evaluated[refToken]
        XCTAssertNotNil(bval, "Item A should resolve items.B.val")
        XCTAssertEqual(bval?.toInt32(), 42)
    }

    // MARK: 3. Update on model change via notifyChanges

    func testUpdateOnPositionChange() {
        // Item has expression `pos.x + 1`; after moving item we check pos value updates.
        // NOTE: ExecutionContext.notifyChanges recalculates item context.
        let exprToken = makeExprToken("pos.x + 1")
        let exprNode = TennNode.newNode(kind: .Expression, exprToken)
        let cmd = TennNode.newCommand("derived", exprNode)

        let item = DiagramItem(kind: .Item, name: "A")
        item.properties.append(cmd)
        item.x = 10
        let element = Element(name: "Diag")
        element.add(item)

        let ctx = ExecutionContext()
        ctx.setElement(element)

        // First evaluation: pos.x = 10, derived = 11
        let evalBefore = ctx.getEvaluated(item)
        XCTAssertEqual(evalBefore[exprToken]?.toInt32(), 11)

        // Simulate position update
        item.x = 20
        let event = ModelEvent(kind: .Layout, element: element)
        event.items[item] = .Update
        ctx.notifyChanges(event)

        // After update: pos.x = 20, derived = 21
        let evalAfter = ctx.getEvaluated(item)
        XCTAssertEqual(evalAfter[exprToken]?.toInt32(), 21)
    }

    // MARK: 4. Cyclic references - must not deadlock or crash

    func testCircularReferenceDoesNotCrash() {
        // Item A references B via items.B.bval
        // Item B references A via items.A.aval
        // The engine limits iteration to 100 cycles (reCalculate) so it must terminate.
        let aToken = makeExprToken("items.B ? (items.B.bval || 0) + 1 : 1")
        let aNode = TennNode.newNode(kind: .Expression, aToken)
        let aCmd = TennNode.newCommand("aval", aNode)

        let bToken = makeExprToken("items.A ? (items.A.aval || 0) + 1 : 1")
        let bNode = TennNode.newNode(kind: .Expression, bToken)
        let bCmd = TennNode.newCommand("bval", bNode)

        let itemA = DiagramItem(kind: .Item, name: "A")
        itemA.properties.append(aCmd)
        let itemB = DiagramItem(kind: .Item, name: "B")
        itemB.properties.append(bCmd)

        let element = Element(name: "Diag")
        element.add(itemA)
        element.add(itemB)

        // Should return without crash/hang. reCalculate caps at 100 iterations.
        let ctx = ExecutionContext()
        ctx.setElement(element)

        // Just verify we get some evaluated result (convergence or not, no crash).
        let evalA = ctx.getEvaluated(itemA)
        let evalB = ctx.getEvaluated(itemB)
        // We don't assert specific values - just that we got here alive.
        XCTAssertNotNil(evalA[aToken])
        XCTAssertNotNil(evalB[bToken])
    }

    // MARK: 5. JS runtime errors do not crash the process

    func testInvalidExpressionReturnsGracefully() {
        // Feed a broken JS expression. Engine captures exception into evaluated dict.
        let badToken = makeExprToken("this is not js !!!")
        let badNode = TennNode.newNode(kind: .Expression, badToken)
        let cmd = TennNode.newCommand("broken", badNode)

        let item = DiagramItem(kind: .Item, name: "A")
        item.properties.append(cmd)
        let element = Element(name: "Diag")
        element.add(item)

        let ctx = ExecutionContext()
        // Should not crash.
        ctx.setElement(element)

        let evaluated = ctx.getEvaluated(item)
        // Engine may store nil or an error JSValue; either way the process must survive.
        // If a value is stored it should not be a normal numeric result.
        if let val = evaluated[badToken] {
            // Likely an Error object or undefined - just ensure it's not the number 42.
            XCTAssertNotEqual(val.toInt32(), 42)
        }
        // Reaching here without crash = pass.
    }

    func testInvalidExpressionBlockReturnsGracefully() {
        // ExpressionBlock path (used for multi-statement JS).
        let badToken = makeExprBlockToken("function( { broken }")
        let badNode = TennNode.newNode(kind: .ExpressionBlock, badToken)
        let cmd = TennNode.newCommand("x", badNode)

        let item = DiagramItem(kind: .Item, name: "A")
        item.properties.append(cmd)
        let element = Element(name: "Diag")
        element.add(item)

        let ctx = ExecutionContext()
        ctx.setElement(element)
        // Must not crash. Evaluated dict may be empty or contain error value.
        _ = ctx.getEvaluated(item)
    }

    // MARK: 6. getEvaluated with explicit node override

    func testGetEvaluatedWithNodeOverride() {
        // Test the overload getEvaluated(_:_:_:) that accepts a TennNode directly.
        let item = DiagramItem(kind: .Item, name: "A")
        let element = Element(name: "Diag")
        element.add(item)

        let ctx = ExecutionContext()
        ctx.setElement(element)

        let exprToken = makeExprToken("2 + 3")
        let exprNode = TennNode.newNode(kind: .Expression, exprToken)
        let overrideBlock = TennNode.newBlockExpr(TennNode.newCommand("v", exprNode))

        let result = ctx.getEvaluated(item, overrideBlock, nil)
        XCTAssertNotNil(result[exprToken])
        XCTAssertEqual(result[exprToken]?.toInt32(), 5)
    }

    // MARK: 7. updateAll calls notifier after recalculation

    func testUpdateAllCallsNotifier() {
        let exprToken = makeExprToken("1 + 1")
        let exprNode = TennNode.newNode(kind: .Expression, exprToken)
        let cmd = TennNode.newCommand("v", exprNode)

        let item = DiagramItem(kind: .Item, name: "A")
        item.properties.append(cmd)
        let element = Element(name: "Diag")
        element.add(item)

        let ctx = ExecutionContext()
        ctx.setElement(element)

        let sema = DispatchSemaphore(value: 0)
        ctx.updateAll {
            sema.signal()
        }
        let result = sema.wait(timeout: .now() + 5)
        XCTAssertEqual(result, .success, "updateAll notifier should be called within 5s")
    }
}
