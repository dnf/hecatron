# Language Installers

Per-language installer scripts live in `.hecatron/languages/<lang>/` as separate single-purpose files. Each script handles both macOS and Linux, and runs its action on execute (no subcommands).

## Commands

```bash
just install                # install all available languages
just install go rust        # install specific languages
just uninstall go rust      # uninstall specific languages
just get-started            # interactive onboarding with status + install menu
just get-started --status   # print language status table
just get-started --check    # exit 0 if all installed, 1 if missing
just doctor                 # check every toolchain actually works, and how to repair it
just doctor --fix go lua    # run the installer of each unhealthy language, then re-check
```

`just install <lang>` is also the repair: installers are idempotent and fix the
broken states their `doctor.sh` detects (a linter built with an older Go,
luacheck on Lua 5.5, a moved virtualenv, an unlinked Homebrew formula...).

Short aliases work everywhere: `just install ad ts py rb rs kt gy lu ft c++ cs fs sw hs ex erl clj gm sc zg el pl dt ml`

Misspelled names get a "did you mean?" prompt (e.g. `just install elixer` suggests `elixir`).

## Shared toolchains

Some languages share a toolchain. Uninstalling one affects the others.

| Toolchain  | Languages             | Effect of uninstalling any one                                          |
| ---------- | --------------------- | ----------------------------------------------------------------------- |
| .NET SDK   | C#, F#                | Both share the same SDK; uninstalling one removes the other             |
| cmake      | C, C++, Fortran       | All three lose build capability until reinstalled                       |
| JDK 17     | Java, Kt, Scala, Gy   | Protected: `just uninstall java` is a no-op (prints a message)          |
| Node.js    | TypeScript, Elm       | Elm depends on npm; Elm installer auto-installs Node if missing         |
| Erlang/OTP | Erlang, Elixir, Gleam | Elixir and Gleam depend on Erlang runtime; uninstall Erlang breaks both |

## Architecture limitations

Some languages lack pre-built binaries on certain architectures. The installer system detects this and skips unavailable languages with a clear message.

| Language   | x86_64 Linux | arm64 Linux | macOS (any) | Notes                                     |
| ---------- | ------------ | ----------- | ----------- | ----------------------------------------- |
| Elm        | Yes          | No          | Yes         | No arm64 Linux binary (elm/compiler#2292) |
| All others | Yes          | Yes         | Yes         |                                           |

`just get-started` shows unavailable languages with `–` instead of `✓`/`✗` in the status table.

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
| apt packages     | Ada, C/C++ (cmake), C#/F# (dotnet), Elixir, Erlang, Fortran, Lua, Perl, PHP, Ruby |
| Direct download  | Dart, Gleam, Go, Swift, Zig                                                       |
| Installer script | Clojure, Haskell (ghcup), Rust (rustup), Python (uv)                              |
| npm global       | Elm, TypeScript (Node via NodeSource apt)                                         |
| Shared           | Java/Kotlin/Scala/Groovy (OpenJDK 17 via apt)                                     |

### Uninstall caveats

| Language         | Caveat                                                                 |
| ---------------- | ---------------------------------------------------------------------- |
| Java/Kt/Scala/Gy | No-op — JDK is shared; prints message instead of removing              |
| Perl             | No-op on macOS (system Perl); removed on Linux                         |
| Swift (Linux)    | No-op — Swift installs many files in /usr/local; manual cleanup needed |

## Installer script structure

Each script sources `.hecatron/languages/_common.sh` for shared helpers:

- `has()` — check if a command exists
- `apt_install()` — apt-get install with auto cache refresh
- `brew_install()` — brew install with `|| true`
- `brew_ensure_linked <formula> <command>` — link a formula Homebrew installed but left unlinked
- `impl_dirs <lang>` — each library's `<library>/impl/<lang>/` directory, one per line
- `each_impl <lang> <command...>` — run a command in each of them (e.g. `each_impl ruby bundle install`)
- `problem` / `doctor_done` — for `doctor.sh`: report a problem; exit non-zero if any were reported
- `$OS` — `"macos"` or `"linux"`
- `$SUDO` — `"sudo"` or `""` (when root)
- `$ROOT` — the repository root

Each language has these scripts:

| Script         | Purpose                                                 | Required |
| -------------- | ------------------------------------------------------- | -------- |
| `install.sh`   | Install toolchain + deps; idempotent, repairs           | Yes      |
| `uninstall.sh` | Remove toolchain                                        | Yes      |
| `version.sh`   | Update version in source/manifests                      | Yes      |
| `test.sh`      | Run one library's native suite (`just test <lang>`)     | Yes      |
| `lint.sh`      | Lint one library (`just lint <lang>`), if it has a linter | No     |
| `build.sh`     | Build one library (`just build <lang>`), if it has a build step | No |
| `clean.sh`     | Remove one library's build artifacts (`just clean <lang>`) | No   |
| `doctor.sh`    | Detect breakages beyond a missing command (`just doctor`) | No     |
| `metadata.sh`  | Update package metadata                                 | No       |

`just doctor` already checks that the `dependency` and `lintDependency`
commands from `config.json` exist. Add a `doctor.sh` when a toolchain can be
present but broken, and make `install.sh` repair that state:

```bash
#!/usr/bin/env bash
# Check that busted and luacheck start (luacheck needs Lua 5.4; see install.sh).
source "$(dirname "$0")/../_common.sh"

busted --version >/dev/null 2>&1 || problem "busted fails to start"
doctor_done
```

Library dependencies install into every library's implementation, not just
age's: `deps() { each_impl ruby bundle install; }`.

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
