//
//  SceneAnimationTests.swift
//  TenniarbTests
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

import Tenn
import XCTest

@testable import Tenniarb

/// Keyframes and the first frame they give the picture, the values of a moment and the frame the painter
/// draws, as go/app/sceneanim.go and go/scene/anim.go.
final class SceneAnimationTests: XCTestCase {

    /// A workspace on disk: the loader resolves the files a scene mounts from it.
    private func tree(_ text: String) throws -> Scene.Tree {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-scene-animation-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        try text.write(to: root.appendingPathComponent("board.tenn"), atomically: true, encoding: .utf8)
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        return try SceneLoader(root: root).load("board.tenn")
    }

    // MARK: - Values

    /// One keyframe at offset, setting the properties in the order Go's frame helper does.
    private func frame(_ offset: Double, _ props: [String: String]) -> Scene.Frame {
        var f = Scene.Frame(percent: "\(offset)", offset: offset)
        for name in ["rotate", "scale", "opacity"] {
            guard let value = props[name] else { continue }
            let node = TennNode(kind: .IntLit, tok: TennToken(type: .intLit, literal: value))
            f.props.append(Scene.Prop(name: name, args: [node], pos: Scene.Pos(), range: Scene.Range()))
        }
        return f
    }

    /// The rotate runs through the animation and starts over at its end.
    func testTheRotateRunsThroughTheLoop() {
        let spin = Scene.Animation(
            name: "spin", duration: 2, easing: "linear", repeatCount: -1,
            frames: [frame(0, ["rotate": "0"]), frame(1, ["rotate": "360"])])
        for (at, want) in [(0.0, 0.0), (1, 180), (1.5, 270), (2, 0), (3, 180)] {
            XCTAssertEqual(spin.values(at: at, loop: false).rotate, want, accuracy: 1e-6, "at \(at)s")
        }
    }

    /// A finite animation keeps its last frame after the end.
    func testAFiniteAnimationKeepsItsLastFrame() {
        let once = Scene.Animation(
            name: "spin", duration: 1, easing: "linear", repeatCount: 1,
            frames: [frame(0, ["rotate": "0"]), frame(1, ["rotate": "360"])])
        XCTAssertEqual(once.values(at: 10, loop: false).rotate, 360, accuracy: 1e-6)
    }

    /// The scale and the opacity interpolate too, from the defaults a frame leaves out.
    func testScaleAndOpacityInterpolateFromTheirDefaults() {
        let fade = Scene.Animation(
            name: "fade", duration: 1, easing: "linear",
            frames: [
                frame(0, ["opacity": "1", "scale": "1"]), frame(1, ["opacity": "0.2", "scale": "2"]),
            ])
        let half = fade.values(at: 0.5, loop: false)
        XCTAssertEqual(half.scale, 1.5, accuracy: 1e-6)
        XCTAssertEqual(half.opacity, 0.6, accuracy: 1e-6)
        XCTAssertEqual(half.rotate, 0, "a frame that leaves rotate out holds it at rest")
    }

    /// A frame that sets one property leaves the others as the still picture has them.
    func testAFrameThatSetsOnePropertyKeepsTheOthersAtRest() {
        let fade = Scene.Animation(
            name: "fade", duration: 1, easing: "linear",
            frames: [frame(0, ["opacity": "0"]), frame(1, [:])])
        let start = fade.values(at: 0, loop: false)
        XCTAssertEqual(start.rotate, 0)
        XCTAssertEqual(start.scale, 1)
        XCTAssertEqual(start.opacity, 0)
        let end = fade.values(at: 1, loop: false)
        XCTAssertEqual(end.rotate, 0)
        XCTAssertEqual(end.scale, 1)
        XCTAssertEqual(end.opacity, 1, accuracy: 1e-6)
    }

    /// A repeat count plays the animation that many times, then the last frame stays.
    func testARepeatCountPlaysTheAnimationThatManyTimes() {
        let twice = Scene.Animation(
            name: "fade", duration: 1, easing: "linear", repeatCount: 2,
            frames: [frame(0, ["opacity": "0"]), frame(1, ["opacity": "1"])])
        XCTAssertEqual(twice.values(at: 1.5, loop: false).opacity, 0.5, accuracy: 1e-6, "the second turn")
        XCTAssertEqual(twice.values(at: 2.5, loop: false).opacity, 1, accuracy: 1e-6, "after the end the last frame stays")
    }

    /// The editor plays a finite animation again, while the export keeps its repeat.
    func testLoopPlaysAFiniteAnimationAgain() {
        let fade = Scene.Animation(
            name: "fade", duration: 1, easing: "linear", repeatCount: 1,
            frames: [frame(0, ["opacity": "0"]), frame(1, ["opacity": "1"])])
        XCTAssertEqual(fade.values(at: 2.5, loop: false).opacity, 1, accuracy: 1e-6, "as written the last frame stays")
        XCTAssertEqual(fade.values(at: 1.25, loop: true).opacity, 0.25, accuracy: 1e-6, "the editor plays it again")
    }

    /// The easings start at 0, end at 1 and stay monotone; ease-in starts slower than linear.
    func testTheEasingsStartAtZeroEndAtOneAndStayMonotone() {
        for name in ["linear", "ease", "ease-in", "ease-out", "ease-in-out"] {
            var prev = -1.0
            for i in 0...10 {
                let v = ease(name, Double(i) / 10)
                XCTAssertGreaterThanOrEqual(v, prev - 1e-9, "\(name) at \(i)")
                XCTAssertGreaterThanOrEqual(v, -1e-9, name)
                XCTAssertLessThanOrEqual(v, 1 + 1e-9, name)
                prev = v
            }
            XCTAssertEqual(ease(name, 0), 0, accuracy: 1e-9)
            XCTAssertEqual(ease(name, 1), 1, accuracy: 1e-9)
        }
        XCTAssertLessThan(ease("ease-in", 0.3), 0.3, "ease-in is slower than linear at the start")
        XCTAssertGreaterThan(ease("ease-out", 0.3), 0.3, "ease-out is faster than linear at the start")
    }

    /// An animation with no frames or no duration holds the node where the layout put it.
    func testAnEmptyAnimationHoldsTheNodeStill() {
        let empty = Scene.Animation(name: "empty", duration: 0, easing: "linear")
        XCTAssertEqual(empty.values(at: 10, loop: true).rotate, 0)
        XCTAssertEqual(empty.values(at: 10, loop: true).scale, 1)
        XCTAssertEqual(empty.values(at: 10, loop: true).opacity, 1)
    }

    func testTheFirstFrameIsTheOpacityOfTheNode() throws {
        let tree = try tree(
            """
            element "Screen" {
                animation fade {
                    0% { opacity 0 }
                    100% { opacity 1 }
                }
                item Card { width 80; height 32; animation fade 600ms ease-out }
            }
            """)
        XCTAssertTrue(tree.diagnostics.isEmpty, "\(tree.diagnostics)")
        let screen = try XCTUnwrap(tree.build("Screen", measurer: Scene.TextMeasurer(estimated: true)))
        let card = try XCTUnwrap(screen.tree.kids.first)

        XCTAssertEqual(screen.p.styles[ObjectIdentifier(card)]?.opacity, 0)
        let a = try XCTUnwrap(screen.p.anim[ObjectIdentifier(card)])
        XCTAssertEqual(a.duration, 0.6)
        XCTAssertEqual(a.easing, "ease-out")
        XCTAssertEqual(a.frames.map(\.offset), [0, 1])
    }

    func testABadKeyframeIsWarned() throws {
        let tree = try tree(
            """
            element "Screen" {
                animation broken {
                    middle { rotate 10 }
                    from { width 40 }
                    to { opacity 0.5 }
                }
                item A { width 10; height 10; animation broken 1s }
            }
            """)
        _ = try XCTUnwrap(tree.build("Screen", measurer: Scene.TextMeasurer(estimated: true)))
        let messages = tree.diagnostics.map(\.message)
        XCTAssertTrue(messages.contains("animation broken: frame \"middle\" is not from, to or a percentage"), "\(messages)")
        XCTAssertTrue(
            messages.contains("animation broken: width is a layout property and is not animated, the frame is ignored"),
            "\(messages)")
        XCTAssertTrue(tree.diagnostics.allSatisfy(\.warn), "a frame that does not read is not an error")
    }

    // MARK: - Canvas

    /// The playback entry of the canvas: a frame at a fixed moment fills the painter, a paused canvas keeps
    /// the frame it had, and a scrub shows the frame it names without playing.
    @MainActor
    func testTheCanvasPlaysFramesAndKeepsTheScrubbedOne() throws {
        let tree = try tree(
            """
            element "Screen" {
                width 200; height 100
                animation spin {
                    from { rotate 0 }
                    to { rotate 360 }
                }
                item Box { pos 20 -20; width 80; height 20; color #ff0000; animation spin 1s linear infinite }
            }
            """)
        let view = SceneCanvasView(frame: .zero)
        view.show(tree)
        let screen = try XCTUnwrap(view.screen, "\(tree.diagnostics)")
        view.frame = NSRect(origin: .zero, size: SceneCanvasView.contentSize(screen))

        XCTAssertTrue(view.hasAnimations)
        XCTAssertTrue(view.animationsPlaying, "the canvas plays by default")
        XCTAssertTrue(view.animationFrames.isEmpty, "the still picture is the first frame")

        view.tickAnimations(0.25)
        let quarter = try XCTUnwrap(view.animationFrames.values.first)
        XCTAssertEqual(quarter.rotate, 90, accuracy: 0.5, "a quarter of the turn")

        view.setAnimationsPlaying(false)
        view.tickAnimations(0.5)
        XCTAssertEqual(
            try XCTUnwrap(view.animationFrames.values.first).rotate, 90, accuracy: 0.5, "a paused canvas keeps its frame")

        view.scrubAnimations(0.5)
        let half = try XCTUnwrap(view.animationFrames.values.first)
        XCTAssertEqual(half.rotate, 180, accuracy: 0.5, "half of the turn")
        XCTAssertFalse(view.animationsPlaying, "the scrub stops the playback")
    }

    // MARK: - Painter

    /// Renders the view offscreen, as the window would, and keeps the pixels.
    @MainActor
    private func bitmap(_ view: NSView) throws -> (rep: NSBitmapImageRep, scale: CGFloat) {
        let rep = try XCTUnwrap(view.bitmapImageRepForCachingDisplay(in: view.bounds))
        view.cacheDisplay(in: view.bounds, to: rep)
        return (rep, CGFloat(rep.pixelsWide) / view.bounds.width)
    }

    /// The bytes of a rendered view, to compare two pictures.
    @MainActor
    private func pixels(_ view: NSView) throws -> Data {
        let rep = try bitmap(view).rep
        let data = try XCTUnwrap(rep.bitmapData)
        return Data(bytes: data, count: rep.bytesPerRow * rep.pixelsHigh)
    }

    /// The colour at a point of the view, y from the top as the canvas draws it.
    @MainActor
    private func color(_ bitmap: (rep: NSBitmapImageRep, scale: CGFloat), _ x: Double, _ y: Double) throws -> NSColor {
        let px = Int((x * bitmap.scale).rounded(.down))
        let py = Int((y * bitmap.scale).rounded(.down))
        return try XCTUnwrap(bitmap.rep.colorAt(x: px, y: py)?.usingColorSpace(.deviceRGB))
    }

    /// A frame override turns, grows and fades the item the painter draws, while the picture without one
    /// stays the still frame the build gave.
    @MainActor
    func testAFrameOverrideTurnsScalesAndFadesTheItem() throws {
        let tree = try tree(
            """
            element "Screen" {
                width 200; height 100
                animation spin {
                    from { rotate 0 }
                    to { rotate 360 }
                }
                item Box { pos 20 -20; width 80; height 20; color #ff0000; animation spin 1s linear infinite }
            }
            """)
        let view = SceneCanvasView(frame: .zero)
        view.show(tree)
        let screen = try XCTUnwrap(view.screen, "\(tree.diagnostics)")
        view.frame = NSRect(origin: .zero, size: SceneCanvasView.contentSize(screen))
        let painter = try XCTUnwrap(view.painter)
        let box = try XCTUnwrap(view.placed?.kids.first)
        let id = ObjectIdentifier(box.r)
        let pad = SceneCanvasView.padding

        let rest = try pixels(view)
        XCTAssertEqual(try pixels(view), rest, "the picture without a frame override does not change")
        let still = try bitmap(view)
        let corner = try color(still, pad + 120, pad + 30)
        XCTAssertNotEqual(corner, try color(still, pad + 60, pad + 30), "the point is beside the box")

        painter.frames = [id: (rotate: 90, scale: 1, opacity: 1)]
        let turned = try bitmap(view)
        XCTAssertNotEqual(try pixels(view), rest, "the item turns")
        XCTAssertNotEqual(
            try color(turned, pad + 60, pad + 60), corner, "a turned 80 by 20 box covers the point above it")

        painter.frames = [id: (rotate: 0, scale: 2, opacity: 1)]
        let grown = try bitmap(view)
        XCTAssertNotEqual(try pixels(view), rest, "the item grows")
        XCTAssertNotEqual(try color(grown, pad + 120, pad + 30), corner, "the box doubled about its center")

        painter.frames = [id: (rotate: 0, scale: 1, opacity: 0.5)]
        let faded = try bitmap(view)
        XCTAssertNotEqual(try pixels(view), rest, "the item fades")
        XCTAssertGreaterThan(
            try color(faded, pad + 60, pad + 30).greenComponent, 0.02, "the fill is drawn over the background")

        painter.frames = [:]
        XCTAssertEqual(try pixels(view), rest, "taking the frame away draws the still picture again")
    }
}
