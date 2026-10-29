# A ten-language library: hecatron or ad hoc?

One library, implemented natively in ten languages in one repository,
maintained with hecatron and without it. Numbers come from this repository
and [sayable](https://github.com/dnf/sayable).

## The library

A date-formatting library ("3 days ago") in **Go, Python, TypeScript, Rust,
Java, Kotlin, C#, Swift, Ruby and C**, with one shared spec and test data. One
maintainer, AI agents writing most of the code, contributors on macOS and
Linux. Every change lands in all ten languages at once.

Either way you write the code, its tests, its manifests and a CI workflow per
language. The difference is everything around them.

## What you maintain

| | Ad hoc | With hecatron |
| --- | --- | --- |
| **Toolchains** | Setup for 10 toolchains × 2 OSes, kept working by hand (hecatron's equivalent: 740 lines) | `hct doctor --fix` installs and repairs them |
| **Machine health** | Found as confusing test failures | `hct doctor` checks toolchains, linters and known breakages |
| **Run everything** | A script running 10 commands in sequence | `hct test`: all 10 in parallel, live display, a log each |
| **Per-language commands** | 10 to remember: `go test ./...`, `./gradlew test`, `dotnet test`... | `hct test <lang>`; hooks are 47 lines for all 10 |
| **CI** | 10 workflows kept consistent by hand | `hct workflows` keeps generated copies in sync<br>`hct check` fails when one is stale or has gaps |
| **Container image** | A hand-edited Dockerfile | `hct dockerfile` generates it; `hct check` catches drift |
| **An 11th language** | Setup docs, CI, Dockerfile, run script | `hct add <lang>` and a few lines of hooks |
| **The tool** | — | A dependency: ~3,500 lines you rely on but don't maintain |

## Everyday tasks

| Task | Ad hoc | With hecatron |
| --- | --- | --- |
| New laptop | Follow 10 sets of instructions | `hct doctor --fix` |
| Did I break anything? | Run 10 commands | `hct test` |
| One language | Remember its command | `hct test rust` |
| Brief an AI agent | Explain each build | Same commands everywhere; `hct doctor` says what's missing |
| Fails on one machine | Compare toolchains by hand | `hct doctor` on both |
| Upgrade a toolchain | Update docs, CI and Dockerfile | One `version` setting ([toolchain versions](versions.md); rolling out per language) |

## What breaks

Toolchain setup rots without anyone touching the repository. Hecatron's
installers, used daily on macOS, were first run on fresh Debian and Ubuntu
machines this month: **6 of these 10 languages failed** (Go, Rust, Swift,
Java, Kotlin, C#), and 22 of all 51.

Ad hoc, every project finds and fixes these itself, usually when someone new
joins. With hecatron they're fixed once, tested on Ubuntu 24.04, Ubuntu 26.04
and Debian 13, and every project gets the fix by updating.

## What it costs

- **Young:** version 0.2; expect changes before 1.0, and bugs (the failures
  above were hecatron's).
- **Requirements:** the bash and Python 3 that current macOS and Linux ship
  (bash 3.2+, Python 3.9+).
- **Conventions:** hooks run per component; unusual layouts need describing.
- **Installers change your machine**, some with sudo, only when you ask.
- **No Windows.**

## When to skip it

- Two or three mainstream languages
- Windows
- Bazel, Nx or Pants already owns your toolchains
- Separate repositories, released independently

## Summary

The work around ten implementations grows with every language and is mostly
the same for every project. Ad hoc, each project builds and repairs its own;
with hecatron, a project writes a few lines per language and shares the
rest. In our judgement that pays off from around five languages.
