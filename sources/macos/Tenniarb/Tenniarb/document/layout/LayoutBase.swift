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
    
    func getNodes() -> [DiagramItem] {
        var result: [DiagramItem] = []
        
        for itm in self.element.items {
            if itm.kind == .Item {
                result.append(itm)
            }
        }
        
        return result
    }
    
    public func getEdges() -> [DiagramItem] {
        var result: [DiagramItem] = []
        
        for itm in self.element.items {
            if itm.kind == .Link {
                result.append(itm)
            }
        }
        
        return result
    }
    
}






















