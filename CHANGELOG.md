# Changelog

Hecatron follows [semantic versioning](https://semver.org); before 1.0, a
minor version can change behaviour. Numbers are
[pull requests](https://github.com/dnf/hecatron/pulls?q=is%3Apr+is%3Amerged).

## Unreleased

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
