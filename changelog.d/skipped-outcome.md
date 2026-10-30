- **Skipped is a third outcome for every run command, not a pass** (#236):
  a hook with nothing to run or that can't run (no tests, no linter or
  docs tool configured, no benchmarks, nothing to build or clean, nothing
  to audit) calls `skip "reason"` from `languages/_hooks.sh`, which now
  marks the component skipped (`HECATRON_SKIPPED`, the file `hct audit`'s
  nothing-to-audit uses) instead of passed. A language without a hook for
  the command (`hct docs go`, `hct lint` of a language whose build is its
  lint) is skipped too: `go: docs skipped: no docs tool for go`. Every
  default hook that already said "(skipping)" now reports skipped, in
  human output (`- go ... skipped`, `3/3 passed, 1 skipped`, then why),
  agent output (`SKIP <lang> (<dirs>): <reason>`), JSON (`"status":
  "skipped"`, `"reason"`), JUnit (`<skipped message=...>`), the job summary
  and `hct status` (`test: skipped 2h ago`). A component skips only when
  its hook does, lint only when its format check skips too, and a hook run
  in several directories below (`run_hook_in`) only when each of them
  skips. Skipped jobs are recorded in `history.log` with status 77, so a
  skipped run neither sets a job's time limit nor runs it first. Writing
  project hooks: [hooks](docs/guide/hooks.md#skipping).
- **`--fail-on-skip`** (or `HECATRON_FAIL_ON_SKIP=1`) on every run
  command: exit 1 when nothing ran, every job skipped, not installed or
  disabled, for CI that must have tested something
  ([output](docs/guide/output.md#skipped)).
