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
    
    var properties: [TennNode] = [] // Extra nodes not supported directly by model.
    
    
    // Item on self diagram of this element
    var selfItem: DiagramItem?
    
    
    // Transient data values
    
    var ox: Double = 0
    var oy: Double = 0
    
    init( name: String = "", createSelf: Bool = false) {
        self.id = UUID()
        self.name = name
        self.kind = .Element
        
        if createSelf {
            self.createSelfItem()
        }
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
    
    func createSelfItem() {
        let item = DiagramItem(kind: .Item, name:self.name)
        item.setData(.RefElement, self)
        self.selfItem = item
        self.items.append(item)
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
        if let index = at {
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
        item.setData(.RefElement, el)
        self.items.append(item)
        assignModel(item)
        
        // We also need to add link from self to this one
        if createLink && self.selfItem != nil {
            let link = DiagramItem(kind: .Link, name:"")
            link.setData(.RefElement, LinkElementData(source: self.selfItem!, target: item))
            self.items.append(link)
            assignModel(link)
        }
        return item
    }
    
    // Add a child element to current diagram
    fileprivate func add( source: DiagramItem, target: DiagramItem ) {
        // We also need to add link from self to this one
        let link = DiagramItem(kind: .Link, name:"")
        link.setData(.LinkData, LinkElementData(source: source, target: target))
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
    
    func getItem( _ el: Element) -> DiagramItem? {
        // find a item for source
        for i in self.items {
            if i.kind == .Item && i.getData(.RefElement) == el {
                return i
            }
        }
        return nil
    }
    
    /// Create a link between two elements
    /// source should be already on current diagram
    /// Target item will be added
    func add( source: Element, target: Element ) -> DiagramItem?  {
        if let sourceItem = getItem( source ) {
            // Update current diagram to have a link between a self and new added item.
            var item = getItem(target)
            if item == nil {
                item = DiagramItem(kind: .Item, name:target.name)
                item!.setData(.RefElement, target)
                self.items.append(item!)
                self.assignModel(item!)
            }
            // We also need to add link from self to this one
            
            let link = DiagramItem(kind: .Link, name:"")
            link.setData(.LinkData, LinkElementData(source: sourceItem, target: item!))
            self.items.append(link)
            assignModel(link)
            
            return item
        }
        return nil
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
            if itm.kind == .Link, let lData: LinkElementData = itm.getData(.LinkData) {
                if lData.source.id == item.id || lData.target.id == item.id {
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
            if $0.kind == .Link, let lData: LinkElementData = $0.getData(.LinkData) {
                if lData.source.id == item.id || lData.target.id == item.id {
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

enum ElementDataKind {
    case Visible
    case RefElement
    case LinkData
}
public class LinkElementData {
    var source: DiagramItem // A direct link to source
    var target: DiagramItem // A direct link to target in case of Link
    
    init( source:DiagramItem, target:DiagramItem) {
        self.source = source
        self.target = target
    }
}

public class DiagramItem {
    var kind: ItemKind
    var name: String
    var id: UUID
    var parent: Element?
    
    var description: String? = nil
    
    var properties: [TennNode] = [] // Extra nodes not supported directly by model.
    
    var itemData: [ElementDataKind: Any] = [:]
    
    var x: CGFloat = 0
    var y: CGFloat = 0
    
    init(kind: ItemKind, name: String) {
        self.id = UUID()
        self.kind = kind
        self.name = name
    }
    
    func setData<T>(_ kind: ElementDataKind, _ value: T) {
        itemData[kind] = value
    }
    func getData<T>(_ kind: ElementDataKind) -> T? {
        if let value = itemData[kind] as? T {
            return value
        }
        return nil
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
        
        pl.selfItem?.x = 0
        pl.selfItem?.y = 0
    }
}
