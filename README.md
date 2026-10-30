<h1 align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/logo/hero-dark.svg">
    <img alt="hecatron" src="assets/logo/hero.svg" width="380">
  </picture>
</h1>

<p align="center"><strong>A universal adaptor for programming languages in the age of AI.</strong></p>

<div align="center">

| CI | Release | Languages | License |
| :-: | :-: | :-: | :-: |
| [![CI](https://github.com/dnf/hecatron/actions/workflows/ci.yml/badge.svg)](https://github.com/dnf/hecatron/actions/workflows/ci.yml) | [![Latest release](https://img.shields.io/github/v/release/dnf/hecatron?sort=semver)](https://github.com/dnf/hecatron/releases) | [![Languages](https://img.shields.io/badge/languages-54-3B5BFF)](docs/guide/coverage.md) <!-- language-count --> | [![License: MIT](https://img.shields.io/github/license/dnf/hecatron)](LICENSE.md) |

</div>

**Run any repository's tests, lints and builds without learning its
toolchain.** `cd` into a Go service, an Elixir library, an Android app or a
repository with fifty languages side by side, and `hct test` runs the
tests. Hecatron finds each language, installs its toolchain at the version
the project asks for, and calls the language's own tools. There's no setup
step and nothing to configure. It knows 54 languages. <!-- language-count -->

That matters to anyone who moves between repositories: the contributor
fixing something in a language they don't use every day, the CI setup
shared across many projects, and above all AI coding agents, which
otherwise spend their first minutes, and their tokens, working out how to
install things and run the tests. An agent learns hecatron once, from a
built-in skill, and then every repository works the same way.

The same commands scale to one library implemented in many languages:
`hct test` runs every language at once with a live display, and hecatron
writes and checks the CI workflows and Dockerfile that cover them. It grew
up running [sayable](https://github.com/dnf/sayable)'s 51 languages.

And it keeps getting better on its own: every two hours, CI builds another
real open-source app with hecatron and files an issue when it doesn't work
out of the box ([how](#tested-against-real-projects)).

## Try it

```bash
curl -fsSL https://raw.githubusercontent.com/dnf/hecatron/main/install.sh | sh

cd your-repo
hct status        # the languages it found, their toolchains, what's installed
hct doctor --fix  # install and repair every toolchain
hct test          # every language's tests, in parallel
```

It runs on the bash and Python 3 that macOS and Linux ship, installs into
`~/.hecatron`, and puts `hct` on your shell's PATH; `hct self update`
updates it and `hct self uninstall` removes it
([installing hecatron](docs/guide/installing-hecatron.md)). Nothing is
written to your repository: hecatron remembers what it detected inside
`.git/`, until `hct init` saves it as `.hecatron/config.json` to commit and
adjust. `hct help` lists every command.

## What it does

- **Toolchains that keep working.** `hct doctor` checks each language's
  toolchain actually works; `--fix` installs or repairs it (a moved
  virtualenv, a linter built with an older compiler). Each component gets
  the version it asks for in its ecosystem's own file (`.python-version`,
  `go.mod`, `rust-toolchain.toml`, `.nvmrc`, `.tool-versions`...), and an
  old checkout gets the toolchain of its day
  ([versions](docs/guide/versions.md)). Sixteen languages install as data
  the binary checks step by step, without root, ready for CI caches
  ([installing toolchains](docs/guide/installing-toolchains.md)).
- **One command per task, for every language.** `hct test`, `lint`,
  `build`, `clean`, `format`, `docs` and `bench`, with defaults chosen from
  the build files present (`Cargo.toml` → Cargo, `pom.xml` → Maven, an
  Xcode project → `xcodebuild` with the right scheme and simulator). A
  project overrides any of them with its own script
  ([hooks](docs/guide/hooks.md)).
- **Dependencies and supply chain.** `hct deps` fetches dependencies held
  to each lock file; `hct audit` runs each ecosystem's vulnerability
  auditor; `hct outdated` lists newer releases; `hct licenses` rolls up
  every dependency's license against a policy; `hct sbom` writes a
  CycloneDX or SPDX bill of materials
  ([audit](docs/guide/audit.md), [outdated](docs/guide/outdated.md),
  [licenses](docs/guide/licenses.md), [sbom](docs/guide/sbom.md)).
- **Each ecosystem's good practice.** `hct advisor` checks lock files,
  toolchain pins and end of life, linter and formatter configuration, and
  CI coverage, each finding with why and a link
  ([advisor](docs/guide/advisor.md)).
- **CI without the busywork.** `hct workflows` writes GitHub Actions
  workflows (or one GitLab pipeline) and `hct dockerfile` an image with
  every toolchain; `hct check` fails when they drift. Runs produce JUnit,
  SARIF and job summaries, and `--changed` runs only what a branch touched
  ([CI](docs/guide/ci.md)).

Hecatron calls each language's own tools (`cargo`, `go`, `gradle`, `uv`,
`xcodebuild`...); it has no build graph of its own and doesn't need to know
what your code does.

## Built for AI agents

- **No setup step.** In any git repository, hecatron detects the languages
  and layout and remembers them inside `.git/`; nothing is written to the
  working tree until you run `hct init`.
- **A skill to load.** The hecatron Agent Skill teaches an agent the whole
  interface without reading the docs: in Claude Code,
  `/plugin install hecatron --marketplace dnf/hecatron`, or `hct skill
  install` (`--project` puts it in the repository, for everyone who opens
  it); other agents that read Agent Skills take it with `hct skill install
  --dir <their skills folder>`; in claude.ai,
  upload a release's `hecatron-skill.zip` under Settings > Features.
- **Output sized for agents.** With `HECATRON_OUTPUT=agent`, a run prints
  one summary line, then only the failures with their log paths and key
  error lines, under a third of the human summary; exit status 3 means a
  toolchain is missing.
- **One reference to learn.** `hct help agents` prints [a compact
  reference](docs/agents.md) written for agents, matching the installed
  version; [llms.txt](llms.txt) points web-reading tools to it.
- **Instructions where agents look.** `hct init` adds a short section to
  `AGENTS.md` (and has an existing `CLAUDE.md` load it), so an agent knows
  how to build and test the repository from its first message.
- **Safe by default.** Commands that change the machine (`install`,
  `doctor --fix`) are separate from those that only run the project's
  tools, and the agent reference says to ask before the former.
- **Only what a change touched.** `--changed` runs only the languages and
  components a branch's changes touch, and `hct sync` catches up after a
  pull ([runner](docs/guide/runner.md#only-what-changed)).

## Tested against real projects

- **A self-improving loop.** Every two hours, a scheduled job takes the
  next open-source app from a list, clones it, and runs `hct init`,
  `hct deps` and `hct build` the way a newcomer would, on two tracks: Xcode
  apps on macOS and Android apps on Linux, a hundred apps between them.
  When anything goes wrong, or nothing builds, it files an issue with the
  versions, each step's result and the failing logs. Each one ends in a
  general fix to hecatron (the first batch taught it to find apps in
  subdirectories, build Catalyst apps for the simulator, and say when
  CocoaPods haven't been installed), or in a getting-started step the app's
  README asks for, recorded in a list of fixups. A label then puts the app
  back in line for a retry. No AI runs in the loop
  ([how it works](docs/contributing/testing.md#real-apps),
  [the issues](https://github.com/dnf/hecatron/issues?q=label%3Areal-app)).
- **Every language, every week,** on fresh Ubuntu, Debian and macOS
  machines (x86_64 and arm64): install, doctor and reinstall, then a smoke
  test that builds and runs a small program
  ([the matrix](docs/guide/coverage.md)).
- **Polyglot projects.** [sayable](https://github.com/dnf/sayable) runs
  all 51 of its languages through hecatron. On a fresh machine,
  [cucumber/tag-expressions](https://github.com/cucumber/tag-expressions)
  (9 languages) and
  [cucumber/cucumber-expressions](https://github.com/cucumber/cucumber-expressions)
  (8) pass every language's own test suite with `hct init`, `hct install`
  and `hct test`, and no configuration.

## The parallel runner

In a repository with many languages, `hct test` (and `lint`, `build`,
`audit`...) runs them all at once: a live row per language, swim lanes for
the jobs still running, the longest jobs started first from the last run's
times, and a log per language. A failure prints its error lines and the
command that reruns it.

<p align="center">
  <img alt="hct test running sayable's 51 languages in parallel: a live row per language with its time, swim lanes for the jobs still running, ending in 53/53 passed" src="assets/demo/hct-test.gif" width="720">
  <br><em><code>hct test</code> on <a href="https://github.com/dnf/sayable">sayable</a>: 51 languages and its own checks in parallel, 4½ minutes at 5× speed (<a href="docs/guide/runner.md">how the runner works</a>).</em>
</p>

## Why many languages in one repository

AI agents make writing and maintaining an implementation in another
language cheap, so one person can ship the same library natively in a
dozen or fifty languages, best kept in one repository where a fix lands
everywhere in one pull request
([more](docs/guide/why-one-repository.md)). What's left is the
infrastructure, a toolchain, commands, CI jobs and Dockerfile lines per
language, and with fifty languages keeping that working becomes the job.
Hecatron does that job.

## Hecatron or ad hoc?

For a library in ten languages ([full comparison](docs/guide/hecatron-vs-ad-hoc.md)):

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
a language hecatron doesn't know. [Configuration](docs/guide/configuration.md) describes
the full configuration.

To have teammates and CI run the same hecatron, pin it: `"hecatron": "0.7"`
takes any 0.7.x, `"hecatron": "0.7.0"` only that release (`">=0.7.0"` sets a
minimum instead). Another version warns on every `hct` command and fails
`hct check`; a generated GitLab pipeline installs the pinned one
(`HECATRON_VERSION=v0.7` makes install.sh install the series' latest).

## Languages

Hecatron ships definitions for 54 languages: <!-- language-count -->

Ada, Bash, C, C++, C#, Clojure, COBOL, Common Lisp, Crystal, D, Dart, Elixir,
Elm, Erlang, F#, Fortran, Gleam, Go, Goose, Groovy, Haskell, Haxe, Janet,
Java, JavaScript, Julia, Kotlin, Lua, Mojo, Nim, Objective-C, OCaml, Octave,
Odin, Pascal, Perl, Pharo, PHP, PowerShell, Prolog, Python, R, Racket, Raku,
Ruby, Rust, Scala, Swift, Tcl, TypeScript, V, VB.NET, Wren and Zig.

[docs/guide/languages.md](docs/guide/languages.md) has each one with a link to its home,
what it's for, and the build systems, test runners and linters hecatron uses
for it; it's also a good place to discover a language you haven't tried.
Adding another is a directory of small scripts; see
[adding a language](docs/contributing/adding-a-language.md).

## Status

Hecatron is young: version 0.7. It works and runs sayable's checks every
day, but expect changes before 1.0; [CHANGELOG.md](CHANGELOG.md) lists
them.

- **Platforms:** macOS (Apple Silicon and Intel) and Linux (Debian and
  Ubuntu, x86_64 and arm64). Releases ship a single Go binary for each,
  which runs the commands ported so far and the scripts for the rest.
- **Next:** native Windows
  ([design](docs/contributing/design.md#19-a-go-core-and-native-windows),
  [#151](https://github.com/dnf/hecatron/pull/151)), and more tracks for
  the real-projects loop.

## Documentation

- **[User guide](docs/guide/README.md):** using hecatron: the
  [command reference](docs/guide/commands.md),
  [project configuration](docs/guide/configuration.md),
  [hooks](docs/guide/hooks.md), [toolchain versions](docs/guide/versions.md),
  [installing toolchains](docs/guide/installing-toolchains.md),
  [the parallel runner](docs/guide/runner.md),
  [generated files](docs/guide/generated-files.md), and each report
  ([audit](docs/guide/audit.md), [outdated](docs/guide/outdated.md),
  [licenses](docs/guide/licenses.md), [format](docs/guide/format.md),
  [advisor](docs/guide/advisor.md)).
- **[Contributor guide](docs/contributing/README.md):** working on
  hecatron: [architecture](docs/contributing/architecture.md),
  [modules](docs/contributing/modules.md),
  [command flow](docs/contributing/command-flow.md),
  [the install engine](docs/contributing/install-engine.md),
  [testing](docs/contributing/testing.md),
  [releasing](docs/contributing/releasing.md),
  [adding a language](docs/contributing/adding-a-language.md), and the
  [design record](docs/contributing/design.md) with the decisions behind it.
- **[Reference for AI agents](docs/agents.md):** the compact reference
  `hct help agents` prints.

## Getting help

Questions, bugs and ideas go to
[GitHub issues](https://github.com/dnf/hecatron/issues). For a security
problem, see [SECURITY.md](SECURITY.md).

## Contributing

Issues and pull requests are welcome.

Especially for toolchains that are less well known.

ESPECIALLY if you are a maintainer of such a toolchain.

See [CONTRIBUTING.md](CONTRIBUTING.md)

## About the authors

Hecatron was created by Don Neufeld and Claude Code.

Don Neufeld was the CTO of [Medium](https://medium.com),
developer of several massively multiplayer online games,
VPE of multiple startups, and is currently an
[Executive Coach](https://vpecoach.com) to engineering leaders.

Don lives in the San Francisco Bay Area with his family,
several thousand bees, and a quantity of fruit trees
some might call excessive.

Claude Code is a good bot.

## Acknowledgements

- [Dave Sifry](https://github.com/dsifry), for
  [metaswarm](https://github.com/dsifry/metaswarm), his open-source framework
  for running teams of AI coding agents through a whole development
  lifecycle.
- The wordmark is set in [Space Grotesk](https://github.com/floriankarsten/space-grotesk)
  by Florian Karsten.

## License

[MIT](LICENSE.md)

## Origin of the Name and Logo

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
