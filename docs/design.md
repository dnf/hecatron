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
| `generated.ci` | The CI systems `hecatron workflows` generates for: `"github"` (each language's workflow, at `generated.workflows`), `"gitlab"` (`.gitlab-ci.yml`), or both as a list (binary only) | `"github"` |
| `ci.cache` | Cache toolchains and dependencies in the generated CI (§11; binary only) | `false` |
| `ci.image` | The image GitLab jobs run in (binary only) | `ubuntu:24.04` |
| `ci.timeoutMinutes` | Each generated CI job's time limit, unless its template sets one (§11) | `60` |
| `ci.languages` | Per language, its jobs' time limit (`{"pascal": 120}`) | `{}` |
| `generated.dependabot` | `true` or a path: `hecatron workflows` writes Dependabot's configuration (§11; binary only) | none (`.github/dependabot.yml` with `true`) |
| `generated.renovate` | `true` or a path: `hecatron workflows` writes Renovate's configuration (binary only) | none (`renovate.json` with `true`) |
| `dependabot` | Settings for every Dependabot `updates` entry (`schedule`, `groups`, `open-pull-requests-limit`...), and per package ecosystem under `ecosystems` (`{"npm": {...}}`) | `{"schedule": {"interval": "weekly"}}` |
| `renovate` | Settings at the top of `renovate.json` (`extends`, `schedule`, `packageRules`...) | `{}` |
| `lint.format` | Whether `hecatron lint` runs each language's format check: `false`, or per language (`{"haskell": false}`) | `true` |
| `format.exclude` | Paths `hecatron format` and its check leave alone, relative to the root (`"tests/fixtures/"`, `"*.pb.go"`), where hecatron gives the formatter its files | `[]` |
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
`install.json` (§19, installs as data) is replaced whole, like the scripts.

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
| `hecatron outdated [--json] [--fail] [lang...]` | Same for `outdated`: each component's direct dependencies with a newer version, from the ecosystem's own report; report-only unless `--fail` ([outdated.md](outdated.md)) | new |
| `hecatron deps [lang...]` | Same for `deps`: fetch each component's dependencies with its ecosystem's tool, held to its lock file | new |
| `hecatron format [--check] [lang...]` | Same for `format`: rewrite the code with the language's formatter; `--check` changes nothing and fails listing what would change ([format.md](format.md)) | new |
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

**Where the build is.** A language's directory isn't always where its build
file is. When it has none, a default hook looks below it (up to three levels,
skipping vendored code, test data and build output) and runs itself in each
of the shallowest directories that have one, failing if any fails (several
crates under `rust/`, a solution in `csharp/src/`); else it uses the nearest
build file above it, never outside the project (`sdk/go` in `sdk/go.mod`, a
root `Package.swift` or CMake project that builds `swift/` or `src/cpp`).
Where the tool itself finds its build file upward (go, cargo, Bundler and
Rake, a Gradle settings file), above comes first and the tool runs from the
directory, scoped to it as far as the tool allows; elsewhere the hook acts
where the build file is (`swift --package-path`, the CMake project's
`build/`). A CMake or Meson file counts only when it declares a `project()`,
and a root Makefile only through a target named for the directory or language
(`check_cxx` for `cxx/`). Node.js hooks and installers use the package manager
the package declares (`packageManager`, else its lockfile, here or in the
workspace above; pnpm and Yarn through Corepack), installing Corepack or Bun
only when a component needs them.

**Dependencies.** A `deps` hook fetches a component's dependencies with its
ecosystem's tool, held to the lock file when there is one (it fails rather
than change it). Hooks that need dependencies installed (Node.js's
`node_modules`, Bundler's bundle, Composer's `vendor/`, Mix's, pub's,
Shards') install them when they're missing through the same function
(`node_fetch`, `bundle_fetch`... in `languages/_hooks.sh`), so `deps` is
never a required first step; where the build tool fetches by itself (go,
cargo, dotnet, `uv run`, SwiftPM, Maven, Gradle) the hooks leave it to the
tool. Installers install each component's dependencies by running
`hecatron deps`, so a project's own `deps.sh` applies there too.

**Formatting.** A `format` hook rewrites files with the language's
formatter, or with `HECATRON_FORMAT_CHECK=1` only checks. `lint` runs that
check after the lint hook, for each component, unless the project turns it
off (`lint.format`) or the lint hook checked formatting itself (it calls
`checks_format`), so a project overriding `lint.sh` still gets it. A
formatter runs by default only where it ships with the toolchain or nearly
all of the language's code follows it (gofmt, rustfmt, `dotnet format`);
elsewhere only where the project configures one (Prettier, clang-format,
Spotless), since a default would rewrite every file of a project that
doesn't use it. (Owner's brief, 2026-10-27; the per-formatter choices are
the implementation's, in [format.md](format.md).)

Environment for hooks: `HECATRON_ROOT` (project root), `HECATRON_COMPONENT`,
`HECATRON_LANG`, `HECATRON_HOME`, and `_common.sh` helpers available by
sourcing `$HECATRON_HOME/languages/_common.sh`.

## 10. `hecatron init`

`hecatron init [dir]` sets up an existing repository (default: the current
directory). Without it, a git repository is still a project: with no
`.hecatron/config.json` above, its root is the repository's top level and its
configuration what init would write, detected and cached in
`.git/hecatron/detected.json`, keyed on the commit, git's index and
hecatron's version, so a command pays for detection only after the tracked
files change (about 17 µs a file: 4 ms for 100 files, 0.8 s for 47,000), and
nothing appears in the working tree or `git status`. Logs and `local.json`
go in `.git/hecatron/` too. Changing the configuration (`add`, `remove`)
saves it, as init would. Init itself:

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
   JavaScript unless a `tsconfig.json` makes it TypeScript), and a
   `package.json` without the definition's `detectScript` npm script doesn't
   count (JavaScript's `test`: Cucumber's `jsonschema/` is tooling).
   Examples, tests, docs, tools, hooks and vendored code (`examples/`,
   `tests/`, `tools/`, `hooks/`, `fixtures/`, `android/`, `third_party/`,
   `vendor/`, `node_modules/` and the like) are skipped. A language left out
   is listed with the reason.
3. Detects the layout. A language's directory may carry any of its
   definition's `dirNames` (C#'s `dotnet/` or `netstd/`, Ruby's `rb/`). If
   the directories share a pattern with one part varying (`age/impl/go`,
   `date/impl/go`), it proposes `components` and `implPath`; otherwise one
   component, at the pattern under `sdks/`, `sdk/`, `lib/`, `lang/`,
   `bindings/`, `ffi/`, `packages/` or the root if there is one, else the
   pattern covering most languages. A language whose directory doesn't fit
   gets its own entry in `implPaths`. A directory the pattern names fits
   without the language's build file only if it holds the language's code:
   one of its `detect` or `detectSources` files, or, for a definition without
   `detectSources`, any file when it sits at the root or under one of the
   parents above (`lib/`, `sdks/`...: Thrift's `lib/go`, whose `go.mod` is at
   the root). So a build file at the root keeps a language there beside a
   `languages/go/` holding no Go. A directory another language was found in
   doesn't fit (Cucumber's `javascript/` is TypeScript's, so JavaScript found
   in `jsonschema/` gets `implPaths` `jsonschema`). `--components` and
   `--impl-path` set the layout explicitly.
4. Writes `.hecatron/config.json`, adds `.hecatron/local.json` and the log
   directory to `.gitignore`.
5. Writes no hooks: defaults apply until the project adds its own. `--hooks`
   copies the defaults in for editing.
6. With `--justfile`, writes `.hecatron/hecatron.just` with recipes that
   delegate to `hecatron` (`test *args: hecatron test {{args}}`) for a project
   justfile to `import`.
7. Writes a short section for AI coding agents into `AGENTS.md` (the file
   most agents read), between `<!-- hecatron:begin -->` and
   `<!-- hecatron:end -->` so a later init replaces only it, and adds
   `@AGENTS.md` to an existing `CLAUDE.md` (Claude Code reads that instead).
   The section is generic (no languages, which would go stale) and points
   to `hecatron help agents`, which prints `docs/agents.md`, the compact
   reference for the installed version. `--no-agents` skips this. Only an
   explicit init writes it: a detected project writes nothing.
8. Prints what it found and the next step: `hecatron doctor`.

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
  `check_workflow_components.py`. Each job under `jobs:` that has a
  `runs-on` and no `timeout-minutes` gets one, after its `runs-on`: 60
  minutes, or config.json's `ci.timeoutMinutes`, or the language's
  `ci.languages` entry. GitHub's own limit is 6 hours, so a hung job (a
  stalled download, a test waiting on input) would hold every merge that
  long; 60 is above every language's slowest run so far. A job calling a
  reusable workflow (`uses:`, no `runs-on`) can't have one and gets none,
  and the rest of the template stays as written. `hecatron check` reports
  a time limit that isn't a whole number of minutes, and `hecatron workflows`
  refuses to generate until it's fixed.

- **GitLab pipeline** (`generated.ci: "gitlab"`, or `hecatron workflows
  --ci gitlab`): `.gitlab-ci.yml`, written from the project's languages
  rather than a template: one job per language that installs hecatron with
  install.sh (pinned to the generating version, `HECATRON_VERSION`) and runs
  `hct install`, `hct test` and `hct lint` for the language, which cover
  every component. The job's image is `ci.image`, and its `timeout` the
  same limit a GitHub job gets (`60m`, or `ci.timeoutMinutes`, or the
  language's `ci.languages` entry).
- **Caching** (`"ci": {"cache": true}`, off by default so that turning it
  on is what changes existing projects' files): each definition's `cache`
  names the directories its installer puts the toolchain in and its package
  manager downloads to (`~/...`, or relative to each component's
  directory), and its lock files. The toolchain cache is keyed on hecatron's
  version, the project's `config.json`, the language's definition in the
  project, and the definition's pin and support files; the dependency
  cache on the lock files, both matched anywhere in the repository (`**/`),
  so adding a component changes nothing. GitHub workflows get an
  `actions/cache` step per cache after each job's `actions/checkout`, with
  the key's prefix as `restore-keys`; GitLab jobs get `cache:key:files`
  with the same files and a prefix. GitLab caches only paths in the
  project, so each job links every home directory it caches to one under
  `.hecatron-cache/<lang>/` before installing. Toolchains installed by apt
  or into `/opt` and `/usr/local` (most JVM languages', Node.js, .NET and
  Ruby on Linux) aren't cached: restoring them needs root. A toolchain
  hecatron's executor downloads from its `install.json` (§19, installs as
  data) goes to `~/.local/share/hecatron/toolchains/<lang>` instead, which
  needs no root, and that directory is the language's toolchain cache,
  keyed on the language, the platform (`runner.os`, `runner.arch`), a hash
  of the built-in `install.json` and default version (written into the
  key, so a hecatron with different data writes a new one), and the
  project's own `install.json`, config and pin files. A restored toolchain
  passes the executor's checks, so the install does nothing; a partial or
  corrupt one fails them and is installed again. The languages converted
  so far all install system packages, so none is cached this way yet;
  each language moved to a download makes the cache cover more. Since the
  toolchain key names hecatron's version, upgrading hecatron makes `hecatron
  check` ask for the files to be regenerated.

- **Dependency update configuration** (`generated.dependabot`,
  `generated.renovate`), written by `hecatron workflows` alongside the CI:
  `.github/dependabot.yml` with one `updates` entry per package ecosystem
  and component directory, each with the project's `dependabot` settings
  (and its ecosystem's under `dependabot.ecosystems`), and
  `renovate.json` with the project's `renovate` settings, `enabledManagers`
  for its languages and `includePaths` for their directories (none when a
  component sits at the root). Each definition names its Dependabot
  ecosystems (`"dependabot"`, a name or rules choosing by the files in the
  directory: uv where there's a `uv.lock`, else pip; Maven, Gradle or sbt
  by build file) and its Renovate managers (`"renovate"`). GitHub Actions
  is covered at the root when the project has GitHub workflows (bumps to a
  generated workflow then belong in its template, which `hecatron check`
  enforces). Toolchain pins (`rust-toolchain`, `dotnet-sdk`, `.nvmrc`)
  aren't updated by the bots: hecatron resolves toolchain versions itself
  (§18), and `hecatron check` would flag a pin that disagrees. Because the
  file lists every directory, adding a language or component makes `hecatron
  check` fail until it is regenerated.

The GitLab pipeline, caching and dependency update configuration are
written only by the Go binary
(§19); the scripts say so when a project's config asks for them.

Generators for the project's own code (sayable's locale codegen) stay in the
project. They can embed the same banner with `hecatron banner --generator
<path> --source <path> --prefix '# '`, so they don't import hecatron's Python
modules by path.

## 12. Doctor and repair

Unchanged from sayable's design, now shared: `doctor` checks each language's
`dependency` and `lintDependency` commands from `config.json`, then runs the
optional `doctor.sh`, which reports one `problem` per line. A line starting
`note: ` (`note` in `_common.sh`) is shown under the language, like the
version notes, but isn't a problem: a language with only notes is healthy,
and `--fix` repairs only problems. Installers are
idempotent and repair what their doctor detects (a linter built with an older
Go, luacheck on Lua 5.5, a moved virtualenv, an unlinked Homebrew formula).
`doctor --fix` runs the installer of each unhealthy language and checks
again. With the binary, a language installed from its `install.json` (§19,
installs as data) gets its checks from that instead of `doctor.sh`, and
`--fix` runs only the steps whose checks fail. Installers that install project dependencies (`bundle install`, `uv
sync`) do so in every component by running `hecatron deps` (`impl_deps` in
`_common.sh`; §9). `hecatron install` with no languages named runs those
once every toolchain is in, since one language's dependencies can need
another's toolchain (cucumber/messages' `npm ci` runs a generator needing
Ruby); a failure there fails the install and names the language.

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

## 14. Implementation and testing

**Language.** Bash for the entry point, installers, hooks and runner, and
Python 3 (standard library only) for config handling and generators, until
the Go core (§19) replaces them.

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

## 18. Toolchain versions

Status: **accepted**; implemented for Python, Go, Rust, Node.js, Java,
Ruby, PHP, .NET and Elixir. Other installers install one version per language, some
the latest at install time (Swift, Julia, PowerShell, Odin, Crystal).

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
3. **The definition's default `version`**, unless the component says it
   supports only older lines: then the newest release of the newest line it
   supports. A definition lists the files saying so in `supportFiles`, read
   like pin files (an entry may name a pattern, and then the newest version
   any matching file gives counts). This only holds a component back, never
   ahead of the default, and the doctor says when it applies:
   - **Python's trove classifiers** (`Programming Language :: Python ::
     3.14`) in `pyproject.toml`, `setup.cfg` or `setup.py`. The newest
     Python often breaks projects for months while compiled dependencies
     catch up, and a project's classifiers already say how far it has.
     `requires-python` isn't read: it almost never has an upper bound.
   - **The project's CI workflows**: the version inputs of GitHub Actions'
     setup steps (`node-version`, `java-version`, `dotnet-version`,
     `go-version`, `python-version`, `ruby-version`, `php-version`) in
     `.github/workflows/`, inline, as a list, or through a matrix variable,
     the newest across all workflows. What CI runs a language on is what the
     project has tested. A value with no usable version (`lts/*`, `stable`,
     Python's `3.x`) is skipped. For .NET it says what to install, not what
     runs (the entry's `"selects": false`): `setup-dotnet` installs those
     SDKs beside the runner's own, and `dotnet` runs the newest SDK it has
     unless a `global.json` picks one, so a project's CI builds with the
     newest (Gherkin's source generator needs SDK 10's compiler although its
     workflows name 8.0 and 9.0). The SDK is installed, the component runs
     what one on the default would, and the doctor says both.
   - **A Maven build's source level** (`maven.compiler.source`, `<source>`,
     `<target>`, `<release>`): javac dropped level 6 in JDK 12 and 7 in JDK
     20, so a build at 6 or below gets JDK 11 at the newest, and one at 7
     JDK 19.

   A supported line older than the installer can install is raised to the
   definition's `oldestInstallable` (below).
4. **In an old checkout, the release current at its last commit.** When the
   repository's last commit is over a year old, every version above short
   of an exact one (a prefix pin, a supported line, the default) becomes
   the newest release then that it matches; the default becomes the newest
   release then on its track (LTS for an LTS track). Old code was built and
   tested with the toolchains of its day, and today's can reject it: a
   newer javac drops the source level it targets, a newer Node.js breaks
   the `ts-node` its lock file pins (Node.js 16.17 did, so even the newest
   16 is too new for a 2021 checkout). An exact version is the project's
   own choice and stands. `HECATRON_AS_OF=YYYY-MM-DD` sets the date, and
   `HECATRON_AS_OF=off` turns this off. (Owner's decision: automatic.)

Components can resolve to different versions; installers install each one.
When a pin file and the project's `version` both apply and disagree, the pin
file wins and `hecatron check` reports the disagreement: two sources for one
version are almost always a mistake.

### Release dates and what can be installed

The dates come from tables shipped with each definition
(`languages/<lang>/releases.json`: version, date, and whether it was an LTS
release), so resolution stays offline and two machines agree.
`libexec/maintenance/release_dates.py` writes them from each language's
release source: nodejs.org, python.org, go.dev's release history, Rust's
`RELEASES.md`, Adoptium, ruby-lang.org, php.net, Microsoft's .NET release
metadata and Elixir's GitHub releases. A weekly workflow (`.github/workflows/release-dates.yml`) reruns
it and opens a pull request when there are new releases. A language sharing
another's toolchain names it in `releasesOf` (JavaScript's Node.js is
TypeScript's, F#'s and VB.NET's SDK C#'s), and shares its version settings.

A dated version is exact where the installer can install any release (a
Node.js build, a Go toolchain, a rustup toolchain, a Temurin JDK, a
ruby-build Ruby, a .NET SDK) and a line where it can't (`asOfPrecision`
`"line"`: uv has only some patch releases of each Python, and a PHP package
is the newest of its line). A definition's `oldestInstallable` is the oldest
version its installer installs and runs on the supported platforms, checked
in an Ubuntu 24.04 container: Python 3.8 (uv has nothing older), Java 11
(Temurin's Java 8 releases are named differently, and its dates start in
2021), .NET 6.0 (older runtimes need OpenSSL 1.1). Node.js back to 10, Go
toolchains back to 1.18, Rust back to 1.40, Ruby back to 2.7 (ruby-build
builds OpenSSL 1.1 for it) and PHP back to 5.6 install, so those have none.
A dated release older than that is raised to the newest release then of the
oldest line it can install, and the doctor says so; a pin stands as written.

### Reading pin files

Each `pinFiles` entry names a file and the format that reads it
(`{"file": ".nvmrc", "format": "nvmrc"}`); the formats live in
`libexec/lib/` with the rest of resolution, so a definition only chooses
among them. A format turns the file's contents into hecatron's version syntax
(below): `.python-version`'s `3.12` becomes the prefix `3.12`, a
`rust-toolchain.toml` channel of `1.80.1` becomes `=1.80.1`. An entry's `strip`,
a regular expression, is removed from the version first: asdf and mise
write Elixir as `1.17.3-otp-27`, the release and the OTP it was built for,
and an entry stripping `-otp-[0-9]+$` reads 1.17.3. A value with no
version in it (`stable`, `lts/*`, `nightly`) is left to the ecosystem's own
tool: hecatron neither installs nor checks that language's version for that
component, and the doctor says so.

Some files name a floor rather than a version, and a format keeps the part
the ecosystem holds fixed: `global.json`'s `rollForward` decides how far the
.NET SDK may move from `sdk.version`, so `latestFeature` from 8.0.100 reads as
`8.0` and `latestMajor` as `8`, the newest release of which always qualifies;
`disable` is exact, and `patch`, the default, keeps the version as written,
since its scope, a feature band (8.0.1xx), isn't a prefix. In
`composer.json`, `config.platform.php` (the PHP Composer resolves for) is a
pin on its line (`8.4.0` reads as `8.4`); the `require` constraint on `php`
isn't, because it is the range a library supports, which the default
normally satisfies. PHP's doctor checks instead that each component's PHP
satisfies what its lock file requires. (Owner's decision pending; this is
the implementation's.)

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
`version`, still matches as written. (Owner's decision.) The .NET SDK's
`releaseLine` is 3, the whole version: its third part holds both the feature
band and the patch (10.0.403 is band 4, patch 3), so no shorter prefix names
a band, and an older band's compiler can't load a newer band's analyzers.

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

### JDKs a build needs besides its own

A JVM build can need JDKs besides the one its component resolves to, and
neither is a version choice, so neither is a pin file:
`libexec/lib/jvm.py` reads them from the build files. **Toolchains** a
Gradle build (`JavaLanguageVersion.of(17)`, `jvmToolchain(17)`) or Maven's
toolchains plugin (`select-jdk-toolchain`'s `[11,12)`) names are installed
side by side, and Gradle and Maven find them themselves. A **ceiling**
comes from a tool that predates newer JDKs and fails to start on them: a
Gradle wrapper older than 9.1 or a Kotlin compiler older than 2.3 runs on
the newest LTS JDK it supports, which the JVM installers install and the
hooks run the build on (`JAVA_HOME`), saying so. The doctor reports either
when missing.

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
  in [advisor.md](advisor.md).
- **`hct licenses`** (#132; `internal/licenses`, `internal/spdx`) is
  native only, like `status`: OSV-Scanner's `scan source --all-packages
  --licenses` per component directory (four at a time), deps.dev's
  licenses for npm, PyPI, Go, crates.io, Maven, NuGet and RubyGems and
  `composer.lock`'s for Packagist; the registries deps.dev lacks are "not
  covered", never guessed. Licenses are parsed as SPDX expressions against
  the embedded SPDX list (deprecated GNU identifiers made `-only` or
  `-or-later`); several declared licenses are read as a choice (`OR`).
  Direct or transitive comes from the lock file or manifest where it says,
  dev from OSV-Scanner's dependency groups. The `"licenses"` policy is
  checked by the binary's `hct check` only. Tests read OSV-Scanner's
  output on fixture projects (`tests/licenses/fixtures`), captured once in
  Docker (`tests/licenses/capture.sh`) into
  `tests/output/osv-scanner/2.7.0/`. `bin/hecatron licenses` without the
  binary says so.

- **The project commands** ported next (#73): `init`, `add`, `remove`,
  `enable`, `disable`, `check`, `dockerfile` and `workflows` (with
  `--check`), with detection (`detect_languages`, `infer_layout`,
  `detected_config`: `detectSources`, `detectUnless`, `sharedBuildFiles`,
  `dirNames`, `detectScript`) in `internal/project` and the generated files
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
  warning).

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

### Language definitions: data, families, scripts

A definition says what to do in the most declarative of three tiers that can
express it.

1. **Rules in `config.json`** for what is a lookup: which build files mean
   which commands. An action names the build files to look for and where
   (`find`: here, `above-first` or `below-first`, as `find_build` does
   today; found only in directories below, the action runs in each), then
   its steps, in order:

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
   dependencies). Its `if` tests files: `file`, `missing`, `contains`, and
   `all`, `any`, `not`.

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

Status: **stage 1 implemented** (the executor, dispatch and the parity
harness, proven on eleven languages whose installers only install system
packages). The format and its staging come from a survey of all the
installers; the rest of the definitions are converted in later batches.

**Decided by the owner (2026-10-28):**

1. **Install data lives in `languages/<lang>/install.json`**, a file of its
   own beside `config.json`, not in it. A project's
   `.hecatron/languages/<lang>/install.json` replaces the built-in one
   whole, as an `install.sh` override does, rather than merging key by key
   as `config.json` does (§7): a project replacing an install wants to
   replace all of it. Versions stay in `config.json` (`version`, `track`,
   `pinFiles`, `path`, `env`), where resolution (§18) reads them; an
   install never states its toolchain's version. The prototype's
   `"install"` key moved out of Go's, Rust's, Python's and Java's
   `config.json` into their `install.json` (Elixir's stays until its
   installer's rewrite, #164, settles).
2. **macOS uses the upstream release download where there is one**, so
   macOS and Linux install the same pinned version of a language
   downloaded as an archive; Homebrew stays for system packages (the
   languages that come from apt on Linux). This lands with the archive
   batch.
3. **PATH: links to each language's default version in one hecatron bin
   directory**, which the `~/.hecatron/env` the top-level `install.sh`
   already writes puts on PATH; no shim. Hecatron's own commands put each
   component's version on PATH themselves (§18), so the links only serve
   interactive shells. This lands with the archive batch, the first to
   install outside the package manager.

**The format.** `install.json` has a `schema` (1), a `toolchain` (what
provides the language: `"versions"`, `"system"` for the package manager's
version, `"single"` or `"managed"` for versions hecatron installs, and one
install kind: `packages` per package manager, or later `archive`, `vendor`,
`via`), then optional lists, each entry with conditions:

- `packages`: more system packages, with `when` (install only if it holds)
  and `unless` (already satisfied: D's `build-essential` unless there's a
  `cc`);
- `tools`: programs the hooks need (ShellCheck, Bats, zef), each with a
  `name`, its `packages`, a `when` whose `because` is the doctor's reason
  ("bats not installed (a component has .bats tests)") and a `check`;
- `steps`: commands run after the packages (`raco pkg install
  compiler-lib`), each with a `check` and the `problem` the doctor reports
  while it fails;
- `uninstall.keep`: packages uninstall never removes, with `because`
  (COBOL's cmake, which C, C++ and Fortran install with their scripts).

Conditions are the hooks' tier-1 vocabulary (`file`, `missing`, `all`,
`any`, `not`) plus `glob` (a file so named anywhere below a component's
directory), `command` (on the language's PATH), `run` (exits 0) and
`platform`; file tests hold when any component's directory satisfies them.
Anything needing a loop, string parsing or a second command's output is a
hook or family code, not new syntax. Later stages add `archive`, `vendor`
and `via` toolchains, shared tools in `languages/_tools.json`, `links`,
`installed` (in place of `installed.sh`), `requires` and `installOf`,
`hook`, `deps` and `legacy` (old locations to clean up).

**Which runs.** `hct install`, `uninstall` and `doctor` (and `--fix`)
choose per language: the project's own `install.sh` (or `uninstall.sh`,
`doctor.sh`) always wins; `HECATRON_INSTALLER=script` forces the scripts;
otherwise an `install.json` the binary can carry out on this platform
(a known schema, only keys and kinds it runs, packages for this machine's
package manager: apt on Debian and Ubuntu, Homebrew on macOS); otherwise
`install.sh`. A key the binary doesn't know declines the definition rather
than being skipped, so a definition written for a newer binary still
installs, through its script. Without the binary, `bin/hecatron` runs the
scripts, so a converted language keeps its `install.sh`, `uninstall.sh`
and `doctor.sh` until 1.0 (#79).

Other installers still run some languages' `install.sh` directly (Elixir's
and Gleam's run Erlang's). A converted language that something calls
starts its `install.sh` with `install_from_data <lang>` (`_common.sh`): given
the binary (`HECATRON_BINARY`, set for every installer the binary runs) and
not forced to the scripts, it runs `hecatron _install <lang>`, which
installs from the data or exits 125 when it wouldn't (a project's own
`install.sh`, a platform the data doesn't cover), and the script then
carries on itself. `_install` never runs a script, so the two can't loop.
Declaring such dependencies in the data (`requires`) comes later; callers
don't depend on it.

**The executor** (`internal/install`):

- **Plan.** Every action of every language being installed, each with its
  check: a toolchain's or a `packages` entry's packages are installed
  (`dpkg-query`, `brew list`), or its `unless` holds; a tool's or step's
  `check` holds. An action whose `when` fails, or with no package for this
  manager, is skipped. `hct install --plan [lang...]` prints the plan, the
  languages left to their scripts and why, and the commands it would run,
  changing nothing.
- **Apply.** The missing packages of every language go to one package
  manager call (`apt-get install`, after `apt-get update` when the lists
  are over an hour old as `apt_install` does; `brew install`), with `sudo`
  unless running as root; if that fails, each language's alone, so one bad
  package fails only its language. Then each language's steps whose checks
  fail, each checked again after it runs, then every check again: an action
  still failing fails its language and is named. A rerun finds every check
  passing and runs nothing.
- **Doctor.** A language installed from data gets the doctor's checks from
  it, in place of `doctor.sh`: after the `dependency` and `lintDependency`
  checks and the version notes, each tool and step that applies and whose
  check fails (a tool checked as the `dependency` or `lintDependency`
  already is isn't reported twice). `--fix` applies the language's plan,
  which runs only what fails.
- **Receipts.** `~/.local/share/hecatron/receipts/<lang>.json`
  (`$XDG_DATA_HOME/hecatron/receipts` when set) records the packages the
  executor installed, only those absent before, so `python3` or a system
  Bash is never hecatron's to remove, and the steps it ran.
- **Uninstall** reads the receipt: it removes the packages it lists that
  are still installed, except those the definition keeps, another
  language's receipt lists, or another definition names, and says which it
  keeps and why; then it removes the receipt. With no receipt (installed by
  the scripts, or before hecatron) it removes nothing and says so;
  `HECATRON_INSTALLER=script` runs the old `uninstall.sh`.

**Converted (stage 1):** Bash, COBOL, D, Erlang, Common Lisp, Octave,
Pascal, Prolog, Racket, Raku and Tcl. Elixir, the twelfth the survey listed
as packages only, is no longer: its installer now installs builds.hex.pm's
releases side by side (#164).

**Parity.** `tests/docker/parity.sh <image> <lang>...` installs each
language in two fresh containers with the binary built from the checkout,
once forced to the scripts and once from its data; each must pass
`tests/install/check.sh` (install, doctor from a login shell, rerun, smoke),
and both must end with the same installed packages and versions, the same
commands on PATH and the same doctor report. Project files in
`tests/docker/parity/<lang>/` (a `.bats` test, a `META6.json`) exercise
what installs only for some files.

**Open:** pinning the twelve languages that install "latest" today, with
`update-versions` sources; version managers' homes (`~/.rustup`, `~/.ghcup`,
`~/.opam`, or under hecatron's toolchains directory); refusing to remove
packages present before hecatron outright (receipts already never list
them); Perl modules and `npm -g` without sudo; a pure-Go xz reader or the
system `tar`; dnf packages, which nothing tests yet; Windows link
directories (hard links or `.cmd` shims).

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

The runner renders each finished job (label, component, exit code,
duration, log path, error excerpt) in one of four formats:

- **human** (default on a terminal): today's live display and summary;
- **agent**: one summary line, then only the failures, each with its log
  path and up to ten lines matching the language's error patterns (else the
  last fifteen, without colour codes or download noise), and a `Rerun:`
  line; one line when everything passes;
- **json**, for scripts;
- **CI**: human output plus GitHub annotations for each error line.

`--output` chooses, else `HECATRON_OUTPUT`, else agent output when stdout
isn't a terminal and an agent marker is set (`CLAUDECODE=1`, and others
once verified), else CI when `GITHUB_ACTIONS` is set, else human. Not a
config setting: the format belongs to whoever is calling, not the project.
Exit codes: 0 passed, 1 failed, 2 usage, 3 a toolchain missing or broken.

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

- **GitHub Actions workflows.** `hecatron workflows` writes one workflow per
  language from the project's templates (§11), and `hecatron check` fails
  when one is behind.
- **GitHub annotations.** With the Go binary, `--output ci` (chosen
  automatically when `GITHUB_ACTIONS` is set) adds `::error` annotations at
  each error line's file and line, shown on the pull request (§19).
- **GitLab CI and caching (#102).** `hecatron workflows` writes
  `.gitlab-ci.yml` (`generated.ci: "gitlab"` or `--ci gitlab`), and with
  `"ci": {"cache": true}` caches toolchains and dependencies with
  `actions/cache` and `cache:key:files` (§11); `hecatron check` fails when
  the pipeline is behind. Bitbucket and Azure on demand.
- **Dependency update configuration (#103).** `hecatron workflows` writes
  `.github/dependabot.yml` and `renovate.json` (§11), and `hecatron check`
  fails when a language or component was added without regenerating them.
- **Audit in CI.** `hecatron audit` fails at a severity threshold and can
  save each tool's raw report (`HECATRON_AUDIT_REPORT_DIR`);
  [audit.md](audit.md) has a workflow step.
- **Reports every CI reads (#100; `internal/reports`).** With the binary,
  `HECATRON_REPORT_DIR` gets JUnit XML from test, lint and build (a suite
  per language and component; the tools' own test cases merged where they
  write them: Surefire, Failsafe, Gradle and sbt's `TEST-*.xml` found
  under the component, `go test -json` through `hecatron _gotest`,
  pytest's `--junitxml`, and anything a hook leaves in
  `HECATRON_TEST_REPORTS`), SARIF 2.1.0 from lint (located error lines)
  and audit (a rule per advisory, a result per package at its lock file,
  accepted risks suppressed), and GitLab Code Quality JSON from lint;
  `--output junit` and `--output sarif` print one instead of the normal
  output. The generated GitLab pipeline hands JUnit and Code Quality to
  the merge request widgets ([runner.md](runner.md#reports)).
- **Job summary (#101).** With `GITHUB_STEP_SUMMARY` set, every run
  command appends a table of languages and components with status and
  time, then each failure's error lines, log and rerun command; audit,
  licenses and advisor add their findings. The bash runner doesn't.

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
