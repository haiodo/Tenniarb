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
        return self.toTennStr(includeSubElements: false, includeAll: true)
    }
}

extension DiagramItem {
    /// Convert items to list of properties
    func toTennProps() -> String {
        let items = TennNode.newNode(kind: .Statements)
        
        Element.buildItems([self], items, [:], true)
        
        return items.toStr()
    }
}
