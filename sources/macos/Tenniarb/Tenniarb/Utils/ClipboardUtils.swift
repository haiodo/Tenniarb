//
//  ClipboardUtils.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 20.07.2019.
//  Copyright © 2019 Andrey Sobolev. All rights reserved.
//

import Foundation
import Cocoa

class ClipboardUtils {
    static func canPaste() -> Bool {
        if let value = NSPasteboard.general.string(forType: .string) {
            let p = TennParser()
            _ = p.parse(value)
            if p.errors.hasErrors() {
                return false // If there is errors, we could not paste.
            }
            return true
        }
        return false
    }
    static func paste( _ operation: (TennNode) -> Void ) {
//        let av = NSPasteboard.general.availableType(from: [.png, .tiff, .URL])
        if let value = NSPasteboard.general.data(forType: .png) {
            if let name = NSPasteboard.general.string(forType: .string) {
                let node = TennNode.newCommand("image", TennNode.newStrNode(name), TennNode.newImageNode(value.base64EncodedString()))
            operation(node)
            }
        }
        
        if let value = NSPasteboard.general.data(forType: .tiff) {
            if let name = NSPasteboard.general.string(forType: .string) {
                let node = TennNode.newCommand("image", TennNode.newStrNode(name), TennNode.newImageNode(value.base64EncodedString()))
            operation(node)
            }
        }
        
        if let value = NSPasteboard.general.string(forType: .string) {
            let p = TennParser()
            let node = p.parse(value)
            if p.errors.hasErrors() {
                return // If there is errors, we could not paste.
            }
            operation(node)
        }
    }
    static func copy( _ node: TennNode) {
        NSPasteboard.general.clearContents()
        let value = node.toStr()
        NSPasteboard.general.setString(value, forType: .string)
    }
}
