//
//  SceneEntryTests.swift
//  TenniarbTests
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//  WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.  See the
//  License for the specific language governing permissions and limitations under
//  the License.
//

import XCTest

@testable import Tenniarb

/// Entry discovery and the entry context of a part file, as go/scene/entry_test.go.
///
/// The numbers are Go's: the tracker board in the context of design.tenn builds and its instance Button
/// is 40 wide, resolved at lib/ui.tenn:1:19 (`go test ./scene -run TestEntry` asserts 40; a one-off `go
/// run` calling scene.LoadAuto and scene.Tree.Build printed entry "design.tenn", width 40.000).
final class SceneEntryTests: XCTestCase {
    private let libText = "element \"Parts\" { item Button { width 40; height 20 } }\nitem Card { width 10; height 10 }\n"
    private let partText = "element \"Screen\" { use ui.Parts.Button as b }\n"

    /// A workspace on disk; the loader and entry discovery read it, as go/scene does.
    private func workspace(_ files: [String: String]) throws -> URL {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-scene-entry-\(UUID().uuidString)", isDirectory: true)
        for (path, text) in files {
            let url = root.appendingPathComponent(path)
            try FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
            try text.write(to: url, atomically: true, encoding: .utf8)
        }
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        return root
    }

    /// The width of the first instance in the built element, as instWidth in entry_test.go.
    private func instanceWidth(_ tree: Scene.Tree, _ path: String) throws -> Double {
        let root = try XCTUnwrap(tree.layout(path), "\(path) does not build: \(tree.diagnostics)")
        func find(_ n: Scene.LayoutNode) -> Double {
            if n.node?.inst == true { return n.rw }
            for k in n.kids {
                let w = find(k)
                if w != 0 { return w }
            }
            return 0
        }
        return find(root)
    }

    func testEntryProvidesLibrary() throws {
        let root = try workspace([
            "design.tenn": "element \"ui\" --from-file \"lib/ui.tenn\"\nelement \"tracker\" --from-file \"apps/tracker.tenn\"\n",
            "lib/ui.tenn": libText,
            "apps/tracker.tenn": partText,
        ])
        let alone = try SceneLoader(root: root).load("apps/tracker.tenn")
        XCTAssertNil(alone.build("Screen"), "ui is not defined without the entry")
        XCTAssertTrue(
            alone.diagnostics.contains {
                $0.pos.file == "apps/tracker.tenn" && $0.message.contains("is not defined here or above")
            }, "\(alone.diagnostics)")

        let tree = try SceneLoader(root: root).loadIn("apps/tracker.tenn", entry: "design.tenn")
        XCTAssertEqual(tree.entry, "design.tenn")
        XCTAssertEqual(tree.main?.pos.file, "apps/tracker.tenn")
        XCTAssertEqual(try instanceWidth(tree, "Screen"), 40)

        let tracker = try XCTUnwrap(tree.file("apps/tracker.tenn"))
        let button = try tree.resolve(tracker, "ui.Parts.Button")
        XCTAssertEqual(button.pos.file, "lib/ui.tenn")
        XCTAssertEqual(button.pos.line, 1)
        XCTAssertEqual(button.pos.col, 19)
    }

    func testEntryOwnFileWins() throws {
        let root = try workspace([
            "design.tenn": "element \"ui\" --from-file \"lib/ui.tenn\"\nelement \"part\" --from-file \"part.tenn\"\n",
            "lib/ui.tenn": libText,
            "part.tenn": "element \"ui\" { element \"Parts\" { item Button { width 7; height 7 } } }\n"
                + "element \"Screen\" { use ui.Parts.Button as b }\n",
        ])
        let tree = try SceneLoader(root: root).loadIn("part.tenn", entry: "design.tenn")
        XCTAssertEqual(try instanceWidth(tree, "Screen"), 7, "the file's own ui must win over the entry's")
    }

    func testEntryTwoEntriesTwoLibraries() throws {
        let root = try workspace([
            "a/design.tenn": "element \"ui\" --from-file \"../lib/a.tenn\"\nelement \"p\" --from-file \"../part.tenn\"\n",
            "b/design.tenn": "element \"ui\" --from-file \"../lib/b.tenn\"\nelement \"p\" --from-file \"../part.tenn\"\n",
            "lib/a.tenn": libText,
            "lib/b.tenn": libText.replacingOccurrences(of: "width 40", with: "width 90"),
            "part.tenn": partText,
        ])
        for (entry, want) in [("a/design.tenn", 40.0), ("b/design.tenn", 90.0)] {
            let tree = try SceneLoader(root: root).loadIn("part.tenn", entry: entry)
            XCTAssertEqual(try instanceWidth(tree, "Screen"), want, entry)
        }
    }

    func testEntrySameFileTwoMounts() throws {
        let root = try workspace([
            "design.tenn": "element \"l1\" { element \"ui\" --from-file \"lib/a.tenn\"; element \"p\" --from-file \"part.tenn\" }\n"
                + "element \"l2\" { element \"ui\" --from-file \"lib/b.tenn\"; element \"p\" --from-file \"part.tenn\" }\n"
                + "element \"Show1\" { use l1.p.Screen as s }\nelement \"Show2\" { use l2.p.Screen as s }\n",
            "lib/a.tenn": libText,
            "lib/b.tenn": libText.replacingOccurrences(of: "width 40", with: "width 90"),
            "part.tenn": partText,
        ])
        let tree = try SceneLoader(root: root).load("design.tenn")
        XCTAssertEqual(try instanceWidth(tree, "Show1"), 40, "first mount")
        XCTAssertEqual(try instanceWidth(tree, "Show2"), 90, "second mount must resolve in its own context")
    }

    func testEntryCheckedOnce() throws {
        let root = try workspace([
            "design.tenn": "element \"ui\" --from-file \"lib/ui.tenn\"\nelement \"tracker\" --from-file \"apps/tracker.tenn\"\n",
            "lib/ui.tenn": libText,
            "apps/tracker.tenn": "element \"Screen\" { use ui.Parts.Nope as b }\n",
        ])
        let tree = try SceneLoader(root: root).load("design.tenn")
        XCTAssertNil(tree.build("tracker.Screen"))
        let failures = tree.diagnostics.filter { !$0.warn }
        XCTAssertEqual(failures.count, 1, "the missing name is reported once: \(tree.diagnostics)")
        XCTAssertEqual(failures.first?.pos.file, "apps/tracker.tenn")
    }

    func testEntryNotMountingFile() throws {
        let root = try workspace(["design.tenn": "element \"x\" {}\n", "part.tenn": partText])
        XCTAssertThrowsError(try SceneLoader(root: root).loadIn("part.tenn", entry: "design.tenn")) { error in
            XCTAssertTrue("\(error)".contains("does not mount part.tenn"), "\(error)")
        }
    }

    func testEntryExplicitProperty() throws {
        let root = try workspace([
            "design.tenn": "element \"ui\" --from-file \"lib/ui.tenn\"\nelement \"p\" --from-file \"deep/er/part.tenn\"\n",
            "lib/ui.tenn": libText,
            "deep/er/part.tenn": "entry \"../../design.tenn\"\n" + partText,
            "deep/er/bad.tenn": "entry \"../../nope.tenn\"\n" + partText,
        ])
        let tree = try SceneLoader(root: root).loadAuto("deep/er/part.tenn")
        XCTAssertEqual(tree.entry, "design.tenn")

        // The property is on by itself: the file loads in its entry even when it would load alone.
        let alone = try SceneLoader(root: root).load("deep/er/part.tenn")
        XCTAssertNil(alone.build("Screen"))

        let bad = try SceneLoader(root: root).loadAuto("deep/er/bad.tenn")
        XCTAssertTrue(
            bad.diagnostics.contains { "\($0.pos): \($0.message)".hasPrefix("deep/er/bad.tenn:1:1: entry:") },
            "\(bad.diagnostics)")
    }

    func testEntryDiscovery() throws {
        let root = try workspace([
            "foundation/Design.tenn": "element \"ui\" --from-file \"lib/ui.tenn\"\nelement \"cat\" --from-file \"categories/all.tenn\"\n",
            "foundation/lib/ui.tenn": libText,
            "foundation/categories/all.tenn": "element \"tracker\" --from-file \"../apps/tracker.tenn\"\n",
            "foundation/apps/tracker.tenn": partText,
            "foundation/apps/other.tenn": "element \"x\" {}\n",
        ])
        let want = root.resolvingSymlinksInPath().appendingPathComponent("foundation/Design.tenn").path
        XCTAssertEqual(
            SceneLoader.findEntry(root: root, file: "foundation/apps/tracker.tenn")?.path, want,
            "the top of the chain, not categories/all.tenn")
        XCTAssertNil(
            SceneLoader.findEntry(root: root, file: "foundation/apps/other.tenn"),
            "a file nothing mounts has no entry")

        let tree = try SceneLoader(root: root).loadAuto("foundation/apps/tracker.tenn")
        XCTAssertEqual(tree.entry, "foundation/Design.tenn")
        // A file that loads clean alone keeps its own context.
        let other = try SceneLoader(root: root).loadAuto("foundation/apps/other.tenn")
        XCTAssertEqual(other.entry, "")
    }

    func testEntryDiscoveryUsesNearest() throws {
        let root = try workspace([
            "top.tenn": "element \"ui\" --from-file \"lib/ui.tenn\"\nelement \"p\" --from-file \"sub/part.tenn\"\n",
            "lib/ui.tenn": libText,
            "sub/part.tenn": partText,
            "sub/near.tenn": "element \"ui\" --from-file \"../lib/ui.tenn\"\nelement \"p\" --from-file \"part.tenn\"\n",
        ])
        // near.tenn is not mounted by top.tenn, so the chain ends at it.
        XCTAssertEqual(SceneLoader.findEntry(root: root, file: "sub/part.tenn")?.lastPathComponent, "near.tenn")
        try FileManager.default.removeItem(at: root.appendingPathComponent("sub/near.tenn"))
        XCTAssertEqual(SceneLoader.findEntry(root: root, file: "sub/part.tenn")?.lastPathComponent, "top.tenn")
    }

    func testEntryMountCycleStops() throws {
        let root = try workspace([
            "a.tenn": "element \"b\" --from-file \"b.tenn\"\n",
            "b.tenn": "element \"a\" --from-file \"a.tenn\"\n",
        ])
        // Go: FindEntry(a.tenn) is a.tenn itself, and LoadAuto keeps the plain tree with the cycle error.
        XCTAssertEqual(SceneLoader.findEntry(root: root, file: "a.tenn")?.lastPathComponent, "a.tenn")
        let tree = try SceneLoader(root: root).loadAuto("a.tenn")
        XCTAssertEqual(tree.entry, "")
        XCTAssertTrue(tree.diagnostics.contains { $0.message.contains("import cycle") }, "\(tree.diagnostics)")
    }

    func testEntryLibraryUsesSiblingLibrary() throws {
        let root = try workspace([
            "design.tenn":
                "element \"icons\" --from-file \"icons.tenn\"\nelement \"ui\" --from-file \"ui.tenn\"\nelement \"app\" --from-file \"app.tenn\"\n",
            "icons.tenn": "item Star { width 12; height 12 }\n",
            "ui.tenn": "item Badge { use icons.Star as s }\n",
            "app.tenn": "element \"Screen\" { use ui.Badge as b }\n",
        ])
        let tree = try SceneLoader(root: root).load("design.tenn")
        XCTAssertNotEqual(try instanceWidth(tree, "app.Screen"), 0, "the instance was not built")
        let ui = try SceneLoader(root: root).load("ui.tenn")
        XCTAssertNil(ui.build(""), "ui.tenn alone has no icons")
    }
}
