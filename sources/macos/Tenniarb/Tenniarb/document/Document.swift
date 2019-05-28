//
//  Document.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 26/05/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Cocoa

class Document: NSDocument, IElementModelListener, NSWindowDelegate {
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
        self.store?.onUpdate.append( self )
        self.store = ElementModelStore(elementModel)
    }

    func notifyChanges(_ evt: ModelEvent ) {
        updateChangeCount(.changeDone)
        vc?.updateWindowTitle()
    }
    
    override func makeWindowControllers() {
        // Returns the Storyboard that contains your Document window.
        let storyboard = NSStoryboard(name: "Main", bundle: nil)
        let windowController = storyboard.instantiateController(withIdentifier: "Document Window Controller") as! NSWindowController
        windowController.window?.acceptsMouseMovedEvents = true
        self.addWindowController(windowController)
        
        
        vc = windowController.contentViewController as? ViewController
        vc?.view.window?.delegate = self
        
        vc?.setElementModel(elementStore: self.store!)
        
        if let uri = self.fileURL?.absoluteString, let window = self.vc?.view.window,
            let data = PreferenceConstants.preference.defaults.value(forKey: windowPositionOption + uri)  {
            if let dta = data as? Data, let frame = NSKeyedUnarchiver.unarchiveObject(with: dta) as? NSRect {
                window.setFrame(frame, display: true)
            }
        }
    }
    
    func saveWindowPosition() {
        if let frame = vc?.view.window?.frame, let uri = self.fileURL?.absoluteString {
            let value = NSKeyedArchiver.archivedData(withRootObject: frame)
            PreferenceConstants.preference.defaults.set(value, forKey: windowPositionOption + uri)
        }
    }
    func windowDidMove(_ notification: Notification) {
        saveWindowPosition()
    }
    func windowDidResize(_ notification: Notification) {
        saveWindowPosition()
    }
    func windowDidExpose(_ notification: Notification) {
        saveWindowPosition()
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
            return store?.modified ?? true
        }
    }
    
    override func write(to url: URL, ofType typeName: String) throws {
        do {
            if let es = self.store {
                let value = es.model.toTennStr()
                try value.write(to: url, atomically: true, encoding: String.Encoding.utf8)
                es.modified = false
                
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
