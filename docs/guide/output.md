# Output

The runner renders each finished job (label, component, exit code,
duration, log path, error excerpt) in one of four formats:

- **human** (default on a terminal): today's live display and summary;
- **agent**: one summary line, then only the failures, each with its log
  path and up to ten lines matching the language's error patterns (else the
  last fifteen, without colour codes or download noise), and a `Rerun:`
  line; one line when everything passes;
- **json**, for scripts: the run's `status`, `exitCode`, `summary`
  (`passed`, `failed`, `timedOut`, `skipped`) and each job's `label`,
  `status`, `exitCode`, `duration`, `log`, `components` and, when it
  failed, its error excerpt (`errors`); `hct audit` adds every finding as
  a `vulnerabilities` record and the secret scan's as `secrets`
  ([audit](audit.md#in-ci));
- **CI**: human output plus GitHub annotations for each error line.

`--output` chooses, else `HECATRON_OUTPUT`, else agent output when stdout
isn't a terminal and an agent marker is set (`CLAUDECODE=1`, and others
once verified), else CI when `GITHUB_ACTIONS` is set, else human. Not a
config setting: the format belongs to whoever is calling, not the project.
Exit codes: 0 passed, 1 failed, 2 usage, 3 a toolchain missing or broken.

A job is **skipped** when every component's hook had nothing to do (an
audit with no lock file its auditor reads): status `skipped` with a
`reason` in JSON, `SKIP <label> (<dirs>): <reason>` in agent output, `-`
and `skipped` in human output, counted apart from passed; it doesn't fail
the run. Not installed and disabled languages are counted as skipped too.

The run commands also take `--output junit` and `--output sarif`, which
print that report in place of the normal output. All of this needs
hecatron's binary; the scripts alone print human output.

How the parallel runner schedules and displays jobs is in
[runner.md](runner.md); the reports CI systems read (JUnit, SARIF, the job
summary) are in [runner.md](runner.md#reports) and [ci.md](ci.md).
