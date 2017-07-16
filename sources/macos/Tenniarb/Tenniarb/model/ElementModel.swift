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
public class ElementModel: Element {
    
    public var onUpdate: [(_ item:Element) -> Void] = []
    
    init() {
        super.init(kind: .Root, name: "Root")
    }
    override func updateModel( _ el: Element) {
        el.model = self
    }
    
    func modified(_ el: Element) {
        for op in onUpdate {
            op(el)
        }
    }
}

public enum ElementKind {
    case Element // A reference to element
    case Diagram // A reference to element
    case Root // A reference to element
    case Link // A link
    case Annontation // Annotation box
    
    var commandName : String {
        switch self {
        // Use Internationalization, as appropriate.
        case .Root: return "_root_";
        case .Element: return "element";
        case .Diagram: return "diagram";
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
    var kind: ElementKind
    var data: ElementData
    var name: String?
    var id: NSUUID
    
    init(kind: ElementKind, data: ElementData) {
        self.id = NSUUID()
        self.kind = kind
        self.data = data
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
    var id: NSUUID
    var name: String
    var elements: [Element] = []
    
    var backReferences: [Element] = []
    var parent: Element? = nil
    
    var model: ElementModel?
    
    // A list of items on diagram
    var items: [DiagramItem] = []
    
    
    // Item on self diagram of this element
    var selfItem: DiagramItem?
    
    init( kind: ElementKind, name: String) {
        self.id = NSUUID()
        self.name = name
        self.kind = kind
        
        self.selfItem = DiagramItem(kind: .Element, data: ElementData(refElement: self))
        self.items.append(self.selfItem!)
    }
    
    convenience init( name: String) {
        self.init( kind: .Element, name: name)
    }
    
    func updateModel( _ el: Element) {
        el.model = self.model
    }
    
    /// Add a diagram item to current diagram
    func add( _ item: DiagramItem) {
        self.items.append(item)
    
        self.model?.modified(self)
    }
    
    // Add a child element to current diagram
    func add( _ el: Element, createLink: Bool = true ) -> DiagramItem {
        el.parent = self
        
        self.updateModel(el)
        self.elements.append(el)
        
        // Update current diagram to have a link between a self and new added item.
        let item = DiagramItem(kind: .Element, data: ElementData(refElement: el))
        self.items.append(item)
        
        // We also need to add link from self to this one
        if createLink {
            let link = DiagramItem(kind: .Link, data: LinkElementData(source: self.selfItem!, target: item))
            self.items.append(link)
        }
        
        self.model?.modified(self)
        return item
    }
    
    func getItem( _ el: Element) -> DiagramItem? {
        // find a item for source
        for i in self.items {
            if i.kind == .Element && i.data.refElement == el {
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
            let item = DiagramItem(kind: .Element, data: ElementData(refElement: target))
            self.items.append(item)
            // We also need to add link from self to this one
            
            let link = DiagramItem(kind: .Link, data: LinkElementData(source: sourceItem, target: item))
            self.items.append(link)
            
            self.model?.modified(self)
            return item
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

/**
 Allow Mapping of element model to tenn and wise verse.
 */
extension Element {
    public func toTenn( ) -> TennNode {
        let result = TennNode(kind: TennNodeKind.Statements)
        
        var lvl = 0
        
        var p = self
        while p.parent != nil {
            p = p.parent!
            lvl += 1
        }
        
        elementsToTenn( result, [self], level: lvl )
        
        return result
    }
    
    func toTennStr( ) -> String {
        let ee = toTenn()
        return ee.toStr(0, false)
    }
    func elementsToTenn( _ parent: TennNode, _ elements: [Element], level: Int ) {
        for e in elements {
            let enode = TennNode.newHashNode(e.name, level: level)
            
            parent.add(enode)
            
            let elsRoot = TennNode.newCommand(".data")
            parent.add(elsRoot)
            
            let elsBlock = TennNode.newBlockExpr()
            elsRoot.add(elsBlock)
            
            for item in self.items {
                let itemRoot = TennNode.newCommand(".item")
                elsBlock.add(itemRoot)
                
                let itemBlock = TennNode.newBlockExpr()
                itemRoot.add(itemBlock)
                itemBlock.add( TennNode.newCommand("kind", TennNode.newIdent(item.kind.commandName)))
                itemBlock.add( TennNode.newCommand("pos", TennNode.newCommand("-x"), TennNode.newFloatNode(Double(item.data.x)), TennNode.newCommand("-y"), TennNode.newFloatNode(Double(item.data.y)) ))
                
                if let linkData = item.data as? LinkElementData {
                    if let sourceName = linkData.source.name {
                        elsBlock.add( TennNode.newCommand("source", TennNode.newIdent(sourceName)))
                    }
                    if let targetName = linkData.target.name {
                        elsBlock.add( TennNode.newCommand("target", TennNode.newIdent(targetName)))
                    }
                }
            }
            if e.elements.count > 0 {
                elementsToTenn(parent, e.elements, level: level + 1)
            }
        }
    }
}

public class ElementModelFactory {
    var elementModel: ElementModel
    public init() {
        self.elementModel = ElementModel()
        
        let pl = Element(name: "platform")
        _ = self.elementModel.add(pl)
        
        let index = pl.add( Element(name: "Index"))
            index.x = 50
            index.y = 50
        
        let st = pl.add( Element(name: "StateTracker"))
            st.x = -50
            st.y = -50
        
        let dt = Element(name: "DeviceTracker")
        let dte = pl.add( dt )
            dte.x = 100
            dte.y = -50
    
        let dev = dt.add( Element(name: "Device"))
            dev.x = -50
            dev.y = 50
        
        let repo = Element(name: "Repository")
        let repoe = pl.add( repo )
            repoe.x = 50
            repoe.y = -100
        
        let db = repo.add( Element(name: "Database"))
            db.x = 40
            db.y = 50
    }
}
