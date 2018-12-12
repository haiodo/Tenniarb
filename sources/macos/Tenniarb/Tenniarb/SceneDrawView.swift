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


class SceneDrawView: NSView, IElementModelListener, NSMenuItemValidation {
//    let background = CGColor(red: 253/255, green: 246/255, blue: 227/255, alpha:1)
    let background = CGColor(red: 0xe7/255, green: 0xe9/255, blue: 0xeb/255, alpha:1)
    let backgroundDark = CGColor(red: 0x2e/255, green: 0x2e/255, blue: 0x2e/255, alpha:1)
    
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
    
    var styleManager: StyleManager?
    
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
        styleManager = StyleManager(scene: self)
    }
    
    func notifyChanges(_ evt: ModelEvent) {
        // We should be smart anought to not rebuild all drawable scene every time
        if evt.items.count > 0, let el = self.element {
            for (k, v) in evt.items {
                if k.kind == .Item || Set([.Append, .Update]).contains(v)  {
                    if el.items.contains(k) {
                        setActiveItem(k)
                        break
                    }
                    else {
                        setActiveItem(nil)
                    }
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
        
        let darkMode = isDarkMode()
        
        let scene = DrawableScene(self.element!, darkMode: darkMode)
        
        if oldActiveItem.count > 0 {
            scene.updateActiveElements(oldActiveItem)
            scene.editingMode = oldEditMode
        }
    
        self.scene = scene
    }
    
    public func setActiveItem( _ element: DiagramItem?, immideateDraw: Bool = false ) {
        var els: [DiagramItem] = []
        if let act = element {
            els.append(act)
        }
        
        self.setActiveItems(els, immideateDraw: immideateDraw)
    }
    public func setActiveItems( _ items: [DiagramItem], immideateDraw: Bool = false ) {
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
        
        if immideateDraw {
            needsDisplay = true
        }
        else {
            scheduleRedraw()
        }
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
    
    @objc func duplicateItem() {
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
        if self.mode == .Normal {
            selectAllItems()
        }
    }
    
    @IBAction func selectNoneItems(_ sender: NSMenuItem) {
        if self.mode == .Normal {
            selectNoneItems()
        }
    }
    
    
    override func keyDown(with event: NSEvent) {        
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
            scheduleRedraw()
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
            var newPositions: [DiagramItem: CGPoint] = [:]
            
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
                        newPositions[de] = newPos
                    }
                }
            }
            if newPositions.count > 0 {
                let dirtyRegion = self.scene!.updateLayout(newPositions)
                
                let p = CGPoint(x: self.ox + bounds.midX + dirtyRegion.origin.x-20, y: self.oy + bounds.midY + dirtyRegion.origin.y - 20)
                scheduleRedraw(invalidRect: CGRect(origin: p, size: CGSize(width: dirtyRegion.size.width + 40, height: dirtyRegion.size.height + 40)))
            }
        }
        else {
            ox += event.deltaX
            oy -= event.deltaY
            
            scheduleRedraw()
        }
    }
    
    func updateMousePosition(_ event: NSEvent) {
        let wloc = event.locationInWindow
        
        let vp = self.convert(wloc, from: nil)
        let pos = CGPoint(x: vp.x - bounds.midX - ox, y: vp.y - bounds.midY - oy )
        
        self.x = pos.x
        self.y = pos.y
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
    
    fileprivate func isDarkMode() -> Bool {
        if #available(OSX 10.14, *) {
            return NSAppearance.current.name == NSAppearance.Name.darkAqua  || NSAppearance.current.name == NSAppearance.Name.vibrantDark
        }
        return false
    }
    
    override func draw(_ dirtyRect: NSRect) {
        if( self.element == nil) {
            return
        }
//        let st = NSDate()
        
        // Check if apperance changed
        
        let nDarkMode = isDarkMode()
        if self.scene?.darkMode != nDarkMode {
            buildScene()
        }
        
        if let context = NSGraphicsContext.current?.cgContext, let scene = self.scene  {
            // Draw background
            if nDarkMode {
                context.setFillColor(backgroundDark)
            }
            else {
                context.setFillColor(background)
            }

            context.fill(dirtyRect)
            
            scene.offset = CGPoint(x: self.ox + bounds.midX, y: self.oy + bounds.midY)
            
            let sceneDirty = CGRect(
                origin: CGPoint(x: dirtyRect.origin.x - scene.offset.x, y: dirtyRect.origin.y-scene.offset.y),
                size:dirtyRect.size
            )
            
            context.saveGState()
            scene.layout(bounds, sceneDirty)
            
            scene.draw(context: context)
            context.restoreGState()
            
            context.setStrokeColor(CGColor.init(red: 1, green: 0, blue: 0, alpha: 0.1))
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
    
    
    /// Selectors
    
    @objc public func removeItmAction(_ sender: NSMenuItem) {
        removeItem()
    }
    
    @objc public func addTopItm(_ sender: NSMenuItem) {
        addTopItem()
    }
    
    @objc func addNewItemNoCopy(_ sender: NSMenuItem) {
        addNewItem(copyProps: false)
    }
    // For selector
    @objc func addNewItemCopy(_ sender: NSMenuItem) {
        addNewItem(copyProps: true)
    }
    
    @objc func delete( _ sender: NSObject ) {
        removeItem()
    }
    
    
    @objc func cut( _ sender: NSObject ) {
        
    }
    
    @objc func copy( _ sender: NSObject ) {
        
    }
    
    @objc func paste( _ sender: NSObject ) {
        
    }
    
    func validateMenuItem(_ menuItem: NSMenuItem) -> Bool {
        if let action = menuItem.action {
            if action == #selector(cut) {
                return !self.activeItems.isEmpty
            }
            else if action == #selector(copy(_:)) {
                return !self.activeItems.isEmpty
            }
            else if action == #selector(delete(_:)) {
                return !self.activeItems.isEmpty
            }
            else if action == #selector(duplicateItem) {
                return !self.activeItems.isEmpty
            }
        }
        return true
    }
    
    @objc func attachImage( _ sender: NSObject ) {
        let myOpen = NSOpenPanel()
        myOpen.allowedFileTypes = ["png", "jpg", "jpeg"]
        myOpen.allowsOtherFileTypes = false
        myOpen.isExtensionHidden = true
        myOpen.nameFieldStringValue = self.element!.name
        myOpen.title = "Attach image file..."
        
        myOpen.begin { (result) -> Void in
            if result.rawValue == NSFileHandlingPanelOKButton {
                if let filename = myOpen.url {
                    do {
                        let data:NSData = try NSData(contentsOf: filename)
                        let encoded = data.base64EncodedString(options: NSData.Base64EncodingOptions.lineLength64Characters)
                        
                        if let active = self.activeItems.first {
                            let newProps = active.properties.clone()
                            
                            var imgNode = newProps.get("image")
                            if imgNode == nil {
                                imgNode = TennNode.newCommand("image", TennNode.newStrNode(filename.lastPathComponent), TennNode.newStrNode(encoded))
                                newProps.append(imgNode!)
                            }
                            else {
                                imgNode?.children = [TennNode.newIdent("image"), TennNode.newStrNode(filename.lastPathComponent), TennNode.newStrNode(encoded)]
                            }
                            self.store?.setProperties(self.element!, active, newProps.asNode(),
                                                        undoManager: self.undoManager,  refresh: {()->Void in})
                        }
                    }
                    catch {
                        Swift.debugPrint("Error saving file")
                    }
                }
            }
        }
    }
    
    
    override func menu(for event: NSEvent) -> NSMenu? {
        if event.buttonNumber != 1 {
            return nil
        }
        self.updateMousePosition(event)
        
        self.dragMap.removeAll()
        self.dragElements.removeAll()
        
        if let drawable = findElement(x: self.x, y:  self.y), let itm = drawable.item {
            if !self.activeItems.contains(itm) {
                self.setActiveItem(itm, immideateDraw: true)
            }
        }
        else {
            self.setActiveItem(nil, immideateDraw: true)
        }
        
        let addAction = NSMenuItem(title: "New item", action: #selector(addTopItm), keyEquivalent: "")
        
        if self.activeItems.count > 0 {
            let menu = NSMenu()
            
            let addLinkedAction = NSMenuItem(
                title: "New linked item", action: #selector(addNewItemNoCopy), keyEquivalent: "")
            let addLinkedCopyAction = NSMenuItem(
                title: "Linked styled item", action: #selector(addNewItemCopy), keyEquivalent: "")
            let deleteAction = NSMenuItem(
                title: "Delete", action: #selector(removeItmAction), keyEquivalent: "")
            
            let duplicateAction = NSMenuItem(
                title: "Duplicate", action: #selector(duplicateItem), keyEquivalent: "")
            
            menu.addItem(addAction)
            menu.addItem(NSMenuItem.separator())
            menu.addItem(addLinkedAction)
            menu.addItem(addLinkedCopyAction)
            menu.addItem(NSMenuItem.separator())
            let style = NSMenuItem(
                title: "Style", action: nil, keyEquivalent: "")
            menu.addItem(style)
            menu.setSubmenu(styleManager?.createMenu(), for: style)
            menu.addItem(NSMenuItem.separator())
            menu.addItem(duplicateAction)
            
            if self.activeItems.count == 1 {
                menu.addItem(NSMenuItem.separator())
                menu.addItem(NSMenuItem(
                    title: "Attach image", action: #selector(attachImage), keyEquivalent: ""))
            }
 
            menu.addItem(NSMenuItem.separator())
            menu.addItem(deleteAction)
            return menu
        }
        else {
            
            self.pivotPoint = CGPoint(x: self.x, y: self.y)
            // No items selected.
            let menu = NSMenu()
            
            let addAction = NSMenuItem(title: "New item", action: #selector(addTopItm), keyEquivalent: "")
            
            menu.addItem(addAction)
            return menu
        }
    }
}
