//
//  StyleViewController.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 27/08/2018.
//  Copyright © 2018 Andrey Sobolev. All rights reserved.
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
import Cocoa

enum StyleOperation: Int {
    case Apply = 1
//    case ApplyDefault
    case AddStyleConfig
    case ShowConfig
    case separator
}

class StyleInfo {
    let name: String
    let node: TennNode?
    let operation: StyleOperation
    
    init( _ name: String, _ node: TennNode?, _ operation: StyleOperation) {
        self.name = name
        self.node = node
        self.operation = operation
    }
}

public class StyleManager: NSObject, NSMenuDelegate {
    
    var styleTypes: [StyleInfo] = []
    
    var scene: SceneDrawView
    
    init( scene: SceneDrawView ) {
        self.scene = scene
    }
    
    var element: Element? {
        get {
            return scene.element
        }
    }
    
    var elementStore: ElementModelStore? {
        get {
            return scene.store
        }
    }
    var undoManager: UndoManager? {
        get {
            return scene.undoManager
        }
    }
    var activeItems: [DiagramItem] {
        get {
            return scene.activeItems
        }
    }
    
    public func update() {
        self.styleTypes.removeAll()
        
        if let active = element {
            if let styleNode = active.properties.get("styles") {
                for styleChild in styleNode.getBlock(1) {
                    if styleChild.isNamedElement(), let styleName = styleChild.getIdent(0), let styleChildBlock = styleChild.getChild(1), (styleName != "item" && styleName != "line" )  {
                        styleTypes.append(StyleInfo(styleName, styleChildBlock, .Apply ))
                    }
                }
            }
        }
        if styleTypes.count != 0 {
            styleTypes.append(StyleInfo("-", nil, .separator))
        }
        styleTypes.append(StyleInfo("Define new style", nil, .AddStyleConfig))
    }
    
    fileprivate func addStyleConfig() {
        if let active = element {
            let newProps = active.properties.clone()
            
            var styleNode = newProps.get("styles")
            if styleNode == nil {
                styleNode = TennNode.newCommand("styles", TennNode.newBlockExpr())
                newProps.append(styleNode!)
            }
            // Add new sync config
            styleNode?.getChild(1)?.add(
                TennNode.newCommand("new_style_" + String(styleNode!.getBlock(1).count + 1), TennNode.newBlockExpr(TennNode.newCommand("color", TennNode.newStrNode("white"))))
            )
            //            self.controller.viewController?.mergeProperties(newProps.asNode())
            elementStore?.setProperties(active, newProps.asNode(),
                                                        undoManager: undoManager,  refresh: {()->Void in})
        }
    }
    fileprivate func doApply( _ styleName: String, _ node: TennNode ) {
        guard let element = self.element else {
            return
        }
        var ops: [ElementOperation] = []
        
        for itm in activeItems {
            let newItemProps = itm.toTennAsProps(.BlockExpr)
            
            var changed = 0
            if let itmProp = newItemProps.getNamedElement("use-style") {
                // Property exists, we need to replace value
                if itmProp.getIdent(1) != styleName {
                    itmProp.children?.removeAll()
                    itmProp.add(TennNode.newIdent("use-style"), TennNode.newIdent(styleName))
                    changed += 1
                }
            }
            else {
                // Just add new property
                newItemProps.add(TennNode.newCommand("use-style", TennNode.newIdent(styleName)))
                changed += 1
            }
            
            if changed > 0 {
                if let op = elementStore?.createProperties(element, itm, newItemProps) {
                    ops.append(op)
                }
            }
        }
        elementStore?.compositeOperation(notifier: self.element!, undoManaget: undoManager, refresh: {}, ops)
    }
    
    @objc func performAction(_ sender: NSMenuItem ) {
        let el = self.styleTypes[sender.tag]
        
        if el.operation == .AddStyleConfig {
            addStyleConfig()
            scene.setActiveItem(nil)
        }
        else if el.operation == .ShowConfig {
            scene.setActiveItem(nil)
        }
        else if el.operation == .Apply, let nde = el.node {
            doApply(el.name, nde)
        }
//        else if el.operation == .ApplyDefault, let nde = el.node {
//            doApply(nde)
//        }
    }
    
    func createMenu() -> NSMenu {
        self.update()
        
        let menu = NSMenu()
        menu.autoenablesItems=true
        var idx = 0
        for itm in self.styleTypes {
            if itm.operation == .separator {
                menu.addItem(NSMenuItem.separator())
                idx += 1
                continue;
            }
            let menuItem = NSMenuItem(title: itm.name, action: #selector(performAction), keyEquivalent: "")
//            let img = NSImage.init(named: itm.imgName)
//            exportHtmlItem.image = NSImage.init(size: NSSize(width: 24, height: 24), flipped: false, drawingHandler: {
//                (rect) in img?.draw(in: rect)
//                return true
//            })
            menuItem.target = self
            menuItem.tag = idx
            idx += 1
            
            menu.addItem(menuItem)
        }
        
        return menu
    }
}
