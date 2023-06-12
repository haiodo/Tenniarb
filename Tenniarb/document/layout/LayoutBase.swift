//
//  LayoutContext.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 30/07/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//
//  See the License for the specific language governing permissions and
//  limitations under the License.

import Foundation


protocol LayoutAlgorithm {
    func apply( context: LayoutContext, clean: Bool) -> [ElementOperation]
}


public class LayoutContext {
    
    // A container to perform layout on
    var element: Element
        
    public var preLayoutPass: [() -> Void] = []
    public var postLayoutPass: [() -> Void] = []
    
    var layout: LayoutAlgorithm?
    let scene: DrawableScene
    
    
    var nodes: [DiagramItem] = []
    var edges: [LinkItem] = []
    
    var store: ElementModelStore
    var bounds: CGRect
    
    public init(_ element: Element, scene: DrawableScene, store: ElementModelStore, bounds: CGRect) {
        self.element = element
        self.scene = scene
        self.store = store
        self.bounds = bounds
        
        nodes = findNodes()
        self.edges = findEdges()
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
    
    public func getBounds() -> CGRect {
        return scene.getBounds()
    }
    
    public func getViewBounds() -> CGRect {
        return self.bounds
    }
    
    public func getBounds(node: DiagramItem) -> CGRect {
        if let dr = scene.drawables[node] {
            return dr.getBounds()
        }
        return CGRect()
    }
    
    func apply(_ clean: Bool) -> [ElementOperation] {
        guard let layout = self.layout else {
            return []
        }
        preLayout()
        let ops = layout.apply(context: self, clean: clean)
        postLayout()
        return ops
    }
    
    func findNodes() -> [DiagramItem] {
        var result: [DiagramItem] = []
        
        for itm in self.element.items {
            if itm.kind == .Item {
                result.append(itm)
            }
        }
        
        return result
    }
    
    func findEdges() -> [LinkItem] {
        var result: [LinkItem] = []
        
        for itm in self.element.items {
            if itm.kind == .Link {
                result.append(itm as! LinkItem)
            }
        }
        
        return result
    }
    public func isMovable(_ node: DiagramItem) -> Bool {
        // For now any item are movable.
        return true
    }
    func getWeight(_ link: LinkItem ) -> CGFloat {
        return 0;
    }
}






















