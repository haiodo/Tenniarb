//
//  CommentsStore.swift
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

import CryptoKit
import Foundation

extension Comments {

    /// Environment variables that point the app and the service at another config folder.
    static let envConfigDir = "TENNIARB_CONFIG_DIR"
    static let envQAData = "GOWT_QA_DATA"

    /// The user's Tenniarb folder: settings, window frames and comments live here.
    static func configDir(_ environment: [String: String] = ProcessInfo.processInfo.environment) -> URL {
        for name in [envConfigDir, envQAData] {
            if let dir = environment[name], !dir.isEmpty {
                return URL(fileURLWithPath: dir, isDirectory: true)
            }
        }
        guard let support = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask).first
        else {
            return FileManager.default.temporaryDirectory.appendingPathComponent("Tenniarb", isDirectory: true)
        }
        return support.appendingPathComponent("Tenniarb", isDirectory: true)
    }

    /// The folder of the comment files in the config folder config.
    static func dirIn(_ config: URL) -> URL {
        config.appendingPathComponent("comments", isDirectory: true)
    }

    /// A comments file that cannot be read or belongs to another document.
    struct StoreError: LocalizedError {
        let message: String

        var errorDescription: String? { message }
    }

    /// The comments of every document in dir, one file each. Writes go through one store; a reader in
    /// another process may read the files at any time, as a write replaces a file whole.
    final class Store {
        let dir: URL
        private let lock = NSLock()

        init(dir: URL) {
            self.dir = dir
        }

        /// The first 8 bytes of the SHA-256 of the key, as hex, name the file of a document.
        func file(for key: String) -> URL {
            let digest = SHA256.hash(data: Data(key.utf8))
            let name = digest.prefix(8).map { String(format: "%02x", $0) }.joined()
            return dir.appendingPathComponent(name + ".json")
        }

        /// A document without a file has no comments.
        func load(_ key: String) throws -> Doc {
            let data: Data
            do {
                data = try Data(contentsOf: file(for: key))
            } catch let error as CocoaError where error.code == .fileReadNoSuchFile {
                return Doc(key: key)
            }
            let doc: Doc
            do {
                doc = try JSONDecoder().decode(Doc.self, from: data)
            } catch {
                throw StoreError(message: "comments of \(key): \(error)")
            }
            guard doc.key == key else {
                throw StoreError(message: "comments file of \(key) holds \(doc.key)")
            }
            return doc
        }

        /// Reads the comments of key, lets change edit them and writes the result with a higher rev. A
        /// change that fails writes nothing; a file that cannot be read is never overwritten.
        func update(_ key: String, _ change: (inout Doc) throws -> Void) throws -> Doc {
            lock.lock()
            defer { lock.unlock() }
            var doc = try load(key)
            try change(&doc)
            doc.v = version
            doc.key = key
            doc.rev += 1
            let encoder = JSONEncoder()
            encoder.outputFormatting = [.prettyPrinted, .withoutEscapingSlashes]
            try writeAtomic(try encoder.encode(doc), to: file(for: key))
            return doc
        }

        /// Writes through a temporary file in the same folder, so a reader sees the old file or the new
        /// one whole.
        private func writeAtomic(_ data: Data, to url: URL) throws {
            let fm = FileManager.default
            try fm.createDirectory(at: dir, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
            let tmp = dir.appendingPathComponent(".tmp-\(UUID().uuidString)")
            do {
                try data.write(to: tmp)
                try fm.setAttributes([.posixPermissions: 0o600], ofItemAtPath: tmp.path)
                let handle = try FileHandle(forWritingTo: tmp)
                try handle.synchronize()
                try handle.close()
                guard rename(tmp.path, url.path) == 0 else {
                    let reason = String(cString: strerror(errno))
                    throw StoreError(message: "cannot replace \(url.lastPathComponent): \(reason)")
                }
            } catch {
                try? fm.removeItem(at: tmp)
                throw error
            }
        }
    }
}
