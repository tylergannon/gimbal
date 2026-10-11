// swift-tools-version: 6.2
import PackageDescription
import Foundation
let probeInfo = URL(fileURLWithPath: #filePath).deletingLastPathComponent().appendingPathComponent("ProbeInfo.plist").path
let package = Package(name: "RecorderTranscription", platforms: [.macOS("27.0"), .iOS(.v26)], products: [
    .library(name: "TranscriptSource", type: .static, targets: ["TranscriptSource"]),
    .library(name: "AppleTranscription", type: .static, targets: ["AppleTranscription"]),
    .library(name: "ScriptedTranscription", type: .static, targets: ["ScriptedTranscription"]),
    .executable(name: "transcription-probe", targets: ["TranscriptionProbe"])
], targets: [
    .target(name: "TranscriptSource"),
    .target(name: "AppleTranscription", dependencies: ["TranscriptSource"]),
    .target(name: "ScriptedTranscription", dependencies: ["TranscriptSource"]),
    .executableTarget(name: "TranscriptionProbe", dependencies: ["AppleTranscription", "TranscriptSource"], linkerSettings: [.unsafeFlags(["-Xlinker", "-sectcreate", "-Xlinker", "__TEXT", "-Xlinker", "__info_plist", "-Xlinker", probeInfo], .when(platforms: [.macOS]))]),
    .testTarget(name: "AudioTests", dependencies: ["AppleTranscription", "TranscriptSource"]),
    .testTarget(name: "SyntheticTests", dependencies: ["ScriptedTranscription", "TranscriptSource"])
])
