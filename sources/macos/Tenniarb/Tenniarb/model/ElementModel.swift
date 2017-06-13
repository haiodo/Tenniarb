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
        super.init(name: "Root", addSelfItem: false)
    }
}

enum ElementKind {
    case Element // A reference to element
    case Link // A link
    case Annontation // Annotation box
}

class DiagramItem: Hashable {
    var kind: ElementKind
    var name: String?
    var id: String
    var visible: Bool = true // Some items could be hided
    
    var x: CGFloat = 0
    var y: CGFloat = 0
    
    var element: Element? = nil
    var source: DiagramItem? = nil // A direct link to source
    var target: DiagramItem? = nil // A direct link to target in case of Link
    
    init(kind: ElementKind, name: String? = nil, element: Element? = nil, source: DiagramItem? = nil, target:DiagramItem? = nil) {
        self.kind = kind
        
        self.id = NSUUID().uuidString
        
        self.name = name
        self.element = element
        self.source = source
        self.target = target
    }
    public var hashValue: Int {
        get {
            return id.hashValue
        }
    }
    static func ==(lhs: DiagramItem, rhs: DiagramItem) -> Bool {
        return lhs.id == rhs.id
    }
}

class Element: Hashable {
    var id: String
    var name: String
    var elements: [Element] = []
    
    var backItems: [DiagramItem] = []
    
    // Elements drawn on diagram
    var items: [DiagramItem] = []
    
    var parent: Element? = nil
    
    var selfItem: DiagramItem? = nil
    
    public var hashValue: Int {
        get {
            return id.hashValue
        }
    }
    
    static func ==(lhs: Element, rhs: Element) -> Bool {
        return lhs.id == rhs.id
    }

    init(name: String, addSelfItem: Bool = true) {
        self.id = NSUUID().uuidString
        self.name = name
        
        // Add self as item
        
        if addSelfItem {
            // Since it is .Element name will be taken from Element itself.
            let si = DiagramItem(kind: .Element, name: nil, element: self)
            self.selfItem = si
            self.backItems.append(si)
            self.items.append(si)
        }
    }
    
    // Operations with containment elements
    func add( _ el: Element ) {
        el.parent = self
        self.elements.append(el)
        
        // We also automatically need to add a diagramElement's
        
        // Since it is .Element name will be taken from Element itself.
        let de = DiagramItem(kind: .Element, name: nil, element: el )
        el.backItems.append(de)
        self.items.append(de)
        
        // We also need to add link from self to this item
        
        let li = DiagramItem(kind: .Link, name: nil, source: selfItem, target: de)
        self.items.append(li)
    }
    

    func add(get: Element) -> Element {
        self.add(get)
        return get
    }
    
    var x: CGFloat {
        set(newx) {
            for de in backItems {
                de.x = newx
            }
        }
        get {
            return 0
        }
    }
    var y: CGFloat {
        set(newx) {
            for de in backItems {
                de.x = newx
            }
        }
        get {
            return 0
        }
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
            dt.x = 100
            dt.y = -50
        
        let dev = dt.add( get: Element(name: "Device"))
            dev.x = -50
            dev.y = 50
            
        let repo = pl.add( get: Element(name: "Repository"))
            repo.x = 50
            repo.y = -100
        let db = repo.add( get: Element(name: "Database"))
            db.x = 40
            db.y = 50
            
    

    }
}
