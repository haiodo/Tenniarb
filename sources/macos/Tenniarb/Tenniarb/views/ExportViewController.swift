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
    case htmlCopy
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
    var imgName: String
    
    init(name: String, exportType: ExportKing, imgName: String) {
        self.name = name
        self.exportType = exportType
        self.imgName = imgName
    }
}

class ExportViewController: NSViewController {
    @IBOutlet weak var exportOutline: NSOutlineView!
    var delegate: ExportViewControllerDelegate?

    var element: Element?
    var scene: DrawableScene?
    var viewController: ViewController?
    
    var exportTypes: [ExportType] = [
        ExportType(name:"Copy as HTML", exportType: .htmlCopy, imgName: "html_logo"),
        ExportType(name:"Export as HTML", exportType: .html, imgName: "html_logo"),
        ExportType(name:"Export as PNG", exportType: .png, imgName: "png_logo"),
        ExportType(name:"Export as JSON", exportType: .json, imgName: "json_logo")
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
        
        exportOutline.calcSize()
        
        var width = CGFloat(0)
        var height = CGFloat(0)
        
        for i in 0...exportTypes.count {
            let frame = exportOutline.frameOfCell(atColumn: i, row: 0).size
            if frame.width > width {
                width = frame.width
            }
            if frame.height > height {
                height = frame.height
            }
        }
        height = (height + exportOutline.intercellSpacing.height ) * CGFloat(exportTypes.count) + 15
        
        self.view.frame = CGRect(origin: self.view.frame.origin, size: CGSize(width: width, height: height))
    }
}

class ExportViewControllerDelegate: NSObject, NSOutlineViewDataSource, NSOutlineViewDelegate {
    var controller: ExportViewController
    var items: [ ExportType : NSView ] = [:]
    init(_ controller: ExportViewController) {
        self.controller = controller
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
                    imageField.image = NSImage.init(named: NSImage.Name.init(el.imgName))
                }
                self.items[el] = view
                return view
            }
        }
        return nil
    }
    
    func renderImage(_ scene: DrawableScene ) -> NSImage {
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
        return img
    }
    
    func displayImageInPopup(_ img: NSImage, _ imgBounds: CGRect) {
        let controller = NSViewController()
        controller.view = NSView(frame: CGRect(x: CGFloat(0), y: CGFloat(0), width: CGFloat(imgBounds.width), height: CGFloat(imgBounds.height)))
        controller.view.autoresizesSubviews = true

        let popover = NSPopover()
        popover.contentViewController = controller

        let displaySize = CGSize(
            width: imgBounds.size.width,
            height: imgBounds.size.height)

        popover.contentSize = displaySize

        popover.behavior = .transient
        popover.animates = false

        let imgView = NSImageView(image: img)

        imgView.frame = CGRect(origin: CGPoint(x:0, y:0), size:displaySize)
        controller.view.addSubview(imgView)
        popover.show(relativeTo: self.controller.viewController!.view.frame,
                     of: self.controller.viewController!.view, preferredEdge: NSRectEdge.minY)
        
    }
    
    fileprivate func exportPng() {
        if let scene = self.controller.scene {
            let img = renderImage(scene)

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
    
    func generateHtml(_ scene: DrawableScene ) -> String? {
        let bounds = scene.getBounds()
        let img = renderImage(scene)

        if let tiff = img.tiffRepresentation, let bitmapImage = NSBitmapImageRep(data: tiff) {
            if let base64Data = bitmapImage.representation(using: .png, properties: [:])?.base64EncodedString() {
                var htmlContent = "<html>\n\t<body>\n"
                htmlContent += "\t\t<img style=\"border: 1px solid #eeeeee;\" width=\"\(bounds.width)\" height=\"\(bounds.height)\" src=\"data:image/png;base64,"
                htmlContent += base64Data
                htmlContent += "\"/>\n\t</body>\n</html>"
                
                return htmlContent
            }
        }
        return nil
    }
    fileprivate func exportHtmlFile(_ htmlContent: String) {
        let mySave = NSSavePanel()
        mySave.allowedFileTypes = ["html"]
        mySave.allowsOtherFileTypes = false
        mySave.isExtensionHidden = true
        mySave.nameFieldStringValue = self.controller.element!.name + ".html"
        mySave.title = "Export diagram as HTML with embedded Image"
        
        mySave.begin { (result) -> Void in
            if result.rawValue == NSFileHandlingPanelOKButton {
                if let filename = mySave.url {
                    do {
                        try htmlContent.write(to: filename, atomically: true, encoding: String.Encoding.utf8)
                    }
                    catch {
                        Swift.debugPrint("Error saving file")
                    }
                }
            }
        }
    }
    
    func exportHtml(_ exportFile: Bool ) {
        if let scene = self.controller.scene {
            if let htmlContent = self.generateHtml(scene) {
                if exportFile {
                    exportHtmlFile(htmlContent)
                }
                else {
                    let pb = NSPasteboard.general
                    pb.clearContents()
                    pb.setString(htmlContent, forType: .html)
                    pb.setString(htmlContent, forType: .string)
                }
            }
        }
    }
    
    @objc func outlineViewSelectionDidChange(_ notification: Notification) {
        
        let selectedIndex = controller.exportOutline.selectedRow
        if let el = controller.exportOutline.item(atRow: selectedIndex) as? ExportType {
            switch el.exportType {
            case .png:
                exportPng()
            case .html:
                exportHtml(true)
            case .htmlCopy:
                exportHtml(false)
            case .json:
                break
            }
        }
        self.controller.dismissViewController(self.controller)
    }

}
