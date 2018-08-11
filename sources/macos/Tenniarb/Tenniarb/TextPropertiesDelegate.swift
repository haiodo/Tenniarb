//
//  TextPropertiesDelegate.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 15/01/2018.
//  Copyright © 2018 Andrey Sobolev. All rights reserved.
//

import Foundation
import Cocoa

let defaultFontSize = CGFloat(15)

class TennTextView: NSTextView {
    override func insertNewline(_ sender: Any?) {
        let loc = self.selectedRange().location
        let parser = TennParser()
        var insertPart = "\n"
        
        let node = parser.parse(self.textStorage!.string)
        if !parser.errors.hasErrors() {
            // No errors found
            
//            self.textStorage.string
//            node.find(loc)
        }
        
        let str = NSAttributedString(
            string:insertPart,
            attributes:[NSAttributedStringKey.font:NSFont.systemFont(ofSize: defaultFontSize)]
        )
        self.textStorage?.insert(str, at: loc)
    }
    override func insertTab(_ sender: Any?) {
        let str = NSAttributedString(
            string:"    ",
            attributes:[NSAttributedStringKey.font:NSFont.systemFont(ofSize: defaultFontSize)])
        self.textStorage?.insert(str, at: self.selectedRange().location)
    }
}

class TextPropertiesDelegate: NSObject, NSTextViewDelegate, NSTextDelegate {
    var controller: ViewController
    var view: NSTextView
    var changes:Int = 0
    var doMerge:Bool = false
    
    public init(_ controller: ViewController, _ textView: NSTextView ) {
        self.controller = controller
        self.view = textView
        super.init()
        
        self.view.delegate = self
        self.view.importsGraphics = false
        self.view.allowsUndo=false
    }
    
    func needUpdate()-> Bool {
        return !doMerge
    }
    
    func setTextValue(_ value: String) {
        if view.string == value || doMerge {
            return; // Same value
        }
        self.view.textStorage?.setAttributedString(NSAttributedString(string: value))
        self.view.isAutomaticQuoteSubstitutionEnabled = false
        self.view.font = NSFont.systemFont(ofSize: defaultFontSize)
        self.view.textColor = NSColor.textColor
        
        highlight()
        
        self.view.scrollToBeginningOfDocument(self)
    }

    func textDidChange(_ notification: Notification) {
        changes += 1
        sheduleUpdate()
    }
    
    func highlight() {
        //get the range of the entire run of text
        let area = NSMakeRange(0, view.textStorage!.length)
        //remove existing coloring
        view.textStorage?.removeAttribute(NSAttributedStringKey.foregroundColor, range: area)
        
        //add new coloring
        view.textStorage?.addAttribute(NSAttributedStringKey.foregroundColor, value: NSColor.textColor, range: area)
        
        let lexer = TennLexer((view.textStorage?.string)!)
        
        var tok = lexer.getToken()
        while tok != nil {
            switch tok!.type {
            case .symbol:
                view.textStorage?.addAttribute(NSAttributedStringKey.foregroundColor, value:
                    NSColor(red: 41/255.0, green: 66/255.0, blue: 119/255.0, alpha: 1),
                                               range: NSMakeRange(tok!.pos, tok!.size))
            case .stringLit, .charLit:
                view.textStorage?.addAttribute(NSAttributedStringKey.foregroundColor, value:
                    NSColor(red: 195/255.0, green: 116/255.0, blue: 28/255.0, alpha: 1),
                                               range: NSMakeRange(tok!.pos, tok!.size))
            case .floatLit, .intLit:
                view.textStorage?.addAttribute(NSAttributedStringKey.foregroundColor, value:
                    NSColor(red: 41/255.0, green: 66/255.0, blue: 119/255.0, alpha: 1),
                                               range: NSMakeRange(tok!.pos, tok!.size))
            default:
                break
            }
            
            tok = lexer.getToken()
        }
        
        view.needsDisplay=true
    }
    
    func textDidEndEditing(_ notification: Notification) {
        Swift.debugPrint("End editing")
        highlight()
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
//                    self.view.textColor = NSColor(red: 0.0, green: 0.0, blue: 0.0, alpha: 0.8)
                    self.highlight()
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
