//
//  Comments.swift
//  Tenniarb
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

import Foundation

/// The comments a user leaves on a design for an agent to answer, as go/comments. They live in the
/// user's config folder, one JSON file per document, never in the repository.
enum Comments {

    /// The author of the comments and replies written in the app.
    static let user = "user"
    static let open = "open"
    static let resolved = "resolved"

    /// What the user did, for the events agents wait for.
    static let eventComment = "comment"
    static let eventReply = "reply"
    static let eventReopen = "reopen"

    /// Caps the text of one comment or reply, in characters.
    static let maxText = 10000

    /// The version of the file format; a field added later keeps it.
    static let version = 1

    /// A place in the coordinates of its anchor: the offset from the top left corner of the node box
    /// (y down, scene units) or, without a node, from the origin of the element content.
    struct Point: Codable, Equatable {
        var x: Double
        var y: Double
    }

    /// What a comment is about: the names from the root down, the selection path or diagram item in that
    /// element, and a place in it. All three may be empty.
    struct Anchor: Equatable {
        var element: [String]? = nil
        var node: String? = nil
        var point: Point? = nil
    }

    /// One answer in a thread; threads are one level deep.
    struct Reply: Equatable {
        var author: String
        var text: String
        var created: Date
    }

    /// A thread: the text of its author and the replies to it.
    struct Comment: Equatable {
        var id: String
        var created: Date
        var author: String
        var text: String
        var anchor: Anchor
        var state: String
        var replies: [Reply]? = nil
    }

    /// The comments of one document.
    struct Doc: Codable, Equatable {
        var v = version
        /// The identity of the document.
        var key: String
        /// Grows with every write; a reader drops an update older than the one it holds.
        var rev: Int64 = 0
        var comments: [Comment] = []
    }

    /// Errors of the operations; the service shows them to the agent and the user as they are.
    enum Error: LocalizedError, Equatable {
        case notFound
        case empty
        case tooLong
        case notYours

        var errorDescription: String? {
            switch self {
            case .notFound: return "no such comment"
            case .empty: return "the text is empty"
            case .tooLong: return "the text is longer than \(Comments.maxText) characters"
            case .notYours: return "only the author can delete a comment"
            }
        }
    }

    /// JSON times as RFC 3339 with milliseconds, which Go's time.Time reads. A time Go wrote may carry
    /// nanoseconds and keeps milliseconds when read back.
    private static let jsonTime = Date.ISO8601FormatStyle(includingFractionalSeconds: true)

    static func timeText(_ date: Date) -> String {
        date.formatted(jsonTime)
    }

    static func time(from text: String) throws -> Date {
        do {
            return try jsonTime.parse(text)
        } catch {
            throw DecodingError.dataCorrupted(
                DecodingError.Context(codingPath: [], debugDescription: "not an ISO-8601 time: \(text)"))
        }
    }

    static func checkText(_ text: String) throws -> String {
        let trimmed = text.trimmingCharacters(in: .whitespacesAndNewlines)
        if trimmed.isEmpty {
            throw Error.empty
        }
        if trimmed.unicodeScalars.count > maxText {
            throw Error.tooLong
        }
        return trimmed
    }

    /// Four random bytes as eight hex characters, unique in the document.
    static func newID(taken: (String) -> Bool) -> String {
        while true {
            // Not crypto-random: an ID only has to be unique in one document.
            let id = String(format: "%08x", UInt32.random(in: .min ... .max))
            if !taken(id) {
                return id
            }
        }
    }

    /// The comments in state open.
    static func openOf(_ comments: [Comment]) -> [Comment] {
        comments.filter { $0.state == open }
    }
}

extension Comments.Doc {

    func find(_ id: String) -> Comments.Comment? {
        comments.first { $0.id == id }
    }

    /// Appends an open comment of author; an empty author means the user.
    @discardableResult
    mutating func add(author: String, text: String, anchor: Comments.Anchor, now: Date) throws -> Comments.Comment {
        let text = try Comments.checkText(text)
        let comment = Comments.Comment(
            id: Comments.newID { find($0) != nil },
            created: now,
            author: author.isEmpty ? Comments.user : author,
            text: text,
            anchor: anchor,
            state: Comments.open)
        comments.append(comment)
        return comment
    }

    /// Adds a reply to the thread id.
    @discardableResult
    mutating func reply(id: String, author: String, text: String, now: Date) throws -> Comments.Comment {
        guard let index = comments.firstIndex(where: { $0.id == id }) else {
            throw Comments.Error.notFound
        }
        let text = try Comments.checkText(text)
        comments[index].replies =
            (comments[index].replies ?? []) + [
                Comments.Reply(author: author, text: text, created: now)
            ]
        return comments[index]
    }

    /// Resolves or reopens the thread id. A resolve may bring a closing reply; its text may be empty.
    @discardableResult
    mutating func setState(id: String, state: String, author: String, text: String, now: Date) throws -> Comments.Comment {
        guard let index = comments.firstIndex(where: { $0.id == id }) else {
            throw Comments.Error.notFound
        }
        if !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            try reply(id: id, author: author, text: text, now: now)
        }
        comments[index].state = state
        return comments[index]
    }

    /// Removes the thread id; only its author may.
    mutating func delete(id: String, author: String) throws {
        guard let index = comments.firstIndex(where: { $0.id == id }) else {
            throw Comments.Error.notFound
        }
        guard comments[index].author == author else {
            throw Comments.Error.notYours
        }
        comments.remove(at: index)
    }
}

extension Comments.Anchor: Codable {

    private enum CodingKeys: String, CodingKey {
        case element, node, point
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        element = try c.decodeIfPresent([String].self, forKey: .element)
        node = try c.decodeIfPresent(String.self, forKey: .node)
        point = try c.decodeIfPresent(Comments.Point.self, forKey: .point)
    }

    func encode(to encoder: Encoder) throws {
        var c = encoder.container(keyedBy: CodingKeys.self)
        if let element, !element.isEmpty {
            try c.encode(element, forKey: .element)
        }
        if let node, !node.isEmpty {
            try c.encode(node, forKey: .node)
        }
        if let point {
            try c.encode(point, forKey: .point)
        }
    }
}

extension Comments.Reply: Codable {

    private enum CodingKeys: String, CodingKey {
        case author, text, created
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        author = try c.decode(String.self, forKey: .author)
        text = try c.decode(String.self, forKey: .text)
        created = try Comments.time(from: c.decode(String.self, forKey: .created))
    }

    func encode(to encoder: Encoder) throws {
        var c = encoder.container(keyedBy: CodingKeys.self)
        try c.encode(author, forKey: .author)
        try c.encode(text, forKey: .text)
        try c.encode(Comments.timeText(created), forKey: .created)
    }
}

extension Comments.Comment: Codable {

    private enum CodingKeys: String, CodingKey {
        case id, created, author, text, anchor, state, replies
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        id = try c.decode(String.self, forKey: .id)
        created = try Comments.time(from: c.decode(String.self, forKey: .created))
        author = try c.decode(String.self, forKey: .author)
        text = try c.decode(String.self, forKey: .text)
        anchor = try c.decode(Comments.Anchor.self, forKey: .anchor)
        state = try c.decode(String.self, forKey: .state)
        replies = try c.decodeIfPresent([Comments.Reply].self, forKey: .replies)
    }

    func encode(to encoder: Encoder) throws {
        var c = encoder.container(keyedBy: CodingKeys.self)
        try c.encode(id, forKey: .id)
        try c.encode(Comments.timeText(created), forKey: .created)
        try c.encode(author, forKey: .author)
        try c.encode(text, forKey: .text)
        try c.encode(anchor, forKey: .anchor)
        try c.encode(state, forKey: .state)
        if let replies, !replies.isEmpty {
            try c.encode(replies, forKey: .replies)
        }
    }
}
