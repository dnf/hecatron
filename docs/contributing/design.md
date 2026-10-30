# Hecatron design

Status: **accepted** (decisions in §17); phase 2 of §15 (the standalone CLI) is implemented, and §18 (toolchain versions) is accepted and implemented for the languages it lists. This repository was split out of
[dnf/sayable](https://github.com/dnf/sayable), where hecatron grew up as its
`.hecatron/` directory. This document is the design record of the tool you
install once and use in any repository (`hct` is a short alias for
`hecatron`):

```bash
brew install dnf/tap/hecatron  # later, from homebrew-core: brew install hecatron
hct init                       # set up the current repository
hct doctor --fix               # install and repair the toolchains it needs
hct test                       # every language's suites, in parallel
```

What hecatron does for a user (configuration, hooks, detection, generated
files, toolchain versions, output) is described in the
[user guide](../guide/README.md); the sections that described it here moved
there, and keep their numbers below as pointers, so older references still
resolve. This record keeps the goals, the decisions and their history, and
the plans.

## 1. What hecatron is

Hecatron manages **polyglot repositories**: one project implemented in many
programming languages (sayable has 51), possibly made of several
**components** (sayable has four libraries: age, date, duration and time),
each implemented in every language. It knows how to:

- install, uninstall, check and repair each language's toolchain on macOS and
  Linux;
- run a project's tests, linters, builds and cleans for one language, for
  every component, or for everything in parallel with a live progress display;
- keep generated infrastructure in sync: one CI workflow per language, a
  Dockerfile with every toolchain, and DO NOT EDIT banners on generated files;
- add, remove, enable and disable languages in a project.

It does not know anything about what the project's code does. Formatting
locales, fuzzing CLIs and benchmarks stay in the project.

## 2. Where it is today

In sayable, hecatron is 425 files under `.hecatron/`, driven by a 1,100-line
`justfile`:

| Part | Files | Generic? |
| --- | --- | --- |
| `tools/runner/run-parallel.sh` (parallel runner, live display, history) | 1 (+2 docs) | Yes, but invokes `just <action> <target>` |
| `tools/language/` (config bridge, add/remove/enable/disable/list, install/uninstall, doctor, per-library dispatch) | 19 | Mostly; computes the project root from its own location |
| `tools/codegen/` (banner, Dockerfile generator) | 2 | Yes, but writes `age/Dockerfile` |
| `tools/metadata/`, `tools/onboarding/`, `tools/bootstrap/`, `tools/lint/` | 5 | Mostly; bootstrap says "sayable-age" and installs `just` |
| `languages/<lang>/` toolchain definitions: `config.json`, `install.sh`, `uninstall.sh`, `doctor.sh`, `dockerfile`, `_common.sh` | ~215 | Yes |
| `languages/<lang>/` project hooks: `test.sh`, `lint.sh`, `build.sh`, `clean.sh`, `version.sh`, `metadata.sh`, `workflow.yml` | ~300 | No: they name sayable's files |
| `config/` (`added.json`, `languages.json`, gitignored `disabled.json`) | 2 | Project data |
| `justfile` recipes `test`, `lint`, `build`, `clean`, `rebuild`, `retest`, `install`, `uninstall`, `doctor`, `add`, `remove`, `enable`, `disable`, `list`, `dockerfile`, `metadata`, `get-started` | ~800 lines | The real CLI, written as just recipes |
| `justfile` recipes `benchmark`, `fuzz`, `generate`, `maintain`, `check`, `version` | ~400 lines | Sayable's own |

The coupling to fix:

1. **Location.** Every script finds the project by walking up from its own
   path (`ROOT = dirname(dirname(...(__file__)))`). Installed once, the tool
   lives elsewhere; the project root must come from the working directory.
2. **Hard-coded layout.** `age/Dockerfile`, `<library>/impl/<lang>`, the
   component list `age date duration time`, and "sayable" in banners and
   bootstrap text.
3. **The justfile is the CLI.** Argument parsing, help text, language
   resolution and dispatch live in just recipes; the runner calls back into
   `just`. A tool has to own its interface.
4. **Two kinds of files share `languages/<lang>/`.** Toolchain knowledge
   (how to install Go) is reusable; project hooks (how to run sayable's Go
   tests) are not.

## 3. Goals and non-goals

Goals:

- `brew install hecatron`, then `hecatron init` in any repository.
- One installed tool serves many projects, each configured by its own
  `.hecatron/` directory, committed with the project.
- Language toolchain knowledge (installers, doctors, Dockerfile snippets) is
  shared and improved in one place.
- Projects keep full control: every built-in behaviour can be overridden per
  project, per language.
- Sayable migrates with no loss of function.
- Works on macOS (Apple Silicon and Intel) and Linux (Debian/Ubuntu first).

Non-goals (for now):

- Windows. Installers and hooks are bash.
- Replacing language build tools. Hecatron calls `cargo`, `go`, `gradle`...;
  it doesn't build anything itself.
- Replacing `just`. Projects can keep a justfile for their own recipes and
  delegate hecatron's commands to it.
- Hermetic or reproducible toolchains (Nix-style). Hecatron installs with the
  platform's package manager and pins only where a project needs it.

## 4. Concepts

**Project.** A repository with a `.hecatron/` directory at its root. Commands
find it by walking up from the working directory, like git.

**Language.** A programming language the project is implemented in. Its
**definition** says how to install and check the toolchain. Hecatron ships
definitions for all 51 languages sayable uses; a project can override any
file or add its own languages.

**Component.** A separately buildable unit implemented in each language:
sayable's age, date, duration and time libraries. A single-component project
has one, implicitly. Each language's code for a component lives at a path
built from a template, e.g. `{component}/impl/{lang}`.

**Action.** Something done to one component in one language: `test`, `lint`,
`build`, `clean`, `audit`, `format`, `deps`, `outdated`. A project hook script implements it; hecatron runs it once
per component (the "every library" rule sayable adopted: a command for one
language acts on all of that language's components).

**Tool.** A non-language dependency of the project's checks, such as
`shellcheck` for `hecatron lint shell` or `cmake` for a CLI build.

## 5. The two repositories

### hecatron (this repository)

Target layout:

```
bin/hecatron                 # entry point: finds HECATRON_HOME, dispatches subcommands
libexec/
  commands/                  # one file per subcommand (init, test, doctor, add, ...)
  runner/run-parallel.sh     # parallel runner (was tools/runner)
  lib/                       # shared shell and Python helpers (was tools/language/*)
  codegen/                   # Dockerfile and workflow generators, banner
languages/<lang>/            # built-in definitions (was .hecatron/languages, minus hooks)
  config.json                # abbreviation, dependency, lintDependency, compiled, ...
  install.sh  uninstall.sh  doctor.sh  dockerfile
  hooks/                     # default test/lint/build/clean for a conventional layout (§9)
languages/_common.sh         # helpers for installers and hooks
templates/init/              # files `hecatron init` writes
docs/                        # this document, installers, adding a language, runner
tests/                       # hecatron's own tests (§14)
examples/sayable/            # snapshot of sayable's hooks and config, for reference
```

This layout is in place: `bin/hecatron` (and the `hct` link), `libexec/`
(`lib/project.py` is the project model every command reads through,
`commands/run.sh` the action runner, `commands/manage.py` the configuration
commands), `languages/`, `tests/`, and `examples/sayable/` with sayable's
hooks, CI templates and config as they were at the split. Default hooks
(`languages/<lang>/hooks/`) come in phase 4. The repository as it is now,
with the Go core: [architecture](architecture.md) and [modules](modules.md).

### A project using hecatron

Moved to the user guide: [configuration](../guide/configuration.md#the-hecatron-directory).

## 6. Project configuration

Moved to the user guide: [configuration](../guide/configuration.md#configjson).

## 7. Resolving a language's files

Moved to the user guide: [configuration](../guide/configuration.md#resolving-a-languages-files).

## 8. Command line

Moved to the user guide: the [command reference](../guide/commands.md). The
commands and the sayable `just` recipes or scripts they replaced:

| Command | From |
| --- | --- |
| `init`, `outdated`, `deps`, `format`, `--version`, `self` | new |
| `test`, `lint`, `build`, `clean`, `rebuild`, `retest` | `just test`, `just lint`, `just build`, `just clean`, same |
| `install`, `uninstall`, `doctor` | `just install`, `just doctor` |
| `add`, `remove`, `enable`, `disable`, `list languages` | the same recipes, `just list` |
| `dockerfile`, `workflows`, `check` | `just dockerfile`, `sync_workflows.py`, part of `just check` |
| `metadata`, `version`, `get-started` | `just metadata`, `just version` (age only then), `just get-started` |
| `banner`, `env`, `parallel` | `codegen/util.py`, `source helpers.sh`, `run-parallel.sh` |

**Commands about hecatron itself go under `hct self` (decided
2026-10-28),** with a space, as package managers do (`rustup self update`,
`uv self update`): `self update`, `self uninstall`, `self test`, `self build`, and any later
self-management command. `hct self` lists them. `self-test` and
`self-build`, their earlier names, still work, unlisted.

## 9. Hooks

Moved to the user guide: [hooks](../guide/hooks.md).

## 10. `hecatron init`

Moved to the user guide: [init and detection](../guide/init.md).

## 11. Generated files

Moved to the user guide: [generated files](../guide/generated-files.md).

## 12. Doctor and repair

Moved to the user guide: [installing toolchains](../guide/installing-toolchains.md#doctor-and-repair).

## 13. Distribution

**Homebrew.** `brew install hecatron` needs the formula in homebrew-core,
which accepts projects with some public adoption. Until then: a tap,
`dnf/homebrew-tap`, with `Formula/hecatron.rb`, installed as `brew install
dnf/tap/hecatron`. The formula:

- installs the release tarball into `libexec/` and a `bin/hecatron` wrapper
  that sets `HECATRON_HOME`;
- depends on `python@3` (macOS has no Python without the Command Line
  Tools); uses the system bash (3.2 or later) and perl;
- runs a `test do` block that inits a temporary repo and runs `hecatron
  doctor` on an empty language list.

**Linux and CI.** Homebrew on Linux works with the same formula. For CI, a
composite GitHub Action (`uses: dnf/hecatron@v1`) downloads a release and puts
it on `PATH`; there is also an `install.sh` for curl-based installs.

**Releases.** Semantic versions, tags `vX.Y.Z`, a GitHub release with the
tarball; a release workflow updates the tap formula's URL and checksum. A
project's `"hecatron": ">=x.y.z"` makes older installs fail with a clear
message instead of misbehaving.

The updates decision (notify and `hct self update`, never auto-update,
2026-10-28) and the project pin (2026-10-28) moved to the user guide:
[installing hecatron](../guide/installing-hecatron.md). The release process is in
[releasing](releasing.md).

## 14. Implementation and testing

**Language.** Bash for the entry point, installers, hooks and runner, and
Python 3 (standard library only) for config handling and generators, until
the Go core (§19) replaces them.

**Testing hecatron** moved to [testing](testing.md).

## 15. Migration of sayable

1. **Split (this repository's first commit).** Scripts, history and docs move
   here. Sayable keeps its vendored `.hecatron/` and keeps working unchanged.
2. **Standalone CLI (done).** §5–§12: project-root discovery, config schema,
   file resolution, `bin/hecatron` and the commands; the runner calls
   `hecatron`, not `just`. Tested against a copy of sayable migrated as in
   step 3: `test`, `lint`, `build`, `clean`, `doctor`, `check` and the
   generators match sayable's results (generated files differ only in banner
   text). One migration step for sayable surfaced: its locale codegen imports
   the banner module from `.hecatron/tools/codegen/util.py`, so it must keep
   its own copy (or use `hecatron banner`).
3. **Sayable consumes hecatron.** Sayable's `.hecatron/` shrinks to
   `config.json`, `local.json` and its hooks; its justfile delegates the
   hecatron recipes; its CI installs hecatron with the action; the vendored
   tools are deleted.
4. **Defaults and `init` for other projects.** Default hooks, detection and
   `--justfile`.
5. **Release 1.0 and homebrew-core.**

## 16. Security

Moved to the user guide: [hooks](../guide/hooks.md#trust).

## 17. Decisions

Settled on 2026-10-26:

1. **Config format: JSON.** `.hecatron/config.json` and `local.json`, read
   with the Python standard library, so nothing beyond Python 3.9 is needed.
2. **Name: components.** Config key `components`, path placeholder
   `{component}`, hook argument `$1` / `HECATRON_COMPONENT`.
3. **Primary interface: the `hecatron` CLI.** It owns arguments, help and
   dispatch; a project's justfile may delegate to it (`hecatron init
   --justfile`).
4. **Default hooks ship with hecatron.** Each language's standard
   test/lint/build/clean commands apply unless a project overrides them.
5. **Tap: `dnf/homebrew-tap`.** `brew install dnf/tap/hecatron` until
   homebrew-core accepts the formula.
6. **Language definitions live in this repository**, versioned and released
   with the CLI.
7. **Toolchain versions (§18).** A project chooses each language's version
   through the ecosystem's own pin file, else `version` in its config, else
   hecatron's default, held back to what the project supports; an old
   checkout gets the releases current at its last commit. `=` marks an exact version. Defaults are pinned, follow
   the language's LTS line where it has one and its newest stable release
   otherwise, and a weekly job updates them. (Owner's decision.)
8. **A Go core and native Windows (§19).** Hecatron becomes one compiled
   binary, starting fast and running natively on Windows, with language
   definitions that are data where they can be. (Owner's decision,
   2026-10-27.)
9. **Installs as data (§19, #77).** A language's install lives in its own
   `install.json`, replaced whole by a project's; macOS downloads the same
   upstream release as Linux where there is one, Homebrew staying for
   system packages; the default versions' commands are linked into one
   hecatron bin directory on PATH, with no shim. (Owner's decision,
   2026-10-28.)

Decided since, and recorded where the behaviour is described:

- **Commands about hecatron itself go under `hct self`** (2026-10-28; §8).
- **Updates: notify and self update, never auto-update** (2026-10-28;
  [installing hecatron](../guide/installing-hecatron.md#updating-and-removing)).
- **A project pins its hecatron** (2026-10-28;
  [installing hecatron](../guide/installing-hecatron.md#pinning-a-projects-hecatron)).
- **Root is asked for once** (2026-10-28; §19).
- **Xcode projects are read from their files; xcodebuild still builds**
  (2026-10-29; §21).
- **New features are Go only** (2026-10-27; §19, Migration).
- **Install data, macOS downloads and the links directory** (2026-10-28;
  [the install engine](install-engine.md)).
- **Formatting defaults** (owner's brief, 2026-10-27; [hooks](../guide/hooks.md#formatting)).
- **Toolchain directories on PATH only for their own language** (2026-10-26;
  [configuration](../guide/configuration.md#resolving-a-languages-files)).
- **An old checkout gets the toolchains of its day, automatically**, and
  defaults match their release line when running (owner's decisions;
  [toolchain versions](../guide/versions.md)).

## 18. Toolchain versions

Moved to the user guide: [toolchain versions](../guide/versions.md). What's left
of the plan:

### Updating the defaults (planned)

`libexec/maintenance/update-versions` asks each language's release source
(go.dev, python.org, nodejs.org, Adoptium, swift.org, GitHub releases, with
endoflife.date where a language has no API) for the newest release on its
track and rewrites the defaults; `--check` reports what's behind without
writing. A weekly GitHub Action, on GitHub-hosted runners, runs it, runs the
Docker install tests (`tests/docker/`) for each language it changed on Ubuntu
24.04 and 26.04 and Debian 13, and opens a pull request that merges when its
checks pass. Each merged update is released as a new minor version, so a
project gets new default toolchains when it updates hecatron, never by
surprise. (Not built yet: the defaults are updated by hand, and only the
release-date tables are rewritten weekly.)

### Later

`version` may become a list, so a library can test against its oldest and
newest supported versions in one run. The first implementation takes one
version.

### Who builds what

The configuration, version resolution and per-language environment (the
`libexec/lib/` and `libexec/commands/` side) are one piece of work; the
installers, doctor checks, `update-versions` and the weekly workflow are
another.

## 19. A Go core and native Windows

Status: **accepted** 2026-10-27 (#71; tracked in #80). Hecatron has two users, people
and AI agents, and two requirements bash and Python can't meet: fast startup
(one `hct test` starts bash and Python again for every component, and a
process costs far more on Windows) and native Windows, which has no bash.

### The binary

One Go program, `hecatron` (and `hct`), built for macOS, Linux and Windows on
x86_64 and arm64 and published with each release. It owns everything that is
logic: argument parsing and help, the project model and config validation
(§6–§7), version resolution and release tables (§18), the job runner and its
output, generated files (§11), audit and outdated parsing, and running
hooks. Target: `hct --version` and `hct list` under 10 ms. Go rather than
Rust because cross-compiling to all six targets is built in, goroutines fit
the runner, and contributors pick it up quickly; both start fast enough.
`install.sh` and a new `install.ps1` download the binary; nothing else is
needed to run it (no Python, bash or perl).

#### Replacing bin/hecatron a command at a time

The binary takes over from `bin/hecatron` gradually (#73, #76). It finds
`HECATRON_HOME` (the environment, else the checkout or install it sits in:
`bin/hct`'s parent, or the directory it was built in) and the project root
as `bin/hecatron` and `project.find_root` do, sets the same environment, and
then either runs a command itself or hands it to today's implementation
(`run.sh`, `manage.py`, ...) with the same arguments, environment and exit
status, replacing its own process where the platform allows. So a command
is ported without changing what anyone sees, and tests compare the two
entry points' output byte for byte. Decided with the first port
(2026-10-27):

- **Ported first:** `--version`, `help`, the run commands', `doctor`'s and
  `version`'s help, `list`, and `env` without a language. Help that
  `manage.py`'s argparse generates is delegated: its wrapping follows the
  terminal width and its wording the Python version, so it is ported with
  the command, not copied.
- **The Go model may decline, never differ.** Where `project.py`'s answer
  rests on how Python treats an odd value (a `"components"` string iterates
  as its letters; `true == 1`; JSON's `NaN`), the Go model says
  "unsupported" and the command is delegated, `project.py` finding the root
  and reporting any error in its own words. Errors the model reports itself
  (no project, an unknown schema, too old a hecatron) use `project.py`'s
  messages. Parity tests run both on fixture projects and fail where Go
  answers differently or answers where `project.py` fails.
- **Python is checked only when needed:** before delegating, the binary
  checks for Python 3.9 as `bin/hecatron` does, with the same message;
  ported commands run without bash or Python.
- `bin/hecatron` and the installers keep using bash and Python until the
  switch to the binary is decided separately.
- **The run commands** (`test` ... `outdated`) ported next (#74), with the
  runner below; `outdated --json`, and a target run.sh would refuse with
  its own message (an unknown or unadded language), are delegated. The
  hooks stay bash, and the shared hook helpers still call Python, so the run
  commands check for it. A git repository without a config is read
  from `project.py`'s cache of what it detected (`.git/hecatron/detected.json`),
  which `project.py root` refreshes; with no current cache the command is
  delegated.
- **Version resolution** (§18; #75) ported next, in `internal/project`:
  pin and support files in every format, the project's `version`, the
  default and its release line, `oldestInstallable`, the as-of rule with
  its release tables (`releasesOf`, `asOfPrecision`), installed versions
  from each definition's `installed.sh`, the version notes and the
  per-component environment. `hct env LANG [-c COMPONENT]` runs natively,
  and the run commands build each component's environment in Go instead of
  starting `project.py env` per component; where the model declines (a
  `"version"` that isn't a string, Python's other `fromisoformat` forms of
  `HECATRON_AS_OF`, a `"path"` entry relative to the working directory, a
  file with non-ASCII digits), they fall back to it. The model reads files
  with Python's notions of whitespace, line ends and word characters, so a
  pin file is read alike. It runs a language's `installed.sh` once per
  command rather than once per component.

- **`hct status`** (#121; `internal/status`) is native only, a command the
  scripts never had: hecatron's version, the config saved or detected, and
  per language its directories, the version it resolves to and why (§18),
  whether the toolchain's command is on each component's PATH (with its
  version's environment), the last run of each action, and its uncommitted
  changes, then the commands worth running next, marking those that
  install. One `git status` (`--porcelain -z`, mapped to languages and
  components by their directories, deepest first) and no doctor scripts;
  `--output` chosen as the runner chooses it (CI output is human). A last
  run is the action's log for the language: when it was written, passed or
  failed from the summary line both runners end it with (named runs, which
  `history.log` doesn't record, included), and its duration from
  `history.log` when the log is from the last run recorded there.
  `bin/hecatron status` without the binary says so.

- **`hct sync [ref]`** (#92; `internal/changed`'s `Catch`, `cmd/hecatron/sync.go`)
  is native only: after a pull, rebase or checkout, the files changed
  between where `HEAD` was (the reflog, before a rebase's steps) or `ref`
  and `HEAD`, mapped as `--changed` maps them, with three outcomes: changed
  lock files (`cache.lockFiles`) run `hct deps` for those components
  through the runner (`--no-deps` only says so); changed pin or support
  files, or a language's project definition, say the version now needed and
  whether `installed.sh` finds it, never installing (`hct install`,
  "ask the user first"); changed `.hecatron/` files or generated files run
  `hct check` (the binary, as a child process, so a delegated check stays
  delegated). Reads git, never pulls or fetches. `bin/hecatron sync`
  without the binary says so.

- **`hct advisor`** (#105; `internal/advisor`) is native only too: what
  each language's ecosystem considers good practice for the project, each
  finding with a level (`recommended`, what the ecosystem asks for, or
  `suggestion`), a category, why and a link to the ecosystem's own
  documentation. Rules are data where they can be, in the definitions
  (`languages/<lang>/advice.json`, and sets several share in
  `languages/_advice.json`), with the conditions of the definitions' steps
  (tier 1 above) plus `tracked`, committed to git; rules needing logic
  (end of life and lines behind from `releases.json`, an npm lock that no
  longer matches its manifest, CI coverage, audit acceptances past their
  date) are Go code parameterised by the same files. It reads files only
  (one `git ls-files`): no network, no toolchain, no auditor. `--fix` only
  creates a missing pin file at the version the component runs; silencing
  is `"advisor": {"ignore": [...]}`, which the Go `hct check` validates
  (`manage.py`'s check doesn't know the key). Its rules and thresholds are
  in [advisor.md](../guide/advisor.md).
- **`hct licenses`** (#132; `internal/licenses`, `internal/spdx`) is
  native only, like `status`: OSV-Scanner's `scan source --all-packages
  --licenses` per component directory (four at a time), deps.dev's
  licenses for npm, PyPI, Go, crates.io, Maven, NuGet and RubyGems and
  `composer.lock`'s for Packagist; the registries deps.dev lacks are "not
  covered", never guessed. Licenses are parsed as SPDX expressions against
  the embedded SPDX list (deprecated GNU identifiers made `-only` or
  `-or-later`); several declared licenses are read as a choice (`OR`).
  Direct or transitive comes from the lock file or manifest where it says,
  dev from OSV-Scanner's dependency groups. Copyleft classes come from
  Blue Oak Council's copyleft and permissive lists (vendored as
  `internal/spdx/copyleft.txt`; the SPDX list doesn't classify), an
  expression's from its parts (`OR` the least restrictive, `AND` the most).
  The project's own packages, which lock files list too (workspace members,
  editable roots, local replaces), are reported apart and not counted. The
  `"licenses"` policy is checked by the binary's `hct check` only, and is
  by identifier (it doesn't read the classes yet). `--timeout` uses
  `hct audit`'s parsing; with no history, there's no default limit. Tests read OSV-Scanner's
  output on fixture projects (`tests/licenses/fixtures`), captured once in
  Docker (`tests/licenses/capture.sh`) into
  `tests/output/osv-scanner/2.7.0/`. `bin/hecatron licenses` without the
  binary says so.
- **`hct sbom`** (#226; `internal/sbom`) is native only and reuses hct
  licenses' pipeline (`licenses.GatherAll`: the same scan, also listing
  the registries deps.dev has no licenses for) rather than a second
  scanner. CycloneDX 1.6 (default) and SPDX 2.3 JSON, written as maps
  without a library. Purls are built from OSV-Scanner's ecosystem, name and
  version. The lock files hecatron reads say direct or transitive but not
  through which package, so CycloneDX's graph has each component's direct
  dependencies and is declared incomplete; SPDX's `DEPENDS_ON` carries
  `direct`/`transitive` as a comment. Unknown licenses are omitted or
  `NOASSERTION`. The serial number and namespace are a name-based UUID of
  the content, so `SOURCE_DATE_EPOCH` gives a reproducible document. The
  golden SBOMs of the license fixtures are checked against the official
  schemas vendored in `tests/schemas/` (`tests/test_sbom.py`).
- **Secret scanning** (#191; `internal/secrets`) is native only: a
  `secrets` job in `hct audit` beside the languages' (as `shell` is in
  lint), and the target `hct audit secrets`. It wraps gitleaks (pinned,
  its release checked against the SHA-256 in the binary, installed into
  hecatron's bin directory by `hct install secrets` through
  `install.InstallBinary`) rather than a scanner of our own. gitleaks runs
  with `--redact` and a generated config extending the project's
  `.gitleaks.toml` (else its defaults) with an allowlist of what git
  ignores and `audit.secrets.ignore`; only a finding's rule, file, line,
  commit and fingerprint are decoded from its report, so no output can
  hold the secret. Findings go to the audit's SARIF and job summary as
  critical. Without gitleaks the job is left out unless the project sets
  `audit.secrets`, so `bin/hecatron audit`, which has no scan, and the
  binary agree where nothing asks for one.

- **The project commands** ported next (#73): `init`, `add`, `remove`,
  `enable`, `disable`, `check`, `dockerfile` and `workflows` (with
  `--check`), with detection (`detect_languages`, `infer_layout`,
  `detected_config`: `detectSources`, `detectUnless`, `sharedBuildFiles`,
  `dirNames`, `detectScript`, since replaced by `detectKeys`, and later
  `detectWithSources`) in `internal/project` and the generated files
  in `internal/generate`. Each command works out everything it will print
  and write before doing any of it, so a project the model declines is
  handed to `manage.py` untouched; files are written as Python writes them
  (`json.dump(indent=2)`'s ASCII escapes, text read with universal
  newlines). Arguments are read as argparse reads the usual forms; others
  (abbreviated options, `-h`, an option between languages, `--`) are
  delegated. A git repository without a config is found and detected in Go
  too, writing `project.py`'s cache (`.git/hecatron/detected.json`, same key)
  and its first-time message, so no command needs Python to find or read
  it. Tests compare stdout, stderr, status and every file written with
  `bin/hecatron` on fixture repositories (including `tests/hooks/fixtures`
  and `tests/output/fixtures`), and detection on clones of real projects.

- **`doctor`'s driver** ported next (#73, #76): the language list (every
  added, enabled one, or those named), each language's command and lint tool
  on PATH with its version's environment, components whose version isn't
  installed, its `doctor.sh` (still a script, run from the root as before),
  the version notes, the summary and exit status. `--fix` still runs each
  unhealthy language's `install.sh`, from the working directory, then
  forgets what `installed.sh` said and checks again. A name `_check` would
  question (unknown, not added, possibly prompting "Did you mean") and a
  project whose versions the model declines are left to `doctor.sh`. Output
  is `doctor.sh`'s byte for byte, but for bash's warning when a doctor
  script prints a NUL byte (dropped, as bash drops it, without the
  warning). The binary checks the languages at once through the runner,
  with `hct test`'s live display on a terminal (erased before the report,
  which stays the same); each check's stderr is held and shown with its
  language. Repairs stay one at a time: Homebrew and apt allow one install
  at a time, languages share toolchains (Node, the JDK), and an installer
  may prompt. A language checked before an earlier repair is checked again
  before its own, which that repair may have made unnecessary.
- **Root is asked for once (decided 2026-10-28).** `hct install` and
  `doctor --fix` name the languages whose install may need root, say why,
  and run `sudo -v` before any installer, refreshing it every minute until
  they finish (`cmd/hecatron/sudo.go`). An installer script may need root
  where its definition's `installNeedsRoot` says (anywhere, without it, or
  for a project's own); one from data, when its plan has a `sudo` command.
  To need root less, tools hecatron downloads itself go in
  `~/.local/share/hecatron/bin` rather than `/usr/local/bin`, and `hct`
  puts that directory first on PATH, as install.sh's env does, so hooks a
  GUI runs and CI find them too.

- **`banner`** (`hecatron banner --generator PATH ...`, for a project's own
  generators) ported next, with `internal/generate`'s banner. It needs no
  project and no Python. argparse's other forms (a missing `--generator`,
  an abbreviated option, a value starting with `-`), arguments that aren't
  UTF-8, and an environment where Python's stdout may not be UTF-8
  (`PYTHONIOENCODING`, `PYTHONUTF8` other than 1, a locale that is neither
  C, POSIX nor UTF-8: Python fails there on the frame) are left to
  `manage.py`.

- **`version <version>`** ported next (`version.sh`'s orchestration): each
  added language's `version.sh` (the project's, else the built-in one;
  still a script) with the version, from the root, with the project's
  exported variables, then the count; a hook that fails stops it with the
  hook's status, as `set -e` does. Bash's own report of a hook killed by a
  signal (`Terminated` on stderr) isn't repeated; the status is
  the same. `metadata` stays a script: it is interactive and edits the
  metadata file through Python string literals.

- **`install` and `uninstall`** ported next (#73, #76): `manage.py`'s
  orchestration (every added language, or the added ones named; each
  language's `install.sh` or `uninstall.sh`, still scripts, from the root
  with the project's variables and the language's version environment,
  `installed.sh` asked again once an earlier installer ran), the deferred
  dependencies (`HECATRON_DEFER_DEPS`, #107: installing everything fetches
  the dependencies the installers listed with `hct deps` once every
  toolchain is in, this binary's own), the messages and exit statuses (a
  script killed by a signal exits as Python's `sys.exit` makes it). The
  languages' environments are worked out before any script runs, so a
  project the model declines goes to `manage.py`; so do argparse's other
  forms (`help`, an option). Parity tests run both with stub scripts on
  fixture definitions.

- **Audit and outdated parsing** (#75; `internal/deps`) ported next:
  every report `audit.py` and `outdated.py` read (npm, pnpm, Yarn 1 and
  2+, Bun, pip-audit, govulncheck, bundler-audit, Composer, .NET and
  OSV-Scanner; npm, pnpm, Yarn 1, uv, Go, Cargo, Bundler, Composer, .NET,
  Hex, pub, Maven, Gradle and cabal), the threshold (`--fail-on`,
  `audit.failOn`, unrated counting as high), accepted risks and their
  expiry, the end-of-life warning and `--json`'s saved rows. The hooks
  still run the tools and save the raw reports
  (`HECATRON_AUDIT_REPORT_DIR`, `HECATRON_OUTDATED_REPORT_DIR`); the run
  commands give them the binary as `HECATRON_BINARY`, and
  `languages/_audit.sh` and `_outdated.sh` call its `_audit` and
  `_outdated` commands (the Python scripts' arguments) instead of Python
  when it's set. Reports are read with Python's `json` module's rules and
  messages and Python's notion of a value (a null where a string belongs,
  a dict key Python can't hash), so a report gives the same findings or
  the same error. Parity tests run both on every captured report
  (`tests/audit/reports`, `tests/outdated/reports`,
  `tests/output/<tool>/<version>/`, captured by
  `tests/output/capture-deps.sh`), the Python tests' hand-written ones, and
  thousands of variants with one value replaced or removed, comparing
  output and status byte for byte; the one wording left out is which pair
  of values a failed sort names.
- **`hct self update`** and the update notice (`internal/selfupdate`) are
  native only (§13), as is `hct self uninstall`. `bin/hecatron self
  update` (or `uninstall`) without the binary says so.

### Language definitions: data, families, scripts

A definition says what to do in the most declarative of three tiers that can
express it.

1. **Rules in `config.json`** for what is a lookup: which build files mean
   which commands. An action names the build files to look for and where
   (`find`: here, `above-first` or `below-first`, as `find_build` does
   today, or `below-else-here`, for a tool that also works without them:
   Python's pytest; found only in directories below, the action runs in
   each), then its steps, in order:

   ```json
   "lint": {"files": ["mix.exs"], "unknown": "no mix.exs", "steps": [
     {"if": {"not": {"any": [{"file": ".formatter.exs"}, {"contains": ["mix.exs", ":credo"]}]}},
      "skip": "no .formatter.exs and no Credo dependency"},
     {"use": "deps"},
     {"if": {"file": ".formatter.exs"}, "run": ["mix", "format", "--check-formatted"]},
     {"if": {"contains": ["mix.exs", ":credo"]}, "run": ["mix", "credo", "--strict"]}
   ]}
   ```

   A step runs an argv (never a shell string) with optional `env`, removes
   files (`remove` globs, `keep` directories; no `rm`, so it works on
   Windows), stops with a `skip` or `unknown` reason, or uses steps the
   definition shares between actions (`shared`, e.g. fetching
   dependencies). Its `if` tests files: `file`, `missing`, `contains`,
   `glob` (a file named like it here or below), and `all`, `any`, `not`. A
   run step's `args` add arguments where their own `if` holds, once or,
   with `eachProject`, for each project inside this one (`{dir}`).

2. **Families in Go** for build systems several languages share, whose logic
   is more than a lookup: `jvm` (Maven, Gradle, sbt, wrappers, choosing a JDK
   the build can run on), `node` (package manager detection, scripts, `tsc`
   fallback), `dotnet`, `native` (CMake, Meson, autotools, Make targets
   above), `mix`, `cabal-stack`, `swiftpm`, `pub`. A definition names its
   family (`"family": "jvm"`) and its rules extend or override it. Families
   replace `_hooks.sh`'s helpers; they are ordinary Go code with unit tests.

3. **Scripts** for what is neither: a step only one language needs, or an
   installer with custom steps. `install.sh` on macOS and Linux,
   `install.ps1` on Windows; same for doctor checks and hooks.

**Installs** are data too, in the shapes the installers take today:

| Shape | Example | Declared as |
| --- | --- | --- |
| Archive | Go, Node.js, Zig, Temurin, Julia | URL template per OS and CPU, checksum source, directories to put on PATH |
| Vendor installer | rustup, dotnet-install, ghcup, uv | its URL and arguments per OS |
| Package manager | apt, Homebrew, winget, Scoop | the package name per manager |
| Tool | golangci-lint, ruff, ktlint | `go install`, `uv tool`, `npm -g`, `cargo install`, or an archive, at a pinned version |
| Script | the rest | `install.sh` / `install.ps1` |

An archive whose URL or checksum a template can't give (Temurin's names
carry a build number) names a resolver, Go code that asks the publisher's
API. A `platforms` entry replaces the install on a platform its usual
source doesn't cover: Temurin has no JDK 25 for Windows on arm64, so Java
there comes from Microsoft's build. `requires` names languages installed
first (Elixir needs Erlang).

Versions install side by side under hecatron's own directory
(`~/.local/share/hecatron/toolchains/<lang>/<version>`, `%LOCALAPPDATA%` on
Windows) where the shape allows, so installing no longer needs root or
edits to shell startup files: the binary puts the right version on `PATH`
when it runs a hook, and `hct env` prints it for a shell. Package-manager
installs stay system-wide.

**Doctor checks** are a version command and the pattern its output must
match, plus checks that need logic (a linter built with an older compiler)
in the language's family or script.

### Installs as data (#77)

Moved to [the install engine](install-engine.md), with the owner's
decisions of 2026-10-28.

### When a tool's output changes

Hecatron reads tools' output in four places: doctor's version checks, the
installed versions it finds, the error lines in agent output, and the
reports `audit` and `outdated` parse. New releases of a tool change that
output from time to time (npm 7 rewrote `npm audit --json`), so:

- **Machine-readable output first.** Where a tool has `--json` or
  `--format`, hecatron uses it, and checks the schema version where the
  tool states one.
- **Patterns and parsers per version.** A definition's patterns are a list
  tried in order, each optionally limited to tool versions (`"versions":
  ">=7"`); a parser in Go recognises each known layout, as `audit` does for
  npm 6 and 7.
- **Recorded output as tests.** Each parser and pattern is tested against
  output captured from real versions of the tool (`tests/output/<tool>/<version>/`),
  so a new layout is added as a new sample beside the old ones.
- **Failing soft.** Output hecatron can't read never fails a run on its own:
  agent output falls back to the log's last lines, a version it can't read
  is reported as such by doctor, and an unreadable audit report is an error
  that names the tool and version.
- **Noticed weekly.** The weekly coverage run saves each tool's output with
  the newest versions and fails when a parser or pattern no longer matches,
  so a change shows up as a failing check, not as a user's bug report.

### Project hooks

A project's `.hecatron/languages/<lang>/<action>` overrides the default as
today. On Windows the binary runs `<action>.ps1`, `.cmd` or `.exe`; elsewhere
`<action>.sh` or any executable. A project that wants one hook for every
platform declares rules in its config instead of a script.

### Output
The four formats, how one is chosen and the exit codes moved to the user
guide: [output](../guide/output.md).

Decided with the runner (#74, `internal/runner`, `cmd/hecatron/run.go`):

- **Jobs stay one per language**, covering its components (and one per
  project task), as run-parallel.sh's: the scheduling, `history.log`, the
  `.lock` and the log paths are shared with it, so the two can alternate.
  A job's result lists its components, and agent output names the failed
  ones' directories: `FAIL go (age/go/, date/go/) exit 1, 12s, log
  .logs/test/go.log`.
- **Named targets** (`hct test go php`) run one at a time as today. Human
  and CI output stream them; agent and JSON output keep their output in the
  logs and report at the end.
- **Human output** off a terminal is run-parallel.sh's, character for
  character but the times; on one, a row per job (state, time, a bar on the
  run's time line) redrawn in place, without the swim lanes; `-s` is
  accepted and has no effect there yet.
- **CI output** adds `::error file=,line=,col=,title=::` for each error line
  naming a file and line (the file joined to its component's directory,
  relative to `GITHUB_WORKSPACE`), and one annotation without a location for
  a failure with none. `--output ci` selects it too.
- **Error patterns** are a definition's `"errorPatterns"`: a list of
  `{"regex", "tool", "versions", "next"}`, tried in order on each line of
  the log (colour codes removed, only what a carriage return left last);
  the first that matches a line takes it. `regex` is RE2 with optional
  named groups `file`, `line`, `col` and `message`; `next` finds the
  location on one of the next three lines (rustc's `--> src/lib.rs:2:25`)
  and adds it to the line; `versions` (`">=7"`, `">=1.2,<3"`) limits a
  pattern to the versions of `tool` found in the log by the definition's
  `"errorToolVersions"` (`{"pytest": "pytest-(\\S+)"}`; a version not found
  doesn't exclude a pattern). Lines matching no language pattern are tried
  against built-in generic ones (`file:line[:col]: error`, `error:`,
  `FAIL`). An excerpt is at most 10 distinct lines of at most 240
  characters; else the log's last 15 lines, without blank lines, download
  and progress lines or repeated lines. hecatron's own `=== test a (go) ===`
  and `go: test failed: a` lines are left out of both. A pattern that
  doesn't compile is skipped, never failing a run.
- **Recorded samples:** `tests/output/<tool>/<version>/<case>.log` is the
  log hecatron wrote for a failing fixture (`tests/output/fixtures/`,
  captured with real toolchains in Docker by `tests/output/capture.sh`);
  `<case>.excerpt` beside it is the excerpt it must give, and a sample whose
  language's patterns match nothing fails the tests.
- **Exit status 3** when every failed job failed only in components whose
  language command (the definition's `dependency`) isn't on the PATH the
  component's hooks run with; whether a toolchain is broken isn't detected.
  A parallel run still skips a language whose command is missing ("not
  installed") and doesn't count it as a failure. Usage errors (an unknown
  component, a bad `-j`, `-s` or `--output`) exit 2; run-parallel.sh's `-j`
  and `-s` errors exited 1.
- `rebuild` and `retest` keep `-c` for both steps (run.sh drops it).
- **Skipped** (#236, building on #239's nothing-to-audit) is a third
  outcome: a hook says it by writing its reason to the file
  `HECATRON_SKIPPED` names and exiting 0 (`skip`, `skip_component`), not by
  a reserved exit status, which a tool could return by itself and which
  every caller of a hook would have to know. Each runner reads the file
  per component and ends the job's log with a `skipped for N
  component(s)` line, which run-parallel.sh and `hct status` read; a job is
  skipped when every component was, or when the language has no hook for
  the command. `history.log` records it as 77, so it sets no time limit
  and isn't scheduled as a failure. It never fails a run; `--fail-on-skip`
  (a flag and `HECATRON_FAIL_ON_SKIP`, not a config setting: what a run
  must achieve is the caller's) fails one in which nothing passed or
  failed, rather than one with any skip, so a monorepo with one package
  without tests can still turn it on in CI.
- **Reports for CI** (#100, #101; `internal/reports`, §20) are written
  beside these formats, not as one of them: `HECATRON_REPORT_DIR` (JUnit,
  SARIF, Code Quality) and `GITHUB_STEP_SUMMARY`. `--output junit` and
  `--output sarif` are the run commands' only, not `HECATRON_OUTPUT`
  values, since other commands share it. A component's start and time are
  recorded for its suite; build tools' report files count when written
  during its run.

### Migration

**New features are Go only (decided 2026-10-27).** Each release ships the
binary for every platform, so a new command or option (`status`, `skill`,
`--changed`, `licenses`, `advisor`...) is written once, in Go; run through
the scripts without the binary, it says it needs the binary. What the
scripts already do, they keep doing, with parity tests against the Go port.

**Shipping the binary (done).** The release workflow
(`.github/workflows/release.yml`) builds the binary for macOS, Linux and
Windows on x86_64 and arm64 when a release is published and attaches it with
`SHA256SUMS`. `install.sh`, at a release, downloads the one for its machine,
checks its checksum and records the commit it belongs to; `bin/hecatron`
runs it only at that commit, so a clone on a branch, a platform without a
binary or a failed download keeps to the scripts. The binary runs the
commands it hasn't ported through the scripts, as `bin/hecatron` would.

1. This design and a prototype of the schema on Go, Rust, Python, Java and
   Elixir (#72).
2. The Go core (#73–#75) runs today's shell definitions unchanged on macOS
   and Linux (#76), so nothing breaks while definitions are converted.
3. Definitions converted in batches (#77), checked by the Docker install
   tests, the parity harness (installs as data, above) and the fixtures.
   Stage 1, the executor and the languages that only install system
   packages, is done.
4. Windows (#78): a CI job, then installers language by language; the
   coverage matrix gets a Windows column.
5. 1.0 (#79), when the Go core passes everything the bash and Python
   version does on macOS and Linux.

Project hooks written in bash keep working on macOS and Linux throughout.
The bash 3.2 rule (§14) then applies only to hooks and install scripts.

## 20. Source control backends

Status: **planned** (#99–#103), except where marked as working today
(#100–#103 are done).
Hecatron runs the same everywhere; what differs between GitHub, GitLab,
Bitbucket and Azure DevOps is where CI is configured and where results are
shown. Hecatron meets each forge with thin adapters (an environment variable
read, a file written in a format the forge knows) and prefers formats every
forge reads over one integration per forge. It never needs a forge's token:
it doesn't post comments or call forge APIs, so it works the same in a fork,
on a laptop and in CI.

### What works today

Moved to the user guide: [hecatron in CI](../guide/ci.md).

### Planned

| Feature | GitHub | GitLab | Others | Issue |
| --- | --- | --- | --- | --- |
| Test only what a pull or merge request changed (`--changed` with the base from the CI environment) | `GITHUB_BASE_REF` | `CI_MERGE_REQUEST_DIFF_BASE_SHA` | Bitbucket, Azure variables | #99 (needs #91) |
| Per-test results (JUnit XML, `--output junit`) | through a reporting action | merge request test widget | Jenkins, Azure, Buildkite, CircleCI | #100 (done) |
| Findings on the change (SARIF from `audit` and `lint`; GitLab Code Quality JSON) | code scanning | security and code quality widgets | any SARIF reader | #100 (done) |
| A results page per run | job summary (`GITHUB_STEP_SUMMARY`) | the JUnit report | | #101 (done) |

Reports are written with the normal output, not instead of it: a run in CI
prints its log and leaves the forge's files in `HECATRON_REPORT_DIR`.

### Left out

- **Comments on pull requests:** they need a token with write access, and
  annotations, job summaries and the merge request widgets already show
  results where reviewers look.
- **CODEOWNERS:** ownership is a team's structure, which hecatron doesn't
  know.
- **Publishing with a forge's trusted-publishing credentials (OIDC):** part
  of a `hecatron publish` design, not yet decided.

## 21. Reading Xcode projects

`internal/xcode` reads an Xcode project's files without Xcode:
`project.pbxproj` (in the old-style plist format), shared `.xcscheme`
files, a workspace's `contents.xcworkspacedata`, and `.xcconfig` files.
From them, `hct _xcode plan` says how to build the project in a directory:
the container, the scheme, the generic destination, the files the build
needs that aren't there yet (and what makes them), and the projects below
when there are none in it. The Swift and Objective-C hooks call it
(`HECATRON_BINARY`); the bash route asks `xcodebuild -list` and
`-showBuildSettings` instead.

Why: building nine open-source apps (#243) showed that the questions a
build needs answered are in the files, and that answering them through
`xcodebuild` alone fails in ways the files explain:

- **The scheme.** A project has a scheme per Swift package it uses
  (NetNewsWire lists 27) and an app may be named neither like its project
  nor its target (home-assistant's `App-Debug`). The app's scheme is the one
  whose first build entry is an app no other app embeds (not an extension's
  scheme, which builds its host app too, nor a watchOS app's), then the one
  that runs Debug, then the one named after its app. Two brands of one app
  (focus-ios's Focus and Klar) stay the project's choice.
- **The destination.** A target's `SDKROOT` (its own, its `.xcconfig`'s,
  the project's) says the platform: `macosx` builds for the Mac, an iOS app
  for its simulator even where it also builds with Catalyst (a separate
  build its dependencies may not support).
- **What's missing.** A configuration based on an `.xcconfig` that isn't
  there, or a workspace naming a project that isn't, fails deep in
  `xcodebuild`. CocoaPods' (`pod install`) and an app's own setup script
  (wikipedia-ios's) both fail this way; the plan names the files and what
  makes them.
- **Projects below.** GoMap's project is in `src/`, firefox-ios's apps
  beside its Swift package; the plan lists them.
- **Off a Mac.** Detection, `hct status` and generated CI can read an iOS
  project on Linux.

It reads structure, not every build setting: conditional settings
(`SDKROOT[sdk=...]`), Xcode's defaults and the settings a build derives are
left to `xcodebuild`, which stays the authority for a build. When the files
don't say (no shared scheme builds an app; a target whose `SDKROOT` is set
only conditionally), the plan says so (`unsure`) and the hook falls back to
asking `xcodebuild`. Standard library only, like the rest of the Go core.

The fallback follows the same rules where it can without the files' full
reading: for the destination it doesn't trust `-showBuildSettings`'
`SDKROOT`, which without a destination describes the first xcodebuild
would build for (for a Catalyst app, the Mac), and takes macOS only for an
app whose `SUPPORTED_PLATFORMS` is `macosx` alone; for the scheme, with
several building the app, it keeps the one whose first build entry is an
app no other app embeds, and skips naming them when several remain rather
than picking by name. Builds pass `-skipMacroValidation
-skipPackagePluginValidation`: a check runs unattended, and nobody can
enable a package's macro as Xcode asks.

An app's Xcode project beside a `Package.swift` (isowords' `App/`) is the
product: where `xcodebuild` is installed and the project has an
application target, Swift's build and clean are the app's, and the
package (the app's modules) keeps SwiftPM for its tests where it builds on
the host. Without `xcodebuild`, SwiftPM builds what it can. The apps below
a directory are the plan's nested projects (or the same scan in
`libexec/lib/xcode.py`) in directories holding the language's sources,
outside examples, tests and vendored code: a scan of the directories, not
detection's pass over every tracked file, which took seconds on
firefox-ios.
