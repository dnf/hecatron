# Output

The runner renders each finished job (label, component, exit code,
duration, log path, error excerpt) in one of four formats:

- **human** (default on a terminal): today's live display and summary;
- **agent**: one summary line, then only the failures, each with its log
  path and up to ten lines matching the language's error patterns (else the
  last fifteen, without colour codes or download noise), and a `Rerun:`
  line; one line when everything passes;
- **json**, for scripts;
- **CI**: human output plus GitHub annotations for each error line.

`--output` chooses, else `HECATRON_OUTPUT`, else agent output when stdout
isn't a terminal and an agent marker is set (`CLAUDECODE=1`, and others
once verified), else CI when `GITHUB_ACTIONS` is set, else human. Not a
config setting: the format belongs to whoever is calling, not the project.
Exit codes: 0 passed, 1 failed, 2 usage, 3 a toolchain missing or broken.

The run commands also take `--output junit` and `--output sarif`, which
print that report in place of the normal output. All of this needs
hecatron's binary; the scripts alone print human output.

How the parallel runner schedules and displays jobs is in
[runner.md](runner.md); the reports CI systems read (JUnit, SARIF, the job
summary) are in [runner.md](runner.md#reports) and [ci.md](ci.md).
