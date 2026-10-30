- **An Xcode project below the root is found:** a language found only in
  and below a directory holding its Xcode project gets that directory
  (`hct init` writes `implPaths`), so GoMap's `src/GoMap.xcodeproj`
  builds instead of failing with "no Package.swift here, below, or above".

- **Apps below a package build too:** Swift's build and clean act on the
  root `Package.swift`, then on each app below with an Xcode project of its
  own that detection takes for Swift (firefox-ios' `Client.xcodeproj` and
  `Blockzilla.xcodeproj`, isowords' `App/isowords.xcodeproj`). Objective-C
  found only inside such apps leaves them to Swift, and its other hooks
  skip at the root instead of failing with "no CMakeLists.txt, meson.build,
  configure or Makefile".

- **The scheme that builds the app:** an Xcode build picks the shared
  scheme whose build action builds the project's application target (named
  like the project, else the only one, else the first by name, preferring
  one without "Release": home-assistant's `App-Debug`), else the scheme of
  the only application target, else the one named like the project; the
  log says which and why.

- **Mac Catalyst apps build for the iOS Simulator:** macOS is the
  destination only for a native Mac app (`SDKROOT` is macOS's), not for an
  iOS app that also supports Mac Catalyst (IceCubesApp, GoMap).

- **CocoaPods:** a workspace whose `Pods/` isn't installed (a `Podfile.lock`
  without `Pods/Pods.xcodeproj`, or a project the workspace names that
  isn't there) skips its build with "the Pods aren't installed: run pod
  install" instead of failing. `hct deps` runs `pod install` for Swift and
  Objective-C where there is a `Podfile` and CocoaPods is installed.
