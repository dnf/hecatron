# Changelog

Hecatron follows [semantic versioning](https://semver.org); before 1.0, a
minor version can change behaviour. Numbers are
[pull requests](https://github.com/dnf/hecatron/pulls?q=is%3Apr+is%3Amerged).

## Unreleased

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
