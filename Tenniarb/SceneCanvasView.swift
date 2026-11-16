//
//  SceneCanvasView.swift
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

/// A read-only scene document on the canvas: one built screen, drawn by the scene engine with the same
/// text metrics as the window. No selection, no drag, no inline edit and no undo.
final class SceneCanvasView: NSView {
    /// Room around the content, as geometry.Padding of the Go canvas.
    static let padding = 15.0

    private(set) var screen: Scene.Screen?
    private var painter: ScenePainter?
    private var message: String?
    /// The scale the content is drawn at; the scroll view fits the screen into itself.
    var zoom = 1.0 {
        didSet { needsDisplay = true }
    }

    override var isFlipped: Bool { true }

    /// Shows the first element of tree, as the Go window opens a document; nil clears the canvas.
    func show(_ tree: Scene.Tree?) {
        guard let tree else {
            screen = nil
            painter = nil
            message = nil
            needsDisplay = true
            return
        }
        let el = tree.main?.elems.first { $0.kind == .element }
        let built: Scene.Screen?
        if let el {
            built = tree.buildElement(el, measurer: Scene.TextMeasurer(), eval: sceneEvalVars)
        } else {
            built = tree.build("", measurer: Scene.TextMeasurer(), eval: sceneEvalVars)
        }
        screen = built
        painter = built.map(ScenePainter.init)
        message =
            built == nil
            ? tree.diagnostics.first { !$0.warn }.map { "\($0.pos): \($0.message)" } ?? "the scene does not build"
            : nil
        needsDisplay = true
    }

    /// The zoom that shows all of a cw by ch content in a vw by vh view, but never above 1, as FitCamera.
    static func fitZoom(_ cw: Double, _ ch: Double, _ vw: Double, _ vh: Double) -> Double {
        let zoom = cw > 0 && ch > 0 && vw > 0 && vh > 0 ? min(vw / cw, vh / ch) : 1
        return min(1, max(0.02, zoom))
    }

    /// The size of the content with the padding around it.
    static func contentSize(_ screen: Scene.Screen) -> NSSize {
        NSSize(width: screen.w + 2 * padding, height: screen.h + 2 * padding)
    }

    override func draw(_ dirtyRect: NSRect) {
        guard let c = NSGraphicsContext.current?.cgContext else { return }
        c.setFillColor(PreferenceConstants.preference.background)
        c.fill(bounds)
        if let screen, let painter {
            let content = SceneCanvasView.contentSize(screen)
            let ox = (bounds.width - content.width * zoom) / 2
            let oy = (bounds.height - content.height * zoom) / 2
            c.saveGState()
            c.translateBy(x: ox + SceneCanvasView.padding * zoom, y: oy + SceneCanvasView.padding * zoom)
            c.scaleBy(x: zoom, y: zoom)
            painter.paint(c)
            c.restoreGState()
        }
        if let message {
            NSAttributedString(
                string: message,
                attributes: [
                    .font: NSFont.systemFont(ofSize: 13),
                    .foregroundColor: NSColor.systemRed,
                ]
            ).draw(at: NSPoint(x: SceneCanvasView.padding, y: SceneCanvasView.padding))
        }
    }
}

/// Holds the canvas in a scroll view: the screen is fitted into it, and a screen too large for the
/// zoom floor scrolls.
final class SceneCanvasScrollView: NSScrollView {
    let canvas = SceneCanvasView(frame: .zero)

    init() {
        super.init(frame: .zero)
        drawsBackground = false
        borderType = .noBorder
        hasVerticalScroller = true
        hasHorizontalScroller = true
        documentView = canvas
    }

    required init?(coder: NSCoder) {
        fatalError("init(coder:) has not been implemented")
    }

    func show(_ tree: Scene.Tree?) {
        canvas.show(tree)
        fit()
    }

    override func layout() {
        super.layout()
        fit()
    }

    /// Sizes the document view so the whole screen fits, and scrolls when it cannot.
    private func fit() {
        let clip = contentView.bounds.size
        guard let screen = canvas.screen else {
            canvas.zoom = 1
            canvas.frame = NSRect(origin: .zero, size: clip)
            return
        }
        let content = SceneCanvasView.contentSize(screen)
        let zoom = SceneCanvasView.fitZoom(content.width, content.height, clip.width, clip.height)
        let size = NSSize(
            width: max(content.width * zoom, clip.width), height: max(content.height * zoom, clip.height))
        if canvas.zoom != zoom {
            canvas.zoom = zoom
        }
        if canvas.frame.size != size {
            canvas.frame = NSRect(origin: .zero, size: size)
        }
    }
}
