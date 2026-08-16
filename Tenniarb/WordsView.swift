//
//  WordsView.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 18/06/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Cocoa
import Foundation

class WordsView: NSOutlineView {
    override var isOpaque: Bool {
        get {
            return false
        }
    }

    //    override func drawBackground(inClipRect clipRect: NSRect) {
    //        // No code here
    //        clipRect.fill(using: .clear)
    //        // NSRectFillUsingOperation(clipRect, NSCompositingOperation.clear);
    //    }
    override func drawCell(_ cell: NSCell) {
        Swift.debugPrint("Draw cell:")
    }
}
