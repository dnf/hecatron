- **Secret scanning:** `hct audit` looks for committed credentials with
  gitleaks, which `hct install secrets` installs (8.30.1, pinned and
  checked against its checksum, into `~/.local/share/hecatron/bin`, no
  root). `hct audit secrets` runs only the scan, of the working tree less
  what git ignores, or of every commit with `--history`. Findings show
  the rule, file, line and gitleaks' fingerprint, never the secret, in
  every output and in the SARIF report and job summary, and fail the
  audit. A project's `.gitleaks.toml` and `.gitleaksignore` apply, and
  `"audit": {"secrets": {"ignore": [...]}}` leaves out paths such as test
  fixtures. `"ci": {"secrets": true}` generates a CI job that runs it
  (#191).
