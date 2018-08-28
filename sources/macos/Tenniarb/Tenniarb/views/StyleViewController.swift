//
//  StyleViewController.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 27/08/2018.
//  Copyright © 2018 Andrey Sobolev. All rights reserved.
//

import Foundation
import Cocoa

enum StyleOperation {
    case Apply
    case ApplyDefault
    case AddStyleConfig
    case ShowConfig
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

public class StyleViewController: NSViewController {
    
    var delegate: StyleViewControllerDelegate?
    
    @IBOutlet weak var styleOutline: NSOutlineView!
    
    var element: Element?
    
    var viewController: ViewController?
    
    var styleTypes: [StyleInfo] = []
    
    var items: [DiagramItem] = []
    
    func setElement(element: Element) {
        self.element = element
    }
    func setViewController(_ viewcontroller: ViewController ) {
        self.viewController = viewcontroller;
    }
    func setActiveItems(_ items: [DiagramItem]) {
        self.items = items;
    }
    
    public override func viewDidLoad() {
//        styleTypes.append(StyleInfo("Make default.", nil, .ApplyDefault))
        
        
        if let active = self.element {
            if let styleNode = active.properties.get("styles") {
                for styleChild in styleNode.getBlock(1) {
                    if styleChild.isNamedElement(), let styleName = styleChild.getIdent(0), let styleChildBlock = styleChild.getChild(1), (styleName != "item" && styleName != "line" )  {
                        styleTypes.append(StyleInfo("Apply - " + styleName, styleChildBlock, .Apply ))
                    }
                }
            }
        }
        if styleTypes.count == 0 {
            styleTypes.append(StyleInfo("Add styles...", nil, .AddStyleConfig))
        }
        
        self.delegate = StyleViewControllerDelegate(self)
        styleOutline.delegate = delegate!
        styleOutline.dataSource = delegate!
        
        styleOutline.reloadData()
        
        styleOutline.calcSize()
        
        var width = CGFloat(0)
        var height = CGFloat(0)
        
        for i in 0...styleTypes.count {
            let frame = styleOutline.frameOfCell(atColumn: i, row: 0).size
            if frame.width > width {
                width = frame.width
            }
            if frame.height > height {
                height = frame.height
            }
        }
        height = (height + styleOutline.intercellSpacing.height ) * CGFloat(styleTypes.count) + 15
        
        self.view.frame = CGRect(origin: self.view.frame.origin, size: CGSize(width: width, height: height))
        
        styleOutline.calcSize()
    }
    
    public override func viewDidAppear() {
        self.styleOutline.becomeFirstResponder()
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
                TennNode.newCommand("new-style-" + String(styleNode!.getBlock(1).count + 1), TennNode.newBlockExpr(TennNode.newCommand("color", TennNode.newStrNode("white"))))
            )
            //            self.controller.viewController?.mergeProperties(newProps.asNode())
            viewController?.elementStore?.setProperties(active, newProps.asNode(),
                                                        undoManager: viewController?.undoManager,  refresh: {()->Void in})
        }
    }
    fileprivate func doApply( _ node: TennNode ) {
        guard let element = self.element else {
            return
        }
        var ops: [ElementOperation] = []
        
        for itm in self.items {
            let newItemProps = itm.toTennAsProps(.BlockExpr)
            
            var changed = 0
            if let named = node.named {
                // Iterate over uniq named properties
                for (name, prop) in named {
                    if let itmProp = newItemProps.getNamedElement(name) {
                        // Property exists, we need to replace value
                        if let childs = prop.children {
                            itmProp.children?.removeAll()
                            itmProp.add(childs)
                        }
                    }
                    else {
                        // Just add new property
                        newItemProps.add(prop)
                    }
                }
                changed += 1
            }
            if changed > 0 {
                if let op = viewController?.elementStore?.createProperties(element, itm, newItemProps) {
                    ops.append(op)
                }
            }
        }
        viewController?.elementStore?.compositeOperation(notifier: self.element!, undoManaget: viewController?.undoManager, refresh: {}, ops)
    }
}

class StyleViewControllerDelegate: NSObject, NSOutlineViewDataSource, NSOutlineViewDelegate {
    var controller: StyleViewController
    
    init(_ controller: StyleViewController) {
        self.controller = controller
    }
    
    func outlineView(_ outlineView: NSOutlineView, numberOfChildrenOfItem item: Any?) -> Int {
        if item == nil {
            return controller.styleTypes.count
        }
        return 0
    }
    func outlineView(_ outlineView: NSOutlineView, child index: Int, ofItem item: Any?) -> Any {
        if item == nil && index < controller.styleTypes.count {
            return controller.styleTypes[index]
        }
        return ""
    }
    func outlineView(_ outlineView: NSOutlineView, isItemExpandable item: Any) -> Bool {
        return false
    }
    
    
    func outlineView(_ outlineView: NSOutlineView, objectValueFor tableColumn: NSTableColumn?, byItem item: Any?) ->  Any? {
        if let el = item as? StyleInfo {
            return el.name
        }
        return nil
    }
    
    func outlineView(_ outlineView: NSOutlineView, viewFor viewForTableColumn: NSTableColumn?, item: Any) -> NSView? {
        if let el = item as? StyleInfo {
            if let view = outlineView.makeView(withIdentifier: NSUserInterfaceItemIdentifier(rawValue: "ExportCellView"), owner: self) as? NSTableCellView {
                if let textField = view.textField {
                    textField.stringValue = el.name
                }
                
                //                if let imageField = view.viewWithTag(0) as? NSImageView {
                //                    imageField.image = NSImage.init(named: NSImage.Name.init(el.imgName))
                //                }
                return view
            }
        }
        return nil
    }
    
    @objc func outlineViewSelectionDidChange(_ notification: Notification) {
        
        let selectedIndex = controller.styleOutline.selectedRow
        if let el = controller.styleOutline.item(atRow: selectedIndex) as? StyleInfo {
            if el.operation == .AddStyleConfig {
                self.controller.addStyleConfig()
                self.controller.viewController?.scene.setActiveItem(nil)
            }
            else if el.operation == .ShowConfig {
                self.controller.viewController?.scene.setActiveItem(nil)
            }
            else if el.operation == .Apply, let nde = el.node {
                self.controller.doApply(nde)
            }
            else if el.operation == .ApplyDefault, let nde = el.node {
                self.controller.doApply(nde)
            }
        }
        self.controller.dismiss(self.controller)
    }
}
