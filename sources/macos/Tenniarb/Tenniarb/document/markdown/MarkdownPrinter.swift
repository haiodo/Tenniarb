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
    public static func toAttributedStr(_ tokens: [MarkdownToken], font: NSFont, paragraphStyle: NSParagraphStyle, foregroundColor: NSColor, shift: inout CGPoint) -> NSAttributedString {
        let result = NSMutableAttributedString()
        
        var currentColor = foregroundColor
        let italicFont = NSFontManager.shared.convert(font, toHaveTrait: .italicFontMask)
        
        var pos = 0
        var prevMultiCode = false
        var lastLiteral = ""
        for t in tokens {
            var literal = t.literal
            if prevMultiCode {
                prevMultiCode = false
                if literal.hasPrefix("\n") {
                    literal = String(literal.suffix(literal.count-1))
                }
                let colorValue = NSColor(cgColor: parseColor("grey-200"))!
                let ps = paragraphStyle.mutableCopy() as! NSMutableParagraphStyle
                result.append(attrStr("\n",[
                    attrType.font: font,
                    attrType.paragraphStyle: ps,
                    attrType.foregroundColor: NSColor.black,
                    attrType.backgroundColor: colorValue,
                ]))
            }
            switch t.type {
            case .text:
                result.append(
                    attrStr( literal,[
                        attrType.font: font, attrType.paragraphStyle: paragraphStyle,
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
                break;
            case .italic:
                result.append(attrStr(literal, [
                    attrType.font: italicFont,
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
            case .code:
                let colorValue = NSColor(cgColor: parseColor("grey-200"))!
                let ps = paragraphStyle.mutableCopy() as! NSMutableParagraphStyle
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
                    attrType.paragraphStyle: ps,
                    attrType.foregroundColor: NSColor.black,
                    attrType.backgroundColor: colorValue,
                ]))
            default:
                break;
            }
            pos += 1
            lastLiteral = literal
        }
        
        return result
    }
}
