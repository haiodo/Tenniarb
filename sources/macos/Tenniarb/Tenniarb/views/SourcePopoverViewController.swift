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
    
    fileprivate func updateContent() {
        // Update source content.
        if let active = self.element {
            let txtValue = active.toTennStr(includeSubElements: false, includeItems: true)
            
//            let encoder = JSONEncoder()
//            encoder.outputFormatting = .prettyPrinted
            
//            let syncModel = active.toSync()
//            let data = try! encoder.encode(syncModel)
//            let txtValue = String( data: data, encoding: .utf8)!

            textViewer.textStorage?.setAttributedString(NSAttributedString(string: txtValue))
        }
    }
    
    override func viewDidLoad() {
        updateContent()
    }
    @IBAction func applyClose(_ sender: NSButton) {
        dismissViewController(self)
    }
    
    public func setElement(element: Element) {
        self.element = element
        self.updateContent()
    }
}
