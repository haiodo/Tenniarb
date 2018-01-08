//
//  ElementPersistence.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 27/11/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Foundation

/**
 Allow Mapping of element model to tenn and wise verse.
 */
extension Element {
    public func toTenn( includeSubElements: Bool = true ) -> TennNode {
        let result = TennNode(kind: TennNodeKind.Statements)
        
        var lvl = 0
        
        var p = self
        while p.parent != nil {
            p = p.parent!
            lvl += 1
        }
        
        if self.kind == .Root {
            buildElements( result, self.elements, lvl, includeSubElements )
        }
        else {
            buildElements( result, [self], lvl, includeSubElements )
        }
        
        return result
    }
    
    func toTennStr( includeSubElements: Bool = true ) -> String {
        let ee = toTenn( includeSubElements: includeSubElements )
        return ee.toStr(0, false)
    }
    
    /// Convert items to list of properties
    func toTennProps() -> String {
        let items = TennNode.newNode(kind: .Statements)
        
        items.add(TennNode.newCommand("name", TennNode.newStrNode(self.name)))
        if let descr = self.description {
            items.add(TennNode.newCommand("description", TennNode.newStrNode(descr)))
        }
        
        return items.toStr()
    }
    
    fileprivate func buildItem(_ item: DiagramItem, _ enodeBlock: TennNode) {
        var name: String = ""
        if let refEl:Element = item.getData(.RefElement) {
            name = refEl.name
        }
        else {
            name = item.name
        }
        
        let itemRoot = TennNode.newCommand(item.kind.commandName, TennNode.newStrNode(name))
        
        enodeBlock.add(itemRoot)
        
        let nx = item.x != 0
        let ny = item.y != 0
        
        let itemBlock = TennNode.newBlockExpr()
        if nx || ny {
            itemBlock.add(TennNode.newCommand("pos", TennNode.newFloatNode(Double(item.x)), TennNode.newFloatNode(Double(item.y))))
        }
        
        if itemBlock.count > 0 {
            itemRoot.add(itemBlock)
        }
    }
    
    fileprivate func buildLink(_ item: DiagramItem, _ enodeBlock: TennNode, _ indexes: [DiagramItem:Int]) {
        if let linkData: LinkElementData = item.getData(.LinkData) {
            let linkCmd = TennNode.newCommand("link")
            linkCmd.add(TennNode.newStrNode(linkData.source.name))
            linkCmd.add(TennNode.newStrNode(linkData.target.name))
            
            let linkDataBlock = TennNode.newBlockExpr()
            
            if let sourceIndex = indexes[linkData.source], sourceIndex != 0 {
                linkDataBlock.add(TennNode.newCommand("source-index", TennNode.newIntNode(sourceIndex)))
            }
            if let targetIndex = indexes[linkData.target], targetIndex != 0 {
                linkDataBlock.add(TennNode.newCommand("target-index", TennNode.newIntNode(targetIndex)))
            }
            
            if linkDataBlock.count > 0 {
                linkCmd.add(linkDataBlock)
            }
            
            enodeBlock.add(linkCmd)
        }
    }
    
    fileprivate func buildItems(_ items: [DiagramItem], _ enodeBlock: TennNode, _ itemIndexes: [DiagramItem:Int]) {
        for item in items {
            if item.kind == .Item {
                buildItem(item, enodeBlock)
            }
            else if item.kind == .Link {
                buildLink(item, enodeBlock, itemIndexes)
            }
        }
    }
    
    func prepareItemRefs( _ items: [DiagramItem] ) -> [DiagramItem:Int] {
        // Prepare element index map
        var itemRefNames:[DiagramItem:Int] = [:]
        
        var strToIndex:[String:Int] = [:]
        
        for item in items {
            if let index = strToIndex[item.name] {
                itemRefNames[item] = index + 1
                strToIndex[item.name] = index + 1
            }
            else {
                strToIndex[item.name] = 0
                itemRefNames[item] = 0
            }
        }

        return itemRefNames
    }
    func buildElements( _ topParent: TennNode, _ elements: [Element], _ level: Int, _ includeSubElements: Bool ) {
        for e in elements {
            let enode = TennNode.newCommand(e.kind.commandName, TennNode.newStrNode(e.name))
            
            topParent.add(enode)
            
            let enodeBlock = TennNode.newBlockExpr()
            
            enode.add(enodeBlock)
            
            if let descr = e.description, descr.count > 0 {
                let elDescr = TennNode.newCommand("description", TennNode.newStrNode(descr))
                enodeBlock.add(elDescr)
            }
            
            let itemIndexes = self.prepareItemRefs(e.items)
            
            buildItems(e.items, enodeBlock, itemIndexes)
            
            if e.elements.count > 0 && includeSubElements {
                buildElements(enodeBlock, e.elements, level + 1, includeSubElements)
            }
        }
    }
}

class IndexedName: Hashable {
    var hashValue: Int {
        get {
            return name.hashValue + index.hashValue
        }
    }
    var name: String = ""
    var index: Int = 0
    
    init(_ name: String, _ index: Int) {
        self.name = name
        self.index = index
    }
    static func ==(lhs: IndexedName, rhs: IndexedName) -> Bool {
        return lhs.name == rhs.name && lhs.index == rhs.index
    }
}

/// Extension to parse tenn models.
extension Element {
    /// Parser tenn model into current element state
    ///
    public static func parseTenn(node: TennNode) -> ElementModel {
        let result = ElementModel()
        if node.kind == .Statements {
            if let childs = node.children {
                for childElement in childs {
                    if childElement.kind == .Command {
                        if let cmd = parseCommand(node: childElement) {
                            result.add(cmd)
                        }
                    }
                }
            }
        }
        else if node.kind == .Command {
            if let cmd = parseCommand(node: node) {
                result.add(cmd)
            }
        }
        return result
    }
    
    static func parseChildCommands( _ node: TennNode, _ blockIndex: Int, _ visitor: (_ cmdName: String, _ child: TennNode) -> Void) {
        if node.count > blockIndex {
            if let block = node.getChild([blockIndex]) {
                if [.Statements, .BlockExpr].contains(block.kind), let blChilds = block.children {
                    for blChild in blChilds {
                        if blChild.kind == .Command, blChild.count > 0, let cmdName = blChild.getIdent(0) {
                            visitor(cmdName, blChild)
                        }
                    }
                }
            }
        }
    }
    
    fileprivate static func prepareRefs( _ items: [DiagramItem] ) -> [IndexedName:DiagramItem] {
        // Prepare element index map
        var itemRefNames:[IndexedName:DiagramItem] = [:]
        
        var strToIndex:[String:Int] = [:]
        
        for item in items {
            if let index = strToIndex[item.name] {
                itemRefNames[IndexedName(item.name, index + 1)] = item
                strToIndex[item.name] = index + 1
            }
            else {
                strToIndex[item.name] = 0
                itemRefNames[IndexedName(item.name, 0)] = item
            }
        }
        
        return itemRefNames
    }
    
    fileprivate static func parseElement(node: TennNode) -> Element? {
        let el = Element(name: "")
        
        var linkElements: [(TennNode, DiagramItem)] = []
        
        if node.count >= 2 {
            if let name = node.getIdent(1) {
                el.name = name
            }
            
            parseChildCommands( node, 2, { (cmdName, blChild) -> Void in
                switch cmdName  {
                case "item":
                    if let item = parseItem(blChild) {
                        el.add(item)
                    }
                case "link":
                    if let item = parseLink(blChild) {
                        el.add(item)
                        linkElements.append((blChild, item))
                    }
                case "element":
                    if let child = parseElement(node: blChild) {
                        el.add(child)
                    }
                default:
                    break;
                }
            })
            
            let refs = prepareRefs(el.items)
            
            for (node, link) in linkElements {
                processLink(link, node, refs)
            }
            
            return el
        }
        
        // Return nil
        //TODo: need to report error
        return nil
    }
    fileprivate static func parseItem(_ node: TennNode) -> DiagramItem? {
        let el = DiagramItem(kind: .Item, name: "")
        
        if node.count >= 2 {
            if let name = node.getIdent(1) {
                el.name = name
            }
            
            parseChildCommands( node, 2, { (cmdName, blChild) -> Void in
                switch cmdName  {
                case "pos":
                    if blChild.count == 3 {
                        if let x = blChild.getFloat(1), let y = blChild.getFloat(2) {
                            el.x = CGFloat(x)
                            el.y = CGFloat(y)
                        }
                    }
                case "description":
                    el.description = blChild.getIdent(1)
                default:
                    break;
                }
            })
        }
        return el
    }
    
    fileprivate static func parseLink(_ node: TennNode) -> DiagramItem? {
        let el = DiagramItem(kind: .Link, name: "")
        
        // Source & Target will be post processed at the end of parsing
        
        if node.count >= 3 {
            parseChildCommands( node, 3, { (cmdName, blChild) -> Void in
                switch cmdName  {
                case "description":
                    break;
                default:
                    break;
                }
            })
        }
        return el
    }
    
    fileprivate static func processLink( _ link: DiagramItem, _ node: TennNode, _ links: [IndexedName: DiagramItem]) {
        
        if node.count >= 2 {
            var sourceIndex = 0
            var targetIndex = 0
            parseChildCommands( node, 3, { (cmdName, blChild) -> Void in
                switch cmdName  {
                case "source-index":
                    if let index = blChild.getInt(1) {
                        sourceIndex = index
                    }
                case "target-index":
                    if let index = blChild.getInt(1) {
                        targetIndex = index
                    }
                default:
                    break;
                }
            })
            
            if let source = node.getIdent(1), let target = node.getIdent(2) {
                let sIndex = IndexedName( source, sourceIndex)
                let tIndex = IndexedName( target, targetIndex)
                
                if let sourceElement = links[sIndex], let targetElement = links[tIndex] {
                    link.setData(.LinkData, LinkElementData(source: sourceElement, target: targetElement))
                }
            }
        }
    }
    
    fileprivate static func parseCommand( node: TennNode) -> Element? {
        if let cmdName = node.getIdent(0) {
            switch cmdName  {
            case "element":
                return parseElement(node: node)
            default:
                break;
            }
        }
        return nil
    }
}
