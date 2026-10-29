- **Zig versions side by side:** a component whose `build.zig.zon` sets
  `minimum_zig_version = "0.14.0"` gets Zig 0.14 (the newest 0.14.x), not
  the default 0.17: Zig breaks compatibility between minor releases.
  `hct install` installs each version the components need next to the
  default in `~/.local/share/hecatron/toolchains/zig/`, and `hct test` and
  the other hooks run each component's own (`hct env zig -c COMPONENT`
  shows it). The `zig` on PATH stays the default. Older releases install
  too (their downloads are named differently before 0.14.1).
