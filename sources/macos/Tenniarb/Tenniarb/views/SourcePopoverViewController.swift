//
//  SourcePopoverViewController.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 01/01/2018.
//  Copyright © 2018 Andrey Sobolev. All rights reserved.
//

import Foundation
import Cocoa

class SourcePopoverViewController: NSViewController {
    var element: Element?
    
    @IBOutlet var textViewer: NSTextView!
    
    override func viewDidLoad() {
        // Update source content.
        if let active = self.element {
            let txtValue = active.toTennStr(includeSubElements: false, includeItems: true)
            textViewer.textStorage?.append(NSAttributedString(string: txtValue))
        }
    }
    @IBAction func applyClose(_ sender: NSButton) {
        dismissViewController(self)
    }
    
    public func setElement(element: Element) {
        self.element = element
    }
}
