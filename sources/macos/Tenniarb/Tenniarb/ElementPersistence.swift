//
//  ElementPersistence.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 27/11/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Foundation


public enum PersistenceItemKind {
    case Item // A reference to element
    case Link // A link
    case Element
    case Model
    case Annontation // Annotation box
    case Description
    case SourceIndex
    case TargetIndex
    case Name
    case Position
    
    var commandName : String {
        switch self {
        // Use Internationalization, as appropriate.
        case .Item: return "item";
        case .Link: return "link";
        case .Element: return "element";
        case .Model: return "model";
        case .Annontation: return "annotation";
        case .Description: return "desription";
        case .SourceIndex: return "source-index";
        case .TargetIndex: return "target-index";
        case .Position: return "pos";
        case .Name: return "name";
        }
    }
}

/**
 Allow Mapping of element model to tenn and wise verse.
 */
extension Element {
    public func toTenn( includeSubElements: Bool = true, includeItems: Bool = true ) -> TennNode {
        let result = TennNode(kind: TennNodeKind.Statements)
        
        if self.kind == .Root {
            buildElements(topParent: result, elements: self.elements, includeSubElements: includeSubElements, includeItems: includeItems)
        }
        else {
            buildElements(topParent: result, elements: [self], includeSubElements: includeSubElements, includeItems: includeItems)
        }
        
        return result
    }
    
    func toTennStr( includeSubElements: Bool = true, includeItems: Bool = true ) -> String {
        let ee = toTenn( includeSubElements: includeSubElements, includeItems: includeItems )
        return ee.toStr(0, false)
    }
    
    static func buildItemData(_ item: DiagramItem, _ itemBlock: TennNode) {
        let nx = item.x != 0
        let ny = item.y != 0
        
        if nx || ny {
            itemBlock.add(TennNode.newCommand(PersistenceItemKind.Position.commandName, TennNode.newFloatNode(Double(item.x)), TennNode.newFloatNode(Double(item.y))))
        }
        
        for p in item.properties {
            itemBlock.add(p.clone())
        }
    }
    
    static func buildItem(_ item: DiagramItem, _ enodeBlock: TennNode) {
        var name: String = ""
        if let refEl:Element = item.getData(.RefElement) {
            name = refEl.name
        }
        else {
            name = item.name
        }
        
        let itemRoot = TennNode.newCommand(PersistenceItemKind.Item.commandName, TennNode.newStrNode(name))
        
        enodeBlock.add(itemRoot)
        
        let itemBlock = TennNode.newBlockExpr()
        
        buildItemData(item, itemBlock)
        
        if itemBlock.count > 0 {
            itemRoot.add(itemBlock)
        }
    }
    
    static func buildLinkData(_ item: DiagramItem, _ linkDataBlock: TennNode) {
        if let descr = item.description {
            linkDataBlock.add(TennNode.newCommand(PersistenceItemKind.Description.commandName, TennNode.newStrNode(descr)))
        }
        
        let nx = item.x != 0
        let ny = item.y != 0
        
        if nx || ny {
            linkDataBlock.add(TennNode.newCommand(PersistenceItemKind.Position.commandName, TennNode.newFloatNode(Double(item.x)), TennNode.newFloatNode(Double(item.y))))
        }
        
        for p in item.properties {
            linkDataBlock.add(p.clone())
        }
    }
    
    static func buildLink(_ item: DiagramItem, _ enodeBlock: TennNode, _ indexes: [DiagramItem:Int]) {
        if let linkData: LinkElementData = item.getData(.LinkData) {
            let linkCmd = TennNode.newCommand(PersistenceItemKind.Link.commandName)
            linkCmd.add(TennNode.newStrNode(linkData.source.name))
            linkCmd.add(TennNode.newStrNode(linkData.target.name))
            
            let linkDataBlock = TennNode.newBlockExpr()
            
            if let sourceIndex = indexes[linkData.source], sourceIndex != 0 {
                linkDataBlock.add(TennNode.newCommand(PersistenceItemKind.SourceIndex.commandName, TennNode.newIntNode(sourceIndex)))
            }
            if let targetIndex = indexes[linkData.target], targetIndex != 0 {
                linkDataBlock.add(TennNode.newCommand(PersistenceItemKind.TargetIndex.commandName, TennNode.newIntNode(targetIndex)))
            }
            
            buildLinkData(item, linkDataBlock)
            
            if linkDataBlock.count > 0 {
                linkCmd.add(linkDataBlock)
            }
            
            enodeBlock.add(linkCmd)
        }
    }
    
    static func buildItems(_ items: [DiagramItem], _ enodeBlock: TennNode, _ itemIndexes: [DiagramItem:Int]) {
        for item in items {
            if item.kind == .Item {
                Element.buildItem(item, enodeBlock)
            }
            else if item.kind == .Link {
                Element.buildLink(item, enodeBlock, itemIndexes)
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
    func buildElementData(_ e: Element, _ enodeBlock: TennNode) {
        if let descr = e.description, descr.count > 0 {
            let elDescr = TennNode.newCommand(PersistenceItemKind.Description.commandName, TennNode.newStrNode(descr))
            enodeBlock.add(elDescr)
        }
        
        for p in e.properties {
            enodeBlock.add(p.clone())
        }
    }
    
    fileprivate func buildElement(e: Element, topParent: TennNode, includeSubElements: Bool,  includeItems: Bool) {
        let enode = TennNode.newCommand(PersistenceItemKind.Element.commandName, TennNode.newStrNode(e.name))
        
        topParent.add(enode)
        
        let enodeBlock = TennNode.newBlockExpr()
        
        enode.add(enodeBlock)
        
        buildElementData(e, enodeBlock)
        
        let itemIndexes = self.prepareItemRefs(e.items)
        
        if includeItems {
            Element.buildItems(e.items, enodeBlock, itemIndexes)
        }
        
        if e.elements.count > 0 && includeSubElements {
            buildElements(topParent: enodeBlock, elements: e.elements, includeSubElements: includeSubElements, includeItems: includeItems)
        }
    }
    
    func buildElements( topParent: TennNode, elements: [Element], includeSubElements: Bool, includeItems: Bool ) {
        for e in elements {
            buildElement(e: e, topParent: topParent, includeSubElements: includeSubElements, includeItems: includeItems)
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
                        else {
                            result.properties.append(childElement)
                        }
                    }
                    else {
                        result.properties.append(childElement)
                    }
                }
            }
        }
        else if node.kind == .Command {
            if let cmd = parseCommand(node: node) {
                result.add(cmd)
            }
        }
        else {
            result.properties.append(node)
        }
        return result
    }
    
    static func traverseBlock(_ block: TennNode, _ visitor: (String, TennNode) -> Void) {
        if [.Statements, .BlockExpr].contains(block.kind), let blChilds = block.children {
            for blChild in blChilds {
                if blChild.kind == .Command, blChild.count > 0, let cmdName = blChild.getIdent(0) {
                    visitor(cmdName, blChild)
                }
            }
        }
    }
    
    static func parseChildCommands( _ node: TennNode, _ blockIndex: Int, _ visitor: (_ cmdName: String, _ child: TennNode) -> Void) {
        if node.count > blockIndex {
            if let block = node.getChild([blockIndex]) {
                traverseBlock(block, visitor)
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
    
    static func parseElementData(_ el:Element, _ cmdName: String, _ blChild: TennNode,_ linkElements: inout [(TennNode, DiagramItem)]) {
        switch cmdName  {
        case PersistenceItemKind.Item.commandName:
            if let item = parseItem(blChild) {
                el.add(item)
            }
            else {
                el.properties.append(blChild)
            }
        case PersistenceItemKind.Link.commandName:
            if let item = parseLink(blChild) {
                el.add(item)
                linkElements.append((blChild, item))
            }
            else {
                el.properties.append(blChild)
            }
        case PersistenceItemKind.Element.commandName:
            if let child = parseElement(node: blChild) {
                el.add(child)
            }
            else {
                el.properties.append(blChild)
            }
        default:
            el.properties.append(blChild)
            break;
        }
    }
    
    fileprivate static func parseElement(node: TennNode) -> Element? {
        let el = Element(name: "")
        
        var linkElements: [(TennNode, DiagramItem)] = []
        
        if node.count >= 2 {
            if let name = node.getIdent(1) {
                el.name = name
            }
            
            parseChildCommands( node, 2, { (cmdName, blChild) -> Void in
                parseElementData(el, cmdName, blChild, &linkElements)
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
    static func parseItemData(_ el: DiagramItem, _ cmdName: String, _ blChild: TennNode) {
        switch cmdName  {
        case PersistenceItemKind.Position.commandName:
            if blChild.count == 3 {
                if let x = blChild.getFloat(1), let y = blChild.getFloat(2) {
                    el.x = CGFloat(x)
                    el.y = CGFloat(y)
                }
                else {
                    el.properties.append(blChild);
                }
            }
            else {
                el.properties.append(blChild);
            }
        case PersistenceItemKind.Description.commandName:
            el.description = blChild.getIdent(1)
        default:
            el.properties.append(blChild);
            break;
        }
    }
    
    fileprivate static func parseItem(_ node: TennNode) -> DiagramItem? {
        let el = DiagramItem(kind: .Item, name: "")
        
        if node.count >= 2 {
            if let name = node.getIdent(1) {
                el.name = name
            }
            
            parseChildCommands( node, 2, { (cmdName, blChild) -> Void in
                parseItemData(el, cmdName, blChild)
            })
        }
        return el
    }
    
    fileprivate static func parseLink(_ node: TennNode) -> DiagramItem? {
        if node.count >= 2 {
            let el = DiagramItem(kind: .Link, name: "")
            return el
        }
        return nil
    }
    
    static func parseLinkData(_ link: DiagramItem, _ cmdName: String, _ blChild: TennNode, _ sourceIndex: inout Int, _ targetIndex: inout Int) {
        switch cmdName  {
        case PersistenceItemKind.Description.commandName:
            link.description = blChild.getIdent(1)
        case PersistenceItemKind.Position.commandName:
            if blChild.count == 3 {
                if let x = blChild.getFloat(1), let y = blChild.getFloat(2) {
                    link.x = CGFloat(x)
                    link.y = CGFloat(y)
                }
                else {
                    link.properties.append(blChild);
                }
            }
            else {
                link.properties.append(blChild);
            }
        case PersistenceItemKind.SourceIndex.commandName:
            if let index = blChild.getInt(1) {
                sourceIndex = index
            }
            else {
                link.properties.append(blChild)
            }
        case PersistenceItemKind.TargetIndex.commandName:
            if let index = blChild.getInt(1) {
                targetIndex = index
            }
            else {
                link.properties.append(blChild)
            }
        default:
            link.properties.append(blChild)
            break;
        }
    }
    
    fileprivate static func processLink( _ link: DiagramItem, _ node: TennNode, _ links: [IndexedName: DiagramItem]) {
        var sourceIndex = 0
        var targetIndex = 0
        parseChildCommands( node, 3, { (cmdName, blChild) -> Void in
            parseLinkData(link, cmdName, blChild, &sourceIndex, &targetIndex)
        })
        
        if let source = node.getIdent(1), let target = node.getIdent(2) {
            let sIndex = IndexedName( source, sourceIndex)
            let tIndex = IndexedName( target, targetIndex)
            
            if let sourceElement = links[sIndex], let targetElement = links[tIndex] {
                link.setData(.LinkData, LinkElementData(source: sourceElement, target: targetElement))
            }
        }
    }
    
    fileprivate static func parseCommand( node: TennNode) -> Element? {
        if let cmdName = node.getIdent(0) {
            switch cmdName  {
            case PersistenceItemKind.Element.commandName, PersistenceItemKind.Model.commandName:
                return parseElement(node: node)
            default:
                break;
            }
        }
        return nil
    }
}
