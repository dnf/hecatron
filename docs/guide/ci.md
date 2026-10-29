# Hecatron in CI

Hecatron runs the same everywhere; what differs between GitHub, GitLab,
Bitbucket and Azure DevOps is where CI is configured and where results are
shown. It never needs a forge's token: it doesn't post comments or call
forge APIs, so it works the same in a fork, on a laptop and in CI. What's
planned is in the [design record](../contributing/design.md#20-source-control-backends).

- **GitHub Actions workflows.** `hct workflows` writes one workflow per
  language from the project's templates ([generated files](generated-files.md)), and `hct check` fails
  when one is behind.
- **GitHub annotations.** With the Go binary, `--output ci` (chosen
  automatically when `GITHUB_ACTIONS` is set) adds `::error` annotations at
  each error line's file and line, shown on the pull request ([output](output.md)).
- **GitLab CI and caching (#102).** `hct workflows` writes
  `.gitlab-ci.yml` (`generated.ci: "gitlab"` or `--ci gitlab`), and with
  `"ci": {"cache": true}` caches toolchains and dependencies with
  `actions/cache` and `cache:key:files` ([generated files](generated-files.md)); `hct check` fails when
  the pipeline is behind. Bitbucket and Azure on demand.
- **Dependency update configuration (#103).** `hct workflows` writes
  `.github/dependabot.yml` and `renovate.json` ([generated files](generated-files.md)), and `hct check`
  fails when a language or component was added without regenerating them.
- **Audit in CI.** `hct audit` fails at a severity threshold and can
  save each tool's raw report (`HECATRON_AUDIT_REPORT_DIR`);
  [audit.md](audit.md) has a workflow step.
- **Reports every CI reads (#100; `internal/reports`).** With the binary,
  `HECATRON_REPORT_DIR` gets JUnit XML from test, lint and build (a suite
  per language and component; the tools' own test cases merged where they
  write them: Surefire, Failsafe, Gradle and sbt's `TEST-*.xml` found
  under the component, `go test -json` through `hecatron _gotest`,
  pytest's `--junitxml`, and anything a hook leaves in
  `HECATRON_TEST_REPORTS`), SARIF 2.1.0 from lint (located error lines)
  and audit (a rule per advisory, a result per package at its lock file,
  accepted risks suppressed), and GitLab Code Quality JSON from lint;
  `--output junit` and `--output sarif` print one instead of the normal
  output. The generated GitLab pipeline hands JUnit and Code Quality to
  the merge request widgets ([runner.md](runner.md#reports)).
- **Job summary (#101).** With `GITHUB_STEP_SUMMARY` set, every run
  command appends a table of languages and components with status and
  time, then each failure's error lines, log and rerun command; audit,
  licenses and advisor add their findings. The bash runner doesn't.
