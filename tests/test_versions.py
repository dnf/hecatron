"""Tests for toolchain versions (docs/design.md §18): the version syntax, pin
files, resolution per component and the per-language environment.

    python3 -m unittest discover -s tests
"""

from __future__ import annotations

import json
import os
import shlex
import subprocess
import sys
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(HOME, "libexec", "lib"))
sys.path.insert(0, os.path.join(HOME, "tests"))

import project  # noqa: E402
import versions  # noqa: E402
from test_hecatron import BIN, Fixture, write  # noqa: E402


class SyntaxTest(unittest.TestCase):
    def test_a_prefix_matches_by_whole_components(self):
        spec = versions.Spec("3.12")
        self.assertTrue(spec.matches("3.12"))
        self.assertTrue(spec.matches("3.12.7"))
        self.assertFalse(spec.matches("3.120"))
        self.assertFalse(spec.matches("3.1"))

    def test_an_exact_version_never_matches_a_longer_one(self):
        spec = versions.Spec("=4.5")
        self.assertTrue(spec.exact)
        self.assertTrue(spec.matches("4.5"))
        self.assertFalse(spec.matches("4.5.1"))

    def test_build_metadata_and_a_leading_v_are_ignored(self):
        self.assertTrue(versions.Spec("=21.0.5").matches("21.0.5+11"))
        self.assertTrue(versions.Spec("20").matches("v20.11.0"))

    def test_a_pre_release_is_not_its_release(self):
        self.assertFalse(versions.Spec("=1.2.0").matches("1.2.0-rc1"))

    def test_newest_orders_numerically_and_puts_pre_releases_first(self):
        self.assertEqual(versions.newest(["1.9", "1.10", "1.10-rc1"]), "1.10")
        self.assertEqual(versions.newest([]), None)


class PinFormatTest(unittest.TestCase):
    def read(self, fmt: str, text: str, key: str | None = None):
        return versions.read_pin(fmt, text, key)

    def test_plain_files_give_their_version(self):
        self.assertEqual(self.read("python-version", "3.12\n"), "3.12")
        self.assertEqual(self.read("node-version", "v20.11.0\n"), "20.11.0")
        self.assertEqual(self.read("nvmrc", "# comment\n20\n"), "20")
        self.assertEqual(self.read("ruby-version", "ruby-3.3.0\n"), "3.3.0")
        self.assertEqual(self.read("swift-version", "5.10\n"), "5.10")
        self.assertEqual(self.read("java-version", "temurin64-21.0.2\n"), "21.0.2")

    def test_a_named_channel_has_no_version(self):
        self.assertIsNone(self.read("nvmrc", "lts/iron\n"))
        self.assertIsNone(self.read("python-version", "system\n"))
        self.assertIsNone(self.read("rust-toolchain", "stable\n"))

    def test_rust_toolchain_reads_both_layouts(self):
        self.assertEqual(self.read("rust-toolchain", "1.80.1\n"), "=1.80.1")
        toml = '[toolchain]\nchannel = "1.80.1"\ncomponents = ["clippy"]\n'
        self.assertEqual(self.read("rust-toolchain-toml", toml), "=1.80.1")
        self.assertEqual(self.read("rust-toolchain", toml), "=1.80.1")
        self.assertIsNone(self.read("rust-toolchain-toml", '[toolchain]\nchannel = "nightly"\n'))

    def test_global_json_reads_the_sdk_version(self):
        self.assertEqual(self.read("global-json", '{"sdk": {"version": "8.0.100"}}'), "8.0.100")
        self.assertEqual(
            self.read("global-json", '{"sdk": {"version": "8.0.100", "rollForward": "disable"}}'), "=8.0.100")
        self.assertIs(self.read("global-json", '{"msbuild-sdks": {}}'), versions.NOT_PINNED)

    def test_go_mod_pins_only_with_a_toolchain_line(self):
        self.assertEqual(self.read("go-mod-toolchain", "module m\n\ngo 1.22\n\ntoolchain go1.22.3\n"), "=1.22.3")
        self.assertIs(self.read("go-mod-toolchain", "module m\n\ngo 1.22\n"), versions.NOT_PINNED)

    def test_gradle_daemon_jvm_gives_the_jdk(self):
        self.assertEqual(self.read("gradle-daemon-jvm", "toolchainVendor=ADOPTIUM\ntoolchainVersion=17\n"), "17")

    def test_multi_language_files_need_their_key(self):
        tool_versions = "nodejs 20.11.0 18.19.0\npython 3.12.1\n"
        self.assertEqual(self.read("tool-versions", tool_versions, "nodejs"), "20.11.0")
        self.assertIs(self.read("tool-versions", tool_versions, "ruby"), versions.NOT_PINNED)
        self.assertIsNone(self.read("tool-versions", "nodejs latest\n", "nodejs"))
        mise = '[env]\nX = "1"\n\n[tools]\nnode = "20"\npython = ["3.12", "3.11"]\njava = { version = "21" }\n'
        self.assertEqual(self.read("mise", mise, "node"), "20")
        self.assertEqual(self.read("mise", mise, "python"), "3.12")
        self.assertEqual(self.read("mise", mise, "java"), "21")
        self.assertIs(self.read("mise", mise, "go"), versions.NOT_PINNED)
        self.assertEqual(self.read("sdkmanrc", "java=21.0.2-tem\ngradle=8.5\n"), "21.0.2")
        self.assertEqual(self.read("sdkmanrc", "java=21.0.2-tem\ngradle=8.5\n", "gradle"), "8.5")

    def test_an_unknown_format_is_an_error(self):
        with self.assertRaises(ValueError):
            self.read("no-such-format", "1.0")


class VersionFixture(Fixture):
    """A project with components a and b, implementing a language `tool` that
    hecatron "ships" (a built-in definition in a temporary directory)."""

    def setUp(self) -> None:
        super().setUp()
        self.builtin = self.path("builtin-languages")
        self._saved = project.BUILTIN_LANGUAGES
        project.BUILTIN_LANGUAGES = self.builtin
        self.configure(languages=["tool"], components=["a", "b"], implPath="{component}/{lang}")
        for component in ("a", "b"):
            os.makedirs(self.path(component, "tool"))
        self.define(version="=2.0.0", pinFiles=[{"file": ".tool-version", "format": "python-version"}])

    def tearDown(self) -> None:
        project.BUILTIN_LANGUAGES = self._saved
        super().tearDown()

    def define(self, **config) -> None:
        write(os.path.join(self.builtin, "tool", "config.json"), json.dumps(config))

    def override(self, **config) -> None:
        write(self.path(".hecatron", "languages", "tool", "config.json"), json.dumps(config))

    def installed(self, *lines: str) -> None:
        """A built-in installed.sh printing these version<TAB>home lines."""
        body = "".join("printf '%s\\t%s\\n' {} {}\n".format(*map(shlex.quote, line.split("\t")))
                       for line in lines)
        write(os.path.join(self.builtin, "tool", "installed.sh"), f"#!/usr/bin/env bash\n{body}", 0o755)

    def load(self) -> project.Project:
        return project.load(self.root)


class ResolutionTest(VersionFixture):
    def test_the_default_applies_without_a_pin_or_project_version(self):
        self.assertEqual(self.load().resolve_version("tool", "a"),
                         {"spec": "=2.0.0", "source": "default", "exact": True})

    def test_a_default_must_be_exact(self):
        self.define(version="2.0")
        with self.assertRaises(project.ProjectError):
            self.load().resolve_version("tool", "a")

    def test_the_project_version_beats_the_default(self):
        self.override(version="1.9")
        self.assertEqual(self.load().resolve_version("tool", "a"),
                         {"spec": "1.9", "source": "project", "exact": False})

    def test_the_nearest_pin_file_wins_per_component(self):
        self.override(version="1.9")
        write(self.path(".tool-version"), "1.7\n")
        write(self.path("b", "tool", ".tool-version"), "1.8\n")
        p = self.load()
        self.assertEqual(p.resolve_version("tool", "a")["spec"], "1.7")
        self.assertEqual(p.resolve_version("tool", "a")["source"], "pin:.tool-version")
        self.assertEqual(p.resolve_version("tool", "b")["spec"], "1.8")
        self.assertEqual(p.resolve_version("tool", "b")["source"], "pin:b/tool/.tool-version")

    def test_a_pin_without_a_version_resolves_to_none(self):
        write(self.path("a", "tool", ".tool-version"), "system\n")
        self.assertEqual(self.load().resolve_version("tool", "a"),
                         {"spec": None, "source": "pin:a/tool/.tool-version", "exact": False})

    def test_a_project_can_add_pin_files(self):
        self.override(pinFiles=[{"file": "tool.txt", "format": "python-version"}])
        write(self.path("a", "tool", "tool.txt"), "1.5\n")
        self.assertEqual(self.load().resolve_version("tool", "a")["spec"], "1.5")

    def test_versions_lists_each_distinct_spec_with_its_sources(self):
        write(self.path("a", "tool", ".tool-version"), "1.8\n")
        self.assertEqual(self.load().versions("tool"),
                         [("1.8", ["pin:a/tool/.tool-version"]), ("=2.0.0", ["default"])])

    def test_check_reports_a_pin_that_disagrees_with_the_project(self):
        self.override(version="1.9")
        write(self.path("a", "tool", ".tool-version"), "1.8\n")
        self.assertEqual(self.load().version_conflicts(),
                         [("tool", "a", "pin:a/tool/.tool-version", "1.8", "1.9")])


class CheckTest(VersionFixture):
    def test_check_fails_on_a_pin_that_disagrees_with_the_project(self):
        self.override(version="1.9")
        write(self.path("a", "tool", ".tool-version"), "1.8\n")
        env = dict(os.environ, HECATRON_BUILTIN_LANGUAGES=self.builtin)
        result = subprocess.run(["bash", BIN, "check"], cwd=self.root, capture_output=True, text=True, env=env)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("a/tool/.tool-version pins 1.8, but the project's version is 1.9", result.stdout)


class EnvironmentTest(VersionFixture):
    def test_the_newest_installed_match_is_used(self):
        self.installed("1.9.0\t/opt/tool-1.9.0", "2.0.0\t/opt/tool-2.0.0", "1.9.3\t/opt/tool-1.9.3")
        self.override(version="1.9")
        self.assertEqual(self.load().installed_version("tool", "a"), ("1.9.3", "/opt/tool-1.9.3"))

    def test_nothing_installed_matches(self):
        self.installed("3.0.0\t/opt/tool-3")
        self.assertIsNone(self.load().installed_version("tool", "a"))

    def test_a_failing_installed_script_means_nothing_is_installed(self):
        write(os.path.join(self.builtin, "tool", "installed.sh"), "#!/usr/bin/env bash\nexit 3\n", 0o755)
        self.assertIsNone(self.load().installed_version("tool", "a"))

    def test_env_expands_home_and_version_placeholders(self):
        home = self.path("opt", "tool-2.0.0")
        os.makedirs(os.path.join(home, "bin"))
        self.installed(f"2.0.0\t{home}")
        self.define(version="=2.0.0", path=["{home}/bin"],
                    env={"TOOL_HOME": "{home}", "TOOL_MAJOR": "{major}", "TOOL_V": "{version}"})
        self.assertEqual(self.load().language_env("tool", "a"),
                         {"path": [os.path.join(home, "bin")],
                          "vars": {"TOOL_HOME": home, "TOOL_MAJOR": "2", "TOOL_V": "2.0.0"}})

    def test_an_unresolvable_version_keeps_only_static_path_entries(self):
        static = self.path("static-bin")
        os.makedirs(static)
        self.define(version="=2.0.0", path=["{home}/bin", static], env={"TOOL_HOME": "{home}"})
        self.assertEqual(self.load().language_env("tool", "a"), {"path": [static], "vars": {}})

    def test_hooks_run_with_their_components_environment(self):
        for component, version in (("a", "1.0.0"), ("b", "2.0.0")):
            os.makedirs(self.path("opt", f"tool-{version}"))
        self.installed(f"1.0.0\t{self.path('opt', 'tool-1.0.0')}", f"2.0.0\t{self.path('opt', 'tool-2.0.0')}")
        self.define(version="=2.0.0", dependency="bash",
                    pinFiles=[{"file": ".tool-version", "format": "python-version"}],
                    env={"TOOL_VERSION": "{version}"})
        write(self.path("a", "tool", ".tool-version"), "1.0\n")
        write(self.path(".hecatron", "languages", "tool", "test.sh"),
              '#!/usr/bin/env bash\necho "$1 runs $TOOL_VERSION"\n', 0o755)
        result = self.run_hecatron("test", "tool")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("a runs 1.0.0", result.stdout)
        self.assertIn("b runs 2.0.0", result.stdout)

    def test_env_command_prints_exports(self):
        os.makedirs(self.path("opt", "tool-2.0.0", "bin"))
        self.installed(f"2.0.0\t{self.path('opt', 'tool-2.0.0')}")
        self.define(version="=2.0.0", path=["{home}/bin"], env={"TOOL_HOME": "{home}"})
        result = self.run_hecatron("env", "tool", "-c", "a")
        self.assertEqual(result.returncode, 0, result.stderr)
        script = f'{result.stdout}\necho "$TOOL_HOME"; echo "$PATH"'
        out = subprocess.run(["bash", "-c", script], capture_output=True, text=True).stdout.splitlines()
        self.assertEqual(out[0], self.path("opt", "tool-2.0.0"))
        self.assertTrue(out[1].startswith(self.path("opt", "tool-2.0.0", "bin") + ":"))

    def run_hecatron(self, *args: str) -> subprocess.CompletedProcess:
        """The CLI with this fixture's built-in definitions."""
        env = dict(os.environ, HECATRON_BUILTIN_LANGUAGES=self.builtin)
        return subprocess.run(["bash", BIN, *args], cwd=self.root, capture_output=True, text=True, env=env)


class ShippedDefinitionsTest(unittest.TestCase):
    def test_every_shipped_default_version_is_exact(self):
        languages = os.path.join(HOME, "languages")
        for lang in sorted(os.listdir(languages)):
            path = os.path.join(languages, lang, "config.json")
            if not os.path.isfile(path):
                continue
            with open(path, encoding="utf-8") as f:
                version = json.load(f).get("version")
            if version is not None:
                self.assertTrue(str(version).startswith("="), f"{lang}: default version {version!r} isn't exact")

    def test_every_shipped_pin_format_exists(self):
        languages = os.path.join(HOME, "languages")
        for lang in sorted(os.listdir(languages)):
            path = os.path.join(languages, lang, "config.json")
            if not os.path.isfile(path):
                continue
            with open(path, encoding="utf-8") as f:
                for entry in json.load(f).get("pinFiles", []):
                    self.assertIn(entry["format"], versions.FORMATS, f"{lang}: {entry}")


if __name__ == "__main__":
    unittest.main()
