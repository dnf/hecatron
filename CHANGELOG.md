# Changelog

Hecatron follows [semantic versioning](https://semver.org); before 1.0, a
minor version can change behaviour. Numbers are
[pull requests](https://github.com/dnf/hecatron/pulls?q=is%3Apr+is%3Amerged).

## Unreleased

- **`hct lint` now fails on unformatted code.** It runs each language's
  format check (below) after its linter, where the linter doesn't check
  formatting already, so a project whose code isn't formatted the way its
  formatter says will see `lint` fail. Run `hct format` once, or turn the
  check off in `.hecatron/config.json` with `"lint": {"format": false}` (or
  `{"format": {"haskell": false}}` for one language). See
  [docs/format.md](docs/format.md).
- **`hecatron format`:** rewrites every component's code with its
  language's formatter, using the project's own configuration: gofmt,
  rustfmt, Ruff (or Black), `dotnet format`, `mix format`, `dart format`,
  `zig fmt`, swift-format, elm-format..., and where the project configures
  one, Prettier or Biome, RuboCop or Standard, PHP CS Fixer or Pint,
  Spotless, ktlint or scalafmt through the build, clang-format, StyLua,
  perltidy, Fourmolu or Ormolu, Air, cljfmt, erlfmt, JuliaFormatter and
  ocamlformat. `--check` changes nothing and lists the files it would
  change. The installers add the formatters a toolchain doesn't ship, at
  pinned versions.
- **`hecatron audit`:** checks every component's dependencies for known
  vulnerabilities with each ecosystem's auditor (npm, pnpm, Yarn, Bun,
  pip-audit, govulncheck, bundler-audit, Composer, the .NET SDK, and
  OSV-Scanner for the rest), fails at `--fail-on` (default high), and takes
  accepted risks with a reason and an expiry date. See
  [docs/audit.md](docs/audit.md).
- **Old checkouts get the toolchains of their day:** when a repository's last
  commit is over a year old, each language's version becomes the release
  that was current then, unless the project pins an exact one. A 2021
  checkout gets the Node.js and JDK it was tested with instead of today's,
  which can no longer build or run it. `HECATRON_AS_OF` sets the date or
  turns this off; `hecatron doctor` says when it applies.
  A weekly workflow keeps the release dates this uses current.
- **CI workflows and Maven builds say how new a toolchain may be:** with no
  pin file, the versions a project's GitHub Actions setup steps use, and a
  Maven build's Java source level, hold the default back.
- **JavaScript components choose their Node.js version** like TypeScript's.
- **Python follows the versions a project supports:** with no pin file, a
  component whose classifiers stop at an older Python than the default
  (Avro's at 3.14) gets that line's newest release, since compiled
  dependencies often lag a new Python. `hecatron doctor` says when.
- **elm-review is on PATH:** global npm commands Elm installs are all linked
  onto PATH, now that Node.js comes from its official builds.

- **CMake builds run fewer compile jobs at once:** one per CPU (or CPU
  quota) but at most one per 2 GB of memory, so a large C++ project no
  longer has its compilers killed for memory. `CMAKE_BUILD_PARALLEL_LEVEL`
  still decides when set.
- **A CMake subdirectory is built as part of its project:** a component
  whose `CMakeLists.txt` calls no `project()` (one a parent's
  `add_subdirectory()` pulls in) is configured from the nearest directory
  above it that does, and its own tests run.
- **CMake test options are the project's own:** the test hook no longer
  turns on a fetched dependency's test options, nor valgrind or
  large-memory ones, and an option whose setup is missing is left off
  instead of failing the configure. A project that registers no CTest
  tests and runs them from a `check` or `tests` target gets that target.
- **C and C++ installers find more of what a CMake build needs:**
  `pkg_check_modules()` and `pkg_search_module()` lookups, headers looked
  for by hand (`gtest/gtest.h`), files `include()`d from outside the
  component, the enclosing project's own lookups, and more libraries (GLib,
  RE2, ICU, Jansson, Snappy, zstd, LZ4, libevent, flex, bison...).
- **Perl installs Module::Install** and the extensions a `Makefile.PL`
  written with it names (`configure_requires`), which a checkout doesn't
  bundle, and installs the CPAN dependencies a `Makefile.PL` or `Build.PL`
  declares, as it did a `cpanfile`'s.
- **Dart installs Flutter** (3.47.8) where a component is a Flutter package,
  and fetches its packages with `flutter pub get`; the doctor reports it
  missing. The test hook says so when a package has no `test/` directory.
- **JVM builds get the JDKs they ask for:** the JVM installers install the
  JDKs a Gradle or Maven toolchain names, and a build whose Gradle wrapper
  (before 9.1) or Kotlin compiler (before 2.3) can't run on the default
  JDK runs on the newest LTS JDK it supports, which they install too.
- **Python installs a C compiler on Linux,** for dependencies that build a
  C extension from source.
- **Default hooks find a build that isn't in the language's directory:**
  above it in the project (pulumi's `sdk/go` in the module `sdk/go.mod`, a
  root `Package.swift`, CMake project or Gradle build), or in directories
  below it, run in each (several crates under `rust/`, a solution in
  `csharp/src/`), or a root Makefile's target named for the directory
  (`check_cxx`).

- **Node.js uses the project's package manager:** the hooks and installers
  run pnpm, Yarn or Bun when `packageManager` or the lockfile says so (pnpm
  and Yarn through Corepack), so a pnpm or Yarn workspace installs instead of
  failing on `workspace:*`.

- **Toolchain versions for .NET (C#, F#, VB.NET):** a pinned default SDK
  (10.0.403 LTS, the 10.0.4xx feature band), or the one a component's
  global.json asks for, installed side by side with Microsoft's
  dotnet-install.sh into `/opt/dotnet` (per-user on macOS) rather than the
  distribution's package, which holds one feature band; the runtimes older
  target frameworks need go there too. A newer band's analyzers no longer
  fail with CS9057 on Ubuntu's 10.0.1xx compiler.
- **Toolchain versions for PHP:** a pinned default (PHP 8.5), or the line a
  component's `.php-version`, composer.json `config.platform.php`,
  `.tool-versions` or mise asks for, installed side by side from Ondřej
  Surý's packages (his PPA on Ubuntu, packages.sury.org on Debian) or
  Homebrew's shivammathur/php tap, with one Composer for all. The doctor
  reports a component whose PHP is older than its composer.lock needs.
- **Toolchain versions for Ruby:** a pinned default (Ruby 4.0.9), or the
  version a component's `.ruby-version`, `.tool-versions` or mise asks for,
  built side by side by ruby-build into `/opt/ruby/<version>` (per-user on
  macOS), each with its own Bundler. Ruby no longer comes from apt, whose
  3.2 is too old for many gems.
- **global.json's `rollForward` counts:** a .NET SDK pin of 8.0.100 with
  `latestFeature` asks for the newest 8.0 SDK, and with `latestMajor` the
  newest 8; `.php-version` and composer.json's `config.platform.php` pin
  PHP.

- **Rust repairs an interrupted install:** a rustup left with no default
  toolchain (an install killed mid-download) is reported by the doctor and
  fixed by the installer.

- **[docs/languages.md](docs/languages.md):** every language with a link to
  its home, a one-line description, and the build systems, test runners and
  linters its default hooks use, generated from the definitions.

## 0.3.0 (2026-10-27)

- **An installer:** `curl -fsSL …/install.sh | sh` installs the latest
  release into `~/.hecatron` and adds it to bash, zsh and fish startup files;
  sourced from a clone, it also sets up the current shell. Rerun to update.
  It first installs what hecatron needs if it's missing: the Command Line
  Tools on macOS, and git, Python 3 and perl on Linux (apt, dnf or zypper).
- **Downloads retry** transient failures (timeouts, dropped connections,
  429 and 5xx answers, stalled transfers) in installers, hooks, apt and the
  installer's git operations; a 404 isn't retried.
- **A smoke test for every language:** each builds and runs a small program
  with the freshly installed toolchain, and checks its output.
- **Groovy, Scala and Kotlin install their own compilers** (Groovy 5.1.3,
  Scala 3.3.8 LTS, Kotlin 2.4.21), not only the JDK.
- **Default hooks for every language.** A conventional project needs no hook
  files: each language's test, lint, build and clean commands are chosen from
  the build files present (#36).
- **New languages:** JavaScript, Objective-C and Haxe, for 54 in all (#36).
- **Toolchain versions for Python, Go, Rust, Node.js and Java:** a pinned
  default (Python 3.15.1, Go 1.27.3, Rust 1.99.1, Node.js 24.22.1 LTS,
  Temurin JDK 25.0.5 LTS), or the version a component's own pin file asks
  for, installed side by side (#19, #27, #28). A component on the default
  accepts any installed version on the default's release line (#31). The JDK
  is Temurin rather than the distribution's OpenJDK 17, and Node.js comes from
  nodejs.org rather than NodeSource.
- **`hct init` finds real layouts:** directory aliases such as `dotnet/`,
  `javascript/` and `lib/py`, a directory per language where one template
  doesn't fit, and examples, tests and vendored code skipped (#33, #34, #35,
  #37).
- **Generated Dockerfiles** install every version the components use (#30).
- **`hct self-test`** runs ShellCheck and the unit tests; hecatron checks at
  startup for bash 3.2+ and Python 3.9+, and no longer asks for bash 4 (#38).
- **[Hecatron or ad hoc?](docs/hecatron-vs-ad-hoc.md),** a ten-language
  library maintained with hecatron and without it (#15).
- **Fixes** for macOS (Python, Mojo, D, Lua, Pharo, Go) and Debian
  (PowerShell) from the first full coverage run (#32), Ruby gems with native
  extensions (#25), and Wren on arm64 (#21).

## 0.2.1 (2026-10-26)

- **Toolchain versions:** the design (#14), version resolution and a
  per-component environment (`hct env LANG -c COMPONENT`) (#20), and a doctor
  check for a component whose version isn't installed (#22).
- **Coverage matrix** of languages by platform, refreshed weekly (#17).
- Fixes to the Dockerfile snippets for arm64 and current releases (#13), F#,
  VB.NET and Nim on Debian (#12), generated images' locale and base packages
  (#16, #18), and a hook that reads stdin skipping later components (#24).

## 0.2.0 (2026-10-26)

- **Linux support:** installers for Debian and Ubuntu (#4, #5, #10).
- The README explains the problem hecatron solves (#9), and the project has
  a logo (#6, #7).

## 0.1.0 (2026-10-26)

- First release, split out of [sayable](https://github.com/dnf/sayable).
