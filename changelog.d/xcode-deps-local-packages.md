- **`hct deps swift` leaves an Xcode project's local packages to it:** with a
  project or workspace in the directory and no `Package.swift`, it no longer
  resolves each package below against that package's own `Package.resolved`,
  which the app doesn't build from and which may be stale (openhab-ios'
  `CommonUI` failed `hct deps` this way).
