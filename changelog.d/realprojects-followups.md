- **`python` for builds that call it:** where there is only `python3`,
  Python's installer (and the generated Dockerfile) links `python` to it in
  `~/.local/share/hecatron/python/bin`, which every language's hooks have
  on PATH, so a Rust or C++ build that runs `python` (rerun's, xgboost's)
  finds it. Nothing outside hecatron's hooks sees the link, and a `python`
  installed later replaces it (#206).
- **pytest collects `Test*.py` files** (antlr4's Python runtime) where no
  test file has pytest's default names and the project sets no
  `python_files` (#206).
- **pytest leaves out the Python projects inside the one it tests**
  (pydantic-core in pydantic, ast-grep's `crates/pyo3`), whose dependencies
  the outer project's environment doesn't install, unless the project's
  `testpaths` name them (#206).
- **A `tsconfig.json` beside mostly JavaScript is a JavaScript package**
  (antlr4's `runtime/JavaScript`, with its type declarations), in detection
  and in finding a language's directory: a definition's new `detectYields`
  rule gives a hit to another language where its files outnumber the
  language's own (#206).
- **Libraries a Makefile build includes:** the C, C++ and Objective-C
  installers install the common libraries the sources of a build without
  CMake `#include` (`<zlib.h>`, `<openssl/ssl.h>`, `<sqlite3.h>`...), as they
  did for CMake's `find_package()` (CobolCraft) (#206).
- **GoogleTest programs CTest doesn't know of run:** a CMake project whose
  build makes a GoogleTest program it never registers with CTest
  (LightGBM's `testlightgbm`) has that program run as its tests (#206).
