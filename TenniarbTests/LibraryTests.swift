//
//  LibraryTests.swift
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

final class LibraryTests: XCTestCase {
    private var savedEnvironment: [String: String?] = [:]

    /// One icon, the form libraries/icons.tenn has.
    private let iconLib = """
        item "zap" {
            width 24; height 24; path "M1 2 L3 4"
            view-box 0 0 24 24; border-color #000000; line-width 2
        }
        """

    override func setUp() {
        super.setUp()
        for name in [Library.envLibraries, "TENNIARB_CONFIG_DIR"] {
            savedEnvironment[name] = ProcessInfo.processInfo.environment[name]
            unsetenv(name)
        }
    }

    override func tearDown() {
        for (name, value) in savedEnvironment {
            if let value {
                setenv(name, value, 1)
            } else {
                unsetenv(name)
            }
        }
        savedEnvironment = [:]
        super.tearDown()
    }

    // MARK: - Names

    func testParsesName() throws {
        let p = try Library.parse("tenniarb-library://icons")
        XCTAssertEqual(p, Library.Path(name: "icons"))
        XCTAssertEqual(p.string, "tenniarb-library://icons")
        XCTAssertEqual(p.rel, "icons.tenn")
        XCTAssertEqual(p.label, "icons")
        XCTAssertNoThrow(try p.valid())
    }

    func testParsesOwnerAndTrailingTenn() throws {
        let p = try Library.parse("tenniarb-library://haiodo/c4.tenn")
        XCTAssertEqual(p.owner, "haiodo")
        XCTAssertEqual(p.name, "c4")
        XCTAssertEqual(p.string, "tenniarb-library://haiodo/c4")
        XCTAssertEqual(p.rel, "haiodo/c4.tenn")
    }

    /// Parse lowercases what it reads, as go/library does; the scheme check itself is case-sensitive.
    func testParseLowercases() throws {
        XCTAssertEqual(try Library.parse("TENNIARB-LIBRARY://Haiodo/C4"), Library.Path(owner: "haiodo", name: "c4"))
        XCTAssertFalse(Library.isScheme("TENNIARB-LIBRARY://icons"))
        XCTAssertTrue(Library.isScheme("tenniarb-library://icons"))
    }

    func testRefusesBadNames() {
        XCTAssertEqual(
            message { _ = try Library.parse("icons") },
            "\"icons\" is not a tenniarb-library:// name")
        XCTAssertEqual(
            message { _ = try Library.parse("tenniarb-library://") },
            "library name \"\": must match \(Library.namePattern), lowercase letters digits and dashes only")
        XCTAssertEqual(
            message { _ = try Library.parse("tenniarb-library://../secret") },
            "library name \"..\": must match \(Library.namePattern), lowercase letters digits and dashes only")
        XCTAssertEqual(
            message { _ = try Library.parse("tenniarb-library://a/b/c") },
            "library \"a/b/c\": want name or owner/name, each matching \(Library.namePattern)")
        XCTAssertEqual(
            message { _ = try Library.parse("tenniarb-library:///icons") },
            "library name \"\": must match \(Library.namePattern), lowercase letters digits and dashes only")
        XCTAssertEqual(
            message { _ = try Library.Path(name: "../secret").valid() },
            "library name \"../secret\": must match \(Library.namePattern), lowercase letters digits and dashes only")
    }

    // MARK: - Folders

    /// An installed library shadows a shipped one of the same name.
    func testInstalledShadowsShipped() throws {
        let installed = try tempDir()
        let shipped = try tempDir()
        try write(iconLib, to: installed.appendingPathComponent("icons.tenn"))
        try write(iconLib, to: shipped.appendingPathComponent("icons.tenn"))

        let finder = Library.Finder(installed: [installed.path], shipped: [shipped.path])
        let file = try finder.resolve(Library.Path(name: "icons"))
        XCTAssertEqual(file.path, installed.appendingPathComponent("icons.tenn").path)

        try FileManager.default.removeItem(at: installed.appendingPathComponent("icons.tenn"))
        let next = try finder.resolve(Library.Path(name: "icons"))
        XCTAssertEqual(next.path, shipped.appendingPathComponent("icons.tenn").path)
    }

    /// TENNIARB_LIBRARIES comes before the folder the application ships, and the installed folder
    /// before both.
    func testFinderOrder() throws {
        let config = try tempDir()
        let env = try tempDir()
        let workspace = try tempDir()
        setenv("TENNIARB_CONFIG_DIR", config.path, 1)
        setenv(Library.envLibraries, env.path, 1)
        let installedDir = config.appendingPathComponent("libraries", isDirectory: true)
        try FileManager.default.createDirectory(at: installedDir, withIntermediateDirectories: true)
        try write(iconLib, to: env.appendingPathComponent("icons.tenn"))

        let finder = Library.default(workspace: workspace)
        XCTAssertEqual(finder.installed, [installedDir.path])
        XCTAssertEqual(finder.shipped.first, env.standardizedFileURL.path)
        XCTAssertEqual(try finder.resolve(Library.Path(name: "icons")).path, env.appendingPathComponent("icons.tenn").path)

        try write(iconLib, to: installedDir.appendingPathComponent("icons.tenn"))
        XCTAssertEqual(
            try finder.resolve(Library.Path(name: "icons")).path,
            installedDir.appendingPathComponent("icons.tenn").path, "an installed copy shadows the shipped one")
    }

    /// The walk up from a workspace finds the libraries of the repository, and an empty folder of
    /// that name is not one.
    func testRepoWalkFindsLibrariesAboveWorkspace() throws {
        let repo = try tempDir()
        let workspace = repo.appendingPathComponent("design/deep", isDirectory: true)
        try FileManager.default.createDirectory(at: workspace, withIntermediateDirectories: true)
        let libs = repo.appendingPathComponent("libraries", isDirectory: true)
        try FileManager.default.createDirectory(at: libs, withIntermediateDirectories: true)
        XCTAssertNil(Library.repoLibraries(workspace), "a folder without a .tenn file is not the one of the format")

        try write(iconLib, to: libs.appendingPathComponent("icons.tenn"))
        XCTAssertEqual(Library.repoLibraries(workspace), libs.standardizedFileURL.path)
    }

    func testResolveErrors() throws {
        let first = try tempDir()
        let second = try tempDir()
        XCTAssertEqual(
            message { _ = try Library.Finder().resolve(Library.Path(name: "nope")) },
            "library nope not found: no library folder to look in (set TENNIARB_LIBRARIES or install the application)")
        XCTAssertEqual(
            message {
                _ = try Library.Finder(installed: [first.path], shipped: [second.path]).resolve(
                    Library.Path(owner: "hai", name: "c4"))
            },
            "library hai/c4 not found: looked for hai/c4.tenn in \(first.path), \(second.path)")
        XCTAssertEqual(
            message { _ = try Library.Finder(shipped: [first.path, first.path]).resolve(Library.Path(name: "nope")) },
            "library nope not found: looked for nope.tenn in \(first.path)")
    }

    /// The folder reference in the app bundle is where the shipped library is looked at.
    func testShippedC4ResolvesFromBundle() throws {
        let resources = try XCTUnwrap(Bundle.main.resourceURL, "the test host has no resource folder")
        let dir = resources.appendingPathComponent("libraries", isDirectory: true)
        guard FileManager.default.fileExists(atPath: dir.appendingPathComponent("c4.tenn").path) else {
            throw XCTSkip("the app bundle has no libraries folder")
        }
        XCTAssertTrue(
            Library.shipped(workspace: nil).contains(dir.standardizedFileURL.path),
            "shipped folders: \(Library.shipped(workspace: nil))")
        let file = try Library.Finder(shipped: [dir.path]).resolve(Library.Path(name: "c4"))
        XCTAssertEqual(file.path, dir.appendingPathComponent("c4.tenn").standardizedFileURL.path)
    }

    // MARK: - Index

    func testIndexRoundTrip() throws {
        let dir = try tempDir().appendingPathComponent("libraries", isDirectory: true)
        XCTAssertEqual(try Library.loadIndex(dir: dir), [])

        let list = [
            Library.Entry(owner: "b", name: "x", url: "https://example.com/b/x", ref: "main"),
            Library.Entry(owner: "a", name: "z", url: "https://example.com/a/z", ref: "v1"),
            Library.Entry(owner: "a", name: "b", url: "https://example.com/a/b", ref: ""),
        ]
        try Library.saveIndex(dir: dir, list)
        XCTAssertEqual(try Library.loadIndex(dir: dir), [list[2], list[1], list[0]])

        let mode = try FileManager.default.attributesOfItem(atPath: dir.path)[.posixPermissions] as? NSNumber
        XCTAssertEqual(mode?.intValue, 0o700)
        let json = try String(contentsOf: dir.appendingPathComponent(Library.indexName), encoding: .utf8)
        XCTAssertTrue(json.contains("\"owner\" : \"a\""), json)
    }

    // MARK: - SceneLoader

    /// A document mounts a library by name: the text comes from the overlay under the reserved path.
    func testLoaderMountsLibrary() throws {
        let libs = try libraryEnv(iconLib)
        try write(iconLib, to: libs.appendingPathComponent("kit/icons.tenn"))
        let root = try workspace([
            "main.tenn": """
            element "icons" --from-file "tenniarb-library://icons"
            element "again" --from-file "tenniarb-library://icons"
            element "kit" --from-file "tenniarb-library://kit/icons"
            element "S" {
                use icons.zap as Z { width 24; height 24; border-color #000000 }
            }
            """
        ])
        let tree = try SceneLoader(root: root).load("main.tenn")
        XCTAssertEqual(tree.diagnostics, [])

        let reserved = "\(SceneLoader.libraryMountDir)/icons.tenn"
        let file = try XCTUnwrap(tree.file(reserved), "the library is not mounted as \(reserved): files \(tree.files)")
        XCTAssertTrue(tree.source(reserved)?.contains("M1 2 L3 4") == true)
        let main = try XCTUnwrap(tree.main)
        XCTAssertTrue(main.elems.first?.mount === file)
        XCTAssertTrue(main.elems[1].mount === file, "two mounts of one library are one file")
        XCTAssertNotNil(
            tree.file("\(SceneLoader.libraryMountDir)/kit/icons.tenn"), "an owner is a folder below the mount")
        XCTAssertEqual(tree.order.count, 3)

        let zap = try tree.resolve(main, "icons.zap")
        XCTAssertEqual(zap.pos.file, reserved)
    }

    /// A scheme name the folders do not have is an error that says what was looked for and where.
    func testLoaderReportsMissingLibrary() throws {
        let libs = try libraryEnv(iconLib)
        let root = try workspace(["main.tenn": "element \"icons\" --from-file \"tenniarb-library://nope\"\n"])
        let tree = try SceneLoader(root: root).load("main.tenn")
        let diagnostic = try XCTUnwrap(tree.diagnostics.first { !$0.warn })
        XCTAssertTrue(diagnostic.message.contains("library nope not found"), diagnostic.message)
        XCTAssertTrue(diagnostic.message.contains("nope.tenn"), diagnostic.message)
        XCTAssertTrue(diagnostic.message.contains(libs.path), diagnostic.message)
        XCTAssertEqual(diagnostic.pos.file, "main.tenn")
    }

    /// A name that leaves the folder is refused as a name, not read as a path.
    func testLoaderRefusesTraversal() throws {
        _ = try libraryEnv(iconLib)
        let root = try workspace(["main.tenn": "element \"secret\" --from-file \"tenniarb-library://../secret\"\n"])
        let tree = try SceneLoader(root: root).load("main.tenn")
        let diagnostic = try XCTUnwrap(tree.diagnostics.first { !$0.warn })
        XCTAssertTrue(diagnostic.message.contains("library name"), diagnostic.message)
    }

    /// A library file that cannot be read is reported with the name a document wrote.
    func testLoaderReportsUnreadableLibrary() throws {
        if geteuid() == 0 {
            throw XCTSkip("root reads a file whatever its permissions")
        }
        let libs = try libraryEnv(iconLib)
        let file = libs.appendingPathComponent("icons.tenn")
        try FileManager.default.setAttributes([.posixPermissions: 0o000], ofItemAtPath: file.path)
        addTeardownBlock { try? FileManager.default.setAttributes([.posixPermissions: 0o644], ofItemAtPath: file.path) }

        let root = try workspace(["main.tenn": "element \"icons\" --from-file \"tenniarb-library://icons\"\n"])
        let tree = try SceneLoader(root: root).load("main.tenn")
        let diagnostic = try XCTUnwrap(tree.diagnostics.first { !$0.warn })
        XCTAssertTrue(
            diagnostic.message.hasPrefix("library tenniarb-library://icons: cannot read \(file.path)"),
            diagnostic.message)
    }

    // MARK: - Helpers

    private func message(_ body: () throws -> Void) -> String? {
        do {
            try body()
            return nil
        } catch {
            return "\(error)"
        }
    }

    private func tempDir() throws -> URL {
        let url = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-library-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: url, withIntermediateDirectories: true)
        addTeardownBlock { try? FileManager.default.removeItem(at: url) }
        return url
    }

    private func write(_ text: String, to url: URL) throws {
        try FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
        try text.write(to: url, atomically: true, encoding: .utf8)
    }

    /// Puts lib into the folder the scheme looks at before the shipped ones; the config folder is
    /// emptied too, so a library installed on the machine cannot shadow it.
    private func libraryEnv(_ lib: String) throws -> URL {
        let dir = try tempDir()
        try write(lib, to: dir.appendingPathComponent("icons.tenn"))
        setenv(Library.envLibraries, dir.path, 1)
        setenv("TENNIARB_CONFIG_DIR", try tempDir().path, 1)
        return dir
    }

    /// A workspace on disk; the loader reads mounts from it, as go/scene does.
    private func workspace(_ files: [String: String]) throws -> URL {
        let root = try tempDir()
        for (path, text) in files {
            try write(text, to: root.appendingPathComponent(path))
        }
        return root
    }
}
