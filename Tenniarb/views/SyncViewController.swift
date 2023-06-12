//
//  SyncViewController.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 29/07/2018.
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

enum SyncOperation {
    case Sync
    case AddSyncConfig
    case ShowConfig
}

class SyncInfo {
    let name: String
    let node: TennNode?
    let operation: SyncOperation
    
    init( _ name: String, _ node: TennNode?, _ operation: SyncOperation) {
        self.name = name
        self.node = node
        self.operation = operation
    }
}

class SyncViewController: NSViewController {
    
    var delegate: SyncViewControllerDelegate?
    
    @IBOutlet weak var syncOutline: NSOutlineView!
    
    var element: Element?

    var viewController: ViewController?
    
    var syncTypes: [SyncInfo] = []
    
    func setElement(element: Element) {
        self.element = element
    }
    func setViewController(_ viewcontroller: ViewController ) {
        self.viewController = viewcontroller;
    }
    
    override func viewDidLoad() {
        syncTypes.append(SyncInfo("Add sync config...", nil, .AddSyncConfig))
        
        if let active = self.element {
            if let syncNode = active.properties.get("sync") {
                for syncChild in syncNode.getBlock(1) {
                    if syncChild.isNamedElement() && syncChild.getIdent(0) == "config", let syncName = syncChild.getIdent(1)  {
                        syncTypes.append(SyncInfo("Sync - " + syncName, syncChild, .Sync ))
                    }
                }
            }
            
        }
        
        
        self.delegate = SyncViewControllerDelegate(self)
        syncOutline.delegate = delegate!
        syncOutline.dataSource = delegate!

        syncOutline.reloadData()

        syncOutline.layout()

        var width = CGFloat(0)
        var height = CGFloat(0)

        for i in 0...syncTypes.count {
            let frame = syncOutline.frameOfCell(atColumn: i, row: 0).size
            if frame.width > width {
                width = frame.width
            }
            if frame.height > height {
                height = frame.height
            }
        }
        height = (height + syncOutline.intercellSpacing.height ) * CGFloat(syncTypes.count) + 15

        self.view.frame = CGRect(origin: self.view.frame.origin, size: CGSize(width: width, height: height))
        
        syncOutline.layout()
    }
    fileprivate func addSyncConfig() {
        if let active = element {
            let newProps = active.properties.clone()
            
            var syncNode = newProps.get("sync")
            if syncNode == nil {
                syncNode = TennNode.newCommand("sync", TennNode.newBlockExpr())
                newProps.append(syncNode!)
            }
            // Add new sync config
            syncNode?.getChild(1)?.add(
                TennNode.newCommand("config",
                                    TennNode.newStrNode("sync-config-" + String(syncNode!.getBlock(1).count + 1)),
                                    TennNode.newBlockExpr(
                                        TennNode.newCommand("command", TennNode.newStrNode("cmd-line"))
                    )
                )
            )
            //            self.controller.viewController?.mergeProperties(newProps.asNode())
            viewController?.elementStore?.setProperties(active, newProps.asNode(),
                                                                        undoManager: viewController?.undoManager,  refresh: {()->Void in})
        }
    }
    fileprivate func doSync( _ node: TennNode ) {
        
    }
}

class SyncViewControllerDelegate: NSObject, NSOutlineViewDataSource, NSOutlineViewDelegate {
    var controller: SyncViewController

    init(_ controller: SyncViewController) {
        self.controller = controller
    }
    
    func outlineView(_ outlineView: NSOutlineView, numberOfChildrenOfItem item: Any?) -> Int {
        if item == nil {
            return controller.syncTypes.count
        }
        return 0
    }
    func outlineView(_ outlineView: NSOutlineView, child index: Int, ofItem item: Any?) -> Any {
        if item == nil && index < controller.syncTypes.count {
            return controller.syncTypes[index]
        }
        return ""
    }
    func outlineView(_ outlineView: NSOutlineView, isItemExpandable item: Any) -> Bool {
        return false
    }
    
    
    func outlineView(_ outlineView: NSOutlineView, objectValueFor tableColumn: NSTableColumn?, byItem item: Any?) ->  Any? {
        if let el = item as? SyncInfo {
            return el.name
        }
        return nil
    }
    
    func outlineView(_ outlineView: NSOutlineView, viewFor viewForTableColumn: NSTableColumn?, item: Any) -> NSView? {
        if let el = item as? SyncInfo {
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
        
        let selectedIndex = controller.syncOutline.selectedRow
        if let el = controller.syncOutline.item(atRow: selectedIndex) as? SyncInfo {
            if el.operation == .AddSyncConfig {
                self.controller.addSyncConfig()
                self.controller.viewController?.scene.setActiveItem(nil)
            }
            else if el.operation == .ShowConfig {
                self.controller.viewController?.scene.setActiveItem(nil)
            }
            else if el.operation == .Sync, let nde = el.node {
                self.controller.doSync(nde)
            }
        }
        self.controller.dismiss(self.controller)
    }
}
