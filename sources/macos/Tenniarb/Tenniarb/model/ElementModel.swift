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

class Element {
    var id: String
    var name: String
    var elements: [Element]
    var x: Int = 0
    var y: Int = 0

    convenience init(name: String) {
        self.init(id: NSUUID().uuidString, name: name)
    }

    init(id: String, name: String) {
        self.id = id
        self.name = name
        self.elements = []
    }

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

    func addGet(_ element: Element) -> Element {
        self.add(element)
        return element
    }
}

public class ElementModelFactory {
    var elementModel: ElementModel
    public init() {
        self.elementModel = ElementModel()
            
            
        let pl = elementModel.addGet(Element(name: "Platform"))
        pl.x = 50
        pl.y = 50
        
        let index = pl.addGet( Element(name: "Index"))
            index.x = 200
            index.y = 50
        let st = pl.addGet( Element(name: "StateTracker"))
            st.x = 200
            st.y = 100
            
        let dt = pl.addGet( Element(name: "DeviceTracker"))
            dt.x = 250
            dt.y = 100
        
        let dev = dt.addGet( Element(name: "Device"))
            dev.x = 250
            dev.y = 150
            
        let repo = pl.addGet( Element(name: "Repository"))
            repo.x = 250
            repo.y = 200
        let db = repo.addGet(Element(name: "Database"))
            db.x = 400
            db.y = 50
            
    

    }
}
