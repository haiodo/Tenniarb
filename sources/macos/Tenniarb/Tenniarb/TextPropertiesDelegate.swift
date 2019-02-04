//
//  TextPropertiesDelegate.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 15/01/2018.
//  Copyright © 2018 Andrey Sobolev. All rights reserved.
//

import Foundation
import Cocoa
import JavaScriptCore

let defaultFontSize = CGFloat(15)

class TennTextView: NSTextView {
    var lineNumberAttributes: [NSAttributedString.Key:Any] = [:]

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
    override func draw(_ dirtyRect: NSRect) {
        super.draw(dirtyRect)
        drawLineNumbers()
    }
    
    func initDone() {
        self.lineNumberAttributes = [
            NSAttributedString.Key.font: NSFont.toolTipsFont(ofSize: defaultFontSize),
            NSAttributedString.Key.foregroundColor: NSColor.gray,
        ]
    }
    
    func drawLineValue( _ lineNumberString:String, _ x:CGFloat, _ y:CGFloat) -> Void {
        let relativePoint = self.convert(NSZeroPoint, from: self)
        let attString = NSAttributedString(string: lineNumberString, attributes: lineNumberAttributes)
        attString.draw(at: NSPoint(x: x, y: relativePoint.y + y))
    }
    func drawLineNumbers() {
        if let layoutManager = self.layoutManager, let delegate = self.delegate as? TextPropertiesDelegate {
            let visibleGlyphRange = layoutManager.glyphRange(forBoundingRect: self.visibleRect, in: self.textContainer!)
            let firstVisibleGlyphCharacterIndex = layoutManager.characterIndexForGlyph(at: visibleGlyphRange.location)
            
            let newLineRegex = try! NSRegularExpression(pattern: "\n", options: [])
            // The line number for the first visible line
            var lineNumber = newLineRegex.numberOfMatches(in: self.string, options: [], range: NSMakeRange(0, firstVisibleGlyphCharacterIndex)) + 1
            
            var glyphIndexForStringLine = visibleGlyphRange.location
            
            // Go through each line in the string.
            while glyphIndexForStringLine < NSMaxRange(visibleGlyphRange) {
                
                // Range of current line in the string.
                let characterRangeForStringLine = (self.string as NSString).lineRange(
                    for: NSMakeRange( layoutManager.characterIndexForGlyph(at: glyphIndexForStringLine), 0 )
                )
                let glyphRangeForStringLine = layoutManager.glyphRange(forCharacterRange: characterRangeForStringLine, actualCharacterRange: nil)
                
                var glyphIndexForGlyphLine = glyphIndexForStringLine
                var glyphLineCount = 0
                
                while ( glyphIndexForGlyphLine < NSMaxRange(glyphRangeForStringLine) ) {
                    
                    // See if the current line in the string spread across
                    // several lines of glyphs
                    var effectiveRange = NSMakeRange(0, 0)
                    
                    // Range of current "line of glyphs". If a line is wrapped,
                    // then it will have more than one "line of glyphs"
                    let lineRect = layoutManager.lineFragmentUsedRect(forGlyphAt: glyphIndexForGlyphLine, effectiveRange: &effectiveRange, withoutAdditionalLayout: true)
                    
                    if glyphLineCount <= 0 {
                        if let value = delegate.expressionLines[lineNumber - 1] {
                            drawLineValue("= \(value)", lineRect.maxX + 5, lineRect.minY)
                        }
                    }
                    
                    // Move to next glyph line
                    glyphLineCount += 1
                    glyphIndexForGlyphLine = NSMaxRange(effectiveRange)
                }
                
                glyphIndexForStringLine = NSMaxRange(glyphRangeForStringLine)
                lineNumber += 1
            }
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
    
    var expressionLines: [Int: String] = [:]
    
    var element: Element?
    var diagramItem: DiagramItem?
    var tennContent: TennNode?
    
    public init(_ controller: ViewController, _ textView: NSTextView ) {
        self.controller = controller
        self.view = textView
        super.init()
        
        self.view.delegate = self
        self.view.importsGraphics = false
        self.view.allowsUndo=false
        (self.view as! TennTextView).initDone()
    }
    
    func needUpdate()-> Bool {
        return !doMerge
    }
    
    func setTextValue(_ element: Element, _ diagramItem: DiagramItem?) {
        self.element = element
        self.diagramItem = diagramItem
        
        self.tennContent = (diagramItem != nil) ? diagramItem!.toTennAsProps() : element.toTennAsProps()
        let valueStr = tennContent!.toStr()
        
        if view.string == valueStr || doMerge {
            return; // Same value
        }
        self.view.textStorage?.setAttributedString(NSAttributedString(string: valueStr))
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
        guard let tennContent = self.tennContent else {
            return
        }
        //get the range of the entire run of text
        let area = NSMakeRange(0, view.textStorage!.length)
        
        expressionLines.removeAll()
        
        //remove existing coloring
        view.textStorage?.removeAttribute(NSAttributedString.Key.foregroundColor, range: area)
        
        //add new coloring
        view.textStorage?.addAttribute(NSAttributedString.Key.foregroundColor, value: NSColor.textColor, range: area)
        
//        let lexer = TennLexer((view.textStorage?.string)!)
        
        var evaluated:[TennToken: JSValue] = [:]
        
        if let di = self.diagramItem, let ictx = self.controller.scene.executionContext.items[di] {
            evaluated = ictx.evaluated
        } else if self.diagramItem == nil, let evl = self.controller.scene.executionContext.rootCtx?.evaluated {
            evaluated = evl
        }
        
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
        
        
        tennContent.traverse({ node in
            Swift.debugPrint("Traversing \(node.token?.literal)")
            guard let tok = node.token else {
                return
            }
            switch tok.type {
            case .symbol:
                view.textStorage?.addAttribute(NSAttributedString.Key.foregroundColor, value:
                    symbolColor, range: NSMakeRange(tok.pos, tok.size))
            case .stringLit:
                // Check to include ", ' as part of sumbols.
                let start = tok.pos - 1 // Since we have ' or "
                let size = tok.size + 2
                
                view.textStorage?.addAttribute(NSAttributedString.Key.foregroundColor, value:
                    stringColor, range: NSMakeRange(start, size))
            case .markdownLit:
                // Check to include ", ' as part of sumbols.
                let start = tok.pos - 2 // Since we have ' or "
                let size = tok.size + 3
                
                view.textStorage?.addAttribute(NSAttributedString.Key.foregroundColor, value:
                    stringColor, range: NSMakeRange(start, size))
            case .expression, .expressionBlock:
                // Check to include "${' or $( as part of sumbols.
                let start = tok.pos - 2 // Since we have } or ) at end
                let size = tok.size + 3
                
                if tok.type == .expression, let evalValue = evaluated[tok] {
                    expressionLines[tok.line] = evalValue.toString()
                }
                view.textStorage?.addAttribute(NSAttributedString.Key.foregroundColor, value:
                    expressionColor, range: NSMakeRange(start, size))
                
            case .floatLit, .intLit:
                view.textStorage?.addAttribute(NSAttributedString.Key.foregroundColor, value:
                    numberColor, range: NSMakeRange(tok.pos, tok.size))
            default:
                break
            }
        })
        
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
                    self.doMerge = true
                    self.controller.mergeProperties(node)
                    self.tennContent = (self.diagramItem != nil) ? self.diagramItem!.toTennAsProps() : self.element!.toTennAsProps()                    
                    self.doMerge = false
                    self.highlight()
                }
                self.view.needsDisplay = true
            }
        })
    }
}
