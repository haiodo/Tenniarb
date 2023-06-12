//
//  ModelActions.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 29/01/2018.
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

var DEBUG_OPERATION_TRACKING=false

public enum ModelEventKind {
    case Structure
    case Layout
}

public enum ModelEventOperation {
    case Append
    case Remove
    case Update
}

public class ModelEvent {
    let kind: ModelEventKind
    var element: Element
    var items: [DiagramItem:ModelEventOperation] = [:]
    var elements: [Element:ModelEventOperation] = [:]

    init(kind: ModelEventKind, element: Element) {
        self.kind = kind
        self.element = element
    }
    init(kind: ModelEventKind, element: Element, items: [DiagramItem]) {
        self.kind = kind
        self.element = element
    }
}

public class ElementOperation {
    var store: ElementModelStore
    var isUndoCalled: Bool = true
    var name:String {
        get {
            return "Unnamed"
        }
    }
    
    init( _ store: ElementModelStore ) {
        self.store = store
    }
    func apply() {
    }
    func undo() {
    }
    
    func getNotifier() -> Element {
        return store.model
    }
    func getEventKind() -> ModelEventKind {
        return .Structure
    }
    func collect( _ items: inout [DiagramItem:ModelEventOperation] ) {
    }
    func collect( _ elements: inout [Element:ModelEventOperation] ) {
    }
}

/*
    A composite operation for Element inside changes.
*/
public class CompositeOperation: ElementOperation {
    var operations: [ElementOperation]
    var notifier: Element
    
    init(_ store:ElementModelStore, _ notifier: Element, _ ops: ElementOperation...) {
        self.operations = ops
        self.notifier = notifier
        super.init(store)
    }
    
    init(_ store:ElementModelStore, _ notifier: Element, _ ops: [ElementOperation]) {
        self.operations = ops
        self.notifier = notifier
        super.init(store)
    }
    
    func add(_ ops: ElementOperation...) {
        self.operations.append(contentsOf: ops)
    }
    
    override func getEventKind() -> ModelEventKind {
        // Check if all operations are add operations, when it is add.
        
        if self.operations.count == 0 {
            return super.getEventKind()
        }
        let kind = self.operations[0].getEventKind()
        for op in self.operations {
            if kind != op.getEventKind() {
                // If not same, return Structure.
                return super.getEventKind()
            }
        }
        return kind
    }
    
    override var name:String {
        get {
            var r = ""
            for op in self.operations {
                r.append(op.name + ",")
            }
            if r.count > 0 {
                r.removeLast()
            }
            return r
        }
    }
    
    override func apply() {
        for op in self.operations {
            op.apply()
            op.isUndoCalled = false
        }
    }
    override func undo() {
        for op in self.operations.reversed() {
            op.undo()
            op.isUndoCalled = true
        }
    }
    override func getNotifier() -> Element {
        return self.notifier
    }
    override func collect( _ items: inout [DiagramItem:ModelEventOperation] ) {
        for op in self.operations {
            op.collect(&items)
        }
    }
    override func collect(_ elements: inout [Element : ModelEventOperation]) {
        for op in self.operations {
            op.collect(&elements)
        }
    }
}

public protocol IElementModelListener {
    func notifyChanges(_ event: ModelEvent )
}

public class ElementModelStore {
    public let model: ElementModel
    
    public var onUpdate: [IElementModelListener] = []
    public var modified: Bool = false
    public var executionContext = ExecutionContext()

    private let internalGroup: DispatchGroup = DispatchGroup()
    
    init(_ model:ElementModel ) {
        self.model = model
    }
    
    func execute( _ action: ElementOperation, _ undoManager: UndoManager?, _ refresh: @escaping () -> Void) {
        self.internalGroup.enter()
        defer {
            self.internalGroup.leave()
        }
        if let manager = undoManager {
            manager.registerUndo(withTarget: self, handler: {(ae: ElementModelStore) -> Void in
                ae.execute(action, manager, refresh)
            })
        }
        
        if DEBUG_OPERATION_TRACKING {
            Swift.debugPrint("Calling operation:",action.name," state:",action.isUndoCalled)
        }
        
        if !action.isUndoCalled {
            action.undo()
            action.isUndoCalled = true
        }
        else {
            action.apply()
            action.isUndoCalled = false
        }
        
        let evt = ModelEvent(kind: action.getEventKind(), element: action.getNotifier())
        action.collect(&evt.items)
        action.collect(&evt.elements)
        
        DispatchQueue.global(qos: .background).async {
            // Do calculation in background
            self.executionContext.notifyChanges(evt)
            
            DispatchQueue.main.async {
                self.modified(evt)
                refresh()
            }
        }
    }
    
    public func updateName( item: DiagramItem, _ newName: String, undoManager: UndoManager?, refresh: @escaping () -> Void) {
        execute(UpdateName(self, item.parent!, item, old: item.name, new: newName), undoManager, refresh)
    }
    
    public func updateName( element: Element, _ newName: String, undoManager: UndoManager?, refresh: @escaping () -> Void) {
        execute(UpdateElementName(self, element, old: element.name, new: newName), undoManager, refresh)
    }
    
    public func updatePosition( item: DiagramItem, newPos: CGPoint, undoManager: UndoManager?, refresh: @escaping () -> Void) {
        execute(self.createUpdatePosition(item: item, newPos: newPos), undoManager, refresh)
    }
    

    public func createUpdatePosition( item: DiagramItem, newPos: CGPoint) -> ElementOperation {
        return UpdatePosition(self, item.parent!, item, old: CGPoint(x: item.x, y: item.y), new: newPos)
    }
    
    public func createUpdateOrder( item: DiagramItem, newPos: Int?) -> [ElementOperation] {
        var ops:[ElementOperation] = []
        ops.append(RemoveItem(self, item.parent!, item))
        ops.append(AddItem(self, item.parent!, item, at: newPos))
        return ops
    }
    
    public func compositeOperation( notifier: Element, undoManaget: UndoManager?, refresh: @escaping () -> Void,
                                    _ operations: [ElementOperation] ) {
        let composite = CompositeOperation(self, notifier, operations )
        execute( composite, undoManaget, refresh)
    }
    
    public func add( _ parent: Element, _ child: Element, undoManager: UndoManager?, refresh: @escaping () -> Void, index: Int? = nil ) {
        execute(AddElement(self, parent, child, index: index), undoManager, refresh)
    }
    
    public func addElements( _ parent: Element, _ childs: [Element], undoManager: UndoManager?, refresh: @escaping () -> Void, index: Int? = nil ) {
        var ops: [ElementOperation] = []
        for el in childs {
            ops.append(AddElement(self, parent, el, index: nil))
        }
        compositeOperation( notifier: parent, undoManaget: undoManager, refresh: refresh, ops)
    }
    
    
    public func move( _ element: Element, _ newParent: Element, undoManager: UndoManager?, refresh: @escaping () -> Void, index: Int ) {
        
        let op = CompositeOperation(self, element)
        
        op.add(RemoveElement(self, element.parent!, element))
        op.add(AddElement(self, newParent, element, index: index))
        
        execute(op, undoManager, refresh)
    }
    
    public func remove( _ parent: Element, _ child: Element, undoManager: UndoManager?, refresh: @escaping () -> Void  ) {
        execute(RemoveElement(self, parent, child), undoManager, refresh)
    }
    
    public func add( _ element: Element, _ item: DiagramItem, undoManager: UndoManager?, refresh: @escaping () -> Void ) {
        execute(AddItem(self, element, item), undoManager, refresh)
    }
    
    public func add( _ element: Element, _ items: [DiagramItem], undoManager: UndoManager?, refresh: @escaping () -> Void ) {
        let ops = items.map({(itm) in AddItem(self, element, itm)})
        let op = CompositeOperation(self, element, ops )
        execute(op, undoManager, refresh)
    }
    
    public func add( _ element: Element, source: DiagramItem, target: DiagramItem, undoManager: UndoManager?, refresh: @escaping () -> Void, props: [TennNode] = [] ) {
        //TODO: Need do via command\
        
        let op = CompositeOperation(self, element)
        
        let link = LinkItem(kind: .Link, name:"", source: source, target: target)
        link.properties.append(contentsOf: props)
        
        if !element.items.contains(source) {
            op.add(AddItem(self, element, source))
        }
        
        if !element.items.contains(target) {
            op.add(AddItem(self, element, target))
        }
        
        op.add(AddItem(self, element, link))
        
        execute(op, undoManager, refresh)
    }
    
    public func remove( _ element: Element, item: DiagramItem, undoManager: UndoManager?, refresh: @escaping () -> Void ) {
        let op = CompositeOperation(self, element)
        let items = element.getRelatedItems (item)
        
        for item in items {
            op.add(RemoveItem(self, element, item ))
        }
        execute(op, undoManager, refresh)
    }
    
    public func remove( _ element: Element, items deleteItems: [DiagramItem], undoManager: UndoManager?, refresh: @escaping () -> Void ) {
        let op = CompositeOperation(self, element)
        
        var items:[DiagramItem] = []
        
        for itm in deleteItems {
            let itmRels = element.getRelatedItems(itm)
            for rel in itmRels {
                if !items.contains(rel) {
                    items.append(rel)
                }
            }
        }
        
        for item in items {
            op.add(RemoveItem(self, element, item ))
        }
        execute(op, undoManager, refresh)
    }
    
    
    func makeNonModified() {
        modified = false
    }
    
    func modified(_ event: ModelEvent ) {
        modified = true
        for op in onUpdate {
            op.notifyChanges(event)
        }
    }
    
    func setProperties( _ element: Element, _ node: TennNode, undoManager: UndoManager?, refresh: @escaping () -> Void) {
        execute(ComplexUpdateElement(self, element, old: element.toTennAsProps(), new: node), undoManager, refresh)
    }
    
    
    func setProperties( _ element: Element, _ item: DiagramItem, _ node: TennNode, undoManager: UndoManager?, refresh: @escaping () -> Void) {
        execute(ComplexUpdateItem(self, element, item, old: item.toTennAsProps(), new: node ), undoManager, refresh)
    }
        
    func createProperties(_ element: Element, _ item: DiagramItem, _ node: TennNode) -> ElementOperation {
        return ComplexUpdateItem(self, element, item, old: item.toTennAsProps(), new: node )
    }
}

class AbstractUpdateValue<ValueType>: ElementOperation {
    let element: Element
    let item: DiagramItem
    let oldValue: ValueType
    let newValue: ValueType
    
    init( _ store: ElementModelStore, _ element: Element, _ item: DiagramItem, old: ValueType, new: ValueType) {
        self.element = element
        self.item = item
        self.oldValue = old
        self.newValue = new
        
        super.init(store)
    }
    
    func apply(_ value: ValueType) {
    }
    
    override func apply() {
        self.apply(newValue)
        super.apply()
    }
    override func undo() {
        self.apply(oldValue)
        super.undo()
    }
    override func collect( _ items: inout [DiagramItem:ModelEventOperation] ) {
        items[item] = .Update
    }
    override func getNotifier() -> Element {
        return element
    }
}

class AbstractUpdateElementValue<ValueType>: ElementOperation {
    let element: Element
    
    let oldValue: ValueType
    let newValue: ValueType
    
    init( _ store: ElementModelStore, _ element: Element, old: ValueType, new: ValueType) {
        self.element = element
        self.oldValue = old
        self.newValue = new
        
        super.init(store)
    }
    
    func apply(_ value: ValueType) {
    }
    
    override func apply() {
        self.apply(newValue)
        super.apply()
    }
    override func undo() {
        self.apply(oldValue)
        super.undo()
    }
    override func getNotifier() -> Element {
        return element
    }
}


class UpdatePosition: AbstractUpdateValue<CGPoint> {
    override var name:String { get {
        return "UpdatePosition: \(self.item.name) OLD:( \(self.oldValue.x), \(self.oldValue.y)" +
        "NEW:( \(self.newValue.x), \(self.newValue.y) " }
    }
    
    override func apply(_ value: CGPoint) {
        self.item.x = value.x
        self.item.y = value.y
    }
//    override func getEventKind() -> ModelEventKind {
//        return isUndoCalled ? .Structure:  .Layout
//    }
}

class UpdateName: AbstractUpdateValue<String> {
    override var name:String { get { return "UpdateName"} }
    
    override func apply(_ value: String) {
        self.item.name = value
    }
}

class ComplexUpdateItem: AbstractUpdateValue<TennNode> {
    override var name:String { get { return "UpdateItem" } }
    
    override func apply(_ value: TennNode) {
        self.item.fromTennProps(value)
    }
}

class ComplexUpdateElement: AbstractUpdateElementValue<TennNode> {
    override var name:String { get { return "UpdateElement"} }
    
    override func apply(_ value: TennNode) {
        self.element.fromTennProps(value)
    }
}

class UpdateElementName: AbstractUpdateElementValue<String> {
    override var name:String { get { return "UpdateElementName"} }
    
    override func apply(_ value: String) {
        self.element.name = value
    }
}

class AddElement: ElementOperation {
    let parent: Element
    let child: Element
    let index: Int?
    init( _ store: ElementModelStore, _ element: Element, _ child: Element, index: Int? = nil ) {
        self.parent = element
        self.child = child
        self.index = index
        super.init(store)
    }
    
    override var name:String { get { return "AddElement"} }
    override func apply() {
        if let idx = index, idx == -1 {
            self.parent.add(child)
        }
        else {
            self.parent.add(child, at: index)
        }
    }
    override func undo() {
        _ = self.parent.remove(child)
    }
    override func getNotifier() -> Element {
        return self.parent
    }
    override func collect(_ elements: inout [Element : ModelEventOperation]) {
        if !isUndoCalled {
            elements[child] = .Append
        } else {
            elements[child] = .Remove
        }
    }
    
}

class AddItem: ElementOperation {
    let parent: Element
    let item: DiagramItem
    let at: Int?
    init( _ store: ElementModelStore, _ element: Element, _ item: DiagramItem, at: Int? = nil ) {
        self.parent = element
        self.item = item
        self.at = at
        super.init(store)
    }
    override var name:String { get { return "AddItem"} }
    override func apply() {
        self.parent.add(item, at: at)
    }
    override func undo() {
        _ = self.parent.remove(item)
    }
    override func getNotifier() -> Element {
        return self.parent
    }
    override func collect( _ items: inout [DiagramItem:ModelEventOperation] ) {
        if !isUndoCalled {
            items[item] = .Append
        } else {
            items[item] = .Remove
        }
    }
}

class RemoveElement: ElementOperation {
    let parent: Element
    let child: Element
    var removeIndex:Int = -1
    init( _ store: ElementModelStore, _ element: Element, _ child: Element ) {
        self.parent = element
        self.child = child
        super.init(store)
    }
    override var name:String { get { return "RemoveElement"} }
    override func apply() {
        self.removeIndex = self.parent.remove(child)
    }
    override func undo() {
        self.parent.add(child, at: removeIndex)
    }
    override func getNotifier() -> Element {
        return self.parent
    }
    override func collect(_ elements: inout [Element : ModelEventOperation]) {
        if !isUndoCalled {
            elements[child] = .Remove
        } else {
            elements[child] = .Append
        }
    }
}

class RemoveItem: ElementOperation {
    let parent: Element
    let child: DiagramItem
    var removeIndex:Int = -1
    init( _ store: ElementModelStore, _ element: Element, _ child: DiagramItem ) {
        self.parent = element
        self.child = child
        super.init(store)
    }
    override var name:String { get { return "RemoveItem"} }
    override func apply() {
        self.removeIndex = self.parent.remove(child)
    }
    override func undo() {
        self.parent.add(child, at: removeIndex)
    }
    override func getNotifier() -> Element {
        return self.parent
    }
    override func collect( _ items: inout [DiagramItem:ModelEventOperation] ) {
        if !isUndoCalled {
            items[child] = .Remove
        } else {
            items[child] = .Append
        }
    }
}
