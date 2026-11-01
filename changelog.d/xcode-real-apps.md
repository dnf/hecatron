- **Xcode builds use the Xcode a `.xcode-version` file asks for:** where
  the selected Xcode isn't that version, `hct build` picks an installed one
  that is (`/Applications/Xcode_27.0.app`, `Xcode-27.0.app`) through
  `DEVELOPER_DIR`; without one, it skips when the selected Xcode is older,
  naming the version to install. A failed build of a project last upgraded
  by a newer Xcode (`LastUpgradeCheck`) says so after its errors.
- **xcodebuild's derived data is outside the repository:** in
  `~/Library/Developer/Xcode/DerivedData/hecatron-<directory>-<hash>`
  (`HECATRON_XCODE_DERIVED_DATA` names another parent), not `.build/xcode`,
  where an app's own SwiftLint build phase linted the package checkouts
  and failed (Gifski). `hct clean` removes it, and a leftover
  `.build/xcode`.
- **A scheme that runs a script target first is still the app's:** an
  aggregate or legacy target listed before the app (LoopKit/Loop's
  `Cartfile`) no longer hides the app's scheme, so `hct build` builds
  `Loop`, not its watchOS app's `WatchApp`.
- **A Swift package an Xcode app below uses is built by the app:** where
  `xcodebuild` is installed, the root `Package.swift` that an app's
  project below refers to as a local package (isowords'
  `App/isowords.xcodeproj`) is built for the app's platform by the app's
  build, not by SwiftPM for the Mac, where its iOS modules don't compile.
