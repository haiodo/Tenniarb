//
//  ExportViewController.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 16/07/2018.
//  Copyright © 2018 Andrey Sobolev. All rights reserved.
//

import Foundation
import Cocoa

enum ExportKing {
    case png
    case html
    case json
}

class ExportType: Hashable {
    static func == (lhs: ExportType, rhs: ExportType) -> Bool {
        return lhs.name == rhs.name
    }
    public var hashValue: Int {
        get {
            return self.name.hashValue
        }
    }
    
    var name: String
    var exportType: ExportKing
    init(name: String, exportType: ExportKing) {
        self.name = name
        self.exportType = exportType
    }
}

class ExportViewController: NSViewController {
    @IBOutlet weak var exportOutline: NSOutlineView!
    var delegate: ExportViewControllerDelegate?
    var hover: NSTrackingArea?
    var element: Element?
    var scene: DrawableScene?
    var viewController: ViewController?
    
    var exportTypes: [ExportType] = [
        ExportType(name:"Export as HTML", exportType: .html),
        ExportType(name:"Export as PNG", exportType: .png),
        ExportType(name:"Export as JSON", exportType: .json)
    ]
    
    func setElement(element: Element) {
        self.element = element
    }
    func setScene( scene: DrawableScene? ) {
        self.scene = scene
    }
    func setViewController(_ viewcontroller: ViewController) {
        self.viewController = viewcontroller;
    }
    
    override func viewDidLoad() {
        self.delegate = ExportViewControllerDelegate(self)
        exportOutline.delegate = delegate!
        exportOutline.dataSource = delegate!
        
        exportOutline.reloadData()
        
        self.hover = NSTrackingArea(rect: self.exportOutline.bounds, options: [NSTrackingArea.Options.activeAlways, NSTrackingArea.Options.mouseMoved], owner: self, userInfo: nil)
        
        exportOutline.addTrackingArea(self.hover!)
    }
    override func mouseMoved(with event: NSEvent) {
//        Swift.debugPrint("Move moved", event.locationInWindow)
        self.delegate?.selectAt(event)
    }
}

class ExportViewControllerDelegate: NSObject, NSOutlineViewDataSource, NSOutlineViewDelegate {
    var controller: ExportViewController
    var items: [ ExportType : NSView ] = [:]
    init(_ controller: ExportViewController) {
        self.controller = controller
    }
    
    func selectAt(_ event: NSEvent ) {
//        let wloc = event.locationInWindow
//        for (itm, vv) in items {
//            let vvv = vv.convert(vv.frame, to: controller.view)
//            Swift.debugPrint("ITM:", itm.name, " BOUNDS:", vvv)
//        }
    }
    
    
    func outlineView(_ outlineView: NSOutlineView, numberOfChildrenOfItem item: Any?) -> Int {
        if item == nil {
            return controller.exportTypes.count
        }
        return 0
    }
    func outlineView(_ outlineView: NSOutlineView, child index: Int, ofItem item: Any?) -> Any {
        if item == nil && index < controller.exportTypes.count {
            return controller.exportTypes[index]
        }
        return ""
    }
    func outlineView(_ outlineView: NSOutlineView, isItemExpandable item: Any) -> Bool {
        return false
    }
    
    
    func outlineView(_ outlineView: NSOutlineView, objectValueFor tableColumn: NSTableColumn?, byItem item: Any?) ->  Any? {
        if let el = item as? ExportType {
            return el.name
        }
        return nil
    }
    
    func outlineView(_ outlineView: NSOutlineView, viewFor viewForTableColumn: NSTableColumn?, item: Any) -> NSView? {
        if let el = item as? ExportType {
            if let view = outlineView.makeView(withIdentifier: NSUserInterfaceItemIdentifier(rawValue: "ExportCellView"), owner: self) as? NSTableCellView {
                if let textField = view.textField {
                    textField.stringValue = el.name
                }
                
                if let imageField = view.viewWithTag(0) as? NSImageView {
                    switch( el.exportType ) {
                    case .html:
                        imageField.image = NSImage.init(named: NSImage.Name.init("html_logo"))
                    case .json:
                        imageField.image = NSImage.init(named: NSImage.Name.init("json_logo"))
                    case .png:
                        imageField.image = NSImage.init(named: NSImage.Name.init("png_logo"))
                    }
                    
                }
                self.items[el] = view
                return view
            }
        }
        return nil
    }
    
    fileprivate func exportPng() {
        if let scene = self.controller.scene {
            let bounds = scene.getBounds()
            let ox = CGFloat(15)
            let oy = CGFloat(15)
            
            let scaleFactor = self.controller.viewController!.view.window!.backingScaleFactor
            
            let imgBounds = bounds.insetBy(dx: CGFloat((-1 * ox) * 2), dy: CGFloat((-1 * oy) * 2))
            
//            Swift.debugPrint("scene bounds: ", bounds)
            
            let colorSpace = CGColorSpaceCreateDeviceRGB()
            let bitmapInfo = CGBitmapInfo(rawValue: CGImageAlphaInfo.premultipliedLast.rawValue)
            let context = CGContext(
                data: nil,
                width: Int(imgBounds.width*scaleFactor),
                height: Int(imgBounds.height*scaleFactor),
                bitsPerComponent: 8,
                bytesPerRow: 0,
                space: colorSpace,
                bitmapInfo: bitmapInfo.rawValue)
            
            let nsContext = NSGraphicsContext(cgContext: context!, flipped: false)
            //                scene.offset = bounds.origin
            NSGraphicsContext.current = nsContext
            context?.scaleBy(x: scaleFactor, y: scaleFactor)
            context?.saveGState()
            scene.offset = CGPoint(x: ox + CGFloat(-1 * bounds.origin.x), y: oy + CGFloat(-1 * bounds.origin.y))
            scene.layout(bounds, bounds)
            scene.draw(context: context!)
            context?.restoreGState()
            
            //                context?.setFillColor(CGColor.black)
            //                context?.setStrokeColor(CGColor.black)
            //                context?.move(to: CGPoint(x:0, y:0))
            //                context?.addLine(to: CGPoint(x: imgBounds.width, y: imgBounds.height))
            //                context?.strokePath()
            //
            //                let info = "width: \(imgBounds.width) height: \(imgBounds.height)" as NSString
            //                info.draw(at: NSPoint(x:0, y:0), withAttributes: nil)
            
            
            let image = context!.makeImage()
            let img = NSImage(cgImage: image!, size: imgBounds.size)
            
            let controller = NSViewController()
            controller.view = NSView(frame: CGRect(x: CGFloat(0), y: CGFloat(0), width: CGFloat(imgBounds.width), height: CGFloat(imgBounds.height)))
            controller.view.autoresizesSubviews = true
            
            //                let popover = NSPopover()
            //                popover.contentViewController = controller
            //
            //                let displaySize = CGSize(
            //                    width: imgBounds.size.width,
            //                    height: imgBounds.size.height)
            //
            //                popover.contentSize = displaySize
            //
            //                popover.behavior = .transient
            //                popover.animates = false
            //
            //                let imgView = NSImageView(image: img)
            //
            //                imgView.frame = CGRect(origin: CGPoint(x:0, y:0), size:displaySize)
            //                controller.view.addSubview(imgView)
            //                popover.show(relativeTo: self.controller.viewController!.view.frame,
            //                             of: self.controller.viewController!.view, preferredEdge: NSRectEdge.minY)
            
            let mySave = NSSavePanel()
            mySave.allowedFileTypes = ["png"]
            mySave.allowsOtherFileTypes = false
            mySave.isExtensionHidden = true
            mySave.nameFieldStringValue = self.controller.element!.name + ".png"
            mySave.title = "Export diagram as PNG"
            
            mySave.begin { (result) -> Void in
                
                if result.rawValue == NSFileHandlingPanelOKButton {
                    if let filename = mySave.url {
                        if let tiff = img.tiffRepresentation, let bitmapImage = NSBitmapImageRep(data: tiff) {
                            do {
                                try bitmapImage.representation(using: .png, properties: [:])?.write(to: filename)
                            }
                            catch {
                                Swift.debugPrint("Error saving file")
                            }
                        }
                    }
                }
            }
        }
    }
    
    @objc func outlineViewSelectionDidChange(_ notification: Notification) {
        
        let selectedIndex = controller.exportOutline.selectedRow
        if let el = controller.exportOutline.item(atRow: selectedIndex) as? ExportType {
            if el.exportType == .png {
                exportPng()
            }
        }
        self.controller.dismissViewController(self.controller)
    }

}
