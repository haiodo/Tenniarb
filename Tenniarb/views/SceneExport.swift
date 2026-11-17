//
//  SceneExport.swift
//  Tenniarb
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.
//

import Cocoa

/// The selection and pages exports of a document: go/app/export_png.go and export_pages.go.
@MainActor
final class SceneExport {
    /// The scales of Selection as PNG, as go/app/config.go.
    static let minScale = 1
    static let maxScale = 3
    static let defaultScale = 2

    /// What a pages export wrote, for the report: total counts every page, skipped names the failed ones.
    struct Pages {
        var written = 0
        var total = 0
        var skipped: [String] = []
        var error: String?
    }

    let document: Document
    /// The canvas of the window; nil when the document has no window or holds a classic diagram.
    let canvas: SceneCanvasView?

    init(document: Document, canvas: SceneCanvasView?) {
        self.document = document
        self.canvas = canvas
    }

    // MARK: - Selection

    /// The name of the selection file; nil when the canvas has no scene to export.
    var selectionFileName: String? {
        selectionName.map(Self.selectionFileName)
    }

    /// The name the selection file takes: the selected item, else the screen.
    var selectionName: String? {
        guard let screen = canvas?.screen, canvas?.placed != nil else { return nil }
        if let path = canvas?.selection, let pl = canvas?.placed(at: path) {
            let name = pl.src?.name ?? ""
            return name.isEmpty ? (canvas?.screenNode?.name ?? screen.name) : name
        }
        // A scene exports the whole drawing when nothing is selected.
        return canvas?.screenNode?.name ?? screen.name
    }

    /// Writes the selection, or the whole screen when nothing is selected, as a PNG with no background.
    func exportSelectionPNG(to url: URL, scale: Int) -> Bool {
        guard let data = selectionPNGData(scale: scale) else { return false }
        do {
            try data.write(to: url, options: .atomic)
            return true
        } catch {
            return false
        }
    }

    func selectionPNGData(scale: Int) -> Data? {
        guard let screen = canvas?.screen, let placed = canvas?.placed else { return nil }
        let area: CGRect
        let painted: Scene.Placed
        if let path = canvas?.selection, let pl = canvas?.placed(at: path) {
            area = CGRect(x: pl.x, y: pl.y, width: pl.w, height: pl.h)
            painted = subtreeRoot(pl, screen: screen)
        } else {
            area = CGRect(x: 0, y: 0, width: screen.w, height: screen.h)
            painted = placed
        }
        guard area.width > 0, area.height > 0 else { return nil }
        let factor = CGFloat(Self.clampScale(scale))
        let w = Int(ceil(area.width * factor))
        let h = Int(ceil(area.height * factor))
        guard w > 0, h > 0, let ctx = Self.bitmap(width: w, height: h) else { return nil }
        ctx.setShouldAntialias(true)
        ctx.translateBy(x: 0, y: CGFloat(h))
        ctx.scaleBy(x: 1, y: -1)
        ctx.scaleBy(x: factor, y: factor)
        ctx.translateBy(x: -area.minX, y: -area.minY)
        ctx.clip(to: area)
        ScenePainter(screen: screen).paint(painted, ctx)
        guard let image = ctx.makeImage() else { return nil }
        return Self.pngData(image)
    }

    /// The selected item as a picture of its own: ScenePainter paints its argument as a screen root and
    /// skips that root's own picture, so the item goes under a stand-in that draws nothing.
    private func subtreeRoot(_ pl: Scene.Placed, screen: Scene.Screen) -> Scene.Placed {
        let r = Scene.RNode(kind: .item)
        screen.p.frames[ObjectIdentifier(r)] = Scene.LayoutNode()
        let root = Scene.Placed(src: nil, inst: false, free: true, parent: nil, x: 0, y: 0, w: pl.w, h: pl.h, r: r)
        root.kids = [pl]
        return root
    }

    // MARK: - Pages

    /// Writes one PNG per page into dir, numbered as go/app's pageFileName.
    func exportPagesPNG(to dir: URL) -> Pages {
        var result = Pages()
        if let tree = document.sceneTree {
            let pages = Self.scenePages(tree)
            result.total = pages.count
            for (i, node) in pages.enumerated() {
                guard let screen = tree.buildElement(node, measurer: Scene.TextMeasurer(), eval: sceneEvalVars),
                    let data = Self.pagePNG(screen)
                else {
                    result.skipped.append(node.name)
                    continue
                }
                let url = dir.appendingPathComponent(Self.pageFileName(i + 1, pages.count, node.name, ".png"))
                do {
                    try data.write(to: url, options: .atomic)
                    result.written += 1
                } catch {
                    result.skipped.append(node.name)
                }
            }
            return result
        }
        let pages = classicPages()
        result.total = pages.count
        for (i, element) in pages.enumerated() {
            guard let image = classicImage(element), let data = Self.pngData(image) else {
                result.skipped.append(element.name)
                continue
            }
            let url = dir.appendingPathComponent(Self.pageFileName(i + 1, pages.count, element.name, ".png"))
            do {
                try data.write(to: url, options: .atomic)
                result.written += 1
            } catch {
                result.skipped.append(element.name)
            }
        }
        return result
    }

    /// Writes one page per element into url; the page of an element is the size of its picture.
    func exportPagesPDF(to url: URL) -> Pages {
        var result = Pages()
        let data = NSMutableData()
        guard let consumer = CGDataConsumer(data: data as CFMutableData),
            let ctx = CGContext(consumer: consumer, mediaBox: nil, nil)
        else {
            result.error = "cannot create the PDF"
            return result
        }
        if let tree = document.sceneTree {
            let pages = Self.scenePages(tree)
            result.total = pages.count
            for node in pages {
                guard let screen = tree.buildElement(node, measurer: Scene.TextMeasurer(), eval: sceneEvalVars) else {
                    result.skipped.append(node.name)
                    continue
                }
                let size = Self.pageSize(SceneCanvasView.contentSize(screen))
                Self.beginPDFPage(ctx, size)
                ctx.saveGState()
                ctx.translateBy(x: 0, y: size.height)
                ctx.scaleBy(x: 1, y: -1)
                ctx.translateBy(x: SceneCanvasView.padding, y: SceneCanvasView.padding)
                ScenePainter(screen: screen).paint(screen.placed(), ctx)
                ctx.restoreGState()
                ctx.endPDFPage()
                result.written += 1
            }
        } else {
            let pages = classicPages()
            result.total = pages.count
            for element in pages {
                guard let image = classicImage(element) else {
                    result.skipped.append(element.name)
                    continue
                }
                let size = NSSize(width: image.width, height: image.height)
                Self.beginPDFPage(ctx, size)
                ctx.draw(image, in: CGRect(origin: .zero, size: size))
                ctx.endPDFPage()
                result.written += 1
            }
        }
        ctx.closePDF()
        // Go writes no file when every page failed: an empty PDF is not an export.
        guard result.written > 0 else { return result }
        do {
            try data.write(to: url, options: .atomic)
        } catch {
            result.error = "cannot write \(url.lastPathComponent): \(error.localizedDescription)"
            result.written = 0
        }
        return result
    }

    /// The elements of the scene file in file order, nested ones after their parent: one page each.
    private static func scenePages(_ tree: Scene.Tree) -> [Scene.Node] {
        var out: [Scene.Node] = []
        func walk(_ nodes: [Scene.Node]) {
            for n in nodes where n.kind == .element {
                out.append(n)
                walk(n.elems)
            }
        }
        walk(tree.main?.elems ?? [])
        return out
    }

    /// The elements of a classic model in tree order, as go/app's pagesOf.
    private func classicPages() -> [Element] {
        var out: [Element] = []
        func walk(_ element: Element) {
            out.append(element)
            for child in element.elements {
                walk(child)
            }
        }
        for element in document.store?.model.elements ?? [] {
            walk(element)
        }
        return out
    }

    /// The picture of one classic element, as ExportManager.renderImage draws it.
    private func classicImage(_ element: Element) -> CGImage? {
        let (image, bounds) = ExportManager.renderImage(
            element, items: nil, scaleFactor: 1, executionContext: document.store?.executionContext)
        guard bounds.width >= 1, bounds.height >= 1 else { return nil }
        var rect = CGRect(origin: .zero, size: image.size)
        return image.cgImage(forProposedRect: &rect, context: nil, hints: nil)
    }

    // MARK: - Raster

    /// A page as a PNG: the content at scale 1 with the canvas padding around it.
    private static func pagePNG(_ screen: Scene.Screen) -> Data? {
        let size = pageSize(SceneCanvasView.contentSize(screen))
        let w = Int(size.width)
        let h = Int(size.height)
        guard w > 0, h > 0, let ctx = bitmap(width: w, height: h) else { return nil }
        ctx.translateBy(x: 0, y: CGFloat(h))
        ctx.scaleBy(x: 1, y: -1)
        ctx.translateBy(x: SceneCanvasView.padding, y: SceneCanvasView.padding)
        ScenePainter(screen: screen).paint(screen.placed(), ctx)
        guard let image = ctx.makeImage() else { return nil }
        return pngData(image)
    }

    private static func pageSize(_ size: NSSize) -> NSSize {
        NSSize(width: ceil(size.width), height: ceil(size.height))
    }

    private static func beginPDFPage(_ ctx: CGContext, _ size: NSSize) {
        var rect = CGRect(origin: .zero, size: size)
        let box = Data(bytes: &rect, count: MemoryLayout<CGRect>.size)
        ctx.beginPDFPage([kCGPDFContextMediaBox as String: box] as CFDictionary)
    }

    private static func bitmap(width: Int, height: Int) -> CGContext? {
        CGContext(
            data: nil, width: width, height: height, bitsPerComponent: 8, bytesPerRow: 0,
            space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)
    }

    private static func pngData(_ image: CGImage) -> Data? {
        NSBitmapImageRep(cgImage: image).representation(using: .png, properties: [:])
    }

    // MARK: - Names

    static func clampScale(_ scale: Int) -> Int {
        min(maxScale, max(minScale, scale == 0 ? defaultScale : scale))
    }

    /// A name fit for a file, as go/app's pageName: letters, digits, - _ . stay, everything else is _.
    static func pageName(_ name: String) -> String {
        let mapped = name.trimmingCharacters(in: .whitespacesAndNewlines).map { ch -> Character in
            if ch.isLetter || ch.isNumber || ch == "-" || ch == "_" || ch == "." { return ch }
            return "_"
        }
        let trimmed = String(mapped).trimmingCharacters(in: CharacterSet(charactersIn: "._"))
        return trimmed.isEmpty ? "Untitled" : trimmed
    }

    /// The file of page i of n: 01-Name.png, the number as wide as n needs.
    static func pageFileName(_ i: Int, _ n: Int, _ name: String, _ ext: String) -> String {
        let number = String(i)
        let width = max(2, String(n).count)
        let padded = String(repeating: "0", count: max(0, width - number.count)) + number
        return padded + "-" + pageName(name) + ext
    }

    /// The default name of a selection export, as go/app's selectionFileName.
    static func selectionFileName(_ name: String) -> String {
        var out = ""
        for ch in name.trimmingCharacters(in: .whitespacesAndNewlines) {
            if "/\\:*?\"<>|".contains(ch) || (ch.asciiValue.map { $0 < 32 } ?? false) {
                out.append("_")
            } else {
                out.append(ch)
            }
        }
        let trimmed = out.trimmingCharacters(in: CharacterSet(charactersIn: ". "))
        return (trimmed.isEmpty ? "Untitled" : trimmed) + ".png"
    }
}

// MARK: - Menu actions

extension ViewController {
    /// The export of this window: its document and the canvas that shows its scene.
    private var sceneExporter: SceneExport? {
        guard let document else { return nil }
        return SceneExport(document: document, canvas: editorViewController?.sceneCanvas?.canvas)
    }

    var canExportSelection: Bool {
        sceneExporter?.selectionFileName != nil
    }

    @IBAction func exportSelectionPNG(_ sender: Any?) {
        exportSelection(scale: SceneExport.defaultScale)
    }

    @IBAction func exportSelectionPNGAtScale(_ sender: NSMenuItem) {
        exportSelection(scale: sender.tag)
    }

    private func exportSelection(scale: Int) {
        guard let exporter = sceneExporter, let name = exporter.selectionFileName else {
            reportExport("Select items to export")
            return
        }
        let panel = NSSavePanel()
        panel.allowedContentTypes = [.png]
        panel.allowsOtherFileTypes = false
        panel.isExtensionHidden = true
        panel.nameFieldStringValue = name
        panel.title = "Export selection as PNG"
        if let dir = document?.fileURL?.deletingLastPathComponent() {
            panel.directoryURL = dir
        }
        panel.begin { response in
            guard response == .OK, var url = panel.url else { return }
            if url.pathExtension.isEmpty {
                url.appendPathExtension("png")
            }
            if exporter.exportSelectionPNG(to: url, scale: scale) {
                exporter.canvas?.setMessage(nil)
            } else {
                self.reportExport("Export failed: \(url.lastPathComponent) was not written")
            }
        }
    }

    @IBAction func exportPagesPNG(_ sender: Any?) {
        guard let exporter = sceneExporter else { return }
        let panel = NSOpenPanel()
        panel.canChooseDirectories = true
        panel.canChooseFiles = false
        panel.canCreateDirectories = true
        panel.prompt = "Export"
        panel.message = "Choose a folder for the pages"
        panel.title = "Export pages as PNG"
        if let dir = document?.fileURL?.deletingLastPathComponent() {
            panel.directoryURL = dir
        }
        panel.begin { response in
            guard response == .OK, let dir = panel.url else { return }
            self.reportPages("Export pages as PNG", dir.path, exporter.exportPagesPNG(to: dir))
        }
    }

    @IBAction func exportPagesPDF(_ sender: Any?) {
        guard let exporter = sceneExporter else { return }
        let panel = NSSavePanel()
        panel.allowedContentTypes = [.pdf]
        panel.allowsOtherFileTypes = false
        panel.isExtensionHidden = true
        panel.nameFieldStringValue = (document?.sceneTree?.main?.name ?? selectedElement?.name ?? "pages") + ".pdf"
        panel.title = "Export pages as PDF"
        if let dir = document?.fileURL?.deletingLastPathComponent() {
            panel.directoryURL = dir
        }
        panel.begin { response in
            guard response == .OK, var url = panel.url else { return }
            if url.pathExtension.isEmpty {
                url.appendPathExtension("pdf")
            }
            self.reportPages("Export pages as PDF", url.path, exporter.exportPagesPDF(to: url))
        }
    }

    /// The one line of go/app's reportPages; a page with no file leaves its name in the line.
    private func reportPages(_ title: String, _ dest: String, _ result: SceneExport.Pages) {
        var text = "\(title): \(result.written) of \(result.total) pages written to \(dest)"
        if !result.skipped.isEmpty {
            text += ", skipped: \(result.skipped.joined(separator: ", "))"
        }
        if let error = result.error {
            text = "\(title): \(error)"
        }
        reportExport(text)
    }

    /// A page report goes over the canvas; a classic window has only a box.
    func reportExport(_ text: String) {
        if let scroll = editorViewController?.sceneCanvas, !scroll.isHidden {
            scroll.canvas.setMessage(text)
            return
        }
        let alert = NSAlert()
        alert.messageText = text
        alert.runModal()
    }
}
