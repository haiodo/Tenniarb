//
//  ImageUtils.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 05.09.2019.
//  Copyright © 2019 Andrey Sobolev. All rights reserved.
//

import Foundation
import Cocoa

public func displayImageInPopup(_ parentView: NSView, _ img: NSImage, _ imgBounds: CGRect) {
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
    popover.show(relativeTo: parentView.frame,
                 of: parentView, preferredEdge: NSRectEdge.minY)
    
}


public func scaleImage(_ image: CGImage, maxWidth: Float, maxHeight: Float ) -> CGImage? {
    let ciImage = CIImage(cgImage: image)
    
    var ratio: Float = 0.0
    let imageWidth = Float(image.width)
    let imageHeight = Float(image.height)
    let maxWidth: Float = maxWidth
    let maxHeight: Float = maxHeight
    
    // Get ratio (landscape or portrait)
    if (imageWidth > imageHeight) {
        ratio = maxWidth / imageWidth
    } else {
        ratio = maxHeight / imageHeight
    }
    
    // Calculate new size based on the ratio
    if ratio > 1 {
        ratio = 1
    }
        
    let filter = CIFilter(name: "CILanczosScaleTransform")!
    filter.setValue(ciImage, forKey: "inputImage")
    filter.setValue(ratio, forKey: "inputScale")
    filter.setValue(1.0, forKey: "inputAspectRatio")
    let outputImage = filter.value(forKey: "outputImage") as! CIImage
    
    let context = CIContext(options: [CIContextOption.useSoftwareRenderer: false])
    return context.createCGImage(outputImage, from: outputImage.extent)
}
