- **Swift packages for iOS build on the Mac:** a package whose
  `platforms:` leave out macOS, which SwiftPM on the Mac can't build, builds
  with `xcodebuild` for its platform's simulator (its `<name>-Package`
  scheme) instead of skipping, unless an app's Xcode project above it
  builds it.
