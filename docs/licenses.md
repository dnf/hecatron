# Dependency licenses

`hct licenses` lists the license of every dependency of every component, in
every language, rolled up per license, so a project can see what it ships
under, and, with a policy, enforce it.

```
hct licenses                   # every language
hct licenses python go         # these languages, every component
hct licenses -c api            # one component
hct licenses --output json     # every package as JSON (also --json)
```

It needs hecatron's binary (which `install.sh` downloads with a release),
[OSV-Scanner](https://google.github.io/osv-scanner/) (`hct install rust`
installs it, as do the other languages whose audit uses it) and the network.

## What it reports

```
hct licenses: 44 packages, 12 licenses, 7 unknown; policy: 1 violation

  license                packages  languages                    components
  MIT                    16        php, python, ruby, typescript  api, web
  Apache-2.0             5         csharp, go, java, python       api, worker
  ...

Unknown licenses (no metadata, or not an SPDX expression: worth a look):
  pypi:pycrypto  2.6.1  declared: non-standard  python, api  direct

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
  how many packages use it, in which languages and components. Identifiers
  are spelled as the SPDX list spells them, and the deprecated GNU ones
  made explicit (`GPL-3.0` is `GPL-3.0-only`, `LGPL-2.1+` is
  `LGPL-2.1-or-later`).
- **Per package**: `ecosystem:name`, version, license, direct or transitive
  (where the lock file says), dev or test, and the language and component
  that use it.
- **Unknown** licenses separately: no metadata, a placeholder
  (`non-standard`, `UNKNOWN`), or not an SPDX expression. They need a
  person's look; hecatron never guesses.
- **Not covered**: languages it can't read licenses for, and components
  without a lock file it reads.
- `--output human|agent|json`, chosen as for `hct test`: agent output is
  the summary, then violations, unknowns, what isn't covered and warnings,
  one a line; JSON has everything.

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

`hct check` checks the section: identifiers on the SPDX list, each
`ignore` entry with a `package`, a `reason` and an `until` date. (Only the
binary reads the section; `bin/hecatron check` without it doesn't.)

## Exit status

0: no policy, or nothing it refuses. 1: a package the policy refuses, or
OSV-Scanner failed. 2: a usage error. 3: OSV-Scanner isn't installed and a
language needs it.

Not yet: an SBOM (CycloneDX or SPDX) or a third-party notices file from the
same data; licenses for the ecosystems deps.dev doesn't cover.
