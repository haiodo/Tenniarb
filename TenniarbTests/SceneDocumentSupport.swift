//
//  SceneDocumentSupport.swift
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

/// The scene suites open documents on copies in a temp workspace. An edited document schedules an
/// autosave; left behind, it fires after the workspace is gone and AppKit asks the owner to duplicate a
/// file that no longer exists. Every document opened here is closed, unedited, before its workspace is
/// removed, and the test checks that at teardown.
class SceneDocumentTestCase: XCTestCase {

    /// The documents opened through openTestDocument, in the order they were opened.
    private var opened: [Document] = []

    /// Opens a document on url; teardown closes it, unedited, before the workspace is removed.
    @MainActor
    func openTestDocument(_ url: URL) throws -> Document {
        let document = Document()
        opened.append(document)
        addTeardownBlock {
            document.store?.modified = false  // isDocumentEdited reads the store, not the change count
            document.updateChangeCount(.changeCleared)
            document.close()
        }
        try document.read(from: url, ofType: "tenn")
        return document
    }

    /// Runs after the teardown blocks, so every document opened by the test is already closed.
    override func tearDown() {
        let documents = opened  // a copy: the closure must not carry the test case to the main actor
        MainActor.assumeIsolated {
            for document in documents {
                let name = document.fileURL?.lastPathComponent ?? "a document"
                XCTAssertFalse(document.isDocumentEdited, "\(name) is left edited: clear it before closing")
                XCTAssertTrue(document.windowControllers.isEmpty, "\(name) is left open after its workspace")
            }
        }
        super.tearDown()
    }
}
