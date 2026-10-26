<h1 align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/logo/hero-dark.svg">
    <img alt="hecatron" src="assets/logo/hero.svg" width="380">
  </picture>
</h1>

<p align="center"><strong>Tools for polyglot repositories.</strong></p>

Hecatron installs, checks and repairs the toolchains of every language in a
repository, runs each language's tests, linters and builds in parallel, and
keeps the CI workflows and Dockerfile that cover them in sync. It is for
repositories where one project is implemented in many programming languages.

## Why

A useful library ought to exist in every language its users write. Usually it
doesn't: there's a reference implementation in one language, ports by other
people that lag behind it, and bindings to a native library that are awkward
to install and feel foreign in the language that calls them. Writing and
maintaining a native, idiomatic implementation per language used to need a
team fluent in each one, so almost nobody did it.

AI coding agents change that cost. An agent can write, port and maintain an
implementation in a language its human maintainer doesn't know, so one person
can ship the same library natively in a dozen or fifty languages. Once
implementations are cheap, the best place for them is **one repository**:

- **One change lands everywhere.** A bug fix or a new feature is one pull
  request across every language, not fifty, and no implementation drifts.
- **One specification and one test suite.** Shared test data defines the
  behaviour, and every language's tests run against it.
- **Agents see everything.** An agent porting a change reads the
  implementation it's porting from, in the same checkout.
- **Users get native packages.** Each language publishes to its own registry
  (crates.io, PyPI, npm, Maven Central...) with an API that feels native, not
  a wrapper around someone else's runtime.

What's left is the infrastructure, and it grows with every language. Each one
has a toolchain to install, and toolchains break in their own ways: a linter
built with an older compiler, a virtualenv that moved, a Homebrew formula
left unlinked. Each has its own test, lint and build commands, CI jobs and
Dockerfile lines. With fifty languages, keeping that working becomes the job.

Hecatron does that job. It grew up inside
[sayable](https://github.com/dnf/sayable), a set of human-friendly formatting
libraries implemented in 51 languages, and was split out so any polyglot
repository can use it.

## What it does

- **Toolchains.** `hecatron doctor` checks that every language's toolchain is
  installed and actually works; `--fix` runs its installer, which is
  idempotent and repairs the broken states the doctor finds. Installers cover
  macOS and Linux (Debian and Ubuntu).
- **Running.** `hecatron test`, `lint`, `build` and `clean` run each
  language's project hooks for every component, or for every language at once
  in parallel with a live progress display and per-language logs.
- **Generated files.** `hecatron workflows` writes one CI workflow per
  language, and `hecatron dockerfile` a Dockerfile with every toolchain, each
  with a DO NOT EDIT banner; `hecatron check` verifies they're current.
- **Project management.** `hecatron add`, `remove`, `enable` and `disable`
  languages; `list` shows where each language's files come from.

Hecatron calls each language's own tools (`cargo`, `go`, `gradle`, `uv`...).
It doesn't build anything itself, and it doesn't know what your code does.

## Quick start

Hecatron needs bash 4 or later and Python 3.9 or later. macOS ships bash 3.2;
`brew install bash` gets a current one.

```bash
git clone https://github.com/dnf/hecatron && export PATH="$PWD/hecatron/bin:$PATH"

cd your-repo
hecatron init          # detects languages from tracked manifests, writes .hecatron/config.json
hecatron doctor        # checks each toolchain; --fix installs and repairs
hecatron test          # every language's tests, in parallel
hct test go -c api     # one language, one component (hct is the short form)
```

`hecatron help` lists every command, and `hecatron <command> help` its options.

## Configuring a project

A project keeps its hecatron configuration in `.hecatron/`, committed with the
code:

```json
{
  "schema": 1,
  "project": "sayable",
  "components": ["age", "date", "duration", "time"],
  "implPath": "{component}/impl/{lang}",
  "languages": ["go", "python", "rust", "typescript"]
}
```

Each language's project hooks live in `.hecatron/languages/<lang>/`:
`test.sh`, `lint.sh`, `build.sh` and `clean.sh`. Each runs once per component,
from that component's directory for the language. A project can also override
any file of a built-in language definition there, or define a language
hecatron doesn't know. [docs/design.md](docs/design.md) describes the full
configuration.

## Languages

Hecatron ships definitions for 51 languages:

Ada, Bash, C, C++, C#, Clojure, COBOL, Common Lisp, Crystal, D, Dart, Elixir,
Elm, Erlang, F#, Fortran, Gleam, Go, Goose, Groovy, Haskell, Janet, Java,
Julia, Kotlin, Lua, Mojo, Nim, OCaml, Octave, Odin, Pascal, Perl, Pharo, PHP,
PowerShell, Prolog, Python, R, Racket, Raku, Ruby, Rust, Scala, Swift, Tcl,
TypeScript, V, VB.NET, Wren and Zig.

Adding another is a directory of small scripts; see
[docs/adding-a-language.md](docs/adding-a-language.md).

## Status

Hecatron is early: version 0.1. The command works and runs sayable's checks,
but expect changes before 1.0.

- **Platforms:** macOS (Apple Silicon and Intel) and Linux (Debian and
  Ubuntu). Windows isn't planned; installers and hooks are bash.
- **Installing:** from a clone, as above. A Homebrew tap and a GitHub Action
  for CI are planned.
- **Next:** default hooks, so a conventionally laid-out project needs no hook
  files, and per-language toolchain versions a project can choose.

## Documentation

| Document | Contents |
| --- | --- |
| [docs/design.md](docs/design.md) | How hecatron works and why: concepts, configuration, commands, and the decisions behind them |
| [docs/installers.md](docs/installers.md) | How each language's toolchain is installed on macOS and Linux, and toolchains languages share |
| [docs/adding-a-language.md](docs/adding-a-language.md) | Writing a language definition |
| [docs/build-conventions.md](docs/build-conventions.md) | Where each language's build artifacts go |
| [libexec/runner/README.md](libexec/runner/README.md) | The parallel runner and its live display |

## Contributing

Issues and pull requests are welcome. Before sending a change, run the checks:

```bash
bash libexec/lint/shellcheck.sh            # zero warnings
python3 -m unittest discover -s tests
```

[CLAUDE.md](CLAUDE.md) has the conventions the code follows (shell style,
idempotent installers, generic language definitions), for people and coding
agents alike. Changes to how hecatron works start in
[docs/design.md](docs/design.md).

## Repository layout

| Path | Contents |
| --- | --- |
| `bin/hecatron`, `bin/hct` | The command |
| `languages/<lang>/` | Language definitions: `config.json`, `install.sh`, `uninstall.sh`, `doctor.sh`, Dockerfile snippet |
| `languages/_common.sh` | Shared installer helpers |
| `libexec/lib/` | The project model (`project.py`), shell helpers, per-component dispatch |
| `libexec/commands/` | The commands (`run.sh` for test/lint/build/clean, `manage.py` for configuration) |
| `libexec/runner/` | The parallel runner and its docs |
| `libexec/codegen/` | DO NOT EDIT banners and the Dockerfile generator |
| `libexec/{metadata,onboarding,bootstrap,lint}/` | Metadata updates, onboarding, machine bootstrap, shellcheck |
| `tests/` | Unit and command tests against fixture repositories |
| [`docs/`](docs/) | Design, installers, adding a language, build conventions |
| [`assets/logo/`](assets/logo/) | The logo, favicons and social preview, and the script that builds them |
| `examples/sayable/` | Sayable's project hooks, CI workflow templates and config at the time of the split |

## Acknowledgements

- [Dave Sifry](https://github.com/dsifry), for
  [metaswarm](https://github.com/dsifry/metaswarm), his open-source framework
  for running teams of AI coding agents through a whole development
  lifecycle.
- The wordmark is set in [Space Grotesk](https://github.com/floriankarsten/space-grotesk)
  by Florian Karsten.

## License

[MIT](LICENSE.md)
