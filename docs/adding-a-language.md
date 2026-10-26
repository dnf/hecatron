# Adding a New Language Implementation

This guide covers everything needed to add a new programming language to
Sayable. A language implements **all four libraries** (age, date, duration and
time) and a `sayable` CLI; the libraries are peers built the same way, so most
steps repeat once per library. The root [`CLAUDE.md`](../../CLAUDE.md) has the
short version of this list.

## Overview

The 168 locales live in `locales/<code>.json` at the repository root, one file
per language with a section per library. Each library has its own codegen, and
a Python emitter per target language turns that library's section into native
locale definitions. You write the emitters, the types, the formatters, the
tests and the CLI; codegen handles the locale data.

```
                        ┌──► age/codegen/targets/<lang>/emitter.py      ──► age/impl/<lang>/.../locales/
locales/<code>.json ────┼──► date/codegen/targets/<lang>/emitter.py     ──► date/impl/<lang>/.../locales/
 (code, name,           ├──► duration/codegen/targets/<lang>/emitter.py ──► duration/impl/<lang>/.../locales/
  pluralRule +          └──► time/codegen/targets/<lang>/emitter.py     ──► time/impl/<lang>/.../locales/
  one section per library)
```

`load_component_locales("<library>")` in `.hecatron/tools/codegen/util.py`
hands each library's generator the shared `code`, `name` and `pluralRule` plus
that library's section, as one flat dict per locale. Field-by-field
documentation of every section is in
[`docs/adding-a-locale.md`](../../docs/adding-a-locale.md).

Use an existing language as the reference throughout. Python
(`<library>/impl/python/`) and TypeScript (`<library>/impl/typescript/`) are
the clearest; Python is also the reference the fuzzer compares against.

## 1. Create the implementation directories

For each library, create `<library>/impl/<lang>/` with the language's
idiomatic layout:

```
<library>/impl/<lang>/
  src/ or lib/            # Source files (varies by convention)
    locales/              # Generated locale files go here (DO NOT EDIT)
    <formatter>.<ext>     # Core formatting logic
    <types>.<ext>         # Locale and option types
  tests/                  # Test files
  README.md               # API docs + ## License section
```

Generated locale files must be in their own `locales/` subdirectory (or the
language's idiomatic equivalent), separate from hand-written source. Some
languages use a flat layout (Go puts the package in the root and the locales in
`locales/`). Match whatever is idiomatic, and use the same layout in all four
libraries.

## 2. Define the types

Port the types from an existing implementation, for example
`<library>/impl/python/src/sayable_<library>/locale.py` or
`<library>/impl/typescript/src/locale.ts`.

**age** (`AgeLocale`):

- `PluralCategory` — `"zero" | "one" | "two" | "few" | "many" | "other"`
- `UnitForms` — a template string, or a map of plural category to template (`other` required)
- `StageLabel` — a string, or `{m, f}` for gendered locales
- `CompactnessOverride` — optional `units`, `half`, `delimiter`, `justBorn`
- `AgeLocale` — code, plural function `(int) -> PluralCategory`, units (day, week, month, year), half, delimiter, justBorn, stages, optional short/compact overrides, and optional `formatNumber` / `formatUnit` callbacks

**date** (`DateLocale`):

- `CompactnessOverride` — optional months, weekdays, dayFormat, datePattern, datePatternWithYear (and the other fields the reference types allow)
- `DateLocale` — code, months, weekdays (Sunday first), dayFormat, dayFormatExceptions, datePattern, datePatternWithYear, today, tomorrow, yesterday, optional dayAfterTomorrow / dayBeforeYesterday, thisWeekday, nextWeekday, lastWeekday, optional monthsInDate, short/compact overrides

**duration** (`DurationLocale`):

- `PluralCategory`, `UnitForms` — as in age
- `CompactnessOverride` — optional `units`, `half`, `delimiter`, `ago`, `inFuture`
- `DurationLocale` — code, plural function, units (second through year), half, delimiter, justNow, ago, inFuture, short/compact overrides, optional `formatNumber` / `formatUnit`

**time** (`TimeLocale`):

- `ClockConfig` — hours (12 names, index 0 is twelve), midnight, noon, patterns (minute bucket 0–55 → template)
- `ShortClockConfig` — `use24h`, `format`, optional `amPm` pair
- `CompactnessOverride` — optional `clock`
- `TimeLocale` — code, clock, short/compact overrides

## 3. Write the codegen emitters

For each library, create `<library>/codegen/targets/<lang>/emitter.py`. Copy
the closest existing target in **that library**, since each library's emitter
reads different fields: for per-file output, `<library>/codegen/targets/python/emitter.py`;
for single-file output, `<library>/codegen/targets/kotlin/emitter.py`.

### Emitter functions

There is no plugin interface: each library's `generate.py` has a
`generate_<lang>()` function that imports what it needs from the emitter. The
common shapes are:

**Per-file emitters** (one file per locale, plus a barrel or registry):

```python
def emit_locale(data: dict) -> str:
    """Generate one locale file from the locale's dict."""

def output_path(code: str, root: str) -> str:
    """Return the output path for a locale file."""

def emit_barrel(all_locales: list[dict]) -> str:
    """Generate the index/barrel/registry file."""

def barrel_path(root: str) -> str:
    """Return the barrel file path."""
```

(Some targets name the last two `emit_registry` / `registry_path`.)

**Single-file emitters** (all locales in one file, like Kotlin or PHP):

```python
def emit_all(all_locales: list[dict]) -> str:
    """Generate a single file containing all locales."""

def output_path(root: str) -> str:
    """Return the output path."""
```

Start every generated file with `make_header(...)` from `codegen.banner`
(the 9-line `DO NOT EDIT` banner, in the target's comment syntax), and put
`locale_source_comment(code, ...)` (`Source: locales/<code>.json`) on each
locale file, or above each locale block in single-file output.

### The `data` dict

Each locale arrives as the shared fields plus one library's section:

| Key          | Type    | Description                                                     |
| ------------ | ------- | --------------------------------------------------------------- |
| `code`       | `str`   | Locale code (e.g. `"en"`, `"bci"`)                              |
| `name`       | `str`   | English name (e.g. `"English"`)                                 |
| `pluralRule` | `str`   | One of the 23 rule names (used by age and duration)            |
| ...          |         | The library's section: see [`docs/adding-a-locale.md`](../../docs/adding-a-locale.md) |

Key helpers to implement in each emitter: string escaping for the target
language (`_quote`), and the library's compound values: unit forms and stage
labels (age), unit forms (duration), string lists and maps (date), hour lists,
pattern maps and clock overrides (time).

Locale codes such as `is` and `or` are reserved words in some languages; see
how existing emitters rename them (e.g. Mojo's `is_locale.mojo`, Nim's
`isLocale.nim`).

## 4. Add plural rules (age and duration)

Create `age/codegen/targets/<lang>/plural_rules.py` and
`duration/codegen/targets/<lang>/plural_rules.py`. There are 23 plural rule
patterns (covering all CLDR cardinal patterns), each defined as a code string
in your target language. Date and time don't need plural rules.

1. Add a `_<LANG>` dict with code strings for all 23 rules
2. Add a getter function: `def get_<lang>_plural(rule: str) -> str`
3. If any rules need named functions (not inline), add a `_<LANG>_NAMED` dict
4. If your language distinguishes simple vs complex plurals, add `is_simple_plural()`

Use the existing entries in any `age/codegen/targets/*/plural_rules.py` as
templates. The two libraries' files for one language are normally identical.
See [`docs/adding-a-locale.md`](../../docs/adding-a-locale.md) for the rule table.

## 5. Register the generators

In each library's `<library>/codegen/generate.py`, add a generator function and
register it in `_GENERATORS`:

```python
def generate_newlang(locales: list[dict], check: bool, diffs: list[str]) -> None:
    from codegen.targets.newlang.emitter import barrel_path, emit_barrel, emit_locale, output_path

    for loc in locales:
        _write_or_check(output_path(loc["code"], ROOT), emit_locale(loc), check, diffs)
    _write_or_check(barrel_path(ROOT), emit_barrel(locales), check, diffs)

_GENERATORS = {
    ...
    "newlang": generate_newlang,
}
```

Per-file targets also need stale-file cleanup (see the `_clean_<lang>()`
helpers in `age/codegen/generate.py`) so a deleted locale's file is removed.
Run `python3 -m codegen.generate --lang newlang` from the library's directory
to generate just your language, and `just generate locales` from the root to
regenerate everything.

## 6. Implement the formatters

Port each library's formatter from an existing implementation, for example
`<library>/impl/python/src/sayable_<library>/format_<library>.py` or
`<library>/impl/typescript/src/format-<library>.ts`. The tiers each library
applies are in its `CLAUDE.md` (`age/CLAUDE.md`, `date/CLAUDE.md`,
`duration/CLAUDE.md`, `time/CLAUDE.md`). In outline:

- **age**: parse the two dates, compute calendar months and days, then apply
  the tiers days → weeks+days → months → years+months → years with ½ → years;
  `format_stage` maps the age to a stage with fixed month boundaries.
- **date**: `format_date`, `format_date_with_year` and `format_relative`, which
  picks today/tomorrow/yesterday, the ±2-day words, weekday phrases or the
  calendar date by day offset.
- **duration**: `format_duration`, `format_since` and `format_until`, tiering
  seconds into seconds → minutes → hours → days → weeks → months → years, with
  the half marker and a 500-year ceiling.
- **time**: `format_time`, rounding to 5 minutes for spoken time and formatting
  digits for short and compact.

All four support three compactness levels (normal, short, compact) through the
locale's overrides. Inputs follow the strict grammar in
[`docs/input-formats.md`](../../docs/input-formats.md): real calendar dates
only, no unpadded or padded-with-spaces input. Lenient parsing is a
cross-language divergence.

**Key constraint:** zero runtime dependencies. Date arithmetic must be
hand-implemented. Expose a version constant matching
`<library>/config/version.json` ([`age/docs/versioning.md`](../../age/docs/versioning.md)).

## 7. Write the tests

All tests come from each library's `<library>/config/tests.json`. Implement
every case in the language's standard test framework, annotated with its
canonical ID as a comment above the test or inline (`// 3.1`, `# 25.10, 25.11`),
one ID per test (`tools/check_test_ids.py` doesn't expand ranges like
"3.1 - 3.12"). Use the explicit inputs and reference dates from the spec; never
the current date, except in the default-asOf sections (age 15.x and 23.x), which
test exactly that: the library must default to today when the as-of date is
absent or empty. A case the language truly can't express (a null argument of a
non-nullable type) gets a `<ID>: not applicable — <reason>` comment instead.

`tools/check_test_ids.py --lang <lang>` lists the canonical IDs a suite never
mentions; `just check` runs it for every language.

## 8. Add the CLI

Create `cli/impl/<lang>/` with a `sayable` CLI that exposes every public
function of all four libraries and implements `--repl` mode, as specified in
[`cli/SPEC.md`](../../cli/SPEC.md). Provide a `build.sh` if the CLI needs
building (see `cli/impl/go/build.sh`); the built binary or wrapper script is
`cli/impl/<lang>/sayable`.

The CLI is how the generated harness and the fuzzer test your implementation
independently of its own tests ([`docs/testing.md`](../../docs/testing.md)):

```bash
python3 cli/check-repl.py cli/impl/<lang>/sayable   # REPL protocol conformance
just test cli <lang>                                 # every canonical case, all four libraries
just fuzz --langs <lang>                             # random requests vs the Python reference
```

## 9. Create the language definition

Everything about the language itself lives in `.hecatron/languages/<lang>/`.
Use an existing language as a template (e.g. `.hecatron/languages/go/`).

### `config.json`

```json
{
  "abbreviation": "nl",
  "dependency": "newlang",
  "lintDependency": null,
  "compiled": true,
  "hasClean": true,
  "hasLint": true
}
```

`.hecatron/tools/language/helpers.sh` builds every language list,
abbreviation and dependency map from these files, so there are no shared lists
to edit. See [`.hecatron/CLAUDE.md`](../CLAUDE.md) for the fields.

### Test runner

`test.sh` runs one library's native suite. `just test <lang>` calls it once per
library (through `.hecatron/tools/language/test.sh`) from inside
`<library>/impl/<lang>/`, with the library name as `$1`. It builds whatever the
suite needs and exits non-zero on a failure:

```bash
#!/usr/bin/env bash
# Run one library's Go tests ($1: age, date, duration or time).
set -euo pipefail
go test ./...
```

Use a `case "$1"` only where file names differ per library. The commands should
match the language's CI workflow.

### Installer scripts

At minimum `install.sh`, `uninstall.sh` and `version.sh`, plus `metadata.sh`
if the language has package manifests. Each script runs its action on execute
(no subcommands) and sources `../_common.sh` for shared helpers. `version.sh`
updates the version constant and manifests (today `just version` bumps age
only; see the root `CLAUDE.md`). See
[`installers.md`](installers.md) for the full reference.

### Dockerfile snippet

Create **`.hecatron/languages/<lang>/dockerfile`** for the Dockerfile
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

`age/Dockerfile` is regenerated automatically by `just add` / `just remove`, or
manually with `just dockerfile`. Never edit it by hand.

### CI workflow

Create `.hecatron/languages/<lang>/workflow.yml`, the canonical source;
`just add <lang>` copies it to `.github/workflows/<lang>.yml` with a banner, and
`just remove <lang>` deletes it. Include a version matrix when the language
supports it. The workflow must:

1. Run the native tests of **every** library: `tools/check_workflow_components.py`
   (part of `just check`) fails if a library that has `<library>/impl/<lang>/`
   isn't tested.
2. Build the CLI and run the REPL conformance check, the harness and the fuzzer
   against it.

Use `.hecatron/languages/go/workflow.yml` as the template (one step per
library), or `.hecatron/languages/goose/workflow.yml` for a matrix over
libraries:

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
        working-directory: age/impl/newlang
    steps:
      - uses: actions/checkout@v4
      - uses: <setup-action>
        with:
          newlang-version: ${{ matrix.newlang-version }}
      - run: <test-command>

      - name: Test date library
        working-directory: date/impl/newlang
        run: <test-command>
      - name: Test duration library
        working-directory: duration/impl/newlang
        run: <test-command>
      - name: Test time library
        working-directory: time/impl/newlang
        run: <test-command>

      - name: Build CLI
        working-directory: .
        run: bash cli/impl/newlang/build.sh
      - name: CLI REPL conformance and harness (single REPL process)
        working-directory: .
        run: python3 cli/check-repl.py cli/impl/newlang/sayable && bash cli/test-harness.sh cli/impl/newlang/sayable
      - name: Fuzz against the Python reference
        working-directory: .
        run: python3 cli/fuzz.py --langs newlang -n 2000
```

## 10. Add justfile arms

Register the language in the root `justfile`'s `build`, `lint` and `clean`
recipes, with its abbreviation. `just test <lang>` needs no arm: it runs the
language's `test.sh` (above) for all four libraries. The bare (no-argument)
runs pick the language up from `config.json` automatically. The date, duration
and time directories also have their own justfiles covering a subset of
languages; adding the language there is optional.

Every recipe must keep handling `help` / `--help` (see the root `CLAUDE.md`).

## 11. Add linting

Set up the language's standard linter with strict settings, for all four
libraries. See [`age/docs/code-quality.md`](../../age/docs/code-quality.md) for
the project's linting philosophy:

- Zero warnings in CI
- Suppress with a documented reason, never a blanket ignore
- Match ecosystem conventions

The generated locale files must pass too; long lines in locale data are
expected and may be suppressed for those files. Document the configuration in
`age/docs/code-quality-compiled.md` or `age/docs/code-quality-dynamic.md`.

## 12. Update cross-language files

### `.gitignore`

Add build artifact patterns to each library's `<library>/impl/.gitignore` (not
the root `.gitignore`), in a section named after the language, keeping
sections in alphabetical order:

```gitignore
# NewLang
newlang/build/
```

The root `.gitignore` is reserved for cross-language patterns (`__pycache__/`,
`.logs/`, etc.).

### READMEs

Every implementation **must** have a `<library>/impl/<lang>/README.md` with
installation, usage and API docs, and a `## License` section linking to
the root `LICENSE.md` (`[MIT](../../../LICENSE.md)` from that depth). Each library's
`README.md` table must link to it in the Docs column. Use an existing
language's README as a template (e.g. `age/impl/go/README.md`).

### Documentation

Several docs contain per-language tables or lists that must include every
implementation. **Keep table rows sorted alphabetically by language name.** Do
not hardcode the number of implementations in new prose; the counts live in the
root `CLAUDE.md` project description, the root `README.md` and each library's
`CLAUDE.md`.

| Document                                                          | What to update                                          |
| ----------------------------------------------------------------- | ------------------------------------------------------- |
| [`CLAUDE.md`](../../CLAUDE.md)                                    | Language count and list in the project description      |
| [`README.md`](../../README.md)                                    | Language counts and list                                |
| `age/CLAUDE.md`, `date/CLAUDE.md`, `duration/CLAUDE.md`, `time/CLAUDE.md` | Language count and list, codegen emitter strategies |
| Each library's `README.md`                                        | Implementations table                                   |
| [`age/codegen/README.md`](../../age/codegen/README.md)            | Output per language                                     |
| [`age/docs/code-quality.md`](../../age/docs/code-quality.md)      | Naming conventions, Immutability, Quick reference       |
| [`age/docs/packaging.md`](../../age/docs/packaging.md)            | File layout / initialization, Tree-shaking action table |
| [`CONTRIBUTING.md`](../../CONTRIBUTING.md)                        | Single-language commands table                          |
| [`docs/compatibility.md`](../../docs/compatibility.md)            | Generated: run `python3 tools/generate_compatibility.py` |

## 13. Register and verify

```bash
just add newlang              # add to added.json, copy the workflow, regenerate age/Dockerfile
python3 tools/generate_compatibility.py
just generate locales         # regenerate every library's locales, including the new language
just check                    # generated files, docs, Dockerfile, workflow coverage all current
just test newlang             # the new language's tests
just lint newlang             # the new language's linter
just test cli newlang         # canonical cases through the new CLI
just fuzz --langs newlang     # agreement with the Python reference
just test                     # all languages, to confirm nothing broke
```

## Checklist

- [ ] `<library>/impl/<lang>/` with idiomatic structure, in all four libraries
- [ ] Types for each library (`AgeLocale`, `DateLocale`, `DurationLocale`, `TimeLocale` and their helpers)
- [ ] Emitter in `<library>/codegen/targets/<lang>/emitter.py` for all four libraries
- [ ] Plural rules (all 23 patterns) in `age/` and `duration/codegen/targets/<lang>/plural_rules.py`
- [ ] Generator registered in each library's `codegen/generate.py` (`_GENERATORS`)
- [ ] Generated locale files in a `locales/` subdirectory, passing the language's linter
- [ ] Formatters ported for all four libraries, with zero runtime dependencies and a version constant
- [ ] Every case in each library's `config/tests.json` implemented and annotated with its ID
- [ ] CLI in `cli/impl/<lang>/` implementing `--repl`; `check-repl.py`, `just test cli <lang>` and `just fuzz --langs <lang>` pass
- [ ] `.hecatron/languages/<lang>/`: `config.json`, `test.sh`, `install.sh`, `uninstall.sh`, `version.sh` (and `metadata.sh`), `dockerfile`, `workflow.yml` testing every library and the CLI
- [ ] `tools/check_test_ids.py --lang <lang>` reports nothing, and `just test <lang>` passes for all four libraries
- [ ] Root justfile arms for build, lint and clean
- [ ] Build artifacts in each `<library>/impl/.gitignore`
- [ ] Linter configured with zero warnings
- [ ] `<library>/impl/<lang>/README.md` with license link, and a row in each library's README table
- [ ] Cross-language docs updated and sorted alphabetically
- [ ] `just add <lang>` run and `docs/compatibility.md` regenerated
- [ ] `just check`, `just test` and `just lint` pass
