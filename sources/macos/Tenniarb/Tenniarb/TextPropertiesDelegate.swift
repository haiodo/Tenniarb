//
//  TextPropertiesDelegate.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 15/01/2018.
//  Copyright © 2018 Andrey Sobolev. All rights reserved.
//

import Foundation
import Cocoa

class TextPropertiesDelegate: NSObject, NSTextViewDelegate {
    var controller: ViewController
    var view: NSTextView
    public init(_ controller: ViewController, _ textView: NSTextView ) {
        self.controller = controller
        self.view = textView
        super.init()
        
        self.view.delegate = self
        self.view.isContinuousSpellCheckingEnabled = false
        self.view.isAutomaticSpellingCorrectionEnabled = false
    }
    func setTextValue(_ value: String) {
        let style = NSMutableParagraphStyle()
        style.headIndent = 50
        style.alignment = .justified
        style.firstLineHeadIndent = 50
        
        self.view.font = NSFont.systemFont(ofSize: 15.0)
        self.view.string = value
        self.view.scrollToBeginningOfDocument(self)
//        self.view.delegate = self
    }
    func textDidChange(_ notification: Notification) {
        Swift.debugPrint("Text did change")
    }
    func textView(_ textView: NSTextView, doCommandBy commandSelector: Selector) -> Bool {
        Swift.debugPrint("Selector:" + commandSelector.description)
        return false
    }
    
    func textView(_ textView: NSTextView, completions words: [String], forPartialWordRange charRange: NSRange, indexOfSelectedItem index: UnsafeMutablePointer<Int>?) -> [String] {
        return ["Beta"]
    }
}
