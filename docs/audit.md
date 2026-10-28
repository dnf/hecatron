# Auditing dependencies

`hct audit` checks every component's dependencies for known
vulnerabilities, with each ecosystem's own auditor where it has one, and
fails when a finding is at or above a severity you choose.

```
hct audit                     # every language, in parallel
hct audit python go           # these languages, every component
hct audit --fail-on critical  # fail only on critical findings
```

Each component prints its findings, then one summary line:

```
  high      lodash 4.17.15  GHSA-35jh-r3h4-6jhm (CVE-2021-23337): Command Injection in lodash
  moderate  lodash 4.17.15  GHSA-29mw-wpgm-hmr9 (CVE-2020-28500): Regular Expression Denial of Service (ReDoS) in lodash
.: typescript 2 vulnerabilities (critical 0, high 1, moderate 1, low 0)
.: typescript: 1 at or above high
```

## What each language uses

| Language | Auditor | Reads |
| --- | --- | --- |
| TypeScript, JavaScript | the package manager's own: `npm audit`, `pnpm audit`, `yarn audit` (Yarn 1), `yarn npm audit` (Yarn 2+), `bun audit` | its lock file |
| Python | [pip-audit](https://github.com/pypa/pip-audit), run with `uvx` (nothing installed) | `uv.lock` (exported), `requirements*.txt`, else the project's metadata |
| Go | [govulncheck](https://go.dev/doc/security/vuln/), run with `go run` | `go.mod`; reports only vulnerabilities the code calls |
| Ruby | [bundler-audit](https://github.com/rubysec/bundler-audit) | `Gemfile.lock` |
| PHP | `composer audit` | `composer.lock` |
| C#, F#, VB.NET | `dotnet list package --vulnerable --include-transitive` | the restored packages |
| Rust, Java, Kotlin, Scala, Groovy, Dart, Elixir, Swift, R, Haskell, C, C++ | [OSV-Scanner](https://google.github.io/osv-scanner/) | `Cargo.lock`, `pom.xml`, `gradle.lockfile`, `pubspec.lock`, `mix.lock`, `Package.resolved`, `renv.lock`, `cabal.project.freeze`, `stack.yaml.lock`, `conan.lock`... |

Other languages have no auditor: `hct audit <lang>` says so and succeeds. A
component whose files the auditor can't read (a Rust crate without
`Cargo.lock`, a C library with no package manager) says "nothing to audit".

`hct install <lang>` installs bundler-audit (Ruby) and OSV-Scanner (the
languages above that use it), each at a pinned version; OSV-Scanner is its
release binary, checked against its published checksum. pip-audit and
govulncheck run at pinned versions without being installed. Auditors need
the network: they check online advisory databases.

**Without a lock file** (Node.js, Ruby, PHP, Python) the auditor resolves
the manifest's dependencies as they are today, in a scratch directory, and
says the result is approximate: what gets installed changes over time.

## Severity and failing

`--fail-on` takes `critical`, `high` (the default), `moderate` or `low`;
`"audit": {"failOn": "critical"}` in `.hecatron/config.json` sets a
project's default. pip-audit and govulncheck give no severity, so their
findings are "unrated" and count as high: govulncheck only reports code
you call, and pip-audit's findings are worth a look.

## Accepting a risk

List advisories you've decided to live with in `.hecatron/config.json`,
each with why and until when:

```json
"audit": {
  "ignore": [
    {"id": "GHSA-35jh-r3h4-6jhm", "reason": "lodash.template isn't called", "until": "2027-03-31"}
  ]
}
```

An accepted finding is still listed, marked as accepted, and doesn't fail
the audit. After its `until` date the entry stops applying and the audit
says it expired, so an accepted risk gets looked at again. The `id` matches
an advisory's ID or any alias (GHSA, CVE, RUSTSEC, GO, PYSEC).
`hct check` fails on an entry without a `reason` or an `until` date.

## The toolchain

For languages whose definition lists release lines (Python, Node.js, Go,
Rust, Java, Ruby, PHP, .NET), the audit also warns when a component's
installed toolchain is past its end of life. It's a warning: upgrading a
toolchain is a project decision, not a dependency bump.

## In CI

New advisories appear while the code stands still, so run it on a schedule
as well as on pull requests:

```yaml
on:
  pull_request:
  schedule:
    - cron: "0 6 * * *"
jobs:
  audit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: curl -fsSL https://raw.githubusercontent.com/dnf/hecatron/main/install.sh | sh
      - run: ~/.hecatron/bin/hct install && ~/.hecatron/bin/hct audit
```

`HECATRON_AUDIT_REPORT_DIR=<dir>` saves each auditor's raw JSON report
there, for a CI artifact or another tool.

With the binary, `HECATRON_REPORT_DIR=<dir>` writes the findings as SARIF
2.1.0 (`audit.sarif`, or `audit-<lang>.sarif` for `hct audit <lang>`), and
`--output sarif` prints it instead of the normal output. Each advisory is a
rule (its ID, title and link, tagged `security` with a
`security-severity`: critical 9.5, high and unrated 8.0, moderate 5.5,
low 2.0), and each affected package a result at the lock file or manifest
the auditor read (OSV-Scanner names it per package), on the first line
naming the package; levels are `error` for critical, high and unrated,
`warning` for moderate, `note` for low. An accepted risk is a suppressed
result with its reason and date. GitHub's code scanning shows them on the
pull request:

```yaml
      - run: ~/.hecatron/bin/hct audit
        env:
          HECATRON_REPORT_DIR: hecatron-reports
      - uses: github/codeql-action/upload-sarif@v3
        if: always()
        with:
          sarif_file: hecatron-reports/audit.sarif
```

In GitHub Actions (`GITHUB_STEP_SUMMARY` set) the job summary gets a table
of the findings. See [runner.md](runner.md#reports) for the other reports.

Not yet: `--fix` (it changes lock files), secret scanning, static analysis.
