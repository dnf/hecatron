- **An Xcode project below the root is found:** a language found only in
  and below a directory holding its Xcode project gets that directory
  (`hct init` writes `implPaths`), so GoMap's `src/GoMap.xcodeproj`
  builds instead of failing with "no Package.swift here, below, or above".

- **Apps below a package build too:** Swift's build and clean act on the
  root `Package.swift`, then on each app below with an Xcode project of its
  own and Swift sources (firefox-ios' `Client.xcodeproj` and
  `Blockzilla.xcodeproj`, isowords' `App/isowords.xcodeproj`), outside
  examples, tests and vendored code (`ThirdParty/`, `third_party/`,
  `Vendor/`, `External/`, `Pods/`, `Carthage/`: firefox-ios'
  `ThirdParty/Deferred` no longer builds as an app). Finding them scans
  the directories instead of detecting every file's language, which made
  `hct clean swift` take seconds on firefox-ios. Objective-C found only
  inside such apps leaves them to Swift, and its other hooks skip at the
  root instead of failing with "no CMakeLists.txt, meson.build, configure
  or Makefile". Detection skips `ThirdParty/` and `third-party/` as it does
  `third_party/`.

- **An app beside its package builds as the app:** where an Xcode project
  with an application target sits beside a `Package.swift` (isowords'
  `App/`), Swift's build and clean are the Xcode app's where `xcodebuild`
  is installed; SwiftPM keeps the package's tests where it builds on the
  host.

- **Xcode builds read the project's files:** where hct's binary runs the
  hooks, the workspace or project, the scheme and the destination come
  from the project's files (`hct _xcode plan`), and xcodebuild decides
  only what they leave unsure. A build missing a file it needs skips
  saying which and what makes it: "the Pods aren't installed
  (Pods/Pods.xcodeproj is missing): run pod install" (eidolon), an
  `.xcconfig` the project's own setup makes (wikipedia-ios). `hct deps`
  runs `pod install` for Swift and Objective-C where there is a `Podfile`
  and CocoaPods is installed.

- **The scheme that builds the app:** an Xcode build picks the shared
  scheme whose build action builds the project's application target
  (named like the project, else the only one, else the only one for an app
  no other app embeds: not an extension's, such as focus-ios'
  `ContentBlocker`, or a watchOS app's), else the scheme of the only
  application target, else the one named like the project; with several
  left (focus-ios' `Focus` and `Klar`) it skips naming them instead of
  picking the first by name. The log says which and why.

- **Mac Catalyst apps build for the iOS Simulator:** macOS is the
  destination only for a Mac-only app (NetNewsWire), not for an iOS app
  that also supports Mac Catalyst, whose settings xcodebuild reports for
  the Mac (GoMap, which failed on `MACOSX_DEPLOYMENT_TARGET`,
  home-assistant, IceCubesApp).

- **Packages' macros and plugins:** Xcode builds pass
  `-skipMacroValidation -skipPackagePluginValidation`, so a package's
  macro builds unattended instead of failing with "Macro ... must be
  enabled before it can be used" (firefox-ios).
