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
    public func toTenn( ) -> TennNode {
        let result = TennNode(kind: TennNodeKind.Statements)
        
        var lvl = 0
        
        var p = self
        while p.parent != nil {
            p = p.parent!
            lvl += 1
        }
        
        if self.kind == .Root {
            elementsToTenn( result, self.elements, level: lvl )
        }
        else {
            elementsToTenn( result, [self], level: lvl )
        }
        
        return result
    }
    
    func toTennStr( ) -> String {
        let ee = toTenn()
        return ee.toStr(0, false)
    }
    
    /// Convert items to list of properties
    func toTennProps() -> String {
        let items = TennNode.newNode(kind: .Statements)
        
        items.add(TennNode.newCommand("name", TennNode.newStrNode(self.name)))
        items.add(TennNode.newCommand("description", TennNode.newStrNode(self.description)))
        
        return items.toStr()
    }
    
    fileprivate func buildItem(_ item: DiagramItem, _ enodeBlock: TennNode) {
        var name: String = ""
        if let refEl = item.data.refElement {
            name = refEl.name
        }
        else {
            if let nn = item.name {
                name = nn
            }
        }
        
        let itemRoot = TennNode.newCommand(item.kind.commandName, TennNode.newStrNode(name))
        
        enodeBlock.add(itemRoot)
        
        let nx = item.data.x != 0
        let ny = item.data.y != 0
        
        let itemBlock = TennNode.newBlockExpr()
        if nx || ny {
            itemBlock.add(TennNode.newCommand("pos", TennNode.newFloatNode(Double(item.data.x)), TennNode.newFloatNode(Double(item.data.y))))
        }
        
        if itemBlock.count > 0 {
            itemRoot.add(itemBlock)
        }
    }
    
    fileprivate func buildLink(_ item: DiagramItem, _ enodeBlock: TennNode) {
        if let linkData = item.data as? LinkElementData {
            var targetName = linkData.target.name
            if let el = linkData.target.data.refElement {
                targetName = el.name
            }
            
            var sourceName = linkData.source.name
            if let el = linkData.source.data.refElement {
                sourceName = el.name
            }
            
            if let tName = targetName, let sName = sourceName {
                let linkCmd = TennNode.newCommand("link")
                linkCmd.add(TennNode.newStrNode(sName))
                linkCmd.add(TennNode.newStrNode(tName))
                enodeBlock.add(linkCmd)
            }
        }
    }
    
    fileprivate func buildItems(_ items: [DiagramItem], _ enodeBlock: TennNode) {
        for item in items {
            if item.kind == .Item {
                buildItem(item, enodeBlock)
            }
            else if item.kind == .Link {
                buildLink(item, enodeBlock)
            }
        }
    }
    
    func elementsToTenn( _ topParent: TennNode, _ elements: [Element], level: Int ) {
        for e in elements {
            let enode = TennNode.newCommand(e.kind.commandName, TennNode.newStrNode(e.name))
            
            topParent.add(enode)
            
            let enodeBlock = TennNode.newBlockExpr()
            
            enode.add(enodeBlock)
            
            if e.description.count > 0 {
                let elDescr = TennNode.newCommand("description", TennNode.newStrNode(e.description))
                enodeBlock.add(elDescr)
            }
            
            if e.items.count == 0 {
                continue
            }
            
            buildItems(e.items, enodeBlock)
            if e.elements.count > 0 {
                elementsToTenn(enodeBlock, e.elements, level: level + 1)
            }
        }
    }
}

/// Extension to parse tenn models.
extension Element {
    /// Parser tenn model into current element state
    ///
    public static func parseTenn(node: TennNode) -> Element? {
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
    
    fileprivate static func parseElement(target: Element, node: TennNode) {
        let el = Element(name: "")
        if node.count == 3 {
            if let name = node.getIdent(1) {
                el.name = name
            }
            
            if let block = node.getChild([2]) {
                if block.kind == .Statements, let blChilds = block.children {
                    for blChild in blChilds {
                        if blChild.kind == .Command, blChild.count > 0, let cmdName = blChild.getIdent(0) {
                            switch cmdName  {
                            case "item":
                                if let item = parseItem(node) {
                                    el.items.append(item)
                                }
                            case "element":
                                let el = Element(name: "")
                                parseElement(target: el, node: node)
                                el.elements.append(el)
                            default:
                                break;
                            }
                        }
                    }
                }
            }
        }
    }
    fileprivate static func parseItem(_ node: TennNode) -> DiagramItem? {
        let el = DiagramItem(kind: .Item, name: "")
        return el
    }
    
    fileprivate static func parseCommand( node: TennNode) -> Element? {
        if let cmdName = node.getIdent(0) {
            switch cmdName  {
            case "model":
                let model = ElementModel()
                parseElement(target: model, node: node)
                return model
            case "element":
                if node.count >= 2 {
                    if let nname = node.getIdent(1) {
                        let model = Element(name: nname )
                        parseElement(target: model, node: node)
                        return model
                    }
                }
                // Report parse error
                return nil
            default:
                break;
            }
        }
        return nil
    }
}
