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
sys.path.insert(0, os.path.join(HOME, "libexec", "codegen"))
sys.path.insert(0, os.path.join(HOME, "tests"))

import dockerfile  # noqa: E402
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

    def test_python_classifiers_give_the_newest_listed_line(self):
        pyproject = ('classifiers = [\n  "Programming Language :: Python :: 3",\n'
                     '  "Programming Language :: Python :: 3.9",\n  "Programming Language :: Python :: 3.14",\n'
                     '  "Programming Language :: Python :: 3.10",\n]\n')
        self.assertEqual(self.read("python-classifiers", pyproject), "3.14")
        self.assertIs(self.read("python-classifiers", 'requires-python = ">=3.10"\n'), versions.NOT_PINNED)

    def test_github_setup_reads_plain_values_lists_and_matrices(self):
        workflow = """\
jobs:
  test:
    strategy:
      matrix:
        node-version: ['12.x', '14.x', 16.x]
        java:
          - 11
          - '1.8'
        include:
          - node-version: '16.x'
    steps:
      - uses: actions/setup-node@v3
        with:
          node-version: ${{ matrix.node-version }}
      - uses: actions/setup-java@v3
        with:
          java-version: ${{ matrix.java }}
      - uses: actions/setup-dotnet@v3
        with:
          dotnet-version: '3.1.x'  # the tests' runtime
"""
        self.assertEqual(self.read("github-setup", workflow, "node-version"), "16")
        self.assertEqual(self.read("github-setup", workflow, "java-version"), "11")
        self.assertEqual(self.read("github-setup", workflow, "dotnet-version"), "3.1")
        self.assertIs(self.read("github-setup", workflow, "go-version"), versions.NOT_PINNED)

    def test_github_setup_skips_values_without_a_usable_version(self):
        workflow = ("python-version: [ '3.x', '2.x', 'pypy-3.8' ]\nnode-version: lts/*\n"
                    "go-version: stable\nruby-version: ${{ env.RUBY }}\njava-version: '1.8'\n")
        for key in ("python-version", "node-version", "go-version", "ruby-version"):
            self.assertIs(self.read("github-setup", workflow, key), versions.NOT_PINNED, key)
        self.assertEqual(self.read("github-setup", workflow, "java-version"), "8")

    def test_maven_source_levels_name_the_newest_jdk_that_compiles_them(self):
        def pom(level):
            return (f"<project><properties><maven.compiler.source>{level}</maven.compiler.source></properties>"
                    "<build><source>../common/src/</source></build></project>")
        self.assertEqual(self.read("maven-source-level", pom("1.6")), "11")
        self.assertEqual(self.read("maven-source-level", pom("1.7")), "19")
        self.assertEqual(self.read("maven-source-level", "<configuration><release>7</release></configuration>"), "19")
        self.assertIs(self.read("maven-source-level", pom("1.8")), versions.NOT_PINNED)
        self.assertIs(self.read("maven-source-level", pom("${java.version}")), versions.NOT_PINNED)

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

    def test_global_json_keeps_the_part_its_roll_forward_policy_fixes(self):
        def read(policy):
            return self.read("global-json", '{"sdk": {"version": "8.0.100", "rollForward": "%s"}}' % policy)
        self.assertEqual(read("latestPatch"), "8.0.100")
        self.assertEqual(read("latestFeature"), "8.0")
        self.assertEqual(read("latestMinor"), "8")
        self.assertEqual(read("latestMajor"), "8")
        self.assertEqual(
            self.read("global-json", '{"sdk": {"version": "11.0.100-rc.1.1", "rollForward": "latestMajor"}}'),
            "11.0.100-rc.1.1")

    def test_composer_json_pins_only_with_a_platform_php(self):
        self.assertEqual(self.read("composer-json", '{"config": {"platform": {"php": "8.4.0"}}}'), "8.4")
        self.assertIs(self.read("composer-json", '{"require": {"php": "^8.4"}}'), versions.NOT_PINNED)
        self.assertIs(self.read("composer-json", "not json"), versions.NOT_PINNED)
        self.assertEqual(self.read("php-version", "8.3\n"), "8.3")

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

    def test_strip_removes_a_pattern_from_the_version(self):
        otp = "-otp-[0-9]+$"
        self.assertEqual(versions.strip_pin("1.17.3-otp-27", otp), "1.17.3")
        self.assertEqual(versions.strip_pin("=1.17.3-otp-27", otp), "=1.17.3")
        self.assertEqual(versions.strip_pin("1.19.1", otp), "1.19.1")
        self.assertEqual(versions.strip_pin("1.16.0-rc.1", otp), "1.16.0-rc.1")
        self.assertIsNone(versions.strip_pin(None, otp))
        self.assertIs(versions.strip_pin(versions.NOT_PINNED, otp), versions.NOT_PINNED)
        self.assertIsNone(versions.strip_pin("1.2.3", "[0-9.]+"))
        self.assertEqual(versions.strip_pin("1.17.3-otp-27", None), "1.17.3-otp-27")

    def test_an_unknown_format_is_an_error(self):
        with self.assertRaises(ValueError):
            self.read("no-such-format", "1.0")


class VersionFixture(Fixture):
    """A project with components a and b, implementing a language `tool` that
    hecatron "ships" (a built-in definition in a temporary directory)."""

    def setUp(self) -> None:
        super().setUp()
        self._saved_as_of = os.environ.pop("HECATRON_AS_OF", None)
        self.builtin = self.path("builtin-languages")
        self._saved = project.BUILTIN_LANGUAGES
        project.BUILTIN_LANGUAGES = self.builtin
        self.configure(languages=["tool"], components=["a", "b"], implPath="{component}/{lang}")
        for component in ("a", "b"):
            os.makedirs(self.path(component, "tool"))
        self.define(version="=2.0.0", pinFiles=[{"file": ".tool-version", "format": "python-version"}])

    def tearDown(self) -> None:
        project.BUILTIN_LANGUAGES = self._saved
        os.environ.pop("HECATRON_AS_OF", None)
        if self._saved_as_of is not None:
            os.environ["HECATRON_AS_OF"] = self._saved_as_of
        super().tearDown()

    def supports(self, path: str, *lines: str) -> None:
        self.define(version="=2.0.0", pinFiles=[{"file": ".tool-version", "format": "python-version"}],
                    supportFiles=[{"file": "pyproject.toml", "format": "python-classifiers"}])
        write(self.path(path), "".join(f'"Programming Language :: Python :: {line}",\n' for line in lines))

    def releases(self, *releases: list) -> None:
        """The built-in definition's release dates."""
        write(os.path.join(self.builtin, "tool", "releases.json"), json.dumps({"releases": list(releases)}))

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

    def test_a_pin_file_entry_can_strip_a_suffix(self):
        self.define(version="=2.0.0", pinFiles=[{"file": ".tool-versions", "format": "tool-versions",
                                                  "key": "tool", "strip": "-otp-[0-9]+$"}])
        write(self.path("a", "tool", ".tool-versions"), "erlang 27.1\ntool 1.17.3-otp-27\n")
        self.assertEqual(self.load().resolve_version("tool", "a"),
                         {"spec": "1.17.3", "source": "pin:a/tool/.tool-versions", "exact": False})

    def test_a_strip_that_isnt_a_regular_expression_is_an_error(self):
        self.define(version="=2.0.0", pinFiles=[{"file": ".tool-version", "format": "python-version", "strip": "("}])
        write(self.path("a", "tool", ".tool-version"), "1.5\n")
        with self.assertRaisesRegex(project.ProjectError, "is not a regular expression"):
            self.load().resolve_version("tool", "a")

    def test_a_project_can_add_pin_files(self):
        self.override(pinFiles=[{"file": "tool.txt", "format": "python-version"}])
        write(self.path("a", "tool", "tool.txt"), "1.5\n")
        self.assertEqual(self.load().resolve_version("tool", "a")["spec"], "1.5")

    def test_versions_lists_each_distinct_spec_with_its_sources(self):
        write(self.path("a", "tool", ".tool-version"), "1.8\n")
        self.assertEqual(self.load().versions("tool"),
                         [("1.8", ["pin:a/tool/.tool-version"]), ("=2.0.0", ["default"])])

    def test_a_component_gets_the_newest_line_it_supports_when_the_default_is_newer(self):
        self.supports("a/tool/pyproject.toml", "1.8", "1.9")
        p = self.load()
        self.assertEqual(p.resolve_version("tool", "a"),
                         {"spec": "1.9", "source": "supports:a/tool/pyproject.toml", "exact": False})
        self.assertEqual(p.resolve_version("tool", "b")["source"], "default")
        self.assertEqual(p.version_notes("tool"), [("a", "1.9, the newest a/tool/pyproject.toml supports")])

    def test_supporting_the_defaults_line_or_newer_keeps_the_default(self):
        self.supports("a/tool/pyproject.toml", "1.9", "2.0")
        self.assertEqual(self.load().resolve_version("tool", "a")["spec"], "=2.0.0")
        self.supports("a/tool/pyproject.toml", "2.1")
        self.assertEqual(self.load().resolve_version("tool", "a")["spec"], "=2.0.0")

    def test_a_pin_or_project_version_beats_what_a_component_supports(self):
        self.supports("a/tool/pyproject.toml", "1.8")
        write(self.path("a", "tool", ".tool-version"), "2.0\n")
        self.assertEqual(self.load().resolve_version("tool", "a")["spec"], "2.0")
        self.override(version="1.5")
        self.assertEqual(self.load().resolve_version("tool", "b")["spec"], "1.5")

    def test_ci_workflows_hold_the_default_back(self):
        self.define(version="=2.0.0",
                    supportFiles=[{"file": ".github/workflows/*.y*ml", "format": "github-setup", "key": "tool-version"}])
        write(self.path(".github", "workflows", "old.yml"), "tool-version: '1.4'\n")
        write(self.path(".github", "workflows", "new.yaml"), "tool-version: [1.7, 1.8.x]\n")
        self.assertEqual(self.load().resolve_version("tool", "a"),
                         {"spec": "1.8", "source": "supports:.github/workflows/new.yaml", "exact": False})

    def test_an_older_release_on_the_defaults_line_keeps_the_default(self):
        self.define(version="=2.0.9",
                    supportFiles=[{"file": ".github/workflows/*.y*ml", "format": "github-setup", "key": "tool-version"}])
        write(self.path(".github", "workflows", "release.yml"), "tool-version: '2.0.6'\n")
        self.assertEqual(self.load().resolve_version("tool", "a")["spec"], "=2.0.9")
        write(self.path(".github", "workflows", "release.yml"), "tool-version: '1.9.6'\n")
        self.assertEqual(self.load().resolve_version("tool", "a")["spec"], "1.9.6")

    def test_a_supported_line_below_what_hecatron_installs_is_raised(self):
        self.supports("a/tool/pyproject.toml", "1.2")
        self.define(version="=2.0.0", oldestInstallable="1.5",
                    supportFiles=[{"file": "pyproject.toml", "format": "python-classifiers"}])
        self.assertEqual(self.load().resolve_version("tool", "a")["spec"], "1.5")

    def test_check_reports_a_pin_that_disagrees_with_the_project(self):
        self.override(version="1.9")
        write(self.path("a", "tool", ".tool-version"), "1.8\n")
        self.assertEqual(self.load().version_conflicts(),
                         [("tool", "a", "pin:a/tool/.tool-version", "1.8", "1.9")])


class AsOfTest(VersionFixture):
    """An old checkout gets the releases current at its last commit."""

    def setUp(self) -> None:
        super().setUp()
        self.define(version="=2.0.0", track="stable",
                    pinFiles=[{"file": ".tool-version", "format": "python-version"}],
                    supportFiles=[{"file": "pyproject.toml", "format": "python-classifiers"}])
        self.releases(["1.8.0", "2019-01-10"], ["1.9.0", "2020-03-01", "lts"], ["1.8.1", "2020-06-01"],
                      ["1.9.1", "2020-09-01", "lts"], ["1.10.0", "2021-02-01"], ["2.0.0", "2023-05-01", "lts"])

    def resolve(self, component: str = "a", when: str | None = "2020-07-01") -> dict:
        if when:
            os.environ["HECATRON_AS_OF"] = when
        return self.load().resolve_version("tool", component)

    def test_the_default_becomes_the_newest_release_then(self):
        self.assertEqual(self.resolve(), {"spec": "=1.9.0", "source": "as-of:2020-07-01 (default)", "exact": True})

    def test_an_lts_default_takes_the_newest_lts_release_then(self):
        self.define(version="=2.0.0", track="lts")
        self.assertEqual(self.resolve(when="2021-06-01")["spec"], "=1.9.1")

    def test_a_prefix_pin_takes_its_newest_release_then(self):
        write(self.path("a", "tool", ".tool-version"), "1.8\n")
        self.assertEqual(self.resolve(), {"spec": "=1.8.1", "source": "as-of:2020-07-01 (pin:a/tool/.tool-version)",
                                          "exact": True})

    def test_a_supported_line_takes_its_newest_release_then(self):
        self.supports("a/tool/pyproject.toml", "1.8")
        self.assertEqual(self.resolve()["spec"], "=1.8.1")
        self.assertEqual(self.load().version_notes("tool")[0],
                         ("a", "1.8.1, current on 2020-07-01, this commit's date (the line from a/tool/pyproject.toml)"))

    def test_an_exact_pin_is_untouched(self):
        self.override(version="=1.10.0")
        self.assertEqual(self.resolve()["spec"], "=1.10.0")

    def test_nothing_released_yet_keeps_the_spec(self):
        self.assertEqual(self.resolve(when="2018-01-01"), {"spec": "=2.0.0", "source": "default", "exact": True})

    def test_a_line_precision_definition_gets_a_line(self):
        self.define(version="=2.0.0", asOfPrecision="line")
        self.assertEqual(self.resolve()["spec"], "1.9")

    def test_a_release_older_than_hecatron_installs_is_raised(self):
        self.supports("a/tool/pyproject.toml", "1.8")
        self.define(version="=2.0.0", oldestInstallable="1.9",
                    supportFiles=[{"file": "pyproject.toml", "format": "python-classifiers"}])
        self.assertEqual(self.resolve()["spec"], "=1.9.0")
        self.define(version="=2.0.0", oldestInstallable="1.10")
        self.assertEqual(self.resolve(when="2021-06-01"),
                         {"spec": "=1.10.0", "source": "as-of:2021-06-01 (default)", "exact": True})
        self.define(version="=2.0.0", oldestInstallable="2.0")
        resolved = self.resolve()
        self.assertEqual(resolved, {"spec": "2.0", "exact": False,
                                    "source": "as-of:2020-07-01 (default; raised to 2.0, the oldest hecatron installs)"})
        # A pin stands, even below what hecatron installs.
        write(self.path("a", "tool", ".tool-version"), "1.8\n")
        self.define(version="=2.0.0", oldestInstallable="1.9",
                    pinFiles=[{"file": ".tool-version", "format": "python-version"}])
        self.assertEqual(self.resolve()["spec"], "1.8")

    def test_off_turns_it_off(self):
        self.assertEqual(self.resolve(when="off")["spec"], "=2.0.0")

    def test_a_recent_checkout_keeps_todays_versions(self):
        self.commit()
        self.assertEqual(self.resolve(when=None)["source"], "default")

    def test_a_checkout_over_a_year_old_uses_its_commit_date(self):
        subprocess.run(["git", "add", "-A"], cwd=self.root, check=True)
        env = dict(os.environ, GIT_COMMITTER_DATE="2020-07-01T12:00:00", GIT_AUTHOR_DATE="2020-07-01T12:00:00")
        subprocess.run(["git", "-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-qm", "old"],
                       cwd=self.root, check=True, env=env)
        self.assertEqual(self.resolve(when=None)["source"], "as-of:2020-07-01 (default)")

    def test_a_bad_date_is_an_error(self):
        with self.assertRaises(project.ProjectError):
            self.resolve(when="last year")

    def test_doctor_says_why(self):
        self.define(version="=2.0.0", dependency="bash")
        self.installed("1.9.0\t/opt/tool-1.9.0")
        env = dict(os.environ, HECATRON_BUILTIN_LANGUAGES=self.builtin, HECATRON_AS_OF="2020-07-01")
        result = subprocess.run(["bash", BIN, "doctor", "tool"], cwd=self.root, capture_output=True, text=True, env=env)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("a: tool 1.9.0, current on 2020-07-01, this commit's date", result.stdout)


class CheckTest(VersionFixture):
    def test_check_fails_on_a_pin_that_disagrees_with_the_project(self):
        self.override(version="1.9")
        write(self.path("a", "tool", ".tool-version"), "1.8\n")
        env = dict(os.environ, HECATRON_BUILTIN_LANGUAGES=self.builtin)
        result = subprocess.run(["bash", BIN, "check"], cwd=self.root, capture_output=True, text=True, env=env)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("a/tool/.tool-version pins 1.8, but the project's version is 1.9", result.stdout)


class DoctorTest(VersionFixture):
    def setUp(self) -> None:
        super().setUp()
        self.define(version="=2.0.0", dependency="bash",
                    pinFiles=[{"file": ".tool-version", "format": "python-version"}])
        write(self.path("a", "tool", ".tool-version"), "1.0\n")

    def doctor(self) -> subprocess.CompletedProcess:
        env = dict(os.environ, HECATRON_BUILTIN_LANGUAGES=self.builtin)
        return subprocess.run(["bash", BIN, "doctor", "tool"], cwd=self.root, capture_output=True, text=True, env=env)

    def test_missing_versions_lists_each_component_without_an_installed_match(self):
        self.installed("1.0.4\t/opt/tool-1.0.4")
        self.assertEqual(self.load().missing_versions("tool"), [("b", "=2.0.0", "default")])

    def test_doctor_reports_a_version_that_isnt_installed(self):
        self.installed("1.0.4\t/opt/tool-1.0.4")
        result = self.doctor()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("b: tool =2.0.0 not installed (from default)", result.stdout)
        self.assertNotIn("a: tool", result.stdout)

    def test_doctor_is_happy_when_every_version_is_installed(self):
        self.installed("1.0.4\t/opt/tool-1.0.4", "2.0.0\t/opt/tool-2.0.0")
        result = self.doctor()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_doctor_says_when_a_component_is_on_the_line_it_supports(self):
        self.define(version="=2.0.0", dependency="bash",
                    supportFiles=[{"file": "pyproject.toml", "format": "python-classifiers"}])
        write(self.path("b", "tool", "pyproject.toml"), '"Programming Language :: Python :: 1.9",\n')
        self.installed("1.9.2\t/opt/tool-1.9.2", "2.0.0\t/opt/tool-2.0.0")
        result = self.doctor()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("b: tool 1.9, the newest b/tool/pyproject.toml supports", result.stdout)

    def test_the_default_takes_any_version_on_its_release_line(self):
        # A machine with another patch of the default's line needs no second copy.
        self.installed("1.0.4\t/opt/tool-1.0.4", "2.0.7\t/opt/tool-2.0.7")
        self.assertEqual(self.load().missing_versions("tool"), [])
        self.assertEqual(self.load().installed_version("tool", "b"), ("2.0.7", "/opt/tool-2.0.7"))

    def test_a_version_off_the_default_line_is_missing(self):
        self.installed("1.0.4\t/opt/tool-1.0.4", "2.1.0\t/opt/tool-2.1.0")
        self.assertEqual(self.load().missing_versions("tool"), [("b", "=2.0.0", "default")])

    def test_a_definition_can_name_a_shorter_release_line(self):
        # Node.js and Java number a line by its major version alone.
        self.define(version="=2.0.0", releaseLine=1, dependency="bash",
                    pinFiles=[{"file": ".tool-version", "format": "python-version"}])
        self.installed("1.0.4\t/opt/tool-1.0.4", "2.5.1\t/opt/tool-2.5.1")
        self.assertEqual(self.load().missing_versions("tool"), [])

    def test_an_exact_project_version_still_matches_exactly(self):
        self.override(version="=2.0.0")
        self.installed("1.0.4\t/opt/tool-1.0.4", "2.0.7\t/opt/tool-2.0.7")
        self.assertEqual(self.load().missing_versions("tool"), [("b", "=2.0.0", "project")])

    def test_without_installed_sh_versions_arent_checked(self):
        self.assertEqual(self.load().missing_versions("tool"), [])
        self.assertEqual(self.doctor().returncode, 0)


class InstallOnlySupportTest(VersionFixture):
    """A support file marked "selects": false says what to install, not what
    runs: a workflow's dotnet-version, since dotnet runs its newest SDK."""

    def setUp(self) -> None:
        super().setUp()
        self.define(version="=2.0.0", pinFiles=[{"file": "global.json", "format": "global-json"}],
                    supportFiles=[{"file": ".github/workflows/*.y*ml", "format": "github-setup",
                                   "key": "tool-version", "selects": False}])
        write(self.path("a", "tool", ".github", "workflows", "test.yml"), "tool-version: 1.8.x\n")

    def test_the_version_is_installed(self):
        self.installed("2.0.0\t/opt/tool")
        p = self.load()
        self.assertEqual(p.resolve_version("tool", "a")["spec"], "1.8")
        self.assertFalse(p.selects_version("tool", "a"))
        self.assertTrue(p.selects_version("tool", "b"))
        self.assertEqual(p.missing_versions("tool"), [("a", "1.8", "supports:a/tool/.github/workflows/test.yml")])

    def test_the_newest_runs(self):
        self.installed("1.8.2\t/opt/tool-1.8", "2.0.0\t/opt/tool-2.0")
        p = self.load()
        self.assertEqual(p.missing_versions("tool"), [])
        self.assertEqual(p.installed_version("tool", "a"), ("2.0.0", "/opt/tool-2.0"))
        self.assertEqual(p.version_notes("tool"), [
            ("a", "1.8 installed for a/tool/.github/workflows/test.yml; runs the newest, 2.0.0, "
                  "unless a global.json picks one")])

    def test_without_the_default_the_newest_installed_runs(self):
        self.installed("1.8.2\t/opt/tool-1.8", "1.9.0\t/opt/tool-1.9")
        self.assertEqual(self.load().installed_version("tool", "a"), ("1.9.0", "/opt/tool-1.9"))

    def test_a_project_version_still_selects(self):
        self.installed("1.5.1\t/opt/tool-1.5", "2.0.0\t/opt/tool-2.0")
        self.override(version="1.5")
        p = self.load()
        self.assertTrue(p.selects_version("tool", "a"))
        self.assertEqual(p.installed_version("tool", "a"), ("1.5.1", "/opt/tool-1.5"))


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


class SdkrootFromXcrunTest(VersionFixture):
    """On macOS, "sdkrootFromXcrun" gives SDKROOT the selected Xcode's SDK."""

    def setUp(self) -> None:
        super().setUp()
        self.define(version="=2.0.0", sdkrootFromXcrun=True)
        self._saved_os, self._saved_sdk = project.HOST_OS, list(project._xcrun_sdk)
        self._saved_sdkroot = os.environ.pop("SDKROOT", None)
        project.HOST_OS = "darwin"
        project._xcrun_sdk[:] = ["/sdk/MacOSX26.sdk"]

    def tearDown(self) -> None:
        project.HOST_OS = self._saved_os
        project._xcrun_sdk[:] = self._saved_sdk
        os.environ.pop("SDKROOT", None)
        if self._saved_sdkroot is not None:
            os.environ["SDKROOT"] = self._saved_sdkroot
        super().tearDown()

    def test_sdkroot_is_set_on_macos(self):
        self.assertEqual(self.load().language_env("tool", "a")["vars"], {"SDKROOT": "/sdk/MacOSX26.sdk"})

    def test_an_sdkroot_already_set_stands(self):
        os.environ["SDKROOT"] = "/mine"
        self.assertEqual(self.load().language_env("tool", "a")["vars"], {})

    def test_not_elsewhere(self):
        project.HOST_OS = "linux"
        self.assertEqual(self.load().language_env("tool", "a")["vars"], {})

    def test_a_failing_xcrun_leaves_it_unset(self):
        project._xcrun_sdk[:] = [None]
        self.assertEqual(self.load().language_env("tool", "a")["vars"], {})

    def test_xcrun_is_asked_once(self):
        project._xcrun_sdk.clear()
        bin_dir = self.path("bin")
        write(os.path.join(bin_dir, "xcrun"), f"#!/bin/sh\necho x >> {self.path('asked')}\necho /sdk/X.sdk\n", 0o755)
        saved_path = os.environ["PATH"]
        os.environ["PATH"] = bin_dir + os.pathsep + saved_path
        try:
            p = self.load()
            for component in ("a", "b", "a"):
                self.assertEqual(p.language_env("tool", component)["vars"], {"SDKROOT": "/sdk/X.sdk"})
        finally:
            os.environ["PATH"] = saved_path
        with open(self.path("asked"), encoding="utf-8") as f:
            self.assertEqual(f.read(), "x\n")


class DockerfileTest(VersionFixture):
    """A snippet's each-version block, written once per version (§18,
    "Generated files")."""

    def snippet(self, text: str, lang: str = "tool") -> None:
        write(os.path.join(self.builtin, lang, "dockerfile"), text)

    def generate(self) -> str:
        return dockerfile.generate(self.load())

    def test_a_block_repeats_for_the_default_then_each_component_version(self):
        self.override(version="=1.7.2")
        write(self.path("a", "tool", ".tool-version"), "1.8\n")
        self.snippet("RUN echo once\n# each-version\nRUN install {version} exact={exact} {major}/{minor}\n"
                     "# end-each-version\nENV PATH=/opt/tool/2.0.0/bin:${PATH}\n")
        content = self.generate()
        self.assertIn("RUN echo once\n"
                      "RUN install 2.0.0 exact=1 2/0\n"
                      "RUN install 1.8 exact= 1/8\n"
                      "RUN install 1.7.2 exact=1 1/7\n"
                      "ENV PATH=/opt/tool/2.0.0/bin:${PATH}\n", content)

    def test_a_version_two_components_share_is_written_once(self):
        self.snippet("# each-version\nRUN install {version}\n# end-each-version\n")
        self.assertEqual(self.generate().count("RUN install 2.0.0"), 1)

    def test_a_missing_part_fills_in_empty(self):
        self.override(version="3")
        self.snippet("# each-version\nRUN install {major}.{minor}.\n# end-each-version\n")
        self.assertIn("RUN install 3..", self.generate())

    def test_a_block_can_repeat_over_another_languages_versions(self):
        # Kotlin's snippet installs the JDKs Java's components resolve to.
        self.define(version="=21.0.1")
        write(os.path.join(self.builtin, "kotlin", "config.json"), "{}")
        self.snippet("# each-version tool\nRUN jdk {version}\n# end-each-version\n", lang="kotlin")
        self.configure(languages=["kotlin"], components=["a", "b"], implPath="{component}/{lang}")
        self.assertIn("RUN jdk 21.0.1", self.generate())

    def test_without_a_block_a_snippet_is_copied_as_it_is(self):
        self.snippet("RUN echo {version} ${HOME}\n")
        self.assertIn("RUN echo {version} ${HOME}\n", self.generate())

    def test_an_unclosed_block_is_an_error(self):
        self.snippet("# each-version\nRUN install {version}\n")
        with self.assertRaisesRegex(ValueError, "without # end-each-version"):
            self.generate()

    def test_a_pin_change_makes_the_dockerfile_stale(self):
        self.configure(languages=["tool"], components=["a", "b"], implPath="{component}/{lang}",
                       generated={"dockerfile": "Dockerfile"})
        self.snippet("# each-version\nRUN install {version}\n# end-each-version\n")
        env = {**os.environ, "HECATRON_BUILTIN_LANGUAGES": self.builtin}
        run = lambda *args: subprocess.run([BIN, "dockerfile", *args], cwd=self.root, env=env,  # noqa: E731
                                           capture_output=True, text=True)
        self.assertEqual(run().returncode, 0)
        write(self.path("a", "tool", ".tool-version"), "1.8\n")
        self.assertNotEqual(run("--check").returncode, 0)


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

    def test_installers_read_their_own_default_version_or_a_named_one(self):
        # Installers call default_version for their own definition (rust's
        # install.sh) and for another's (default_version zig), under set -u.
        languages = os.path.join(HOME, "languages")
        with open(os.path.join(languages, "rust", "config.json"), encoding="utf-8") as f:
            rust = json.load(f)["version"].lstrip("=")
        with open(os.path.join(languages, "zig", "config.json"), encoding="utf-8") as f:
            zig = json.load(f)["version"].lstrip("=")
        script = os.path.join(languages, "rust", "probe.sh")
        result = subprocess.run(["bash", "-c", 'source "$1"; default_version; default_version zig', script,
                                 os.path.join(languages, "_common.sh")], capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.split(), [rust, zig])

    def test_every_shipped_release_table_parses_and_has_its_default(self):
        languages = os.path.join(HOME, "languages")
        for lang in sorted(os.listdir(languages)):
            path = os.path.join(languages, lang, "releases.json")
            if not os.path.isfile(path):
                continue
            with open(path, encoding="utf-8") as f:
                releases = json.load(f)["releases"]
            with open(os.path.join(languages, lang, "config.json"), encoding="utf-8") as f:
                default = json.load(f)["version"].lstrip("=")
            for release in releases:
                self.assertTrue(versions.is_version(release[0]), f"{lang}: {release}")
                self.assertRegex(release[1], r"^\d{4}-\d{2}-\d{2}$", f"{lang}: {release}")
            self.assertIn(default, [release[0] for release in releases], f"{lang}: the default isn't in releases.json")

    def test_languages_sharing_a_toolchain_share_its_versions(self):
        fields = ("version", "track", "releaseLine", "pinFiles", "supportFiles", "oldestInstallable", "asOfPrecision")
        for lang, owner in (("javascript", "typescript"), ("fsharp", "csharp"), ("vbnet", "csharp")):
            with open(os.path.join(HOME, "languages", lang, "config.json"), encoding="utf-8") as f:
                config = json.load(f)
            with open(os.path.join(HOME, "languages", owner, "config.json"), encoding="utf-8") as f:
                theirs = json.load(f)
            self.assertEqual(config.get("releasesOf"), owner, lang)
            for field in fields:
                self.assertEqual(config.get(field), theirs.get(field), f"{lang}.{field}")

    def test_every_shipped_pin_format_exists(self):
        languages = os.path.join(HOME, "languages")
        for lang in sorted(os.listdir(languages)):
            path = os.path.join(languages, lang, "config.json")
            if not os.path.isfile(path):
                continue
            with open(path, encoding="utf-8") as f:
                config = json.load(f)
                for entry in config.get("pinFiles", []) + config.get("supportFiles", []):
                    self.assertIn(entry["format"], versions.FORMATS, f"{lang}: {entry}")


if __name__ == "__main__":
    unittest.main()
