// swift-tools-version:5.9
import PackageDescription

let package = Package(
  name: "Adder",
  products: [
    .library(name: "Adder", targets: ["Adder"])
  ],
  targets: [
    .target(name: "Adder", path: "swift/Sources/Adder"),
    .testTarget(name: "AdderTests", dependencies: ["Adder"], path: "swift/Tests/AdderTests"),
  ]
)
