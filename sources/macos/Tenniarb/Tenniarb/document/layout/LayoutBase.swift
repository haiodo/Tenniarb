//
//  LayoutContext.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 30/07/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Foundation


protocol LayoutFilter {
    /**
         Return [true] if item is irrelevant for layout
     */
    func isLayoutIrrelevant( item: DiagramItem ) -> Bool
}

protocol LayoutAlgorithm {
    func apply( context: LayoutContext, clean: Bool)
}


public class LayoutContext {
    
    // A container to perform layout on
    var element: Element
    
    var filters: [LayoutFilter] = []
    
    public var preLayoutPass: [() -> Void] = []
    public var postLayoutPass: [() -> Void] = []
    
    public init(_ element: Element) {
        self.element = element
    }
    
    public func preLayout() {
        for pre in preLayoutPass {
            pre()
        }
    }
    public func postLayout() {
        for post in postLayoutPass {
            post()
        }
    }
    
    func getAllItems(kind: ElementKind) -> [DiagramItem] {
        var result: [DiagramItem] = []
        
        var queue: [DiagramItem] = []
        
        for itm in self.element.items {
            queue.append(itm)
        }
        
        while !queue.isEmpty {
            let item = queue.removeFirst()
            if item.kind == kind {
                result.append(item)
            }
            if let childItems = item.items {
                for ci in childItems {
                    queue.append(ci)
                }
            }
        }
        
        return result
    }
    
    public func getNodes() -> [DiagramItem] {
        return []
    }
    
}






















