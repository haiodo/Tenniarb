//
//  SceneImage.swift
//  Tenniarb
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//  WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.  See the
//  License for the specific language governing permissions and limitations under
//  the License.
//

import Foundation
import ImageIO
import Tenn

extension Scene {
    /// A picture of an item: the file of a scene, or the inline data of a classic `image "name" @(...)`.
    /// The size is read from the header, as image.DecodeConfig does, not from the pixels.
    struct ItemImage {
        var w = 0
        var h = 0
        /// The absolute path of the file; "" for an inline picture, whose pixels are not kept.
        var path = ""
        /// How the picture fills its box: fill, cover or contain.
        var fit = "fill"
    }

    /// An inline picture drawn inside a title or a body: the box the reference @(name|width) asks for.
    struct TextImage: Equatable {
        var w = 0.0
        var h = 0.0
    }
}

/// fileImage reports whether pr is the `image "path"` of an item; the classic `image <name> @(...)` of an
/// old diagram is not one.
func fileImage(_ pr: Scene.Prop?) -> Bool {
    guard let pr else { return false }
    return pr.val != nil || pr.args.count < 2 || pr.args[1].kind != .Image
}

/// inlineImage reports whether pr is the classic `image "name" @(base64)` of an old diagram: the data is
/// the picture, the name is how the text of the item refers to it.
func inlineImage(_ pr: Scene.Prop?) -> Bool {
    guard let pr else { return false }
    return pr.val == nil && pr.args.count >= 2 && pr.args[1].kind == .Image
}

extension Scene.Prep {
    /// loadImage reads the picture of an item: the path is relative to the file that writes the property, as
    /// with `element --from-file`. A picture that cannot be drawn is a warning, so the item stays empty.
    func loadImage(_ rn: Scene.RNode) -> Scene.ItemImage? {
        let pr = prop(rn, "image")
        if classic && inlineImage(pr) {
            // An image the text names with @(name) is drawn there; one it does not name is the content of
            // the box, as a file image is.
            let name = pr?.text(0) ?? ""
            let im = inlineImages(rn)[name]
            if im == nil || namesInline(rn, name) {
                return nil
            }
            return im
        }
        guard let pr, fileImage(pr) else { return nil }
        let rel = pr.text(0) ?? ""
        if rel.isEmpty {
            t.warn(pr.pos, "image takes a path: image \"assets/photo.png\"")
            return nil
        }
        let (abs, _, inside) = t.resolveIn(pr.pos.file, rel)
        guard inside else {
            t.fail(pr.pos, "image \"\(rel)\" leaves the workspace root")
            return nil
        }
        let f: FileHandle
        do {
            f = try FileHandle(forReadingFrom: abs)
        } catch {
            // FileHandle names a missing file NSCocoaErrorDomain 4, the read errors 260; both are ENOENT.
            let code = (error as NSError).code
            if code == NSFileNoSuchFileError || code == NSFileReadNoSuchFileError {
                t.warn(pr.pos, "image \"\(rel)\": no such file, the item is drawn empty")
            } else {
                t.warn(pr.pos, "image \"\(rel)\": the file cannot be read, the item is drawn empty")
            }
            return nil
        }
        defer { try? f.close() }
        let hdr = (try? f.read(upToCount: 16)) ?? Data()
        let src = CGImageSourceCreateWithURL(abs as CFURL, [kCGImageSourceShouldCache: false] as CFDictionary)
        let cfg = config(src, hdr)
        if !cfg.why.isEmpty {
            t.warn(pr.pos, "image \"\(rel)\": \(cfg.why), the item is drawn empty")
            return nil
        }
        return Scene.ItemImage(w: cfg.w, h: cfg.h, path: abs.path, fit: fitOf(rn, pr))
    }

    /// fitOf reads fit: fill stretches the picture into the box, cover fills the box and cuts the sides,
    /// contain fits all of it inside with empty bands; it goes after the path or in a command of its own.
    func fitOf(_ rn: Scene.RNode, _ pr: Scene.Prop) -> String {
        var s = "fill"
        if let word = pr.text(1), !inlineImage(pr) {
            if word != "fit" {
                t.warn(pr.pos, "image takes a path and fit fill, cover or contain: image \"a.png\" fit cover")
                return s
            }
            s = pr.text(2) ?? ""
        }
        if let fp = prop(rn, "fit") {
            s = fp.text(0) ?? ""
        }
        switch s {
        case "fill", "cover", "contain":
            return s
        case "":
            return "fill"
        default:
            t.warn(pr.pos, "fit takes fill, cover or contain, got \"\(s)\"; fill is used")
            return "fill"
        }
    }

    /// rotation reads the static rotate of an item: degrees, clockwise, about the center of what it draws.
    func rotation(_ rn: Scene.RNode) -> Double {
        guard let pr = prop(rn, "rotate") else {
            return 0
        }
        if let v = pr.num(0) {
            return v
        }
        t.warn(pr.pos, "rotate takes degrees, as in rotate 15; ignored")
        return 0
    }

    /// inlineImages collects the classic `image "name" @(data)` of rn by name: the pictures its text draws
    /// with @(name) or @(name|width).
    func inlineImages(_ rn: Scene.RNode) -> [String: Scene.ItemImage] {
        if !classic {
            return [:]
        }
        let id = ObjectIdentifier(rn)
        if let imgs = inlines[id] {
            return imgs
        }
        var imgs: [String: Scene.ItemImage] = [:]
        for pr in rn.props where inlineImage(pr) {
            let name = pr.text(0) ?? ""
            if name.isEmpty {
                t.warn(pr.pos, "image takes a name: image \"photo\" @(...)")
                continue
            }
            if let im = loadInline(pr, name) {
                imgs[name] = im
            }
        }
        inlines[id] = imgs
        return imgs
    }

    /// loadInline reads the header of an inline picture.
    private func loadInline(_ pr: Scene.Prop, _ name: String) -> Scene.ItemImage? {
        let cfg = inlineConfig(pr.text(1) ?? "")
        if !cfg.why.isEmpty {
            t.warn(pr.pos, "image \"\(name)\": \(cfg.why), the item is drawn empty")
            return nil
        }
        return Scene.ItemImage(w: cfg.w, h: cfg.h)
    }

    /// namesInline reports whether a text the item shows refers to the inline image name: its title, its
    /// body, or the markdown file the body reads with --from-file.
    func namesInline(_ rn: Scene.RNode, _ name: String) -> Bool {
        if name.isEmpty {
            return false
        }
        let ref = "@(" + name
        for field in ["title", "body"] {
            if let pr = prop(rn, field), propRefers(pr, ref) {
                return true
            }
        }
        if let pr = prop(rn, "body") {
            let (file, ok) = bodyFileArg(pr.args)
            if ok, let text = bodyFileText(pr, file, false), text.contains(ref) {
                return true
            }
        }
        return false
    }
}

/// propRefers reports whether the text of a title or body command names ref: in an argument, an evaluated
/// value or the string of its block.
func propRefers(_ pr: Scene.Prop, _ ref: String) -> Bool {
    func has(_ nodes: [TennNode]) -> Bool {
        nodes.contains { ($0.getIdentText() ?? "").contains(ref) }
    }
    if let val = pr.val, val.contains(where: { $0.contains(ref) }) {
        return true
    }
    if has(pr.args) {
        return true
    }
    if let block = pr.block {
        for c in block.children ?? [] where has(bodyCommandArgs(c)) {
            return true
        }
    }
    return false
}

/// inlineConfig tells the pixel size of an inline picture without keeping its pixels.
func inlineConfig(_ data: String) -> (w: Int, h: Int, why: String) {
    guard let raw = decodeBase64(data) else {
        return (0, 0, "the data is not base64")
    }
    return config(CGImageSourceCreateWithData(raw as CFData, [kCGImageSourceShouldCache: false] as CFDictionary), Data(raw.prefix(16)))
}

private let base64Chars = Set("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=")

/// decodeBase64 reads the payload of an inline image; unknown characters are dropped, as Swift's
/// Data(base64Encoded:options:.ignoreUnknownCharacters) does, and the raw padding Go's decoder takes.
func decodeBase64(_ data: String) -> Data? {
    var s = String(data.filter { base64Chars.contains($0) })
    while s.hasSuffix("=") {
        s.removeLast()
    }
    if s.count % 4 == 1 {
        return nil
    }
    s += String(repeating: "=", count: (4 - s.count % 4) % 4)
    return Data(base64Encoded: s)
}

/// config is the pixel size of a picture from its header; why names what kept it from being a picture.
private func config(_ src: CGImageSource?, _ raw: Data) -> (w: Int, h: Int, why: String) {
    let b = [UInt8](raw)
    if b.count >= 12, b[4..<8].elementsEqual("ftyp".utf8) {
        let brand = String(decoding: b[8..<12], as: UTF8.self)
        // ImageIO reads an AVIF; the decoders of the scene do not, as in Go.
        if brand == "avif" || brand == "avis" {
            return (0, 0, whyNoImage(raw))
        }
    }
    guard let src, let props = CGImageSourceCopyPropertiesAtIndex(src, 0, nil) as? [CFString: Any],
        let w = (props[kCGImagePropertyPixelWidth] as? NSNumber)?.intValue,
        let h = (props[kCGImagePropertyPixelHeight] as? NSNumber)?.intValue,
        w > 0, h > 0
    else {
        return (0, 0, whyNoImage(raw))
    }
    return (w, h, "")
}

/// whyNoImage tells a format the decoders of the scene do not read from a file that is no picture at all.
private func whyNoImage(_ raw: Data) -> String {
    let b = [UInt8](raw)
    if b.count >= 12, b[4..<8].elementsEqual("ftyp".utf8) {
        let brand = String(decoding: b[8..<12], as: UTF8.self)
        if brand == "avif" || brand == "avis" {
            return "the AVIF format is not read (PNG, JPEG, GIF, WebP and TIFF are)"
        }
    }
    return "the file is not a PNG, JPEG, GIF, WebP or TIFF image"
}

/// resolveInline is the picture the reference @(name|width) names, nil when the item has no inline image of
/// that name; the style sets the width, or the width and the height with x, as go/markdown does.
func resolveInline(_ imgs: [String: Scene.ItemImage], _ ref: String) -> Scene.TextImage? {
    if imgs.isEmpty {
        return nil
    }
    var name = ref
    var style = ""
    if let bar = ref.firstIndex(of: "|") {
        name = String(ref[..<bar])
        style = String(ref[ref.index(after: bar)...])
    }
    guard let im = imgs[name] else {
        return nil
    }
    var w = Double(im.w)
    var h = Double(im.h)
    if !style.isEmpty {
        var width = style
        var height = ""
        if let x = style.firstIndex(of: "x") {
            width = style[..<x].trimmingCharacters(in: .whitespaces)
            height = style[style.index(after: x)...].trimmingCharacters(in: .whitespaces)
        }
        if let v = Int(width) {
            w = Double(v)
        }
        if let v = Int(height) {
            h = Double(v)
        }
        // One number is the aspect scale of the picture, so the other side keeps its shape.
        if width.isEmpty || height.isEmpty {
            let r = maxRect(w, h, Double(im.w), Double(im.h))
            w = r.w
            h = r.h
        }
    }
    return Scene.TextImage(w: w, h: h)
}

/// maxRect fits a picture into maxWidth by maxHeight and enlarges it, as markdown.GetMaxRect.
private func maxRect(_ maxWidth: Double, _ maxHeight: Double, _ imageWidth: Double, _ imageHeight: Double)
    -> (w: Double, h: Double)
{
    let ratiox = maxWidth / imageWidth
    let ratioy = maxHeight / imageHeight
    // Swift.min differs from this comparison on NaN, as Go notes.
    var ratio = ratiox
    if ratioy < ratiox {
        ratio = ratioy
    }
    if ratio > 1 {
        ratio = 1
    }
    return (imageWidth * ratio, imageHeight * ratio)
}

/// splitTitleImages cuts a title at the @(name) references of the inline images of its item; nil when the
/// title names none.
func splitTitleImages(_ title: String, _ imgs: [String: Scene.ItemImage]) -> [Scene.TitlePart]? {
    if imgs.isEmpty {
        return nil
    }
    let chars = Array(title)
    var parts: [Scene.TitlePart] = []
    var plain = 0
    var found = false
    var i = 0
    while i + 1 < chars.count {
        if chars[i] != "@" || chars[i + 1] != "(" {
            i += 1
            continue
        }
        let from = i + 2
        guard let close = chars[from...].firstIndex(of: ")") else {
            break
        }
        guard let img = resolveInline(imgs, String(chars[from..<close])) else {
            i += 2
            continue
        }
        if i > plain {
            parts.append(Scene.TitlePart(text: String(chars[plain..<i])))
        }
        parts.append(Scene.TitlePart(img: img))
        i = close + 1
        plain = i
        found = true
    }
    if !found {
        return nil
    }
    if plain < chars.count {
        parts.append(Scene.TitlePart(text: String(chars[plain...])))
    }
    return parts
}
