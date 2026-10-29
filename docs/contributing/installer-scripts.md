# Installer scripts

How a language's `install.sh`, `uninstall.sh` and `doctor.sh` are
written. Using them is in
[installing toolchains](../guide/installing-toolchains.md); installs as
data in [the install engine](install-engine.md).

## Installer script structure

Each script sources `languages/_common.sh` for shared helpers:

- `has()` — check if a command exists
- `apt_install()` — apt-get install with auto cache refresh
- `brew_install()` — brew install with `|| true`
- `brew_ensure_linked <formula> <command>` — link a formula Homebrew installed but left unlinked
- `impl_dirs <lang>` — each component's directory for the language (from the project's `components` and `implPath`), one per line
- `each_impl <lang> <command...>` — run a command in each of them (e.g. `each_impl ruby bundle install`)
- `each_impl_versioned <lang> <command...>` — the same, with each component's toolchain version on PATH
- `impl_deps <lang>` — install each component's dependencies with `hct deps` (its deps hook, held to the lock file); when `hct install` installs every language, after all the installers have run
- `impl_configures <lang> <file...>` — whether a component (or a directory above it) has one of the files, for a tool installed only where it's configured
- `release_binary_install <name> <url> <sha256> [path]` — a pinned release download, checked against its checksum, into hecatron's bin directory (`~/.local/share/hecatron/bin`; no root)
- `uv_install` — uv into `~/.local/bin`
- `install_from_data <lang>` — first thing in a converted language's `install.sh` that other installers run: install from `install.json` through hecatron's binary, when there is one
- `problem` / `doctor_done` — for `doctor.sh`: report a problem; exit non-zero if any were reported
- `$OS` — `"macos"` or `"linux"`
- `hecatron_bin` — hecatron's bin directory (`~/.local/share/hecatron/bin`), first on PATH: put or link a tool there rather than in `/usr/local/bin`, which needs root
- `unlink_into <dir>` — for `uninstall.sh`: remove the links into a directory from hecatron's bin directory and `/usr/local/bin`
- `$SUDO` — `"sudo"` or `""` (when root)
- `$ROOT` — the repository root

A language definition has these files (see
[adding a language](adding-a-language.md)):

| File           | Purpose                                                       | Required |
| -------------- | ------------------------------------------------------------- | -------- |
| `config.json`  | Dependency commands, abbreviation, detection globs, PATH entries | Yes   |
| `install.json` | Install as data, which the binary runs (above)                | No       |
| `install.sh`   | Install toolchain + deps; idempotent, repairs                 | Yes      |
| `uninstall.sh` | Remove toolchain                                              | Yes      |
| `doctor.sh`    | Detect breakages beyond a missing command (`hct doctor`) | No       |
| `dockerfile`   | Snippet for `hct dockerfile`                             | No       |

A project adds hooks for its own code in `.hecatron/languages/<lang>/`:

| Hook           | Purpose                                                       |
| -------------- | ------------------------------------------------------------- |
| `test.sh`      | Test one component (`hct test <lang>`)                   |
| `lint.sh`      | Lint one component, if there's a separate linter              |
| `build.sh`     | Build one component, if there's a build step                  |
| `clean.sh`     | Remove one component's build artifacts                        |
| `version.sh`   | Update the version in source and manifests (`hct version`) |
| `metadata.sh`  | Update package metadata (`hct metadata`)                 |

`hct doctor` already checks that the `dependency` and `lintDependency`
commands from `config.json` exist. Add a `doctor.sh` when a toolchain can be
present but broken, and make `install.sh` repair that state:

```bash
#!/usr/bin/env bash
# Check that busted and luacheck start (luacheck needs Lua 5.4; see install.sh).
source "$(dirname "$0")/../_common.sh"

busted --version >/dev/null 2>&1 || problem "busted fails to start"
doctor_done
```

Project dependencies install into every component's implementation through
the language's deps hook: `deps() { impl_deps ruby; }`.

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
