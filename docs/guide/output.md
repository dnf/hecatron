# Output

The runner renders each finished job (label, component, exit code,
duration, log path, error excerpt) in one of four formats:

- **human** (default on a terminal): today's live display and summary;
- **agent**: one summary line, then only the failures, each with its log
  path and up to ten lines matching the language's error patterns (else the
  last fifteen, without colour codes or download noise), and a `Rerun:`
  line; one line when everything passes;
- **json**, for scripts: the run's `command`, `status` (`passed`,
  `failed` or `skipped`), `exitCode`, `duration`, `summary` (`passed`,
  `failed`, `timedOut`, `skipped`), `logs`, `rerun` and, with
  `--changed`, what it compared (`changed`); and each job's `label`,
  `kind` (`language`, `task` or `tool`), `language`, `status` (`passed`,
  `failed`, `timed out`, `skipped`, `not installed` or `disabled`),
  `reason`, `exitCode`, `duration`, `log`, `timeLimit`, `components`,
  `docs` (`hct docs`) and, when it failed, its error excerpt (`errors`);
  `hct audit` adds every finding as
  a `vulnerabilities` record and the secret scan's as `secrets`
  ([audit](audit.md#in-ci));
- **CI**: human output plus GitHub annotations for each error line.

`--output` chooses, else `HECATRON_OUTPUT`, else agent output when stdout
isn't a terminal and an agent marker is set (`CLAUDECODE=1`, and others
once verified), else CI when `GITHUB_ACTIONS` is set, else human. Not a
config setting: the format belongs to whoever is calling, not the project.
Exit codes: 0 passed (or skipped), 1 failed (or, with `--fail-on-skip`,
nothing ran), 2 usage, 3 a toolchain missing or broken.

## Skipped

A run command has three outcomes for a job, not two: passed, failed, and
**skipped**, when it had nothing to run or couldn't run, so a run that
tested nothing doesn't read as one that tested and passed. A component is
skipped when its hook says so (`skip "reason"`: no tests, no linter or
docs tool configured, nothing to audit; [hooks](hooks.md#skipping)); a
language job is skipped when every component was, or when the language
has no hook for the command (`hct lint` of a language whose build is its
lint, `hct docs` of one with no docs tool). A job some of whose components
skipped and the rest passed has passed, naming the skipped ones.

- **Human:** `- go ... skipped` in the chart, `3/3 passed, 1 skipped` in
  the tally, then the line each skipped job's log ends with, which says
  why: `go: docs skipped: no docs tool for go`, `js: test skipped for 1
  component(s): web (package.json has no test script)`.
- **Agent:** `skipped` in the summary line, then `SKIP <label> (<dirs>):
  <reason>` for each job with a skipped component.
- **JSON:** the job's `status` is `skipped`, with its `reason` (also on a
  passed job some of whose components skipped); each skipped component has
  `"skipped": "<reason>"`; the run's `status` is `skipped` when nothing
  passed or failed.
- **JUnit:** the component's test case is `<skipped message="skipped:
  <reason>"/>`; the job summary's table says `skipped: <reason>`; audit's
  SARIF adds a note.
- **`hct status`:** the last run is `skipped`, not passed.
- **Counts:** never as passed. The agent and JSON `skipped` count also
  holds languages not installed or disabled.

A skip doesn't fail the run. `--fail-on-skip` (or `HECATRON_FAIL_ON_SKIP=1`)
makes a run in which nothing ran, every job skipped, not installed or
disabled, exit 1 (`hecatron: nothing ran: every job skipped
(--fail-on-skip)`), for CI that must have tested something; one job that
passed or failed is enough. With `rebuild` and `retest` it applies to the
build or test, not the clean. A command with no jobs at all ("Nothing to
test.") still exits 0. It's a flag and a variable, not a config setting,
as `--timeout` is: what a run must achieve is the caller's (a CI job's).

The run commands also take `--output junit` and `--output sarif`, which
print that report in place of the normal output. All of this needs
hecatron's binary; the scripts alone print human output.

How the parallel runner schedules and displays jobs is in
[runner.md](runner.md); the reports CI systems read (JUnit, SARIF, the job
summary) are in [runner.md](runner.md#reports) and [ci.md](ci.md).
