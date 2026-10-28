- **Rust's system dependencies:** `hct install rust` installs what a
  component's crates build with, read from its Cargo.lock files: the
  libraries common `-sys` crates link (OpenSSL, GLib, GTK and WebKitGTK,
  D-Bus, ALSA...), Python's headers for PyO3, and the tools build
  dependencies run (CMake for the `cmake` crate and zlib-ng, `protoc` for
  prost-build unless protoc is vendored, Cap'n Proto, libclang for bindgen).
