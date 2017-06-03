//
//  Document.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 26/05/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Cocoa

class Document: NSDocument {
    var elementModel: ElementModel?
    
    var vc: ViewController?
    
    override init() {
        super.init()
        // Add your subclass-specific initialization here.
        
        // Do any additional setup after loading the view.
        self.elementModel = ElementModelFactory().elementModel
    }

    override class func autosavesInPlace() -> Bool {
        return true
    }

    override func makeWindowControllers() {
        // Returns the Storyboard that contains your Document window.
        let storyboard = NSStoryboard(name: "Main", bundle: nil)
        let windowController = storyboard.instantiateController(withIdentifier: "Document Window Controller") as! NSWindowController
        windowController.window?.acceptsMouseMovedEvents = true
        self.addWindowController(windowController)
        
        
        vc = windowController.contentViewController as? ViewController
        
        
        vc?.setElementModel(elementModel: self.elementModel!)
        
    }
}
