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
```

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
- `$OS` — `"macos"` or `"linux"`
- `$SUDO` — `"sudo"` or `""` (when root)

Each language has up to four scripts:

| Script         | Purpose                            | Required |
| -------------- | ---------------------------------- | -------- |
| `install.sh`   | Install toolchain + deps           | Yes      |
| `uninstall.sh` | Remove toolchain                   | Yes      |
| `version.sh`   | Update version in source/manifests | Yes      |
| `metadata.sh`  | Update package metadata            | No       |

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
