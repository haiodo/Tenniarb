//
//  RecoveryTests.swift
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
//

import XCTest

@testable import Tenniarb

/// The recovery folder of a stopped run and the restore of it: go/app/recovery_test.go without windows.
@MainActor
final class RecoveryTests: SceneDocumentTestCase {

    private let scene = "element A { item B { auto-layout row } }\n"

    private func workspace(_ files: [String: String] = [:]) throws -> URL {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-recovery-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        for (name, text) in files {
            try Data(text.utf8).write(to: root.appendingPathComponent(name))
        }
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        return root
    }

    /// The run folder name, as Recovery.write makes it.
    private func runName(_ date: Date) -> String {
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.calendar = Calendar(identifier: .gregorian)
        formatter.timeZone = .current
        formatter.dateFormat = "yyyyMMdd-HHmmss"
        return formatter.string(from: date)
    }

    /// The path Recovery.write keeps a document under.
    private func realPath(_ url: URL) -> String {
        url.resolvingSymlinksInPath().standardizedFileURL.path
    }

    /// Opens the document the way the app does, so restoreRecovery finds it in the controller.
    private func openManagedDocument(_ url: URL) throws -> Document {
        let document = try openTestDocument(url)
        NSDocumentController.shared.addDocument(document)
        return document
    }

    // MARK: - The folder

    func testConfigDirectoryFollowsTheEnvironment() {
        XCTAssertEqual(
            Recovery.configDirectory(["TENNIARB_CONFIG_DIR": "/tmp/tenniarb-config"]).path, "/tmp/tenniarb-config")

        let fallback = Recovery.configDirectory([:])
        XCTAssertEqual(fallback.lastPathComponent, "Tenniarb")
        XCTAssertEqual(fallback.deletingLastPathComponent().lastPathComponent, "Application Support")
    }

    func testRoundTrip() throws {
        let dir = try workspace()
        let at = Date(timeIntervalSince1970: 1_796_000_000)
        let texts = [
            "/w/design.tenn": "element A {}\n",
            "/w/lib/forms.tenn": "item Input { width 10 }\n",
        ]

        let folder = try XCTUnwrap(Recovery.write(texts, to: dir, now: at))

        XCTAssertEqual(folder.deletingLastPathComponent(), Recovery.root(dir))
        XCTAssertEqual(folder.lastPathComponent, runName(at))
        let loaded = try XCTUnwrap(Recovery.load(from: dir))
        XCTAssertEqual(realPath(loaded.folder), realPath(folder))
        XCTAssertEqual(loaded.manifest.run, runName(at))
        XCTAssertEqual(loaded.manifest.entries.count, 2)
        for entry in loaded.manifest.entries {
            let text = try Recovery.text(folder: loaded.folder, entryFile: entry.file)
            XCTAssertEqual(text, texts[entry.path], entry.path)
        }
        XCTAssertEqual(
            loaded.manifest.entries.map(\.path), ["/w/design.tenn", "/w/lib/forms.tenn"], "the copies follow the paths")

        try Recovery.drop(folder: loaded.folder)

        XCTAssertNil(try Recovery.load(from: dir), "a dropped folder is not offered again")
        XCTAssertFalse(FileManager.default.fileExists(atPath: loaded.folder.path))
    }

    func testNothingIsWrittenWithoutTexts() throws {
        let dir = try workspace()

        XCTAssertNil(try Recovery.write([:], to: dir, now: Date()))

        XCTAssertFalse(
            FileManager.default.fileExists(atPath: Recovery.root(dir).path), "the recovery folder was made for nothing")
    }

    func testANewerIncompleteOrCorruptFolderIsSkipped() throws {
        let dir = try workspace()
        let now = Date(timeIntervalSince1970: 1_796_000_000)
        let valid = try XCTUnwrap(Recovery.write(["/w/a.tenn": "item A"], to: dir, now: now))
        let root = Recovery.root(dir)

        try FileManager.default.createDirectory(
            at: root.appendingPathComponent(runName(now.addingTimeInterval(1))), withIntermediateDirectories: true)
        let corrupt = root.appendingPathComponent(runName(now.addingTimeInterval(2)))
        try FileManager.default.createDirectory(at: corrupt, withIntermediateDirectories: true)
        try Data("{".utf8).write(to: corrupt.appendingPathComponent("manifest.json"))

        let loaded = try XCTUnwrap(Recovery.load(from: dir))

        XCTAssertEqual(realPath(loaded.folder), realPath(valid))
        XCTAssertEqual(loaded.manifest.entries.count, 1)
    }

    // MARK: - Restore

    func testAMissingCopyKeepsTheFolder() throws {
        let root = try workspace()
        let url = root.appendingPathComponent("a.tenn")
        try Data(scene.utf8).write(to: url)
        let document = try openManagedDocument(url)

        let missing = root.appendingPathComponent("b.tenn").path
        let draft = "element A { item B { auto-layout column } }\n"
        let texts = [realPath(url): draft, missing: "item B"]
        let folder = try XCTUnwrap(Recovery.write(texts, to: root, now: Date()))
        let manifest = try XCTUnwrap(Recovery.load(from: root)).manifest
        let entry = try XCTUnwrap(manifest.entries.first { $0.path == missing })
        try FileManager.default.removeItem(at: folder.appendingPathComponent(entry.file))

        XCTAssertThrowsError(try AppDelegate().restoreRecovery(folder: folder, manifest: manifest))

        XCTAssertEqual(document.sceneText, draft, "the entry whose copy is there is restored")
        XCTAssertTrue(FileManager.default.fileExists(atPath: folder.path), "a failed restore keeps the folder")
    }

    func testASuccessfulRestoreDropsTheFolder() throws {
        let root = try workspace()
        let url = root.appendingPathComponent("a.tenn")
        try Data(scene.utf8).write(to: url)
        let document = try openManagedDocument(url)

        let draft = "element A { item B { auto-layout column } }\n"
        let folder = try XCTUnwrap(Recovery.write([realPath(url): draft], to: root, now: Date()))
        let manifest = try XCTUnwrap(Recovery.load(from: root)).manifest

        XCTAssertEqual(try AppDelegate().restoreRecovery(folder: folder, manifest: manifest), 1)

        XCTAssertEqual(document.sceneText, draft)
        XCTAssertTrue(document.isDocumentEdited)
        XCTAssertFalse(FileManager.default.fileExists(atPath: folder.path), "a successful restore drops the folder")
    }

    func testRestoreDraftOnASceneUndoesToTheFile() throws {
        let root = try workspace(["a.tenn": scene])
        let document = try openTestDocument(root.appendingPathComponent("a.tenn"))
        let draft = "element A { item B { auto-layout column } }\n"
        XCTAssertEqual(document.snapshotText(), scene)

        try document.restoreDraft(draft)

        XCTAssertEqual(document.sceneText, draft)
        XCTAssertTrue(document.isDocumentEdited)
        document.undoManager?.undo()
        XCTAssertEqual(document.sceneText, scene)
        XCTAssertFalse(document.isDocumentEdited, "undo back to the file clears the edited mark")
    }

    func testRestoreDraftOnAClassicDocumentUndoesToTheFile() throws {
        let before = "element \"D\" {\n    item \"A\" {\n        pos 1.0 2.0\n    }\n}"
        let draft = "element \"D\" {\n    item \"A\" {\n        pos 3.0 4.0\n    }\n}"
        let root = try workspace(["a.tenn": before])
        let document = try openTestDocument(root.appendingPathComponent("a.tenn"))
        XCTAssertEqual(document.snapshotText(), before)

        XCTAssertThrowsError(try document.restoreDraft("element {"))
        XCTAssertEqual(document.snapshotText(), before, "a text that does not parse leaves the document alone")

        try document.restoreDraft(draft)

        XCTAssertEqual(document.store?.model.toTennStr(), draft)
        XCTAssertTrue(document.isDocumentEdited)
        document.undoManager?.undo()
        XCTAssertEqual(document.store?.model.toTennStr(), before)
        XCTAssertFalse(document.isDocumentEdited, "undo back to the file clears the edited mark")
    }
}
