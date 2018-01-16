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
    func toTennProps() -> String {
        let items = TennNode.newNode(kind: .Statements)
        buildElementData(self, items)
        return items.toStr()
    }
    
    func fromTennProps(_ node: TennNode ) {
        self.properties = []
        
        var linkElements:[(TennNode, DiagramItem)] = []
        Element.traverseBlock(node, {(cmdName, blChild) -> Void in
            Element.parseElementData(self, cmdName, blChild, &linkElements)
        })
        self.model?.modified(self, .Structure)
    }
}

extension DiagramItem {
    /// Convert items to list of properties
    func toTennProps() -> String {
        let items = TennNode.newNode(kind: .Statements)
        
        if self.kind == .Item {
            Element.buildItemData(self, items)
        }
        else if self.kind == .Link {
            Element.buildLinkData(self, items)        
        }
        
        return items.toStr()
    }
    func fromTennProps(_ node: TennNode ) {
        if self.kind == .Item {
            self.properties = []
            Element.traverseBlock(node, {(cmdName, blChild) -> Void in
                Element.parseItemData(self, cmdName, blChild)
            })
        }
        else if self.kind == .Link {
            var sourceIndex = 0
            var targetIndex = 0
            self.properties = []
            Element.traverseBlock(node, {(cmdName, blChild) -> Void in
                Element.parseLinkData(self, cmdName, blChild, &sourceIndex, &targetIndex)
            })
        }
        if let p = self.parent {
            p.model?.modified(p, .Structure)
        }
    }
}
