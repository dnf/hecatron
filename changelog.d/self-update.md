- **`hct self`:** hecatron's own commands take a space, as `rustup self`
  and `uv self` do: `hct self update`, `hct self test` and `hct self
  build`; `hct self` lists them. `self-test` and `self-build` still work.

- **`hct self update`:** updates an installation install.sh made to the
  latest release (or `--version X.Y.Z`), its binary checked against the
  release's `SHA256SUMS` and the new version swapped in whole, so a
  failure leaves the old one working; rewrites `env` and `env.fish` as
  install.sh does, so new PATH entries (the toolchains' links) take
  effect; refreshes the Agent Skill `hct skill install` put in place. `--check` only reports (exit 3: a newer release).
  Once a day at most, a release binary looks for a newer release in the
  background and says so in one line after a command, on a terminal in
  human output only (never in CI, agent or JSON output, or with
  `HECATRON_NO_UPDATE_CHECK=1`); `hct status` reports it from that check.
  Hecatron never updates itself. Needs the binary.
