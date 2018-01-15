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
    
    public init(_ controller: ViewController, _ textView: NSTextView ) {
        self.controller = controller
        self.view = textView
        super.init()
        
        self.view.delegate = self
    }
    
    func setTextValue(_ value: String) {
        let style = NSMutableParagraphStyle()
        style.headIndent = 50
        
        style.alignment = .justified
        style.firstLineHeadIndent = 50
        
        self.view.textStorage?.setAttributedString(NSAttributedString(string: value))
        self.view.isAutomaticQuoteSubstitutionEnabled = false
        self.view.font = NSFont.systemFont(ofSize: 15.0)
        self.view.textColor = NSColor(red: 0.0, green: 0.0, blue: 0.0, alpha: 0.8)
        
        self.view.scrollToBeginningOfDocument(self)
        
//        self.view.delegate = self
    }
    func textDidChange(_ notification: Notification) {
        changes += 1
        sheduleUpdate()
    }
    
    fileprivate func sheduleUpdate( ) {
        let curChanges = self.changes
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.3, execute: {
            if curChanges == self.changes {
                let parser = TennParser()
                let node = parser.parse(self.view.textStorage!.string)
                if parser.errors.hasErrors() {
                    self.view.textColor = NSColor(red: 1.0, green: 0, blue: 0, alpha: 0.8)
                }
                else {
                    self.view.textColor = NSColor(red: 0.0, green: 0.0, blue: 0.0, alpha: 0.8)
                }
                Swift.debugPrint(node.toStr())
                self.view.needsDisplay = true
            }
        })
    }
    
    func textView(_ textView: NSTextView, doCommandBy commandSelector: Selector) -> Bool {
        Swift.debugPrint("Selector:" + commandSelector.description)
        return false
    }
    
}
