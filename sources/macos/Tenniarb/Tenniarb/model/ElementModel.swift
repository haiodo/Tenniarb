//
//  ElementModel.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 29/05/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Foundation

public enum ElementKind {
    case Root
    case Element // A reference to element
    
    var commandName : String {
        switch self {
        // Use Internationalization, as appropriate.
        case .Root: return "model";
        case .Element: return "element";
        }
    }
}

public class ModelProperties: Sequence {
    public typealias Element = TennNode
    
    public typealias Iterator = Array<TennNode>.Iterator
    var node: TennNode = TennNode.newBlockExpr()
    
    init() {
    }
    init( _ props: [TennNode]) {
        node.add(props)
    }
    
    init( node: TennNode) {
        self.node = node
    }
    
    public func makeIterator() -> Iterator {
        if self.node.count == 0 {
            let nde: [TennNode] = []
            return nde.makeIterator()
        }
        return self.node.children!.makeIterator()
    }
    public func append( _ itm: TennNode) {
        self.node.add(itm)
    }
    public func append(contentsOf: [TennNode]) {
        for c in contentsOf {
            append(c)
        }
    }
    
    public func get(_ name: String) -> TennNode? {
        return self.node.getNamedElement(name)
    }
    
    var count:Int {
        get {
            return self.node.count
        }
    }
    public func clone() -> ModelProperties {
        let result = ModelProperties()
        result.node = self.node.clone()
        
        return result
    }
    public func asNode() -> TennNode {
        return self.node
    }
}

public class Element {
    var kind: ElementKind
    var id: UUID
    var name: String
    var elements: [Element] = []
    
    var parent: Element? = nil
    
    var model: ElementModel?
    
    // A list of items on diagram
    var items: [DiagramItem] = []
    
    var description: String? = nil
    
    var properties: ModelProperties = ModelProperties() // Extra nodes not supported directly by model.
    
    // Transient data values
    
    var ox: Double = 0
    var oy: Double = 0
    
    init( name: String = "", createSelf: Bool = false) {
        self.id = UUID()
        self.name = name
        self.kind = .Element
    }
    
    // Doing a full clone of this element with all childrens.
    func clone(cloneItems:Bool = true, cloneElement:Bool = true) -> Element {
        let cloneEl = Element(name: self.name)
        
        cloneEl.ox = self.ox
        cloneEl.oy = self.oy
        cloneEl.description = self.description
        
        cloneEl.properties.append(contentsOf: self.properties.map({(itm) in itm.clone()}))
        
        if cloneItems {
            var itemsMap: [DiagramItem: DiagramItem] = [:]
            
            var linksToProcess: [LinkItem] = []
            
            cloneEl.items.append(contentsOf: self.items.map({(itm) in
                let copy = itm.clone()
                copy.parent = cloneEl
                itemsMap[itm] = copy
                if let link =  copy as? LinkItem {
                    linksToProcess.append(link)
                }
                return copy
            }))
            
            // We need to process links with right items
            for link in linksToProcess {
                if let src = link.source {
                    link.source = itemsMap[src]
                }
                if let dst = link.target {
                    link.target = itemsMap[dst]
                }
            }
        }
        
        if cloneElement {
            cloneEl.elements.append(contentsOf: self.elements.map({
                (el) in
                let copy = el.clone(cloneItems: true, cloneElement: true)
                copy.parent = cloneEl
                return copy
            }))
        }
        
        return cloneEl
    }
    
    var count: Int {
        get {
            return elements.count
        }
    }
    
    var itemCount: Int {
        get {
            return items.count
        }
    }
    
    func assignModel( _ el: Element) {
        self.model?.assignModel(el)
        
        // Assign all childs a proper model
        for child in el.elements {
            assignModel(child)
        }
    }
    
    func assignModel( _ el: DiagramItem) {
        el.parent = self
    }
    
    /// Add a diagram item to current diagram
    func add( _ item: DiagramItem, at: Int? =  nil) {
        
        assignModel(item)
        
        if let index = at {
            self.items.insert(item, at: index)
        }
        else {
            self.items.append(item)
        }
    }
    
    /// Add a diagram item to current diagram
    func add( get item: DiagramItem) -> DiagramItem {
        self.add(item)
        return item
    }
    
    func add( _ el: Element, at: Int? = nil ) {
        el.parent = self
        
        self.assignModel(el)
        if let index = at, self.elements.count > index {
            self.elements.insert(el, at: index)
        }
        else {
            self.elements.append(el)
        }
    }
    
    // Add a child element to current diagram
    func add( makeItem el: Element, createLink: Bool = true ) -> DiagramItem {
        self.add(el)
        // Update current diagram to have a link between a self and new added item.
        let item = DiagramItem(kind: .Item, name: el.name)
        self.items.append(item)
        assignModel(item)
        
        return item
    }
    
    // Add a child element to current diagram
    func add( source: DiagramItem, target: DiagramItem ) {
        // We also need to add link from self to this one
        let link = LinkItem(kind: .Link, name:"", source: source, target: target)
        self.items.append(link)
        assignModel(link)
        
        if !self.items.contains(source) {
            self.items.append(source)
            assignModel(source)
        }
        if !self.items.contains(target) {
            self.items.append(target)
            assignModel(target)
        }
    }    
    func remove(_ element: Element) -> Int {
        if let index = self.elements.index(of: element) {
            self.elements.remove(at: index)
            return index
        }
        return -1
    }
    
    func findLinks(_ item: DiagramItem) -> [DiagramItem] {
        var result:[DiagramItem] = []
        for itm in self.items {
            // Need to check if item is Link and source or target is our client
            if itm.kind == .Link, let lData = itm as? LinkItem {
                if lData.source?.id == item.id || lData.target?.id == item.id {
                    result.append(itm)
                }
            }
        }
        return result
    }
    
    func remove(_ item: DiagramItem) -> Int {
        if let index = self.items.index(of: item) {
            self.items.remove(at: index)
            return index
        }
        return -1
    }
    
    func getRelatedItems(_ item: DiagramItem ) -> [DiagramItem] {
        return self.items.filter {
            // Need to check if item is Link and source or target is our client
            if $0.kind == .Link, let lData = $0 as? LinkItem {
                if lData.source?.id == item.id || lData.target?.id == item.id {
                    return true
                }
            }
            if $0.id == item.id {
                return true
            }
            return false
        }
    }
}

//public class Resource {
//    public var name: String
//    public var data: Data
//}

/**
 A root of element map
 */
public class ElementModel: Element {
    public var modelName: String = ""
    
    init() {
        super.init(name: "Root")
        self.kind = .Root
    }
    override func assignModel( _ el: Element) {
        el.model = self
        
        // Assign all childs a proper model
        for child in el.elements {
            assignModel(child)
        }
    }
}

public enum ItemKind {
    case Item // A reference to element
    case Link // A link
    case Annontation // Annotation box
    
    var commandName : String {
        switch self {
        // Use Internationalization, as appropriate.
        case .Item: return "item";
        case .Link: return "link";
        case .Annontation: return "annotation";
        }
    }
}

public class DiagramItem {
    var kind: ItemKind
    var name: String
    var id: UUID
    var parent: Element?
    
    var description: String? = nil
    
    var properties: ModelProperties = ModelProperties() // Extra nodes not supported directly by model.
    
    var x: CGFloat = 0
    var y: CGFloat = 0
    
    init(kind: ItemKind, name: String) {
        self.id = UUID()
        self.kind = kind
        self.name = name
    }
    
    func clone() -> DiagramItem {
        let cloneItm = DiagramItem(kind: self.kind, name: self.name)
        
        cloneItm.x = self.x
        cloneItm.y = self.y
        cloneItm.description = self.description
        
        cloneItm.properties.append(contentsOf: self.properties.map({ (nde) in nde.clone()}))
        
        return cloneItm
    }
}

class LinkItem: DiagramItem {
    var source: DiagramItem? // A direct link to source
    var target: DiagramItem? // A direct link to target in case of Link
    
    init( kind: ItemKind, name: String, source: DiagramItem?, target: DiagramItem? ) {
        self.source = source
        self.target = target
        super.init(kind: kind, name: name)
    }
    override func clone() -> LinkItem {
        let cloneItm = LinkItem(kind: self.kind, name: self.name, source: self.source, target: self.target)
        
        cloneItm.x = self.x
        cloneItm.y = self.y
        cloneItm.description = self.description
        
        cloneItm.properties.append(contentsOf: self.properties.map({ (nde) in nde.clone()}))
        
        return cloneItm
    }
    
}

extension Element: Hashable {
    public var hashValue: Int {
        get {
            return id.hashValue
        }
    }
    
    public static func ==(lhs: Element, rhs: Element) -> Bool {
        return lhs.id == rhs.id
    }
}

extension DiagramItem: Hashable {
    public var hashValue: Int {
        get {
            return id.hashValue
        }
    }
    
    public static func ==(lhs: DiagramItem, rhs: DiagramItem) -> Bool {
        return lhs.id == rhs.id
    }
}

/*
 Default element factory.
 */
public class ElementModelFactory {
    var elementModel: ElementModel
    public init() {
        self.elementModel = ElementModel()
        
        let pl = Element(name: "Unnamed diagram", createSelf: false)
        _ = self.elementModel.add(pl)        
    }
}
