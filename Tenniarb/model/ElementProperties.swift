//
//  ElementProperties.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 08/01/2018.
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

extension Element {
    /// Convert items to list of properties
    func toTennAsProps(_ kind: TennNodeKind = .Statements, reparse: Bool = false) -> TennNode {
        let items = TennNode.newNode(kind: kind)
        items.add(TennNode.newCommand("name", TennNode.newStrNode(self.name)))
        
        buildElementData(self, items)
        // We need to convert it to/back to have a proper positioning
        
        if reparse {
            let parser = TennParser()
            return parser.parse(items.toStr())
        }
        return items
    }
    
    func fromTennProps(_ node: TennNode ) {
        self.properties = ModelProperties()
        
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
    func toTennAsProps(_ kind: TennNodeKind = .Statements, reparse: Bool = false ) -> TennNode {
        let items = TennNode.newNode(kind: kind)
        
        if self.kind == .Item {
            if !self.name.isEmpty {
                items.add(TennNode.newCommand(PersistenceItemKind.Name.commandName, TennNode.newStrNode(self.name)))
            }
            Element.buildItemData(self, items, addPos: true)
        }
        else if self.kind == .Link {
            Element.buildLinkData(self, items, addPos: true)
        }
        
        if reparse {
            let parser = TennParser()
            return parser.parse(items.toStr())
        }
        
        return items
    }
    func fromTennProps( _ node: TennNode ) {
        if self.kind == .Item {
            self.properties = ModelProperties()
            self.x = 0 // In case pos was deleted
            self.y = 0
            Element.traverseBlock(node, {(cmdName, blChild) -> Void in
                if cmdName == PersistenceItemKind.Name.commandName {
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
            self.properties = ModelProperties()
            self.x = 0 // In case pos was deleted
            self.y = 0
            self.name = ""
            Element.traverseBlock(node, {(cmdName, blChild) -> Void in
                if cmdName == PersistenceItemKind.Label.commandName {
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
