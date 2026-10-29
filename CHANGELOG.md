# Changelog

Hecatron follows [semantic versioning](https://semver.org); before 1.0, a
minor version can change behaviour. Numbers are
[pull requests](https://github.com/dnf/hecatron/pulls?q=is%3Apr+is%3Amerged).

## Unreleased

## 0.7.0 (2026-10-29)

- **`curl … | sh` on macOS** says how to put `hct` on PATH in the current
  shell, instead of "Ready: try 'hct help'": macOS's `sh` is bash, and
  install.sh took bash reading it from a pipe for being sourced.

- **Homebrew doesn't stop to ask:** `hct install` and `hct doctor --fix`
  set `HOMEBREW_NO_ASK=1` for brew, which asks "Do you want to proceed
  with the installation?" before each install by default in current
  versions, and hide its environment-variable hints.

- **`hct doctor` checks the languages at once,** with `hct test`'s live
  display on a terminal, then prints the same report as before. With
  `--fix`, repairs still run one at a time (package managers allow one
  install at a time), and a language an earlier repair fixed isn't
  repaired again.

- **One password prompt, up front:** `hct install` and `hct doctor --fix`
  name the languages whose install may need administrator rights and say
  why, then ask for the password once (`sudo -v`) before any installer
  runs, keeping it valid until they finish, instead of sudo prompting
  halfway through. Nothing is asked as root, in CI or off a terminal.

- **A project pins its hecatron:** `"hecatron": "0.6"` in
  `.hecatron/config.json` takes any 0.6.x, `"0.6.1"` only that release
  (`">=0.6.0"` is still a minimum). Another version warns on stderr and
  fails `hct check` (a dev build only warns), and the generated GitLab
  pipeline installs the pinned version (#167).
- **install.sh takes a series:** `HECATRON_VERSION=v0.6` installs the latest
  0.6.x release (#167).

- **Root needed less:** OSV-Scanner, formatters' release builds (StyLua,
  Air, Fourmolu, Ormolu), V, Goose, Elm's tools and bundler-audit now go
  in hecatron's bin directory, `~/.local/share/hecatron/bin`, rather than
  `/usr/local/bin` or `/opt`, so installing them needs no password. `hct`
  puts that directory first on PATH for everything it runs. On macOS only
  Perl's and Dart's installers still ask for administrator rights.

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
  language's own. Committed build output doesn't count (`dist/`, `build/`,
  `out/`, `lib/`, the `outDir` in `tsconfig.json`, a `.js` beside its
  `.ts`), so a TypeScript package that tracks its compiled JavaScript
  stays TypeScript (#206).
- **Libraries a Makefile build includes:** the C, C++ and Objective-C
  installers install the common libraries the sources of a build without
  CMake `#include` (`<zlib.h>`, `<openssl/ssl.h>`, `<sqlite3.h>`...), as they
  did for CMake's `find_package()` (CobolCraft) (#206).
- **GoogleTest programs CTest doesn't know of run:** a CMake project whose
  build makes a GoogleTest program it never registers with CTest
  (LightGBM's `testlightgbm`) has that program run as its tests (#206).

- **Zig versions side by side:** a component whose `build.zig.zon` sets
  `minimum_zig_version = "0.14.0"` gets Zig 0.14 (the newest 0.14.x), not
  the default 0.17: Zig breaks compatibility between minor releases.
  `hct install` installs each version the components need next to the
  default in `~/.local/share/hecatron/toolchains/zig/`, and `hct test` and
  the other hooks run each component's own (`hct env zig -c COMPONENT`
  shows it). The `zig` on PATH stays the default. Older releases install
  too (their downloads are named differently before 0.14.1).

- **A current Erlang/OTP on Ubuntu:** Erlang's installer installs
  builds.hex.pm's OTP 29.1.2 (the default `version` in its definition) into
  `~/.local/share/hecatron/toolchains/erlang`, with its commands and
  rebar3's release (3.27.3, checked against its SHA-256) in
  `~/.local/share/hecatron/bin`, without root, instead of Ubuntu's own
  Erlang: 24.04's OTP 25 is too old for current libraries (hackney) and
  Gleam's standard library, which need OTP 26. Debian, which builds.hex.pm
  has no builds for, still gets its distribution's Erlang and rebar3, and
  macOS Homebrew's. The generated Dockerfile installs the same OTP and
  rebar3. Gleam installs Erlang's OTP when the `erl` on PATH is older than
  26, and its doctor reports one that is; `hct uninstall erlang` removes
  the OTP build and rebar3 it installed (#206).

- **Current SWI-Prolog on Ubuntu:** `hct install prolog` adds SWI-Prolog's
  PPA (`ppa:swi-prolog/stable`, its key checked against its fingerprint)
  and installs or upgrades SWI-Prolog from it, so packs that need 9.2 or
  later (lsp_server) work; Debian and macOS keep their own. `hct doctor`
  reports the PPA missing, and `hct uninstall prolog` removes it. An
  `install.json` can name such a repository for any language
  (`aptSources`).
- **Quicklisp for Common Lisp:** `hct install lisp` installs Quicklisp into
  `~/.local/share/hecatron/quicklisp` (no root; its quickstart pinned by
  checksum), and the Common Lisp hooks fetch the dependencies of the
  systems they build and test (their test systems' too) with it, so a
  project such as cl-ppcre, whose tests need cl-unicode, tests without
  setup. `hct doctor` reports Quicklisp missing; `hct uninstall lisp`
  removes it. A Quicklisp the user's init file loads is used instead.
- **JavaScript or TypeScript below:** without a `package.json` in a
  component's directory, the Node.js hooks pick the packages below as
  detection does, so a mostly-JavaScript package with a `tsconfig.json` is
  JavaScript's, not TypeScript's.

- **Secret scanning:** `hct audit` looks for committed credentials with
  gitleaks, which `hct install secrets` installs (8.30.1, pinned and
  checked against its checksum, into `~/.local/share/hecatron/bin`, no
  root). `hct audit secrets` runs only the scan, of the working tree less
  what git ignores, or of every commit with `--history`. Findings show
  the rule, file, line and gitleaks' fingerprint, never the secret, in
  every output and in the SARIF report and job summary, and fail the
  audit. A project's `.gitleaks.toml` and `.gitleaksignore` apply, and
  `"audit": {"secrets": {"ignore": [...]}}` leaves out paths such as test
  fixtures. `"ci": {"secrets": true}` generates a CI job that runs it
  (#191).

- **Docs in two sets:** a [user guide](docs/guide/README.md), with a
  reference for every command and its options, the project's
  configuration, hooks, toolchain versions and generated files, and a
  [contributor guide](docs/contributing/README.md), with the architecture,
  each module, how a command flows through them, the install engine,
  testing and releasing. `docs/design.md` is now the design record in
  `docs/contributing/design.md`; `docs/agents.md` stays where it was (#98).

- **`hct exec LANG [-c COMPONENT] -- CMD...`** runs a command with a
  language's environment for a component, what `hct env LANG -c COMPONENT`
  prints, without an `eval` first: useful where components use different
  versions of one toolchain. Without `-c`, the component whose directory
  holds the working directory, else the only one in the language. CMD runs
  in the working directory and its exit status is hct's; a version not
  installed is reported with the `hct install` to run (#225). Needs the
  binary.

- **A software bill of materials:** `hct sbom` writes CycloneDX 1.6 JSON
  (or SPDX 2.3 with `--format spdx`) for every component in every
  language, from what `hct licenses` reads: each package's purl, its SPDX
  license where known (never guessed), direct or transitive where the lock
  file says, dev and test dependencies marked (`--no-dev` leaves them out),
  and the project's name, commit and a timestamp (`SOURCE_DATE_EPOCH` for
  reproducible output). To stdout or `-o FILE`; positional languages and
  `-c COMPONENT` select as `hct licenses` does. Packages from registries
  deps.dev has no licenses for (pub.dev, Hex...) are listed without
  licenses (#226).

- **`hct docs` and `hct bench`:** two new run commands. `hct docs` builds
  each component's API documentation with its ecosystem's tool into the
  tool's own directory (`cargo doc --no-deps`, Sphinx with a
  `docs/conf.py` or pdoc, Javadoc, Dokka, Scaladoc, DocFX, ExDoc, EDoc,
  DocC, Haddock, odoc, `dart doc`, Documenter, TypeDoc, YARD or RDoc,
  Doxygen with a `Doxyfile`...) and ends saying where (also in agent and
  JSON output). `hct bench` runs benchmarks (`cargo bench`, `go test -run
  '^$' -bench . ./...`, pytest-benchmark, JMH, BenchmarkDotNet,
  `cabal bench`, package-benchmark, a CMake bench target or Google
  Benchmark programs...). Both run only tools a component already has;
  a language or component with none says so and passes. A project's
  `docs.sh` or `bench.sh` replaces the default
  ([languages](docs/guide/languages.md#api-docs-and-benchmarks)) (#227, #228).

## 0.6.0 (2026-10-28)

- **Detection:** dotfiles aren't sources (a `.nvim.lua` no longer adds
  Lua); Lua isn't detected from premake build scripts, nor R from an
  Octave package's `DESCRIPTION`.

- **Default hooks find more layouts:** PHP's test and deps hooks use the
  nearest `composer.json` above (a root one covering `lang/php`, running
  its PHPUnit on the directory's `phpunit.xml`); Dart's test hook tests the
  packages below with a `test/` (a pub workspace's root, `pkgs/*`); R's
  tests each package below with `tests/testthat/` (`R-package/`), and its
  deps hook installs a `DESCRIPTION`'s missing dependencies from CRAN
  (`hct install r` now fetches them); Ruby compiles a gem's native
  extension (`rake compile`) before testing; Racket links sibling packages
  (each with its `info.rkt`) before `raco test`; Elm runs
  `elm-explorations/test` 1.x on the last elm-test supporting it, with
  Elm 0.19.1.

- **Installs:** PHP installs the extensions a `composer.json` or its
  dependencies require (`ext-curl`, `ext-mongodb`...) where packages
  exist; opam installs the system packages dependencies need; Tcl installs
  `make` for TEA extensions' tests.

- **Node.js workspaces built before their tests:** when a package in the
  workspace a test runs in needs building (a native addon or entry point
  not built yet), `hct test typescript` and `hct test javascript` build
  the workspace first (`pnpm -r run build`, `npm run build --workspaces`,
  Yarn's `workspaces foreach --topological run build`), so a root `test`
  script that runs every package's tests, or a package importing another
  one's `dist/`, finds them built.

- **.NET's `rollForward: latestMajor` is a floor:** a `global.json` asking
  for SDK 8.0.100 or later now gets hecatron's default SDK (10), or the
  newest of its own major when that is newer than the default, raised to
  the newest major the component's projects target (`net11.0` needs SDK
  11). Before, it read as SDK 8, which can't build a project targeting
  `net10.0`. Definitions name such files in the new `floorFiles` key.

- **`hct self`:** hecatron's own commands take a space, as `rustup self`
  and `uv self` do: `hct self update`, `hct self test` and `hct self
  build`; `hct self` lists them. `self-test` and `self-build` still work.

- **`hct self update`:** updates an installation install.sh made to the
  latest release (or `--version X.Y.Z`), its binary checked against the
  release's `SHA256SUMS` and the new version swapped in whole, so a
  failure leaves the old one working; rewrites `env` and `env.fish` as
  install.sh does, so new PATH entries (the toolchains' links) take
  effect; refreshes the Agent Skill `hct skill install` put in place. `--check` only reports (exit 3: a newer release).
  Once a day at most, a release binary looks for a newer release in the
  background and says so in one line after a command, on a terminal in
  human output only (never in CI, agent or JSON output, or with
  `HECATRON_NO_UPDATE_CHECK=1`); `hct status` reports it from that check.
  Hecatron never updates itself. Needs the binary.

- **Detection:** a language whose build files are all more than three
  levels below the directory the layout gives it (antlr4's Go module in
  `runtime/Go/antlr/v4`, jsii's in `packages/@jsii/...`) gets its own
  `implPaths` entry instead, since the hooks look no deeper and failed with
  "no go.mod here, below, or above it"; files under `vendored/` no longer
  make a language.

- **JVM and .NET builds in real projects:** a Java, Kotlin, Scala or
  Groovy directory inside a Gradle build that isn't one of its projects
  (`moshi/src/main/java`) runs Gradle from the project it belongs to,
  instead of failing as "not part of the build"; a Kotlin Multiplatform
  build without a `test` task runs `jvmTest`, else `allTests`; a
  multi-module Maven build tests through the package phase; Mill builds (`build.mill`) are tested;
  the JVM gets a UTF-8 locale when none is set; Maven, Gradle and sbt are
  installed for builds below the language's directory too. .NET tests roll
  forward to the installed runtime (a test project targeting net8.0 runs on
  .NET 10), and a directory with several projects and no solution tests
  each test project. A Clojure project with `deps.edn` but no `:test`
  alias tests with Leiningen when it has `project.clj`. A Gradle build's
  toolchain JDK is found from its root and its version catalog
  (`libs.versions.jdk`), so a source directory's build gets it installed,
  and a build whose `java` is a runtime without `javac` (a JRE another
  package pulled in) runs on the newest installed JDK.

- **`hct dockerfile`:** the generated Dockerfile gives curl a config file
  (through `CURL_HOME`) with the installers' retry policy plus
  `--retry-all-errors`, so a transient download failure, such as a dropped
  TLS connection to api.adoptium.net, is retried instead of failing the
  build. It applies to every `curl` in the build and in the image.

- **Python tests in monorepos:** `hct test python` in a directory without
  a `pyproject.toml`, `setup.py` or `setup.cfg` now tests the Python
  projects in the directories below (`py-polars/`, `python-package/`),
  each in its own environment, instead of running pytest over the whole
  tree; with none below, pytest runs there on its own as before.

- **Swift:** a package that declares no test target (a C library's
  `Package.swift` for SwiftPM users) passes `hct test` instead of failing
  on "no tests found".

- **pnpm and Yarn for every language's hooks:** Corepack's pnpm and Yarn
  shims live in `~/.cache/hecatron/corepack-shims`, which `hct install`
  now creates when a package needs them and which is a static `path`
  entry of JavaScript and TypeScript, so another language's build or
  tests that run `pnpm` (a Rust test in a pnpm monorepo) find it.

- **Bash is detected from a Bats suite:** a definition's `detectTests`
  (Bash's `*.bats`) count in `test/` and `tests/` for a language not
  otherwise found, placing it at the directory tested, so bats-core and
  Bash-it are Bash projects. Bash's test hook leaves out `.bats` files
  under a `fixtures/` directory, which are test data.

- **Fixed: a random "Unknown language", skipped dependencies or wrong doctor
  result under load.** The shell scripts checked list membership by piping
  into `grep -q`; when grep matched early, the writer was killed and, under
  `pipefail`, the check said "not found". They now use a plain loop (#197).

- **A Kotlin JVM target picks the JDK:** a Gradle build whose Kotlin sets
  `jvmTarget` (11 or later) and that sets no Java target or toolchain runs on
  that JDK, which `hct install` installs, instead of failing Kotlin's check
  that Java and Kotlin target the same JVM.

- **JDK 8 installs:** a Gradle or Maven toolchain asking for JDK 8 gets
  Temurin 8, whose releases are named `jdk8u412-b08`. Before, `hct install`
  failed with "No Temurin JDK release matches 8" and installed no other JDK
  the build needed either.

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

- **Rust's system dependencies:** `hct install rust` installs what a
  component's crates build with, read from its Cargo.lock files: the
  libraries common `-sys` crates link (OpenSSL, GLib, GTK and WebKitGTK,
  D-Bus, ALSA...), Python's headers for PyO3, and the tools build
  dependencies run (CMake for the `cmake` crate and zlib-ng, `protoc` for
  prost-build unless protoc is vendored, Cap'n Proto, libclang for bindgen),
  and the `rust-src` component where ui_test's UI tests need it.

- **Python test dependencies in dependency groups:** `hct test python`
  and `hct deps python` now install every PEP 735 dependency group
  (`uv run --all-groups`, `uv sync --all-groups`), not only `dev`, so
  test dependencies kept in a `test` or `tests` group are there.

- **Meson versions:** where a `meson.build` asks for a newer Meson
  (`project(..., meson_version: '>=1.8.3')`) than the one installed, `hct
  doctor` reports it and `hct install c` (or `cpp`, `objectivec`) installs
  the current Meson from PyPI as a uv tool on Linux, whose distribution
  Meson lags (Ubuntu 24.04's 1.3 can't build NumPy or SciPy).

- **Rust's system dependencies:** `hct install rust` reads every Cargo.lock
  in the project outside the trees detection skips (examples, tests,
  vendored code...), not only those of the Rust components, so a crate
  another language's build compiles (a Java binding's Maven build running
  cargo) gets what it needs too, such as `protoc`.

- **Rust tests:** `hct test rust` runs `cargo test` rather than `cargo test
  --all-targets`: Cargo's default targets, now with doctests, and without
  running benchmarks as tests (a nightly-only benchmark broke stable builds,
  and others fail or run long in test mode).

- A fresh install from `install.sh` no longer prints git's "is not a commit" warning and its detached HEAD advice.

- **`hct self uninstall`** removes hecatron: `~/.hecatron`, its lines in
  your shell's startup files and fish's conf.d, and its Agent Skill. It
  asks first (`--yes` skips the question). The toolchains it installed
  stay; `--all` also removes those it unpacked (`~/.local/share/hecatron`)
  and its download cache (`~/.cache/hecatron`). Run from a checkout, it
  keeps the checkout.

- **Node.js packages built before their tests:** `hct test typescript`
  and `hct test javascript` first build a package whose tests need it
  built: a native addon (napi-rs, node-gyp) not built yet, or an entry
  point (`main`, `exports`) the build writes that isn't there. Through
  the package's build script, else `napi build --platform` or `node-gyp
  rebuild`, instead of failing with "Failed to load native binding" or
  "Cannot find module dist/...". A configured test runner that fails now
  fails as itself, and npm's placeholder test script counts as none.

- **Node.js tests in more layouts:** `hct test typescript` and `hct test
  javascript` now test a workspace root without a `test` script through its
  packages' scripts, a package with a configured Vitest, Jest or Mocha but
  no script through that runner, and a directory without a `package.json`
  (or a root `package.json` holding only tooling) through the language's
  packages below it, where they failed with "no test command". Scripts
  that call pnpm or Yarn by name find Corepack's shim on PATH, and tests
  run with `CI=true` (unless it's set), so runners that would otherwise
  watch for changes (Create React App's, Vitest's) run once.

- **Detection:** `hct init` no longer proposes components from directories
  like `src/<service>/src/main/<lang>` when another language's directory
  isn't named for it (TypeScript in `src/frontend`), which left that
  language, and every other one outside the pattern, with no directory.

- **Other languages' tools on PATH:** a language's hooks (and
  `hct env LANG`) now also have the project's other added, enabled
  languages' static `path` directories on PATH, after the language's own,
  so a Ruby, OCaml or D build that runs `cargo` finds the Rust that
  `hct install` put in `~/.cargo/bin`.

- **Release downloads as data (#77, stage 2):** `install.json` can name a
  release download, always checked against a SHA-256 (inline, a published
  checksum file, or the publisher's index). Gleam and Zig install that way,
  at the version `config.json` pins (Zig installed whatever was latest),
  into `~/.local/share/hecatron/toolchains` without root, on macOS too
  instead of Homebrew, with their commands linked into
  `~/.local/share/hecatron/bin`, which `~/.hecatron/env` now puts on PATH.
  Downloads are cached in `~/.cache/hecatron/downloads`; a toolchain that
  doesn't work is replaced whole. With `"ci": {"cache": true}` both are
  cached in CI.

- **Installs as data (#77, stage 1):** with the binary, Bash, COBOL, D,
  Erlang, Common Lisp, Octave, Pascal, Prolog, Racket, Raku and Tcl install
  from a `languages/<lang>/install.json` instead of `install.sh`: every
  step has a check, so a rerun does nothing and `doctor --fix` runs only
  what fails; the doctor's checks come from the same data; installing
  several languages makes one `apt-get install` (or `brew install`) for all
  their packages; receipts record what hecatron installed, and uninstall
  removes only that, keeping what another language needs. `hct install
  --plan` shows what would happen. A project's own `install.sh` still
  wins, and `HECATRON_INSTALLER=script` forces the scripts.
  `tests/docker/parity.sh` checks the two ways agree. The prototype's
  `"install"` key moved from `config.json` to `install.json`. Toolchains
  the engine downloads go to `~/.local/share/hecatron/toolchains` (no
  root), and `"ci": {"cache": true}` caches that directory, keyed on what
  the engine would install; the eleven languages converted so far come
  from apt, which isn't cached, so this grows with the languages moved to
  downloads.
- **Uninstalling a language leaves shared and system software:** an
  `uninstall.sh` removes only what is the language's own and hecatron
  installed. A toolchain another language's installer also installs stays,
  with a line saying so (`Left CMake installed: C, C++, Objective-C, COBOL
  and Fortran share it.`): CMake (C, C++, Fortran), Node.js (TypeScript,
  JavaScript), the .NET SDK (C#, F#, VB.NET), Erlang/OTP (Erlang, which now
  removes only rebar3), uv (Python) and Homebrew's gcc (Fortran). Python no
  longer removes the distribution's `python3` (`apt-get remove python3`
  could take much of a Debian or Ubuntu system with it); it removes uv's
  Python builds, unless Mojo runs on one. PHP removes only the `-cli`,
  `-xml` and `-mbstring` packages its installer installs, instead of
  `php*` followed by `apt-get autoremove`. D also removes dub.
- **Elixir versions:** Elixir installs side by side from builds.hex.pm's
  precompiled releases, with a default of 1.20.4 (it was Ubuntu's apt 1.14,
  which current libraries warn about), a `.tool-versions` or `mise.toml`
  pin (`1.17.3-otp-27` reads as 1.17.3) or the `elixir-version` a CI
  workflow tests choosing another. Each version runs on an OTP it supports,
  linked as `<home>/erlang`: the Erlang on PATH when it is one, else
  builds.hex.pm's OTP build on Ubuntu or Homebrew's `erlang@N` on macOS. On
  Debian, which has no builds there, an Elixir its own Erlang can't run
  fails with the reason. Each version has its own Hex (`MIX_ARCHIVES`), so
  one built on a newer OTP doesn't break an older Elixir's. `hct doctor
  elixir` notes which OTP each component's Elixir runs on.

- **Doctor notes:** a `doctor.sh` line starting `note: ` (`note` in
  `_common.sh`) is shown under the language, like the version notes, without
  counting as a problem, for what's worth knowing about a healthy toolchain.

- **`hct sync`:** after a pull, rebase or checkout, says in a few lines
  what changed for hecatron since where `HEAD` was (or a ref): runs
  `hct deps` for the components whose lock files changed (`--no-deps` only
  says so), says the toolchain version each changed pin now needs and
  whether it's installed (it never installs), and runs `hct check` when the
  config, hooks or generated files changed. Never pulls or fetches; needs
  the binary (#92).
- **Reports every CI reads:** with the binary, `HECATRON_REPORT_DIR`
  receives JUnit XML from `hct test`, `lint` and `build` (a suite per
  language and component, with the tools' own test cases where they write
  them: Maven Surefire and Failsafe, Gradle, sbt, `go test -json`, pytest,
  and anything a hook leaves in `HECATRON_TEST_REPORTS`), SARIF 2.1.0 from
  `hct lint` and `hct audit` (an advisory per rule, at the lock file, for
  GitHub code scanning), and GitLab Code Quality JSON from `hct lint`,
  beside the normal output; `--output junit` and `--output sarif` print one
  instead. The generated GitLab pipeline hands JUnit and Code Quality to
  the merge request widgets (#100).
- **GitHub Actions job summary:** with `GITHUB_STEP_SUMMARY` set, each run
  command appends a table of languages and components with status and
  time, then each failure's error lines, log and rerun command; `audit`,
  `licenses` and `advisor` add their findings (#101).
- **`hct install` and `hct uninstall` run in the binary:** choosing the
  languages, running their installers and fetching the deferred
  dependencies no longer goes through manage.py (76 ms to 12 ms around a stub
  installer); the installers stay scripts.
- **`hct version <version>` runs in the binary** (90 ms to 11 ms around a
  stub hook); each language's `version.sh` stays a script.
- **`hct banner` runs in the binary,** without Python: 3 ms instead of
  51 ms, for generators that call it for every file they write.
- **The run commands' help lists `--output`, `--timeout` and `--changed`**
  (`hct test help`), noting that they need hecatron's binary.
- **OSV-Scanner for `hct licenses`:** every language `hct licenses` reads
  (TypeScript, JavaScript, Python, Go, Ruby, PHP, C#, F#, VB.NET and
  Clojure, besides those whose audit already used it) installs OSV-Scanner
  2.7.0 with its toolchain, so a project of only those languages no longer
  stops at "OSV-Scanner isn't installed".
- **Pin files can drop a suffix:** a `pinFiles` or `supportFiles` entry's
  `"strip"`, a regular expression, is removed from the version the file
  gives, for ecosystems that write more than the version in it (asdf's and
  mise's Elixir `1.17.3-otp-27`).

- **Detection skips more code that isn't the project's:** `hct init`
  leaves out directories it skips in any case (`Examples/`), hidden
  directories (`.jenkins/`), `templates/` and test data
  (`tests_data/`, `test-data/`), so a Jenkinsfile, a code generator's
  templates or an example no longer adds a language.

## 0.5.0 (2026-10-28)

- **Leave files out of formatting:** `"format": {"exclude": [...]}` lists
  paths `hct format` and `hct lint`'s format check skip (a fixture
  misformatted on purpose, generated code), where hecatron gives the
  formatter its files.
- **A missing linter says so:** `hct lint` with a language's linter not
  installed (golangci-lint, say) reports the toolchain missing (exit 3), and
  "command not found" leads the failure's excerpt.
- **`init` detects better:** a build file at the root keeps its language
  there unless the directory the layout names for it holds the language's
  code (in a repository with `go.mod` at the root and a `languages/go/` of
  other files, Go lives at the root, not in `languages/go/`), and files
  under `hooks/` are tooling, not a language (a Tcl hook script no longer
  adds Tcl). Applies to `hecatron init` and to a repository read without a
  config.
- **`hct licenses`:** every dependency's license, in every language and
  component, from the lock files (OSV-Scanner with deps.dev for npm, PyPI,
  Go, crates.io, Maven, NuGet and RubyGems; `composer.lock` for PHP):
  rolled up per SPDX license, each package with its version, direct or
  transitive and dev marked, unknown licenses apart, and what isn't covered
  said so. A `"licenses"` policy in the config (`allow`, `deny`, `ignore`
  with a reason and an expiry, `includeDev`) fails the command on a package
  it refuses; `hct check` validates it. `--output human|agent|json`. Needs
  the binary (#132).
- **CI jobs get a time limit:** `hecatron workflows` adds
  `timeout-minutes: 60` to each generated GitHub job that sets none, and
  `timeout: 60m` to each GitLab job, so a hung job stops after an hour
  instead of GitHub's 6. Set it for the project or
  per language in `.hecatron/config.json`
  (`"ci": {"timeoutMinutes": 30, "languages": {"pascal": 120}}`); a
  template's own `timeout-minutes` stands. Every generated workflow
  changes once: run `hecatron workflows` after upgrading, or
  `hecatron check` reports them out of date.
- **An Agent Skill for hecatron:** AI agents that read Agent Skills learn
  hecatron without the docs: first steps, the test-lint-format loop, exit
  statuses, asking before installs, and what to do when something fails.
  In Claude Code: `/plugin install hecatron --marketplace dnf/hecatron`; in
  claude.ai, upload the release's `hecatron-skill.zip`.
- **More of hecatron runs in the binary, and faster:** toolchain version
  resolution and each language's environment (`hct test` across 8
  languages × 3 components: 0.28 s of setup to 0.07 s), `init` and the
  other project commands (`check`, `add`, `remove` under 10 ms), and
  `doctor`'s checks (1.6 s to 0.6 s for 8 languages). Same output as
  before, checked byte for byte.
- **macOS fixes:** Ruby installs when Homebrew's ruby-build lags (it uses
  ruby-build's repository, as on Linux); Fortran's doctor checks gfortran,
  so `--fix` installs it; Scala's smoke test uses Scala CLI's flag; Ada sets
  `SDKROOT` from `xcrun`, so GNAT links when the Command Line Tools are
  newer than the selected Xcode.
- **Elixir on Linux installs Erlang's development files,** so dependencies
  that compile Erlang sources (leex, yecc) build.
- **OCaml installs every opam package's dependencies,** for a repository
  with several `.opam` files too.
- **`hct skill install`:** puts hecatron's Agent Skill where an agent finds
  it: `~/.claude/skills` (the default), the repository's `.claude/skills`
  (`--project`, to commit) or another agent's skills folder (`--dir`).
  Reinstalling replaces hecatron's own copy and nothing else.
- **`hct status`**: what to know first in a repository, in one command:
  hecatron's version, whether the config is saved or detected, and for each
  language its directories, the toolchain version it resolves to and why,
  whether that toolchain is installed, the last run of each action, and its
  uncommitted changes; then the commands worth running next, marking those
  that install something. `--output human|agent|json`, chosen as for
  `hct test`. Needs the binary that releases ship (#121).
- **`hct advisor`**: where `hct doctor` keeps the machine working, the
  advisor reads the project and recommends what each language's ecosystem
  considers good practice, each finding with what to change, why, and a
  link to the ecosystem's own documentation: lock files committed (with each
  ecosystem's nuance: Cargo binaries, Composer projects, Dart and Ruby
  applications) and `package-lock.json` still matching `package.json`,
  toolchains pinned in the ecosystem's own file and not past end of life
  (from the release tables hecatron ships), a formatter and linter
  configured, Gradle's build and configuration caches, `npm ci` in CI, CI
  covering every language, GitHub Actions not pinned to a branch, and audit
  acceptances past their date. Rules are data in each language definition
  (`advice.json`); rules for 23 definitions so far. Reads files only;
  `--fix` adds missing pin files at the version in use; `--fail-on
  recommended|suggestion` for CI; `"advisor": {"ignore": [...]}` in config,
  validated by `hct check`. Needs the binary (#105).
- **GitLab CI and caching in generated CI:** `hct workflows` writes a
  `.gitlab-ci.yml` with a job per language (install, test and lint through
  `hct`) when `generated.ci` is `"gitlab"` (or `["github", "gitlab"]`, or
  with `--ci gitlab`), and `hct check` fails when it falls behind. With
  `"ci": {"cache": true}` the generated GitHub workflows and GitLab jobs
  cache each language's toolchain (keyed on its pin files and hecatron's
  version) and dependencies (keyed on its lock files); each definition
  lists the directories in its new `cache` key. Off by default, so existing
  workflows don't change. Needs the binary (#102).
- **Dependabot and Renovate configuration:** with `generated.dependabot`
  (or `generated.renovate`), `hct workflows` writes `.github/dependabot.yml`
  with an `updates` entry for each package ecosystem and component
  directory (`renovate.json` with the managers and paths), merging the
  project's `dependabot` (or `renovate`) settings, and `hct check` fails
  when a language or component was added without regenerating it. Each
  definition names its ecosystems in new `dependabot` and `renovate` keys.
  Needs the binary (#103).

## 0.4.0 (2026-10-27)

- **A fast binary:** installing a release also installs the Go core for
  your machine (macOS and Linux, x86_64 and arm64), checksum-verified, and
  `hct` runs it: `--version` and `list` answer in about a millisecond
  (80 ms before, on a 51-language project), and help, `list`, `env` and the
  run commands no longer depend on the system's bash (macOS's 3.2). The
  run commands take `--output human|agent|json|ci` (or `HECATRON_OUTPUT`):
  agent output is a summary line and only the failures with their key
  error lines; exit status 3 means a toolchain is missing. The rest still
  runs through the scripts. `HECATRON_NO_GO=1` keeps to the scripts.
- **Fixes on macOS's bash 3.2:** every run command's help and `hct
  doctor` no longer fail with "unbound variable" in a repository with no
  languages (which the binary avoids altogether).
- **.NET runs the SDK it should:** a version the project's config names
  runs exactly that SDK, each version in its own view of `/opt/dotnet`; a
  version a GitHub workflow names is installed but, as `setup-dotnet`
  does, the newest SDK runs unless a `global.json` picks one, and
  `hecatron doctor` now says which. Downloading `dotnet-install.sh`
  retries a dropped connection.
- **Installing everything fetches dependencies last:** with no languages
  named, `hecatron install` installs every toolchain first, then fetches
  dependencies once, so one language's fetch that needs another's
  toolchain (a `prepare` script running Ruby) no longer fails.
- **Perl runs a project's own Makefile test target,** which may set what
  its tests need (`AUTHOR_TESTING`): cucumber/tag-expressions runs 169
  tests through it, where hecatron's `prove` ran 68.
- **`hecatron init` skips a package with no test script** for a JavaScript
  or TypeScript implementation (a JSON-schema package beside the real
  one), saying why.
- **A generated image fails when its PowerShell modules didn't install,**
  instead of shipping without Pester.
- **Instructions for AI agents:** `hct init` writes a short section to
  `AGENTS.md` (and has an existing `CLAUDE.md` load it); `--no-agents` skips
  it. `hct help agents` prints a compact reference written for agents.
- **No setup step:** in a git repository without `.hecatron/config.json`,
  every command works on what `hct init` would detect, remembered inside
  `.git/` until the tracked files change; nothing is written to the working
  tree. `hct init` saves it, to commit and adjust.
- **`hct lint` now fails on unformatted code.** It runs each language's
  format check (below) after its linter, where the linter doesn't check
  formatting already, so a project whose code isn't formatted the way its
  formatter says will see `lint` fail. Run `hct format` once, or turn the
  check off in `.hecatron/config.json` with `"lint": {"format": false}` (or
  `{"format": {"haskell": false}}` for one language). See
  [docs/format.md](docs/format.md).
- **`hecatron outdated`:** lists each component's direct dependencies that
  have a newer version, from each ecosystem's own report (npm, pnpm, Yarn,
  uv, `go list -u -m`, Cargo's dry-run update, Bundler, Composer, the .NET
  SDK, Hex, pub, Maven's versions plugin, Gradle's Versions plugin where a
  build applies it, cabal), with current, wanted and latest versions.
  Report-only unless `--fail`; `--json` prints one array. See
  [docs/outdated.md](docs/outdated.md).
- **`hecatron deps`:** fetches every component's dependencies with its
  ecosystem's tool, held to the lock file where there is one: npm, pnpm,
  Yarn or Bun's frozen install, `uv sync --locked`, Bundler frozen,
  `composer install`, `dotnet restore --locked-mode`, `go mod download`,
  `cargo fetch --locked`, `mix deps.get`, `pub get --enforce-lockfile`,
  `swift package resolve`, Maven's `dependency:go-offline`, Gradle, sbt,
  Stack or cabal, and more. Languages without a dependency manager say so.
  The test, lint and build hooks that install missing dependencies, and the
  installers, now do it the same way, so **pnpm, Yarn, Bun, Bundler, Dart
  and Shards installs no longer change a lock file**: one that's out of
  date fails, as `npm ci` already did.
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
