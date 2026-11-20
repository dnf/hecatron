#!/usr/bin/env python3
"""hct prune's trimming (docs/guide/prune.md): remove the stale part of a
build directory or cache and keep what the next build reuses, unlike hct
clean, which removes it all. The prune hooks (languages/<lang>/hooks/
prune.sh) call it, one kind of directory at a time.

    prune.py check                  validate HECATRON_PRUNE_OLDER_THAN and
                                    HECATRON_PRUNE_MAX_SIZE (exit 2, saying why)
    prune.py cargo TARGET           a cargo target directory: old builds
    prune.py zig CACHE...           Zig caches (.zig-cache/, zig-cache/), whole
    prune.py swiftpm BUILD          SwiftPM's .build/: old configurations
    prune.py derived-data DIR       Xcode's DerivedData/: folders whose
                                    workspace is gone
    prune.py dotnet DIR             .NET's bin/<configuration>/ and
                                    obj/<configuration>/ below DIR
    prune.py nuget PACKAGES         ~/.nuget/packages: old package versions
    prune.py gradle PROJECT         a Gradle build's .gradle/<version>/ for
                                    the Gradle versions it no longer uses
    prune.py tool CACHE -- COMMAND...
                                    a tool's own trim of its cache (uv cache
                                    prune, npm cache verify): run it, and say
                                    how much the cache shrank

What every kind shares:
  - an item is stale when it's older than HECATRON_PRUNE_OLDER_THAN (each
    kind has its own default, and its own idea of "older": a cargo build is
    old next to its group's newest build). Age is the modification time,
    never the access time, which noatime and relatime mounts don't keep;
  - with HECATRON_PRUNE_MAX_SIZE, more items go, oldest first, until the
    directory is under it (never the newest build of anything);
  - with HECATRON_PRUNE_DRY_RUN=1, nothing is removed;
  - a directory a build holds a lock on is skipped (cargo's .cargo-lock,
    SwiftPM's .build/.lock, the files Zig locks in its cache's h/), and
    the locks are held while removing, so a build that starts waits;
  - nothing git tracks is removed, and no symlink is followed;
  - it ends with one line per directory, which the runner reads:
        LANG: prune: would free|freed 1.2 GiB (BYTES bytes) in N items from WHERE
    (a tool's own trim says no items: it doesn't say what it removed).

Sizes count what removing frees: a file with another hard link (cargo's
copies of its binaries) frees nothing.
"""

from __future__ import annotations

import fcntl
import os
import plistlib
import re
import shutil
import stat
import subprocess
import sys
import time
from dataclasses import dataclass, field

MINUTE, HOUR, DAY, WEEK = 60, 3600, 86400, 7 * 86400
UNITS = {"s": 1, "m": MINUTE, "h": HOUR, "d": DAY, "w": WEEK}
DURATION = re.compile(r"^(?:\d+(?:\.\d+)?[smhdw])+$")
SIZE = re.compile(r"^(\d+(?:\.\d+)?)\s*([kmgt]?)(?:i?b)?$", re.IGNORECASE)
SIZE_UNITS = {"": 1, "k": 1 << 10, "m": 1 << 20, "g": 1 << 30, "t": 1 << 40}
# What a build may still be writing: never removed, whatever the settings.
RECENT = 10 * MINUTE


def parse_duration(text: str) -> float | None:
    """Seconds in 90s, 6h, 7d, 2w, 1h30m; None when it isn't one."""
    if not DURATION.match(text):
        return None
    return sum(float(n) * UNITS[u] for n, u in re.findall(r"(\d+(?:\.\d+)?)([smhdw])", text))


def parse_size(text: str) -> int | None:
    """Bytes in 500M, 50G, 50GiB, 1.5T, 1024; None when it isn't one. K, M,
    G and T are powers of 1024."""
    m = SIZE.match(text.strip())
    if not m:
        return None
    return int(float(m.group(1)) * SIZE_UNITS[m.group(2).lower()])


def human(n: float) -> str:
    for unit in ("B", "KiB", "MiB", "GiB"):
        if n < 1024 or unit == "GiB":
            return f"{n:.0f} {unit}" if unit == "B" else f"{n:.1f} {unit}"
        n /= 1024
    return f"{n:.1f} GiB"


def describe_age(seconds: float) -> str:
    for unit, size in (("w", WEEK), ("d", DAY), ("h", HOUR), ("m", MINUTE)):
        if seconds >= size and seconds % size == 0:
            return f"{seconds // size:.0f}{unit}"
    return f"{seconds:.0f}s"


@dataclass
class Settings:
    older_than: float
    max_size: int | None
    dry_run: bool
    lang: str
    root: str

    @classmethod
    def from_env(cls, default_age: float) -> Settings:
        age = os.environ.get("HECATRON_PRUNE_OLDER_THAN", "")
        size = os.environ.get("HECATRON_PRUNE_MAX_SIZE", "")
        return cls(older_than=parse_duration(age) if age else default_age,
                   max_size=parse_size(size) if size else None,
                   dry_run=os.environ.get("HECATRON_PRUNE_DRY_RUN") == "1",
                   lang=os.environ.get("HECATRON_LANG", "") or "prune",
                   root=os.environ.get("HECATRON_ROOT", ""))

    def say(self, text: str) -> None:
        print(f"{self.lang}: prune: {text}", flush=True)

    def where(self, path: str) -> str:
        """path as the summary names it: relative to the project, or ~/."""
        path = os.path.abspath(path)
        root = os.path.realpath(self.root) if self.root else ""
        real = os.path.realpath(path)
        if root and (real == root or real.startswith(root + os.sep)):
            return os.path.relpath(real, root)
        home = os.path.expanduser("~")
        if home and path.startswith(home + os.sep):
            return "~" + path[len(home):]
        return path


@dataclass
class Item:
    """What goes together: a cargo build's files, one cache, one
    configuration's output. The first path is removed first."""
    name: str
    paths: list[str]
    time: float           # when it was last built or used, by mtime
    stale: bool = False   # goes by age
    evictable: bool = True  # may go to get under --max-size
    size: int = -1

    def bytes(self) -> int:
        if self.size < 0:
            self.size = size_of(self.paths)
        return self.size


@dataclass
class Plan:
    items: list[Item] = field(default_factory=list)
    total: int = 0  # the directory's size, for --max-size

    def choose(self, settings: Settings, now: float) -> list[Item]:
        """The stale items, then, while over --max-size, the oldest
        evictable ones; never one written in the last few minutes."""
        fresh = [i for i in self.items if i.time > now - RECENT]
        chosen = [i for i in self.items if i.stale and i not in fresh]
        if settings.max_size is not None:
            left = self.total - sum(i.bytes() for i in chosen)
            for item in sorted(self.items, key=lambda i: i.time):
                if left <= settings.max_size:
                    break
                if item.evictable and item not in chosen and item not in fresh:
                    chosen.append(item)
                    left -= item.bytes()
        return chosen


def walk_files(path: str):
    """Every file and directory below path (and path), as lstat results,
    without following a symlink."""
    try:
        st = os.lstat(path)
    except OSError:
        return
    yield path, st
    if not stat.S_ISDIR(st.st_mode):
        return
    try:
        entries = list(os.scandir(path))
    except OSError:
        return
    for e in entries:
        yield from walk_files(e.path)


def size_of(paths: list[str]) -> int:
    """Bytes freed by removing paths (a hard link kept elsewhere frees nothing)."""
    total = 0
    for p in paths:
        for _path, st in walk_files(p):
            if stat.S_ISREG(st.st_mode) and st.st_nlink == 1:
                total += st.st_blocks * 512
    return total


def newest_mtime(path: str) -> float:
    return max((st.st_mtime for _p, st in walk_files(path)), default=0.0)


def remove(path: str) -> None:
    if os.path.isdir(path) and not os.path.islink(path):
        shutil.rmtree(path, ignore_errors=True)
    else:
        try:
            os.unlink(path)
        except FileNotFoundError:
            pass


class Locks:
    """Build tools' lock files, held without waiting; held is False when a
    build holds one (and nothing is held then)."""

    def __init__(self, paths: list[str]):
        self.fds: list[int] = []
        self.held = True
        for path in paths:
            try:
                fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
            except OSError:
                continue
            try:
                fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
            except OSError:
                os.close(fd)
                self.release()
                self.held = False
                return
            self.fds.append(fd)

    def release(self) -> None:
        for fd in self.fds:
            os.close(fd)
        self.fds = []


def tracked_in(directory: str) -> list[str]:
    """The files git tracks below directory, as absolute paths; none
    outside a repository."""
    if not os.path.isdir(directory):
        return []
    try:
        out = subprocess.run(["git", "-C", directory, "ls-files", "-z", "--", "."],
                             capture_output=True, text=True, check=False)
    except OSError:
        return []
    if out.returncode != 0:
        return []
    return [os.path.join(os.path.realpath(directory), p) for p in out.stdout.split("\0") if p]


def holds_tracked(item: Item, tracked: list[str]) -> bool:
    for path in item.paths:
        real = os.path.realpath(path)
        if any(t == real or t.startswith(real + os.sep) for t in tracked):
            return True
    return False


def carry_out(settings: Settings, plan: Plan, where: str, tracked: list[str] | None = None,
              now: float | None = None, show: int = 5) -> tuple[int, int]:
    """Remove (or, in a dry run, only count) what the plan chooses, and say
    how much: (bytes, items)."""
    now = time.time() if now is None else now
    chosen = [i for i in plan.choose(settings, now) if not tracked or not holds_tracked(i, tracked)]
    freed = 0
    for item in chosen:
        freed += item.bytes()
        if not settings.dry_run:
            for path in item.paths:
                remove(path)
    if settings.dry_run and chosen:
        for item in sorted(chosen, key=lambda i: -i.bytes())[:show]:
            settings.say(f"  {human(item.bytes())}  {item.name}")
    summary(settings, freed, len(chosen), where)
    return freed, len(chosen)


def summary(settings: Settings, freed: int, items: int, where: str) -> None:
    verb = "would free" if settings.dry_run else "freed"
    settings.say(f"{verb} {human(freed)} ({freed} bytes) in {items} item{'' if items == 1 else 's'} from {where}")


# --- cargo -------------------------------------------------------------------
#
# Cargo never deletes a build. Each time a unit's metadata hash changes (a
# Cargo.lock bump, another feature set, a new toolchain), the new build lands
# beside the old ones, so a target directory CI reuses only grows (one
# reached 114 GiB, 69 of it old builds).
#
# A unit, in each profile directory (debug, release, <triple>/release: any
# directory holding cargo's .cargo-lock or a .fingerprint), is everything
# named with one metadata hash: .fingerprint/<pkg>-<hash>/, the deps/,
# examples/ and build/ entries named <name>-<hash>[.ext], and, in cargo
# 1.100's layout, build/<pkg>/<hash>/. Units are grouped by what they build,
# from their fingerprint (the package and target: lib-serde,
# test-integration-test-it), and each group keeps the builds made within the
# window (--older-than, 6h) of its newest. A window rather than the newest
# one: a run builds several variants of a package side by side (other
# features, a crate as a build dependency too), minutes apart; an old hash
# is hours or days older. A unit goes fingerprint first: one without its
# fingerprint is one cargo rebuilds, so a prune cut short never leaves cargo
# trusting a half-removed unit. Hashes no fingerprint names are left alone,
# and so are the copies cargo hard-links out of deps/ into the profile.

CARGO_HASH = re.compile(r"^(.+)-([0-9a-f]{16})(?=\.|$)")
HEX16 = re.compile(r"^[0-9a-f]{16}$")
CARGO_FLAT = ("deps", "examples", "build", ".fingerprint")


def cargo_profiles(target: str, depth: int = 5) -> list[str]:
    out: list[str] = []

    def go(d: str, n: int) -> None:
        try:
            entries = list(os.scandir(d))
        except OSError:
            return
        if any(e.name in (".fingerprint", ".cargo-lock") for e in entries):
            out.append(d)
            return
        if n == 0:
            return
        for e in entries:
            if e.is_dir(follow_symlinks=False) and not e.name.startswith(".") \
                    and e.name not in ("doc", "tmp", "deps", "build", "incremental"):
                go(e.path, n - 1)

    go(target, depth)
    return sorted(out)


def cargo_describe(fingerprint: str, package: str) -> tuple[str, float]:
    """A unit's group (package and target, from its fingerprint's .json
    files) and when it was built (the newest file in its fingerprint)."""
    kinds, built = [], 0.0
    try:
        entries = list(os.scandir(fingerprint))
    except OSError:
        return "", 0.0
    for e in entries:
        try:
            built = max(built, e.stat(follow_symlinks=False).st_mtime)
        except OSError:
            pass
        if e.name.endswith(".json"):
            kinds.append(e.name[:-5])
    if not kinds:
        return "", built
    return f"{package}/{'+'.join(sorted(kinds))}", built


@dataclass
class CargoUnit:
    hash: str
    group: str = ""
    built: float = 0.0
    fingerprint: str = ""
    paths: list[str] = field(default_factory=list)


def cargo_units(profile: str) -> list[CargoUnit]:
    units: dict[str, CargoUnit] = {}

    def unit(h: str) -> CargoUnit:
        return units.setdefault(h, CargoUnit(h))

    for sub in CARGO_FLAT:
        d = os.path.join(profile, sub)
        if not os.path.isdir(d) or os.path.islink(d):
            continue
        for e in os.scandir(d):
            m = CARGO_HASH.match(e.name)
            if m:
                u = unit(m.group(2))
                u.paths.append(e.path)
                if sub == ".fingerprint" and e.is_dir(follow_symlinks=False):
                    u.fingerprint = e.path
                    u.group, u.built = cargo_describe(e.path, m.group(1))
            elif sub == "build" and e.is_dir(follow_symlinks=False):
                # cargo 1.100: build/<pkg>/<hash>/{fingerprint,out,...}
                for h in os.scandir(e.path):
                    if HEX16.match(h.name) and h.is_dir(follow_symlinks=False):
                        u = unit(h.name)
                        u.paths.append(h.path)
                        fp = os.path.join(h.path, "fingerprint")
                        if os.path.isdir(fp):
                            u.fingerprint = fp
                            u.group, u.built = cargo_describe(fp, e.name)
    return [u for u in units.values() if u.fingerprint and u.group]


def cargo_plan(profile: str, window: float) -> Plan:
    plan = Plan()
    groups: dict[str, list[CargoUnit]] = {}
    for u in cargo_units(profile):
        groups.setdefault(u.group, []).append(u)
    for group, units in sorted(groups.items()):
        newest = max(u.built for u in units)
        for u in units:
            # The fingerprint first: no fingerprint, no reuse.
            paths = [u.fingerprint] + [p for p in u.paths if p != u.fingerprint]
            plan.items.append(Item(name=f"{os.path.basename(profile)}: {group} {u.hash}", paths=paths, time=u.built,
                                   stale=u.built < newest - window, evictable=u.built < newest))
    # incremental/: rustc's own caches, one per crate; one no build has
    # written within the window of the profile's newest is stale too.
    incremental = os.path.join(profile, "incremental")
    if os.path.isdir(incremental) and not os.path.islink(incremental):
        newest = max((i.time for i in plan.items), default=0.0)
        for e in os.scandir(incremental):
            if e.is_dir(follow_symlinks=False):
                t = newest_mtime(e.path)
                plan.items.append(Item(name=f"{os.path.basename(profile)}: incremental/{e.name}", paths=[e.path],
                                       time=t, stale=t < newest - window))
    return plan


def prune_cargo(target: str) -> int:
    settings = Settings.from_env(6 * HOUR)
    where = settings.where(target)
    if not os.path.isdir(target) or os.path.islink(target):
        settings.say(f"no target directory at {where}")
        summary(settings, 0, 0, where)
        return 0
    settings.say(f"keeping each package and target's builds made within {describe_age(settings.older_than)} "
                 "of its newest" + (f", under {human(settings.max_size)} in all" if settings.max_size is not None else ""))
    tracked = tracked_in(target)
    freed = items = 0
    profiles = cargo_profiles(target)
    plans: list[tuple[str, Plan, Locks | None]] = []
    for profile in profiles:
        locks = None
        if not settings.dry_run:
            locks = Locks([os.path.join(profile, n) for n in sorted(os.listdir(profile))
                           if n.startswith(".cargo") and n.endswith("lock")])
            if not locks.held:
                settings.say(f"cargo is building in {settings.where(profile)}; skipped it")
                continue
        plans.append((profile, cargo_plan(profile, settings.older_than), locks))
    try:
        # --max-size is the whole target directory's: one plan for all.
        plan = Plan(items=[i for _p, pl, _l in plans for i in pl.items])
        if settings.max_size is not None:
            plan.total = size_of([target])
        freed, items = carry_out(settings, plan, where, tracked)
    finally:
        for _p, _pl, locks in plans:
            if locks:
                locks.release()
    return 0


# --- Zig -----------------------------------------------------------------------
#
# A Zig cache can't be trimmed entry by entry: a manifest (h/) names its
# output (o/<digest>) only through a hash, a cache hit touches neither, and
# a manifest whose output is gone fails the build (FileNotFound, Zig 0.17)
# rather than rebuilding it. So a cache goes whole, when no build has used
# it (h/timestamp, which every build touches) for --older-than (7d), or
# oldest first for --max-size. Zig locks the manifests a build uses.

def zig_last_used(cache: str) -> float:
    try:
        return os.stat(os.path.join(cache, "h", "timestamp")).st_mtime
    except OSError:
        return newest_mtime(cache)


def zig_locks(cache: str) -> list[str]:
    h = os.path.join(cache, "h")
    try:
        return [e.path for e in os.scandir(h) if e.is_file(follow_symlinks=False)]
    except OSError:
        return []


def prune_zig(caches: list[str]) -> int:
    settings = Settings.from_env(7 * DAY)
    now = time.time()
    named = caches
    caches = [c for c in caches if os.path.isdir(c) and not os.path.islink(c)]
    where = ", ".join(settings.where(c) for c in caches or named[:1])
    plan, held = Plan(), []
    try:
        for cache in caches:
            locks = None
            if not settings.dry_run:
                locks = Locks(zig_locks(cache))
                if not locks.held:
                    settings.say(f"a Zig build is using {settings.where(cache)}; skipped it")
                    continue
                held.append(locks)
            used = zig_last_used(cache)
            plan.items.append(Item(name=settings.where(cache), paths=[cache], time=used,
                                   stale=used < now - settings.older_than))
        if settings.max_size is not None:
            plan.total = size_of(caches)
        carry_out(settings, plan, where, now=now)
    finally:
        for locks in held:
            locks.release()
    return 0


# --- SwiftPM -------------------------------------------------------------------
#
# .build/<triple>/<configuration>/ (debug, release) is one build's output; a
# configuration nothing has built for --older-than (7d) goes; the build
# that needs it again finds its outputs missing and rebuilds them. checkouts/, repositories/ and artifacts/ hold the
# dependencies' sources and binaries, and stay. SwiftPM holds .build/.lock
# while it builds.

# A build directory is named for its target triple (arm64-apple-macosx,
# x86_64-unknown-linux-gnu); nothing else in .build/ is one.
SWIFTPM_TRIPLE = re.compile(r"^[a-z0-9_]+-[a-z0-9_]+-[a-z0-9_.-]+$")


def prune_swiftpm(build: str) -> int:
    settings = Settings.from_env(7 * DAY)
    where = settings.where(build)
    now = time.time()
    if not os.path.isdir(build) or os.path.islink(build):
        summary(settings, 0, 0, where)
        return 0
    locks = None
    if not settings.dry_run:
        locks = Locks([os.path.join(build, ".lock")])
        if not locks.held:
            settings.say(f"SwiftPM is building in {where}; skipped it")
            summary(settings, 0, 0, where)
            return 0
    try:
        plan = Plan()
        for triple in os.scandir(build):
            if not triple.is_dir(follow_symlinks=False) or not SWIFTPM_TRIPLE.match(triple.name):
                continue
            for config in os.scandir(triple.path):
                if not config.is_dir(follow_symlinks=False):
                    continue
                t = newest_mtime(config.path)
                plan.items.append(Item(name=f"{triple.name}/{config.name}", paths=[config.path], time=t,
                                       stale=t < now - settings.older_than))
        newest = max((i.time for i in plan.items), default=0.0)
        for item in plan.items:
            item.evictable = item.time < newest
        if settings.max_size is not None:
            plan.total = size_of([build])
        carry_out(settings, plan, where, tracked_in(build), now=now)
    finally:
        if locks:
            locks.release()
    return 0


# --- Xcode's DerivedData ---------------------------------------------------------
#
# Each folder's info.plist names the workspace or project it built
# (WorkspacePath); a folder whose workspace is gone (a deleted clone, a
# removed worktree) is never used again, whatever its age. With
# --older-than, a folder unused for that long goes too.

def prune_derived_data(directory: str) -> int:
    settings = Settings.from_env(0)
    by_age = bool(os.environ.get("HECATRON_PRUNE_OLDER_THAN"))
    where = settings.where(directory)
    now = time.time()
    plan = Plan()
    if os.path.isdir(directory):
        for e in os.scandir(directory):
            if not e.is_dir(follow_symlinks=False):
                continue
            info = os.path.join(e.path, "info.plist")
            workspace = ""
            try:
                with open(info, "rb") as f:
                    workspace = str(plistlib.load(f).get("WorkspacePath", ""))
            except (OSError, plistlib.InvalidFileException, ValueError):
                pass
            t = newest_top(e.path)
            gone = bool(workspace) and not os.path.exists(workspace)
            plan.items.append(Item(name=e.name + (f" ({workspace} is gone)" if gone else ""), paths=[e.path], time=t,
                                   stale=gone or (by_age and t < now - settings.older_than)))
        if settings.max_size is not None:
            plan.total = size_of([directory])
    carry_out(settings, plan, where, now=now)
    return 0


def newest_top(path: str) -> float:
    """The newest mtime of path and its direct entries: what a build in it
    touches, without walking a large folder."""
    times = []
    try:
        times.append(os.lstat(path).st_mtime)
        times += [e.stat(follow_symlinks=False).st_mtime for e in os.scandir(path)]
    except OSError:
        pass
    return max(times, default=0.0)


# --- .NET ------------------------------------------------------------------------
#
# Each project's bin/<configuration>/ and obj/<configuration>/ (Debug,
# Release) are one configuration's build; one nothing has built for
# --older-than (7d) goes, while the configuration built most recently in
# the project stays. obj/project.assets.json and the restore files beside it
# are the restore's, and stay.

def dotnet_projects(directory: str) -> list[str]:
    out = []
    for root, dirs, files in os.walk(directory):
        dirs[:] = [d for d in dirs if not d.startswith(".") and d not in ("bin", "obj", "node_modules")]
        if any(f.endswith("proj") for f in files):
            out.append(root)
    return sorted(out)


def prune_dotnet(directory: str) -> int:
    settings = Settings.from_env(7 * DAY)
    where = settings.where(directory)
    now = time.time()
    plan = Plan()
    for project in dotnet_projects(directory):
        configs: dict[str, list[str]] = {}
        for out in ("bin", "obj"):
            d = os.path.join(project, out)
            if not os.path.isdir(d) or os.path.islink(d):
                continue
            for e in os.scandir(d):
                if e.is_dir(follow_symlinks=False):
                    configs.setdefault(e.name, []).append(e.path)
        items = []
        for config, paths in sorted(configs.items()):
            t = max(newest_mtime(p) for p in paths)
            name = os.path.relpath(os.path.join(project, config), directory)
            items.append(Item(name=f"{name} ({config})", paths=paths, time=t, stale=t < now - settings.older_than))
        newest = max((i.time for i in items), default=0.0)
        for item in items:
            item.evictable = item.time < newest
            item.stale = item.stale and item.time < newest
        plan.items += items
    if settings.max_size is not None:
        plan.total = sum(i.bytes() for i in plan.items)
    carry_out(settings, plan, where, tracked_in(directory), now=now)
    return 0


# --- NuGet's packages folder -----------------------------------------------------
#
# ~/.nuget/packages/<id>/<version>/: a package version restore extracted.
# Every package keeps its highest version and the one used last; another
# version goes when it's
# older than --older-than (30d) by its .nupkg.metadata, which NuGet writes
# when it extracts the version and touches on each restore that uses it
# where updatePackageLastAccessTime is set. A restore that needs it again
# downloads it again.

def version_key(name: str) -> list:
    """A version's order: its numbers, as numbers (13.0.10 after 13.0.9)."""
    return [(0, int(p)) if p.isdigit() else (1, p) for p in re.split(r"[.+-]", name.split(" ", 1)[-1])]


def prune_nuget(directory: str) -> int:
    settings = Settings.from_env(30 * DAY)
    where = settings.where(directory)
    now = time.time()
    plan = Plan()
    if os.path.isdir(directory):
        for package in os.scandir(directory):
            if not package.is_dir(follow_symlinks=False):
                continue
            versions = []
            for version in os.scandir(package.path):
                if not version.is_dir(follow_symlinks=False):
                    continue
                marker = os.path.join(version.path, ".nupkg.metadata")
                try:
                    t = os.stat(marker).st_mtime
                except OSError:
                    t = newest_top(version.path)
                # The marker first: NuGet takes a version without it for
                # one it hasn't finished extracting, and extracts it again.
                paths = ([marker] if os.path.exists(marker) else []) + [version.path]
                versions.append(Item(name=f"{package.name} {version.name}", paths=paths, time=t,
                                     stale=t < now - settings.older_than))
            newest = max((v.time for v in versions), default=0.0)
            highest = max((v.name for v in versions), key=version_key, default="")
            for v in versions:
                if v.time >= newest or v.name == highest:
                    v.stale = v.evictable = False
            plan.items += versions
        if settings.max_size is not None:
            plan.total = size_of([directory])
    carry_out(settings, plan, where, now=now)
    return 0


# --- Gradle's project cache ------------------------------------------------------
#
# A Gradle build keeps a project cache per Gradle version, .gradle/<version>/
# (file hashes, the configuration cache, execution history), and Gradle
# never removes one for a version the build has moved off (it trims its own
# caches in ~/.gradle itself). One that isn't the wrapper's version or the
# newest goes when unused for --older-than (7d).

GRADLE_VERSION_DIR = re.compile(r"^\d+\.\d+(\.\d+)?([-.].*)?$")
GRADLE_WRAPPER_VERSION = re.compile(r"distributionUrl=.*gradle-([0-9][^-/]*?)-(?:bin|all)\.zip")


def gradle_wrapper_version(project: str) -> str:
    try:
        with open(os.path.join(project, "gradle", "wrapper", "gradle-wrapper.properties")) as f:
            m = GRADLE_WRAPPER_VERSION.search(f.read())
    except OSError:
        return ""
    return m.group(1) if m else ""


def prune_gradle(project: str) -> int:
    settings = Settings.from_env(7 * DAY)
    cache = os.path.join(project, ".gradle")
    where = settings.where(cache)
    now = time.time()
    plan = Plan()
    wrapper = gradle_wrapper_version(project)
    if os.path.isdir(cache) and not os.path.islink(cache):
        for e in os.scandir(cache):
            if e.is_dir(follow_symlinks=False) and GRADLE_VERSION_DIR.match(e.name):
                t = newest_mtime(e.path)
                plan.items.append(Item(name=f".gradle/{e.name}", paths=[e.path], time=t,
                                       stale=t < now - settings.older_than))
        newest = max((i.time for i in plan.items), default=0.0)
        for item in plan.items:
            if item.time >= newest or item.name == f".gradle/{wrapper}":
                item.stale = item.evictable = False
        if settings.max_size is not None:
            plan.total = size_of([cache])
    carry_out(settings, plan, where, tracked_in(project), now=now)
    return 0


# --- a tool's own trim -------------------------------------------------------------

def prune_tool(cache: str, command: list[str]) -> int:
    """Run a tool's own trim of its cache, measuring the cache before and
    after; a dry run says what it would run (the tools can't say ahead
    what they'd remove)."""
    settings = Settings.from_env(0)
    where = settings.where(cache)
    if settings.dry_run:
        settings.say(f"would run {' '.join(command)} on {where} ({human(size_of([cache]))} now)")
        return 0
    before = size_of([cache])
    print("+ " + " ".join(command), flush=True)
    status = subprocess.run(command, check=False).returncode
    freed = max(0, before - size_of([cache]))
    verb = "freed"
    settings.say(f"{verb} {human(freed)} ({freed} bytes) from {where}")
    return status


def check() -> int:
    age = os.environ.get("HECATRON_PRUNE_OLDER_THAN", "")
    size = os.environ.get("HECATRON_PRUNE_MAX_SIZE", "")
    if age and parse_duration(age) is None:
        print(f"hecatron: --older-than takes a duration such as 6h, 7d or 2w (got '{age}')", file=sys.stderr)
        return 2
    if size and parse_size(size) is None:
        print(f"hecatron: --max-size takes a size such as 500M or 50G (got '{size}')", file=sys.stderr)
        return 2
    return 0


def main(argv: list[str]) -> int:
    if not argv:
        print(__doc__, file=sys.stderr)
        return 2
    if check() != 0:
        return 2
    kind, args = argv[0], argv[1:]
    if kind == "check":
        return 0
    one = {"cargo": prune_cargo, "swiftpm": prune_swiftpm, "derived-data": prune_derived_data,
           "dotnet": prune_dotnet, "nuget": prune_nuget}
    if kind in one and len(args) == 1:
        return one[kind](args[0])
    if kind == "gradle" and len(args) == 1:
        return prune_gradle(args[0])
    if kind == "zig" and args:
        return prune_zig(args)
    if kind == "tool" and len(args) >= 3 and args[1] == "--":
        return prune_tool(args[0], args[2:])
    print(__doc__, file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
