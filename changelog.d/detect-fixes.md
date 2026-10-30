- **Node.js apps without tests are JavaScript:** a `package.json` counts
  when it has runtime `dependencies`, an entry point (`main`, `bin`,
  `exports`, `module`, `browser`), `workspaces`, or a `start`, `dev`,
  `serve` or `test` script, instead of only with a `test` script, so an
  Express app is detected and audit, outdated and licenses cover it. A
  `package.json` carrying only tooling (husky, commitlint, Prettier in
  `devDependencies`) still isn't a language. `hct test` skips a package
  with no test script ("package.json has no test script") instead of
  failing.

- **Python apps with only a `requirements.txt` are Python,** where `*.py`
  files sit beside it or below (one for a docs build in a Go repository
  isn't). Their tests run with the requirements' packages
  (`uv run --with-requirements`), and skip where there are no test files.

- **Xcode apps are Swift or Objective-C:** an `*.xcodeproj` or
  `*.xcworkspace` with `*.swift` sources is Swift, with `*.m` or `*.mm`
  Objective-C, without a `Package.swift` or Podfile. On macOS `hct build`
  runs `xcodebuild` on the root workspace (CocoaPods') or project, with
  the scheme named like it, for a generic destination its supported
  platforms give (`generic/platform=macOS`, else `iOS Simulator`...), into
  `.build/xcode`, which `hct clean` removes; where it can't tell (no such
  scheme, several projects), it skips, saying to write a hook. An app with
  both Swift and Objective-C builds once, with Swift's hook. Tests skip,
  saying to write a hook with the scheme and simulator destination, and
  the app's local Swift packages aren't tested on the Mac with SwiftPM;
  nor, on macOS, is any package whose `platforms:` leaves out macOS. CocoaPods' `Pods/` and
  Carthage's `Carthage/` are vendored code to detection.
