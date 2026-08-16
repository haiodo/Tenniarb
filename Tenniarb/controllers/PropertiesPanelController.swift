//
//  PropertiesPanelController.swift
//  Tenniarb
//
//  Created by Assistant on 07/03/2026.
//  Copyright © 2026 Andrey Sobolev. All rights reserved.
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

import Cocoa

@MainActor
class PropertiesPanelController: NSObject {
    weak var viewController: ViewController?

    private(set) var textView: NSTextView!
    private(set) var scrollView: NSScrollView!
    private(set) var containerView: NSView!

    private var textViewDelegate: TextPropertiesDelegate?

    func createView(width: CGFloat = 718, height: CGFloat = 127) -> NSView {
        let stackView = NSStackView(frame: NSRect(x: 0, y: 0, width: width, height: height))
        stackView.orientation = .vertical
        stackView.alignment = .centerX
        stackView.spacing = 0
        stackView.distribution = .fill
        stackView.translatesAutoresizingMaskIntoConstraints = false
        self.containerView = stackView

        // Scroll view
        let textScroll = NSScrollView()
        textScroll.translatesAutoresizingMaskIntoConstraints = false
        textScroll.hasVerticalScroller = true
        textScroll.hasHorizontalScroller = false
        textScroll.autohidesScrollers = true
        textScroll.borderType = .noBorder
        textScroll.drawsBackground = false
        textScroll.contentView.wantsLayer = true
        textScroll.contentView.layer?.backgroundColor = NSColor(red: 1.0, green: 1.0, blue: 1.0, alpha: 0.8).cgColor
        textScroll.contentView.drawsBackground = false
        stackView.addArrangedSubview(textScroll)
        self.scrollView = textScroll

        // Text view
        let textView = TennTextView(frame: NSRect(x: 0, y: 0, width: width, height: height))
        textView.drawsBackground = false
        textView.isVerticallyResizable = true
        textView.isHorizontallyResizable = true
        textView.autoresizingMask = [.width, .height]
        textView.textContainer?.widthTracksTextView = true
        textView.textContainer?.heightTracksTextView = false
        textView.maxSize = NSSize(width: CGFloat.greatestFiniteMagnitude, height: CGFloat.greatestFiniteMagnitude)
        textView.minSize = NSSize(width: width, height: height)
        textScroll.documentView = textView
        self.textView = textView

        return stackView
    }

    func setupDelegate(_ viewController: NSViewController) {
        self.textViewDelegate = TextPropertiesDelegate(viewController as! ViewController, textView)
    }

    func updateProperties(for element: Element, item: DiagramItem? = nil) {
        textViewDelegate?.setTextValue(element, item)
    }

    func highlight() {
        textViewDelegate?.highlight()
    }

    // MARK: - Text View Proxy Methods

    func selectAll(_ sender: Any?) {
        textView?.selectAll(sender)
    }

    func setSelectedRange(_ range: NSRange) {
        textView?.setSelectedRange(range)
    }

    var isFirstResponder: Bool {
        return textView?.window?.firstResponder == textView
    }

    func makeFirstResponder() -> Bool {
        return textView?.window?.makeFirstResponder(textView) ?? false
    }
}
