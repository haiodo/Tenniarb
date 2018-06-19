//
//  TextPropertiesDelegate.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 15/01/2018.
//  Copyright © 2018 Andrey Sobolev. All rights reserved.
//

import Foundation
import Cocoa

class TextPropertiesDelegate: NSObject, NSTextViewDelegate, NSTouchBarDelegate {
    var controller: ViewController
    var view: NSTextView
    var changes:Int = 0
    var doMerge:Bool = false
    
    public init(_ controller: ViewController, _ textView: NSTextView ) {
        self.controller = controller
        self.view = textView
        super.init()
        
        self.view.delegate = self
        self.view.allowsUndo=false
    }
    
    func needUpdate()-> Bool {
        return !doMerge
    }
    
    
    func setTextValue(_ value: String) {
        if view.string == value || doMerge {
            return; // Same value
        }
        let style = NSMutableParagraphStyle()
        style.headIndent = 50
        
        style.alignment = .justified
        style.firstLineHeadIndent = 50
        
        self.view.textStorage?.setAttributedString(NSAttributedString(string: value))
        self.view.isAutomaticQuoteSubstitutionEnabled = false
        self.view.font = NSFont.systemFont(ofSize: 15.0)
        self.view.textColor = NSColor.textColor
        
        self.view.scrollToBeginningOfDocument(self)
    }
    
    func textDidChange(_ notification: Notification) {
        changes += 1
        sheduleUpdate()
    }
    
    func textDidEndEditing(_ notification: Notification) {
        Swift.debugPrint("End editing")
    }
    
    fileprivate func sheduleUpdate( ) {
        let curChanges = self.changes
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.3, execute: {
            if curChanges == self.changes {
                let parser = TennParser()
                let textContent = self.view.textStorage!.string
                let node = parser.parse(textContent)
                if parser.errors.hasErrors() {
                    self.view.textColor = NSColor(red: 1.0, green: 0, blue: 0, alpha: 0.8)
                }
                else {
                    self.view.textColor = NSColor(red: 0.0, green: 0.0, blue: 0.0, alpha: 0.8)
                    self.doMerge = true
                    self.controller.mergeProperties(node)
                    self.doMerge = false
                }
                self.view.needsDisplay = true
            }
        })
    }
    
    func textView(_ textView: NSTextView, doCommandBy commandSelector: Selector) -> Bool {
//        Swift.debugPrint("Selector:" + commandSelector.description)
        return false
    }
    
}
