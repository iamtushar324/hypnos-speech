// swift-tools-version: 5.9
import PackageDescription
let package = Package(
    name: "HypnosSpeechCore",
    platforms: [.macOS("15.0")],
    products: [.library(name: "HypnosSpeechCore", targets: ["HypnosSpeechCore"])],
    targets: [
        .target(name: "HypnosSpeechCore", path: "VoiceInk/Hypnos/Core"),
        .testTarget(name: "HypnosSpeechCoreTests", dependencies: ["HypnosSpeechCore"], path: "HypnosTests")
    ]
)
