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
//        let frame = NSMakeRect(100, 100, 500, 300)
//        let window = NSWindow(contentRect: frame ,
//                              styleMask: [.closable, .resizable, .unifiedTitleAndToolbar, .titled] ,
//                              backing: .buffered,
//                              defer: false)
//        let wc: NSWindowController = MasterWindowController.init(window: window)
//        self.addWindowController(wc)
        // Returns the Storyboard that contains your Document window.
        let storyboard = NSStoryboard(name: "Main", bundle: nil)
        let windowController = storyboard.instantiateController(withIdentifier: "Document Window Controller") as! NSWindowController
        windowController.window?.acceptsMouseMovedEvents = true
        self.addWindowController(windowController)
        
        
        vc = windowController.contentViewController as? ViewController
        vc?.view.window?.delegate = self
        
        vc?.setElementModel(elementStore: self.store!)
        
        if let uri = self.fileURL?.absoluteString, let window = self.vc?.view.window,
            let data = PreferenceConstants.preference.defaults.string(forKey: windowPositionOption + uri)  {
            let p = TennParser()
            let node = p.parse(data)
            if( !p.errors.hasErrors()) {
                if let pos = node.getChild(0) {
                    let frame = CGRect(x: CGFloat(pos.getFloat(1) ?? 0), y: CGFloat(pos.getFloat(2) ?? 0), width: CGFloat(pos.getFloat(3) ?? 0), height: CGFloat(pos.getFloat(4) ?? 0))
                    window.setFrame(frame, display: true)
                }
            }
        }
    }
    
    func saveWindowPosition() {
        if let frame = vc?.view.window?.frame, let uri = self.fileURL?.absoluteString {
            let nde = TennNode.newCommand("pos",
                                          TennNode.newFloatNode( Double(frame.origin.x)),
                                          TennNode.newFloatNode( Double(frame.origin.y)),
                                          TennNode.newFloatNode( Double(frame.size.width)),
                                          TennNode.newFloatNode( Double(frame.size.height))
            )
            PreferenceConstants.preference.defaults.set(nde.toStr(), forKey: windowPositionOption + uri)
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
            
            let now = Date()
            
            let parser = TennParser()
            let node = parser.parse(storedValue)
            
            if parser.errors.hasErrors() {
                return
            }
            
            let elementModel = ElementModel.parseTenn(node: node)
            
            Swift.debugPrint("Elapsed parse \(Date().timeIntervalSince(now))")
            
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
