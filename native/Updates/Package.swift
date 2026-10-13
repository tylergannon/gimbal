// swift-tools-version: 6.0
import PackageDescription

let package = Package(
  name: "GimbalUpdates",
  platforms: [.macOS(.v13), .iOS(.v17)],
  products: [.library(name: "GimbalUpdates", targets: ["GimbalUpdates"])],
  dependencies: [
    .package(url: "https://github.com/sparkle-project/Sparkle", exact: "2.10.0")
  ],
  targets: [
    .target(
      name: "GimbalUpdates",
      dependencies: [
        .product(name: "Sparkle", package: "Sparkle", condition: .when(platforms: [.macOS]))
      ]
    )
  ]
)
