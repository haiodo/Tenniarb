//
//  AttributedPrinter.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 18.09.2019.
//  Copyright © 2019 Andrey Sobolev. All rights reserved.
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

class TennColors {
    public static let symbolColorWhite = NSColor(red: 0x81/255.0, green: 0x5f/255.0, blue: 0x03/255.0, alpha: 1)
    public static let stringColorWhite = NSColor(red: 0x1c/255.0, green: 0x00/255.0, blue: 0xcf/255.0, alpha: 1)
    public static let numberColorWhite = NSColor(red: 0x1c/255.0, green: 0x00/255.0, blue: 0xcf/255.0, alpha: 1)
    public static let expressionColorWhite = NSColor(red: 100/255.0, green: 100/255.0, blue: 133/255.0, alpha: 1)

    public static let symbolColorDark = NSColor(red: 0x75/255.0, green: 0xb4/255.0, blue: 0x92/255.0, alpha: 1)
    public static let stringColorDark = NSColor(red: 0xfc/255.0, green: 0x6a/255.0, blue: 0x5d/255.0, alpha: 1)
    public static let numberColorDark = NSColor(red: 0x96/255.0, green: 0x86/255.0, blue: 0xf5/255.0, alpha: 1)
    public static let expressionColorDark = NSColor(red: 198/255.0, green: 124/255.0, blue: 72/255.0, alpha: 1)
}

extension TennNode {
    private static let spaces = "    "
    private func makeSeq(_ sb: inout NSMutableAttributedString, pattern: String, count: Int ) {
        if count > 0 {
            for _ in 0..<count {
                sb.append(NSAttributedString(string: pattern))
            }
        }
    }
    private func getSeq(pattern: String, count: Int ) -> String {
        var sb = ""
        if count > 0 {
            for _ in 0..<count {
                sb.append(pattern)
            }
        }
        return sb
    }
    private func quote(_ val: String, _ tok: String = "\"") -> String {
        return val.replacingOccurrences(of: tok, with: "\\" + tok, options: .literal, range: nil)
    }
    
    public func childsToStr(_ font: NSFont, _ textColor: NSColor, _ result: inout NSMutableAttributedString, _ ind: Int, _ clean: Bool) {
        if let children = self.children {
            var i = 0
            for c in children {
                result.append(c.toAttributedStr(font, textColor, ind, clean))
                if i != children.count - 1 {
                    if self.kind == .BlockExpr || self.kind == .Statements {
                        result.append(NSAttributedString(string:"\n"))
                    }
                    else {
                        result.append(NSAttributedString(string:" "))
                    }
                }
                i += 1
            }
        }
    }
    
    public func toAttributedStr( _ font: NSFont, _ textColor: NSColor, _ indent: Int = 0, _ clean: Bool = false) -> NSAttributedString {
        let darkMode = PreferenceConstants.preference.darkMode
        
        let symbolColor = !darkMode ? TennColors.symbolColorWhite: TennColors.symbolColorDark
        let stringColor = !darkMode ? TennColors.stringColorWhite: TennColors.stringColorDark
        let numberColor = !darkMode ? TennColors.numberColorWhite: TennColors.numberColorDark
        let expressionColor = !darkMode ? TennColors.expressionColorWhite: TennColors.expressionColorDark
        
        var result = NSMutableAttributedString()
        
        if self.kind == .Command {
            makeSeq(&result, pattern: TennNode.spaces, count: indent)
        }
        let defaultStyle = [NSAttributedString.Key.foregroundColor : textColor ,NSAttributedString.Key.font: font]
        if let tok = self.token {
            switch self.kind {
            case .CharLit:
                result.append(NSAttributedString(string:tok.literal, attributes: defaultStyle))
            case .IntLit, .FloatLit:
                result.append(NSAttributedString(string:tok.literal, attributes: [NSAttributedString.Key.foregroundColor : numberColor,NSAttributedString.Key.font: font]))
            case .Ident:
                result.append(NSAttributedString(string:tok.literal, attributes: [NSAttributedString.Key.foregroundColor : symbolColor, NSAttributedString.Key.font: font]))
            case .StringLit:
                if clean {
                    result.append(NSAttributedString(string:tok.literal, attributes: [NSAttributedString.Key.foregroundColor : stringColor,NSAttributedString.Key.font: font]))
                }
                else {
                    result.append(NSAttributedString(string:"\"\(quote(tok.literal))\"", attributes: [NSAttributedString.Key.foregroundColor : stringColor,NSAttributedString.Key.font: font]))
                }
            case .Expression:
                result.append(NSAttributedString(string:"$(\(tok.literal))", attributes: [NSAttributedString.Key.foregroundColor : expressionColor,NSAttributedString.Key.font: font]))
            case .Image:
                result.append(NSAttributedString(string:" ", attributes: defaultStyle))
                let image1Attachment = NSTextAttachment()
                if let dta = Data(base64Encoded: tok.literal, options: .ignoreUnknownCharacters) {
                    let img = NSImage(data: dta)
                    image1Attachment.image = img
                                        
                    if let sz = img?.size {
                        let bb = getMaxRect(maxWidth: 100, maxHeight: 100, imageWidth: sz.width, imageHeight: sz.height)
                        image1Attachment.bounds = CGRect(x: 0, y: font.capHeight/2 + -1 * bb.height / 2, width: bb.width, height: bb.height)
                    }
                }
                result.append(NSAttributedString(attachment: image1Attachment))
                result.append(NSAttributedString(string:" ", attributes: defaultStyle))
            case .ExpressionBlock:
                result.append(NSAttributedString(string:"${", attributes: defaultStyle))
                result.append(NSAttributedString(string:"\(tok.literal)", attributes: [NSAttributedString.Key.foregroundColor : expressionColor, NSAttributedString.Key.font: font]))
                result.append(NSAttributedString(string:"}", attributes: defaultStyle))
            case .MarkdownLit:
                result.append(NSAttributedString(string:"%{\(tok.literal)}", attributes: [NSAttributedString.Key.foregroundColor : stringColor, NSAttributedString.Key.font: font]))
            default:
                break
            }
        }
        var ind = indent
        var postfix: String? = nil
        if self.kind == .BlockExpr {
            result.append(NSAttributedString(string:"{\n"))
            if self.count > 0 {
                postfix = "\n\(getSeq(pattern: TennNode.spaces, count: indent))}"
            }
            else {
                postfix = "\(getSeq(pattern: TennNode.spaces, count: indent))}"
            }
            ind += 1
        }
        childsToStr(font, textColor, &result, ind, clean)
        if let p = postfix {
            result.append(NSAttributedString(string:p))
        }
        
        return result
    }
}
