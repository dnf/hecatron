# Language Installers

Each language's definition lives in `languages/<lang>/` in this repository (a project can override any file in its own `.hecatron/languages/<lang>/`), as separate single-purpose files. Each script handles both macOS and Linux, and runs its action on execute (no subcommands).

## Commands

```bash
hecatron install                # install all available languages
hecatron install go rust        # install specific languages
hecatron uninstall go rust      # uninstall specific languages
hecatron get-started            # interactive onboarding with status + install menu
hecatron get-started --status   # print language status table
hecatron get-started --check    # exit 0 if all installed, 1 if missing
hecatron doctor                 # check every toolchain actually works, and how to repair it
hecatron doctor --fix go lua    # run the installer of each unhealthy language, then re-check
```

`hecatron install <lang>` is also the repair: installers are idempotent and fix the
broken states their `doctor.sh` detects (a linter built with an older Go,
luacheck on Lua 5.5, a moved virtualenv, an unlinked Homebrew formula...).

Short aliases work everywhere: `hecatron install ad ts py rb rs kt gy lu ft c++ cs fs sw hs ex erl clj gm sc zg el pl dt ml`

Misspelled names get a "did you mean?" prompt (e.g. `hecatron install elixer` suggests `elixir`).

## Shared toolchains

Some languages share a toolchain. Uninstalling one affects the others.

| Toolchain  | Languages             | Effect of uninstalling any one                                          |
| ---------- | --------------------- | ----------------------------------------------------------------------- |
| .NET SDK   | C#, F#                | Both share the same SDK; uninstalling one removes the other             |
| cmake      | C, C++, Fortran       | All three lose build capability until reinstalled                       |
| JDK        | Java, Kt, Scala, Gy   | Protected: `hecatron uninstall java` is a no-op (prints a message)      |
| Node.js    | TypeScript, JS, Elm   | JavaScript's installer is TypeScript's; Elm's installs Node if missing  |
| Erlang/OTP | Erlang, Elixir, Gleam | Elixir and Gleam depend on Erlang runtime; uninstall Erlang breaks both |

## Architecture limitations

Some languages lack pre-built binaries on certain architectures. The installer system detects this and skips unavailable languages with a clear message.

| Language   | x86_64 Linux | arm64 Linux | macOS (any) | Notes                                     |
| ---------- | ------------ | ----------- | ----------- | ----------------------------------------- |
| Elm        | Yes          | No          | Yes         | No arm64 Linux binary (elm/compiler#2292) |
| All others | Yes          | Yes         | Yes         |                                           |

`hecatron get-started` shows unavailable languages with `–` instead of `✓`/`✗` in the status table.

## Platform behavior

### macOS

Most languages install via Homebrew (including Dart via `dart-sdk`). Exceptions:
- **Perl** — uses system Perl (no install needed)
- **Swift** — uses Xcode Swift (no install needed)
- **Rust** — installed via rustup (not Homebrew, to avoid conflicts)
- **Haskell** — installed via ghcup (GHC + cabal)
- **Elm** — installed via `npm install -g` (requires Node.js)

### Linux

Languages install via a mix of apt packages and direct downloads:

| Method           | Languages                                                                         |
| ---------------- | --------------------------------------------------------------------------------- |
| apt packages     | Ada, C/C++ (cmake), C#/F# (dotnet), Elixir, Erlang, Fortran, Lua, Perl, PHP (Ondřej Surý's packages) |
| Direct download  | Dart, Gleam, Go, Java/Kotlin/Scala/Groovy (Temurin), Node.js, Swift, Zig          |
| Installer script | Clojure, Haskell (ghcup), Rust (rustup), Python (uv)                              |
| Built from source | Ruby (ruby-build)                                                                |
| npm global       | Elm                                                                               |

### Uninstall caveats

| Language         | Caveat                                                                 |
| ---------------- | ---------------------------------------------------------------------- |
| Java/Kt/Scala/Gy | No-op — JDK is shared; prints message instead of removing              |
| Perl             | No-op on macOS (system Perl); removed on Linux                         |
| Swift (Linux)    | No-op — Swift installs many files in /usr/local; manual cleanup needed |

## Installer script structure

Each script sources `languages/_common.sh` for shared helpers:

- `has()` — check if a command exists
- `apt_install()` — apt-get install with auto cache refresh
- `brew_install()` — brew install with `|| true`
- `brew_ensure_linked <formula> <command>` — link a formula Homebrew installed but left unlinked
- `impl_dirs <lang>` — each component's directory for the language (from the project's `components` and `implPath`), one per line
- `each_impl <lang> <command...>` — run a command in each of them (e.g. `each_impl ruby bundle install`)
- `each_impl_versioned <lang> <command...>` — the same, with each component's toolchain version on PATH (for dependencies built for one version: bundles, Composer locks)
- `problem` / `doctor_done` — for `doctor.sh`: report a problem; exit non-zero if any were reported
- `$OS` — `"macos"` or `"linux"`
- `$SUDO` — `"sudo"` or `""` (when root)
- `$ROOT` — the repository root

A language definition has these files (see
[adding-a-language.md](adding-a-language.md)):

| File           | Purpose                                                       | Required |
| -------------- | ------------------------------------------------------------- | -------- |
| `config.json`  | Dependency commands, abbreviation, detection globs, PATH entries | Yes   |
| `install.sh`   | Install toolchain + deps; idempotent, repairs                 | Yes      |
| `uninstall.sh` | Remove toolchain                                              | Yes      |
| `doctor.sh`    | Detect breakages beyond a missing command (`hecatron doctor`) | No       |
| `dockerfile`   | Snippet for `hecatron dockerfile`                             | No       |

A project adds hooks for its own code in `.hecatron/languages/<lang>/`:

| Hook           | Purpose                                                       |
| -------------- | ------------------------------------------------------------- |
| `test.sh`      | Test one component (`hecatron test <lang>`)                   |
| `lint.sh`      | Lint one component, if there's a separate linter              |
| `build.sh`     | Build one component, if there's a build step                  |
| `clean.sh`     | Remove one component's build artifacts                        |
| `version.sh`   | Update the version in source and manifests (`hecatron version`) |
| `metadata.sh`  | Update package metadata (`hecatron metadata`)                 |

`hecatron doctor` already checks that the `dependency` and `lintDependency`
commands from `config.json` exist. Add a `doctor.sh` when a toolchain can be
present but broken, and make `install.sh` repair that state:

```bash
#!/usr/bin/env bash
# Check that busted and luacheck start (luacheck needs Lua 5.4; see install.sh).
source "$(dirname "$0")/../_common.sh"

busted --version >/dev/null 2>&1 || problem "busted fails to start"
doctor_done
```

Project dependencies install into every component's implementation:
`deps() { each_impl ruby bundle install; }`.

Templates:

**install.sh:**
```bash
#!/usr/bin/env bash
source "$(dirname "$0")/../_common.sh"

install_macos() { ... }
install_linux() { ... }
deps() { ... }  # project-specific setup (npm install, etc.)

log "Installing Foo..."
"install_${OS}"
deps
```

**uninstall.sh:**
```bash
#!/usr/bin/env bash
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { ... }
uninstall_linux() { ... }

log "Uninstalling Foo..."
"uninstall_${OS}"
```

**version.sh:**
```bash
#!/usr/bin/env bash
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
# sed commands to update version in source and manifests
```

**metadata.sh** (only for languages with package manifests):
```bash
#!/usr/bin/env bash
source "$(dirname "$0")/../_common.sh"

# update package metadata using PT_* env vars
```
