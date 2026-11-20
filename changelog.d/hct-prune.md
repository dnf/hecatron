- **`hct prune [lang...]`:** trims the stale part of each language's build
  output and keeps what the next build reuses, beside `hct clean`, which
  removes it all. Rust's prune removes a cargo target directory's old
  builds (each package and target's builds older than 6h next to its
  newest; `CARGO_TARGET_DIR` respected), Zig's its caches unused for 7d,
  Swift's SwiftPM configurations unused for 7d, .NET's build configurations
  unused for 7d, and Gradle's project caches for Gradle versions no longer
  used; Go, Python and Node say what their tools trim themselves.
  `--dry-run` says what each directory would free, in bytes and items;
  `--older-than DURATION` and `--max-size SIZE` set what goes; `--caches`
  also trims the per-user caches (NuGet's old package versions, Xcode's
  DerivedData of deleted projects, `uv cache prune`, `npm cache verify`).
  It never touches a directory a build holds a lock on (cargo's
  `.cargo-lock`, SwiftPM's `.build/.lock`, Zig's manifests), judges age by
  modification time, and never removes tracked files or installed
  dependencies. Each language's `prune.sh` hook is overridable like the
  others; agent and JSON output carry what each directory freed
  ([prune](docs/guide/prune.md)).
