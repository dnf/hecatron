<h1 align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/logo/hero-dark.svg">
    <img alt="hecatron" src="assets/logo/hero.svg" width="380">
  </picture>
</h1>

<p align="center"><strong>Tools for polyglot repositories.</strong></p>

<div align="center">

| CI | Release | Languages | License |
| :-: | :-: | :-: | :-: |
| [![CI](https://github.com/dnf/hecatron/actions/workflows/ci.yml/badge.svg)](https://github.com/dnf/hecatron/actions/workflows/ci.yml) | [![Latest release](https://img.shields.io/github/v/release/dnf/hecatron?sort=semver)](https://github.com/dnf/hecatron/releases) | [![Languages](https://img.shields.io/badge/languages-54-3B5BFF)](docs/coverage.md) | [![License: MIT](https://img.shields.io/github/license/dnf/hecatron)](LICENSE.md) |

</div>

Hecatron installs, checks and repairs the toolchains of every language in a
repository, runs each language's tests, linters and builds in parallel, and
keeps the CI workflows and Dockerfile that cover them in sync. It is for
repositories where one project is implemented in many programming languages.

## Why

Historically, a library has existed in one language, or at most a handful.
That was never a decision about where it belonged; it was a cost. Porting a
library takes time, and keeping the ports in sync with the original takes
time every release after, so most libraries never get ported, and the ports
that exist lag behind. That's why learning a new language has also meant
learning a new ecosystem of libraries: the date library, the parser, the
client you rely on in one language usually isn't there in the next.

It doesn't have to be that way any more. AI coding agents make writing and
maintaining an implementation in another language cheap, so one person can
ship the same library natively in a dozen or fifty languages. The best place
for those implementations is one repository, where a fix lands everywhere in
one pull request and every language runs the same shared tests
([more](docs/why-one-repository.md)).

What's left is the infrastructure, and it grows with every language: a
toolchain to install and keep working, test, lint and build commands, CI jobs
and Dockerfile lines, each different. With fifty languages, keeping that
working becomes the job. Hecatron does that job. It grew up inside
[sayable](https://github.com/dnf/sayable), a set of formatting libraries in 51
languages, and was split out so any polyglot repository can use it.

## Install

Hecatron runs on the bash and Python 3 that current macOS and Linux ship (bash
3.2 or later, Python 3.9 or later).

```bash
curl -fsSL https://raw.githubusercontent.com/dnf/hecatron/main/install.sh | sh
```

This installs anything hecatron itself needs that's missing (the Command Line
Tools on macOS; git, Python 3 and perl on Linux), then the latest release
into `~/.hecatron`, and adds it to your shell's startup files (bash, zsh and
fish), so new shells have `hct`. It prints the one line that puts it on PATH
in the current shell. Run it again to update. From a clone instead, source the installer, which also sets up the
shell you're in:

```bash
git clone https://github.com/dnf/hecatron ~/.hecatron && . ~/.hecatron/install.sh
```

## Quick start

```bash
cd your-repo
hct init          # detects each language and where its code lives, writes .hecatron/config.json
hct doctor --fix  # installs and repairs every language's toolchain
hct test          # every language's tests, in parallel
hct test go       # one language
```

`hct` is short for `hecatron`. `hct help` lists every command, and
`hct <command> help` its options.

## What it does

- **Toolchains that keep working.** `hct doctor` checks that every language's
  toolchain is installed and actually works; `--fix` runs its installer,
  which is idempotent and repairs the broken states the doctor finds (a
  linter built with an older compiler, a moved virtualenv, an unlinked
  Homebrew formula).
- **The right version for each component.** For Python, Go, Rust, Node.js and
  Java, a pinned default, or the version a component asks for in its
  ecosystem's own file (`.python-version`, `go.mod`, `rust-toolchain.toml`,
  `.nvmrc`, `.java-version`, `.tool-versions`, `mise.toml`...). Versions are
  installed side by side, and each component runs with its own.
- **No hook files for conventional projects.** Default test, lint, build and
  clean commands for all 54 languages, chosen from the build files present
  (`pom.xml` → Maven, `Cargo.toml` → Cargo, `CMakeLists.txt` → CMake and
  CTest...). A project overrides any of them with its own script.
- **Everything at once.** `hct test`, `lint`, `build` and `clean` run every
  language in parallel with a live display and a log per language.
- **Generated CI and Dockerfile.** `hct workflows` writes one CI workflow per
  language and `hct dockerfile` an image with every toolchain (and, for the
  languages with versions, each version the components use); `hct check`
  fails when either falls behind.
- **Finds your layout.** `hct init` recognises the directory names real
  projects use (`dotnet/` for C#, `javascript/`, `lib/py`...) and skips
  examples, tests and vendored code.

Hecatron calls each language's own tools (`cargo`, `go`, `gradle`, `uv`...).
It doesn't build anything itself, and it doesn't know what your code does.

## Tested

- **Every language, every week,** on fresh Ubuntu 24.04, Ubuntu 26.04 and
  Debian 13 machines (x86_64 and arm64) and on macOS (Apple Silicon and
  Intel): install, doctor and reinstall, then a smoke test that builds and
  runs a small program with the new toolchain.
  [docs/coverage.md](docs/coverage.md) has the matrix.
- **Real projects.** [sayable](https://github.com/dnf/sayable) runs all 51 of
  its languages through hecatron. On a fresh machine,
  [cucumber/tag-expressions](https://github.com/cucumber/tag-expressions)
  (9 languages) and
  [cucumber/cucumber-expressions](https://github.com/cucumber/cucumber-expressions)
  (8) pass every language's own test suite with `hct init`, `hct install`
  and `hct test`, and no configuration.

## Hecatron or ad hoc?

For a library in ten languages ([full comparison](docs/hecatron-vs-ad-hoc.md)):

| Ad hoc | With hecatron |
| --- | --- |
| Setup docs for 10 toolchains × 2 OSes | `hct doctor --fix` |
| 10 test commands to remember | `hct test`, all in parallel |
| CI and Dockerfile kept in sync by hand | `hct workflows`, `hct dockerfile`<br>`hct check` catches drift |
| Each project fixes toolchain rot itself | Fixed once, tested on every supported distribution |

## Compared with other tools

| Tool | What it does | Next to hecatron |
| --- | --- | --- |
| [mise](https://mise.jdx.dev), [asdf](https://asdf-vm.com) | Install and switch tool versions | Hecatron also installs versions, and reads `.tool-versions` and `mise.toml`, but its job is the whole loop: repairing toolchains, running every language's tests and lints, and generating CI |
| [Nx](https://nx.dev), [Bazel](https://bazel.build), [moon](https://moonrepo.dev), [Pants](https://www.pantsbuild.org) | A build graph across the repository: dependencies between targets, caching, building only what changed | Hecatron has no build graph or cache; it runs each language's own build tool. It fits one library implemented separately per language; a repository whose parts depend on each other across languages is those tools' ground |
| [devbox](https://www.jetify.com/devbox), [Nix](https://nixos.org) | Reproducible environments from Nix packages | Hecatron uses each ecosystem's own installers (official releases, rustup, uv, apt, Homebrew) and checks that they work, without Nix |

## Configuring a project

`hct init` writes `.hecatron/config.json`, committed with the code:

```json
{
  "schema": 1,
  "project": "sayable",
  "components": ["age", "date", "duration", "time"],
  "implPath": "{component}/impl/{lang}",
  "languages": ["go", "python", "rust", "typescript"]
}
```

A project can override any default hook with its own
`.hecatron/languages/<lang>/test.sh` (or `lint.sh`, `build.sh`, `clean.sh`),
run once per component from that component's directory for the language. It
can also override any file of a built-in language definition there, or define
a language hecatron doesn't know. [docs/design.md](docs/design.md) describes
the full configuration.

## Languages

Hecatron ships definitions for 54 languages:

Ada, Bash, C, C++, C#, Clojure, COBOL, Common Lisp, Crystal, D, Dart, Elixir,
Elm, Erlang, F#, Fortran, Gleam, Go, Goose, Groovy, Haskell, Haxe, Janet,
Java, JavaScript, Julia, Kotlin, Lua, Mojo, Nim, Objective-C, OCaml, Octave,
Odin, Pascal, Perl, Pharo, PHP, PowerShell, Prolog, Python, R, Racket, Raku,
Ruby, Rust, Scala, Swift, Tcl, TypeScript, V, VB.NET, Wren and Zig.

[docs/languages.md](docs/languages.md) has each one with a link to its home,
what it's for, and the build systems, test runners and linters hecatron uses
for it; it's also a good place to discover a language you haven't tried.
Adding another is a directory of small scripts; see
[docs/adding-a-language.md](docs/adding-a-language.md).

## Status

Hecatron is young: version 0.3. It works and runs sayable's checks, but
expect changes before 1.0. Changes are listed in
[CHANGELOG.md](CHANGELOG.md).

- **Platforms:** macOS (Apple Silicon and Intel) and Linux (Debian and
  Ubuntu, x86_64 and arm64). Windows isn't planned; installers and hooks are
  bash.
- **Next:** a GitHub Action for CI.

## Documentation

| Document | Contents |
| --- | --- |
| [docs/why-one-repository.md](docs/why-one-repository.md) | Why one library in many languages belongs in one repository |
| [docs/design.md](docs/design.md) | How hecatron works and why: concepts, configuration, commands, and the decisions behind them |
| [docs/languages.md](docs/languages.md) | Every language: its home, what it's for, and how hecatron builds, tests and lints it |
| [docs/coverage.md](docs/coverage.md) | Which languages install and work on which platforms |
| [docs/hecatron-vs-ad-hoc.md](docs/hecatron-vs-ad-hoc.md) | A ten-language library maintained with hecatron and without it |
| [docs/installers.md](docs/installers.md) | How each language's toolchain is installed on macOS and Linux, and toolchains languages share |
| [docs/adding-a-language.md](docs/adding-a-language.md) | Writing a language definition |
| [docs/build-conventions.md](docs/build-conventions.md) | Where each language's build artifacts go |
| [libexec/runner/README.md](libexec/runner/README.md) | The parallel runner and its live display |

## Getting help

Questions, bugs and ideas go to
[GitHub issues](https://github.com/dnf/hecatron/issues). For a security
problem, see [SECURITY.md](SECURITY.md).

## Contributing

Issues and pull requests are welcome. [CONTRIBUTING.md](CONTRIBUTING.md) has
the checks to run (`hct self-test`), the conventions the code follows, and a
map of the repository.

## Acknowledgements

- [Dave Sifry](https://github.com/dsifry), for
  [metaswarm](https://github.com/dsifry/metaswarm), his open-source framework
  for running teams of AI coding agents through a whole development
  lifecycle.
- The wordmark is set in [Space Grotesk](https://github.com/floriankarsten/space-grotesk)
  by Florian Karsten.

## License

[MIT](LICENSE.md)

## The name

Hecatron comes from the [Hecatoncheires](https://en.wikipedia.org/wiki/Hecatoncheires),
the "hundred-handed ones" of [Greek myth](https://en.wikipedia.org/wiki/Greek_mythology)
(*hekaton*, a hundred, and *cheir*, a hand). [Hesiod](https://en.wikipedia.org/wiki/Hesiod)'s
[*Theogony*](https://en.wikipedia.org/wiki/Theogony) tells of three brothers,
Briareus, Cottus and Gyges, each with a hundred hands and fifty heads, who
turned the [war against the Titans](https://en.wikipedia.org/wiki/Titanomachy)
by hurling three hundred boulders at once. A repository in a dozen languages
needs that many hands: one for each toolchain, package manager, test runner
and CI job, all working at once.

The logo draws on another figure from the same myths:
[Hecate](https://en.wikipedia.org/wiki/Hecate), goddess of crossroads. Its
three angle brackets are the roads, pointing in at the point where they meet,
as every language's tooling meets in one command.
