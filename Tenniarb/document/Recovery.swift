//
//  Recovery.swift
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

/// One unsaved text kept from a stopped run, as go/app's recoveryEntry.
struct RecoveryEntry: Codable {
    /// Where the text belongs, absolute.
    let path: String
    /// The copy inside the folder, relative to it.
    let file: String
    let at: Date
}

/// What a recovery folder holds.
struct RecoveryManifest: Codable {
    let run: String
    let at: Date
    let entries: [RecoveryEntry]
}

/// A run that is told to stop keeps its unsaved work here, one folder per run: a make install or a kill
/// must not lose what the user has not saved. The next start offers it back.
enum Recovery {

    /// The folder the runs of this configuration write into.
    static func root(_ configDir: URL) -> URL {
        configDir.appendingPathComponent("recovery", isDirectory: true)
    }

    /// Where runs are kept: TENNIARB_CONFIG_DIR when set, else the application support folder.
    static func configDirectory(_ environment: [String: String] = ProcessInfo.processInfo.environment) -> URL {
        if let dir = environment["TENNIARB_CONFIG_DIR"], !dir.isEmpty {
            return URL(fileURLWithPath: dir, isDirectory: true)
        }
        let support =
            FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask).first
            ?? FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent("Library/Application Support")
        return support.appendingPathComponent("Tenniarb", isDirectory: true)
    }

    /// Writes the texts, by absolute path, into a folder of their own under configDir and returns it.
    /// Nothing is written when there is nothing to keep.
    static func write(_ texts: [String: String], to configDir: URL, now: Date) throws -> URL? {
        guard !texts.isEmpty else { return nil }
        let run = runName(now)
        let folder = root(configDir).appendingPathComponent(run, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        var entries: [RecoveryEntry] = []
        for (i, path) in texts.keys.sorted().enumerated() {
            let name = String(format: "%04d-%@", i + 1, (path as NSString).lastPathComponent)
            try Data(texts[path]!.utf8).write(to: folder.appendingPathComponent(name))
            entries.append(RecoveryEntry(path: path, file: name, at: now))
        }
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted]
        encoder.dateEncodingStrategy = .iso8601
        // The manifest goes last: a folder without one is a run that died mid-write and is not offered.
        try encoder.encode(RecoveryManifest(run: run, at: now, entries: entries))
            .write(to: folder.appendingPathComponent("manifest.json"))
        return folder
    }

    /// The newest folder that holds work; nil when no run kept anything. A corrupt or empty newer folder
    /// does not hide an older one.
    static func load(from configDir: URL) throws -> (folder: URL, manifest: RecoveryManifest)? {
        let root = root(configDir)
        guard
            let names = try? FileManager.default.contentsOfDirectory(
                at: root, includingPropertiesForKeys: [.isDirectoryKey], options: [])
        else {
            return nil
        }
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        var failures: [String] = []
        for folder in names.sorted(by: { $0.lastPathComponent > $1.lastPathComponent }) {
            guard (try? folder.resourceValues(forKeys: [.isDirectoryKey]).isDirectory) == true else { continue }
            guard let data = try? Data(contentsOf: folder.appendingPathComponent("manifest.json")) else { continue }
            do {
                let manifest = try decoder.decode(RecoveryManifest.self, from: data)
                if !manifest.entries.isEmpty {
                    return (folder, manifest)
                }
            } catch {
                failures.append("\(folder.lastPathComponent): \(error.localizedDescription)")
            }
        }
        guard failures.isEmpty else { throw RecoveryError(failures: failures) }
        return nil
    }

    /// The kept text of one entry.
    static func text(folder: URL, entryFile: String) throws -> String {
        let data = try Data(contentsOf: folder.appendingPathComponent(entryFile))
        return String(decoding: data, as: UTF8.self)
    }

    /// Deletes a recovered folder, once its work is saved or thrown away.
    static func drop(folder: URL) throws {
        try FileManager.default.removeItem(at: folder)
    }

    /// The run folder name, in the local time zone as go/app's `20060102-150405`.
    private static func runName(_ now: Date) -> String {
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.calendar = Calendar(identifier: .gregorian)
        formatter.timeZone = .current
        formatter.dateFormat = "yyyyMMdd-HHmmss"
        return formatter.string(from: now)
    }
}

/// Why a recovery folder could not be read or restored, one line per entry.
struct RecoveryError: LocalizedError {
    let failures: [String]

    var errorDescription: String? { failures.joined(separator: "; ") }
}
