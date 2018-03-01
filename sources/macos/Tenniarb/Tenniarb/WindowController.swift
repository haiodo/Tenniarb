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
}
