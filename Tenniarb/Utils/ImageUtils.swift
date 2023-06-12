//
//  ImageUtils.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 05.09.2019.
//  Copyright © 2019 Andrey Sobolev. All rights reserved.
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

public func getMaxRect( maxWidth: CGFloat, maxHeight: CGFloat, imageWidth: CGFloat, imageHeight: CGFloat) -> NSRect {
    // Get ratio (landscape or portrait)
    let ratiox = maxWidth / imageWidth
    let ratioy = maxHeight / imageHeight
    
    var ratio = min(ratiox, ratioy)
        
    // Calculate new size based on the ratio
    if ratio > 1 {
        ratio = 1
    }
    return NSRect(x: 0, y: 0, width: imageWidth*ratio, height: imageHeight*ratio)
}

public func scaleImage(_ image: CGImage, maxWidth: Float, maxHeight: Float ) -> CGImage? {
    if image.width == Int(maxWidth) && image.height == Int(maxHeight) {
        return image
    }
    let ciImage = CIImage(cgImage: image)
    
    let imageWidth = Float(image.width)
    let imageHeight = Float(image.height)
    let maxWidth: Float = maxWidth
    let maxHeight: Float = maxHeight
    
    let ratiox = maxWidth / imageWidth
    let ratioy = maxHeight / imageHeight
    
    var ratio = min(ratiox, ratioy)
    
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
