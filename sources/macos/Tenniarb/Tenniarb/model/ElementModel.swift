//
//  ElementModel.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 29/05/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Foundation


/**
 A root of element map
 */
class ElementModel: Element {
    init() {
        super.init(id: NSUUID().uuidString, name: "Root")
    }
}

enum ElementKind {
    case Element // A reference to element
    case Link // A link
    case Annontation // Annotation box
}

class DiagramItem {
    var kind: ElementKind
    var name: String?
    var id: String
    var visible: Bool = true // Some items could be hided
    
    init(kind: ElementKind, id: String? = nil, name: String? = nil) {
        self.kind = kind
        
        if id == nil {
            self.id = NSUUID().uuidString
        }
        else {
            self.id = id!
        }
        
        self.name = name
    }
}

class Element {
    var id: String
    var name: String
    var elements: [Element]
    
    // Elements drawn on diagram
    var items: [DiagramItem]
    
    var x: Int = 0
    var y: Int = 0

    convenience init(name: String) {
        self.init(id: NSUUID().uuidString, name: name)
    }

    init(id: String, name: String) {
        self.id = id
        self.name = name
        self.elements = []
        self.items = []
    }
    
    // Operations with containment elements
    func add( _ el: Element ) {
        self.elements.append(el)
    }

    func remove( id: String ) {
        var i = -1
        for (j, x) in elements.enumerated() {
            if x.id == id {
                i = j
                break
            }
        }
        if (i != -1) {
            elements.remove(at: i)
        }
    }

    func add(get: Element) -> Element {
        self.add(get)
        return get
    }
    
    // Operations with
}

public class ElementModelFactory {
    var elementModel: ElementModel
    public init() {
        self.elementModel = ElementModel()
            
            
        let pl = elementModel.add(get: Element(name: "Platform"))
        pl.x = 0
        pl.y = 0
        
        let index = pl.add( get: Element(name: "Index"))
            index.x = 50
            index.y = 50
        let st = pl.add( get: Element(name: "StateTracker"))
            st.x = -50
            st.y = -50
            
        let dt = pl.add( get: Element(name: "DeviceTracker"))
            dt.x = 50
            dt.y = -50
        
        let dev = dt.add( get: Element(name: "Device"))
            dev.x = -50
            dev.y = 50
            
        let repo = pl.add( get: Element(name: "Repository"))
            repo.x = 50
            repo.y = 00
        let db = repo.add( get: Element(name: "Database"))
            db.x = 40
            db.y = 50
            
    

    }
}
