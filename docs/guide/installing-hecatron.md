# Installing hecatron

Hecatron runs on the bash and Python 3 that current macOS and Linux ship
(bash 3.2 or later, Python 3.9 or later). Install it, or update it to the
latest release:

```bash
curl -fsSL https://raw.githubusercontent.com/dnf/hecatron/main/install.sh | sh
```

This installs anything hecatron itself needs that's missing (the Command
Line Tools on macOS; git, Python 3 and perl on Linux), then the latest
release into `~/.hecatron` with its Go binary for the machine (checked
against its checksum), and adds a line sourcing `~/.hecatron/env` to your
shell's startup files (bash, zsh and fish). Run it again to update. From a
clone instead, source the installer, which also sets up the shell you're in:

```bash
git clone https://github.com/dnf/hecatron ~/.hecatron && . ~/.hecatron/install.sh
```

| Variable | Effect |
| --- | --- |
| `HECATRON_VERSION` | Install another release (`v0.6.1`), a series' latest (`v0.6`), or `main` |
| `HECATRON_INSTALL_DIR` | Install somewhere other than `~/.hecatron` |
| `HECATRON_NO_MODIFY_PATH=1` | Leave the startup files alone |
| `HECATRON_NO_GO=1` | At run time: keep to the bash and Python scripts, not the binary |

## Updating and removing

**Updates: notify and self update, never auto-update (decided
2026-10-28).** Hecatron generates CI workflows and a Dockerfile, so a
silent version change would change a project's generated files under it.
So it only says a release is out and updates when asked, as `gh` does:

- `hct self update` (`internal/selfupdate`; binary only) updates an
  installation install.sh made (a clone at a release tag, with
  `bin/.hecatron-go` at that commit and `env`) to the latest release, or
  `--version X.Y.Z`. It finds the release through GitHub's releases API
  (`GITHUB_TOKEN` when set; `HECATRON_UPDATE_URL` replaces the API
  address, for tests), downloads the machine's binary and `SHA256SUMS`
  and refuses a mismatch or a short download, fetches the tag into the
  installation's clone, clones that beside the installation with the
  binary and the installation's untracked files (the cache, the skill
  record), writes `env` and `env.fish` as the release's install.sh would
  (a Go port of its writer, held to install.sh's output byte for byte by
  a parity test, and the new binary's own `hct _write-env` when it has
  one), so new PATH entries reach shells, and swaps the two directories
  by renaming, undoing the first rename if the second fails. It refuses a dev build (`hct
  self build`'s or `go build`'s), a checkout on a branch or with changes
  ("update with git pull"), a directory install.sh didn't make, and one it
  can't write. It then refreshes the skill copies `hct skill install`
  made (`--user`, and each `--dir`, recorded in `.skill-installs`).
  `--check` only reports: exit 0 up to date, 3 a newer release, 1 the
  check failed.
- **`hct self uninstall`** (binary only) removes an installation
  install.sh made, its lines in the startup files (each line ending
  `  # hecatron`, as install.sh replaces them), fish's conf.d file and the
  skill copies, lines first, so a failure never leaves a shell sourcing a
  file that's gone. Run from a checkout it keeps the checkout. It refuses
  an installation with changes to its own files. Toolchains stay: they're
  the projects', and `hct uninstall <language>` knows what each installed;
  `--all` adds the data directory (unpacked toolchains, receipts, links)
  and the download cache. It asks first, and without a terminal needs
  `--yes`.
- **The notice.** A release binary (not a dev build) reads
  `.update-check.json` in the installation (the latest version and when it
  was checked) before each command. Once it is 24 hours old, it writes the
  time and starts `hct _update-check` on its own, which asks the API with a
  10-second timeout and caches the answer, so no command waits on the
  network. A newer release than this one is said in one line on stderr
  after the command (before a delegated one), only with stderr a terminal
  and human output: never in CI (`CI` set), with `--output`, `--json` or
  `HECATRON_OUTPUT` naming another format, in an agent's session, in
  GitHub Actions, or with `HECATRON_NO_UPDATE_CHECK=1`, which also stops
  the check. `hct status` reports the cached newer release in every format
  (`update` in JSON) and suggests `hct self update`, asking the user
  first; it never checks itself.

## Pinning a project's hecatron

**A project pins its hecatron (decided 2026-10-28).** Teammates and CI
should run the hecatron a project was set up with, as Go's `toolchain`
line, Gradle's wrapper and mise do; auto-update was ruled out above. So
config.json's `hecatron` takes a version as well as a minimum: `"0.7"`
pins a series (any 0.7.x, matched on dot boundaries, so not 0.70.0),
`"0.7.0"` a release, and `">=0.7.0"` stays a minimum. A version older than
the pin is refused as a minimum is, by both implementations; the rest is
the binary's only (`internal/project/pin.go`):

- **A warning.** Every command that finds the project says once on stderr
  `hecatron: warning: this project pins hecatron 0.6, but this is 0.7.0
  (hct self update --version 0.6.N installs one that matches)`, but not
  `hct help`, `--version`, `self`, `version` or the hidden commands; the
  hct commands it runs (tasks, hooks) don't repeat it
  (`HECATRON_PIN_WARNED`).
- **`hct check` fails** on the mismatch, so CI catches drift, and on a
  `hecatron` that is neither a version nor a minimum. A dev build (`hct self
  build`'s, `bin/.hecatron-go.commit` saying `dev`) only warns: its VERSION
  is the last release's whatever the checkout holds, so the pin can't tell
  whether it matches.
- **Generated CI installs the pin.** The GitLab pipeline's
  `HECATRON_VERSION` is the pinned version (else the generating one, as
  before), and toolchain cache keys carry it. A release is installed by its
  own install.sh; a series (`v0.6`) by main's, which resolves it to the
  series' latest tag. The Dockerfile doesn't install hecatron, and GitHub
  workflows are the project's own templates, so neither changes.
- **Later:** hct fetching and running the pinned version itself, as Go
  does with `GOTOOLCHAIN`, from `hct self update`'s download and checksum
  code.
