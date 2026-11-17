//
//  TenniarbTests.swift
//  TenniarbTests
//
//  Created by Andrey Sobolev on 25/06/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
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

import Tenn
import XCTest

@testable import Tenniarb

class PersistenceTests: XCTestCase {

    override func setUp() {
        super.setUp()
        // Put setup code here. This method is called before the invocation of each test method in the class.
    }

    override func tearDown() {
        // Put teardown code here. This method is called after the invocation of each test method in the class.
        super.tearDown()
    }

    func saveModelTest() {
        let ef = TestElementFactory.createModel()

        let storedValue = ef.toTennStr()
        Swift.print(storedValue)

        let parser = TennParser()
        let node = parser.parse(storedValue)

        XCTAssertTrue(!parser.errors.hasErrors())

        _ = ElementModel.parseTenn(node: node)
    }

    func testSaveEmptyModel() {
        let model = ElementModel()

        let storedValue = model.toTennStr()
        Swift.print(storedValue)
        XCTAssertEqual("", storedValue)

        let parser = TennParser()
        let node = parser.parse(storedValue)

        XCTAssertTrue(!parser.errors.hasErrors())

        let model2 = ElementModel.parseTenn(node: node)
        XCTAssertNotNil(model2)
        XCTAssertEqual(model.name, model2.name)
    }
    func testSaveOneElement() {
        let model = ElementModel()

        model.add(Element(name: "Item1"))

        let storedValue = model.toTennStr()
        XCTAssertEqual("element \"Item1\" {\n}", storedValue)
        Swift.print(storedValue)

        let parser = TennParser()
        let node = parser.parse(storedValue)

        XCTAssertTrue(!parser.errors.hasErrors())

        let model2 = ElementModel.parseTenn(node: node)
        XCTAssertNotNil(model2)
        XCTAssertEqual(model.name, model2.name)
    }
    func testSaveTwoElementWithPos() {
        let model = ElementModel()

        let diagram = Element(name: "Diagram 1")
        _ = model.add(makeItem: diagram)

        let originalItem1 = diagram.add(get: DiagramItem(kind: .Item, name: "Demo element 1"))

        originalItem1.x = 10.5
        originalItem1.y = 30

        diagram.add(DiagramItem(kind: .Item, name: "Demo element 2"))

        let storedValue = model.toTennStr()
        XCTAssertEqual(
            "element \"Diagram 1\" {\n    item \"Demo element 1\" {\n        pos 10.5 30.0\n    }\n    item \"Demo element 2\"\n}",
            storedValue)
        Swift.print(storedValue)

        let parser = TennParser()
        let node = parser.parse(storedValue)

        XCTAssertTrue(!parser.errors.hasErrors())

        let model2 = ElementModel.parseTenn(node: node)
        XCTAssertNotNil(model2)
        XCTAssertEqual(model.name, model2.name)

        XCTAssertEqual(model2.count, 1)

        let diagramL = model2.elements[0]
        XCTAssertEqual(diagramL.itemCount, 2)
        let item0 = diagramL.items[0]
        let item1 = diagramL.items[1]
        XCTAssertEqual("Demo element 1", item0.name)

        XCTAssertEqual(10.5, item0.x)
        XCTAssertEqual(30, item0.y)
        XCTAssertEqual("Demo element 2", item1.name)

        XCTAssertEqual(0, item1.x)
        XCTAssertEqual(0, item1.y)
    }

    func testSaveLinks() {
        let model = ElementModel()

        let diagram = Element(name: "Diagram 1")
        _ = model.add(makeItem: diagram)

        let originalItem1 = diagram.add(get: DiagramItem(kind: .Item, name: "Demo element 1"))
        let originalItem2 = diagram.add(get: DiagramItem(kind: .Item, name: "Demo element 2"))

        diagram.add(source: originalItem1, target: originalItem2)

        let storedValue = model.toTennStr()
        XCTAssertEqual(
            "element \"Diagram 1\" {\n    item \"Demo element 1\"\n    item \"Demo element 2\"\n    link \"Demo element 1\" \"Demo element 2\"\n}",
            storedValue)
        Swift.print(storedValue)

        let parser = TennParser()
        let node = parser.parse(storedValue)

        XCTAssertTrue(!parser.errors.hasErrors())

        let model2 = ElementModel.parseTenn(node: node)
        XCTAssertNotNil(model2)
        XCTAssertEqual(model.name, model2.name)

        XCTAssertEqual(model2.count, 1)

        let diagramL = model2.elements[0]
        XCTAssertEqual(diagramL.itemCount, 3)
        let item0 = diagramL.items[0]
        let item1 = diagramL.items[1]
        //        let linkItem = diagramL.items[2]

        XCTAssertEqual("Demo element 1", item0.name)
        XCTAssertEqual("Demo element 2", item1.name)
    }

    private func load(_ text: String) -> ElementModel {
        let parser = TennParser()
        let node = parser.parse(text)
        XCTAssertFalse(parser.errors.hasErrors())
        return ElementModel.parseTenn(node: node)
    }

    func testItemDescriptionSurvivesSave() {
        let model = load("element \"D\" {\n    item \"A\" {\n        description \"hello\"\n        pos 1 2\n    }\n}")
        XCTAssertEqual("hello", model.elements[0].items[0].description)

        let saved = model.toTennStr()
        XCTAssertEqual("element \"D\" {\n    item \"A\" {\n        description \"hello\"\n        pos 1.0 2.0\n    }\n}", saved)
        XCTAssertEqual("hello", load(saved).elements[0].items[0].description)
    }

    func testRootPropertiesSurviveSave() {
        let model = load("zoom 2\nelement \"D\" {\n}\nstray")
        XCTAssertEqual(2, model.properties.count)

        let saved = model.toTennStr()
        XCTAssertEqual("zoom 2\nstray\nelement \"D\" {\n}", saved)
        let again = load(saved)
        XCTAssertEqual(2, again.properties.count)
        XCTAssertEqual(1, again.elements.count)
        XCTAssertEqual(saved, again.toTennStr())
    }

    func testUnresolvedLinkSurvivesSave() {
        let text = [
            "element \"D\" {",
            "    item \"A\"",
            "    item \"A\"",
            "    link \"A\" \"Missing\"",
            "    link \"A\" \"A\" {",
            "        target-index 7",
            "    }",
            "    link \"A\" \"A\" {",
            "        target-index 1",
            "    }",
            "}",
        ].joined(separator: "\n")
        let diagram = load(text).elements[0]
        XCTAssertEqual(3, diagram.items.count)  // two items + the resolved link
        XCTAssertEqual(2, diagram.properties.count)

        let saved = load(text).toTennStr()
        XCTAssertTrue(saved.contains("link \"A\" \"Missing\""))
        XCTAssertTrue(saved.contains("target-index 7"))
        XCTAssertTrue(saved.contains("target-index 1"))
        XCTAssertEqual(saved, load(saved).toTennStr())
    }

}

/// An open document follows its file when another program changes it, as go/app's 500 ms poll.
@MainActor
final class ExternalChangeTests: SceneDocumentTestCase {

    private let scene = "element A { item B { auto-layout row } }\n"

    private func workspace(_ files: [String: String] = [:]) throws -> URL {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-external-change-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        for (path, text) in files {
            try Data(text.utf8).write(to: root.appendingPathComponent(path))
        }
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        return root
    }

    /// Writes data as another program would, a minute ahead: the stamp differs even when the size does not.
    private func write(_ data: Data, to url: URL) throws {
        try data.write(to: url, options: .atomic)
        try FileManager.default.setAttributes(
            [.modificationDate: Date().addingTimeInterval(60)], ofItemAtPath: url.path)
    }

    func testAnUneditedDocumentFollowsTheFile() throws {
        let root = try workspace(["a.tenn": scene])
        let url = root.appendingPathComponent("a.tenn")
        let document = try openTestDocument(url)
        XCTAssertEqual(document.sceneText, scene)

        let next = "element A { item B { auto-layout column } }\n"
        try write(Data(next.utf8), to: url)
        document.pollExternalChanges()

        XCTAssertEqual(document.sceneText, next)
        XCTAssertFalse(document.isDocumentEdited)
    }

    func testAnEditedDocumentKeepsItsText() throws {
        let root = try workspace(["a.tenn": scene])
        let url = root.appendingPathComponent("a.tenn")
        let document = try openTestDocument(url)
        let text = try XCTUnwrap(document.sceneText)
        let row = try XCTUnwrap(text.range(of: "row"))
        let start = text.utf8.distance(from: text.startIndex, to: row.lowerBound)
        try document.applySceneEdit(Scene.TextEdit(start: start, end: start + 3, new: "column"), in: "a.tenn")
        XCTAssertTrue(document.isDocumentEdited)

        let external = "element Z { item Y { auto-layout row } }\n"
        try write(Data(external.utf8), to: url)
        document.pollExternalChanges()

        XCTAssertEqual(document.sceneText, text.replacingOccurrences(of: "row", with: "column"))
        XCTAssertTrue(document.isDocumentEdited)
        XCTAssertEqual(try String(contentsOf: url, encoding: .utf8), external, "the file on disk is left alone")
    }

    func testTheSameChangeIsReadOnce() throws {
        let root = try workspace(["a.tenn": scene])
        let url = root.appendingPathComponent("a.tenn")
        let document = try openTestDocument(url)

        let next = "element A { item B { auto-layout column } }\n"
        try write(Data(next.utf8), to: url)
        document.pollExternalChanges()
        let tree = try XCTUnwrap(document.sceneTree)

        document.pollExternalChanges()

        XCTAssertTrue(document.sceneTree === tree, "the same change is not read a second time")
        XCTAssertEqual(document.sceneText, next)
    }

    func testACanvasChangeIsConvertedAgain() throws {
        let root = try workspace()
        let url = root.appendingPathComponent("board.canvas")
        try Data(canvas("First").utf8).write(to: url)
        let document = try openTestDocument(url)
        XCTAssertTrue(try XCTUnwrap(document.sceneText).contains("First"))

        let next = canvas("Second")
        try write(Data(next.utf8), to: url)
        document.pollExternalChanges()

        XCTAssertEqual(document.sceneText, try JSONCanvas.canvasText(path: url.path, data: Data(next.utf8)).text)
        XCTAssertFalse(document.isDocumentEdited)
    }

    func testAnOwnSaveIsNotAChange() throws {
        let root = try workspace(["a.tenn": scene])
        let url = root.appendingPathComponent("a.tenn")
        let document = try openTestDocument(url)
        let tree = try XCTUnwrap(document.sceneTree)

        try document.write(to: url, ofType: "tenn")
        document.pollExternalChanges()

        XCTAssertTrue(document.sceneTree === tree, "the stamp of the save matches the file it wrote")
        XCTAssertEqual(document.sceneText, scene)
    }

    func testAMissingFileKeepsTheText() throws {
        let root = try workspace(["a.tenn": scene])
        let url = root.appendingPathComponent("a.tenn")
        let document = try openTestDocument(url)

        try FileManager.default.removeItem(at: url)
        document.pollExternalChanges()

        XCTAssertEqual(document.sceneText, scene)
        XCTAssertFalse(document.isDocumentEdited)
    }

    /// A group over a text node: the conversion nests items, which makes the file a scene.
    private func canvas(_ text: String) -> String {
        """
        {"nodes":[
         {"id":"g","type":"group","x":0,"y":0,"width":400,"height":300,"label":"Group"},
         {"id":"t","type":"text","x":20,"y":20,"width":120,"height":40,"text":"\(text)"}
        ]}
        """
    }
}
