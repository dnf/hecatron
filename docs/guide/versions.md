# Toolchain versions

Status: **accepted**; implemented for Python, Go, Rust, Node.js, Java,
Ruby, PHP, .NET, Elixir and Zig (a `"managed"` release download, [installs as data](../contributing/install-engine.md)); Gleam and Erlang (its OTP, on Ubuntu) install the one
version their definition pins. Other installers install one
version per language, some the latest at install time (Swift, Julia,
PowerShell, Odin, Crystal).

A project can need an older or newer version of a language than the
newest one, and two projects on the same machine can need different ones. So
a language's version is data: hecatron has a default, and a project can
choose another.

## Choosing a version

For a language in a component, the version is the first of:

1. **The ecosystem's own pin file**, nearest first from the language's
   implementation directory (`implPath`) up to the project root, as the
   ecosystems' own tools search, read with that ecosystem's meaning:
   `.python-version`, `.nvmrc` or `.node-version`, `rust-toolchain.toml` or
   `rust-toolchain`, `global.json` (`sdk.version`), the `toolchain` line of
   `go.mod`, `.ruby-version`, Gradle's `gradle-daemon-jvm.properties`
   (`toolchainVersion`, the JDK), `build.zig.zon`'s
   `minimum_zig_version`... Each definition lists the files it reads
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
file wins and `hct check` reports the disagreement: two sources for one
version are almost always a mistake.

## Release dates and what can be installed

The dates come from tables shipped with each definition
(`languages/<lang>/releases.json`: version, date, and whether it was an LTS
release), so resolution stays offline and two machines agree.
`libexec/maintenance/release_dates.py` writes them from each language's
release source: nodejs.org, python.org, go.dev's release history, Rust's
`RELEASES.md`, Adoptium, ruby-lang.org, php.net, Microsoft's .NET release
metadata, Elixir's GitHub releases and ziglang.org's index of downloads. A weekly workflow (`.github/workflows/release-dates.yml`) reruns
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

## Reading pin files

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
`8.0` and `latestMinor` as `8`, the newest release of which always qualifies;
`disable` is exact, and `patch`, the default, keeps the version as written,
since its scope, a feature band (8.0.1xx), isn't a prefix. `latestMajor` is a
floor and nothing more, as .NET treats it (decided): the default when it is
that version or later, else the newest release of the floor's major, raised
to the newest major the component's `floorFiles` need. Those are a
definition's files read in the component's directory and up to three levels
below it (not build output): for .NET, the projects' `TargetFramework(s)`, so
a `global.json` asking for 8.0.100 or later in a repository whose projects
target `net10.0` gets SDK 10. In
`composer.json`, `config.platform.php` (the PHP Composer resolves for) is a
pin on its line (`8.4.0` reads as `8.4`); the `require` constraint on `php`
isn't, because it is the range a library supports, which the default
normally satisfies. PHP's doctor checks instead that each component's PHP
satisfies what its lock file requires. (Owner's decision pending; this is
the implementation's.) `build.zig.zon`'s `minimum_zig_version` reads the
other way, as a pin on its line (`0.14.0` reads as `0.14`): Zig breaks
compatibility between minor releases, so code written for 0.14 needs the
newest 0.14.x, not a newer line.

## Version syntax

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

A component on the default can also say which versions it runs on, as a
range rather than a version: a definition's `rangeFiles`, read like pin
files (nearest first, up to the root; a file whose entry has nothing to say,
such as a `package.json` without `engines.node`, is passed over). JavaScript
and TypeScript read `package.json`'s `engines.node` (format `npm-engines`,
npm's range syntax: `>=22`, `^20.1 || ^22`, `20 - 22`, `22.x`). When none of
the default's line is installed, the newest installed version in the range
runs, and the doctor counts it as installed and says so ("clients/web:
typescript 26.13.0 runs: clients/web/package.json allows >=22, and the
default 24.22.1 isn't installed"), so a machine with Node.js 26 needs no 24
for a package asking for 22 or later. A range isn't a pin: what installers
and the Dockerfile install, and what runs where the default's line is
installed, stay the default, even when the range excludes it (holding a
component back to the newest line its range allows is
`supportFiles`' job, which `engines` doesn't do yet). A pin file, the
project's `version`, a supported line or an old checkout's date wins over
a range. (The implementation's decision, #257; the owner may revise it.)

## Defaults and how they are updated

Each definition's `config.json` records its default `version` (exact) and its
`track`: `"lts"` where the language has long-term support releases (Java,
Node.js, .NET) and `"stable"` (the newest release) otherwise. Installers
never resolve "latest" at install time, so two machines installing on
different days get the same toolchain and every change is a reviewable diff.

A default changes only with a hecatron release, so a project gets new
default toolchains when it updates hecatron, never by surprise. (Updating
the defaults automatically each week is planned: see the
[design record](../contributing/design.md#18-toolchain-versions).)

## Installing and using a version

- **Side by side.** Installers add the resolved version next to any others,
  through the ecosystem's version manager where it has one (uv, rustup,
  ghcup, .NET's side-by-side SDKs, `openjdk-N` or Temurin, Go's toolchain
  downloads, a `"managed"` release download's homes under hecatron's
  toolchains directory), and never replace a version another project may
  use.
- **What's installed.** A definition's optional `installed.sh` prints one
  `version<TAB>home` line per installed version, offline (each ecosystem
  already keeps a list: `rustup toolchain list`, `dotnet --list-sdks`, uv's,
  ghcup's, the JDKs under `/usr/lib/jvm`). The core runs it with the
  language's static `path` entries on PATH and stdin closed, and treats a
  failure, or a run over 30 seconds, as nothing installed. A component's
  installed version is the newest line its spec matches.
- **Per-language environment.** The `path` mechanism ([configuration](configuration.md#resolving-a-languages-files)) becomes a
  per-language environment. `path` entries and the new `env` variables may
  name `{home}`, `{version}`, `{major}` and `{minor}` of the installed
  version (`"path": ["{home}/bin"]`, `"env": {"JAVA_HOME": "{home}"}`), and
  are applied wherever `path` was. It is per component, since components can
  resolve to different versions: `each-component.sh` applies the
  component's environment before running its hook, and
  `hct env LANG [-c COMPONENT]` prints it as shell exports for a
  project's own recipes. With no installed version matching (or no
  `installed.sh`), entries and variables naming one are left out, static
  entries still apply, and the doctor reports it. The project's other
  added, enabled languages' static `path` entries follow the language's own
  (in the project's language order, each directory once): a build in one
  language often runs another's tool (a Ruby gem's Rakefile or a dune rule
  calling `cargo`), installed where only that language's environment looks.
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
  `# end-each-version` lines, which `hct dockerfile` repeats for the
  definition's default and then each other spec in component order, filling
  in `{version}` (the spec without `=`), `{exact}` (`1` for an exact spec,
  else empty), `{major}` and `{minor}`. `# each-version java` repeats over
  another language's versions (Kotlin's snippet installs Java's JDKs).
  Generation stays offline: a block given a prefix resolves it at build time,
  installs where `installed.sh` looks, and skips a version already there.
  `ENV` lines stay outside the block and name the default; a component's own
  version comes from `hct env` at run time. A workflow template names a language's resolved version with
  a placeholder (`{{version:java}}`, or `{{version:java:age}}` for one
  component), which `hct workflows` fills in, and `hct check`
  fails when a generated file is behind its versions.

## JDKs a build needs besides its own

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

## .NET runtimes a component runs on

A .NET SDK builds for every framework, but runs tests and apps only on a
runtime it has. The C# installer installs, and the doctor asks for, the
runtime of each `netX.0` framework a project that runs targets: a test
project (`Microsoft.NET.Test.Sdk`, `<IsTestProject>`), an executable
(`<OutputType>Exe</OutputType>` or `WinExe`) or a web or worker app (their
SDKs), with its frameworks from the project file or, failing that, the
nearest `Directory.Build.props` that sets them. A class library's
frameworks need only their targeting packs, which the SDK restores, so a
library multi-targeting `net8.0;netstandard2.1` whose tests target
`net10.0` needs no .NET 8 runtime.
