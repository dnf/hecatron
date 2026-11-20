# Pruning build output

```
hct prune [lang...] [--dry-run] [--older-than DURATION] [--max-size SIZE] [--caches]
```

`hct clean` removes a language's build output; the next build starts
cold. `hct prune` removes only the stale part, the builds nothing has used
lately, and keeps what the next build reuses. It's for the directories that
only grow: a cargo target directory CI reuses keeps every build of every
dependency version, feature set and toolchain it ever made (one reached
114 GiB, 69 of it old builds), and a machine's DerivedData keeps the builds
of clones deleted long ago.

```bash
hct prune --dry-run          # what each language would free, in bytes and items
hct prune rust               # trim Rust's target directories
hct prune --caches           # the per-user caches too (NuGet, uv, npm...)
hct prune rust --max-size 40G   # then the oldest builds, until under 40 GiB
```

Like the other [run commands](commands.md#run-commands), it runs each
language's hook, `prune.sh`, once for every component, every language in
parallel when none is named. A language without one is skipped, saying so.
A project replaces a language's prune with its own
`.hecatron/languages/<lang>/prune.sh` ([hooks](hooks.md)).

## Flags

| Flag | Meaning |
| --- | --- |
| `--dry-run` | Remove nothing: say what each directory would free, in bytes and items, and the largest items |
| `--older-than DURATION` | What counts as stale: `90s`, `30m`, `6h`, `7d`, `2w`, `1h30m`. Each language has its own default and its own idea of age (below) |
| `--max-size SIZE` | After the stale items, remove the oldest ones until each directory is under `SIZE` (`500M`, `50G`, `1.5T`; powers of 1024), never the newest build of anything |
| `--caches` | Also the per-user caches each language's tools keep outside the project |

Each directory ends with one line, which `hct prune` repeats at the end
with the total, and which agent and JSON output carry (`"pruned"`: each
directory's `where`, `bytes`, `items` and `dryRun`):

```
rust: prune: would free 69.2 GiB (74301234567 bytes) in 10412 items from target
```

## What each language prunes

| Language | In the project | With `--caches` |
| --- | --- | --- |
| Rust | The target directory (`CARGO_TARGET_DIR` and `build.target-dir` respected, as cargo finds it): each package and target's builds made more than `--older-than` (6h) before its newest build, and rustc's incremental caches as old | Nothing: cargo 1.88 and later trim `~/.cargo`'s registry and git caches themselves |
| Zig | `.zig-cache/` (and `zig-cache/`), whole, once no build has used it for `--older-than` (7d) | Nothing: the global cache holds the packages' sources |
| Swift | SwiftPM's `.build/<triple>/<configuration>/` unused for `--older-than` (7d); the fetched dependencies stay | On macOS, Xcode's DerivedData folders whose project or workspace is gone (with `--older-than`, also those unused that long) |
| C#, F#, Visual Basic | Each project's `bin/<configuration>/` and `obj/<configuration>/` unused for `--older-than` (7d), never the configuration built last; the restore's files stay | `~/.nuget/packages` (or `NUGET_PACKAGES`): versions no restore has used for `--older-than` (30d), never a package's highest or last-used version |
| Java, Kotlin, Groovy, Scala | A Gradle build's project caches (`.gradle/<version>/`) for Gradle versions it no longer uses, unused for `--older-than` (7d) | Nothing: Gradle trims `~/.gradle` itself (it says how to set its retention) |
| Python | Nothing (skipped): the virtualenv is the installed dependencies | `uv cache prune` |
| JavaScript, TypeScript | Nothing (skipped): `node_modules` is the installed dependencies | `npm cache verify`, and `pnpm store prune` in a pnpm project |
| Go | Nothing (skipped): the go command trims its build cache itself, removing entries unused for 5 days | |

Every other language has no prune hook and is skipped.

### Rust

Cargo never deletes a build. Each time a unit's metadata hash changes (a
`Cargo.lock` bump, another feature set, a new toolchain), the new build
lands beside the old ones. A unit is everything named with one hash in a
profile directory (`debug`, `release`, `<triple>/release`):
`.fingerprint/<pkg>-<hash>/`, the `deps/`, `examples/` and `build/` entries
named `<name>-<hash>`, and in cargo 1.100's layout `build/<pkg>/<hash>/`.
Units are grouped by what they build, from their fingerprint (the package
and target), and each group keeps the builds made within `--older-than` of
its newest: one run builds several variants of a package side by side,
minutes apart, while an old hash is hours or days older. A unit goes
fingerprint first, so a prune cut short leaves a unit cargo rebuilds, never
one it trusts half removed.

### Zig

A Zig cache can't be trimmed entry by entry: a manifest names its output
only through a hash, a cache hit touches neither, and a build whose
manifest's output is gone fails (`FileNotFound`) instead of rebuilding. So
a cache goes whole, judged by `h/timestamp`, which every build touches.

## Safety

- **Never a directory a build is using.** Cargo's profile directories are
  skipped while cargo holds their `.cargo-lock`, SwiftPM's `.build/` while
  it holds `.build/.lock`, a Zig cache while a build holds a lock on a
  manifest; prune holds those locks while it removes, so a build that starts
  meanwhile waits. Nothing written in the last 10 minutes is removed.
- **Age is the modification time**, never the access time, which `noatime`
  and `relatime` mounts don't keep.
- **Never tracked files, never installed dependencies' sources.** Anything
  git tracks stays; so do `node_modules`, virtualenvs, SwiftPM's
  checkouts, Gradle's and the module caches the toolchains manage
  themselves. Prune never follows a symlink.
- **A removed build rebuilds.** Everything prune removes is something the
  tool makes again when it's needed, at the cost of that rebuild.

## In CI

A CI runner that keeps a target directory between runs can prune it before
each build, so it stays near the size of the builds still in use:

```bash
hct prune rust --max-size 40G
```
