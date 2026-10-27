# Hecatron design

Status: **accepted** (decisions in §17); phase 2 of §15 (the standalone CLI) is implemented, and §18 (toolchain versions) is accepted but not yet implemented. This repository was split out of
[dnf/sayable](https://github.com/dnf/sayable), where hecatron grew up as its
`.hecatron/` directory. This document is the design of the tool you install
once and use in any repository (`hct` is a short alias for `hecatron`):

```bash
brew install dnf/tap/hecatron  # later, from homebrew-core: brew install hecatron
hecatron init                  # set up the current repository
hecatron doctor --fix          # install and repair the toolchains it needs
hecatron test                  # every language's suites, in parallel
```

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
`build`, `clean`. A project hook script implements it; hecatron runs it once
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
(`languages/<lang>/hooks/`) come in phase 4.

### A project using hecatron

```
.hecatron/
  config.json                # project configuration (§6), committed
  local.json                 # per-developer settings (disabled languages), gitignored
  languages/<lang>/          # project hooks and overrides, committed
    test.sh lint.sh build.sh clean.sh   # override the built-in default hooks
    workflow.yml             # this language's CI workflow template
    version.sh metadata.sh   # optional: update versions and package metadata
    install.sh doctor.sh ... # optional: override a built-in definition file
```

## 6. Project configuration

`.hecatron/config.json` (JSON, read with the Python standard library):

```json
{
  "schema": 1,
  "hecatron": ">=0.1.0",
  "project": "sayable",
  "components": ["age", "date", "duration", "time"],
  "implPath": "{component}/impl/{lang}",
  "languages": ["ada", "bash", "c", "..."],
  "tools": {
    "cli": { "dependency": "cmake" },
    "shell": { "dependency": "shellcheck" }
  },
  "generated": {
    "dockerfile": "age/Dockerfile",
    "workflows": ".github/workflows/{lang}.yml"
  },
  "tasks": {
    "test": { "codegen": "cd age && python3 -m unittest codegen.test_locale_groups" }
  },
  "metadata": "age/config/metadata.json",
  "logs": ".logs"
}
```

| Key | Meaning | Default |
| --- | --- | --- |
| `schema` | Config format version; hecatron refuses formats it doesn't know | required |
| `hecatron` | Minimum hecatron version the project needs | none |
| `project` | Name used in banners, Docker image tags and messages | directory name |
| `components` | Component names, in order | `["."]` (one, unnamed) |
| `implPath` | Where a component's code in a language lives | `{lang}` with one component, else `{component}/{lang}` |
| `implPaths` | A language's own `implPath`, for one kept elsewhere (`{"csharp": "dotnet"}`). A directory filled in from `{lang}` is the language's only when none of its `detectUnless` files sit there; one named outright is taken as written | `{}` |
| `languages` | Languages the project implements (was `added.json`) | `[]` |
| `tools` | Non-language dependencies, as `{ "dependency": "<command>" }` | `{}` |
| `generated.dockerfile` | Where `hecatron dockerfile` writes; omit for none | none |
| `generated.workflows` | Where `hecatron workflows` writes each language's CI workflow | none |
| `tasks` | Extra named jobs per action (`test`, `lint`, ...): shell commands run from the root, in parallel with the languages and by name (`hecatron test codegen`) | `{}` |
| `metadata` | JSON file `hecatron metadata` reads (name, description, author, license, repository) | none |
| `logs` | Log directory for runs | `.logs` |

`.hecatron/local.json` holds per-developer state, currently only
`{"disabled": ["haskell"]}` (was `disabled.json`). `hecatron init` adds it to
`.gitignore`.

## 7. Resolving a language's files

For language `L`, a file `F` (e.g. `install.sh`, `test.sh`) resolves to the
first of:

1. `.hecatron/languages/L/F` in the project,
2. `$HECATRON_HOME/languages/L/F` (hooks under `hooks/`).

`config.json` is merged instead: built-in keys, then the project's keys on top.

A definition's optional `path` lists toolchain directories that aren't on PATH
in every shell (`~/.cargo/bin` from rustup, `~/.local/bin` from uv,
`~/.local/share/alire/toolchains/*/bin` for Ada's GNAT), with `~` expanded and
globs allowed; an entry can also name the installed version's directory,
`{home}/bin` (§18). Existing entries go on PATH only for that language's own
work (its hooks, doctor checks and the re-check after `--fix`, install and
uninstall, and dependency detection), and are expanded each time, so a
directory an installer just created counts. They are never added globally:
Alire's GNAT directory also holds gcc and g++, which would shadow the system
compilers other languages build with. (Agreed with the Linux portability work,
2026-10-26.)
A project can add a language hecatron doesn't know by giving it a complete
directory. `hecatron list languages --all` shows where each file comes from.

## 8. Command line

Every command accepts `help` / `--help`, resolves language abbreviations
(`ts`, `rs`, `c++`) and suggests the nearest name on a typo, as the just
recipes do today. Output of single-language runs streams and is also saved to
`<logs>/<command>/<lang>.log`.

| Command | What it does | From |
| --- | --- | --- |
| `hecatron init [dir]` | Set up a repository (§10) | new |
| `hecatron test [lang...]` | Run `test` for every component of each language; no languages means every language in parallel, plus project checks | `just test` |
| `hecatron lint [lang...\|shell]` | Same for `lint`; languages without a lint hook rely on warnings-as-errors builds; `shell` runs shellcheck on every tracked shell script | `just lint` |
| `hecatron build [lang...]`, `hecatron clean [lang...]` | Same for `build`/`clean` | `just build`, `just clean` |
| `hecatron rebuild`, `hecatron retest` | clean then build/test | same |
| `hecatron install [lang...]`, `uninstall` | Run definition installers; all languages when none are named, continuing past failures | `just install` |
| `hecatron doctor [--fix] [lang...]` | Check toolchains; `--fix` runs installers and re-checks | `just doctor` |
| `hecatron add`, `remove`, `enable`, `disable` | Edit `languages` / `local.json`; add and remove also sync the workflow and Dockerfile | same |
| `hecatron list languages [--all]` | Show languages, status and file origins | `just list` |
| `hecatron dockerfile [--check]` | Generate the Dockerfile from definition snippets | `just dockerfile` |
| `hecatron workflows [--check]` | Copy each language's `workflow.yml` to the workflow path, with a banner | `sync_workflows.py` |
| `hecatron check` | Every `--check` hecatron owns: Dockerfile, workflows, each workflow covers every component | part of `just check` |
| `hecatron metadata` | Interactive package-metadata update through `metadata.sh` hooks | `just metadata` |
| `hecatron version <ver> [component]` | Version bump through `version.sh` hooks | `just version` (age only today) |
| `hecatron get-started` | Onboarding status table and install menu | `just get-started` |
| `hecatron banner ...` | Print a DO NOT EDIT banner for a generator to embed | `codegen/util.py` |
| `hecatron env` | Shell code for a project's own scripts: `eval "$(hecatron env)"` loads the language lists and helpers | `source helpers.sh` |
| `hecatron parallel <name> "job"...` | Run a project's own job list (just recipes by default) with the parallel display and logs | `run-parallel.sh` |
| `hecatron --version` | Hecatron's own version | new |

Flags shared by the run commands: `-j N|P%|unlimited` (concurrency) and
`-s scroll|full|zoom` (display) for parallel runs, as today, and `-c
COMPONENT` (repeatable) to run a language on some components only
(`hecatron test go -c age`).

Which languages an action covers follows from their hooks: a language with a
`build.sh` (project or default) is built, one with `lint.sh` is linted, and
so on, so `config.json` needs no `compiled`/`hasLint` flags. `hecatron
bootstrap` sets up a fresh machine for the project (base packages, then
`hecatron install`).

Exit status: 0 when everything run passed, 1 when anything failed, 2 for a
usage error. A language whose toolchain is missing is reported as "not
installed" and doesn't fail a parallel run, as today.

## 9. Hooks

A hook is a bash script run once per component, from inside
`implPath` for that component and language, with the component name as `$1`.
It builds or installs whatever it needs and exits non-zero on failure. This is
the contract sayable's `test.sh`, `lint.sh`, `build.sh` and `clean.sh` already
follow; `each-library.sh` becomes hecatron's dispatcher.

Rules carried over from sayable:

- Every component runs even after one fails, and the summary names the
  failures.
- `clean` removes build artifacts only, never installed dependencies or
  tracked files.
- A language-specific command acts on every component of that language.

**Default hooks.** Many hooks are the language's standard command (`go test
./...`, `cargo test`, `cargo clippy --all-targets -- -D warnings`, `dotnet
test`). Hecatron ships these as defaults under `languages/<lang>/hooks/`, so a
conventional project needs no hook files at all. Sayable's hooks, kept in
`examples/sayable/`, are the starting point for the defaults; where they name
sayable's files (`test_format_${1}.wren`), the default uses a convention and
the project overrides.

Environment for hooks: `HECATRON_ROOT` (project root), `HECATRON_COMPONENT`,
`HECATRON_LANG`, `HECATRON_HOME`, and `_common.sh` helpers available by
sourcing `$HECATRON_HOME/languages/_common.sh`.

## 10. `hecatron init`

`hecatron init [dir]` sets up an existing repository (default: the current
directory):

1. Refuses if `dir` isn't inside a git repository, and is a no-op on an
   already initialised project unless `--force`.
2. Detects languages from their build files: `go.mod` → go, `Cargo.toml` →
   rust, `tsconfig.json` → typescript, `pyproject.toml` → python, `*.cabal` →
   haskell, and so on (a definition's `detect` globs). A language with no
   build file is detected from its sources (`detectSources`, such as `*.c`),
   keeping only the directories outside every other language's: C files in
   `python/ext/` are Python's extension, and C found only inside `cpp/` isn't
   added. A build file several languages share (`sharedBuildFiles`, such as
   C's and C++'s `CMakeLists.txt`) counts for the one with the most sources
   beside it or under its `src/` and `include/`. A hit is dropped where one
   of the definition's `detectUnless` files sits beside it (`package.json` is
   JavaScript unless a `tsconfig.json` makes it TypeScript). Examples, tests,
   docs, tools and vendored code (`examples/`, `tests/`, `tools/`,
   `fixtures/`, `android/`, `third_party/`, `vendor/`, `node_modules/` and
   the like) are skipped. A language left out is listed with the reason.
3. Detects the layout. A language's directory may carry any of its
   definition's `dirNames` (C#'s `dotnet/` or `netstd/`, Ruby's `rb/`). If the
   directories share a pattern with one part varying (`age/impl/go`,
   `date/impl/go`), it proposes `components` and `implPath`; otherwise one
   component, at the pattern under `sdks/`, `sdk/`, `lib/`, `lang/`,
   `bindings/`, `ffi/`, `packages/` or the root if there is one, else the
   pattern covering most languages. A language whose directory doesn't fit
   gets its own entry in `implPaths`; a directory another language was found
   in doesn't fit (Cucumber's `javascript/` is TypeScript's, so JavaScript
   found in `jsonschema/` gets `implPaths` `jsonschema`). `--components` and
   `--impl-path` set the layout explicitly.
4. Writes `.hecatron/config.json`, adds `.hecatron/local.json` and the log
   directory to `.gitignore`.
5. Writes no hooks: defaults apply until the project adds its own. `--hooks`
   copies the defaults in for editing.
6. With `--justfile`, writes `.hecatron/hecatron.just` with recipes that
   delegate to `hecatron` (`test *args: hecatron test {{args}}`) for a project
   justfile to `import`.
7. Prints what it found and the next step: `hecatron doctor`.

It never installs anything or touches CI by itself; `hecatron workflows` and
`hecatron dockerfile` are separate, explicit steps.

## 11. Generated files

Hecatron generates two kinds of project files, each with the 9-line
DO NOT EDIT banner naming the generator and its source:

- **Dockerfile** (`generated.dockerfile`): collected from each language's
  `dockerfile` snippet (`# apt:` lines deduplicated into one `apt-get
  install`, `# after:` ordering, literal `RUN`/`ENV` blocks, and a block
  repeated per toolchain version; see §18).
- **CI workflows** (`generated.workflows`): each language's project
  `workflow.yml`, with a banner. `hecatron check` verifies that each workflow
  mentions every component, generalising sayable's
  `check_workflow_components.py`.

Generators for the project's own code (sayable's locale codegen) stay in the
project. They can embed the same banner with `hecatron banner --generator
<path> --source <path> --prefix '# '`, so they don't import hecatron's Python
modules by path.

## 12. Doctor and repair

Unchanged from sayable's design, now shared: `doctor` checks each language's
`dependency` and `lintDependency` commands from `config.json`, then runs the
optional `doctor.sh`, which reports one `problem` per line. Installers are
idempotent and repair what their doctor detects (a linter built with an older
Go, luacheck on Lua 5.5, a moved virtualenv, an unlinked Homebrew formula).
`doctor --fix` runs the installer of each unhealthy language and checks
again. Installers that install project dependencies (`bundle install`, `uv
sync`) do so in every component, through `each_impl`, which now reads
`components` and `implPath` from the project config.

## 13. Distribution

**Homebrew.** `brew install hecatron` needs the formula in homebrew-core,
which accepts projects with some public adoption. Until then: a tap,
`dnf/homebrew-tap`, with `Formula/hecatron.rb`, installed as `brew install
dnf/tap/hecatron`. The formula:

- installs the release tarball into `libexec/` and a `bin/hecatron` wrapper
  that sets `HECATRON_HOME`;
- depends on `bash` (the runner needs bash 4+; macOS ships 3.2) and `python@3`
  (macOS has no Python without the Command Line Tools); uses the system perl;
- runs a `test do` block that inits a temporary repo and runs `hecatron
  doctor` on an empty language list.

**Linux and CI.** Homebrew on Linux works with the same formula. For CI, a
composite GitHub Action (`uses: dnf/hecatron@v1`) downloads a release and puts
it on `PATH`; there is also an `install.sh` for curl-based installs.

**Releases.** Semantic versions, tags `vX.Y.Z`, a GitHub release with the
tarball; a release workflow updates the tap formula's URL and checksum. A
project's `"hecatron": ">=x.y.z"` makes older installs fail with a clear
message instead of misbehaving.

## 14. Implementation and testing

**Language.** Keep bash for the entry point, installers, hooks and runner,
and Python 3 (standard library only) for config handling and generators, as
today: it reuses the existing, working code. A single static binary (Go) would
drop the Python and bash 4 dependencies, but means rewriting the runner and
config bridge; worth revisiting after 1.0.

**Testing hecatron.**

- shellcheck on every script; Python `unittest` for config, resolution and
  generators.
- Fixture projects under `tests/fixtures/` (one component, several
  components, a language override) driven by command-level tests.
- A downstream job that checks out sayable and runs `hecatron check` and
  `hecatron test go python` against it, so a hecatron change that breaks the
  project it came from fails hecatron's CI.

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

Installers run with the user's privileges and some pipe a downloaded script
into bash (Homebrew, Pharo's zeroconf, rustup). Hecatron only runs an
installer when asked (`install`, `doctor --fix`) and names each one before it
runs. Hooks are project code: running `hecatron test` in a repository runs
that repository's scripts, exactly like running its justfile or Makefile, so
the same trust applies. Hecatron never runs hooks on its own (no shell
integration, no git hooks).

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
   hecatron's default. `=` marks an exact version. Defaults are pinned, follow
   the language's LTS line where it has one and its newest stable release
   otherwise, and a weekly job updates them. (Owner's decision.)

## 18. Toolchain versions

Status: **accepted**, not yet implemented. Until it is, installers install
one version per language, some pinned (JDK 17, Node 22, .NET 10) and some
the latest at install time (Go, Swift, Julia, PowerShell, Odin, Crystal).

A project can need an older or newer version of a language than the
newest one, and two projects on the same machine can need different ones. So
a language's version is data: hecatron has a default, and a project can
choose another.

### Choosing a version

For a language in a component, the version is the first of:

1. **The ecosystem's own pin file**, nearest first from the language's
   implementation directory (`implPath`) up to the project root, as the
   ecosystems' own tools search, read with that ecosystem's meaning:
   `.python-version`, `.nvmrc` or `.node-version`, `rust-toolchain.toml` or
   `rust-toolchain`, `global.json` (`sdk.version`), the `toolchain` line of
   `go.mod`, `.ruby-version`, Gradle's `gradle-daemon-jvm.properties`
   (`toolchainVersion`, the JDK)... Each definition lists the files it reads
   in `config.json` (`pinFiles`). Using these means a project declares a
   version once, where its other tools already look.
2. **`version` in the project's `.hecatron/languages/<lang>/config.json`.**
3. **The definition's default `version`.**

Components can resolve to different versions; installers install each one.
When a pin file and the project's `version` both apply and disagree, the pin
file wins and `hecatron check` reports the disagreement: two sources for one
version are almost always a mistake.

### Reading pin files

Each `pinFiles` entry names a file and the format that reads it
(`{"file": ".nvmrc", "format": "nvmrc"}`); the formats live in
`libexec/lib/` with the rest of resolution, so a definition only chooses
among them. A format turns the file's contents into hecatron's version syntax
(below): `.python-version`'s `3.12` becomes the prefix `3.12`, a
`rust-toolchain.toml` channel of `1.80.1` becomes `=1.80.1`. A value with no
version in it (`stable`, `lts/*`, `nightly`) is left to the ecosystem's own
tool: hecatron neither installs nor checks that language's version for that
component, and the doctor says so.

### Version syntax

In hecatron's configuration:

| Written | Means |
| --- | --- |
| `"3.12"` | The newest release whose version begins with the components 3.12: 3.12, 3.12.7... but not 3.120 |
| `"=4.5"` | Exactly the release named 4.5 and never anything else, even if the language later publishes 4.5.1 |

A prefix follows new patch releases, and a project that doesn't want that
writes `=`. Without the marker, a project pinning `"4.5"` would silently move
to 4.5.x once upstream added a level of versioning, which is outside the
project's control. Hecatron's own defaults are always exact.

A prefix is resolved in two places: installing resolves it to the newest
matching release upstream, and running (a hook's environment, the doctor)
resolves it to the newest matching version installed on the machine, so
running never needs the network.

A default is exact for installing (an installer, the Dockerfile) but not for
running: a component on the default takes the newest installed version on
the default's release line, so a machine that already has Node.js 24.19
doesn't need a second 24 for a default of 24.22.1. A definition's
`releaseLine` names how many leading parts make a line (1 for Node.js and
Java, whose 24 and 25 are lines); without it, a line is every part but the
last (Go 1.27.3 is line 1.27). A version the project chooses, by pin file or
`version`, still matches as written. (Owner's decision.)

### Defaults and how they are updated

Each definition's `config.json` records its default `version` (exact) and its
`track`: `"lts"` where the language has long-term support releases (Java,
Node.js, .NET) and `"stable"` (the newest release) otherwise. Installers
never resolve "latest" at install time, so two machines installing on
different days get the same toolchain and every change is a reviewable diff.

`libexec/maintenance/update-versions` asks each language's release source
(go.dev, python.org, nodejs.org, Adoptium, swift.org, GitHub releases, with
endoflife.date where a language has no API) for the newest release on its
track and rewrites the defaults; `--check` reports what's behind without
writing. A weekly GitHub Action, on GitHub-hosted runners, runs it, runs the
Docker install tests (`tests/docker/`) for each language it changed on Ubuntu
24.04 and 26.04 and Debian 13, and opens a pull request that merges when its
checks pass. Each merged update is released as a new minor version, so a
project gets new default toolchains when it updates hecatron, never by
surprise.

### Installing and using a version

- **Side by side.** Installers add the resolved version next to any others,
  through the ecosystem's version manager where it has one (uv, rustup,
  ghcup, .NET's side-by-side SDKs, `openjdk-N` or Temurin, Go's toolchain
  downloads), and never replace a version another project may use.
- **What's installed.** A definition's optional `installed.sh` prints one
  `version<TAB>home` line per installed version, offline (each ecosystem
  already keeps a list: `rustup toolchain list`, `dotnet --list-sdks`, uv's,
  ghcup's, the JDKs under `/usr/lib/jvm`). The core runs it with the
  language's static `path` entries on PATH and stdin closed, and treats a
  failure, or a run over 30 seconds, as nothing installed. A component's
  installed version is the newest line its spec matches.
- **Per-language environment.** The `path` mechanism (§7) becomes a
  per-language environment. `path` entries and the new `env` variables may
  name `{home}`, `{version}`, `{major}` and `{minor}` of the installed
  version (`"path": ["{home}/bin"]`, `"env": {"JAVA_HOME": "{home}"}`), and
  are applied wherever `path` was. It is per component, since components can
  resolve to different versions: `each-component.sh` applies the
  component's environment before running its hook, and
  `hecatron env LANG [-c COMPONENT]` prints it as shell exports for a
  project's own recipes. With no installed version matching (or no
  `installed.sh`), entries and variables naming one are left out, static
  entries still apply, and the doctor reports it.
- **Queries.** `project.py version LANG [COMPONENT]` prints the spec, where it
  came from and the installed match; `project.py versions LANG` prints each
  distinct spec the components resolve to with its sources (`-` for a pin
  file naming no version), so an installer installs each one.
- **Doctor.** `doctor` checks that the resolved version is installed and is
  the one a language's commands run; `--fix` installs it. It reports each
  component whose version has no installed match ("b: java 21 not installed
  (from pin:b/java/.java-version)"), for languages with an `installed.sh`. A version past its
  upstream end of life is a warning, not a failure.
- **Generated files.** The Dockerfile installs every version any component
  resolves to. A snippet writes a block between `# each-version` and
  `# end-each-version` lines, which `hecatron dockerfile` repeats for the
  definition's default and then each other spec in component order, filling
  in `{version}` (the spec without `=`), `{exact}` (`1` for an exact spec,
  else empty), `{major}` and `{minor}`. `# each-version java` repeats over
  another language's versions (Kotlin's snippet installs Java's JDKs).
  Generation stays offline: a block given a prefix resolves it at build time,
  installs where `installed.sh` looks, and skips a version already there.
  `ENV` lines stay outside the block and name the default; a component's own
  version comes from `hecatron env` at run time. A workflow template names a language's resolved version with
  a placeholder (`{{version:java}}`, or `{{version:java:age}}` for one
  component), which `hecatron workflows` fills in, and `hecatron check`
  fails when a generated file is behind its versions.

### Later

`version` may become a list, so a library can test against its oldest and
newest supported versions in one run. The first implementation takes one
version.

### Who builds what

The configuration, version resolution and per-language environment (the
`libexec/lib/` and `libexec/commands/` side) are one piece of work; the
installers, doctor checks, `update-versions` and the weekly workflow are
another.
