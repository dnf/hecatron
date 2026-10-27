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
| `config.json` | `abbreviation`, `dependency` (the command that proves the toolchain is installed), optional `lintDependency`, `detect` (build-file globs `hecatron init` looks for), optional `detectSources` (source globs, used only where no build file is found), optional `detectUnless` (files that, beside a hit, mean another language), optional `sharedBuildFiles` (build files other languages use too, such as `CMakeLists.txt`, which count for the language with the most `detectSources` files beside them or under their `src/` and `include/`), optional `dirNames` (other names the language's directory goes by, e.g. `["dotnet", "netstd"]` for C#), optional `path` (toolchain directories to put on PATH for this language only, e.g. `["~/.cargo/bin"]`), and optional `unsupported` (platforms with no toolchain upstream, e.g. `["linux-aarch64"]`, shown as – in the [coverage matrix](coverage.md)) | Yes |
| `install.sh` | Install the toolchain on macOS and Linux; idempotent, and repairs what `doctor.sh` detects | Yes |
| `uninstall.sh` | Remove the toolchain (a no-op with a message when other languages share it) | Yes |
| `doctor.sh` | Detect a toolchain that is installed but broken, one `problem` per issue | No |
| `installed.sh` | The versions installed on this machine, one `version<TAB>home` line each; offline and fast. The core picks the version a component needs from it ([design.md §18](design.md)) | No |
| `releases.sh` | The language's releases, newest first, as `version<TAB>flags` (`lts`, `eol`); `endoflife_releases <product>` from `_common.sh` covers languages endoflife.date tracks | No |
| `dockerfile` | Snippet for `hecatron dockerfile`: `# apt:` packages, `# after: <lang>`, `RUN`/`ENV` blocks, an `# each-version` … `# end-each-version` block repeated per version ([design §18](design.md#18-toolchain-versions)) | No |
| `hooks/test.sh`, `lint.sh`, `build.sh`, `clean.sh`, `audit.sh`, `format.sh`, `deps.sh`, `outdated.sh` | Default hooks for a conventional layout (design.md §9) | No |

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
`each_impl ruby bundle install`), `impl_deps` (a component's dependencies,
through its deps hook), and `problem`/`doctor_done` for doctor
scripts. See [installers.md](installers.md) for platform behaviour and shared
toolchains.

## Hooks

A hook runs once per component, from inside that component's implementation
directory, with the component name as `$1` and `HECATRON_ROOT`,
`HECATRON_COMPONENT`, `HECATRON_LANG` set. It exits non-zero on failure.
`clean.sh` removes build artifacts only, never installed dependencies or
tracked files. `format.sh` rewrites files with the language's formatter, or
with `HECATRON_FORMAT_CHECK=1` changes nothing and fails listing the files
it would change (`languages/_format.sh` has the shared helpers); a lint hook
whose linter already checks formatting calls `checks_format` so `hct lint`
doesn't check twice ([format.md](format.md)). `deps.sh` fetches the
component's dependencies, held to its lock file; where other hooks install
missing dependencies too, both call one `*_fetch` function in
`languages/_hooks.sh` (the rest are in `languages/_deps.sh`).

## Checklist

- [ ] `config.json` with `dependency` and `detect`, and for
      [languages.md](languages.md) `name`, `homepage`, `summary`,
      `buildSystems`, `testing`, `linting` and (with a format hook)
      `formatting`; then run
      `python3 libexec/codegen/languages_doc.py`
- [ ] `install.sh` and `uninstall.sh` for macOS and Linux, idempotent
- [ ] `doctor.sh` if the toolchain can be present but broken
- [ ] `dockerfile` snippet
- [ ] `tests/docker/smoke/<lang>.sh`, a smoke test that builds and runs a small
      program and checks its output (and runs the language's linter, where
      hecatron installs one), and `tests/docker/install-test.sh ubuntu:24.04 <lang>`
      passing
- [ ] The language count updated wherever the docs state it: `grep -rn
      language-count` lists every line (the self-test fails on any it misses)
- [ ] `hct self-test` passes
