# Smoke test for zig: build and run a small program in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf 'const std = @import("std");\n\npub fn main() void {\n    std.debug.print("hello\\n", .{});\n}\n' > hello.zig
zig run hello.zig
