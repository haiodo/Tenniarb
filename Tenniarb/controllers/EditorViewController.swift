//
//  EditorViewController.swift
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

class EditorViewController: NSObject {
    weak var viewController: ViewController?
    
    private(set) var sceneView: SceneDrawView!
    private(set) var titleField: NSTextField!
    private(set) var zoomLabel: NSTextField!
    private(set) var containerView: NSView!
    private(set) var exportSegments: NSSegmentedCell!
    
    private var exportMgr = ExportManager()
    
    func createView() -> NSView {
        let stackView = NSStackView()
        stackView.orientation = .vertical
        stackView.alignment = .centerX
        stackView.spacing = 0
        stackView.distribution = .fill
        stackView.translatesAutoresizingMaskIntoConstraints = false
        self.containerView = stackView
        
        // Title bar
        let titleBar = createTitleBar()
        stackView.addArrangedSubview(titleBar)
        
        // Scene view
        let scene = SceneDrawView(frame: .zero)
        scene.translatesAutoresizingMaskIntoConstraints = false
        stackView.addArrangedSubview(scene)
        
        scene.setContentHuggingPriority(.defaultLow, for: .vertical)
        scene.setContentCompressionResistancePriority(.defaultLow, for: .vertical)
        
        let sceneWidthConstraint = scene.widthAnchor.constraint(equalTo: stackView.widthAnchor, constant: -5)
        sceneWidthConstraint.priority = .defaultHigh
        sceneWidthConstraint.isActive = true
        
        self.sceneView = scene
        
        return stackView
    }
    
    private func createTitleBar() -> NSBox {
        let titleBar = NSBox()
        titleBar.translatesAutoresizingMaskIntoConstraints = false
        titleBar.boxType = .custom
        titleBar.borderType = .noBorder
        titleBar.titlePosition = .noTitle
        titleBar.fillColor = .clear
        
        // Disable autoresizing mask for content view to avoid constraint conflicts
        titleBar.contentView?.translatesAutoresizingMaskIntoConstraints = false
        
        let heightConstraint = titleBar.heightAnchor.constraint(equalToConstant: 34)
        heightConstraint.priority = NSLayoutConstraint.Priority(999)
        heightConstraint.isActive = true
        
        // Add constraints for content view to fill the box
        if let contentView = titleBar.contentView {
            NSLayoutConstraint.activate([
                contentView.leadingAnchor.constraint(equalTo: titleBar.leadingAnchor),
                contentView.trailingAnchor.constraint(equalTo: titleBar.trailingAnchor),
                contentView.topAnchor.constraint(equalTo: titleBar.topAnchor),
                contentView.bottomAnchor.constraint(equalTo: titleBar.bottomAnchor)
            ])
        }
        
        // Title field
        let titleField = NSTextField(string: "Window title")
        titleField.translatesAutoresizingMaskIntoConstraints = false
        titleField.isBordered = false
        titleField.isEditable = false
        titleField.isSelectable = false
        titleField.drawsBackground = false
        titleField.focusRingType = .none
        titleField.font = NSFont.systemFont(ofSize: 15)
        titleField.textColor = NSColor.labelColor
        titleField.cell?.lineBreakMode = .byClipping
        titleField.cell?.isScrollable = true
        titleBar.contentView!.addSubview(titleField)
        self.titleField = titleField
        
        NSLayoutConstraint.activate([
            titleField.leadingAnchor.constraint(equalTo: titleBar.contentView!.leadingAnchor, constant: 15),
            titleField.centerYAnchor.constraint(equalTo: titleBar.contentView!.centerYAnchor)
        ])
        
        // Zoom controls
        let zoomStack = createZoomControls()
        titleBar.contentView!.addSubview(zoomStack)
        
        // Extra buttons
        let extraSeg = createExtraButtons()
        titleBar.contentView!.addSubview(extraSeg)
        
        // Help button
        let helpSeg = createHelpButton()
        titleBar.contentView!.addSubview(helpSeg)
        
        // Export button
        let exportControl = createExportButton()
        titleBar.contentView!.addSubview(exportControl)
        
        // Layout
        // Lower priority for spacing constraints to avoid conflicts during initial layout with zero width
        let spacingPriority = NSLayoutConstraint.Priority(750)
        
        let zoomLeading = zoomStack.leadingAnchor.constraint(greaterThanOrEqualTo: titleField.trailingAnchor, constant: 20)
        zoomLeading.priority = spacingPriority
        
        let helpLeading = helpSeg.leadingAnchor.constraint(greaterThanOrEqualTo: zoomStack.trailingAnchor, constant: 16)
        helpLeading.priority = spacingPriority
        
        let exportLeading = exportControl.leadingAnchor.constraint(greaterThanOrEqualTo: helpSeg.trailingAnchor, constant: 16)
        exportLeading.priority = spacingPriority
        
        let extraLeading = extraSeg.leadingAnchor.constraint(greaterThanOrEqualTo: exportControl.trailingAnchor, constant: 16)
        extraLeading.priority = spacingPriority
        
        NSLayoutConstraint.activate([
            zoomLeading,
            zoomStack.centerYAnchor.constraint(equalTo: titleBar.contentView!.centerYAnchor),
            
            helpLeading,
            helpSeg.centerYAnchor.constraint(equalTo: titleBar.contentView!.centerYAnchor),
            
            exportLeading,
            exportControl.centerYAnchor.constraint(equalTo: titleBar.contentView!.centerYAnchor),
            
            extraLeading,
            extraSeg.trailingAnchor.constraint(equalTo: titleBar.contentView!.trailingAnchor, constant: -10),
            extraSeg.centerYAnchor.constraint(equalTo: titleBar.contentView!.centerYAnchor)
        ])
        
        return titleBar
    }
    
    private func createZoomControls() -> NSStackView {
        let zoomStack = NSStackView()
        zoomStack.translatesAutoresizingMaskIntoConstraints = false
        zoomStack.orientation = .horizontal
        zoomStack.spacing = 2
        zoomStack.alignment = .centerY
        zoomStack.distribution = .fill
        
        let zoomOutButton = NSButton()
        zoomOutButton.bezelStyle = .texturedRounded
        zoomOutButton.image = NSImage(named: NSImage.removeTemplateName)
        zoomOutButton.imagePosition = .imageOnly
        zoomOutButton.target = self
        zoomOutButton.action = #selector(zoomOutAction(_:))
        zoomStack.addArrangedSubview(zoomOutButton)
        
        let zoomLabel = NSTextField()
        zoomLabel.translatesAutoresizingMaskIntoConstraints = false
        zoomLabel.isEditable = false
        zoomLabel.isSelectable = false
        zoomLabel.isBordered = false
        zoomLabel.drawsBackground = false
        zoomLabel.alignment = .center
        zoomLabel.font = NSFont.systemFont(ofSize: 11)
        zoomLabel.textColor = NSColor.labelColor
        zoomLabel.stringValue = "100%"
        let widthConstraint = zoomLabel.widthAnchor.constraint(equalToConstant: 40)
        widthConstraint.priority = .defaultHigh
        widthConstraint.isActive = true
        zoomStack.addArrangedSubview(zoomLabel)
        self.zoomLabel = zoomLabel
        
        let zoomInButton = NSButton()
        zoomInButton.bezelStyle = .texturedRounded
        zoomInButton.image = NSImage(named: NSImage.addTemplateName)
        zoomInButton.imagePosition = .imageOnly
        zoomInButton.target = self
        zoomInButton.action = #selector(zoomInAction(_:))
        zoomStack.addArrangedSubview(zoomInButton)
        
        let resetZoomButton = NSButton()
        resetZoomButton.bezelStyle = .texturedRounded
        resetZoomButton.title = "100%"
        resetZoomButton.font = NSFont.systemFont(ofSize: 11)
        resetZoomButton.target = self
        resetZoomButton.action = #selector(resetZoomAction(_:))
        zoomStack.addArrangedSubview(resetZoomButton)
        
        return zoomStack
    }
    
    private func createExtraButtons() -> NSSegmentedControl {
        let extraSeg = NSSegmentedControl(
            labels: ["", ""],
            trackingMode: .momentary,
            target: self,
            action: #selector(extraSegmentAction(_:))
        )
        extraSeg.segmentStyle = .texturedRounded
        extraSeg.controlSize = .regular
        extraSeg.translatesAutoresizingMaskIntoConstraints = false
        extraSeg.setWidth(20, forSegment: 0)
        extraSeg.setWidth(20, forSegment: 1)
        if let addImg = NSImage(named: NSImage.addTemplateName) { extraSeg.setImage(addImg, forSegment: 0) }
        if let remImg = NSImage(named: NSImage.removeTemplateName) { extraSeg.setImage(remImg, forSegment: 1) }
        return extraSeg
    }
    
    private func createHelpButton() -> NSSegmentedControl {
        let helpSeg = NSSegmentedControl(
            labels: [""],
            trackingMode: .momentary,
            target: self,
            action: #selector(showHelp(_:))
        )
        helpSeg.segmentStyle = .texturedRounded
        helpSeg.controlSize = .regular
        helpSeg.translatesAutoresizingMaskIntoConstraints = false
        helpSeg.setWidth(20, forSegment: 0)
        if let bookmarkImg = NSImage(named: NSImage.bookmarksTemplateName) {
            helpSeg.setImage(bookmarkImg, forSegment: 0)
        }
        return helpSeg
    }
    
    private func createExportButton() -> NSSegmentedControl {
        let exportControl = NSSegmentedControl(
            labels: [""],
            trackingMode: .momentary,
            target: nil,
            action: nil
        )
        exportControl.segmentStyle = .texturedRounded
        exportControl.controlSize = .regular
        exportControl.translatesAutoresizingMaskIntoConstraints = false
        exportControl.setWidth(20, forSegment: 0)
        if let shareImg = NSImage(named: NSImage.shareTemplateName) {
            exportControl.setImage(shareImg, forSegment: 0)
        }
        self.exportSegments = exportControl.cell as? NSSegmentedCell
        return exportControl
    }
    
    func setupExportMenu(viewController: ViewController) {
        exportMgr.setViewController(viewController)
        let exportMenu = exportMgr.createMenu()
        exportSegments?.setMenu(exportMenu, forSegment: 0)
    }

    func onLoad(_ viewController: ViewController) {
        sceneView?.onLoad(viewController)
    }
    
    func onAppear() {
        sceneView?.onAppear()
    }
    
    func setModel(_ store: ElementModelStore) {
        sceneView?.setModel(store: store)
    }
    
    func setActiveElement(_ element: Element) {
        sceneView?.setActiveElement(element)
    }
    
    func scheduleRedraw() {
        sceneView?.scheduleRedraw()
    }
    
    // MARK: - Actions
    
    @objc private func zoomOutAction(_ sender: NSButton) {
        sceneView?.zoomOut(nil)
    }
    
    @objc private func zoomInAction(_ sender: NSButton) {
        sceneView?.zoomIn(nil)
    }
    
    @objc private func resetZoomAction(_ sender: NSButton) {
        sceneView?.resetZoom(nil)
    }
    
    @objc private func extraSegmentAction(_ sender: NSSegmentedControl) {
        switch sender.selectedSegment {
        case 0:
            viewController?.scene?.addNewItem()
        case 1:
            viewController?.scene?.removeItem()
        default:
            break
        }
    }
    
    @objc private func showHelp(_ sender: Any?) {
        viewController?.showHelp(nil)
    }
    
    func updateZoomLabel(_ percentage: Int) {
        zoomLabel?.stringValue = "\(percentage)%"
    }
    
    func updateTitle(_ title: String) {
        titleField?.stringValue = title
    }
    
    // MARK: - Scene Proxy Methods
    
    var activeItems: [DiagramItem] {
        return sceneView?.activeItems ?? []
    }
    
    func addNewItem() {
        sceneView?.addNewItem()
    }
    
    func removeItem() {
        sceneView?.removeItem()
    }
    
    func duplicateItem() {
        sceneView?.duplicateItem()
    }
    
    func addTopItem() {
        sceneView?.addTopItem()
    }
    
    func selectAllItems() {
        sceneView?.selectAllItems()
    }
    
    func selectNoneItems() {
        sceneView?.selectNoneItems()
    }
    
    func selectAllByKind(kind: ItemKind) {
        sceneView?.selectAllByKind(kind: kind)
    }
    
    func setActiveItem(_ item: DiagramItem?) {
        sceneView?.setActiveItem(item)
    }
    
    func centerItem(_ item: DiagramItem, _ offset: CGFloat) {
        sceneView?.centerItem(item, offset)
    }
    
    func editTitle(_ item: DiagramItem, _ type: Int) {
        // Use raw value or convert as needed
        // For now just call with Name as default
        sceneView?.editTitle(item, .Name)
    }
    
    func getSelectionBounds() -> NSRect {
        return sceneView?.getSelectionBounds() ?? .zero
    }
    
    func pasteAsItem(_ sender: NSMenuItem) {
        sceneView?.pasteAsItem(sender)
    }
    
    func pasteAsItemSet(_ sender: NSMenuItem) {
        sceneView?.pasteAsItemSet(sender)
    }
}
