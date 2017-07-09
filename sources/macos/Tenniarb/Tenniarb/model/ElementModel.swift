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
        super.init(kind: .Root, name: "Root")
    }
    override func updateModel( _ el: Element) {
        el.model = self
    }
    
    func modified() {
        for op in onUpdate {
            op()
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
    
    var x: CGFloat = 0
    var y: CGFloat = 0
}
public class LinkElementData: ElementData {
    var source: Element // A direct link to source
    var target: Element // A direct link to target in case of Link
    
    init( source:Element, target:Element) {
        self.source = source
        self.target = target
        super.init()
    }
}

public class Element: Hashable {
    var kind: ElementKind
    var id: NSUUID
    var name: String
    var elements: [Element] = []
    
    var backReferences: [Element] = []
    var parent: Element? = nil
    var model: ElementModel?
    
    var data: ElementData?
    
    public var hashValue: Int {
        get {
            return id.hashValue
        }
    }
    
    var x: CGFloat {
        get {
            if let d = data {
                return d.x
            }
            return 0
        }
        set {
            if let d = data {
                d.x = newValue
            }
            self.model?.modified()
        }
    }
    var y: CGFloat {
        get {
            if let d = data {
                return d.y
            }
            return 0
        }
        set {
            if let d = data {
                d.y = newValue
            }
            self.model?.modified()
        }
    }
    
    public static func ==(lhs: Element, rhs: Element) -> Bool {
        return lhs.id == rhs.id
    }
    
    init( kind: ElementKind, name: String) {
        self.id = NSUUID()
        self.name = name
        self.kind = kind
        
        if kind == .Link {
            Swift.debugPrint("Invalid element kind for this constructor")
        }
        else {
            self.data = ElementData()
        }
    }
    
    convenience init( name: String) {
        self.init( kind: .Element, name: name)
    }
    
    convenience init( name: String, source: Element, target: Element ) {
        self.init(kind: .Link, name: name)
        self.data = LinkElementData(source: source, target: target)
    }
    
    func updateModel( _ el: Element) {
        el.model = self.model
    }
    
    // Operations with containment elements
    func add( _ el: Element ) {
        el.parent = self
        
        self.updateModel(el)
        self.elements.append(el)
        
        self.model?.modified()
    }
    
    func add( from source: Element, to target: Element ) {
        // Check for self element, create if necessesary
        let li = Element(name: source.name + ":" + target.name, source:source, target: target )
        self.updateModel(li)
        self.elements.append(li)
        
        self.model?.modified()
    }
    
    func add(get: Element) -> Element {
        self.add(get)
        return get
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
            
            if let data = e.data {
                let elsRoot = TennNode.newCommand(".data")
                parent.add(elsRoot)
                
                let elsBlock = TennNode.newBlockExpr()
                elsRoot.add(elsBlock)
                
                elsBlock.add( TennNode.newCommand("kind", TennNode.newIdent(e.kind.commandName)))
                elsBlock.add( TennNode.newCommand("pos", TennNode.newCommand("-x"), TennNode.newFloatNode(Double(data.x)), TennNode.newCommand("-y"), TennNode.newFloatNode(Double(data.y)) ))
                
                if let linkData = data as? LinkElementData {
                    elsBlock.add( TennNode.newCommand("source", TennNode.newIdent(linkData.source.name)))
                    elsBlock.add( TennNode.newCommand("target", TennNode.newIdent(linkData.target.name)))
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
        
        let pe = self.elementModel.add(get: Element(kind: .Diagram, name: "Platform"))
        
        let pl = pe.add(get: Element(name: "Platform"))
        pl.x = 0
        pl.y = 0
        
        let index = pe.add( get: Element(name: "Index"))
            index.x = 50
            index.y = 50
        
        pe.add(from: pl, to: index)
        
        let st = pe.add( get: Element(name: "StateTracker"))
            st.x = -50
            st.y = -50
        
        pe.add(from: pl, to: st)
            
        let dt = pe.add( get: Element(name: "DeviceTracker"))
            dt.x = 100
            dt.y = -50
        
        pe.add(from: pl, to: dt)
        
        let dev = pe.add( get: Element(name: "Device"))
            dev.x = -50
            dev.y = 50
        
        pe.add(from: dt, to: dev)
            
        let repo = pe.add( get: Element(name: "Repository"))
            repo.x = 50
            repo.y = -100
        
        pe.add(from: pl, to: repo)
        let db = pe.add( get: Element(name: "Database"))
            db.x = 40
            db.y = 50
        
        pe.add(from: repo, to: db)
            
    

    }
}
