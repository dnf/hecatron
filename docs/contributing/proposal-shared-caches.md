# Proposal: shared caches (`hct cache`)

Status: **proposal**, for the owner's decisions (below). Nothing is built.
Researched and measured 2026-11-20.

## The problem

A repository worked on by many agents at once has many checkouts: one git
worktree per agent, plus CI's own. Each new worktree starts cold. It
installs every language's dependencies again (`npm ci`, `dotnet restore`,
`uv sync`, Gradle's resolution) and compiles everything again (cargo's
`target/`, SwiftPM's `.build/`, `obj/`, `.zig-cache/`). The owner's old
trick for Node was to pre-populate `node_modules` with hard links into a
shared directory before `npm install` ran. The question is what each
ecosystem offers now, what is safe, and what hct should do about it.

The Corriva repository is the motivating case. On the Mac, its agents'
worktrees are on an external APFS volume (`/Volumes/Data/corriva-worktrees/`).
On legion1, its remote CI copies are on an ext4 disk (`/data/agent-ci`),
which can't reflink. It already shares Rust compiles through sccache. It
also learned the hard way that a cargo target dir shared between checkouts
builds stale or wrong output (Corriva #2142, #2159: cargo bakes
`CARGO_MANIFEST_DIR` into binaries and judges freshness by mtime). Its
fix was a fixed-path slot: each slot copies the sources to one path and
keeps that path's target dir.

## What sharing means: three layers

| Layer | Example | Safe to share? |
| --- | --- | --- |
| **Downloads** | npm's `_cacache`, `~/.cargo/registry`, `GOMODCACHE`, `~/.nuget/packages`, `~/.gradle/caches/modules-2`, SwiftPM's `repositories/` | Yes, everywhere. Keyed by content or name and version, immutable, and the tools lock them. Usually already per-user. The only thing to get right is putting them on the right disk |
| **Installed trees** | `node_modules`, a virtualenv's `site-packages` | Yes, when the tool links them from its own store (pnpm, bun, Yarn, uv). The hazard is a hard-linked file edited in place, which edits the store and every other checkout |
| **Compiled outputs** | `target/`, `.build/`, `obj/`, `.zig-cache/`, Gradle task outputs | Only through a cache that hashes inputs (sccache, ccache, Gradle's build cache, `GOCACHE`, Bazel). Copying or sharing the output directory itself fails, either at once or quietly, because absolute paths are baked in |

The rule that falls out: **hct shares stores and caches, and lets each
tool link from them. It never shares a build directory between paths.**

## Survey

Checked on 2026-11-20. Sources are linked. Numbers are as reported, not
re-measured, except under [measurements](#measurements-on-the-mac).

### JavaScript and TypeScript

- **pnpm.** pnpm keeps a content-addressable store, one per disk
  ([store-dir](https://pnpm.io/settings/store)).
  - **How it links.** `packageImportMethod: auto` clones on macOS and
    hard-links first on Linux. It also takes `hardlink`, `clone`,
    `clone-or-copy` and `copy`
    ([settings](https://pnpm.io/settings/node-modules)).
  - **Disk rule.** The store must be on the same disk as the project, or
    pnpm copies.
  - **Hazard: hard links.** "the file in `node_modules` *is* the file in the
    store", so an edit reaches every project. `verifyStoreIntegrity` re-checks
    modified files.
  - **Side-effects cache.** `sideEffectsCache` (default on) also stores
    install scripts' output.
  - **Speed.** pnpm's benchmark (2026-11-17, pnpm 12.21) installs a ~1 GB
    project in 14.4 s clean, 2.6 s with cache and lockfile, and 16 ms when
    everything is warm. npm takes 1m44s, 20.5 s and 2.8 s
    ([benchmarks](https://pnpm.io/benchmarks)).
- **Yarn Berry.** `enableGlobalCache` (default on) keeps zips in
  `~/.yarn/berry`.
  - **PnP** (the default `nodeLinker`) runs packages from those zips with no
    `node_modules`, so worktrees share them with no copy at all.
  - **Zero-installs** commits `.yarn/cache` into the repository: every
    worktree checks it out, which is the opposite of what a large repository
    wants ([caching](https://yarnpkg.com/features/caching)).
- **bun.** bun keeps a global cache in `~/.bun/install/cache`. It installs
  with `clonefile` on macOS and `hardlink` on Linux
  ([install](https://bun.com/docs/pm/cli/install)).
  - **Measured by others.** One project measured a worktree
    ([DashFrame #419](https://github.com/youhaowei/DashFrame/pull/419),
    2026-09):

    | Install | Time | Disk |
    | --- | --- | --- |
    | Cold cache | ~30 s | 1362 MiB |
    | Warm cache | ~10 s | 313 MiB |
    | `cp -c` of `node_modules`, then `bun install --frozen-lockfile` | ~1 s | ~43 MiB |
- **npm.** `_cacache` shares downloads only: every install extracts a full
  copy.
  - **Hazard: `npm ci` deletes a warm tree.** `npm ci` removes an existing
    `node_modules` first
    ([npm ci](https://docs.npmjs.com/cli/v11/commands/npm-ci)), so a seeded
    `node_modules` must be followed by `npm install`, not `npm ci`.

### Rust

- **sccache.** Shares compiles of library crates through a content-keyed
  cache ([Rust notes](https://github.com/mozilla/sccache/blob/main/docs/Rust.md)).
  - **What it never caches.** Linked crates (`bin`, `dylib`, `cdylib`,
    `proc-macro`), and incremental builds by default.
  - **Paths.** `SCCACHE_BASEDIRS` strips checkout paths from the key. A crate
    that embeds `CARGO_MANIFEST_DIR` can then hit another checkout's entry
    unless `SCCACHE_BASEDIRS_VERBATIM_ENV_DEPS` names it.
  - **Speed.** A primed cache cut 70 s to 14 s on native Linux
    ([NeoSmart, 2024-07](https://neosmart.net/blog/benchmarking-rust-compilation-speedups-and-slowdowns-from-sccache-and-zthreads/)).
    Corriva's `scripts/dev/sccache/sccache` wrapper drops the `CARGO_*`
    variables that name the worktree for the same reason.
- **Cargo itself.**
  - **`build.build-dir`.** Stable since 1.91 (2025-10-30). It splits
    intermediate output from final output
    ([changelog](https://doc.rust-lang.org/cargo/CHANGELOG.html)).
  - **The new layout.** Cargo 1.100 (2026-11-12) made the per-unit layout
    (`build/<pkg>/<hash>/`) the default, which "unblocks ... cross-workspace
    caching" ([call for testing](https://blog.rust-lang.org/2026/03/13/call-for-testing-build-dir-layout-v2)).
  - **Sharing a build-dir is not endorsed.** The Cargo team "does not
    officially endorse sharing a build-dir across workspaces". The 2026
    project goal [Cargo cross workspace cache](https://goals.rust-lang.org/2026/cargo-cross-workspace-cache.html)
    starts nightly-only, with plain crates (no build scripts or proc macros).
    Stabilising is "likely not" within the goal period.
  - **Why it's unsafe.** [cargo#12516](https://github.com/rust-lang/cargo/issues/12516)
    (open since 2023) is exactly Corriva's bug. Path dependencies are keyed
    by their workspace-relative path, so worktrees of one repository collide
    in a shared target dir, and mtimes decide freshness.
- **cargo-chef.** It caches dependency builds as a Docker layer, and needs
    the same path for cook and build. It is only for container builds
    ([cargo-chef](https://github.com/LukeMathWalker/cargo-chef)).
- **ccache (C, C++).** `base_dir` rewrites absolute paths so different
  directories share entries. `hash_dir` (default on) keeps debug builds
  apart. `file_clone` reflinks results, and `hard_link` can corrupt the cache
  when an output is modified
  ([manual 4.14](https://ccache.dev/manual/latest.html)).

### Python

- **uv** links from its cache with `link-mode` (`clone`, `hardlink`, `copy`,
  `symlink`). It clones by default on macOS and, since 0.10.5 (2026-02), on
  Linux, falling back to a hard link and then a copy, so on ext4 it
  hard-links ([settings](https://docs.astral.sh/uv/reference/settings/)).
  - **Disk rule.** A cache on another filesystem means full copies, with a
    warning ([cache](https://docs.astral.sh/uv/concepts/cache/)).
  - **Concurrency.** Concurrent uv commands are safe.
  - **Hazard.** `symlink` mode breaks environments on `uv cache clean`.
  - **Venvs.** A virtualenv embeds its absolute path (`pyvenv.cfg`, script
    shebangs), so it is recreated per worktree, never copied.
- **pip.** The wheel and HTTP cache shares downloads and built wheels. Every
  install is a copy.

### Go

- **Module and build caches.** `GOMODCACHE` files are read-only, which
  protects the store. `GOCACHE` is "safe for concurrent invocations"
  ([cmd/go](https://pkg.go.dev/cmd/go)). Both are per-user already.
- **Path hazard.** Without `-trimpath`, a package's directory is in its cache
  key ([exec.go](https://github.com/golang/go/blob/master/src/cmd/go/internal/work/exec.go)).
  So the main module's packages miss across worktrees and their
  dependencies hit, which is the safe direction.
- **`GOCACHEPROG`.** Stable since [Go 1.24](https://go.dev/doc/go1.24). It
  plugs in an external cache program, such as a remote cache.

### JVM (Gradle)

- **Build cache.** Gradle's build cache (local in `GRADLE_USER_HOME`, plus an
  HTTP remote) shares task outputs across checkouts for `@CacheableTask`s.
  - **Opt-in.** It needs `org.gradle.caching=true`.
  - **Path hazard.** Inputs are keyed by absolute path unless a task
    declares `@PathSensitive(RELATIVE)`
    ([build cache](https://docs.gradle.org/current/userguide/build_cache.html)).
- **Configuration cache.** Lives in each build's `.gradle/` and is local
  only ([configuration cache](https://docs.gradle.org/current/userguide/configuration_cache.html)).
- **Dependency cache.** A shared `GRADLE_USER_HOME` is safe between processes
  on one machine, which coordinate by file locks.
- **Read-only cache.** `GRADLE_RO_DEP_CACHE` serves a seeded read-only copy
  of `modules-2`, for ephemeral CI machines
  ([dependency caching](https://docs.gradle.org/current/userguide/dependency_caching.html)).

### .NET (NuGet)

- **Global packages.** `~/.nuget/packages` (`NUGET_PACKAGES`) is the
  extracted store, used in place by every PackageReference project, so it
  is already shared.
- **Locks.** Processes coordinate through a temp directory they must all
  share ([global packages](https://learn.microsoft.com/en-us/nuget/consume-packages/managing-the-global-packages-and-cache-folders)).
- **`obj/` can't be copied.** `obj/project.assets.json` holds absolute
  `projectPath`, `outputPath` and `packageFolders` (checked in Corriva's
  `clients/dotnet`), so a copied `obj/` must be restored again. With a warm
  global folder, restore is quick anyway.

### Swift

- **Shared cache.** SwiftPM shares `~/Library/Caches/org.swift.swiftpm`
  (`repositories/`, `artifacts/`, `manifests/`, `prebuilts/`): `--cache-path`
  sets it and `--enable-dependency-cache` is on by default (Swift 6.4.2's
  `swift build --help`).
- **`.build` is path-bound.** It embeds absolute paths, including clang's
  module cache path ([SR-7718](https://bugs.swift.org/browse/SR-7718)).
  This document's [measurements](#measurements-on-the-mac) show what that
  does to a copied `.build`.
- **Xcode 26 compilation caching.** `COMPILATION_CACHE_ENABLE_CACHING` keeps
  a local LLVM CAS in DerivedData. Apple calls it local-only. Hits across
  directories need `SWIFT_ENABLE_PREFIX_MAPPING`, and a user saw ~80% reuse
  from another directory against 100% in place
  ([forums](https://forums.swift.org/t/about-swift-shared-cache-across-machines/81850)).

### Zig

- **Caches.** The global cache (`~/.cache/zig`, `ZIG_GLOBAL_CACHE_DIR`) holds
  fetched packages and is shared already. The local `.zig-cache` can be
  pointed elsewhere (`ZIG_LOCAL_CACHE_DIR`), but Zig documents no guarantee
  for sharing it ([ziggit](https://ziggit.dev/t/moving-zig-local-cache-dir-is-it-safe/8713)).

### The reference point: Bazel and Nix

- **Bazel.** `--disk_cache` and the remote cache key every action by its
  inputs and command line, which is the design the ecosystems above
  approximate ([remote caching](https://bazel.build/remote/caching)).
- **Even Bazel gets poisoned.** It is poisoned by tools outside the
  workspace and by files edited mid-build
  (`--experimental_guard_against_concurrent_changes`), and it can't evict
  selectively.
- **Nix.** The store is input-addressed. Content-addressed derivations are
  still experimental (Nix 2.36).
- **Lesson for hct.** It isn't going to be Bazel. It can make sure each
  tool's own content-keyed cache is on, on the right disk and bounded.

### Copying a warm worktree

- **Tools that clone into a new worktree.** Several do (cowtree, parhel,
  HazAT/ultimate #3). They use `cp -c` (APFS `clonefile`) or
  `cp --reflink=always`.
- **Cost.** It's paid per directory entry, not per byte. A 750k-file
  `node_modules` was still too slow to clone, and that author moved to a
  pool of six pre-installed worktrees
  ([2026-03](https://daveschumaker.net/use-git-worktrees-they-said-itll-be-fun-they-said/)).
- **APFS.** `cp -c` falls back to a byte copy, silently, across volumes.
- **Linux.** coreutils 9's `cp` defaults to `--reflink=auto`, which on ext4
  silently copies. Only btrfs and XFS (`reflink=1`) clone
  ([coreutils NEWS](https://raw.githubusercontent.com/coreutils/coreutils/master/NEWS)).
- **For hct.** Probe, never trust `auto`.

## Measurements on the Mac

**Conditions.** Corriva at `49fdfecf9`, two fresh worktrees under
`/private/tmp` (APFS, internal disk), on a 14-core Mac.

- **Load.** It was 45 to 70 throughout from other agents, though no CI job
  was running. Treat the times as relative, not absolute.
- **Priority.** Every build ran `nice -n 10`.
- **Repeats.** Each warm figure repeated within 0.1 s.
- **Tool versions.** Node 26.13, npm 11.20, pnpm 11.5.1, Swift 6.4.2
  (default `swiftbuild` build system).

### `clients/web` (220 packages, 8,030 files, 167 MiB `node_modules`)

These npm times include the package's `prepare` script, a `tsc` build of
about 1.2 s.

| Step | Time |
| --- | --- |
| `npm ci`, empty cache (downloads 37 MiB) | 2.8 s |
| `npm ci`, warm cache (`--prefer-offline`) | 2.1 s |
| `cp -c -R` of a warm `node_modules` (clonefile) | 0.89 s |
| `cp -R` of the same (byte copy) | 1.24 s |
| `npm install` on the cloned tree ("up to date") | 1.5 s |
| `npm ci` on the cloned tree (deletes it first) | 2.2 s |
| pnpm, empty store (`pnpm import` from package-lock) | 2.5 s |
| pnpm, warm store, `clone` (macOS default) | 0.71 s |
| pnpm, warm store, `hardlink` | 1.20 s |
| pnpm, warm store, `copy` | 0.82 s |

- **The cloned tree works.** `npm run build`, `typecheck` and the vitest
  suite pass on it (886 tests).
- **The win is small here.** On APFS, a clone is cheaper than a hard-link
  farm: creating 8,000 links costs more than 8,000 clones.

### `clients/swift` (no external dependencies; 6,705 files, 293 MiB `.build`)

| Step | Time |
| --- | --- |
| `swift build`, cold, first worktree | 15.0 s (includes the manifest cache's first fill) |
| `swift build`, cold, another path | 11.5 s |
| `swift build`, no-op | 1.4 to 1.5 s |
| `cp -c -R` of the warm `.build` | 0.8 s |
| `swift build` on the cloned `.build` (`swiftbuild`) | 10.8 s, a full rebuild (231/234 steps); `.build` grew to 387 MiB with the old build description kept |
| `swift build --build-system native` on a cloned `.build` | **fails**, every time: "precompiled file ... was compiled with module cache path '.../wt-a/...', but the path is currently '.../wt-b/...'" |

- **Don't copy `.build`.** A copied Swift `.build` is useless with the
  default build system and broken with the native one, and it does not heal
  on a rerun.
- **What to share instead.** Swift needs its dependency cache shared (the
  default already) and a compilation cache with prefix mapping, which isn't
  ready.

### What this says for Corriva

- **Dependencies are already cheap.** Installing them takes 2 to 3 s
  (`npm ci` 2.1 to 2.8 s), and the Swift SDK has no dependencies.
- **Compiling is the cost.** A fresh worktree's real cost is Rust (handled
  by sccache), Swift, Gradle and .NET builds.
- **What hct can fix there.** For those, the shared cache has to be the
  tool's own content-keyed cache: sccache, Gradle's build cache, `GOCACHE`.
  hct's value is setting those caches up correctly on every machine and
  disk, and keeping them bounded. Cloning output directories doesn't help.
- **Hecatron's other users.** Copying installed trees helps projects with
  large dependency trees (the 750k-file and Electron examples above), not
  Corriva.

## Design

### Name: `hct cache`

A noun with subcommands, like `hct self`. It covers configuring, seeding,
reporting and pruning, where `warm` names only one of them and `share`
says nothing about pruning.

### Commands

```
hct cache status [lang...] [--json]   # what each language shares, where, how (clone/hardlink/copy), size, same-disk or not
hct cache setup [lang...] [--root DIR] [--dry-run]
                                     # point each tool's stores at a root on the project's filesystem;
                                     # write .hecatron/local.json (per machine, untracked), never the user's dotfiles
hct cache env [lang...]               # the variables `setup` chose, for eval or a hook; also folded into `hct env`
hct cache seed --from WORKTREE [lang...] [--dry-run]
                                     # copy-on-write clone of the safe installed trees from a warm worktree,
                                     # then each language's own install to reconcile
hct prune --caches                    # (from #378) also trims the shared stores hct set up
```

- **`setup` is the main command.** Its whole effect is environment
  variables or tool config scoped to the project. For example, pnpm's
  `store-dir`, `UV_CACHE_DIR`, `npm_config_cache`, `GOMODCACHE` and
  `GOCACHE`, `NUGET_PACKAGES`, the sccache dir and wrapper, and Gradle's
  `org.gradle.caching=true` through an init script in the hct root.
- **It puts each store on the project's filesystem.** The root defaults to
  `<volume of the project>/.hct-cache/` (`hct cache setup --root` changes
  it). A worktree on another volume gets its own root on that volume.
- **It is idempotent.** It respects variables the user already set and says
  so.
- **`status` is the doctor.** It flags a store on another filesystem from
  the project (every link would be a copy) and two sccache servers on one
  dir. It also flags `npm ci` in a hook after a seed, and a language whose
  outputs can't be shared ("Swift: .build is per path; shared: package
  cache").
- **`seed` clones only what a language marks seedable.** The marks are in
  the table below. It uses the platform's copy-on-write and nothing else:
  on a filesystem without clones it refuses and says why, instead of
  copying gigabytes. Then it runs the language's `deps.sh`, which must
  reconcile, never wipe. For npm that means `npm install --prefer-offline`
  after a seed, not `npm ci`.
- **Per-language behaviour is a hook.** A new `cache.sh` per language, with
  actions `status`, `setup`, `env` and `seed`, follows the shape of
  `prune.sh` in #378. A project overrides it in
  `.hecatron/languages/<lang>/cache.sh` (for Corriva, a Rust hook that knows
  about its slots).

### Per-language strategy

| Language | Share (setup) | How | Seed from a warm worktree? | Hazards and mitigations |
| --- | --- | --- | --- | --- |
| JS/TS, pnpm | Store (`store-dir`) | pnpm links: clone on APFS/btrfs/XFS, hard link on ext4 | No need (warm store: 0.7 s) | Hard links: edits reach the store. Keep `verifyStoreIntegrity`; on ext4 consider `package-import-method=clone-or-copy` where edits happen |
| JS/TS, bun | Global cache | bun's clonefile or hardlink | No need | As pnpm |
| JS/TS, Yarn PnP | Global cache | Zips used in place | No need | Never zero-installs in a large repository |
| JS/TS, npm | `_cacache` | Download cache only | **Yes**, `node_modules` by clone (APFS, btrfs, XFS) | `npm ci` deletes the seed: the hook runs `npm install`. Lockfile change: install reconciles. Never on ext4 (would be a byte copy) |
| Python (uv) | `UV_CACHE_DIR` on the project's disk | uv clone, or hard link on ext4 | No: venvs embed their path; `uv sync` from a warm cache is quick | Never `symlink` mode. Hard links: don't edit site-packages |
| Rust | sccache dir and wrapper; `~/.cargo/registry` | sccache; registry per user | **No**, never a target dir across paths (#2142, cargo#12516) | Offer the fixed-path slot pattern (Corriva #2159) as a documented recipe. Revisit when cargo's cross-workspace cache stabilises |
| Go | `GOMODCACHE`, `GOCACHE` | Per-user, already shared; just the right disk | No | None at the defaults. Don't force `-trimpath` (it changes binaries) |
| JVM (Gradle) | `GRADLE_USER_HOME`; build cache on | Gradle's local build cache; remote HTTP cache optional | No: `.gradle/` and `build/` are per path | Missing task inputs give wrong hits: on by project choice. One machine shares safely. Ephemeral CI uses `GRADLE_RO_DEP_CACHE` |
| .NET | `NUGET_PACKAGES`; one shared temp dir | Packages used in place | No: `obj/` holds absolute paths | Processes must share NuGet's temp dir (scratch locks) |
| Swift | SwiftPM cache (`--cache-path`) | Shared repositories and artifacts | **No** (measured: full rebuild or a broken build) | Watch Xcode/swift-build compilation caching with prefix mapping |
| Zig | Global cache | Already shared | No | Don't share `.zig-cache` (no guarantee) |
| C/C++ | ccache dir | `base_dir` set to the worktree root | No | `hash_dir` and debug info; never `hard_link` |

### Platforms

| Filesystem | Copy-on-write | What hct does |
| --- | --- | --- |
| APFS (macOS) | `clonefile(2)` / `cp -c`, within one volume | Clone. Stores per volume (`/private/tmp` and `/Volumes/Data` are different volumes, so a store on one is a copy source for the other) |
| btrfs, XFS with `reflink=1` | `FICLONE` / `cp --reflink=always` | Clone |
| ext4 | none | Tools' own hard links (pnpm, uv, bun) into a store on the **same** filesystem; `seed` refuses |
| Windows NTFS / ReFS | ReFS block cloning only | Later; tools hard-link |

- **Probe, don't assume.** hct probes once per root, by cloning a temp file
  with `--reflink=always` or `cp -c` and comparing device numbers.
- **The probe result goes in `status`.**

## How Corriva would use it

- **`scripts/dev/worktree-create.sh`.**
  - After `git worktree add`, it runs `hct cache seed --from "$repo"
    javascript` on the Mac. The main checkout is the warm source, or else
    the newest worktree whose lockfile matches.
  - It doesn't seed Swift, .NET or Rust.
  - Corriva's worktrees are on `/Volumes/Data` while the main checkout is
    on the internal disk, so a clone from the main checkout is impossible.
    The warm source has to be a worktree on the same volume, or a "warm
    template" worktree hct keeps there (a pool of one).
- **`scripts/dev/run.sh` and `ci-local.sh`** source `hct cache env` instead
  of each `*-env.sh`. sccache's setup moves into the Rust hook, with its
  per-OS dir and size, and the one-server-per-port rule stays.
- **`ci-remote` on legion1.**
  - Its home directory is on `/` (nvme0) while `/data` is nvme1, so
    `~/.npm`, `~/.cache/uv` and pnpm's store can't hard-link into
    `/data/agent-ci`.
  - `hct cache setup --root /data/cache` puts them beside the existing
    `/data/cache/sccache` and `go-build`.
  - The fixed-path slots stay, and they are already the "pool" pattern for
    Rust.
  - `hct prune --caches` then trims `/data/cache`. `trim-caches.py` was
    never installed (#2159).
- **Not worth changing.** Corriva doesn't need to move from npm to pnpm:
  the install it would save is about 2 s.

## Phased plan

1. **`hct cache status` and `setup`/`env`.** No copying. This alone fixes
   the cross-filesystem case (legion1) and makes each tool's own sharing
   visible. Hooks for javascript, python, rust (sccache), go, jvm, dotnet,
   swift and zig. `hct prune --caches` learns the roots `setup` chose.
2. **`hct cache seed`**, JavaScript (npm) only, with the copy-on-write
   probe. Then Corriva's `worktree-create.sh` uses it.
3. **Recipes, not features**, for what can't be made generic: the cargo
   fixed-path slot (from Corriva #2159), Gradle's remote build cache for CI,
   and a warm-worktree pool. Each is a guide page with a hook to copy.
4. **Revisit** when cargo's cross-workspace cache or Swift's prefix-mapped
   compilation cache stabilises. Either would let `setup` share compiled
   Rust or Swift across worktrees directly.

## Decisions for the owner

| # | Question | Options | Recommendation |
| --- | --- | --- | --- |
| 1 | Name | `hct cache`, `hct warm`, `hct share` | **`hct cache`**, a noun group like `hct self` |
| 2 | Default posture | (a) stores only; (b) stores plus seeding by default | **(a)**: seeding is opt-in per language (`seed`, or a project's `cache.seed` list) |
| 3 | Where `setup` writes | User dotfiles (`~/.npmrc`, `~/.config/uv`); `.hecatron/local.json` plus `hct env`; tracked config | **`.hecatron/local.json` plus `hct env`**: per machine, never edits the user's tools globally |
| 4 | Rust target dirs | Seed and evict workspace units; share a build-dir; don't | **Don't**: sccache plus fixed-path slots; revisit with cargo's goal (#2142, #2159, cargo#12516) |
| 5 | Hard links on ext4 | Allow the tools' own (pnpm, uv, bun); forbid | **Allow the tools' own**, never hct-made hard links of build outputs |
| 6 | No copy-on-write available | Fall back to a byte copy; refuse | **Refuse** and say so. A byte copy is no faster than an install (1.24 s vs 0.89 s here, gigabytes elsewhere) |
| 7 | Corriva's web SDK on pnpm | Switch; stay on npm | **Stay on npm**: the measured saving is about 2 s per worktree |
| 8 | Warm-worktree pool (pre-made worktrees handed out) | Build it into hct; leave it to the project | **Leave it to the project** for now (a recipe in phase 3). It is the only approach that helps when cloning is slow, but it is workflow, not toolchain |
