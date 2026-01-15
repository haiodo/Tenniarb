//
//  SourcePopoverViewController.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 01/01/2018.
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

/// Programmatic popover controller that shows source text for an `Element`.
/// This replaces the storyboard-based layout so the popover can be instantiated
/// entirely in code.
class SourcePopoverViewController: NSViewController {
    var element: Element?
    // Programmatically created text view
    var textViewer: NSTextView!

    override func loadView() {
        // Root view with a reasonable default size for a source popover
        let root = NSView(frame: NSRect(x: 0, y: 0, width: 360, height: 240))
        root.wantsLayer = true
        self.view = root

        // Visual effect background (matches storyboard popover look)
        let vfx = NSVisualEffectView(frame: .zero)
        vfx.translatesAutoresizingMaskIntoConstraints = false
        vfx.blendingMode = .behindWindow
        vfx.material = .popover
        vfx.state = .followsWindowActiveState
        root.addSubview(vfx)
        NSLayoutConstraint.activate([
            vfx.leadingAnchor.constraint(equalTo: root.leadingAnchor),
            vfx.trailingAnchor.constraint(equalTo: root.trailingAnchor),
            vfx.topAnchor.constraint(equalTo: root.topAnchor),
            vfx.bottomAnchor.constraint(equalTo: root.bottomAnchor),
        ])

        // Scrollable text area
        let scroll = NSScrollView()
        scroll.translatesAutoresizingMaskIntoConstraints = false
        scroll.hasVerticalScroller = true
        scroll.hasHorizontalScroller = false
        scroll.autohidesScrollers = true
        vfx.addSubview(scroll)

        // Close button at the bottom-right
        let closeButton = NSButton(title: "Close", target: self, action: #selector(applyClose(_:)))
        closeButton.translatesAutoresizingMaskIntoConstraints = false
        closeButton.bezelStyle = .rounded
        vfx.addSubview(closeButton)

        NSLayoutConstraint.activate([
            closeButton.trailingAnchor.constraint(equalTo: vfx.trailingAnchor, constant: -8),
            closeButton.bottomAnchor.constraint(equalTo: vfx.bottomAnchor, constant: -8),

            scroll.leadingAnchor.constraint(equalTo: vfx.leadingAnchor, constant: 8),
            scroll.trailingAnchor.constraint(equalTo: vfx.trailingAnchor, constant: -8),
            scroll.topAnchor.constraint(equalTo: vfx.topAnchor, constant: 8),
            scroll.bottomAnchor.constraint(equalTo: closeButton.topAnchor, constant: -8)
        ])

        // Configure the text view to be non-editable and monospaced
        let text = NSTextView(frame: NSRect(x: 0, y: 0, width: 320, height: 180))
        text.isEditable = false
        text.isSelectable = true
        text.drawsBackground = false
        text.isVerticallyResizable = true
        text.isHorizontallyResizable = false
        text.autoresizingMask = [.width]
        if #available(macOS 10.15, *) {
            text.font = NSFont.monospacedSystemFont(ofSize: 12, weight: .regular)
        } else {
            text.font = NSFont.userFixedPitchFont(ofSize: 12)
        }

        scroll.documentView = text
        self.textViewer = text

        // Prefered popover size (presenters can still override)
        self.preferredContentSize = NSSize(width: 360, height: 240)
    }

    fileprivate func updateContent() {
        // Update source content.
        guard let active = self.element, let tv = self.textViewer else {
            return
        }
        let txtValue = active.toTennStr(includeSubElements: false, includeItems: true)
        // Use text storage to preserve formatting possibilities
        tv.textStorage?.setAttributedString(NSAttributedString(string: txtValue))
    }

    override func viewDidLoad() {
        super.viewDidLoad()
        updateContent()
    }

    @IBAction func applyClose(_ sender: Any) {
        // Dismiss the popover (or presented controller)
        dismiss(self)
    }

    public func setElement(element: Element) {
        self.element = element
        // If the view is loaded, update immediately; otherwise update when loaded.
        if isViewLoaded {
            updateContent()
        }
    }
}
