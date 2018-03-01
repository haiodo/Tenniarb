//
//  Document.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 26/05/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Cocoa

class Document: NSDocument {
    var store: ElementModelStore?
    
    var vc: ViewController?
    
    
    override init() {
        super.init()
        
        // By default create with sample scene.
        let elementModel = ElementModelFactory().elementModel
        elementModel.modelName = "Unnamed"
        
        updateStore(elementModel)
    }
    
    fileprivate func updateStore(_ elementModel: ElementModel) {
        if let oldStore = store {
            oldStore.model.onUpdate.removeAll()
        }
        elementModel.onUpdate.append( onUpdate )
        self.store = ElementModelStore(elementModel)
    }

    func onUpdate(element: Element, updateEvent: UpdateEventKind) {
        updateChangeCount(.changeDone)
    }
    
    override func makeWindowControllers() {
        // Returns the Storyboard that contains your Document window.
        let storyboard = NSStoryboard(name: NSStoryboard.Name(rawValue: "Main"), bundle: nil)
        let windowController = storyboard.instantiateController(withIdentifier: NSStoryboard.SceneIdentifier(rawValue: "Document Window Controller")) as! NSWindowController
        windowController.window?.acceptsMouseMovedEvents = true
        self.addWindowController(windowController)
        
        
        vc = windowController.contentViewController as? ViewController
        vc?.setElementModel(elementStore: self.store!)
    }
    
    override func read(from url: URL, ofType typeName: String) throws {
        
        do {
            let storedValue = try String(contentsOf: url, encoding: String.Encoding.utf8)
            
            let parser = TennParser()
            let node = parser.parse(storedValue)
            
            if parser.errors.hasErrors() {
                return
            }
            
            let elementModel = ElementModel.parseTenn(node: node)
            elementModel.modelName = url.lastPathComponent
            
            vc?.setElementModel(elementStore: self.store!)
            
            self.updateStore(elementModel)
            self.fileURL = url
        }
        catch {
            Swift.print("Failed to load file")
        }
    }
    
    override var isDocumentEdited: Bool {
        get {
            return store?.model.modified ?? true
        }
    }
    
    override func write(to url: URL, ofType typeName: String) throws {
        do {
            if let es = self.store {
                let value = es.model.toTennStr()
                try value.write(to: url, atomically: true, encoding: String.Encoding.utf8)
                es.model.modified = false
                
                es.model.modelName = url.lastPathComponent
                updateChangeCount(.changeCleared)
                vc?.updateWindowTitle()
                self.fileURL = url
            }
        }
        catch {
            Swift.print("Some error happen")
        }
    }
}
