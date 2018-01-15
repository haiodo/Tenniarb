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
}
