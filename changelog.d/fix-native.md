- **Native builds:** C, C++ and Objective-C build with CMake or Meson in
  `_build/` when the project tracks files in `build/` (configuring into it
  mixed build output with its sources, and `hct clean` deleted them); a CMake
  project that never sets up a test passes `hct test` after building instead
  of failing on "No tests were found"; installers install Meson and Ninja
  where a component builds with Meson, pkg-config (which Makefiles call)
  with C and C++, and the libraries PCRE2 and cJSON where a CMake build
  looks for them.
