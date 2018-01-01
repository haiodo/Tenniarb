//
//  ElementModel.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 29/05/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Foundation


public enum UpdateEventKind {
    case Structure
    case Layout
}

/**
 A root of element map
 */
public class ElementModel: Element {
    public var onUpdate: [(_ item:Element, _ kind: UpdateEventKind) -> Void] = []
    
    public var modelName: String = ""
    public var modified: Bool = false
    
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
    
    func makeNonModified() {
        modified = false
    }
    
    func modified(_ el: Element, _ kind: UpdateEventKind ) {
        modified = true
        for op in onUpdate {
            op(el, kind)
        }
    }
}

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

public class ElementData {
    var refElement: Element? = nil // A reference to some other element
    var visible: Bool = true // Some items could be hided
    init( ) {
    }
    
    convenience init( refElement: Element) {
        self.init()
        self.refElement = refElement
    }
    
    var x: CGFloat = 0
    var y: CGFloat = 0
}
public class LinkElementData: ElementData {
    var source: DiagramItem // A direct link to source
    var target: DiagramItem // A direct link to target in case of Link
    
    init( source:DiagramItem, target:DiagramItem) {
        self.source = source
        self.target = target
        super.init()
    }
}

public class DiagramItem {
    var kind: ItemKind
    var data: ElementData
    var name: String
    var id: UUID
    var parent: Element?
    
    var description: String? = nil
    
    var properties: [TennNode] = [] // Extra nodes not supported directly by model.
    
    init(kind: ItemKind, name: String, data: ElementData) {
        self.id = UUID()
        self.kind = kind
        self.name = name
        self.data = data
    }
    convenience init( kind: ItemKind, name: String) {
        self.init(kind: kind, name: name, data: ElementData())
    }
    var x: CGFloat {
        get {
            return data.x
        }
        set {
            data.x = newValue
        }
    }
    var y: CGFloat {
        get {
            return data.y
        }
        set {
            data.y = newValue
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
        let item = DiagramItem(kind: .Item, name:self.name, data: ElementData(refElement: self))
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
    func add( _ item: DiagramItem) {
        self.items.append(item)
        
        assignModel(item)
        self.model?.modified(self, .Structure)
    }
    
    /// Add a diagram item to current diagram
    func add( get item: DiagramItem) -> DiagramItem {
        self.add(item)
        return item
    }
    
    func add( _ el: Element ) {
        el.parent = self
        
        self.assignModel(el)
        self.elements.append(el)
        
        self.model?.modified(self, .Structure)
    }
    
    // Add a child element to current diagram
    func add( makeItem el: Element, createLink: Bool = true ) -> DiagramItem {
        self.add(el)
        // Update current diagram to have a link between a self and new added item.
        let item = DiagramItem(kind: .Item, name: el.name, data: ElementData(refElement: el))
        self.items.append(item)
        assignModel(item)
        
        // We also need to add link from self to this one
        if createLink && self.selfItem != nil {
            let link = DiagramItem(kind: .Link, name:"", data: LinkElementData(source: self.selfItem!, target: item))
            self.items.append(link)
            assignModel(link)
        }
        
        self.model?.modified(self, .Structure)
        return item
    }
    
    // Add a child element to current diagram
    func add( source: DiagramItem, target: DiagramItem ) {
        // We also need to add link from self to this one        
        let link = DiagramItem(kind: .Link, name:"", data: LinkElementData(source: source, target: target))
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
        
        self.model?.modified(self, .Structure)
    }
    
    func getItem( _ el: Element) -> DiagramItem? {
        // find a item for source
        for i in self.items {
            if i.kind == .Item && i.data.refElement == el {
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
                item = DiagramItem(kind: .Item, name:target.name, data: ElementData(refElement: target))
                self.items.append(item!)
                self.assignModel(item!)
            }
            // We also need to add link from self to this one
            
            let link = DiagramItem(kind: .Link, name:"", data: LinkElementData(source: sourceItem, target: item!))
            self.items.append(link)
            assignModel(link)
            
            self.model?.modified(self, .Structure)
            return item
        }
        return nil
    }
    func remove(_ element: Element) {
        self.elements = self.elements.filter {$0.id != element.id }
        self.model?.modified(self, .Structure)
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

extension DiagramItem {
    /// Convert items to list of properties
    func toTennProps() -> String {
        let items = TennNode.newNode(kind: .Statements)
        
        items.add(TennNode.newCommand("name", TennNode.newStrNode(self.name ?? "")))
        
        if self.kind == .Item {
            items.add(TennNode.newCommand("pos", TennNode.newFloatNode(Double(self.data.x)), TennNode.newFloatNode(Double(self.data.y))))
        }
        
        return items.toStr()
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
