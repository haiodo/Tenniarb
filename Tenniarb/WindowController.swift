//
//  WindowController.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 21/06/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Foundation

import Cocoa

class WindowController: NSWindowController {
    
    override func windowTitle(forDocumentDisplayName displayName: String) -> String {
        return displayName
    }
    override func windowDidLoad() {
        // Init default preferences
        PreferenceConstants.preference.checkDefaults()
        
        let ver = ProcessInfo.processInfo.operatingSystemVersion
        
        if ver.majorVersion == 10 && ver.minorVersion == 12 {    
            self.window?.titleVisibility = .visible
            self.window?.titlebarAppearsTransparent = true
            self.window?.styleMask.remove(NSWindow.StyleMask.fullSizeContentView)
            self.window?.styleMask.remove(NSWindow.StyleMask.unifiedTitleAndToolbar)
        }
    }
}
