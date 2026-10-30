# Dependency licenses

`hct licenses` lists the license of every dependency of every component, in
every language, rolled up per license, so a project can see what it ships
under, and, with a policy, enforce it.

```
hct licenses                   # every language
hct licenses python go         # these languages, every component
hct licenses -c api            # one component
hct licenses --output json     # every package as JSON (also --json)
hct licenses --timeout 10m     # stop after ten minutes (also HECATRON_TIMEOUT)
```

It needs hecatron's binary (which `install.sh` downloads with a release),
[OSV-Scanner](https://google.github.io/osv-scanner/) (`hct install` of any
language it reads, or whose audit uses it, installs it) and the network.

## What it reports

```
hct licenses: 44 packages, 12 licenses (11 identifiers), 7 unknown; copyleft: 1 strong, 3 weak, 1 unclassified; policy: 1 violation

  license                       copyleft      packages  languages                      components
  MIT                           none          16        php, python, ruby, typescript  api, web
  Apache-2.0                    none          5         csharp, go, java, python       api, worker
  BSD-3-Clause OR GPL-2.0-only  none          1         python                         api
  ...

Per license identifier (a package under AND or OR counts for each):
  identifier     copyleft      packages  languages
  MIT            none          19        csharp, php, python, ruby, rust, typescript
  GPL-2.0-only   strong        1         python
  ...

Unknown licenses (no metadata, or not an SPDX expression: worth a look):
  pypi:pycrypto  2.6.1  declared: non-standard  python, api  direct

The project's own packages (not counted):
  pypi:api  0.27.0.post18.dev0+g1a2b3c4  python, api

Not covered:
  dart: deps.dev, where OSV-Scanner finds licenses, has none for pub.dev

Packages:
  package           version  license    dependency   language, component
  npm:chalk         4.1.2    MIT        direct       typescript, web
  npm:is-number     7.0.0    MIT        direct, dev  typescript, web
  ...

Policy (dev and test dependencies not counted): 1 violation
  pypi:pyqt5  5.15.10  GPL-3.0-only  GPL-3.0-only denied  python, api
```

- **Per license**, as an [SPDX](https://spdx.org/licenses/) expression
  (`MIT`, `Apache-2.0 OR MIT`, `GPL-2.0-only WITH Classpath-exception-2.0`):
  how many packages use it, in which languages and components, and its
  [copyleft](#copyleft). Identifiers are spelled as the SPDX list spells
  them, and the deprecated GNU ones made explicit (`GPL-3.0` is
  `GPL-3.0-only`, `LGPL-2.1+` is `LGPL-2.1-or-later`).
- **Per license identifier**, when that differs: a package under
  `Apache-2.0 AND LGPL-3.0-or-later` counts for both, so how many packages
  name a GPL or LGPL license is one row each.
- **Copyleft counts** in the summary line: packages whose license, as a
  whole, is strong, weak, network or maximal copyleft, and those whose
  class isn't known (unclassified).
- **Per package**: `ecosystem:name`, version, license, direct or transitive
  (where the lock file says), dev or test, and the language and component
  that use it.
- **Unknown** licenses separately: no metadata, a placeholder
  (`non-standard`, `UNKNOWN`), or not an SPDX expression. They need a
  person's look; hecatron never guesses.
- **The project's own packages** separately, not counted: see
  [below](#the-projects-own-packages).
- **Not covered**: languages it can't read licenses for, and components
  without a lock file it reads. A lock file outside every component
  directory of its language (a root `package-lock.json` in a monorepo
  whose components are its subdirectories) isn't read, and a warning
  names it: `hct list` shows where the components are.
- `--output human|agent|json`, chosen as for `hct test`: agent output is
  the summary, then violations, unknowns, what isn't covered and warnings,
  one a line, then the own packages and the licenses per expression and
  per identifier, each with its class when it isn't `none`; JSON has
  everything (below).

### JSON

`hct licenses --json` is one document:

- `packages`: each dependency, once per language and component, with
  `ecosystem`, `name`, `version`, `license` (the canonical expression, `""`
  when unknown), `licenseIds` (its license identifiers, each once: `[]`
  when unknown), `copyleft` (the expression's [class](#copyleft), or
  `unknown`), `declared` (what the registry or lock file says),
  `dependency` (`direct`, `transitive` or `""`), `dev`, `language`,
  `component` and `source` (the lock file).
- `own`: the project's own packages, as `packages` has them.
- `licenses`: per expression, and `licenseIds`: per identifier; each with
  `license`, `copyleft`, `packages` (each name and version once),
  `languages` and `components`.
- `copyleft`: how many packages (each name and version once) are in each
  class: `none`, `weak`, `strong`, `network`, `maximal`, `unclassified`,
  `unknown`.
- `notCovered` (`language`, `component`, `reason`), `warnings`, and, with a
  policy, `policy` (`allow`, `deny`, `includeDev`, `violations`, `accepted`,
  `expired`).

## Copyleft

Each license identifier has a class, from
[Blue Oak Council](https://blueoakcouncil.org/)'s
[copyleft list](https://blueoakcouncil.org/copyleft) and its
[permissive list](https://blueoakcouncil.org/list), vendored in
`internal/spdx/copyleft.txt` (the SPDX list itself doesn't classify):

| Class | Means | Such as |
| --- | --- | --- |
| `none` | permissive | MIT, Apache-2.0, BSD-3-Clause, ISC, Zlib, PSF-2.0 |
| `weak` | the licensed files stay under it | LGPL, MPL, EPL, CDDL, CeCILL |
| `strong` | the whole work | GPL, Sleepycat |
| `network` | the whole work, offered over a network too | AGPL, SSPL, EUPL, OSL |
| `maximal` | beyond the work | RPL, CAL, Parity |
| `unclassified` | on neither list | CC-BY-4.0, a `LicenseRef-` |

An expression's class is its licenses' taken together: `OR` is a choice,
so the least restrictive option counts (`BSD-3-Clause OR GPL-2.0-only` is
`none`); `AND` applies each part, so the most restrictive does
(`Apache-2.0 AND LGPL-3.0-or-later` is `weak`). An unclassified part makes
the whole unclassified unless the rest decides it anyway (an `OR` with a
permissive option, an `AND` with a maximal part). A license `WITH` an
exception has the license's class (`GPL-2.0-only WITH
Classpath-exception-2.0` is `strong`), as the policy judges it; whether the
exception lifts the copyleft for your use is a person's call.

## The project's own packages

Lock files list the project itself and its workspace members, which have
no license on deps.dev (they aren't published, or not at that version):
uv's editable or virtual root (`version = "0.27.0.post18.dev0+g1a2b3c4"`)
and its workspace members, npm's workspaces, Cargo's workspace members, a
`Gemfile.lock`'s `PATH` gems (a gemspec's own), Go's requirements that a
`replace` points at a directory, Composer's path repositories, NuGet's
project references, Poetry's directory dependencies; and any package a
component's own manifest names (`package.json` with its workspaces,
`pyproject.toml`, `Cargo.toml`, `go.mod`'s module, `composer.json`). They
are listed apart as the project's own, out of the dependencies, the
counts and the policy.

## Where the licenses come from

| Language | Lock files | Licenses from |
| --- | --- | --- |
| TypeScript, JavaScript | `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, `bun.lock` | deps.dev (through OSV-Scanner) |
| Python | `uv.lock`, `poetry.lock`, `Pipfile.lock`, `requirements*.txt` | deps.dev |
| Go | `go.mod` | deps.dev |
| Rust | `Cargo.lock` | deps.dev |
| Java, Kotlin, Scala, Groovy, Clojure | `pom.xml`, `gradle.lockfile` | deps.dev |
| C#, F#, VB.NET | `packages.lock.json` | deps.dev |
| Ruby | `Gemfile.lock` | deps.dev |
| PHP | `composer.lock` | `composer.lock` itself |

OSV-Scanner (`osv-scanner scan source --recursive --all-packages --licenses`)
reads each component's lock files and asks [deps.dev](https://deps.dev) for
each package's license. deps.dev has no licenses for pub.dev, Hex, Swift
packages, CRAN, Hackage or Conan, so Dart, Elixir, Erlang, Gleam, Swift, R,
Haskell, C and C++ are reported as not covered, as are languages without a
package registry. A lock file in a component's subdirectory belongs to the
deepest component directory holding it.

**Direct or transitive** comes from `package.json` (npm), `go.mod`'s
`// indirect`, `Cargo.lock`'s own crates, `composer.json`, `Gemfile.lock`'s
`DEPENDENCIES` and `packages.lock.json`; for `pom.xml` and
`requirements*.txt`, what the file lists is direct and what OSV-Scanner
resolved from it transitive. Elsewhere it's left blank. **Dev and test**
dependencies are those the lock file puts in a dev or test group.

**Without a lock file** (`pom.xml`, unpinned requirements), OSV-Scanner
resolves the dependencies as they are today, and hecatron warns that the
result is approximate.

A package that declares several licenses (RubyGems', npm's and Composer's
lists) is read as a choice between them: `Ruby OR BSD-2-Clause`.

**Many PyPI packages are unknown** (`non-standard`): deps.dev already
reads a package's `License-Expression` (PEP 639) and a classifier that
names one license (`License :: OSI Approved :: MIT License` is `MIT`), so
what's left is what neither names: a classifier for a family
(`License :: OSI Approved :: BSD License`, which doesn't say which BSD;
`Apache Software License` without a version), or free text in `License`
(`Dual License`, a copyright line). Those stay unknown rather than
guessed; an `ignore` entry accepts one once a person has looked.

## A policy

```json
"licenses": {
  "allow": ["MIT", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause", "ISC"],
  "deny": ["GPL-3.0-only", "AGPL-3.0-only"],
  "ignore": [{"package": "npm:caniuse-lite", "reason": "CC-BY-4.0 data, reviewed", "until": "2027-06-01"}],
  "includeDev": false
}
```

With a `"licenses"` section in `.hecatron/config.json`, `hct licenses`
fails (exit 1) when a package's license isn't permitted:

- a denied license fails; with `allow`, so does any license not listed, and
  an unknown one;
- an expression passes when one choice of an `OR` passes, and an `AND` when
  each part does; `WITH` an exception is judged by the license;
- dev and test dependencies don't count unless `"includeDev": true`;
- an `ignore` entry accepts a package (`ecosystem:name`, or
  `ecosystem:name@version` for one version; ecosystems are `npm`, `pypi`,
  `go`, `crates.io`, `maven`, `nuget`, `rubygems`, `packagist`) until its
  `until` date. The package is still listed, as accepted; after the date it
  is checked again and the output says the entry expired.

The policy is by identifier; it doesn't read the [copyleft](#copyleft)
class yet (denying every strong copyleft license would be one entry
instead of a list): `deny` lists them.

`hct check` checks the section: identifiers on the SPDX list, each
`ignore` entry with a `package`, a `reason` and an `until` date. (Only the
binary reads the section; `bin/hecatron check` without it doesn't.)

## Exit status

0: no policy, or nothing it refuses. 1: a package the policy refuses, or
OSV-Scanner failed, or it ran past `--timeout` (or `HECATRON_TIMEOUT`:
`90s`, `30m`, `1h30m`, a bare number of seconds; `0` for no limit, the
default, as it keeps no history of its runs to derive one from, as
`hct audit` does). 2: a usage error. 3: OSV-Scanner isn't installed and a
language needs it.

The same data makes a bill of materials: [`hct sbom`](sbom.md). Not yet:
a third-party notices file; licenses for the ecosystems deps.dev doesn't
cover.
