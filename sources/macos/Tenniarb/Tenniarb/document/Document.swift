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
        
        // By default create with sample scene.
        self.elementModel = ElementModelFactory().elementModel
        self.elementModel?.modelName = "Unnamed"
        
        self.elementModel?.onUpdate.append( onUpdate )
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
        vc?.setElementModel(elementModel: self.elementModel!)
    }
    
    override func read(from url: URL, ofType typeName: String) throws {
        
        do {
            let storedValue = try String(contentsOf: url, encoding: String.Encoding.utf8)
            
            let parser = TennParser()
            let node = parser.parse(storedValue)
            
            if parser.errors.hasErrors() {
                return
            }
            if let oldModel = elementModel {
                oldModel.onUpdate.removeAll()
            }
            elementModel = ElementModel.parseTenn(node: node)
            elementModel?.modelName = url.lastPathComponent
            vc?.setElementModel(elementModel: self.elementModel!)
            elementModel?.onUpdate.append( onUpdate )
        }
        catch {
            Swift.print("Failed to load file")
        }
    }
    
    override var isDocumentEdited: Bool {
        get {
            return elementModel?.modified ?? true
        }
    }
    
    override func write(to url: URL, ofType typeName: String) throws {
        do {
            if let em = elementModel {
                let value = em.toTennStr()
                try value.write(to: url, atomically: true, encoding: String.Encoding.utf8)
                em.modified = false
                
                em.modelName = url.lastPathComponent
                updateChangeCount(.changeCleared)
                vc?.updateWindowTitle()
            }
        }
        catch {
            Swift.print("Some error happen")
        }
    }
}
