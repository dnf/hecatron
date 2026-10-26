# A ten-language library: hecatron or ad hoc?

This is a worked comparison of maintaining one library, implemented natively
in ten languages in one repository, with hecatron and without it. The numbers
come from this repository and from [sayable](https://github.com/dnf/sayable),
which hecatron grew out of; where something is an estimate, it says so.

## The library

A date-formatting library ("3 days ago", "next Tuesday") with one shared
specification and test data, implemented in **Go, Python, TypeScript, Rust,
Java, Kotlin, C#, Swift, Ruby and C**. One maintainer works with AI coding
agents, which write most of the implementations, and contributors work on
macOS and Linux. Every change should land in all ten languages at once, and
CI should prove it.

Either way, some work is the same: the ten implementations, their test
suites, each language's package manifest, and a CI workflow per language.
The comparison is about everything around them.

## What you maintain

| | Ad hoc | With hecatron |
| --- | --- | --- |
| **Toolchain setup** | Instructions or scripts for ten toolchains on macOS and on Linux, kept working as distributions and toolchains change. Hecatron's definitions for these ten languages are 585 lines plus 155 lines of shared helpers, which is roughly what doing this properly takes. | None. `hecatron doctor --fix` installs and repairs them. |
| **Checking a machine** | Nothing, usually: breakages show up as confusing test failures. | `hecatron doctor` checks every toolchain and its linter, plus known breakages (a linter built with an older compiler, a moved virtualenv). |
| **Running everything** | A script or Makefile that runs ten test commands, usually one after another, with output interleaved or lost. | `hecatron test` runs all ten in parallel with a live display, a summary, and a log per language. |
| **Per-language commands** | Ten different commands to remember (`go test ./...`, `./gradlew test`, `dotnet test`, `swift test`...), for people and agents alike. | Hooks of one to a few lines each: sayable's test, lint, build and clean hooks for these ten languages are 47 lines of commands in total. Everyone runs `hecatron test <lang>`. |
| **CI** | Ten workflow files, written and kept consistent by hand. | The same ten workflow files (sayable's are 475 lines), but `hecatron workflows` keeps the generated copies in sync, and `hecatron check` (run it in CI) fails when one is stale or doesn't cover every component. |
| **Container image** | A Dockerfile with ten toolchains, edited by hand. | Generated from each language's snippet with `hecatron dockerfile`; `hecatron check` reports drift. |
| **Adding an eleventh language** | Write its setup instructions, CI workflow, Dockerfile lines, and add it to the run-everything script. | `hecatron add <lang>`, a few short hooks, a workflow template. If hecatron knows the language (51 so far), the toolchain side is done. |
| **The tool itself** | Nothing. | A dependency: hecatron's core is about 3,500 lines of bash and Python you don't maintain but do rely on. |

## Everyday tasks

| Task | Ad hoc | With hecatron |
| --- | --- | --- |
| Set up a new laptop | Follow ten sets of instructions; discover the ones that are out of date. | `hecatron doctor --fix` |
| "Did my change break anything?" | Run ten commands, or the script, and read the output. | `hecatron test` |
| Test one language | Remember its command and directory. | `hecatron test rust` |
| Hand a task to an AI agent | Explain how each language is built and tested, or let it guess. | The same commands for every language; `doctor` tells it what's missing. |
| A test fails only on one machine | Compare toolchains by hand. | `hecatron doctor` on both machines. |
| Upgrade a toolchain | Update instructions, CI and the Dockerfile separately. | Change one definition (planned: one version setting, see [design.md §18](design.md)). |

## What breaks, and who fixes it

Toolchain setup rots, and it rots without anyone touching the repository.
When hecatron's definitions, written and used daily on macOS, were first run
on fresh Debian and Ubuntu machines, **6 of these 10 languages failed to
install or to build a hello world**:

- **Go:** pinned to a version too old to build its linter, and never put on
  PATH.
- **Rust:** no C compiler on a fresh machine, so nothing could link.
- **Swift:** an old build, missing system libraries its package manager
  needs.
- **Java and Kotlin:** Debian 13 no longer packages the JDK version they
  used.
- **C#:** Debian doesn't package .NET at all.

Across all 51 languages, 22 failed on a fresh machine, and the Dockerfile
snippets needed more for arm64. In an ad hoc setup, each project finds these
problems itself, usually when a new contributor joins or CI's base image
changes, and fixes them in its own scripts. With hecatron, each was fixed
once, in one place, with a test that installs every language on Ubuntu 24.04,
Ubuntu 26.04 and Debian 13 (`tests/docker/`), and every project gets the fix
by updating hecatron.

## What hecatron costs you

- **A young dependency.** Hecatron is at version 0.2. Its commands and
  configuration may still change before 1.0, and it has bugs; the list above
  was hecatron's own.
- **Its requirements.** bash 4 or later and Python 3.9 or later; on macOS
  that means installing a newer bash.
- **Its conventions.** Hooks run once per component from the component's
  directory; a project with an unusual layout has to describe it in
  `config.json`.
- **Installers change your machine.** They install system packages, some with
  sudo. Hecatron only runs them when asked (`install`, `doctor --fix`).
- **No Windows.** Installers and hooks are bash.

## When ad hoc is the better choice

- **Two or three mainstream languages.** Each has a well-maintained
  `setup-*` GitHub Action and a familiar install; there's little to manage.
- **You need Windows.**
- **You already use a polyglot build system** (Bazel, Nx, Pants) that owns
  toolchains and test runs.
- **The languages live in separate repositories** and are released and
  versioned independently.

## Summary

With ten languages, the work around the code (toolchains on every machine,
one way to run everything, CI and a container image that stay consistent)
grows with every language, and most of it is the same for every project.
Ad hoc, each project writes and repairs its own version of it. With
hecatron, a project writes a few lines of hooks per language and shares the
rest; the cost is depending on a young tool and following its conventions.
For two or three languages, ad hoc is fine. In our judgement the shared
version becomes less work somewhere around five languages, and clearly is
at ten.
