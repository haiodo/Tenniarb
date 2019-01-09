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
        let insertPart = "\n"
        
        let str = NSAttributedString(
            string:insertPart,
            attributes:[NSAttributedString.Key.font:NSFont.systemFont(ofSize: defaultFontSize)]
        )
        self.textStorage?.insert(str, at: loc)
        
        if let dlg = (self.delegate as? TextPropertiesDelegate) {
            dlg.sheduleUpdate()
        }
    }
    override func insertTab(_ sender: Any?) {
        let str = NSAttributedString(
            string:"    ",
            attributes:[NSAttributedString.Key.font:NSFont.systemFont(ofSize: defaultFontSize)])
        self.textStorage?.insert(str, at: self.selectedRange().location)
        if let dlg = (self.delegate as? TextPropertiesDelegate) {
            dlg.sheduleUpdate()
        }
    }
}

class TextPropertiesDelegate: NSObject, NSTextViewDelegate, NSTextDelegate {
    var controller: ViewController
    var view: NSTextView
    var changes:Int = 0
    var doMerge:Bool = false
    
    let symbolColorWhite = NSColor(red: 0x81/255.0, green: 0x5f/255.0, blue: 0x03/255.0, alpha: 1)
    let stringColorWhite = NSColor(red: 0x1c/255.0, green: 0x00/255.0, blue: 0xcf/255.0, alpha: 1)
    let numberColorWhite = NSColor(red: 0x1c/255.0, green: 0x00/255.0, blue: 0xcf/255.0, alpha: 1)
    let expressionColorWhite = NSColor(red: 100/255.0, green: 100/255.0, blue: 133/255.0, alpha: 1)
    
    let symbolColorDark = NSColor(red: 0x75/255.0, green: 0xb4/255.0, blue: 0x92/255.0, alpha: 1)
    let stringColorDark = NSColor(red: 0xfc/255.0, green: 0x6a/255.0, blue: 0x5d/255.0, alpha: 1)
    let numberColorDark = NSColor(red: 0x96/255.0, green: 0x86/255.0, blue: 0xf5/255.0, alpha: 1)
    let expressionColorDark = NSColor(red: 198/255.0, green: 124/255.0, blue: 72/255.0, alpha: 1)
    
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
        view.textStorage?.removeAttribute(NSAttributedString.Key.foregroundColor, range: area)
        
        //add new coloring
        view.textStorage?.addAttribute(NSAttributedString.Key.foregroundColor, value: NSColor.textColor, range: area)
        
        let lexer = TennLexer((view.textStorage?.string)!)
        
        var darkMode = false
        if #available(OSX 10.14, *) {
            if NSAppearance.current.name == NSAppearance.Name.darkAqua  || NSAppearance.current.name == NSAppearance.Name.vibrantDark {
                darkMode = true
            }
        }
        
        let symbolColor = !darkMode ? symbolColorWhite: symbolColorDark
        let stringColor = !darkMode ? stringColorWhite: stringColorDark
        let numberColor = !darkMode ? numberColorWhite: numberColorDark
        let expressionColor = !darkMode ? expressionColorWhite: expressionColorDark
        
        var tok = lexer.getToken()
        while tok != nil {
            switch tok!.type {
            case .symbol:
                view.textStorage?.addAttribute(NSAttributedString.Key.foregroundColor, value:
                    symbolColor, range: NSMakeRange(tok!.pos, tok!.size))
            case .stringLit:
                // Check to include ", ' as part of sumbols.
                let start = tok!.pos - 1 // Since we have ' or "
                let size = tok!.size + 2
                
                view.textStorage?.addAttribute(NSAttributedString.Key.foregroundColor, value:
                    stringColor, range: NSMakeRange(start, size))
            case .expression, .expressionBlock:
                // Check to include "${' or $( as part of sumbols.
                let start = tok!.pos - 2 // Since we have } or ) at end
                let size = tok!.size + 3
                
                view.textStorage?.addAttribute(NSAttributedString.Key.foregroundColor, value:
                    expressionColor, range: NSMakeRange(start, size))
                
            case .floatLit, .intLit:
                view.textStorage?.addAttribute(NSAttributedString.Key.foregroundColor, value:
                    numberColor, range: NSMakeRange(tok!.pos, tok!.size))
            default:
                break
            }
            
            tok = lexer.getToken()
        }
        
        view.needsDisplay=true
    }
    
    func textDidEndEditing(_ notification: Notification) {
        highlight()
    }
    
    public func sheduleUpdate( ) {
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
                    self.highlight()
                    self.doMerge = true
                    self.controller.mergeProperties(node)
                    self.doMerge = false
                }
                self.view.needsDisplay = true
            }
        })
    }
}
