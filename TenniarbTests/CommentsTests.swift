//
//  CommentsTests.swift
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

/// The comments model and its per-document store: go/comments without windows.
final class CommentsTests: XCTestCase {

    /// 2026-10-28T10:00:00Z, the time of the Go tests.
    private let t0 = Date(timeIntervalSince1970: 1_793_181_600)

    private let anchor = Comments.Anchor(
        element: ["Tracker", "Boards#1"], node: "card/Title", point: Comments.Point(x: 4.5, y: 8))

    private func workspace() throws -> URL {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-comments-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        return root.appendingPathComponent("comments", isDirectory: true)
    }

    private func permissions(_ url: URL) throws -> Int {
        let attributes = try FileManager.default.attributesOfItem(atPath: url.path)
        return (attributes[.posixPermissions] as? NSNumber)?.intValue ?? -1
    }

    // MARK: - The model

    func testAddReplyStateDelete() throws {
        var doc = Comments.Doc(key: "path:/a.tenn")
        let first = try doc.add(author: "", text: "  too tight\n", anchor: anchor, now: t0)

        XCTAssertEqual(first.author, Comments.user)
        XCTAssertEqual(first.state, Comments.open)
        XCTAssertEqual(first.text, "too tight")
        XCTAssertNotNil(first.id.range(of: "^[0-9a-f]{8}$", options: .regularExpression), first.id)
        XCTAssertEqual(doc.find(first.id), first)
        XCTAssertNil(doc.find("nope"))

        XCTAssertThrowsError(try doc.add(author: Comments.user, text: " \n\t ", anchor: Comments.Anchor(), now: t0)) {
            XCTAssertEqual($0 as? Comments.Error, Comments.Error.empty)
        }
        XCTAssertThrowsError(try doc.reply(id: "nope", author: "agent", text: "x", now: t0)) {
            XCTAssertEqual($0 as? Comments.Error, Comments.Error.notFound)
        }
        XCTAssertThrowsError(try doc.reply(id: first.id, author: "agent", text: "  ", now: t0)) {
            XCTAssertEqual($0 as? Comments.Error, Comments.Error.empty)
        }

        let replied = try doc.reply(
            id: first.id, author: "claude-code", text: " padding is 4 now ", now: t0.addingTimeInterval(60))
        XCTAssertEqual(replied.replies?.map(\.text), ["padding is 4 now"])

        let resolved = try doc.setState(
            id: first.id, state: Comments.resolved, author: "claude-code", text: "done", now: t0.addingTimeInterval(120))
        XCTAssertEqual(resolved.state, Comments.resolved)
        XCTAssertEqual(resolved.replies?.map(\.text), ["padding is 4 now", "done"])

        let reopened = try doc.setState(id: first.id, state: Comments.open, author: Comments.user, text: "  ", now: t0)
        XCTAssertEqual(reopened.state, Comments.open)
        XCTAssertEqual(reopened.replies?.count, 2, "an empty closing text adds no reply")

        XCTAssertThrowsError(try doc.delete(id: first.id, author: "agent")) {
            XCTAssertEqual($0 as? Comments.Error, Comments.Error.notYours)
        }
        XCTAssertThrowsError(try doc.delete(id: "nope", author: Comments.user)) {
            XCTAssertEqual($0 as? Comments.Error, Comments.Error.notFound)
        }

        try doc.delete(id: first.id, author: Comments.user)
        XCTAssertTrue(doc.comments.isEmpty)

        XCTAssertThrowsError(try doc.delete(id: first.id, author: Comments.user)) {
            XCTAssertEqual($0 as? Comments.Error, Comments.Error.notFound)
        }
    }

    func testErrorTexts() {
        XCTAssertEqual(Comments.Error.notFound.localizedDescription, "no such comment")
        XCTAssertEqual(Comments.Error.empty.localizedDescription, "the text is empty")
        XCTAssertEqual(Comments.Error.tooLong.localizedDescription, "the text is longer than 10000 characters")
        XCTAssertEqual(Comments.Error.notYours.localizedDescription, "only the author can delete a comment")
    }

    func testTextLimitCountsCharacters() throws {
        var doc = Comments.Doc(key: "path:/a.tenn")
        let atLimit = String(repeating: "я", count: Comments.maxText)

        let added = try doc.add(author: Comments.user, text: atLimit, anchor: Comments.Anchor(), now: t0)
        XCTAssertEqual(added.text.count, Comments.maxText)

        XCTAssertThrowsError(try doc.add(author: Comments.user, text: atLimit + "я", anchor: Comments.Anchor(), now: t0)) {
            XCTAssertEqual($0 as? Comments.Error, Comments.Error.tooLong)
        }
    }

    func testIDsAreUnique() throws {
        var doc = Comments.Doc(key: "path:/a.tenn")
        for _ in 0..<8 {
            try doc.add(author: Comments.user, text: "x", anchor: Comments.Anchor(), now: t0)
        }
        let ids = doc.comments.map(\.id)
        XCTAssertEqual(Set(ids).count, ids.count)
        for id in ids {
            XCTAssertNotNil(id.range(of: "^[0-9a-f]{8}$", options: .regularExpression), id)
        }
    }

    func testOpenOfKeepsOpenOnly() throws {
        var doc = Comments.Doc(key: "path:/a.tenn")
        let first = try doc.add(author: Comments.user, text: "first", anchor: Comments.Anchor(), now: t0)
        let second = try doc.add(author: Comments.user, text: "second", anchor: Comments.Anchor(), now: t0)
        try doc.setState(id: first.id, state: Comments.resolved, author: "claude-code", text: "", now: t0)

        XCTAssertEqual(Comments.openOf(doc.comments).map(\.id), [second.id])
    }

    // MARK: - JSON

    func testJSONUsesGoKeysAndOmitsEmptyParts() throws {
        var doc = Comments.Doc(key: "repo:github.com/a/b#design/P.tenn")
        let full = try doc.add(author: Comments.user, text: "too tight", anchor: anchor, now: t0)
        let plain = try doc.add(author: Comments.user, text: "plain", anchor: Comments.Anchor(), now: t0)
        try doc.reply(id: full.id, author: "claude-code", text: "padding is 4 now", now: t0.addingTimeInterval(60))

        let data = try JSONEncoder().encode(doc)
        let object = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        XCTAssertEqual(Set(object.keys), ["v", "key", "rev", "comments"])

        let list = try XCTUnwrap(object["comments"] as? [[String: Any]])
        XCTAssertEqual(list.count, 2)
        XCTAssertEqual(Set(list[0].keys), ["id", "created", "author", "text", "anchor", "state", "replies"])
        XCTAssertEqual(list[0]["id"] as? String, full.id)
        XCTAssertEqual(list[0]["author"] as? String, Comments.user)
        XCTAssertEqual(list[0]["state"] as? String, Comments.open)
        XCTAssertEqual(list[0]["created"] as? String, "2026-10-28T10:00:00.000Z")

        let anchorJSON = try XCTUnwrap(list[0]["anchor"] as? [String: Any])
        XCTAssertEqual(Set(anchorJSON.keys), ["element", "node", "point"])
        XCTAssertEqual(anchorJSON["element"] as? [String], ["Tracker", "Boards#1"])
        XCTAssertEqual(anchorJSON["node"] as? String, "card/Title")
        let pointJSON = try XCTUnwrap(anchorJSON["point"] as? [String: Any])
        XCTAssertEqual(Set(pointJSON.keys), ["x", "y"])

        XCTAssertEqual(list[1]["id"] as? String, plain.id)
        XCTAssertEqual(
            Set(list[1].keys), ["id", "created", "author", "text", "anchor", "state"], "a thread without replies omits them")
        XCTAssertEqual((list[1]["anchor"] as? [String: Any])?.isEmpty, true, "an empty anchor is an empty object")

        XCTAssertEqual(try JSONDecoder().decode(Comments.Doc.self, from: data), doc)
    }

    func testTimesWrittenByGoReadBack() throws {
        let json = """
            {"v":1,"key":"path:/a.tenn","rev":3,"comments":[{"id":"ae12bc34",
            "created":"2026-10-28T12:00:00.123456789+02:00","author":"user","text":"x","anchor":{},
            "state":"open","replies":[{"author":"claude-code","text":"y","created":"2026-10-28T10:01:00Z"}]}]}
            """
        let doc = try JSONDecoder().decode(Comments.Doc.self, from: Data(json.utf8))
        let comment = try XCTUnwrap(doc.comments.first)

        XCTAssertEqual(comment.created.timeIntervalSince1970, t0.timeIntervalSince1970 + 0.123, accuracy: 0.001)
        XCTAssertEqual(comment.replies?.first?.created, t0.addingTimeInterval(60))
    }

    // MARK: - The store

    func testStoreRoundTrip() throws {
        let dir = try workspace()
        let store = Comments.Store(dir: dir)
        let key = "repo:github.com/a/b#design/P.tenn"

        let empty = try store.load(key)
        XCTAssertEqual(empty, Comments.Doc(key: key))
        XCTAssertEqual(empty.v, Comments.version)
        XCTAssertEqual(empty.rev, 0)
        XCTAssertFalse(FileManager.default.fileExists(atPath: dir.path), "a read creates nothing")

        var id = ""
        let added = try store.update(key) { doc in
            id = try doc.add(author: Comments.user, text: "  too tight  ", anchor: anchor, now: t0).id
        }
        XCTAssertEqual(added.rev, 1)

        try store.update(key) { doc in
            try doc.reply(id: id, author: "claude-code", text: "padding is 4 now", now: t0.addingTimeInterval(60))
        }

        let want = Comments.Comment(
            id: id, created: t0, author: Comments.user, text: "too tight", anchor: anchor, state: Comments.open,
            replies: [
                Comments.Reply(author: "claude-code", text: "padding is 4 now", created: t0.addingTimeInterval(60))
            ])
        let loaded = try store.load(key)
        XCTAssertEqual(loaded.rev, 2)
        XCTAssertEqual(loaded.key, key)
        XCTAssertEqual(loaded.comments, [want])
        XCTAssertEqual(try Comments.Store(dir: dir).load(key), loaded, "a second store reads the same file")
        XCTAssertEqual(try store.load("path:/x.tenn").comments, [], "documents do not share a file")
    }

    func testFileNameIsTheStableHashOfTheKey() throws {
        let store = Comments.Store(dir: try workspace())
        let url = store.file(for: "repo:a/b")

        XCTAssertEqual(url, store.file(for: "repo:a/b"))
        XCTAssertNotEqual(url, store.file(for: "repo:a/c"))
        XCTAssertEqual(url.deletingLastPathComponent(), store.dir)
        XCTAssertEqual(url.pathExtension, "json")
        XCTAssertEqual(url.lastPathComponent.count, 21)
        XCTAssertNotNil(url.lastPathComponent.range(of: "^[0-9a-f]{16}\\.json$", options: .regularExpression))
    }

    func testWritesOneFileWithTightPermissions() throws {
        let dir = try workspace()
        let store = Comments.Store(dir: dir)
        for _ in 0..<3 {
            try store.update("path:/a.tenn") { doc in
                try doc.add(author: Comments.user, text: "x", anchor: Comments.Anchor(), now: t0)
            }
        }

        let names = try FileManager.default.contentsOfDirectory(atPath: dir.path)
        XCTAssertEqual(names.count, 1, "a write leaves the file and nothing else")
        XCTAssertEqual(try permissions(dir) & 0o777, 0o700)
        XCTAssertEqual(try permissions(store.file(for: "path:/a.tenn")) & 0o777, 0o600)
    }

    func testAFailedChangeWritesNothing() throws {
        let dir = try workspace()
        let store = Comments.Store(dir: dir)
        let key = "path:/a.tenn"
        try store.update(key) { doc in
            try doc.add(author: Comments.user, text: "x", anchor: Comments.Anchor(), now: t0)
        }
        let file = store.file(for: key)
        let before = try Data(contentsOf: file)

        XCTAssertThrowsError(
            try store.update(key) { doc in
                try doc.reply(id: "nope", author: "agent", text: "b", now: t0)
            }
        ) {
            XCTAssertEqual($0 as? Comments.Error, Comments.Error.notFound)
        }
        XCTAssertEqual(try Data(contentsOf: file), before, "a refused change does not rewrite the file")

        // A file that does not parse is kept: the next write must not destroy what is there.
        let broken = Data("{broken".utf8)
        try broken.write(to: file)
        XCTAssertThrowsError(
            try store.update(key) { doc in
                try doc.add(author: Comments.user, text: "x", anchor: Comments.Anchor(), now: t0)
            })
        XCTAssertEqual(try Data(contentsOf: file), broken)
    }

    func testAFileOfAnotherDocumentIsRefused() throws {
        let dir = try workspace()
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let store = Comments.Store(dir: dir)
        let key = "path:/a.tenn"
        let other = try JSONEncoder().encode(Comments.Doc(key: "path:/b.tenn"))
        let file = store.file(for: key)
        try other.write(to: file)

        XCTAssertThrowsError(try store.load(key)) { error in
            XCTAssertTrue(error.localizedDescription.contains(key), error.localizedDescription)
            XCTAssertTrue(error.localizedDescription.contains("path:/b.tenn"), error.localizedDescription)
        }
        XCTAssertThrowsError(
            try store.update(key) { doc in
                try doc.add(author: Comments.user, text: "x", anchor: Comments.Anchor(), now: t0)
            })
        XCTAssertEqual(try Data(contentsOf: file), other, "a file of another document is never overwritten")
    }

    // MARK: - The config folder

    func testConfigDirectoryFollowsTheEnvironment() {
        XCTAssertEqual(
            Comments.configDir(["TENNIARB_CONFIG_DIR": "/cfg", "GOWT_QA_DATA": "/qa"]).path, "/cfg")
        XCTAssertEqual(Comments.configDir(["GOWT_QA_DATA": "/qa"]).path, "/qa")
        XCTAssertEqual(Comments.configDir(["TENNIARB_CONFIG_DIR": "", "GOWT_QA_DATA": "/qa"]).path, "/qa")
        XCTAssertEqual(Comments.dirIn(URL(fileURLWithPath: "/cfg", isDirectory: true)).path, "/cfg/comments")

        let fallback = Comments.configDir([:])
        XCTAssertEqual(fallback.lastPathComponent, "Tenniarb")
        XCTAssertEqual(fallback.deletingLastPathComponent().lastPathComponent, "Application Support")
    }
}
