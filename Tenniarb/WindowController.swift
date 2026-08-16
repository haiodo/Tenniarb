//
//  WindowController.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 21/06/2017.
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

import Cocoa
import Foundation

class WindowController: NSWindowController {

    override func windowTitle(forDocumentDisplayName displayName: String) -> String {
        return displayName
    }
    override func windowDidLoad() {
        super.windowDidLoad()

        // Init default preferences (lazy initialization ensures AppKit is ready)
        PreferenceConstants.preference.checkDefaults()

        // Set minimum window size to prevent constraint conflicts
        self.window?.minSize = NSSize(width: 600, height: 400)

        let ver = ProcessInfo.processInfo.operatingSystemVersion

        if ver.majorVersion == 10 && ver.minorVersion == 12 {
            self.window?.titleVisibility = .visible
            self.window?.titlebarAppearsTransparent = true
            self.window?.styleMask.remove(NSWindow.StyleMask.fullSizeContentView)
            self.window?.styleMask.remove(NSWindow.StyleMask.unifiedTitleAndToolbar)
        }
    }
}
