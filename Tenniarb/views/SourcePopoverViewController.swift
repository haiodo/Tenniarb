//
//  SourcePopoverViewController.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 01/01/2018.
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

class SourcePopoverViewController: NSViewController {
    var element: Element?
    
    @IBOutlet var textViewer: NSTextView!
    
    fileprivate func updateContent() {
        // Update source content.
        if let active = self.element {
            let txtValue = active.toTennStr(includeSubElements: false, includeItems: true)
            
            textViewer.textStorage?.setAttributedString(NSAttributedString(string: txtValue))
        }
    }
    
    override func viewDidLoad() {
        updateContent()
    }
    @IBAction func applyClose(_ sender: NSButton) {
        dismiss(self)
    }
    
    public func setElement(element: Element) {
        self.element = element
        self.updateContent()
    }
}
