- **Native tests:** a CMake project that calls `enable_testing()` in a
  subdirectory rather than at its top level (Zstandard) runs its CTest tests
  from that subdirectory's build directory instead of failing on "No tests
  were found"; a Makefile project builds its default target before `make
  check` or `make test` (Redis's tests run programs only `make` builds); and
  a Meson build directory a failed setup left behind is set up again.
