//
//  ElementProperties.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 08/01/2018.
//  Copyright © 2018 Andrey Sobolev. All rights reserved.
//

import Foundation

extension Element {
    /// Convert items to list of properties
    func toTennAsProps() -> TennNode {
        let items = TennNode.newNode(kind: .Statements)
        items.add(TennNode.newCommand("name", TennNode.newStrNode(self.name)))
        
        buildElementData(self, items)
        return items
    }
    func toTennProps() -> String {
        return toTennAsProps().toStr()
    }
    
    func fromTennProps( _ store: ElementModelStore, _ node: TennNode ) {
        self.properties = []
        
        var linkElements:[(TennNode, LinkItem)] = []
        Element.traverseBlock(node, {(cmdName, blChild) -> Void in
            if cmdName == "name" {
                if let newName = blChild.getIdent(1) {
                    self.name = newName
                }
                return
            }
            
            Element.parseElementData(self, cmdName, blChild, &linkElements)
        })
    }
}

extension DiagramItem {
    /// Convert items to list of properties
    func toTennAsProps() -> TennNode {
        let items = TennNode.newNode(kind: .Statements)
        
        if self.kind == .Item {
            items.add(TennNode.newCommand("name", TennNode.newStrNode(self.name)))
            Element.buildItemData(self, items)
        }
        else if self.kind == .Link {
            Element.buildLinkData(self, items)
        }
        
        return items
    }
    func toTennProps() -> String {
        return toTennAsProps().toStr()
    }
    func fromTennProps( _ store: ElementModelStore, _ node: TennNode ) {
        if self.kind == .Item {
            self.properties = []
            self.x = 0 // In case pos was deleted
            self.y = 0
            Element.traverseBlock(node, {(cmdName, blChild) -> Void in
                if cmdName == "name" {
                    if let newName = blChild.getIdent(1) {
                        self.name = newName
                    }
                    return
                }
                Element.parseItemData(self, cmdName, blChild)
            })
        }
        else if self.kind == .Link {
            var sourceIndex = 0
            var targetIndex = 0
            self.properties = []
            self.x = 0 // In case pos was deleted
            self.y = 0
            Element.traverseBlock(node, {(cmdName, blChild) -> Void in
                if cmdName == "name" {
                    if let newName = blChild.getIdent(1) {
                        self.name = newName
                    }
                    return
                }
                Element.parseLinkData(self, cmdName, blChild, &sourceIndex, &targetIndex)
            })
        }
    }
}
