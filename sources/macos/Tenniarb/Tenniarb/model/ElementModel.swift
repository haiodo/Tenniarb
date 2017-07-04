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
    
    public var onUpdate: [() -> Void] = []
    
    init() {
        super.init(name: "Root", addSelfItem: false)
    }
    override func updateModel( _ el: Element) {
        el.model = self
    }
    override func updateModel( _ item: DiagramItem) {
        item.model = self
    }
    
    func modified() {
        for op in onUpdate {
            op()
        }
    }
}

public enum ElementKind {
    case Element // A reference to element
    case Link // A link
    case Annontation // Annotation box
    
    var commandName : String {
        switch self {
        // Use Internationalization, as appropriate.
        case .Element: return "element";
        case .Link: return "link";
        case .Annontation: return "annotation";
        }
    }
}

public class DiagramItem: Hashable {
    var kind: ElementKind
    var name: String?
    
    // id is transient and used only for identity
    var id: NSUUID
    var visible: Bool = true // Some items could be hided
    
    var x: CGFloat = 0 {
        didSet {
            self.model?.modified()
        }
    }
    var y: CGFloat = 0 {
        didSet {
            self.model?.modified()
        }
    }
    
    var element: Element? = nil
    var source: DiagramItem? = nil // A direct link to source
    var target: DiagramItem? = nil // A direct link to target in case of Link
    
    var model: ElementModel?
    
    public init(kind: ElementKind, name: String? = nil, element: Element? = nil, source: DiagramItem? = nil, target:DiagramItem? = nil) {
        self.kind = kind
        
        self.id = NSUUID()
        
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
    public static func ==(lhs: DiagramItem, rhs: DiagramItem) -> Bool {
        return lhs.id == rhs.id
    }
}

public class Element: Hashable {
    var id: NSUUID
    var name: String
    var elements: [Element] = []
    
    var backItems: [DiagramItem] = []
    
    // Elements drawn on diagram
    var items: [DiagramItem] = []
    
    var parent: Element? = nil
    
    var selfItem: DiagramItem? = nil
    
    var model: ElementModel?
    
    public var hashValue: Int {
        get {
            return id.hashValue
        }
    }
    
    public static func ==(lhs: Element, rhs: Element) -> Bool {
        return lhs.id == rhs.id
    }

    init(name: String, addSelfItem: Bool = true) {
        self.id = NSUUID()
        self.name = name
        
        // Add self as item
        
        if addSelfItem {
            // Since it is .Element name will be taken from Element itself.
            let si = DiagramItem(kind: .Element, name: nil, element: self)
            
            self.updateModel(si)
            self.selfItem = si
            self.backItems.append(si)
            self.items.append(si)
        }
    }
    
    func updateModel( _ el: Element) {
        el.model = self.model
    }
    
    func updateModel( _ item: DiagramItem) {
        item.model = self.model
    }
    
    // Operations with containment elements
    func add( _ el: Element ) {
        el.parent = self
        
        self.updateModel(el)
        self.elements.append(el)
        
        // We also automatically need to add a diagramElement's
        
        // Since it is .Element name will be taken from Element itself.
        let de = DiagramItem(kind: .Element, name: nil, element: el )
        self.updateModel(de)
        el.backItems.append(de)
        self.items.append(de)
        
        // We also need to add link from self to this item
        
        let li = DiagramItem(kind: .Link, name: nil, source: selfItem, target: de)
        self.updateModel(li)
        self.items.append(li)
        
        self.model?.modified()
    }
    
    func add( from source: DiagramItem, to element: Element ) {
        // Since it is .Element name will be taken from Element itself.
        
        let de = DiagramItem(kind: .Element, name: nil, element: element )
        self.updateModel(de)
        element.backItems.append(de)
        self.items.append(de)
        
        // We also need to add link from self to this item
        
        let li = DiagramItem(kind: .Link, name: nil, source: source, target: de)
        self.updateModel(li)
        self.items.append(li)
        
        self.model?.modified()
    }
    
    func add(get: Element) -> Element {
        self.add(get)
        return get
    }
    
    func add(item: Element) -> DiagramItem {
        return self.add(get: item).backItems[0]
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
        set(newy) {
            for de in backItems {
                de.y = newy
            }
        }
        get {
            return 0
        }
    }
    
    // Operations with
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
            
            if e.items.count > 0 {
                let elsRoot = TennNode.newCommand(".items")
                parent.add(elsRoot)
                
                let elsBlock = TennNode.newBlockExpr()
                elsRoot.add(elsBlock)
                
                for ie in e.items {
                    let enode = TennNode.newCommand("." + ie.kind.commandName)
                    if let n = ie.name {
                        enode.addAll(TennNode.newIdent("-name"), TennNode.newStrNode(n))
                    }
                    
                    enode.addAll(TennNode.newIdent("-x"), TennNode.newFloatNode(Double(ie.x)))
                    enode.addAll(TennNode.newIdent("-y"), TennNode.newFloatNode(Double(ie.y)))
                    
                    // Store node parameters
                    elsBlock.add(enode)
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
