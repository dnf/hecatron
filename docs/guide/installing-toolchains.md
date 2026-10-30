# Installing toolchains

Each language's definition lives in `languages/<lang>/` in this repository (a project can override any file in its own `.hecatron/languages/<lang>/`), as separate single-purpose files. Each script handles both macOS and Linux, and runs its action on execute (no subcommands).

## Commands

```bash
hct install                # install all available languages
hct install go rust        # install specific languages
hct install --plan         # show how each would install and what would run (needs the binary)
hct install --no-deps      # toolchains only, not the project's dependencies (hct deps)
hct uninstall go rust      # uninstall specific languages
hct get-started            # interactive onboarding with status + install menu
hct get-started --status   # print language status table
hct get-started --check    # exit 0 if all installed, 1 if missing
hct doctor                 # check every toolchain actually works, and how to repair it
hct doctor --fix go lua    # run the installer of each unhealthy language, then re-check
hct doctor --fix --no-deps # repair toolchains only, not the project's dependencies
hct doctor --output json   # each language's problems and notes as JSON (needs the binary)
```

`hct install <lang>` is also the repair: installers are idempotent and fix the
broken states their `doctor.sh` detects (a linter built with an older Go,
luacheck on Lua 5.5, a moved virtualenv, an unlinked Homebrew formula...).

**Administrator rights are asked for once.** Before `hct install` or
`hct doctor --fix` runs anything that may need root (a language installed
by its script on a platform its `installNeedsRoot` names, where it may run
apt-get or write to `/usr/local` or `/opt`, or from data with apt
packages), it names those languages, says why, and
runs `sudo -v`, so the password is asked for once, up front, never
halfway through; it keeps sudo's credentials fresh until the installers
finish. Nothing is asked as root, where sudo needs no password (CI), or
off a terminal. Declining installs nothing. (Needs the binary.) Tools
hecatron downloads itself (OSV-Scanner, formatters' release builds,
gitleaks from `hct install secrets` for [secret scanning](audit.md#secrets)) go in
its bin directory, `~/.local/share/hecatron/bin`, which needs no root and
which `hct` puts first on PATH for everything it runs; on macOS, only Perl
(cpanm into the system Perl) and Dart (Flutter's cask) still ask.

Short aliases work everywhere: `hct install ad ts py rb rs kt gy lu ft c++ cs fs sw hs ex erl clj gm sc zg el pl dt ml`

Misspelled names get a "did you mean?" prompt (e.g. `hct install elixer` suggests `elixir`).

## Doctor and repair

Unchanged from sayable's design, now shared: `doctor` checks each language's
`dependency` and `lintDependency` commands from `config.json`, then runs the
optional `doctor.sh`, which reports one `problem` per line. A missing
`dependency` is a problem; a missing `lintDependency` (Go's golangci-lint,
Lua's luacheck, Bash's ShellCheck) is a note ("lint tool golangci-lint not
found: hct lint go needs it"), since the toolchain works without it and a
project may lint its own way (a project `lint.sh`); `hct lint` still needs
it, and `hct install` installs it. A line starting
`note: ` (`note` in `_common.sh`) is shown under the language, like the
version notes, but isn't a problem: a language with only notes is healthy,
and `--fix` repairs only problems. Installers are
idempotent and repair what their doctor detects (a linter built with an older
Go, luacheck on Lua 5.5, a moved virtualenv, an unlinked Homebrew formula).
`doctor --fix` runs the installer of each unhealthy language and checks
again. With the binary, a language installed from its `install.json` ([installs as
data](../contributing/install-engine.md)) gets its checks from that instead of `doctor.sh`, and
`--fix` runs only the steps whose checks fail. Installers that install project dependencies (`bundle install`, `uv
sync`) do so in every component by running `hct deps` (`impl_deps` in
`_common.sh`; [hooks](hooks.md#dependencies)). `hct install` with no languages named runs those
once every toolchain is in, since one language's dependencies can need
another's toolchain (cucumber/messages' `npm ci` runs a generator needing
Ruby); a failure there fails the install and names the language. With
`--no-deps` (`hct install --no-deps`, `hct doctor --fix --no-deps`) the
installers install toolchains only and run none of the project's code.

## Installs as data

A language with a `languages/<lang>/install.json` is installed by
hecatron's binary from that data instead of its `install.sh`
([the install engine](../contributing/install-engine.md)). So far that covers the
languages whose installers only install system packages (Bash, COBOL, D,
Common Lisp, Octave, Pascal, Prolog, Racket, Raku and Tcl) and two
release downloads (Gleam, Zig).

```json
{
  "schema": 1,
  "toolchain": { "versions": "system", "packages": { "apt": ["rakudo"], "brew": ["rakudo"] } },
  "tools": [
    { "name": "zef",
      "when": { "glob": "META6.json", "because": "a component has a META6.json" },
      "packages": { "apt": ["raku-zef"], "brew": ["zef"] },
      "check": { "command": "zef" } }
  ]
}
```

- **Every step has a check**, so a rerun does nothing and `doctor --fix`
  runs only what fails. The doctor reports the failing checks of the tools
  and steps that apply ("zef not installed (a component has a META6.json)").
- **System packages are batched**: installing several languages makes one
  `apt-get install` (or `brew install`) call for all their packages.
- **Receipts** in `~/.local/share/hecatron/receipts/<lang>.json` record the
  packages hecatron installed (only those that weren't there before), and
  `hct uninstall` removes only those, keeping any another language's
  receipt or definition also needs, or that the definition keeps (COBOL's
  cmake). A language installed by its script has no receipt, and uninstall
  then removes nothing and says so.
- **Which runs**: the project's own `install.sh` always wins;
  `HECATRON_INSTALLER=script` forces the scripts (for comparing or
  bisecting); otherwise the data, when the binary can carry it out on this
  machine (apt on Debian and Ubuntu, Homebrew on macOS); otherwise
  `install.sh`. Without the binary, `bin/hecatron` runs the scripts, which
  stay.
- **Release downloads** are pinned (the version in `config.json`), always
  checked against a SHA-256 (inline, a published checksum file, or the
  publisher's index), unpacked into
  `~/.local/share/hecatron/toolchains/<lang>/<version>` without root, and
  their commands linked into `~/.local/share/hecatron/bin`, which
  `~/.hecatron/env` puts on PATH. macOS gets the same download as Linux.
- **`hct install --plan`** prints, for each language, every action and
  whether it is done, will install or run, or is skipped (and why), the
  languages left to their scripts and why, and the commands it would run.

**CI caching.** Toolchains the binary downloads (not system packages) go
to `~/.local/share/hecatron/toolchains/<lang>`, the user's own directory,
so installing needs no root and a CI cache can restore them. With
`"ci": {"cache": true}`, `hct workflows` caches that directory for each
such language, keyed on the language, the platform, the definition and
default version hecatron would install, and the project's own
`install.json` and pin files. A restored toolchain passes the install's
checks, so `hct install` does nothing; a partial restore fails them and is
repaired. Gleam and Zig are cached this way; the other converted languages come
from apt, which isn't cached (restoring it needs root), so caching gets
better as more languages move to downloaded toolchains.

```bash
hct install --plan zig    # what would install, and how
hct workflows             # with "ci": {"cache": true}, caches what can be
```

An installer that runs a converted language's `install.sh` keeps working:
that script starts with `install_from_data <lang>`, which hands over to the
binary when there is one. Gleam's data runs Erlang's `install.sh` as a step
when the `erl` on PATH is older than OTP 26.

## Shared toolchains

Some languages share a toolchain. Uninstalling a language removes only what
is its own and was installed by hecatron: a tool another language's
installer also installs stays, and the uninstaller prints a line saying so
(`Left CMake installed: C, C++, Objective-C, COBOL and Fortran share it.`).
Remove one of these by hand once no language needs it.

| Toolchain       | Languages                              | Uninstalling any one of them                     |
| --------------- | -------------------------------------- | ------------------------------------------------ |
| .NET SDK        | C#, F#, VB.NET                         | Leaves the SDKs (`/opt/dotnet`, `~/.local/share/hecatron/dotnet`) and their views |
| cmake           | C, C++, Objective-C, COBOL, Fortran    | Leaves cmake                                     |
| JDK             | Java, Kotlin, Scala, Groovy, Clojure   | Leaves the JDKs; `hct uninstall java` only prints where they are |
| Node.js         | TypeScript, JavaScript, Elm            | Leaves Node.js (JavaScript's installer is TypeScript's; Elm's installs Node if missing) |
| Erlang/OTP      | Erlang, Elixir, Gleam                  | Leaves the distribution's or Homebrew's Erlang/OTP; Erlang's removes rebar3 and its own OTP build (Elixir's removes the OTPs it installed for itself, see below) |
| uv              | Python, C, C++, Objective-C, Mojo      | Leaves uv; Python's removes uv's Python builds, unless Mojo runs on one |
| gcc (Homebrew)  | Fortran                                | Leaves Homebrew's gcc, which other formulae use  |

Nothing an operating system depends on is removed: Python's uninstaller
leaves the distribution's `python3`, and none runs `apt-get autoremove`.

## Architecture limitations

Some languages lack pre-built binaries on certain architectures. The installer system detects this and skips unavailable languages with a clear message.

| Language   | x86_64 Linux | arm64 Linux | macOS (any) | Notes                                     |
| ---------- | ------------ | ----------- | ----------- | ----------------------------------------- |
| Elm        | Yes          | No          | Yes         | No arm64 Linux binary (elm/compiler#2292) |
| All others | Yes          | Yes         | Yes         |                                           |

`hct get-started` shows unavailable languages with `–` instead of `✓`/`✗` in the status table.

## Platform behavior

### macOS

Most languages install via Homebrew (including Dart via `dart-sdk`). Exceptions:
- **Perl** — uses system Perl (no install needed)
- **Swift** — uses Xcode Swift (no install needed)
- **Rust** — installed via rustup (not Homebrew, to avoid conflicts)
- **Haskell** — installed via ghcup (GHC + cabal)
- **Elm** — installed via `npm install -g` (requires Node.js)
- **Gleam, Zig** — the same release download as on Linux, into `~/.local/share/hecatron/toolchains`

### Linux

Languages install via a mix of apt packages and direct downloads:

| Method           | Languages                                                                         |
| ---------------- | --------------------------------------------------------------------------------- |
| apt packages     | Ada, C/C++ (cmake), Erlang (off Ubuntu), Fortran, Lua, Perl, PHP (Ondřej Surý's packages) |
| Direct download  | Dart, Elixir and Erlang (builds.hex.pm), Gleam, Go, Java/Kotlin/Scala/Groovy (Temurin), Node.js, Swift, Zig |
| Installer script | Clojure, Haskell (ghcup), Rust (rustup), Python (uv), .NET (dotnet-install.sh)    |
| Built from source | Ruby (ruby-build)                                                                |
| npm global       | Elm                                                                               |

**Erlang** on Ubuntu is builds.hex.pm's OTP build at the version
`config.json` pins (OTP 29), in
`~/.local/share/hecatron/toolchains/erlang/<version>` with its commands
linked into `~/.local/share/hecatron/bin`, and rebar3's release escript
there too: Ubuntu's own Erlang is too old for current
libraries and Gleam's standard library, which need OTP 26 (24.04's is 25).
builds.hex.pm has no Debian builds, so Debian gets its distribution's
Erlang and rebar3 (Debian 13's OTP 27), and `hct doctor erlang` notes
an OTP older than 26. macOS gets Homebrew's `erlang` and `rebar3`.

**Elixir** comes from builds.hex.pm's precompiled releases, one directory
per version (`/opt/elixir/<version>`, `~/.local/share/hecatron/elixir/<version>`
on macOS). Each release is built for a few OTP majors (1.20 for 27 to 29),
so each version runs on an OTP it supports, linked as `<home>/erlang`: the
Erlang on PATH when it is one (Erlang's installer's, above), else one
installed beside it, builds.hex.pm's OTP build in `/opt/erlang/<version>`
on Ubuntu or Homebrew's `erlang@N` on macOS. Debian has no builds there, so
a Debian machine runs the Elixirs its own Erlang supports (Debian 13's OTP 27 runs
1.17 to 1.20), and the installer says so for one it can't. Each version
has its own Hex in `<home>/archives` (`MIX_ARCHIVES`), since an archive
built on one OTP doesn't load on an older one, so only these Elixirs count
as installed, not the distribution's or Homebrew's. `hct doctor
elixir` notes each pairing it finds ("a: Elixir 1.20.4 on OTP 27.3.4.3").

### Uninstall caveats

| Language         | Caveat                                                                 |
| ---------------- | ---------------------------------------------------------------------- |
| Java/Kt/Scala/Gy | No-op — JDK is shared; prints message instead of removing              |
| C, C++, C#, F#, VB.NET, JavaScript, TypeScript | No-op apart from the message — their toolchain is shared (above) |
| PHP (Linux)      | Removes the `php*-cli`, `-xml` and `-mbstring` packages it installs; what apt pulled in with them stays for `apt-get autoremove` |
| Perl             | No-op — the system's Perl                                              |
| Swift (macOS)    | No-op — Xcode's Swift                                                  |

How an installer is written: [installer scripts](../contributing/installer-scripts.md).
