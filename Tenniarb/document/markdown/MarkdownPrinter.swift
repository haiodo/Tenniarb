//
//  MarkdownPrinter.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 01.10.2019.
//  Copyright © 2019 Andrey Sobolev. All rights reserved.
//

import Foundation
import Cocoa

typealias attrType = NSAttributedString.Key

class MarkDownAttributedPrinter {
    fileprivate static func calcTitleFontSize(_ text: String, fontSize: CGFloat) -> (String, CGFloat, Int) {
        var hlevel = 0
        var calch = true
        var result = ""
        for c in text {
            if calch {
                if c == "#" {
                    hlevel += 1
                    continue
                } else if c == " " || c == "\t" {
                    continue
                }
                else {
                    calch = false
                }
            }
            result.append(c)
        }
        return (result, fontSize + 5 - CGFloat(hlevel * 2), hlevel)
    }
    
    private static func attrStr(_ text: String, _ attributes: [NSAttributedString.Key: Any]) -> NSAttributedString {
        return NSAttributedString(string: text, attributes: attributes)
    }
    public static func toAttributedStr(_ tokens: [MarkdownToken], font originalFont: NSFont, paragraphStyle: NSParagraphStyle, foregroundColor: NSColor, shift: inout CGPoint, imageProvider: ImageProvider) -> NSMutableAttributedString {
        let result = NSMutableAttributedString()
        
        var currentColor = foregroundColor
        var font = originalFont
        
        var pos = 0
        var prevMultiCode = false
        var lastLiteral = ""
        var lastToken: MarkdownTokenType = .eof
        for t in tokens {
            var literal = t.literal
            if prevMultiCode {
                prevMultiCode = false
                if literal.hasPrefix("\n") {
                    literal = String(literal.suffix(literal.count-1))
                }
                let colorValue = NSColor(cgColor: parseColor("grey-200"))!
                result.append(attrStr("\n",[
                    attrType.font: font,
                    attrType.paragraphStyle: paragraphStyle,
                    attrType.foregroundColor: NSColor.black,
                    attrType.backgroundColor: colorValue,
                ]))
            }
            if t.type != .eof {
                lastToken = t.type
            }
            switch t.type {
            case .text:
                result.append(
                    attrStr( literal,[
                        attrType.font: font,
                        attrType.paragraphStyle: paragraphStyle,
                        attrType.foregroundColor: currentColor
                    ])
                )
            case .bold:
                result.append(attrStr(literal, [
                    attrType.font: NSFont.boldSystemFont(ofSize: font.pointSize),
                    attrType.paragraphStyle: paragraphStyle,
                    attrType.foregroundColor: currentColor
                ]))
                break;
            case .bullet:
                let ps = paragraphStyle.mutableCopy() as! NSMutableParagraphStyle
                ps.headIndent = CGFloat( 5 * literal.count )
                let l = NSTextList(markerFormat: .diamond, options: 0)
                l.startingItemNumber = 1
                ps.textLists.append(l)
                if ps.headIndent > shift.x {
                    shift.x = ps.headIndent
                }
                result.append(attrStr("•", [
                    attrType.font: font,
                    attrType.paragraphStyle: ps,
                    attrType.foregroundColor: currentColor
                ]))
                break;
            case .image:
                                
                let (image, rect) = imageProvider.resolveImage( path: t.literal )
                if let img = image, let r = rect {
                    let image1Attachment = NSTextAttachment()
                    image1Attachment.image = img
                    var bnds = r
                    bnds.origin.y = font.capHeight/2 + -1 * bnds.height / 2
                    image1Attachment.bounds = bnds
                                        
                    let strImg = NSMutableAttributedString(attachment: image1Attachment)
                    strImg.addAttributes([
                        attrType.paragraphStyle: paragraphStyle,
                        attrType.font: font],
                                         range: NSMakeRange(0, strImg.length))
                    
                        // We need to add at least one space to be able to see image.
                        
                        strImg.append(attrStr(" ",[
                            attrType.font: font,
                            attrType.paragraphStyle: paragraphStyle,
                            attrType.foregroundColor: NSColor.black,
                        ]))
                    
                    result.append(strImg)
                }
                break;
            case .italic:
                result.append(attrStr(literal, [
                    attrType.font: NSFontManager.shared.convert(font, toHaveTrait: .italicFontMask),
                    attrType.paragraphStyle: paragraphStyle,
                    attrType.foregroundColor: currentColor
                ]))
            case .underline:
                result.append(attrStr(literal, [
                    attrType.font: font,
                    attrType.paragraphStyle: paragraphStyle,
                    attrType.foregroundColor: currentColor,
                    attrType.underlineStyle: NSUnderlineStyle.single.rawValue
                ]))
            case .scratch:
                result.append(attrStr(literal, [
                    attrType.font: font,
                    attrType.paragraphStyle: paragraphStyle,
                    attrType.foregroundColor: currentColor,
                    attrType.strikethroughStyle: NSUnderlineStyle.single.rawValue
                ]))
            case .title:
                let (title, titleSize, hlevel) = calcTitleFontSize(literal, fontSize: font.pointSize)
                let ps = paragraphStyle.mutableCopy() as! NSMutableParagraphStyle
                ps.headerLevel = hlevel
                ps.paragraphSpacing = 5
                
                result.append(attrStr(title,[
                    attrType.font: NSFont.systemFont(ofSize: titleSize ),
                    attrType.paragraphStyle: ps,
                    attrType.foregroundColor: currentColor
                ]))
            case .color:
                if let splitPos = literal.firstIndex(of: "|") {
                    let color = String(literal.prefix(upTo: splitPos))
                    let word = String(literal.suffix(from: literal.index(after: splitPos)))
                    
                    let colorValue = NSColor(cgColor: parseColor(color))!
                    result.append(attrStr(word,[
                        attrType.font: font,
                        attrType.paragraphStyle: paragraphStyle,
                        attrType.foregroundColor: colorValue
                    ]))
                } else {
                    if literal.trimmingCharacters(in: CharacterSet.whitespacesAndNewlines).count == 0 {
                        currentColor = foregroundColor
                    } else {
                        currentColor = NSColor(cgColor: parseColor(literal))!
                    }
                }
                break
            case .font:
                if let splitPos = literal.firstIndex(of: "|") {
                    if let fontSize = Double(String(literal.prefix(upTo: splitPos))) {
                    let word = String(literal.suffix(from: literal.index(after: splitPos)))
                    
                        result.append(attrStr(word,[
                            attrType.font: NSFont.systemFont(ofSize: CGFloat(fontSize)),
                            attrType.paragraphStyle: paragraphStyle,
                            attrType.foregroundColor: currentColor
                        ]))
                    }
                } else {
                    if literal.trimmingCharacters(in: CharacterSet.whitespacesAndNewlines).count == 0 {
                        font = originalFont
                    } else {
                        if let fontSize = Double(String(literal)) {
                            font = NSFont.systemFont(ofSize: CGFloat(fontSize))
                        } else {
                            font = originalFont
                        }
                    }
                }
                break
            case .code:
                let colorValue = NSColor(cgColor: parseColor("grey-200"))!
                if literal.contains("\n") {
                    if !literal.hasPrefix("\n") && !lastLiteral.hasSuffix("\n") {
                        result.append(attrStr("\n",[
                            attrType.font: font,
                            attrType.paragraphStyle: paragraphStyle,
                            attrType.foregroundColor: NSColor.black,
                        ]))
                    }
                    if !literal.hasSuffix("\n") {
                        prevMultiCode = true
                    }
                }
                result.append(attrStr(literal,[
                    attrType.font: font,
                    attrType.paragraphStyle: paragraphStyle,
                    attrType.foregroundColor: NSColor.black,
                    attrType.backgroundColor: colorValue,
                ]))
            default:
                break;
            }
            pos += 1
            lastLiteral = literal
        }
        
        if lastToken == .image {
            result.append(attrStr("\n",[
                attrType.font: font,
                attrType.paragraphStyle: paragraphStyle,
                attrType.foregroundColor: NSColor.black
            ]))
        }
    
//        var table = NSTextTable()
//        table.numberOfColumns = 2
//        table.collapsesBorders = true
//        table.backgroundColor = .brown
//
//        func makeCell(row: Int, col column: Int, text: String) -> NSMutableAttributedString {
//            let textBlock = NSTextTableBlock(table: table, startingRow: row, rowSpan: 1, startingColumn: column, columnSpan: 1)
//
//            textBlock.setBorderColor(.black)
//            textBlock.setWidth(5.0, type: NSTextBlock.ValueType.absoluteValueType, for: NSTextBlock.Layer.border)
//            textBlock.setWidth(5.0, type: NSTextBlock.ValueType.absoluteValueType, for: NSTextBlock.Layer.padding)
//            
//            let paragraph = NSMutableParagraphStyle()
//            paragraph.textBlocks = [textBlock]
//
//            let cell = NSMutableAttributedString(string: text + "\n", attributes: [
//                .paragraphStyle: paragraph,
//            ])
//
//            return cell
//        }
//
//        let content = NSMutableAttributedString(string: "some text")
//        content.append(NSAttributedString(string: "\n")) // this newline is required in case content is not empty.
//
//        //If you append table cells to some text without newline, the first row might not show properly.
//        content.append(makeCell(row: 0, col: 0, text: "c00"))
//        content.append(makeCell(row: 0, col: 1, text: "c 0 1"))
//        content.append(makeCell(row: 1, col: 0, text: "c 1 0"))
//        content.append(makeCell(row: 1, col: 1, text: "c11"))
//        
//        result.append(content)
        return result
    }
}

