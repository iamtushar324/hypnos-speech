// swift-tools-version: 5.9
import PackageDescription
let package = Package(
    name: "VokiriCore",
    platforms: [.macOS("15.0")],
    products: [.library(name: "VokiriCore", targets: ["VokiriCore"])],
    targets: [
        .target(name: "VokiriCore", path: "apps/macos/Sources/Core"),
        .testTarget(name: "VokiriCoreTests", dependencies: ["VokiriCore"], path: "apps/macos/Tests")
    ]
)
