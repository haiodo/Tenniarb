//
//  ExportViewController.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 16/07/2018.
//  Copyright © 2018 Andrey Sobolev. All rights reserved.
//

import Foundation
import Cocoa

enum ExportKind: Int {
    case png = 1
    case pngCopy
    case html
    case htmlCopy
    case json
    case jsonCopy
    case tenn
    case separator
    case preview
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
    var exportType: ExportKind
    var imgName: String
    
    init(name: String, exportType: ExportKind, imgName: String) {
        self.name = name
        self.exportType = exportType
        self.imgName = imgName
    }
}

class ExportManager: NSObject, NSMenuDelegate {
    var viewController: ViewController?
    
    var exportTypes: [ExportType] = [
        ExportType(name:"Export as HTML", exportType: .html, imgName: "html_logo"),
        ExportType(name:"Export as PNG", exportType: .png, imgName: "png_logo"),
        ExportType(name:"Export as JSON", exportType: .json, imgName: "json_logo"),
        ExportType(name:"-", exportType: .separator, imgName: "-"),
        ExportType(name:"Copy as HTML", exportType: .htmlCopy, imgName: "html_logo"),
        ExportType(name:"Copy as PNG", exportType: .pngCopy, imgName: "png_logo"),
        ExportType(name:"Copy as JSON", exportType: .jsonCopy, imgName: "json_logo"),
        ExportType(name:"-", exportType: .separator, imgName: "-"),
        ExportType(name:"Export current to file", exportType: .tenn, imgName: "Icon"),
//        ExportType(name:"Export selection to file", exportType: .tenn, imgName: "Icon"),
//        ExportType(name:"Preview printable value", exportType: .preview, imgName: "Icon")
    ]
    
    var scene: DrawableScene? {
        get {
            return viewController?.scene.scene
        }
    }
    
    var element: Element? {
        get {
            return viewController?.selectedElement
        }
    }
    
    func setViewController(_ viewcontroller: ViewController) {
        self.viewController = viewcontroller;
    }

    func renderImage(_ scene: DrawableScene ) -> NSImage {
        let bounds = scene.getBounds()
        let ox = CGFloat(15)
        let oy = CGFloat(15)
        
        let scaleFactor = self.viewController!.view.window!.backingScaleFactor
        
        let imgBounds = bounds.insetBy(dx: CGFloat((-1 * ox) * 2), dy: CGFloat((-1 * oy) * 2))
        
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
        
        let imgSize = img.size
        
        let imgView = NSImageView(image: img)
        imgView.imageScaling = .scaleProportionallyDown
        imgView.setBoundsSize(imgSize)
        imgView.setFrameSize(imgSize)
        
        let scrView = NSScrollView()
        scrView.drawsBackground = false
        
        scrView.setFrameSize(NSSize(width: imgBounds.width, height: imgBounds.height))
        scrView.hasVerticalScroller = true
        scrView.hasHorizontalScroller = true
        scrView.documentView = imgView
        
        controller.view.addSubview(scrView)
        popover.show(relativeTo: self.viewController!.view.frame,
                     of: self.viewController!.view, preferredEdge: NSRectEdge.minY)
        
    }
    
    fileprivate func exportPng(_ writeFile: Bool) {
        if let scene = self.scene {
            let img = renderImage(scene)
            
            if let tiff = img.tiffRepresentation, let bitmapImage = NSBitmapImageRep(data: tiff) {
                if let pngData = bitmapImage.representation(using: .png, properties: [:]) {
                    if writeFile {
                        let mySave = NSSavePanel()
                        mySave.allowedFileTypes = ["png"]
                        mySave.allowsOtherFileTypes = false
                        mySave.isExtensionHidden = true
                        mySave.nameFieldStringValue = self.element!.name
                        mySave.title = "Export diagram as PNG"
                        
                        mySave.begin { (result) -> Void in
                            
                            if result.rawValue == NSFileHandlingPanelOKButton {
                                if let filename = mySave.url {
                                    do {
                                        try pngData.write(to: filename)
                                    }
                                    catch {
                                        Swift.debugPrint("Error saving file")
                                    }
                                }
                            }
                        }
                    }
                    else {
                        let pb = NSPasteboard.general
                        pb.clearContents()
                        pb.setData(pngData, forType: .png)
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
    
    func hideExtension(_ url: URL ) {
        do {
            try FileManager.default.setAttributes(
                [FileAttributeKey.extensionHidden: NSNumber(value: true)], ofItemAtPath: url.path)
            
        }
        catch _{
            Swift.print("Unable to hide extension")
        }
    }
    
    fileprivate func exportHtmlFile(_ htmlContent: String) {
        let mySave = NSSavePanel()
        mySave.allowedFileTypes = ["html"]
        mySave.allowsOtherFileTypes = false
        mySave.isExtensionHidden = true
        mySave.nameFieldStringValue = self.element!.name
        mySave.title = "Export diagram as HTML with embedded Image"
        
        mySave.begin { (result) -> Void in
            if result.rawValue == NSFileHandlingPanelOKButton {
                if let filename = mySave.url {
                    do {
                        try htmlContent.write(to: filename, atomically: true, encoding: String.Encoding.utf8)
                        self.hideExtension(filename)
                    }
                    catch {
                        Swift.debugPrint("Error saving file")
                    }
                }
            }
        }
    }
    
    func exportHtml(_ exportFile: Bool ) {
        if let scene = self.scene {
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
    func exportTenn() {
        if let element = self.element {
            
            let tennSource = element.toTennStr()
            
            let mySave = NSSavePanel()
            mySave.allowedFileTypes = ["tenn"]
            mySave.allowsOtherFileTypes = false
            mySave.isExtensionHidden = true
            mySave.nameFieldStringValue = element.name
            mySave.title = "Export element to file"
            
            mySave.begin { (result) -> Void in
                if result.rawValue == NSFileHandlingPanelOKButton {
                    if let filename = mySave.url {
                        do {
                            try tennSource.write(to: filename, atomically: true, encoding: String.Encoding.utf8)
                            self.hideExtension(filename)
                        }
                        catch {
                            Swift.debugPrint("Error saving file")
                        }
                    }
                }
            }
        }
    }
    func exportJson(_ saveFile: Bool) {
        if let element = self.element {
            
            let jsonSource = element.toSyncJson()
            
            if saveFile {
                let mySave = NSSavePanel()
                mySave.allowedFileTypes = ["json"]
                mySave.allowsOtherFileTypes = false
                mySave.isExtensionHidden = true
                mySave.nameFieldStringValue = element.name
                mySave.title = "Export element to json file"
                
                mySave.begin { (result) -> Void in
                    if result.rawValue == NSFileHandlingPanelOKButton {
                        if let filename = mySave.url {
                            do {
                                try jsonSource.write(to: filename, atomically: true, encoding: String.Encoding.utf8)
                                self.hideExtension(filename)
                            }
                            catch {
                                Swift.debugPrint("Error saving file")
                            }
                        }
                    }
                }
            }
            else {
                let pb = NSPasteboard.general
                pb.clearContents()
                pb.setString(jsonSource, forType: .string)
            }
        }
    }
    
    @objc func exportAction(_ sender: NSMenuItem ) {
        if let kind = ExportKind.init(rawValue: sender.tag) {
            switch kind {
            case .png:
                exportPng(true)
            case .pngCopy:
                exportPng(false)
            case .html:
                exportHtml(true)
            case .htmlCopy:
                exportHtml(false)
            case .json:
                exportJson(true)
            case .jsonCopy:
                exportJson(false)
            case .tenn:
                exportTenn()
            case .preview:
                if let scene = scene {
                    let img = renderImage(scene)
                    displayImageInPopup(img, CGRect(x:0, y:0, width: 1024, height: 768))
                }
            default:
                break;
            }
        }
    }
    
    func createMenu() -> NSMenu {
        let menu = NSMenu()
        menu.autoenablesItems=true
        for itm in self.exportTypes {
            if itm.exportType == .separator {
                menu.addItem(NSMenuItem.separator())
                continue;
            }
            let menuItem = NSMenuItem(title: itm.name, action: #selector(exportAction), keyEquivalent: "")
            let img = NSImage.init(named: itm.imgName)
            menuItem.image = NSImage.init(size: NSSize(width: 24, height: 24), flipped: false, drawingHandler: {
                (rect) in img?.draw(in: rect)
                return true
            })
            menuItem.target = self
            menuItem.tag = itm.exportType.rawValue
            
            menu.addItem(menuItem)
        }        
        return menu
    }
    
}