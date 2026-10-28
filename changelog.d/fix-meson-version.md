- **Meson versions:** where a `meson.build` asks for a newer Meson
  (`project(..., meson_version: '>=1.8.3')`) than the one installed, `hct
  doctor` reports it and `hct install c` (or `cpp`, `objectivec`) installs
  the current Meson from PyPI as a uv tool on Linux, whose distribution
  Meson lags (Ubuntu 24.04's 1.3 can't build NumPy or SciPy).
