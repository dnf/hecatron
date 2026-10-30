- **`hct audit --output json` lists every finding:** a top-level
  `vulnerabilities` array, one record per finding (component, language,
  package, installed version, fixed versions, advisory ID and aliases,
  severity, title, link, auditor, lock file), complete rather than the
  log's last lines in `jobs[].errors`; the secret scan's are `secrets`.
  Fixed versions come from each auditor (pip-audit, govulncheck,
  OSV-Scanner, bundler-audit) or the advisory's patched or vulnerable range
  (npm, pnpm, Yarn, Bun, Composer); SARIF results name them too. `npm
  audit`'s findings show the installed version, read from
  `package-lock.json`.
- **Nothing to audit is skipped, not passed:** a component without a lock
  file its auditor reads is shown as skipped, with why and how to write
  one (for Gradle, `gradle dependencies --write-locks`), in human, agent
  and JSON output (`"status": "skipped"`, `"reason"`), JUnit (`<skipped>`),
  SARIF (a note) and the job summary. It doesn't fail the audit, but it
  isn't counted as passed.
- **OSV-Scanner audits only the language's own lock files:** Swift reads
  `Package.resolved`, Rust `Cargo.lock`, the JVM languages `pom.xml` and
  Gradle's lock files, and so on, each named with `--lockfile`, instead of
  a recursive scan that reported every ecosystem's packages (a root
  `package-lock.json`'s npm findings in Swift's audit) once per language.
  Dependencies, build output and nested components of the same language
  are left out.
