#!/usr/bin/env swift
// Deterministic vector artwork for Vokiri. Run from the repository root on macOS.
import AppKit

let root = URL(fileURLWithPath: FileManager.default.currentDirectoryPath)
let catalog = root.appendingPathComponent("apps/macos/Resources/Assets.xcassets")
let icons = catalog.appendingPathComponent("VokiriIcon.appiconset")
try FileManager.default.createDirectory(at: icons, withIntermediateDirectories: true)
let info: [String: Any] = ["info": ["author": "vokiri", "version": 1]]
try JSONSerialization.data(withJSONObject: info, options: [.prettyPrinted, .sortedKeys])
    .write(to: catalog.appendingPathComponent("Contents.json"))

// A lime V with a rising voice stroke on a dark, rounded tile.
for size in [16, 32, 64, 128, 256, 512, 1024] {
    let bitmap = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: size, pixelsHigh: size,
        bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
        colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: bitmap)
    let transform = NSAffineTransform()
    transform.scale(by: CGFloat(size) / 1024)
    transform.concat()
    NSColor(srgbRed: 17/255, green: 23/255, blue: 19/255, alpha: 1).setFill()
    NSBezierPath(roundedRect: NSRect(x: 64, y: 64, width: 896, height: 896), xRadius: 196, yRadius: 196).fill()
    NSColor(srgbRed: 185/255, green: 227/255, blue: 93/255, alpha: 1).setStroke()
    let mark = NSBezierPath()
    mark.lineWidth = 88
    mark.lineCapStyle = .round
    mark.lineJoinStyle = .round
    mark.move(to: NSPoint(x: 284, y: 650))
    mark.line(to: NSPoint(x: 464, y: 340))
    mark.line(to: NSPoint(x: 664, y: 698))
    mark.stroke()
    let voice = NSBezierPath()
    voice.lineWidth = 56
    voice.lineCapStyle = .round
    voice.move(to: NSPoint(x: 758, y: 458))
    voice.line(to: NSPoint(x: 758, y: 590))
    voice.stroke()
    NSGraphicsContext.restoreGraphicsState()
    try bitmap.representation(using: .png, properties: [:])!
        .write(to: icons.appendingPathComponent("\(size).png"))
}
let images = [16, 32, 128, 256, 512].flatMap { size in
    [1, 2].map { scale in
        ["idiom": "mac", "size": "\(size)x\(size)", "scale": "\(scale)x", "filename": "\(size * scale).png"]
    }
}
try JSONSerialization.data(withJSONObject: ["images": images, "info": info["info"]!],
    options: [.prettyPrinted, .sortedKeys]).write(to: icons.appendingPathComponent("Contents.json"))
print("Generated Vokiri app icons.")
