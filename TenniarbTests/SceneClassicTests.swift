//
//  SceneClassicTests.swift
//  TenniarbTests
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.
//

import XCTest

@testable import Tenniarb

/// The classic hook of go/scene/classic.go: the element model names of the drawn nodes of a classic diagram.
final class SceneClassicTests: XCTestCase {

    /// A workspace on disk: classic diagrams are loaded from a file, as in the app.
    private func tree(_ src: String) throws -> Scene.Tree {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-scene-classic-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        try src.write(to: root.appendingPathComponent("board.tenn"), atomically: true, encoding: .utf8)
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        return try SceneLoader(root: root).load("board.tenn")
    }

    /// classicRefs numbers items per NFC name in file order with the element flagged as the root, and links
    /// per end pair; the hook sees those names once per node and property.
    func testClassicRefsNumberItemsAndLinks() throws {
        let tree = try tree(
            """
            element "E" {
                width ${w}
                item A { pos 0 0; title ${t} }
                item A { pos 0 40; title ${t} }
                item B { pos 120 0; title "B" }
                link A B { color ${c} }
                link A B { color ${c} }
            }
            """)
        XCTAssertTrue(tree.diagnostics.isEmpty, "\(tree.diagnostics)")
        var calls: [(at: Scene.ClassicRef, prop: String)] = []
        let hook: Scene.ClassicFunc = { at, prop in
            calls.append((at, prop))
            switch prop {
            case "width": return ["120"]
            case "title": return ["t"]
            case "color": return ["#000000"]
            default: return nil
            }
        }
        let screen = try XCTUnwrap(tree.build("E", classic: hook))
        XCTAssertEqual(screen.w, 120, "the width of the element comes from the hook")

        let root = try XCTUnwrap(calls.first { $0.prop == "width" })
        XCTAssertEqual(root.at, Scene.ClassicRef(kind: .item, name: "E", root: true))

        let titles = calls.filter { $0.prop == "title" }.map(\.at)
        XCTAssertEqual(
            titles,
            [
                Scene.ClassicRef(kind: .item, name: "A", nth: 0),
                Scene.ClassicRef(kind: .item, name: "A", nth: 1),
            ], "two items of one name are numbered in file order")
        XCTAssertEqual(
            calls.filter { $0.prop == "color" }.map(\.at),
            [
                Scene.ClassicRef(kind: .link, from: "A", to: "B", nth: 0),
                Scene.ClassicRef(kind: .link, from: "A", to: "B", nth: 1),
            ], "a link is numbered among the links with its end pair")
    }
}
