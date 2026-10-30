"""hecatron format: the format action and --check, the format check hecatron
lint adds (and the config switch that turns it off), and the shell helpers
the default format hooks share (languages/_format.sh). The formatters
themselves are checked in containers (tests/format/check.sh)."""

from __future__ import annotations

import json
import os
import subprocess
import sys
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(HOME, "tests"))
sys.path.insert(0, os.path.join(HOME, "libexec", "lib"))

import project  # noqa: E402
from test_hecatron import Fixture, write  # noqa: E402

HOOKS = os.path.join(HOME, "languages", "_hooks.sh")
FORMAT = os.path.join(HOME, "languages", "_format.sh")


class FormatExcludeTest(unittest.TestCase):
    def test_the_setting(self):
        self.assertEqual(project.format_exclude({}), [])
        self.assertEqual(project.format_exclude({"format": {"exclude": ["tests/fixtures/", "*.pb.go"]}}),
                         ["tests/fixtures/", "*.pb.go"])
        self.assertEqual(project.format_config_problems({"format": {"exclude": ["a/"]}}), [])
        self.assertTrue(project.format_config_problems({"format": []}))
        self.assertTrue(project.format_config_problems({"format": {"exclude": "a/"}}))
        self.assertTrue(project.format_config_problems({"format": {"exclude": [""]}}))

    def test_the_hooks_leave_excluded_files_alone(self):
        import tempfile
        with tempfile.TemporaryDirectory() as root:
            root = os.path.realpath(root)
            for path in ("go/fixtures/x/bad.go", "go/src/b.go", "go/src/c.pb.go"):
                os.makedirs(os.path.dirname(os.path.join(root, path)), exist_ok=True)
                write(os.path.join(root, path), "package x\n")
            def listed(exclude):
                env = dict(os.environ, HECATRON_ROOT=root, HECATRON_FORMAT_EXCLUDE="\n".join(exclude))
                result = subprocess.run(["bash", "-c", f'set -euo pipefail; source "{FORMAT}"; format_files go'],
                                        cwd=os.path.join(root, "go"), env=env, capture_output=True, text=True, check=True)
                return result.stdout.split()
            self.assertEqual(listed([]), ["./fixtures/x/bad.go", "./src/b.go", "./src/c.pb.go"])
            self.assertEqual(listed(["go/fixtures/"]), ["./src/b.go", "./src/c.pb.go"])
            self.assertEqual(listed(["go/fixtures/", "go/src/*.pb.go"]), ["./src/b.go"])


class LintConfigTest(unittest.TestCase):
    def test_lint_checks_format_by_default(self):
        self.assertTrue(project.lint_checks_format({}, "go"))
        self.assertTrue(project.lint_checks_format({"lint": {}}, "go"))

    def test_false_turns_it_off_everywhere(self):
        self.assertFalse(project.lint_checks_format({"lint": {"format": False}}, "go"))

    def test_an_object_turns_it_off_per_language(self):
        config = {"lint": {"format": {"haskell": False}}}
        self.assertFalse(project.lint_checks_format(config, "haskell"))
        self.assertTrue(project.lint_checks_format(config, "go"))

    def test_check_validates_the_setting(self):
        self.assertEqual(project.lint_config_problems({}), [])
        self.assertEqual(project.lint_config_problems({"lint": {"format": False}}), [])
        self.assertEqual(project.lint_config_problems({"lint": {"format": {"go": False}}}), [])
        self.assertTrue(project.lint_config_problems({"lint": {"format": "no"}}))
        self.assertTrue(project.lint_config_problems({"lint": {"format": {"go": "no"}}}))
        self.assertTrue(project.lint_config_problems({"lint": []}))


class FormatCommandTest(Fixture):
    """A project-defined language whose hooks say how they were run."""

    def setUp(self):
        super().setUp()
        self.configure(languages=["fixture"])
        os.makedirs(self.path("fixture"))
        self.define("config.json", json.dumps({"dependency": "bash"}))
        self.define("format.sh", 'echo "format check=${HECATRON_FORMAT_CHECK:-0}"')
        self.define("lint.sh", 'echo "lint ran"')

    def define(self, name: str, body: str) -> None:
        path = self.path(".hecatron", "languages", "fixture", name)
        if name.endswith(".sh"):
            write(path, f"#!/usr/bin/env bash\n{body}\n", 0o755)
        else:
            write(path, body)

    def test_format_rewrites_by_default(self):
        result = self.hecatron("format", "fixture")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("format check=0", result.stdout)

    def test_check_changes_nothing(self):
        result = self.hecatron("format", "--check", "fixture")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("format check=1", result.stdout)

    def test_lint_runs_the_format_check_after_the_linter(self):
        result = self.hecatron("lint", "fixture")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertLess(result.stdout.index("lint ran"), result.stdout.index("format check=1"))

    def test_an_unformatted_file_fails_lint(self):
        self.define("format.sh", '[ "${HECATRON_FORMAT_CHECK:-}" = 1 ] && exit 1; exit 0')
        result = self.hecatron("lint", "fixture")
        self.assertNotEqual(result.returncode, 0)

    def test_the_project_can_turn_the_lint_check_off(self):
        for setting in (False, {"fixture": False}):
            self.configure(languages=["fixture"], lint={"format": setting})
            result = self.hecatron("lint", "fixture")
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("lint ran", result.stdout)
            self.assertNotIn("format check", result.stdout)

    def test_a_linter_that_checks_formatting_itself_isnt_followed_by_the_check(self):
        self.define("lint.sh", f'source {HOOKS}\nchecks_format\necho "lint ran"')
        result = self.hecatron("lint", "fixture")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("lint ran", result.stdout)
        self.assertNotIn("format check", result.stdout)

    def test_a_language_without_a_formatter_is_skipped(self):
        os.remove(self.path(".hecatron", "languages", "fixture", "format.sh"))
        with open(self.path(".hecatron", "languages", "fixture", "config.json"), "w", encoding="utf-8") as f:
            f.write(json.dumps({"dependency": "bash"}))
        result = self.hecatron("format", "fixture")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("fixture: format skipped: no formatter for fixture", result.stdout)
        lint = self.hecatron("lint", "fixture")
        self.assertEqual(lint.returncode, 0, lint.stderr)

    def test_check_reports_a_bad_switch(self):
        self.configure(languages=["fixture"], lint={"format": "off"})
        result = self.hecatron("check")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("lint.format", result.stdout)


class FormatHelpersTest(Fixture):
    """languages/_format.sh, sourced as a hook sources it."""

    def shell(self, script: str, cwd: str | None = None) -> subprocess.CompletedProcess:
        env = {**os.environ, "HECATRON_ROOT": self.root, "HECATRON_LANG": "fixture"}
        return subprocess.run(["bash", "-c", f"source {HOOKS}\nsource {FORMAT}\n{script}"],
                              cwd=cwd or self.root, env=env, capture_output=True, text=True)

    def test_precommit_rev_reads_the_hook_repository_version(self):
        write(self.path(".pre-commit-config.yaml"), """repos:
  - repo: https://github.com/psf/black
    rev: 24.1.0
    hooks: [{id: black}]
  - repo: https://github.com/astral-sh/ruff-pre-commit
    rev: "v0.6.9"
    hooks:
      - id: ruff-format
""")
        os.makedirs(self.path("python"))
        result = self.shell("precommit_rev ruff-pre-commit; precommit_rev psf/black; precommit_rev nope || echo none",
                            cwd=self.path("python"))
        self.assertEqual(result.stdout.split(), ["0.6.9", "24.1.0", "none"])

    def test_uv_locked_reads_a_package_version(self):
        write(self.path("uv.lock"), """version = 1

[[package]]
name = "ruff-lsp"
version = "0.0.1"

[[package]]
name = "ruff"
version = "0.5.0"
source = { registry = "https://pypi.org/simple" }
""")
        result = self.shell("uv_locked ruff; uv_locked black || echo none")
        self.assertEqual(result.stdout.split(), ["0.5.0", "none"])

    def test_format_files_skips_build_output_and_vendored_code(self):
        for name in ("src/a.c", "include/a.h", "build/gen.c", "third_party/x/y.c", ".git/z.c", "src/notes.txt"):
            write(self.path(name), "")
        result = self.shell("format_files c h")
        self.assertEqual(result.stdout.split(), ["./include/a.h", "./src/a.c"])

    def test_checking_fails_listing_the_files(self):
        result = self.shell("HECATRON_FORMAT_CHECK=1\nformat_listing printf '%s\\n' ./a.go ./b.go")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("not formatted", result.stderr)
        self.assertIn("  a.go\n  b.go", result.stderr)

    def test_formatting_lists_what_it_changed_and_succeeds(self):
        result = self.shell("format_listing printf '%s\\n' ./a.go")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("./a.go", result.stdout)

    def test_nothing_listed_passes_the_check(self):
        result = self.shell("HECATRON_FORMAT_CHECK=1\nformat_listing true")
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_node_depends_on_reads_the_package_and_its_workspace(self):
        write(self.path("package.json"), json.dumps({"devDependencies": {"prettier": "3.3.3"}}))
        write(self.path("packages", "a", "package.json"), json.dumps({"dependencies": {"left-pad": "1"}}))
        result = self.shell("node_depends_on prettier && echo yes; node_depends_on @biomejs/biome || echo no",
                            cwd=self.path("packages", "a"))
        self.assertEqual(result.stdout.split(), ["yes", "no"])

    def test_a_formatter_without_configuration_skips(self):
        write(self.path("src", "a.c"), "int x;\n")
        result = self.shell("format_clang c h")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("no .clang-format", result.stdout)


class DefinitionsTest(unittest.TestCase):
    def test_every_format_hook_is_described(self):
        languages = os.path.join(HOME, "languages")
        for lang in sorted(os.listdir(languages)):
            hook = os.path.join(languages, lang, "hooks", "format.sh")
            if not os.path.isfile(hook):
                continue
            with open(os.path.join(languages, lang, "config.json"), encoding="utf-8") as f:
                config = json.load(f)
            with self.subTest(lang=lang):
                self.assertTrue(config.get("formatting"), f"{lang} has a format hook but no \"formatting\"")
                self.assertTrue(os.access(hook, os.X_OK))


if __name__ == "__main__":
    unittest.main()
