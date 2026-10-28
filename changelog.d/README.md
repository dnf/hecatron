# Changelog fragments

Each pull request records its change here as its own file, instead of
editing `CHANGELOG.md`, so parallel pull requests never conflict over the
changelog. `libexec/maintenance/release.sh` adds every fragment to
`CHANGELOG.md` under the new version, in the order they were merged, and
removes them.

- **Name:** the branch, e.g. `changelog.d/self-update.md`.
- **Content:** one or more entries, written as they'll read in `CHANGELOG.md`:

  ```markdown
  - **`hct self update`:** downloads the latest release, checks it against
    the release's SHA256SUMS and replaces the installation (#172).
  ```

- **No user-visible change** (tests, CI, refactoring): add no fragment, and
  label the pull request `no changelog`. The `changelog` check requires one
  or the other.
