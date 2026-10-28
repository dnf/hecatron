- **Native builds:** C, C++ and Objective-C build with CMake or Meson in
  `_build/` when the project tracks files in `build/` (configuring into it
  mixed build output with its sources, and `hct clean` deleted them); a CMake
  project that never sets up a test passes `hct test` after building instead
  of failing on "No tests were found"; installers install Meson and Ninja
  where a component builds with Meson, pkg-config (which Makefiles call)
  with C and C++, and the libraries PCRE2 and cJSON where a CMake build
  looks for them.

- **Native tests:** a CMake project that calls `enable_testing()` in a
  subdirectory rather than at its top level (Zstandard) runs its CTest tests
  from that subdirectory's build directory instead of failing on "No tests
  were found"; a Makefile project builds its default target before `make
  check` or `make test` (Redis's tests run programs only `make` builds); and
  a Meson build directory a failed setup left behind is set up again.
