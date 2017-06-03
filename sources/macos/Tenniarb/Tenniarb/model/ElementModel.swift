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
            
        let dt = pl.addGet( Element(name: "DeviceTracker"))
            dt.add( Element(name: "Device"))
            
            let repo = pl.addGet( Element(name: "Repository"))
            repo.add(Element(name:"Database"))
            
            pl.add( Element(name: "Index"))
            pl.add( Element(name: "StateTracker"))
        
    }
}
