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
    
    init() {
        super.init(kind: .Root, name: "Root")
    }
    override func assignModel( _ el: Element) {
        el.model = self
    }
    
    func modified(_ el: Element, _ kind: UpdateEventKind ) {
        for op in onUpdate {
            op(el, kind)
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
    var id: UUID
    var parent: Element?
    
    // Add sub elemnts are linked to parent one
    var items:[DiagramItem]?
    
    init(kind: ElementKind, data: ElementData) {
        self.id = UUID()
        self.kind = kind
        self.data = data
    }
    convenience init( kind: ElementKind, name: String) {
        self.init(kind: kind, data: ElementData())
        self.name = name
    }
    
    private func checkItems() {
        if self.items == nil {
            self.items = []
        }
    }
    
    /// Add a diagram item to current diagram
    func add( _ item: DiagramItem) {
        checkItems()
        self.items?.append(item)
        
        if let p = parent {
            p.model?.assignModel(item)
            p.model?.modified(p, .Structure)
        }
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
    
    var backReferences: [Element] = []
    var parent: Element? = nil
    
    var model: ElementModel?
    
    // A list of items on diagram
    var items: [DiagramItem] = []
    
    var description: String = ""
    
    
    // Item on self diagram of this element
    var selfItem: DiagramItem?
    
    init( kind: ElementKind, name: String, createSelf: Bool = false) {
        self.id = UUID()
        self.name = name
        self.kind = kind
        
        if createSelf {
            self.createSelfItem()
        }
    }
    
    func createSelfItem() {
        let item = DiagramItem(kind: .Element, data: ElementData(refElement: self))
        self.selfItem = item
        self.items.append(item)
    }
    
    convenience init( name: String, createSelf: Bool = false) {
        self.init( kind: .Element, name: name, createSelf: createSelf)
    }
    
    func assignModel( _ el: Element) {
        self.model?.assignModel(el)
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
    
    // Add a child element to current diagram
    func add( _ el: Element, createLink: Bool = true ) -> DiagramItem {
        el.parent = self
        
        self.assignModel(el)
        self.elements.append(el)
        
        // Update current diagram to have a link between a self and new added item.
        let item = DiagramItem(kind: .Element, data: ElementData(refElement: el))
        self.items.append(item)
        assignModel(item)
        
        // We also need to add link from self to this one
        if createLink && self.selfItem != nil {
            let link = DiagramItem(kind: .Link, data: LinkElementData(source: self.selfItem!, target: item))
            self.items.append(link)
            assignModel(link)
        }
        
        self.model?.modified(self, .Structure)
        return item
    }
    
    // Add a child element to current diagram
    func add( source: DiagramItem, target: DiagramItem ) {
        // We also need to add link from self to this one        
        let link = DiagramItem(kind: .Link, data: LinkElementData(source: source, target: target))
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
            var item = getItem(target)
            if item == nil {
                item = DiagramItem(kind: .Element, data: ElementData(refElement: target))
                self.items.append(item!)
                self.assignModel(item!)
            }
            // We also need to add link from self to this one
            
            let link = DiagramItem(kind: .Link, data: LinkElementData(source: sourceItem, target: item!))
            self.items.append(link)
            assignModel(link)
            
            self.model?.modified(self, .Structure)
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
        return ee.toStr(1, false)
    }
    func elementsToTenn( _ parent: TennNode, _ elements: [Element], level: Int ) {
        for e in elements {
            let enode = TennNode.newHashNode(e.name, level: level)
            
            parent.add(enode)
            
            if e.description.count > 0 {
                let elDescr = TennNode.newCommand("description", TennNode.newStrNode(e.description))
                parent.add(elDescr)
            }
            
            if e.items.count == 0 {
                continue
            }
            
            // Put all links from element where it is source.
            
            var links: [DiagramItem:[DiagramItem]] = [:]
            for item in e.items {
                if item.kind == .Link  {
                    if let linkData = item.data as? LinkElementData {
                        var targets = links[linkData.source]
                        if targets == nil {
                            targets = []
                        }
                        targets?.append(item)
                        links[linkData.source] = targets
                    }
                }
            }
            
            for item in e.items {
                if item.kind == .Element {
                    let itemRoot = TennNode.newCommand(item.kind.commandName)
                    parent.add(itemRoot)
                    
                    if let refEl = item.data.refElement {
                        itemRoot.add(TennNode.newIdent("-name"), TennNode.newStrNode(refEl.name))
                    }
                    else {
                        if let nn = item.name {
                            itemRoot.add(TennNode.newIdent("-name"), TennNode.newStrNode(nn))
                        }
                    }
                    
                    let nx = item.data.x != 0
                    let ny = item.data.y != 0

                    if nx {
                        itemRoot.add(TennNode.newIdent("-x"), TennNode.newFloatNode(Double(item.data.x)))
                    }
                    if ny {
                        itemRoot.add(TennNode.newIdent("-y"), TennNode.newFloatNode(Double(item.data.y)))
                    }
                    
                    let itemBlock = TennNode.newBlockExpr()
                    
//                    itemBlock.add(TennNode.newCommand("uuid", TennNode.newStrNode(item.id.uuidString)))
                    
                    if let elLinks = links[item] {
                        for lnk in elLinks {
                            if let linkData = lnk.data as? LinkElementData {
                                var targetName = linkData.target.name
                                if let el = linkData.target.data.refElement {
                                    targetName = el.name
                                }
                                if let tName = targetName {
                                    let linkCmd = TennNode.newCommand("link")
                                    linkCmd.add(TennNode.newIdent("-to"), TennNode.newStrNode(tName))
                                    itemBlock.add(linkCmd)
                                }
                            }
                        }
                    }
                    
                    
                    if itemBlock.count > 0 {
                        itemRoot.add(itemBlock)
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
        
        let pl = Element(name: "platform", createSelf: true)
        _ = self.elementModel.add(pl)
        
        pl.selfItem?.x = 0
        pl.selfItem?.y = 0
    
        let index = pl.add( Element(name: "Index"))
            index.x = -84
            index.y = 102
        
        let st = pl.add( Element(name: "StateTracker"))
            st.x = -189
            st.y = -99
        
        let dt = Element(name: "DeviceTracker", createSelf: true)
        let dte = pl.add( dt )
            dte.x = 129
            dte.y = 48
    
        let dev = dt.add( Element(name: "Device"))
            dev.x = -50
            dev.y = 50
        
        let repo = Element(name: "Repository", createSelf: true)
        let repoe = pl.add( repo )
            repoe.x = 56
            repoe.y = -109
        
        let dbe = Element(name: "Database")
        _ = repo.add( dbe )
        
        let dbi = pl.add(source: repo, target: dbe)
            dbi?.x = 126
            dbi?.y = -216
        
        
        // Add small just platform diagram.
        
        let dm = DiagramItem(kind:.Element, name: "DataModel")
        dm.x = -100
        dm.y = -200
        pl.add( dm)
        
        let str = DiagramItem(kind:.Element, name: "Structure")
        dm.add( str )
        str.add( DiagramItem(kind:.Element, name: "Elements"))
        str.add( DiagramItem(kind:.Element, name: "DiagramItems"))
        
    }
}
