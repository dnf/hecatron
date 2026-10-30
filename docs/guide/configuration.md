# Project configuration

A project is configured by its `.hecatron/` directory, committed with the
code. In a git repository without one, hecatron detects the configuration
and keeps it inside `.git/` ([init and detection](init.md)); `hct init`
saves it as `.hecatron/config.json` to commit and adjust.

## The `.hecatron/` directory

```
.hecatron/
  config.json                # project configuration (below), committed
  local.json                 # per-developer settings (disabled languages), gitignored
  languages/<lang>/          # project hooks and overrides, committed
    test.sh lint.sh build.sh clean.sh   # override the built-in default hooks
    workflow.yml             # this language's CI workflow template
    version.sh metadata.sh   # optional: update versions and package metadata
    install.sh doctor.sh ... # optional: override a built-in definition file
```

## config.json

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
| `hecatron` | The hecatron the project uses: `"0.7"` (any 0.7.x) or `"0.7.0"` pins it (binary only; [pinning](installing-hecatron.md#pinning-a-projects-hecatron)), `">=0.7.0"` sets a minimum | none |
| `project` | Name used in banners, Docker image tags and messages | directory name |
| `components` | Component names, in order; or their directories, with `implPath` `{component}` (`[".", "layers/a/nodejs"]`: [detection](init.md) writes these for a root package beside packages below it). A directory several components share, for a language whose `implPath` doesn't name `{component}`, runs once, for the first | `["."]` (one, unnamed) |
| `implPath` | Where a component's code in a language lives | `{lang}` with one component, else `{component}/{lang}` |
| `implPaths` | A language's own `implPath`, for one kept elsewhere (`{"csharp": "dotnet"}`). A directory filled in from `{lang}` is the language's only when none of its `detectUnless` files sit there (or another language's `detectYields` gives it back) and its own `detectYields` doesn't give it away; one named outright is taken as written | `{}` |
| `languages` | Languages the project implements (was `added.json`) | `[]` |
| `tools` | Non-language dependencies, as `{ "dependency": "<command>" }` | `{}` |
| `generated.dockerfile` | Where `hct dockerfile` writes; omit for none | none |
| `generated.workflows` | Where `hct workflows` writes each language's CI workflow | none |
| `generated.ci` | The CI systems `hct workflows` generates for: `"github"` (each language's workflow, at `generated.workflows`), `"gitlab"` (`.gitlab-ci.yml`), or both as a list (binary only) | `"github"` |
| `ci.cache` | Cache toolchains and dependencies in the generated CI ([generated files](generated-files.md); binary only) | `false` |
| `ci.image` | The image GitLab jobs run in (binary only) | `ubuntu:24.04` |
| `ci.secrets` | Generate a job running `hct audit secrets`: a GitHub workflow named `secrets` at `generated.workflows`, a `secrets` job in `.gitlab-ci.yml` ([audit.md](audit.md#secrets); binary only) | `false` |
| `ci.timeoutMinutes` | Each generated CI job's time limit, unless its template sets one ([generated files](generated-files.md)) | `60` |
| `ci.languages` | Per language, its jobs' time limit (`{"pascal": 120}`) | `{}` |
| `generated.dependabot` | `true` or a path: `hct workflows` writes Dependabot's configuration ([generated files](generated-files.md); binary only) | none (`.github/dependabot.yml` with `true`) |
| `generated.renovate` | `true` or a path: `hct workflows` writes Renovate's configuration (binary only) | none (`renovate.json` with `true`) |
| `dependabot` | Settings for every Dependabot `updates` entry (`schedule`, `groups`, `open-pull-requests-limit`...), and per package ecosystem under `ecosystems` (`{"npm": {...}}`) | `{"schedule": {"interval": "weekly"}}` |
| `renovate` | Settings at the top of `renovate.json` (`extends`, `schedule`, `packageRules`...) | `{}` |
| `lint.format` | Whether `hct lint` runs each language's format check: `false`, or per language (`{"haskell": false}`) | `true` |
| `format.exclude` | Paths `hct format` and its check leave alone, relative to the root (`"tests/fixtures/"`, `"*.pb.go"`), where hecatron gives the formatter its files | `[]` |
| `tasks` | Extra named jobs per action (`test`, `lint`, ...): shell commands run from the root, in parallel with the languages and by name (`hct test codegen`) | `{}` |
| `metadata` | JSON file `hct metadata` reads (name, description, author, license, repository) | none |
| `logs` | Log directory for runs | `.logs` |

`.hecatron/local.json` holds per-developer state, currently only
`{"disabled": ["haskell"]}` (was `disabled.json`). `hct init` adds it to
`.gitignore`.

How a language's version is chosen (pin files, `version`, the defaults) is
in [toolchain versions](versions.md); hooks are in [hooks](hooks.md).

## Resolving a language's files

For language `L`, a file `F` (e.g. `install.sh`, `test.sh`) resolves to the
first of:

1. `.hecatron/languages/L/F` in the project,
2. `$HECATRON_HOME/languages/L/F` (hooks under `hooks/`).

`config.json` is merged instead: built-in keys, then the project's keys on top.
`install.json` ([installs as data](../contributing/install-engine.md)) is replaced whole, like the scripts.

A definition's optional `path` lists toolchain directories that aren't on PATH
in every shell (`~/.cargo/bin` from rustup, `~/.local/bin` from uv,
`~/.local/share/alire/toolchains/*/bin` for Ada's GNAT), with `~` expanded and
globs allowed; an entry can also name the installed version's directory,
`{home}/bin` ([versions](versions.md#installing-and-using-a-version)). Existing entries go on PATH only for that language's own
work (its hooks, doctor checks and the re-check after `--fix`, install and
uninstall, and dependency detection), and are expanded each time, so a
directory an installer just created counts. They are never added globally:
Alire's GNAT directory also holds gcc and g++, which would shadow the system
compilers other languages build with. (Agreed with the Linux portability work,
2026-10-26.)
A project can add a language hecatron doesn't know by giving it a complete
directory. `hct list languages --all` shows where each file comes from.
