"""hct prune: libexec/lib/prune.py's trimming of each kind of directory, on
fixtures built here (ages set with utime), the default prune hooks, the
command's flags and summary, and real cargo and Zig builds pruned and built
again where their toolchains are on this machine: a pruned build must
rebuild, never be trusted half-removed."""

from __future__ import annotations

import fcntl
import json
import os
import plistlib
import shutil
import subprocess
import sys
import time
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(HOME, "tests"))
sys.path.insert(0, os.path.join(HOME, "libexec", "lib"))

import prune  # noqa: E402
from test_hecatron import Fixture, write  # noqa: E402

PRUNE = os.path.join(HOME, "libexec", "lib", "prune.py")
DAY = 86400


def age(path: str, days: float) -> None:
    """Make path and everything below it look `days` older."""
    for root, dirs, files in os.walk(path):
        for name in dirs + files:
            p = os.path.join(root, name)
            st = os.lstat(p)
            os.utime(p, (st.st_atime, st.st_mtime - days * DAY), follow_symlinks=False)
    st = os.lstat(path)
    os.utime(path, (st.st_atime, st.st_mtime - days * DAY), follow_symlinks=False)


def blob(path: str, size: int = 4096) -> None:
    write(path, "x" * size)


class ParseTest(unittest.TestCase):
    def test_durations(self):
        self.assertEqual(prune.parse_duration("90s"), 90)
        self.assertEqual(prune.parse_duration("6h"), 6 * 3600)
        self.assertEqual(prune.parse_duration("1h30m"), 5400)
        self.assertEqual(prune.parse_duration("2w"), 14 * DAY)
        self.assertEqual(prune.parse_duration("1.5d"), 1.5 * DAY)
        for bad in ("", "6", "h", "6 h", "-1d", "6y"):
            self.assertIsNone(prune.parse_duration(bad), bad)

    def test_sizes(self):
        self.assertEqual(prune.parse_size("1024"), 1024)
        self.assertEqual(prune.parse_size("500M"), 500 << 20)
        self.assertEqual(prune.parse_size("50G"), 50 << 30)
        self.assertEqual(prune.parse_size("50GiB"), 50 << 30)
        self.assertEqual(prune.parse_size("2gb"), 2 << 30)
        self.assertEqual(prune.parse_size("1.5T"), int(1.5 * (1 << 40)))
        for bad in ("", "G", "50X", "-1G"):
            self.assertIsNone(prune.parse_size(bad), bad)

    def test_human_sizes(self):
        self.assertEqual(prune.human(512), "512 B")
        self.assertEqual(prune.human(1536), "1.5 KiB")
        self.assertEqual(prune.human(69 * (1 << 30)), "69.0 GiB")
        self.assertEqual(prune.human(3 * (1 << 40)), "3072.0 GiB")


class PruneTest(Fixture):
    """prune.py run as the hooks run it, with the flags in the environment."""

    def run_prune(self, *args: str, **env: str) -> str:
        environment = {**os.environ, "HECATRON_LANG": "lang", "HECATRON_ROOT": self.root, **env}
        for name in ("HECATRON_PRUNE_DRY_RUN", "HECATRON_PRUNE_OLDER_THAN", "HECATRON_PRUNE_MAX_SIZE"):
            if name not in env:
                environment.pop(name, None)
        result = subprocess.run([sys.executable, PRUNE, *args], cwd=self.root, env=environment,
                                capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return result.stdout

    def summary(self, output: str) -> tuple[str, int, int]:
        """The summary line's verb, bytes and items."""
        line = [line for line in output.splitlines() if " bytes) " in line][-1]
        verb = "would free" if ": prune: would free " in line else "freed"
        n = int(line.split("(")[1].split(" bytes")[0])
        items = int(line.split(" in ")[1].split(" item")[0])
        return verb, n, items


class CargoTest(PruneTest):
    """A fake target directory: units named by hash, as cargo lays them out."""

    def unit(self, profile: str, package: str, kind: str, h: str, days: float, layout: str = "flat") -> None:
        d = self.path("target", profile)
        if layout == "flat":
            write(os.path.join(d, ".fingerprint", f"{package}-{h}", f"{kind}.json"), "{}")
            blob(os.path.join(d, "deps", f"lib{package}-{h}.rlib"))
            blob(os.path.join(d, "deps", f"{package}-{h}.d"), 10)
            for p in (os.path.join(d, ".fingerprint", f"{package}-{h}"), os.path.join(d, "deps", f"lib{package}-{h}.rlib"),
                      os.path.join(d, "deps", f"{package}-{h}.d")):
                age(p, days)
        else:  # cargo 1.100: build/<pkg>/<hash>/
            u = os.path.join(d, "build", package, h)
            write(os.path.join(u, "fingerprint", f"{kind}.json"), "{}")
            blob(os.path.join(u, "out", "gen.rs"))
            age(u, days)
        write(os.path.join(d, ".cargo-lock"), "")

    def test_keeps_each_groups_builds_within_the_window_of_its_newest(self):
        self.unit("debug", "serde", "lib-serde", "0" * 16, days=10)
        self.unit("debug", "serde", "lib-serde", "1" * 16, days=2)
        self.unit("debug", "serde", "lib-serde", "2" * 16, days=2.1)
        self.unit("debug", "old", "lib-old", "3" * 16, days=30)  # alone in its group: its newest
        self.unit("debug", "it", "test-integration-test-it", "4" * 16, days=9, layout="nested")
        self.unit("debug", "it", "test-integration-test-it", "5" * 16, days=1, layout="nested")
        output = self.run_prune("cargo", self.path("target"))
        self.assertEqual(self.summary(output)[0::2], ("freed", 2), output)
        deps = os.listdir(self.path("target", "debug", "deps"))
        self.assertFalse([n for n in deps if "0" * 16 in n], deps)
        self.assertTrue([n for n in deps if "1" * 16 in n] and [n for n in deps if "2" * 16 in n], deps)
        self.assertTrue([n for n in deps if "3" * 16 in n], deps)
        self.assertEqual(os.listdir(self.path("target", "debug", "build", "it")), ["5" * 16])
        self.assertIn("from target", output)

    def test_a_dry_run_removes_nothing_and_shows_the_largest(self):
        self.unit("debug", "serde", "lib-serde", "0" * 16, days=10)
        self.unit("debug", "serde", "lib-serde", "1" * 16, days=1)
        output = self.run_prune("cargo", self.path("target"), HECATRON_PRUNE_DRY_RUN="1")
        verb, n, items = self.summary(output)
        self.assertEqual((verb, items), ("would free", 1))
        self.assertGreater(n, 4096)
        self.assertIn("lib-serde " + "0" * 16, output)
        self.assertEqual(len(os.listdir(self.path("target", "debug", ".fingerprint"))), 2)

    def test_older_than_sets_the_window(self):
        self.unit("debug", "serde", "lib-serde", "0" * 16, days=3)
        self.unit("debug", "serde", "lib-serde", "1" * 16, days=1)
        self.assertEqual(self.summary(self.run_prune("cargo", self.path("target"),
                                                     HECATRON_PRUNE_OLDER_THAN="3d"))[2], 0)
        self.assertEqual(self.summary(self.run_prune("cargo", self.path("target"),
                                                     HECATRON_PRUNE_OLDER_THAN="1d"))[2], 1)

    def test_max_size_trims_oldest_first_but_never_a_groups_newest(self):
        for i, days in enumerate((5, 4, 3, 2)):
            self.unit("debug", "a", "lib-a", str(i) * 16, days=days + 0.01 * i)
        self.unit("release", "b", "lib-b", "9" * 16, days=20)
        output = self.run_prune("cargo", self.path("target"), HECATRON_PRUNE_OLDER_THAN="30d",
                                HECATRON_PRUNE_MAX_SIZE="1")
        self.assertEqual(self.summary(output)[2], 3, output)
        self.assertEqual(os.listdir(self.path("target", "debug", ".fingerprint")), ["a-" + "3" * 16])
        self.assertEqual(os.listdir(self.path("target", "release", ".fingerprint")), ["b-" + "9" * 16])

    def test_a_profile_cargo_is_building_in_is_skipped(self):
        self.unit("debug", "serde", "lib-serde", "0" * 16, days=10)
        self.unit("debug", "serde", "lib-serde", "1" * 16, days=1)
        fd = os.open(self.path("target", "debug", ".cargo-lock"), os.O_RDONLY)
        try:
            fcntl.flock(fd, fcntl.LOCK_EX)
            output = self.run_prune("cargo", self.path("target"))
        finally:
            os.close(fd)
        self.assertIn("cargo is building in target/debug; skipped it", output)
        self.assertEqual(self.summary(output)[2], 0)
        self.assertEqual(len(os.listdir(self.path("target", "debug", ".fingerprint"))), 2)

    def test_tracked_files_stay(self):
        self.unit("debug", "serde", "lib-serde", "0" * 16, days=10)
        self.unit("debug", "serde", "lib-serde", "1" * 16, days=1)
        subprocess.run(["git", "add", "-f", "target/debug/deps"], cwd=self.root, check=True)
        output = self.run_prune("cargo", self.path("target"))
        self.assertEqual(self.summary(output)[2], 0, output)
        self.assertEqual(len(os.listdir(self.path("target", "debug", ".fingerprint"))), 2)

    def test_no_target_directory(self):
        output = self.run_prune("cargo", self.path("target"))
        self.assertEqual(self.summary(output), ("freed", 0, 0))


class ZigTest(PruneTest):
    def cache(self, name: str, days: float) -> None:
        write(self.path(name, "h", "timestamp"), "")
        write(self.path(name, "h", "0123.txt"), "manifest")
        blob(self.path(name, "o", "abcd", "main.o"))
        age(self.path(name), days)

    def test_a_cache_goes_whole_once_unused(self):
        self.cache(".zig-cache", 10)
        self.cache("zig-cache", 1)
        output = self.run_prune("zig", self.path(".zig-cache"), self.path("zig-cache"))
        self.assertEqual(self.summary(output)[2], 1)
        self.assertFalse(os.path.exists(self.path(".zig-cache")))
        self.assertTrue(os.path.exists(self.path("zig-cache", "o", "abcd", "main.o")))

    def test_a_cache_a_build_uses_is_skipped(self):
        self.cache(".zig-cache", 10)
        fd = os.open(self.path(".zig-cache", "h", "0123.txt"), os.O_RDONLY)
        try:
            fcntl.flock(fd, fcntl.LOCK_SH)
            output = self.run_prune("zig", self.path(".zig-cache"))
        finally:
            os.close(fd)
        self.assertIn("a Zig build is using .zig-cache; skipped it", output)
        self.assertTrue(os.path.exists(self.path(".zig-cache")))

    def test_max_size_removes_the_oldest_cache_first(self):
        self.cache(".zig-cache", 2)
        self.cache("zig-cache", 3)
        output = self.run_prune("zig", self.path(".zig-cache"), self.path("zig-cache"), HECATRON_PRUNE_MAX_SIZE="10K")
        self.assertEqual(self.summary(output)[2], 1, output)
        self.assertTrue(os.path.exists(self.path(".zig-cache")))
        self.assertFalse(os.path.exists(self.path("zig-cache")))


class SwiftPMTest(PruneTest):
    def test_old_configurations_go_and_the_dependencies_stay(self):
        build = self.path(".build")
        blob(os.path.join(build, "arm64-apple-macosx", "debug", "App.build", "main.o"))
        blob(os.path.join(build, "arm64-apple-macosx", "release", "App.build", "main.o"))
        blob(os.path.join(build, "checkouts", "dep", "Sources", "x.swift"))
        blob(os.path.join(build, "index-build", "arm64-apple-macosx", "debug", "x.o"))
        age(build, 30)
        os.utime(os.path.join(build, "arm64-apple-macosx", "debug", "App.build", "main.o"))
        output = self.run_prune("swiftpm", build)
        self.assertEqual(self.summary(output)[2], 1, output)
        self.assertFalse(os.path.exists(os.path.join(build, "arm64-apple-macosx", "release")))
        for kept in ("arm64-apple-macosx/debug", "checkouts/dep", "index-build"):
            self.assertTrue(os.path.exists(os.path.join(build, kept)), kept)

    def test_a_build_in_progress_is_skipped(self):
        build = self.path(".build")
        blob(os.path.join(build, "arm64-apple-macosx", "release", "x.o"))
        write(os.path.join(build, ".lock"), "")
        age(build, 30)
        fd = os.open(os.path.join(build, ".lock"), os.O_RDONLY)
        try:
            fcntl.flock(fd, fcntl.LOCK_EX)
            output = self.run_prune("swiftpm", build)
        finally:
            os.close(fd)
        self.assertIn("SwiftPM is building in .build; skipped it", output)
        self.assertTrue(os.path.exists(os.path.join(build, "arm64-apple-macosx", "release")))


class DerivedDataTest(PruneTest):
    def folder(self, name: str, workspace: str) -> str:
        d = self.path("DerivedData", name)
        os.makedirs(os.path.join(d, "Build"))
        blob(os.path.join(d, "Build", "x.o"))
        with open(os.path.join(d, "info.plist"), "wb") as f:
            plistlib.dump({"WorkspacePath": workspace}, f)
        age(d, 1)
        return d

    def test_a_folder_whose_workspace_is_gone_goes(self):
        gone = self.folder("Gone-abc", self.path("deleted", "Gone.xcodeproj"))
        os.makedirs(self.path("App.xcodeproj"))
        here = self.folder("App-def", self.path("App.xcodeproj"))
        output = self.run_prune("derived-data", self.path("DerivedData"))
        self.assertEqual(self.summary(output)[2], 1)
        self.assertFalse(os.path.exists(gone))
        self.assertTrue(os.path.exists(here))


class DotnetTest(PruneTest):
    def test_configurations_not_built_lately_go_but_never_the_last_built(self):
        write(self.path("src", "App", "App.csproj"), "<Project/>")
        for config in ("Debug", "Release"):
            blob(self.path("src", "App", "bin", config, "net8.0", "App.dll"))
            blob(self.path("src", "App", "obj", config, "net8.0", "App.dll"))
        write(self.path("src", "App", "obj", "project.assets.json"), "{}")
        write(self.path("src", "Old", "Old.fsproj"), "<Project/>")
        blob(self.path("src", "Old", "bin", "Release", "Old.dll"))
        age(self.path("src"), 30)
        os.utime(self.path("src", "App", "bin", "Debug", "net8.0", "App.dll"))
        output = self.run_prune("dotnet", self.root)
        self.assertEqual(self.summary(output)[2], 1, output)
        self.assertFalse(os.path.exists(self.path("src", "App", "bin", "Release")))
        self.assertFalse(os.path.exists(self.path("src", "App", "obj", "Release")))
        for kept in ("App/bin/Debug", "App/obj/Debug", "App/obj/project.assets.json", "Old/bin/Release"):
            self.assertTrue(os.path.exists(self.path("src", kept)), kept)


class NuGetTest(PruneTest):
    def version(self, package: str, version: str, days: float) -> None:
        d = self.path("packages", package, version)
        blob(os.path.join(d, f"{package}.{version}.nupkg"))
        write(os.path.join(d, ".nupkg.metadata"), "{}")
        age(d, days)

    def test_old_versions_go_and_each_packages_newest_stays(self):
        self.version("newtonsoft.json", "12.0.1", 90)
        self.version("newtonsoft.json", "13.0.3", 60)
        self.version("newtonsoft.json", "13.0.1", 5)
        self.version("xunit", "2.4.0", 100)
        output = self.run_prune("nuget", self.path("packages"))
        self.assertEqual(self.summary(output)[2], 1, output)
        self.assertEqual(sorted(os.listdir(self.path("packages", "newtonsoft.json"))), ["13.0.1", "13.0.3"])
        self.assertEqual(os.listdir(self.path("packages", "xunit")), ["2.4.0"])


class GradleTest(PruneTest):
    def test_project_caches_of_versions_no_longer_used_go(self):
        write(self.path("gradle", "wrapper", "gradle-wrapper.properties"),
              "distributionUrl=https\\://services.gradle.org/distributions/gradle-8.10.2-bin.zip\n")
        for version, days in (("8.5", 30), ("8.10.2", 30), ("9.0", 1)):
            blob(self.path(".gradle", version, "fileHashes", "fileHashes.bin"))
            age(self.path(".gradle", version), days)
        blob(self.path(".gradle", "buildOutputCleanup", "x.bin"))
        age(self.path(".gradle", "buildOutputCleanup"), 30)
        output = self.run_prune("gradle", self.root)
        self.assertEqual(self.summary(output)[2], 1, output)
        self.assertEqual(sorted(os.listdir(self.path(".gradle"))), ["8.10.2", "9.0", "buildOutputCleanup"])


class ToolTest(PruneTest):
    def test_says_how_much_the_tools_trim_freed(self):
        blob(self.path("cache", "a"), 8192)
        output = self.run_prune("tool", self.path("cache"), "--", "rm", self.path("cache", "a"))
        self.assertIn("lang: prune: freed 8.0 KiB (8192 bytes) from cache", output)
        output = self.run_prune("tool", self.path("cache"), "--", "false", HECATRON_PRUNE_DRY_RUN="1")
        self.assertIn("would run false on cache", output)


class CommandTest(Fixture):
    """hecatron prune through bin/hecatron (the scripts: the Go tests compare
    the binary with them)."""

    def setUp(self):
        super().setUp()
        self.bin = self.path(".fakebin")
        os.makedirs(self.bin)

    def run_hct(self, *args: str, **env: str) -> subprocess.CompletedProcess:
        environment = {**os.environ, "HECATRON_NO_GO": "1", **env}
        for name in ("HECATRON_OUTPUT", "HECATRON_PRUNE_DRY_RUN", "HECATRON_PRUNE_CACHES"):
            environment.pop(name, None)
        return subprocess.run(["bash", os.path.join(HOME, "bin", "hecatron"), *args], cwd=self.root,
                              env=environment, capture_output=True, text=True, stdin=subprocess.DEVNULL)

    def test_bad_flags_are_refused(self):
        self.configure(languages=["go"], implPath=".")
        result = self.run_hct("prune", "--older-than", "soon")
        self.assertEqual(result.returncode, 2)
        self.assertIn("--older-than takes a duration such as 6h, 7d or 2w (got 'soon')", result.stderr)
        result = self.run_hct("prune", "--max-size=lots")
        self.assertEqual(result.returncode, 2)
        self.assertIn("--max-size takes a size such as 500M or 50G (got 'lots')", result.stderr)

    def test_flags_reach_the_hooks(self):
        self.configure(languages=["go"], implPath=".")
        write(self.path(".hecatron", "languages", "go", "prune.sh"),
              'echo "dry=${HECATRON_PRUNE_DRY_RUN:-} age=${HECATRON_PRUNE_OLDER_THAN:-} '
              'size=${HECATRON_PRUNE_MAX_SIZE:-} caches=${HECATRON_PRUNE_CACHES:-}"\n')
        result = self.run_hct("prune", "go", "--dry-run", "--older-than", "2d", "--max-size=5G", "--caches")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("dry=1 age=2d size=5G caches=1", result.stdout)

    def test_go_says_its_toolchain_trims_its_cache(self):
        self.configure(languages=["go"], implPath=".")
        write(self.path("go.mod"), "module x\n")
        result = self.run_hct("prune", "go")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("trims its build cache (GOCACHE) itself", result.stdout)
        self.assertIn("go: prune skipped for 1 component(s)", result.stdout)

    def test_a_language_without_a_prune_hook_is_skipped(self):
        self.configure(languages=["ruby"], implPath=".")
        result = self.run_hct("prune", "ruby")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("ruby: prune skipped: no prune hook for ruby (hecatron clean ruby removes its build artifacts)",
                      result.stdout)

    def test_a_parallel_run_ends_with_what_each_freed_and_the_total(self):
        self.configure(languages=["go", "python"], implPath=".")
        for lang, n in (("go", 1024), ("python", 2048)):
            write(self.path(".hecatron", "languages", lang, "prune.sh"),
                  f'echo "{lang}: prune: would free 1.0 KiB ({n} bytes) in 1 item from {lang}-cache"\n')
        result = self.run_hct("prune", "--dry-run")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("go: prune: would free 1.0 KiB (1024 bytes) in 1 item from go-cache\n"
                      "python: prune: would free 1.0 KiB (2048 bytes) in 1 item from python-cache\n"
                      "prune: would free 3.0 KiB in all\n", result.stdout)


class DefaultHookTest(Fixture):
    """The default prune hooks' choices, each tool a fake on PATH."""

    def setUp(self):
        super().setUp()
        self.bin = self.path(".fakebin")
        os.makedirs(self.bin)

    def fake(self, tool: str, body: str = "") -> None:
        write(os.path.join(self.bin, tool), f'#!/bin/sh\n{body}\necho "{tool} $*"\n', 0o755)

    def hook(self, lang: str, **env: str) -> subprocess.CompletedProcess:
        environment = {**os.environ, "PATH": self.bin + os.pathsep + "/usr/bin:/bin", "HECATRON_LANG": lang,
                       "HECATRON_ROOT": self.root, "HECATRON_HOME": "", "HOME": self.path("home"), **env}
        for name in ("HECATRON_PRUNE_DRY_RUN", "HECATRON_PRUNE_CACHES", "HECATRON_PRUNE_OLDER_THAN"):
            if name not in env:
                environment.pop(name, None)
        result = subprocess.run(["bash", os.path.join(HOME, "languages", lang, "hooks", "prune.sh")], cwd=self.root,
                                env=environment, capture_output=True, text=True, stdin=subprocess.DEVNULL)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return result

    def test_python_and_node_have_nothing_in_the_project(self):
        self.assertIn("--caches trims uv's cache (skipping)", self.hook("python").stdout)
        self.assertIn("--caches trims npm's cache (skipping)", self.hook("javascript").stdout)

    def test_caches_runs_the_tools_own_trim(self):
        blob(self.path("home", "npm-cache", "x"))
        self.fake("npm", f'[ "$1" = config ] && {{ echo "{self.path("home", "npm-cache")}"; exit 0; }}')
        self.fake("uv", f'[ "$1 $2" = "cache dir" ] && {{ echo "{self.path("home", "uv-cache")}"; exit 0; }}')
        output = self.hook("typescript", HECATRON_PRUNE_CACHES="1").stdout
        self.assertIn("npm cache verify", output)
        self.assertIn("typescript: prune: freed 0 B (0 bytes) from home/npm-cache", output)
        output = self.hook("python", HECATRON_PRUNE_CACHES="1", HECATRON_PRUNE_DRY_RUN="1").stdout
        self.assertIn("python: prune: would run uv cache prune on home/uv-cache", output)
        self.assertNotIn("uv cache prune\n", output)

    def test_dotnet_prunes_configurations_and_with_caches_nuget(self):
        write(self.path("App.csproj"), "<Project/>")
        blob(self.path("bin", "Release", "App.dll"))
        blob(self.path("bin", "Debug", "App.dll"))
        age(self.path("bin", "Release"), 30)
        for version, days in (("1.0.0", 90), ("2.0.0", 60)):
            blob(self.path("home", ".nuget", "packages", "dep", version, "dep.nupkg"))
            age(self.path("home", ".nuget", "packages", "dep", version), days)
        output = self.hook("csharp", HECATRON_PRUNE_CACHES="1").stdout
        self.assertIn("csharp: prune: freed 4.0 KiB (4096 bytes) in 1 item from .\n", output)
        self.assertIn("in 1 item from home/.nuget/packages", output)
        self.assertEqual(os.listdir(self.path("bin")), ["Debug"])
        self.assertEqual(os.listdir(self.path("home", ".nuget", "packages", "dep")), ["2.0.0"])

    def test_gradle_prunes_its_project_caches_and_maven_skips(self):
        write(self.path("settings.gradle"), "")
        for version, days in (("7.6", 30), ("8.10", 1)):
            blob(self.path(".gradle", version, "x.bin"))
            age(self.path(".gradle", version), days)
        os.makedirs(self.path("app", "src"))
        result = subprocess.run(["bash", os.path.join(HOME, "languages", "kotlin", "hooks", "prune.sh")],
                                cwd=self.path("app"), capture_output=True, text=True, stdin=subprocess.DEVNULL,
                                env={**os.environ, "HECATRON_LANG": "kotlin", "HECATRON_ROOT": self.root,
                                     "HECATRON_HOME": ""})
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("kotlin: prune: freed 4.0 KiB (4096 bytes) in 1 item from .gradle", result.stdout)
        self.assertEqual(os.listdir(self.path(".gradle")), ["8.10"])
        os.remove(self.path("settings.gradle"))
        write(self.path("pom.xml"), "<project/>")
        self.assertIn("Maven's and sbt's builds keep nothing for prune", self.hook("java").stdout)

    def test_swift_prunes_the_packages_build(self):
        write(self.path("Package.swift"), "// swift-tools-version:5.9\n")
        blob(self.path(".build", "x86_64-unknown-linux-gnu", "release", "x.o"))
        blob(self.path(".build", "checkouts", "dep", "x.swift"))
        age(self.path(".build"), 30)
        output = self.hook("swift").stdout
        self.assertIn("swift: prune: freed 4.0 KiB (4096 bytes) in 1 item from .build", output)
        self.assertTrue(os.path.exists(self.path(".build", "checkouts", "dep", "x.swift")))


@unittest.skipUnless(shutil.which("cargo"), "no cargo")
class CargoBuildTest(Fixture):
    """A small workspace (no dependencies, offline) built for real: a lib
    with a build script whose OUT_DIR file it includes, its integration
    test, and a bin using it. An old variant (another feature set, aged)
    is pruned with its fingerprint and build-script output; building it
    again rebuilds it."""

    def setUp(self):
        super().setUp()
        src = self.path("ws")
        write(os.path.join(src, "Cargo.toml"), '[workspace]\nmembers = ["tlib", "tbin"]\nresolver = "2"\n')
        write(os.path.join(src, "tlib", "Cargo.toml"), '[package]\nname = "tlib"\nversion = "0.1.0"\nedition = "2021"\n'
              '[features]\ndefault = []\nextra = []\n')
        write(os.path.join(src, "tlib", "build.rs"), 'fn main() {\n    let out = std::env::var("OUT_DIR").unwrap();\n'
              '    std::fs::write(format!("{out}/gen.rs"), "pub fn gen() -> u32 { 7 }").unwrap();\n}\n')
        write(os.path.join(src, "tlib", "src", "lib.rs"), 'include!(concat!(env!("OUT_DIR"), "/gen.rs"));\n'
              '#[cfg(feature = "extra")]\npub fn extra() -> u32 { 1 }\n')
        write(os.path.join(src, "tlib", "tests", "it.rs"), "#[test]\nfn gen_is_seven() { assert_eq!(tlib::gen(), 7); }\n")
        write(os.path.join(src, "tbin", "Cargo.toml"), '[package]\nname = "tbin"\nversion = "0.1.0"\nedition = "2021"\n'
              '[dependencies]\ntlib = { path = "../tlib" }\n')
        write(os.path.join(src, "tbin", "src", "main.rs"), "fn main() { println!(\"{}\", tlib::gen()); }\n")
        self.src = src
        # Outside the workspace, as CARGO_TARGET_DIR: the hook finds it as cargo does.
        self.target = self.path("shared-target")

    def env(self) -> dict:
        env = {**os.environ, "CARGO_TARGET_DIR": self.target, "CARGO_INCREMENTAL": "0", "HECATRON_NO_GO": "1"}
        env.pop("RUSTC_WRAPPER", None)
        for name in ("HECATRON_OUTPUT", "HECATRON_PRUNE_DRY_RUN", "HECATRON_PRUNE_OLDER_THAN", "HECATRON_PRUNE_MAX_SIZE"):
            env.pop(name, None)
        return env

    def cargo(self, *args: str) -> subprocess.CompletedProcess:
        return subprocess.run(["cargo", *args, "--offline"], cwd=self.src, env=self.env(), capture_output=True,
                              text=True)

    def hct_prune(self, *args: str) -> str:
        result = subprocess.run(["bash", os.path.join(HOME, "bin", "hecatron"), "prune", "rust", *args],
                                cwd=self.src, env=self.env(), capture_output=True, text=True, stdin=subprocess.DEVNULL)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return result.stdout

    def fingerprints(self, package: str) -> list[str]:
        """package's units, as <package>-<hash>, in either layout."""
        debug = os.path.join(self.target, "debug")
        found = set()
        fp = os.path.join(debug, ".fingerprint")
        if os.path.isdir(fp):
            found |= {n for n in os.listdir(fp) if n.startswith(package + "-")}
        b = os.path.join(debug, "build", package)
        if os.path.isdir(b):
            found |= {f"{package}-{h}" for h in os.listdir(b) if os.path.isdir(os.path.join(b, h, "fingerprint"))}
        return sorted(found)

    def named_with(self, hashes: set[str]) -> list[str]:
        out = []
        for root, dirs, files in os.walk(os.path.join(self.target, "debug")):
            out += [os.path.join(root, n) for n in dirs + files if any(h in n for h in hashes)]
        return out

    def test_an_old_variant_is_pruned_and_rebuilds(self):
        self.configure(languages=["rust"], implPath="ws")
        r = self.cargo("test", "--workspace", "--no-run")
        self.assertEqual(r.returncode, 0, r.stderr[-2000:])
        before = self.fingerprints("tlib")
        age(self.target, 2)
        # The same targets with another feature set: new hashes for tlib's units.
        r = self.cargo("test", "-p", "tlib", "--features", "extra", "--no-run")
        self.assertEqual(r.returncode, 0, r.stderr[-2000:])
        both = self.fingerprints("tlib")
        self.assertGreater(len(both), len(before))

        output = self.hct_prune("--dry-run")
        self.assertIn("rust: prune: would free", output)
        self.assertIn("from shared-target", output)
        self.assertEqual(self.fingerprints("tlib"), both)

        output = self.hct_prune()
        self.assertIn("rust: prune: freed", output)
        after = self.fingerprints("tlib")
        self.assertLessEqual(set(both) - set(before), set(after), "the new variant's units stay")
        gone = [n for n in before if n not in after]
        self.assertTrue(gone, f"the aged variant's units go: {before} -> {after}")
        self.assertEqual(self.named_with({n.rsplit("-", 1)[1] for n in gone}), [])

        # The pruned variant is needed again: cargo rebuilds it.
        r = self.cargo("test", "--workspace")
        self.assertEqual(r.returncode, 0, r.stderr[-3000:])
        self.assertIn("Compiling tlib", r.stderr)
        r = self.cargo("run", "-q", "-p", "tbin")
        self.assertEqual((r.returncode, r.stdout.strip()), (0, "7"), r.stderr[-1000:])

        # Builds within the window stay.
        self.assertIn(" in 0 items from shared-target", self.hct_prune())


@unittest.skipUnless(shutil.which("zig"), "no zig")
class ZigBuildTest(Fixture):
    def test_a_pruned_cache_rebuilds(self):
        project = self.path("z")
        os.makedirs(project)
        env = {**os.environ, "HECATRON_NO_GO": "1"}
        env.pop("HECATRON_OUTPUT", None)
        env.pop("ZIG_LOCAL_CACHE_DIR", None)
        r = subprocess.run(["zig", "init", "--minimal"], cwd=project, env=env, capture_output=True, text=True)
        if r.returncode != 0:
            r = subprocess.run(["zig", "init"], cwd=project, env=env, capture_output=True, text=True)
        self.assertEqual(r.returncode, 0, r.stderr)
        r = subprocess.run(["zig", "build"], cwd=project, env=env, capture_output=True, text=True)
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertTrue(os.path.isdir(os.path.join(project, ".zig-cache")))
        self.configure(languages=["zig"], implPath="z")
        age(os.path.join(project, ".zig-cache"), 30)
        r = subprocess.run(["bash", os.path.join(HOME, "bin", "hecatron"), "prune", "zig"], cwd=self.root, env=env,
                           capture_output=True, text=True, stdin=subprocess.DEVNULL)
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn("zig: prune: freed", r.stdout)
        self.assertFalse(os.path.exists(os.path.join(project, ".zig-cache")))
        r = subprocess.run(["zig", "build"], cwd=project, env=env, capture_output=True, text=True)
        self.assertEqual(r.returncode, 0, r.stderr)


if __name__ == "__main__":
    unittest.main()
