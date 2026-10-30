# Software bill of materials

`hct sbom` writes a software bill of materials (SBOM) for the whole
project: every component, in every language, and the packages each one's
lock files resolve to, with their licenses.

```
hct sbom                          # CycloneDX 1.6 JSON on stdout
hct sbom --format spdx -o sbom.spdx.json
hct sbom python go                # these languages, every component
hct sbom -c api --no-dev          # one component, without dev and test dependencies
SOURCE_DATE_EPOCH=$(git log -1 --format=%ct) hct sbom -o bom.cdx.json   # reproducible
```

It reads what [`hct licenses`](licenses.md) reads, the same way:
OSV-Scanner's report on each component's lock files and deps.dev's
licenses. So it needs hecatron's binary,
[OSV-Scanner](https://google.github.io/osv-scanner/) (`hct install` of a
language it reads installs it) and the network. `--timeout DURATION`
(else `HECATRON_TIMEOUT`) stops it after `DURATION`, as for `hct licenses`.
Exit status: 0 written, 1 OSV-Scanner failed or timed out, or the file
couldn't be written, 2 a usage error, 3 OSV-Scanner isn't installed.

## What's in it

- **Formats:** CycloneDX 1.6 JSON (`--format cyclonedx`, the default) or
  SPDX 2.3 JSON (`--format spdx`), on stdout or into `-o FILE`.
- **The project:** its name, the repository's commit (when it's in git),
  hecatron and its version as the tool, a timestamp, and a serial number
  (CycloneDX) or document namespace (SPDX) derived from the content.
  `SOURCE_DATE_EPOCH` sets the timestamp, so the same commit gives the same
  document byte for byte.
- **Each component** of the project, with its languages.
- **Each package** once (the project's own packages, which its lock files
  list too, aren't: [hct licenses](licenses.md#the-projects-own-packages)), with its [purl](https://github.com/package-url/purl-spec)
  (`pkg:npm/lodash@4.17.21`) and its license as an SPDX expression, where
  it's known (the rules are [hct licenses'](licenses.md#where-the-licenses-come-from)).
  An unknown license is left out (CycloneDX) or `NOASSERTION` (SPDX), never
  guessed; what the registry declared is kept beside it.
- **Dependencies:** which component uses which package, and whether
  directly or transitively where the lock file says.
- **Dev and test dependencies** (in every component that uses them) are
  marked; `--no-dev` leaves them out.

Packages from registries deps.dev has no licenses for (pub.dev, Hex,
Swift packages, CRAN, Hackage, Conan) are listed without licenses.
Languages without a package registry, and components with no lock file
OSV-Scanner reads, aren't in it; what's left out is said on stderr.

## CycloneDX

The project is `metadata.component`; each component of it is an
`application` (`bom-ref` `component:NAME`), and each package a `library`
whose `bom-ref` is its purl. The dependency graph has the project depending
on its components and each component on its direct dependencies. Which
package a transitive dependency comes through isn't in the lock files
hecatron reads, so the graph is declared `incomplete` (`compositions`).
Dev and test dependencies have `scope: optional`, the rest `required`.
Properties say the rest: `hecatron:dependency` (`direct` or `transitive`),
`hecatron:component`, `hecatron:language`, `hecatron:source` (the lock
file), `hecatron:declaredLicense` (when it isn't an SPDX expression),
`hecatron:commit` on the project.

## SPDX

The document `DESCRIBES` the project, which `CONTAINS` each component.
Each component `DEPENDS_ON` every package it uses, the relationship's
comment saying `direct` or `transitive` where known; a dev or test
dependency is the component's `DEV_DEPENDENCY_OF` instead. Packages carry
their purl as a `PACKAGE-MANAGER` external reference, `licenseDeclared`
(the SPDX expression, or `NOASSERTION`) and `licenseConcluded:
NOASSERTION` (hecatron reads metadata, not the code). A license naming a
`LicenseRef-` is put in `licenseComments`, as the document doesn't carry
its text. The project's commit is its `sourceInfo`.
