# .hecatron

Build tooling and language management for multi-language projects. This directory is the infrastructure layer — it manages language toolchains, generates CI workflows and Dockerfiles, runs parallel test/build/lint operations, and keeps project metadata in sync across all implementations.

## Directory structure

```
.hecatron/
  config/                          # Project-level configuration
    languages.json                 # Tools config (non-language deps like cmake for CLI)
    languages/
      added.json                   # Active language set (committed, sorted array)
      disabled.json                # Temporarily disabled languages (gitignored, sorted array)
  languages/                       # Per-language definitions
    _common.sh                     # Shared bash helpers sourced by all installer scripts
    <lang>/
      config.json                  # Language properties (abbreviation, dependency, flags)
      install.sh                   # Install toolchain (macOS + Linux)
      uninstall.sh                 # Remove toolchain
      version.sh                   # Update version in source/manifests
      metadata.sh                  # Update package metadata (optional, only for langs with manifests)
      dockerfile                   # Dockerfile snippet for the generator
      workflow.yml                 # CI workflow template (copied to .github/workflows/ by `just add`)
  tools/
    language/                      # Language management scripts
      _helpers.py                  # Shared Python module (config loading, workflow sync, etc.)
      emit-config.py               # Generates shell variables + functions for eval in helpers.sh
      helpers.sh                   # Shell helpers sourced by all justfile recipes
      add.py                       # Add languages to active set
      remove.py                    # Remove languages from active set
      list.py                      # List languages (active or all)
      install.py                   # Install language toolchains
      uninstall.py                 # Uninstall language toolchains
      disable.py                   # Temporarily disable languages from parallel runs
      enable.py                    # Re-enable disabled languages
      sync_workflows.py            # Re-copy all active workflows with banners
    codegen/
      util.py                      # Banner generator (DO NOT EDIT box-frame)
      dockerfile.py                # Dockerfile generator from per-language snippets
    metadata/
      set.sh                       # Interactive metadata updater for package manifests
    runner/
      run-parallel.sh              # Parallel test/build/lint executor with live progress
      ARCHITECTURE.md              # Internal design docs for the runner
      README.md                    # Runner usage docs
    onboarding/
      get-started.sh               # Interactive onboarding with status + install menu
    bootstrap/
      macos.sh                     # macOS development environment setup
      linux.sh                     # Linux development environment setup
    docker/
      test.sh                      # Docker-based test runner
  docs/
    adding-a-language.md           # Full guide for adding a new language implementation
    installers.md                  # Installer script reference
```

## How it works

### Language discovery

Languages are discovered by scanning `.hecatron/languages/*/config.json`. There is no central registry of languages — the filesystem is the source of truth. Any directory under `languages/` with a `config.json` is a known language.

### Configuration split

Configuration is split across three concerns:

| File | Purpose | Committed |
|---|---|---|
| `languages/<lang>/config.json` | Static language properties | Yes |
| `config/languages/added.json` | Which languages are active | Yes |
| `config/languages/disabled.json` | Temporarily skip in parallel runs | No (gitignored) |
| `config/languages.json` | Non-language tool dependencies | Yes |

**added vs disabled**: `added.json` controls which languages exist in the project — removed languages are refused by all commands. `disabled.json` is a local-only mechanism to temporarily skip languages in parallel runs (e.g., skip Haskell if you don't have `cabal` installed). Disabled languages can still be run individually.

### Shell/Python bridge

The justfile sources `tools/language/helpers.sh`, which runs `tools/language/emit-config.py` via `eval`. The Python script reads all config files and emits shell variables and functions:

- **Lists**: `_ALL_LANGS`, `_ADDED_LANGS`, `_COMPILED_LANGS`, `_CLEAN_LANGS`, `_LINT_LANGS`, `_NO_BUILD_LANGS`, `_DISABLED_LANGS` (newline-separated)
- **Functions**: `_name()` (abbreviation → full name), `_abbr()` (full → abbreviation), `_dep()` (language → build dependency), `_lint_dep()` (language → lint dependency), `_is_disabled()`

This means adding or modifying language properties only requires editing `config.json` files — no shell script changes needed.

### Helpers.sh

`tools/language/helpers.sh` is sourced by every justfile recipe. Beyond the emitted config, it provides:

- `_is_removed()` — check if a language is known but not in `added.json`
- `_ada_env()` — add Alire-managed GNAT to PATH on macOS
- `_lang_grid()` — format languages into a multi-column grid with abbreviations
- `_lang_csv()` — format languages as comma-separated string
- `_suggest()` — fuzzy matching for misspelled language names
- `_check()` — validate + resolve a language name (with fuzzy suggestion on miss)

### Per-language config.json

Each language's `config.json` has these fields:

| Field | Type | Description |
|---|---|---|
| `abbreviation` | `string\|null` | Short alias (e.g., `"ad"` for Ada, `"c++"` for C++) |
| `dependency` | `string` | Build dependency command name (e.g., `"go"`, `"gnat"`, `"cmake"`) |
| `lintDependency` | `string\|null` | Lint-specific dependency if different from build (e.g., `"golangci-lint"` for Go) |
| `compiled` | `boolean` | Whether `just build` applies |
| `hasClean` | `boolean` | Whether `just clean` applies |
| `hasLint` | `boolean` | Whether `just lint` applies |

### Installer scripts

Each language has a set of single-purpose scripts in `languages/<lang>/`:

- **`install.sh`** — Installs the toolchain. Sources `_common.sh`, defines `install_macos()` + `install_linux()` + `deps()`, calls `"install_${OS}"` then `deps`.
- **`uninstall.sh`** — Removes the toolchain. Same pattern with `uninstall_macos()` + `uninstall_linux()`.
- **`version.sh`** — Takes a version string as `$1`, runs sed commands to update version in source files and manifests.
- **`metadata.sh`** — (optional) Updates package metadata using `PT_*` env vars exported by `tools/metadata/set.sh`.

`_common.sh` provides: `$OS` (macos/linux), `$SUDO`, `$ROOT`, `log()`, `has()`, `brew_install()`, `apt_install()`, `_sed_i()`, `_set_metadata_gradle_kts()`, `_set_metadata_gradle_groovy()`.

### Dockerfile generation

`tools/codegen/dockerfile.py` generates the root `Dockerfile` from per-language `languages/<lang>/dockerfile` snippets. Snippets support:

- `# apt: pkg1 pkg2` — collected, deduplicated, sorted into one `apt-get install`
- `# after: otherlang` — topological ordering constraint
- Literal `RUN`/`ENV`/comment lines — emitted verbatim

The generator reads `config/languages/added.json` to determine which languages to include.

### CI workflow management

Each language has a `workflow.yml` template in its language directory. `just add <lang>` copies it to `.github/workflows/<lang>.yml` with a generated DO NOT EDIT banner. `just remove <lang>` deletes the workflow file. `sync_workflows.py` re-copies all active workflows (used after banner format changes).

### Banner system

`tools/codegen/util.py` generates a 9-line double-line box-frame `DO NOT EDIT` banner used on all generated files (Dockerfile, workflows, locale files). The `make_banner()` function accepts 5 body lines and configurable line prefix/suffix for different comment syntaxes. `make_header()` is a convenience for codegen-generated files. `locale_source_comment()` generates the `Source: locales/src/{code}.json` line.

### Parallel runner

`tools/runner/run-parallel.sh` runs justfile recipes in parallel with a live terminal progress display (bar chart + swim lanes). Supports slot-based concurrency (`-j N`, `-j P%`) and unlimited mode. Captures output to `.logs/<command>/<language>.log`.

### Metadata system

`tools/metadata/set.sh` is an interactive script that reads `config/metadata.json`, prompts for changes, exports both old (`PT_OLD_*`) and new (`PT_*`) values as env vars, then calls each language's `metadata.sh` script to perform targeted replacements in package manifests.

### Python helpers (_helpers.py)

The shared Python module provides:

- `project_root()` — compute project root from file location
- `config_path()`, `load_config()`, `save_config()` — central config access
- `load_languages(root)` — discover languages from filesystem
- `added_path()`, `load_added()`, `save_added()` — active language set
- `disabled_path()`, `read_disabled()`, `write_disabled()` — disabled set
- `resolve_name()` — resolve abbreviation or full name to canonical name
- `sync_workflow()` — copy/remove CI workflow with banner
- `workflow_banner()` — generate workflow-specific banner
- `regenerate_dockerfile()` — invoke the Dockerfile generator

## Key design decisions

1. **Filesystem-as-registry**: Languages are discovered from `languages/*/config.json` rather than a central list. Adding a language = adding a directory. This prevents merge conflicts when adding languages concurrently.

2. **Config split**: `added.json` (committed) vs `disabled.json` (gitignored) vs per-language `config.json` (committed). Each has a different change frequency and visibility need.

3. **Shell/Python bridge via eval**: Python reads JSON configs and emits shell code. This avoids JSON parsing in bash while keeping the justfile as the user-facing interface.

4. **Single-purpose scripts**: Each installer action is a separate executable file, not a case-arm in a monolithic script. This makes each file small and independently testable.

5. **Banner on generated files**: Every generated file (Dockerfile, workflows, locale files) carries a prominent DO NOT EDIT banner with the source path and regeneration command. This prevents people from editing generated files by hand.

6. **Abbreviation support**: Languages can have short aliases (e.g., `ts` for TypeScript, `rs` for Rust). The shell `_name()` function resolves abbreviations; `_check()` validates with fuzzy suggestion on typos.

## Shell conventions

- **Brace-delimit all variables**: `${var}` not `$var` (exception: `$1`, `$?`, `$@`, etc.)
- **Brace-delimit before Unicode**: `${_s_pass}✓` not `$_s_pass✓` (Bash 3.2 treats Unicode as identifier chars)
- **ANSI colors**: Base `_c_*` variables define raw escapes; semantic `_s_*` variables give meaning. Always use `_s_*` in output (except structural codes like `_c_reset`).
- **One entry per line**: Language lists, case arms, and grids use one-per-line format to prevent merge conflicts.

## Relationship to project root

The `.hecatron/` directory is infrastructure — it knows how to manage the languages in `src/`, but doesn't contain any library source code. The root `justfile` is the user-facing entry point that delegates to `.hecatron/tools/` scripts. The split:

- `justfile` — user interface (recipes)
- `.hecatron/tools/` — implementation (scripts)
- `.hecatron/languages/` — per-language definitions
- `.hecatron/config/` — project-level config
- `src/` — actual library implementations
- `config/` — project config (version.json, metadata.json, tests.json)
- `codegen/` — locale code generator (reads `locales/*.json`, writes to `src/<lang>/locales/`)
