# Releasing

## Changelog fragments

Each change adds its changelog entry as its own file,
`changelog.d/<branch>.md`, not by editing `CHANGELOG.md`, so parallel pull
requests don't conflict ([changelog.d/README.md](../../changelog.d/README.md)). A
change users can't see adds none and is labelled `no changelog`. The
[Changelog workflow](../../.github/workflows/changelog.yml) requires one or
the other.

## Making a release

From an up-to-date `main`:

```bash
libexec/maintenance/release.sh 0.4.0 --dry-run   # the checks, the changes and the notes
libexec/maintenance/release.sh 0.4.0
```

It checks the tree and that there are changes to release, and runs `hct self test`, then bumps
`VERSION`, moves the fragments into a dated changelog section and updates the README's status line
in a pull request, merges it when CI passes, tags `v0.4.0` and publishes a
GitHub release with the changelog section as its notes. The installer
installs the newest tag. Before 1.0, a release that changes behaviour bumps
the minor version.

In detail ([`release.sh`](../../libexec/maintenance/release.sh)):

1. **Checks:** on `main`, clean and up to date with origin; `gh` signed in;
   the version is X.Y.Z, newer than `VERSION`, and not tagged yet; there are
   changes to release (`changelog.d/` fragments, or entries under
   `CHANGELOG.md`'s "## Unreleased"); `hct self test` passes.
2. **On a `release-X.Y.Z` branch:** `VERSION`; the Unreleased heading
   becomes "## X.Y.Z (date)" under a new empty Unreleased, with the
   fragments added in the order they were merged and removed; the README's
   "version X.Y" status line.
3. **A pull request**, merged (squash) once its CI passes.
4. **The tag** `vX.Y.Z` on the merge commit, and a GitHub release whose
   notes are the changelog section.

`HECATRON_RELEASE_TRAILERS`, if set, is appended to the commit message. It
stops at the first failure; rerunning picks up where it can (an existing
branch or pull request is reused).

## What a published release triggers

The [release workflow](../../.github/workflows/release.yml) builds the Go
binary for macOS, Linux and Windows on x86_64 and arm64 from the tag,
smoke-tests the Linux one, packages the Agent Skill as `hecatron-skill.zip`,
and attaches them to the release with `SHA256SUMS`. `install.sh` at a
release downloads the binary for its machine and checks it against
`SHA256SUMS`; `hct self update` does the same
([installing hecatron](../guide/installing-hecatron.md)). It can be run by
hand (`workflow_dispatch`) for a release already published.

## Scheduled maintenance

| Workflow | What it does |
| --- | --- |
| [Weekly updates](../../.github/workflows/release-dates.yml) | Reruns `libexec/maintenance/release_dates.py` and opens a pull request with the new release dates; checks every external link in the docs (`check_links.py`) and keeps one issue open listing the broken ones |
| [Coverage](../../.github/workflows/coverage.yml) | Every language's install test on every platform; writes [coverage.md](../guide/coverage.md) |
