//
//  ResourcesViewController.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 05/11/2018.
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

import Cocoa
import Foundation

class ResourcesViewController: NSViewController {

    var resources: NSOutlineView!

    override func loadView() {
        // Build the resources UI programmatically so this controller doesn't require a storyboard
        let root = NSView(frame: NSRect(x: 0, y: 0, width: 300, height: 220))
        root.wantsLayer = true
        self.view = root

        // Visual effect background for popover-like appearance
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

        // Simple segmented control for operations
        let segmented = NSSegmentedControl(
            labels: ["Add", "Remove"], trackingMode: .momentary, target: self, action: #selector(handleResourceSegment(_:)))
        segmented.translatesAutoresizingMaskIntoConstraints = false
        segmented.segmentStyle = .texturedRounded
        vfx.addSubview(segmented)

        // Scrollable outline view
        let scroll = NSScrollView(frame: .zero)
        scroll.translatesAutoresizingMaskIntoConstraints = false
        scroll.hasVerticalScroller = true
        scroll.autohidesScrollers = true
        vfx.addSubview(scroll)

        NSLayoutConstraint.activate([
            segmented.leadingAnchor.constraint(equalTo: vfx.leadingAnchor, constant: 8),
            segmented.topAnchor.constraint(equalTo: vfx.topAnchor, constant: 8),

            scroll.leadingAnchor.constraint(equalTo: vfx.leadingAnchor, constant: 8),
            scroll.trailingAnchor.constraint(equalTo: vfx.trailingAnchor, constant: -8),
            scroll.topAnchor.constraint(equalTo: segmented.bottomAnchor, constant: 8),
            scroll.bottomAnchor.constraint(equalTo: vfx.bottomAnchor, constant: -8),
        ])

        let outline = NSOutlineView(frame: .zero)
        outline.translatesAutoresizingMaskIntoConstraints = false
        let col = NSTableColumn(identifier: NSUserInterfaceItemIdentifier(rawValue: "ResourceColumn"))
        col.title = "Resource"
        outline.addTableColumn(col)
        outline.outlineTableColumn = col
        outline.headerView = nil
        outline.rowHeight = 20
        scroll.documentView = outline

        self.resources = outline
    }

    public override func viewDidLoad() {
        super.viewDidLoad()
        // Additional setup if needed (delegate/datasource can be assigned here)
    }

    @objc func handleResourceSegment(_ sender: NSSegmentedControl) {
        // Bridge segmented control to existing IBAction signature (NSSegmentedCell) for compatibility.
        if let cell = sender.cell as? NSSegmentedCell {
            handleResourceOperation(cell)
        }
    }

    @IBAction func handleResourceOperation(_ sender: NSSegmentedCell) {
        // Implement resource operations here.
        // Example: switch on sender.selectedSegment to add/remove resource entries.
        // Left intentionally minimal; the original method was empty and this preserves behavior.
    }
}
