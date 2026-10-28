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
| `config.json` | `abbreviation`, `dependency` (the command that proves the toolchain is installed), optional `lintDependency`, `detect` (build-file globs `hecatron init` looks for), optional `detectSources` (source globs, used where no build file is found, and to tell whether a directory named for the language holds its code), optional `detectUnless` (files that, beside a hit, mean another language), optional `detectScript` (an npm script a `package.json` hit must have, e.g. JavaScript's `"test"`, so a tooling package isn't a language), optional `sharedBuildFiles` (build files other languages use too, such as `CMakeLists.txt`, which count for the language with the most `detectSources` files beside them or under their `src/` and `include/`), optional `dirNames` (other names the language's directory goes by, e.g. `["dotnet", "netstd"]` for C#), optional version keys (`version`, `pinFiles`, `supportFiles`; see [design.md §18](design.md), where a `supportFiles` entry with `"selects": false` says what to install but not what runs, as for .NET, whose `dotnet` runs its newest SDK, and a `pinFiles` or `supportFiles` entry's `"strip"` is a regular expression, in the syntax Python's `re` and Go's `regexp` share, removed from the version the file gives, as `"-otp-[0-9]+$"` reads asdf's Elixir `1.17.3-otp-27` as 1.17.3), optional `path` (toolchain directories to put on PATH for this language only, e.g. `["~/.cargo/bin"]`), optional `sdkrootFromXcrun` (on macOS, set `SDKROOT` to the selected Xcode's SDK, `xcrun --show-sdk-path`, when it isn't set: Ada's, since GNAT's gcc links against the Command Line Tools' SDK, which can be newer than the selected Xcode's `ld` knows, where clang asks xcrun),, optional `unsupported` (platforms with no toolchain upstream, e.g. `["linux-aarch64"]`, shown as ➖ in the [coverage matrix](coverage.md)), optional `cache` (what generated CI caches with `"ci": {"cache": true}`, [design.md §11](design.md#11-generated-files): `toolchain`, the directories the installer puts the toolchain in, keyed on the pin and support files; `dependencies`, where the package manager downloads to, keyed on `lockFiles`; each directory either `~/...` or relative to a component's directory, none inside another, and only ones an unprivileged user can restore, so not apt's or `/opt`'s; e.g. Rust's `{"toolchain": ["~/.rustup", "~/.cargo/bin"], "dependencies": ["~/.cargo/registry", "~/.cargo/git"], "lockFiles": ["Cargo.lock"]}`), optional `dependabot` (the [Dependabot package ecosystem](https://docs.github.com/en/code-security/dependabot/working-with-dependabot/dependabot-options-reference#package-ecosystem-) a component's directory is updated as: a name, `"cargo"`, or rules each taking the directory when one of its `files` is there and none of its `unless` is, e.g. Python's `[{"ecosystem": "uv", "files": ["uv.lock"]}, {"ecosystem": "pip", "unless": ["uv.lock"]}]`; package managers only, not toolchain pins), and optional `renovate` (the [Renovate managers](https://docs.renovatebot.com/modules/manager/) for its dependencies, e.g. `["cargo"]`) | Yes |
| `install.json` | The install as data, which hecatron's binary runs in place of `install.sh` (below; [installers.md](installers.md#installs-as-data)) | No |
| `install.sh` | Install the toolchain on macOS and Linux; idempotent, and repairs what `doctor.sh` detects | Yes |
| `uninstall.sh` | Remove the toolchain (a no-op with a message when other languages share it) | Yes |
| `doctor.sh` | Detect a toolchain that is installed but broken, one `problem` per issue; `note` (a line starting `note: `) says something worth knowing that isn't a problem, such as which runtime a component's toolchain runs on, shown under the language without failing it | No |
| `installed.sh` | The versions installed on this machine, one `version<TAB>home` line each; offline and fast. The core picks the version a component needs from it ([design.md §18](design.md)) | No |
| `releases.sh` | The language's releases, newest first, as `version<TAB>flags` (`lts`, `eol`); `endoflife_releases <product>` from `_common.sh` covers languages endoflife.date tracks | No |
| `dockerfile` | Snippet for `hecatron dockerfile`: `# apt:` packages, `# after: <lang>`, `RUN`/`ENV` blocks, an `# each-version` … `# end-each-version` block repeated per version ([design §18](design.md#18-toolchain-versions)) | No |
| `advice.json` | `hct advisor`'s rules for the language: its ecosystem's own guidance on lock files, pins, end of life, linter configuration and CI, each citing that ecosystem's documentation, or `include`s of shared sets in `languages/_advice.json` ([advisor.md](advisor.md)); each rule needs a fixture in `internal/advisor/testdata/rules.json` | No |
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

## Installs as data

Where the installer only installs system packages (with conditions, and
commands guarded by checks), write `install.json` as well
([design.md §19](design.md#installs-as-data-77)): the binary installs from
it, batches its packages with other languages', checks and repairs it in
`hct doctor`, and uninstalls from its receipt. Keep `install.sh`,
`uninstall.sh` and `doctor.sh` doing the same, for `bin/hecatron` without
the binary. Racket's:

```json
{
  "schema": 1,
  "toolchain": { "versions": "system", "packages": { "apt": ["racket"], "brew": ["minimal-racket"] } },
  "steps": [
    { "name": "compiler-lib",
      "run": ["raco", "pkg", "install", "--auto", "--skip-installed", "compiler-lib"],
      "check": { "run": ["raco", "test", "--help"] },
      "problem": "raco test is missing (package compiler-lib)" }
  ]
}
```

- `toolchain.packages`: per package manager (`apt`, `brew`); the language's
  `dependency` command proves it.
- `packages`: more packages, with `when` (install only if it holds) and
  `unless` (already satisfied).
- `tools`: `name`, `packages`, `when` (with `because`, the doctor's reason)
  and `check`, for a program the hooks need.
- `steps`: `run`, `check` and `problem`, run after the packages when the
  check fails.
- `uninstall.keep` (with `because`): packages uninstall leaves, shared with
  languages still installed by scripts.
- Conditions: `file`, `missing`, `glob`, `command`, `run`, `platform`,
  `all`, `any`, `not`, one per condition.

If another language's installer runs this `install.sh`, start it with
`install_from_data <lang>`. Check the two agree with
`tests/docker/parity.sh ubuntu:24.04 <lang>` (add project files that make
a conditional tool install under `tests/docker/parity/<lang>/`), and
`hct install --plan <lang>` shows what the data would do.

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
      `formatting`, each just the tools' names. A condition (only with a
      config file, only when the build applies it) goes in `notes`, keyed by
      the column (`"notes": {"formatting": "Only when the project has a
      .ocamlformat."}`), and becomes a footnote. Each tool named gets its
      site in `libexec/codegen/tool-links.json`. Then run
      `python3 libexec/codegen/languages_doc.py`
- [ ] `install.sh` and `uninstall.sh` for macOS and Linux, idempotent, and
      `install.json` when the install is only system packages, checked
      with `tests/docker/parity.sh`
- [ ] `doctor.sh` if the toolchain can be present but broken
- [ ] `dockerfile` snippet
- [ ] `advice.json` with the ecosystem's own recommendations, if it has
      any ([advisor.md](advisor.md))
- [ ] `tests/docker/smoke/<lang>.sh`, a smoke test that builds and runs a small
      program and checks its output (and runs the language's linter, where
      hecatron installs one), and `tests/docker/install-test.sh ubuntu:24.04 <lang>`
      passing
- [ ] The language count updated wherever the docs state it: `grep -rn
      language-count` lists every line (the self test fails on any it misses)
- [ ] `hct self test` passes
