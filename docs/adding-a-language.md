# Adding a New Language Implementation

This guide covers everything needed to add a new language to parent-time.
The process has four parts: codegen emitter, plural rules, core library, and
build integration.

## Overview

parent-time uses a codegen system where 71 locale JSON files are the single
source of truth. A Python emitter for each language converts these JSON files
into native locale definitions. You write the emitter, the core formatter, and
the tests; codegen handles the locale data.

```
locales/*.json  ──►  codegen/targets/<lang>/emitter.py  ──►  src/<lang>/locales/*
                                                                    (per-file emitters)
locales/*.json  ──►  codegen/targets/<lang>/emitter.py  ──►  src/<lang>/.../Locales.<ext>
                                                                    (single-file emitters)
```

## 1. Create the language directory

Follow the project layout convention:

```
src/<lang>/
  src/ or lib/          # Source files (varies by convention)
    locales/            # Generated locale files go here
    formatter.<ext>     # Core format_age / format_stage logic
    types.<ext>         # AgeLocale, PluralCategory, CompactnessOverride types
  tests/                # Test files
```

Some languages use a flat layout (Go puts everything in the package root).
Match whatever is idiomatic.

## 2. Define types

Implement these types (see `src/typescript/src/locale.ts` or `src/python/src/parent_time/locale.py` for examples):

- **`PluralCategory`** — `"zero" | "one" | "two" | "few" | "many" | "other"`
- **`UnitForms`** — either a single template string or a map of PluralCategory to template string
- **`StageLabel`** — either a string or `{m: string, f: string}` for gendered locales
- **`CompactnessOverride`** — optional overrides for short/compact modes (units, half, delimiter, justBorn)
- **`AgeLocale`** — the full locale record: code, plural function, units, half, delimiter, justBorn, stages, optional short/compact overrides, and optional `formatNumber` / `formatUnit` callbacks

The `plural` field is a function `(int) -> PluralCategory`.

## 3. Write the codegen emitter

Create `codegen/targets/<lang>/emitter.py`. Use an existing emitter as a
reference — `codegen/targets/python/emitter.py` for per-file output,
`codegen/targets/kotlin/emitter.py` for single-file output.

### Required exports

**Per-file emitters** (one file per locale):

```python
def emit_locale(data: dict) -> str:
    """Generate one locale file from canonical JSON data."""

def output_path(code: str, root: str) -> str:
    """Return the output path for a locale file."""

def emit_barrel(all_locales: list[dict]) -> str | None:
    """Generate an index/barrel file, or None if not needed."""

def barrel_path(root: str) -> str | None:
    """Return the barrel file path, or None."""
```

**Single-file emitters** (all locales in one file, like Kotlin/PHP):

```python
def emit_all(all_locales: list[dict]) -> str:
    """Generate a single file containing all locales."""

def output_path(root: str) -> str:
    """Return the output path."""
```

### Key helpers to implement

- `_quote(s)` — string escaping in the target language
- `_unit_forms(unit)` — emit a UnitForms value (simple string or plural map)
- `_stage_label(stage)` — emit a StageLabel value (plain or gendered)

### The `data` dict

Each locale's JSON is passed as a dict with these keys:

| Key          | Type    | Description                                              |
| ------------ | ------- | -------------------------------------------------------- |
| `code`       | `str`   | ISO 639-1 code (e.g. `"en"`)                             |
| `name`       | `str`   | Display name (e.g. `"English"`)                          |
| `pluralRule` | `str`   | One of the 23 rule names (e.g. `"one_other"`)            |
| `units`      | `dict`  | `{day, week, month, year}` — each a string or plural map |
| `half`       | `str`   | Template for half values (e.g. `"{n}½"`)                 |
| `delimiter`  | `str`   | Separator between parts (e.g. `", "`)                    |
| `justBorn`   | `str`   | Label for age 0 (e.g. `"Newborn"`)                       |
| `stages`     | `dict`  | `{newborn, infant, toddler, child, teenager, adult}`     |
| `short`      | `dict?` | Optional short-form overrides                            |
| `compact`    | `dict?` | Optional compact-form overrides                          |

## 4. Add plural rules

Create `codegen/targets/<lang>/plural_rules.py`. There are 23 plural rule
patterns (covering all CLDR cardinal patterns), each defined as a code
string for your target language.

1. Add a `_<LANG>` dict with code strings for all 23 rules
2. Add a getter function: `def get_<lang>_plural(rule: str) -> str`
3. If any rules need named functions (not inline), add a `_<LANG>_NAMED` dict
4. If your language distinguishes simple vs complex plurals, add `is_simple_plural()`

Use the existing entries in any `codegen/targets/*/plural_rules.py` as
templates. See `docs/adding-a-locale.md` for the full rule table.

## 5. Register in codegen/generate.py

Add a generator function and register it in `_GENERATORS`:

```python
def generate_newlang(locales: list[dict], check: bool, diffs: list[str]) -> None:
    from codegen.targets.newlang.emitter import emit_locale, output_path

    for loc in locales:
        path = output_path(loc["code"], ROOT)
        content = emit_locale(loc)
        _write_or_check(path, content, check, diffs)

    # Add barrel file handling here if needed

_GENERATORS = {
    ...
    "newlang": generate_newlang,
}
```

Run `python -m codegen.generate --lang newlang` to generate just your language.

## 6. Implement the core formatter

Port the formatting logic from an existing implementation
(e.g. `src/typescript/src/format-age.ts` or `src/python/src/parent_time/format_age.py`). The algorithm:

1. Parse ISO date strings into year/month/day
2. Compute calendar differences (years, months, days)
3. Apply tier logic: days (0-6d) → weeks+days (7d to <3mo) → months (3-23mo) → years+months (24-35mo) → years with ½ (3-6yr) → integer years (7+yr)
4. Look up the locale's unit forms, apply plural rules, format the string
5. Handle short/compact overrides when requested

Also implement `format_stage` — maps age to developmental stage (newborn,
infant, toddler, child, teenager, adult) using fixed month boundaries.

**Key constraint:** zero runtime dependencies. Date arithmetic must be
hand-implemented.

## 7. Write tests

All tests come from `config/tests.json`. Implement every test case using the
language's standard test framework. The test structure across languages:

- Null/edge cases
- Days tier, weeks+days tier, months tier
- Half-year tier, years+months tier, integer years tier
- Month boundary edge cases, leap year handling
- `format_stage` with gender support
- Multi-locale tests (at minimum: English, Arabic, Chinese, French, German,
  Japanese, Korean, Polish, Russian, Spanish)
- Compactness modes (short, compact)

Use explicit `(dateOfBirth, asOf)` pairs — never use the current date.

## 8. Add to the justfile

Register the new language in all four recipes (`test`, `build`, `lint`,
`clean`). Follow the existing pattern with alias support:

```bash
# In the test recipe:
elif [ "{{lang}}" = "newlang" ] || [ "{{lang}}" = "nl" ]; then
    cd src/newlang && <test-command>
```

Add the language to the bare (no-arg) branch of each recipe so `just test`,
`just build`, `just lint`, and `just clean` include it.

## 9. Add linting

Set up the language's standard linter with strict settings. See
`docs/code-quality.md` for the project's linting philosophy:

- Zero warnings in CI
- Suppress with a documented reason, never a blanket ignore
- Match ecosystem conventions

Add a `just lint newlang` entry and document the configuration in
`docs/code-quality-compiled.md` or `docs/code-quality-dynamic.md`.

## 10. Add a CI workflow

Create the workflow at `.hecatron/languages/<lang>/workflow.yml` — this is the
canonical source. Running `just add <lang>` copies it to
`.github/workflows/<lang>.yml`; `just remove <lang>` deletes it. Use an
existing workflow as a template (e.g., `ruby.yml` for interpreted languages,
`rust.yml` for compiled ones). Include a version matrix when the language
supports it.

```yaml
name: NewLang

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        newlang-version: ["1.0", "1.1", "1.2"]
    defaults:
      run:
        working-directory: src/newlang
    steps:
      - uses: actions/checkout@v4
      - uses: <setup-action>
        with:
          newlang-version: ${{ matrix.newlang-version }}
      - run: <test-command>
```

## 11. Update cross-language files

### `.gitignore`

Add build artifact patterns to `src/.gitignore` (not the root `.gitignore`).
The root `.gitignore` is reserved for cross-language patterns (`__pycache__/`,
`.logs/`, etc.). Language-specific entries go in `src/.gitignore` with a comment:

```gitignore
# NewLang
newlang/build/
```

### Installer script and Dockerfile snippet

Create per-language installer scripts in `.hecatron/languages/<lang>/` — at minimum
`install.sh`, `uninstall.sh`, and `version.sh` (plus `metadata.sh` if the language has
package manifests). Use an existing language as a template (e.g., `.hecatron/languages/go/`).
Each script runs its action on execute (no subcommands) and sources `../_common.sh` for
shared helpers.

Also create a **`.hecatron/languages/<lang>/dockerfile`** snippet for the Dockerfile
generator. The snippet supports three directives:

- `# apt: pkg1 pkg2` — packages to include in the shared `apt-get install` (deduplicated across all languages)
- `# after: otherlang` — ordering constraint (this language's blocks emit after `otherlang`)
- All other lines (`RUN`, `ENV`, comments) are emitted verbatim

Example (apt-only, no RUN blocks):
```
# apt: openjdk-17-jdk-headless
```

Example (with RUN/ENV blocks):
```
# Go
RUN ARCH="$(dpkg --print-architecture)" && \
    curl -fsSL "https://go.dev/dl/go1.21.13.linux-${ARCH}.tar.gz" | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:${PATH}"
```

The Dockerfile is regenerated automatically by `just add` / `just remove`, or
manually with `just dockerfile`. Never edit `Dockerfile` by hand.

### Language-specific README

Every language implementation **must** have a `src/<lang>/README.md`. This is a
project rule — the root `README.md` table must link to it in the Docs column.

Create `src/<lang>/README.md` with installation, usage, and API docs. Include
a `## License` section linking to [`LICENSE.md`](../../LICENSE.md). Use an existing
language's README as a template (e.g., `src/go/README.md`).

### Documentation

Several docs contain per-language tables that must include every implementation.
Add a row for the new language in each. **Keep table rows sorted alphabetically
by language name.** Do not hardcode the number of implementations in prose —
use "all" instead. The count only appears in two places: the `CLAUDE.md` project description
(with the enumerated list) and the `README.md` intro paragraph.

| Document                                    | Table(s) to update                                      |
| ------------------------------------------- | ------------------------------------------------------- |
| [`docs/code-quality.md`](../../docs/code-quality.md)   | Naming conventions, Immutability, Quick reference       |
| [`docs/packaging.md`](../../docs/packaging.md)         | File layout / initialization, Tree-shaking action table |
| [`codegen/README.md`](../../codegen/README.md) | Output per language                                     |
| [`README.md`](../../README.md)                 | Implementations table                                   |
| [`CONTRIBUTING.md`](../../CONTRIBUTING.md)     | Test commands table                                     |
| [`CLAUDE.md`](../../CLAUDE.md)                 | Test command table                                      |

## 12. Verify

```bash
just generate           # regenerate all (includes new language)
just check              # verify generated files match
just test newlang       # run new language's tests
just lint newlang       # run new language's linter
just test               # run all languages to confirm nothing broke
```

## Checklist

- [ ] Language directory with idiomatic structure
- [ ] Type definitions (AgeLocale, PluralCategory, etc.)
- [ ] Codegen emitter in `codegen/targets/<lang>/emitter.py`
- [ ] Plural rules in `codegen/targets/<lang>/plural_rules.py` (all 23 patterns)
- [ ] Generator function registered in `codegen/generate.py`
- [ ] Generated locale files pass the language's linter
- [ ] Core formatter ported from an existing implementation
- [ ] All test cases from `config/tests.json` implemented
- [ ] justfile updated (test, build, lint, clean recipes)
- [ ] Build artifacts added to `src/.gitignore`
- [ ] Linter configured with zero warnings
- [ ] `just test` passes for all languages
- [ ] `just check` reports no drift
- [ ] Installer scripts created at `.hecatron/languages/<lang>/` (install.sh, uninstall.sh, version.sh, metadata.sh)
- [ ] Dockerfile snippet created at `.hecatron/languages/<lang>/dockerfile`
- [ ] CI workflow at `.hecatron/languages/<lang>/workflow.yml` with version matrix
- [ ] Language-specific README with license link added
- [ ] Cross-language tables updated and sorted alphabetically
- [ ] CLAUDE.md updated with new test/build commands
