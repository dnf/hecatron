- **Simulator builds are for this Mac's architecture:** `hct build` on an
  Xcode app passes `ARCHS=<this Mac's>` for a simulator destination, unless
  the project's settings exclude it, so code that only compiles for one
  architecture builds (Delta's melonDS) and the build takes less time.
