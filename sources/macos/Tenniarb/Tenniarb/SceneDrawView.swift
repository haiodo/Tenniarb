//
//  SceneDrawView.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 26/05/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//


import Cocoa
import CoreText
import CoreImage


enum SceneMode {
    case Normal // A usual selection mode
    case Editing  // Title editing mode
    case Dragging // Dragging mode
    case DiagramMove
    case LineDrawing // Line dragging mode
}

class EditTitleDelegate: NSObject, NSTextFieldDelegate, NSTextDelegate {
    var view: SceneDrawView
    init( _ view: SceneDrawView) {
        self.view = view
    }
    func control(_ control: NSControl, textView: NSTextView, doCommandBy commandSelector: Selector) -> Bool {
        if commandSelector == #selector(NSView.cancelOperation(_:)) {
            self.view.commitTitleEditing(nil)
            return true
        }
        if commandSelector == #selector(NSView.insertNewline(_:)) {
            self.view.commitTitleEditing(textView)
            return true
        }
        // TODO: Resize both text and drawed item to fit value smoothly.
        
        return false
    }
}

class SceneDrawView: NSView, IElementModelListener {
    let background = CGColor(red: 253/255, green: 246/255, blue: 227/255, alpha:0.3)
    
    var store: ElementModelStore?
    
    var element: Element?
    
    var activeItems: [DiagramItem] = []
    
    var dragElements: [DiagramItem] = []
    
    var dragMap:[DiagramItem: CGPoint] = [:]
    
    var lineToPoint: CGPoint?
    var lineTarget: DiagramItem?
    
    var createIndex: Int = 1
    
    var x: CGFloat = 0
    var y: CGFloat = 0
    
    var mode: SceneMode = .Normal
    
    var editBox: NSTextField? = nil
    var editBoxItem: Drawable? = nil
    var editBoxDelegate: EditTitleDelegate?
    
    var pivotPoint: CGPoint = CGPoint(x:0, y:0)
    
    var ox: CGFloat {
        set {
            if let active = element {
                active.ox = Double(newValue)
            }
        }
        get {
            if let active = element {
                return CGFloat(active.ox)
            }
            return 0
        }
    }
    var oy: CGFloat {
        set {
            if let active = element {
                active.oy = Double(newValue)
            }
        }
        get {
            if let active = element {
                return CGFloat(active.oy)
            }
            return 0
        }
    }
    
    var mouseDownState = false
    
    var onSelection: [( [DiagramItem] ) -> Void] = []
    
    var scene: DrawableScene?
    
    var drawScheduled: Bool = false
    
    required init?(coder aDecoder: NSCoder) {
        super.init(coder: aDecoder)
        self.acceptsTouchEvents=true
    }
    
    var prevTouch: NSTouch? = nil
    
    @objc override func touchesBegan(with event: NSEvent) {
        if self.mode == .Editing || self.mode == .LineDrawing || self.mode == .Dragging {
            return
        }
        let touches = event.touches(matching: NSTouch.Phase.touching, in: self)
        if touches.count == 2 {
            prevTouch = touches.first
        }
    }
    
//    override func scrollWheel(with event: NSEvent) {
//        Swift.debugPrint("Scroll", event)
//    }
    
    func scheduleRedraw() {
        scheduleRedraw(invalidRect: nil)
    }
    var lastInvalidRect: CGRect? = nil
    fileprivate func scheduleRedraw( invalidRect: CGRect? ) {
        if !self.drawScheduled {
            drawScheduled = true
            lastInvalidRect = invalidRect
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.01, execute: {
                if let rect = invalidRect {
                    self.setNeedsDisplay(rect)
                    self.lastInvalidRect = nil
                }
                else {
                    self.needsDisplay = true
                }
                self.drawScheduled = false
            })
        }
        else {
            if lastInvalidRect != nil && invalidRect != nil {
                lastInvalidRect = lastInvalidRect!.union(invalidRect!)
            }
        }
    }
    
    @objc override func touchesMoved(with event: NSEvent) {
        if self.mode == .Editing || self.mode == .LineDrawing || self.mode == .Dragging {
            return
        }
        let touches = event.touches(matching: NSTouch.Phase.touching, in: self)
        if touches.count == 2 {
            if prevTouch == nil {
                prevTouch = touches.first
                return
            }
            var touch: NSTouch? = nil
            for t in touches {
                if t.identity.isEqual(prevTouch?.identity) {
                    touch = t
                    break
                }
            }
            if touch != nil {
                let np1 = prevTouch!.normalizedPosition
                let np2 = touch!.normalizedPosition
                
                self.ox += (np2.x-np1.x)*prevTouch!.deviceSize.width*3
                self.oy += (np2.y-np1.y)*prevTouch!.deviceSize.height*3
                
                scheduleRedraw()
            }
            prevTouch = touch
        }
    }
    
    @objc override func touchesEnded(with event: NSEvent) {
        prevTouch = nil
    }
    
    override var mouseDownCanMoveWindow: Bool {
        get {
            return false
        }
    }
    
    func onLoad() {
//        let sc1 = NSScroller()
//        sc1.frame = NSRect(x:0, y:0, width: 5, height: 200)
//        sc1.controlSize = .regular
//        sc1.arrowsPosition = .scrollerArrowsMinEnd
//        sc1.scrollerStyle = .overlay
//        self.addSubview(sc1)
    }
    
    func notifyChanges(_ evt: ModelEvent) {
        // We should be smart anought to not rebuild all drawable scene every time
        if evt.items.count > 0 {
            if let firstItem = evt.items.first(where: { (itm) in itm.kind == .Item } ) {
                if evt.element.items.contains(firstItem) {
                    setActiveItem(firstItem)
                }
                else {
                    setActiveItem(nil)
                }
            }
        }
        if evt.kind == .Structure  {
            self.buildScene()
            scheduleRedraw()
        }
        else {
            scheduleRedraw()
        }
    }
    
    public func setModel( store: ElementModelStore ) {
        if let um = self.undoManager {
            um.removeAllActions()
        }
        
        if self.store?.model != store.model {
            self.store = store
            
            self.store?.onUpdate.append( self )
        }
    }
    
    public func setActiveElement(_ elementModel: Element ) {
        
        if self.element == elementModel {
            return
        }
        // Discard any editing during switch
        self.commitTitleEditing(nil)
        
        self.element = elementModel
        self.activeItems.removeAll()
        
        
        // Center diagram to fit all items
        
        self.buildScene()
        
        if let bounds = scene?.getBounds() {
            self.ox = -1 * bounds.midX
            self.oy = -1 * bounds.midY
        }
        self.pivotPoint = CGPoint(x:0, y:0)
        
        needsDisplay = true
    }
    
    private func buildScene() {
        // We need preserve selection of previous scene
        
        var oldActiveItem: [DiagramItem] = []
        var oldEditMode: Bool = false
        if let oldScene = self.scene {
            oldActiveItem = oldScene.activeElements
            oldEditMode = oldScene.editingMode
        }
        
        let scene = DrawableScene(self.element!)
        
        if oldActiveItem.count > 0 {
            scene.updateActiveElements(oldActiveItem)
            scene.editingMode = oldEditMode
        }
    
        self.scene = scene
    }
    
    public func setActiveItem( _ element: DiagramItem? ) {
        var els: [DiagramItem] = []
        if let act = element {
            els.append(act)
        }
        
        self.setActiveItems(els)
    }
    public func setActiveItems( _ items: [DiagramItem] ) {
        if items.count == 0 && self.activeItems.count == 0 {
            return
        }
        activeItems = items
        
        for f in onSelection {
            f(items)
        }
        
        // We need to rebuild scene as active element is changed
        scene?.updateActiveElements(items)
        
        // We need to update pivot point
        
        
        if let act = items.first {
            var offset = CGFloat(100.0)
            if let dr = scene?.drawables[act] {
                offset = CGFloat(dr.getBounds().width + 10)
            }
            self.pivotPoint = CGPoint(x: act.x + offset , y: act.y)
        }
        
        scheduleRedraw()
    }
    
    fileprivate func commitTitleEditing(_ textView: NSTextView?) {
        guard let textBox = self.editBox else {
            return
        }
        if let active = self.activeItems.first {
            if let tv = textView {
                let textValue = tv.string
                if textValue.count > 0 {
                    self.store?.updateName(item: active, textValue, undoManager: self.undoManager, refresh: scheduleRedraw)
                }
            }
        }
        
        self.setNormalMode()
        textBox.removeFromSuperview()
        self.editBox = nil
        self.editBoxItem = nil
        self.window?.makeFirstResponder(self)
        scheduleRedraw()
    }
    
    fileprivate func getEditBoxBounds( item: Drawable ) -> CGRect {
        let deBounds = self.editBoxItem!.getBounds()
        let bounds = CGRect(
            x: deBounds.origin.x + scene!.offset.x,
            y: deBounds.origin.y + scene!.offset.y,
            width: max(deBounds.width, 100),
            height: deBounds.height
        )
        return bounds
    }
    fileprivate func editTitle(_ active: DiagramItem) {
        self.mode = .Editing
        scene?.editingMode = true
        guard let de = scene?.drawables[active] else {
            return
        }

        if editBox != nil {
            editBox!.removeFromSuperview()
        }
        self.editBoxItem = de
        
        let bounds = getEditBoxBounds(item: de)
        editBox = NSTextField(frame: bounds)
        if self.editBoxDelegate == nil {
            self.editBoxDelegate = EditTitleDelegate(self)
        }
        
        let style = self.scene!.sceneStyle.defaultItemStyle.copy()
        style.parseStyle(active.properties)
        
        editBox?.delegate = self.editBoxDelegate
        editBox?.stringValue = active.name
        editBox?.drawsBackground = true
        editBox?.isBordered = true
        editBox?.focusRingType = .none
        editBox?.font = NSFont.systemFont(ofSize: style.fontSize)
        
        self.addSubview(editBox!)
        
        self.window?.makeFirstResponder(editBox!)
        
        scheduleRedraw()
    }
    
    fileprivate func setNormalMode() {
        self.mode = .Normal
        scene?.editingMode = false
        self.editBoxItem = nil
        needsDisplay = true
    }
    
    func addTopItem() {
        // Add top element
        let newEl = DiagramItem(kind: .Item, name: "Untitled \(createIndex)")
        self.createIndex += 1
        
        newEl.x = pivotPoint.x
        newEl.y = pivotPoint.y
        self.store?.add(self.element!, newEl, undoManager: self.undoManager, refresh: self.scheduleRedraw)
        
        self.setActiveItem(newEl)
        scheduleRedraw()
    }
    func addNewItem(copyProps:Bool = false) {
        guard let active = self.activeItems.first else {
            self.addTopItem()
            return
        }
        if active.kind == .Item {
            // Create and add to activeEl
            let newEl = DiagramItem(kind: .Item, name: "Untitled \(createIndex)")
            self.createIndex += 1
            
            
            newEl.x = pivotPoint.x
            newEl.y = pivotPoint.y
            
            
            if copyProps {
                // Copy parent properties
                for p in active.properties {
                    newEl.properties.append(p.clone())
                }
            }
            
            self.store?.add(self.element!, source: active, target: newEl, undoManager: self.undoManager, refresh: self.scheduleRedraw)
            self.setActiveItem(newEl)
            scheduleRedraw()
        }
    }
    
    func duplicateItem() {
        var items: [DiagramItem] = []
        for active in self.activeItems {
            if active.kind == .Item {
                // Create and add to activeEl
                let newEl = DiagramItem(kind: .Item, name: active.name )
                newEl.description = active.description
                
                var offset = CGFloat(100.0)
                if let dr = scene?.drawables[active] {
                    offset = CGFloat(dr.getBounds().width + 10)
                }
                
                newEl.x = active.x + offset
                newEl.y = active.y
                
                
                // Copy parent properties
                for p in active.properties {
                    newEl.properties.append(p.clone())
                }
                
                items.append(newEl)
            }
        }
        if items.count > 0 {
            self.store?.add(self.element!, items, undoManager: self.undoManager, refresh: self.scheduleRedraw)
            self.setActiveItems(items)
            scheduleRedraw()
        }
    }
    
    func showAlert(question: String, _ button1: String = "Yes", _ button2: String = "Cancel", _ text: String = "") -> Bool {
        let alert = NSAlert()
        alert.messageText = question
        alert.informativeText = text
        alert.alertStyle = .warning
        alert.addButton(withTitle: button1)
        alert.addButton(withTitle: button2)
        return alert.runModal() == .alertFirstButtonReturn
    }
    
    func removeItem() {
        // Backspace character
        if self.activeItems.count > 0  {
            self.store?.remove(self.element!, items: self.activeItems, undoManager: self.undoManager, refresh: self.scheduleRedraw)
            scheduleRedraw()
        }
    }
    
    @IBAction override public func moveUp(_ sender: Any?) {
        ox += 0
        oy -= 200
        scheduleRedraw()
    }
    @IBAction override public func moveDown(_ sender: Any?) {
        ox += 0
        oy -= -200
        scheduleRedraw()
    }
    
    @IBAction override public func moveLeft(_ sender: Any?) {
        ox += 200
        oy -= 0
        scheduleRedraw()
    }
    
    @IBAction override public func moveRight(_ sender: Any?) {
        ox += -200
        oy -= 0
        scheduleRedraw()
    }

    @IBAction func selectAllItems(_ sender: NSMenuItem) {
        selectAllItems()
    }
    
    @IBAction func selectNoneItems(_ sender: NSMenuItem) {
        selectNoneItems()
    }
    
    
    override func keyDown(with event: NSEvent) {
        if let chars = event.characters {
            Swift.debugPrint(chars.unicodeScalars)
        }
        if event.characters == "\t" {
            addNewItem(copyProps: event.modifierFlags.contains(NSEvent.ModifierFlags.option))
        }
    
        if event.characters == "\u{0D}" {
            if let active = self.activeItems.first, active.kind == .Item  {
                editTitle(active)
            }
        }
        else if event.characters == "\u{7f}" {
            removeItem()
        }
//        else if event.characters == " " {
//            if let active = self.activeElement  {
//                if let drawable = scene?.drawables[active] {
//                    let drBounds = drawable.getBounds()
//                    let off = CGPoint(x: self.ox + bounds.midX, y: self.oy + bounds.midY)
//                    let rect = CGRect(x: drBounds.minX + off.x, y: drBounds.minY + off.y, width: drBounds.width, height: drBounds.height)
//                    showPopover(bounds: rect)
//                }
//            }
//        }
//        Swift.debugPrint("Keycode:", event.keyCode, " characters: ", event.characters)
    }
    
    public func getActiveItemBounds() -> CGRect? {
        if let active = self.activeItems.first, let drawable = scene?.drawables[active] {
            let drBounds = drawable.getBounds()
            let off = CGPoint(x: self.ox + bounds.midX, y: self.oy + bounds.midY)
            return CGRect(x: drBounds.minX + off.x, y: drBounds.minY + off.y, width: drBounds.width, height: drBounds.height)
        }
        return nil
    }
    
    private func showPopover(bounds: CGRect) {
        let controller = NSViewController()
        controller.view = NSView(frame: CGRect(x: CGFloat(0), y: CGFloat(0), width: CGFloat(100), height: CGFloat(50)))
        controller.view.autoresizesSubviews = true
        
        let popover = NSPopover()
        popover.contentViewController = controller
        popover.contentSize = controller.view.frame.size
        
        popover.behavior = .transient
        popover.animates = false
        
        // let txt = NSTextField(frame: NSMakeRect(100,50,50,22))
        let txt = NSTextField(frame: controller.view.frame)
        txt.stringValue = "Hello world"
        txt.textColor = NSColor.black.withAlphaComponent(0.95)
        controller.view.addSubview(txt)
        txt.sizeToFit()
        popover.show(relativeTo: bounds, of: self, preferredEdge: NSRectEdge.maxY)
    }
    
    public func findElement(x: CGFloat, y: CGFloat) -> ItemDrawable? {
        let point = CGPoint(x: x, y: y)
        
        if let drawable = self.scene?.find(point) {
            return drawable
        }
        
        return nil
    }
    
    
    override func mouseUp(with event: NSEvent) {
        self.updateMousePosition(event)
        
        if self.mode == .Editing {
            // No dragging allowed until editing is not done
            return
        }
        
        if self.mode == .LineDrawing {
            if let source = self.dragElements.first, let target = self.lineTarget {
                    // Create a new line if not yet pressent between elements
                    
                store?.add(element!, source:source, target: target, undoManager: self.undoManager, refresh: self.scheduleRedraw, props: [TennNode.newCommand("display", TennNode.newStrNode("arrow"))])
            }
                
            scene?.removeLineTo()
            self.lineToPoint = nil
            self.lineTarget = nil
        }
        else {
            var ops: [ElementOperation] = []
            
            for de in self.dragElements {
                if let newPos = self.dragMap.removeValue(forKey: de) {
                    let pos = newPos
                    if pos.x != de.x || pos.y != de.y {
                        ops.append(store!.createUpdatePosition(item: de, newPos: newPos))
                    }
                }
            }
            if ops.count > 0 {
                store?.compositeOperation(notifier: self.element!, undoManaget: self.undoManager, refresh: scheduleRedraw, ops)
                self.setActiveItems(self.dragElements)
                scheduleRedraw()
            }
        }
        
        self.mouseDownState = false
        for de in self.dragElements {
            self.dragMap.removeValue(forKey: de)
        }
        self.dragElements.removeAll()
        
        self.mode = .Normal
    }
    
    override func mouseDown(with event: NSEvent) {
        self.updateMousePosition(event)
        
        if self.mode == .Editing {
            // No dragging allowed until editing is not done
            self.commitTitleEditing(nil)
        }
        
        self.mouseDownState = true
        
        self.dragMap.removeAll()
        self.dragElements.removeAll()
        
        guard let drawable = findElement(x: self.x, y: self.y) else {
            self.setActiveItem(nil)
            self.mode = .DiagramMove
            return
        }
            
        if event.modifierFlags.contains(NSEvent.ModifierFlags.command) {
            // This is selection operation
            guard let itm = drawable.item else {
                return
            }
            if self.activeItems.contains(itm) {
                self.activeItems.remove(at: self.activeItems.firstIndex(of: itm)!)
            }
            else {
                self.activeItems.append(itm)
            }
            setActiveItems(self.activeItems)
            return
        }
        else {
            if let di = drawable.item, !self.activeItems.contains(di) {
                self.setActiveItem(di)
                scene?.updateActiveElements(self.activeItems)
            }
        }
        
        self.dragElements.append(contentsOf: self.activeItems)
        
        if event.modifierFlags.contains(NSEvent.ModifierFlags.control) && self.dragElements.count == 1 {
            self.mode = .LineDrawing
            self.lineToPoint = CGPoint(x: self.x, y: self.y )
        }
        else {
            self.mode = .Dragging
            for de in self.dragElements  {
                self.dragMap[de] = CGPoint(x: de.x, y: de.y)
            }
        }
    }
    
    override var acceptsFirstResponder: Bool {
        get {
            return true
        }
    }
    
    override func mouseDragged(with event: NSEvent) {
        if self.mode != .Dragging && self.mode != .DiagramMove && self.mode != .LineDrawing  {
            return
        }
        
        self.updateMousePosition(event)
        
        if self.dragElements.count > 0 {
            if let em = self.element {
                for de in dragElements {
                    if self.mode == .LineDrawing {
                        self.lineToPoint = CGPoint(x: self.x, y: self.y )
                        self.lineTarget = scene?.updateLineTo( de, self.lineToPoint! )
                        
                        scheduleRedraw()
                    }
                    else {
                        if let pos = self.dragMap[de], (de.kind == .Item || self.dragElements.count == 1) {
                            let newPos = CGPoint(x: pos.x + event.deltaX, y:pos.y - event.deltaY)
                            self.dragMap[de] = newPos
                    
                            self.store?.modified(ModelEvent(kind: .Layout, element: em))
                            let dirtyRegion = self.scene!.updateLayout(de, newPos)
                            
                            let p = CGPoint(x: self.ox + bounds.midX + dirtyRegion.origin.x-20, y: self.oy + bounds.midY + dirtyRegion.origin.y - 20)
                            scheduleRedraw(invalidRect: CGRect(origin: p, size: CGSize(width: dirtyRegion.size.width + 40, height: dirtyRegion.size.height + 40)))
                        }
                    }
                }
            }
        }
        else {
            ox += event.deltaX
            oy -= event.deltaY
            scheduleRedraw()
        }
    }
    
    func updateMousePosition(_ event: NSEvent) {
        //TODO: It is so dirty hack. Also devider positions are missied few pixel
        
        let wloc = event.locationInWindow
        
        let sv = self.superview?.superview
        
        let sv2 = sv?.superview
        
        let treeBounds = sv2?.subviews[0].bounds
        
        let textarea = sv?.subviews[1].bounds
        
        self.x = (wloc.x - treeBounds!.width - bounds.midX) - ox
        self.y = (wloc.y - bounds.midY - textarea!.height) - oy
    }
    
    override func mouseMoved(with event: NSEvent) {
        if self.mode == .Editing {
            return
        }
        self.updateMousePosition(event)
    }
    
    override func viewWillStartLiveResize() {
        if self.mode == .Editing {
            commitTitleEditing(nil)
        }
    }
    
    override func draw(_ dirtyRect: NSRect) {
        if( self.element == nil) {
            return
        }
        
        if let context = NSGraphicsContext.current?.cgContext, let scene = self.scene  {
            // Draw background
            
            context.setFillColor(background)
            context.fill(dirtyRect)
            
            scene.offset = CGPoint(x: self.ox + bounds.midX, y: self.oy + bounds.midY)
            
            let sceneDirty = CGRect(
                origin: CGPoint(x: dirtyRect.origin.x - scene.offset.x, y: dirtyRect.origin.y-scene.offset.y),
                size:dirtyRect.size
            )
            
//            context.saveGState()
            context.saveGState()
//            context.setShadow(offset: CGSize(width: 2, height:-2), blur: 4, color: CGColor(red:0,green:0,blue:0,alpha: 0.5))
//            scene.drawBox(context: context)
//            context.restoreGState()
            scene.layout(bounds, sceneDirty)
            
            // TODO: Add dirty rect filteting
            scene.draw(context: context)
            context.restoreGState()
        }
    }
    
    public func selectAllItems() {
        self.setActiveItems(self.element!.items.filter { (itm) in itm.kind == ItemKind.Item })
        scheduleRedraw()
    }
    public func selectNoneItems() {
        self.setActiveItems([])
        scheduleRedraw()
    }
}
