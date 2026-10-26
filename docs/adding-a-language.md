# Adding a language

A language definition teaches hecatron how to install, check and package a
language's toolchain. Built-in definitions live in `languages/<lang>/` in this
repository; a project can add its own (or override any file) in
`.hecatron/languages/<lang>/` with the same layout. See
[design.md §7](design.md#7-resolving-a-languages-files) for how the two
combine.

## Files

| File | Purpose | Required |
| --- | --- | --- |
| `config.json` | `abbreviation`, `dependency` (the command that proves the toolchain is installed), optional `lintDependency`, `detect` (manifest globs `hecatron init` looks for), and optional `path` (toolchain directories to put on PATH for this language only, e.g. `["~/.cargo/bin"]`) | Yes |
| `install.sh` | Install the toolchain on macOS and Linux; idempotent, and repairs what `doctor.sh` detects | Yes |
| `uninstall.sh` | Remove the toolchain (a no-op with a message when other languages share it) | Yes |
| `doctor.sh` | Detect a toolchain that is installed but broken, one `problem` per issue | No |
| `dockerfile` | Snippet for `hecatron dockerfile`: `# apt:` packages, `# after: <lang>`, `RUN`/`ENV` blocks | No |
| `hooks/test.sh`, `lint.sh`, `build.sh`, `clean.sh` | Default hooks for a conventional layout (design.md §9) | No |

`config.json`:

```json
{
  "abbreviation": "rs",
  "dependency": "cargo",
  "lintDependency": null,
  "detect": ["Cargo.toml"]
}
```

## Installers

Every script sources `languages/_common.sh`:

```bash
#!/usr/bin/env bash
# Install Rust
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install rustup; rustup-init -y --no-modify-path; }
install_linux() { has cargo || curl -fsSL https://sh.rustup.rs | sh -s -- -y; }
deps() { :; }

log "Installing Rust..."
"install_${OS}"
deps
```

`_common.sh` provides `$OS`, `$SUDO`, `$ROOT` (the project), `log`, `has`,
`brew_install`, `brew_ensure_linked`, `apt_install`, `impl_dirs`,
`each_impl` (run a command in every component's implementation, e.g.
`each_impl ruby bundle install`), and `problem`/`doctor_done` for doctor
scripts. See [installers.md](installers.md) for platform behaviour and shared
toolchains.

## Hooks

A hook runs once per component, from inside that component's implementation
directory, with the component name as `$1` and `HECATRON_ROOT`,
`HECATRON_COMPONENT`, `HECATRON_LANG` set. It exits non-zero on failure.
`clean.sh` removes build artifacts only, never installed dependencies or
tracked files.

## Checklist

- [ ] `config.json` with `dependency` and `detect`
- [ ] `install.sh` and `uninstall.sh` for macOS and Linux, idempotent
- [ ] `doctor.sh` if the toolchain can be present but broken
- [ ] `dockerfile` snippet
- [ ] `bash libexec/lint/shellcheck.sh` and `python3 -m unittest discover -s tests` pass
