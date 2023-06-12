//
//  TextPropertiesDelegate.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 15/01/2018.
//  Copyright © 2018 Andrey Sobolev. All rights reserved.
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
        drawAnnotations()
    }
    
    func initDone() {
        self.lineNumberAttributes = [
            NSAttributedString.Key.font: NSFont.toolTipsFont(ofSize: defaultFontSize),
            NSAttributedString.Key.foregroundColor: NSColor.gray,
        ]
        self.isHorizontallyResizable=true
        self.maxSize = NSSize(width: Double.greatestFiniteMagnitude, height: Double.greatestFiniteMagnitude)
        self.textContainer?.containerSize = NSSize(width: Double.greatestFiniteMagnitude, height: Double.greatestFiniteMagnitude)
        self.textContainer?.widthTracksTextView = false
    }
    
    func drawLineValue( _ lineNumberString:String, _ x:CGFloat, _ y:CGFloat) -> Void {
        let relativePoint = self.convert(NSZeroPoint, from: self)
        let attString = NSAttributedString(string: lineNumberString, attributes: lineNumberAttributes)
        attString.draw(at: NSPoint(x: x, y: relativePoint.y + y))
    }
    func drawAnnotations() {
        if let layoutManager = self.layoutManager, let delegate = self.delegate as? TextPropertiesDelegate,
            let context = NSGraphicsContext.current?.cgContext {
            
            context.saveGState()
            defer {
                context.restoreGState()
            }
            
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
                            let finalValue = value.replacingOccurrences(of: "\n", with: "\\n")
                            let lineStart = lineRect.maxX + 5
                            let textPos = max(lineRect.maxX + 10, 250)
                            drawLineValue("\(finalValue)", textPos, lineRect.minY)
                            context.move(to: CGPoint(x: lineStart, y: lineRect.midY))
                            context.addLine(to: CGPoint(x: textPos - 5, y: lineRect.midY))
                            context.setLineDash(phase: 2, lengths: [2])
                            context.drawPath(using: .stroke)
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

class TextPropertiesDelegate: NSObject, NSTextViewDelegate, NSTextDelegate, IElementModelListener {
    var controller: ViewController
    var view: NSTextView
    var changes:Int = 0
    
    var expressionLines: [Int: String] = [:]
    
    var element: Element?
    var diagramItem: DiagramItem?
    var ourUpdate = false
    
    public init(_ controller: ViewController, _ textView: NSTextView ) {
        self.controller = controller
        self.view = textView
        super.init()
        
        self.view.delegate = self
        self.view.importsGraphics = false
        self.view.allowsUndo=false
        (self.view as! TennTextView).initDone()
    }
    
    fileprivate func updateAnnotations() {
        expressionLines.removeAll()
        
        let currentText = view.string
        DispatchQueue.global(qos: .background).async {
            let p = TennParser()
            let currentNode = p.parse(currentText)
            if let ctx = self.controller.elementStore?.executionContext {
                var evaluated:[TennToken: JSValue] = [:]
                if let di = self.diagramItem {
                    evaluated = ctx.getEvaluated(di, currentNode, self.controller.scene.scene?.drawables[di])
                } else if self.diagramItem == nil, let ictx = ctx.rootCtx, ictx.element == self.element {
                    evaluated = ctx.getEvaluated(self.element!, currentNode)
                }
                for (k,v) in evaluated {
                    self.expressionLines[k.line] = v.toString()
                }
            }
            
            DispatchQueue.main.async {
                self.view.setNeedsDisplay(self.view.bounds, avoidAdditionalLayout: true)
            }
        }
    }
    
    func notifyChanges(_ event: ModelEvent) {
        // We need to handle our element changes in case it is not called by us
        if self.ourUpdate {
            self.ourUpdate = false
            updateAnnotations()
        } else {
            DispatchQueue.main.async(execute: {
                self.setTextValue(self.element, self.diagramItem)
            })
        }
    }
    
    func setTextValue(_ element: Element?, _ diagramItem: DiagramItem?) {
        self.element = element
        self.diagramItem = diagramItem
        
        guard let tennContent = (diagramItem != nil) ? diagramItem!.toTennAsProps() : element?.toTennAsProps() else {
            return
        }
        let valueStr = tennContent.toStr()
        
        let attrStr = tennContent.toAttributedStr(NSFont.systemFont(ofSize: defaultFontSize), NSColor.textColor)
        
        let p = TennParser()
        let minified = p.parse(view.string).toStr()
        
        if minified == valueStr {
            // Annotation could be different
            DispatchQueue.main.asyncAfter(deadline: .now(), execute: {
                //                self.highlight()
                self.updateAnnotations()
            })
            return; // Same value, do not need to modify curren test
        }
        self.view.isAutomaticQuoteSubstitutionEnabled = false
        self.view.font = NSFont.systemFont(ofSize: defaultFontSize)
        self.view.textColor = NSColor.textColor
        self.view.textStorage?.setAttributedString(attrStr)
        self.view.scrollToBeginningOfDocument(self)
        
        // We need to register self to listen for model changes to update annotations
        if !self.controller.elementStore!.onUpdate.contains(where: {$0 is TextPropertiesDelegate}) {
            self.controller.elementStore?.onUpdate.append(self)
        }
        
        DispatchQueue.main.asyncAfter(deadline: .now(), execute: {
            //            self.highlight()
            self.updateAnnotations()
        })
    }
    
    func textDidChange(_ notification: Notification) {
        changes += 1
        sheduleUpdate()
    }
    
    func highlight() {
        let lexer = TennLexer((view.textStorage?.string)!)
        
        let darkMode = PreferenceConstants.preference.darkMode
        
        let symbolColor = !darkMode ? TennColors.symbolColorWhite: TennColors.symbolColorDark
        let stringColor = !darkMode ? TennColors.stringColorWhite: TennColors.stringColorDark
        let numberColor = !darkMode ? TennColors.numberColorWhite: TennColors.numberColorDark
        let expressionColor = !darkMode ? TennColors.expressionColorWhite: TennColors.expressionColorDark
        
        //get the range of the entire run of text
        let textLength = view.textStorage!.length
        let area = NSMakeRange(0, textLength)

        class Attr {
            let key: NSAttributedString.Key
            let value: Any
            let range: NSRange
            init(key: NSAttributedString.Key, value: Any, range: NSRange) {
                self.key = key
                self.value = value
                self.range = range
            }
        }
        
        var attrs: [Attr] = []
        
        while( true ) {
            guard let tok = lexer.getToken() else {
                break
            }
            if tok.size == 0 {
                continue
            }
            switch tok.type {
            case .symbol:
                if tok.size > 0 {
                    attrs.append(Attr(key: NSAttributedString.Key.foregroundColor, value: symbolColor, range: NSMakeRange(tok.pos, tok.size)))
                }
            case .stringLit:
                // Check to include ", ' as part of sumbols.
                let start = tok.pos - 1 // Since we have ' or "
                let size = tok.size + 2
                
                if size > 0  {
                    attrs.append(Attr(key: NSAttributedString.Key.foregroundColor, value: stringColor, range: NSMakeRange(start, size)))
                }
            case .markdownLit:
                // Check to include ", ' as part of sumbols.
                let start = tok.pos - 2 // Since we have ' or "
                let size = tok.size + 3
                
                if size > 0 {
                    attrs.append(Attr(key: NSAttributedString.Key.foregroundColor, value: stringColor, range: NSMakeRange(start, size)))
                }
            case .expression, .expressionBlock:
                // Check to include "${' or $( as part of sumbols.
                let start = tok.pos - 1 // Since we have } or ) at end
                let size = tok.size + 2
                
                attrs.append(Attr(key: NSAttributedString.Key.foregroundColor, value: expressionColor, range: NSMakeRange(start, size)))
                
            case .floatLit, .intLit:
                attrs.append(Attr(key: NSAttributedString.Key.foregroundColor, value: numberColor, range: NSMakeRange(tok.pos, tok.size)))
            default:
                break
            }
        }
        //remove existing coloring
        view.textStorage?.removeAttribute(NSAttributedString.Key.foregroundColor, range: area)
        
        //add new coloring
        view.textStorage?.addAttribute(NSAttributedString.Key.foregroundColor, value: NSColor.textColor, range: area)
        
        for attr in attrs {
            var r = attr.range
            if r.upperBound >= view.textStorage!.length {
                r = NSMakeRange(r.lowerBound, view.textStorage!.length - r.upperBound - 1)
            }
            if r.length > 0 {
                view.textStorage?.addAttribute(attr.key, value: attr.value, range: r)
            }
            
        }
        view.needsDisplay=true
    }
    
    func textDidEndEditing(_ notification: Notification) {
        sheduleUpdate()        
    }
    
    func generateTextContent(_ parser: TennParser) -> TennNode {
        let textStorage = self.view.textStorage!
        let result = textStorage.string
        var finalResult = ""
        var idx = 0
        var images: [String:String] = [:]
        for c in result {
            if let attr = textStorage.attribute(NSAttributedString.Key.attachment, at: idx, effectiveRange: nil),
                let attachment = attr as? NSTextAttachment,
                let image = attachment.image {
                if let tiffData = image.tiffRepresentation {
                    let imageRep = NSBitmapImageRep(data: tiffData)
                    if let pngData = imageRep?.representation(using: .png, properties: [:]) {
                        let nme = "image:\(c)-\(idx)"
                        images[nme] = pngData.base64EncodedString()
                        finalResult.append("@(\(nme))")
                    }
                }
            }
            else {
                finalResult.append(c)
            }
            idx += 1
        }
        let node = parser.parse(finalResult)
        
        if parser.errors.hasErrors() {
            return node
        }
        
        // We need to iterate and update images.
        
        node.traverse {nde in
            if nde.kind == .Image, let t = nde.token, let imgData = images[t.literal] {
                nde.token = TennToken(type: .imageData, literal: imgData)
            }
        }
        
        return node
    }
    
    public func sheduleUpdate( ) {
        let curChanges = self.changes
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.3, execute: {
            if curChanges == self.changes {
                let parser = TennParser()
                let node = self.generateTextContent(parser)
                
                if parser.errors.hasErrors() {
                    self.view.textColor = NSColor(red: 1.0, green: 0, blue: 0, alpha: 0.8)
                }
                else {
                    self.updateAnnotations()
                    self.highlight()
                    self.ourUpdate = true
                    self.controller.mergeProperties(node)                 
                }
                self.view.needsDisplay = true
            }
        })
    }
}
