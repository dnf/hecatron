"""Tests for hecatron's project model and commands, run against fixture repositories.

    python3 -m unittest discover -s tests
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
import tempfile
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
BIN = os.path.join(HOME, "bin", "hecatron")
sys.path.insert(0, os.path.join(HOME, "libexec", "lib"))
sys.path.insert(0, os.path.join(HOME, "libexec", "commands"))

import project  # noqa: E402
import manage  # noqa: E402


def write(path: str, content: str, mode: int | None = None) -> None:
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(content)
    if mode:
        os.chmod(path, mode)


class Fixture(unittest.TestCase):
    """A temporary git repository, optionally initialised for hecatron."""

    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.root = os.path.realpath(self._tmp.name)
        subprocess.run(["git", "init", "-q", self.root], check=True)

    def tearDown(self) -> None:
        self._tmp.cleanup()

    def path(self, *parts: str) -> str:
        return os.path.join(self.root, *parts)

    def configure(self, **config) -> None:
        data = {"schema": 1, "languages": [], **config}
        write(self.path(".hecatron", "config.json"), json.dumps(data))

    def commit(self) -> None:
        subprocess.run(["git", "add", "-A"], cwd=self.root, check=True)
        subprocess.run(["git", "-c", "user.email=t@example.com", "-c", "user.name=t",
                        "commit", "-qm", "fixture"], cwd=self.root, check=True)

    def hecatron(self, *args: str, cwd: str | None = None) -> subprocess.CompletedProcess:
        return subprocess.run(["bash", BIN, *args], cwd=cwd or self.root,
                              capture_output=True, text=True)


class ProjectModelTest(Fixture):
    def test_root_is_found_from_a_subdirectory(self):
        self.configure()
        os.makedirs(self.path("a", "b"))
        self.assertEqual(project.find_root(self.path("a", "b")), self.root)

    def test_no_project_is_an_error(self):
        with self.assertRaises(project.ProjectError):
            project.find_root(self.root)

    def test_unknown_schema_is_refused(self):
        write(self.path(".hecatron", "config.json"), json.dumps({"schema": 99}))
        with self.assertRaises(project.ProjectError):
            project.load(self.root)

    def test_newer_hecatron_requirement_is_refused(self):
        self.configure(hecatron=">=999.0.0")
        with self.assertRaises(project.ProjectError):
            project.load(self.root)

    def test_defaults_for_a_single_component_project(self):
        self.configure()
        p = project.load(self.root)
        self.assertEqual(p.components, ["."])
        self.assertEqual(p.impl_path, "{lang}")
        self.assertEqual(p.name, os.path.basename(self.root))

    def test_component_directories_follow_the_impl_path(self):
        self.configure(components=["age", "date"], implPath="{component}/impl/{lang}")
        os.makedirs(self.path("age", "impl", "go"))
        p = project.load(self.root)
        self.assertEqual(p.impl_dirs("go"), [("age", self.path("age", "impl", "go"))])

    def test_project_files_override_built_in_ones(self):
        self.configure()
        p = project.load(self.root)
        builtin = p.resolve("go", "install.sh")
        self.assertTrue(builtin.startswith(project.BUILTIN_LANGUAGES))
        write(self.path(".hecatron", "languages", "go", "install.sh"), "#!/bin/sh\n")
        self.assertEqual(p.resolve("go", "install.sh"), self.path(".hecatron", "languages", "go", "install.sh"))

    def test_project_config_merges_over_built_in(self):
        write(self.path(".hecatron", "languages", "go", "config.json"), json.dumps({"abbreviation": "golang"}))
        self.configure()
        p = project.load(self.root)
        self.assertEqual(p.definitions["go"]["abbreviation"], "golang")
        self.assertEqual(p.definitions["go"]["dependency"], "go")
        self.assertEqual(p.resolve_name("golang"), "go")

    def test_a_project_can_define_a_new_language(self):
        write(self.path(".hecatron", "languages", "brainfuck", "config.json"), json.dumps({"dependency": "bf"}))
        self.configure()
        self.assertIn("brainfuck", project.load(self.root).definitions)

    def test_every_built_in_language_has_a_valid_definition(self):
        for lang in sorted(os.listdir(project.BUILTIN_LANGUAGES)):
            directory = os.path.join(project.BUILTIN_LANGUAGES, lang)
            if not os.path.isdir(directory):
                continue
            with self.subTest(lang=lang):
                with open(os.path.join(directory, "config.json"), encoding="utf-8") as f:
                    config = json.load(f)
                self.assertIn("dependency", config)
                self.assertIsInstance(config.get("detect", []), list)
                for script in ("install.sh", "uninstall.sh"):
                    self.assertTrue(os.path.isfile(os.path.join(directory, script)), script)


class InitTest(Fixture):
    def test_infers_one_component_per_language_directory(self):
        self.assertEqual(manage.infer_layout({"go": {"go"}, "rust": {"rust", "rust/src"}}), (["."], "{lang}"))

    def test_infers_components_from_a_varying_segment(self):
        found = {"go": {"age/impl/go", "date/impl/go"}, "rust": {"age/impl/rust", "date/impl/rust/src"}}
        self.assertEqual(manage.infer_layout(found), (["age", "date"], "{component}/impl/{lang}"))

    def test_single_language_repository(self):
        write(self.path("go.mod"), "module x\n")
        self.commit()
        result = self.hecatron("init")
        self.assertEqual(result.returncode, 0, result.stderr)
        with open(self.path(".hecatron", "config.json"), encoding="utf-8") as f:
            config = json.load(f)
        self.assertEqual(config["languages"], ["go"])
        self.assertEqual(config["implPath"], ".")
        with open(self.path(".gitignore"), encoding="utf-8") as f:
            self.assertIn(".hecatron/local.json", f.read())

    def test_init_is_a_no_op_on_an_initialised_project(self):
        self.configure(languages=["go"])
        self.hecatron("init")
        with open(self.path(".hecatron", "config.json"), encoding="utf-8") as f:
            self.assertEqual(json.load(f)["languages"], ["go"])

    def test_init_refuses_a_directory_outside_git(self):
        with tempfile.TemporaryDirectory() as outside:
            result = subprocess.run(["bash", BIN, "init", outside], capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)


class LanguageCommandsTest(Fixture):
    def test_add_remove_enable_disable(self):
        self.configure()
        self.assertEqual(self.hecatron("add", "rs", "go").returncode, 0)
        self.assertEqual(project.load(self.root).languages, ["go", "rust"])
        self.hecatron("disable", "go")
        self.assertEqual(project.load(self.root).disabled, ["go"])
        self.hecatron("enable", "--all")
        self.assertEqual(project.load(self.root).disabled, [])
        self.hecatron("remove", "go")
        self.assertEqual(project.load(self.root).languages, ["rust"])

    def test_unknown_language_is_an_error(self):
        self.configure()
        self.assertNotEqual(self.hecatron("add", "nosuchlang").returncode, 0)


class GeneratedFilesTest(Fixture):
    def setUp(self):
        super().setUp()
        self.configure(languages=["go"], components=["a", "b"], implPath="{component}/{lang}",
                       generated={"workflows": "ci/{lang}.yml", "dockerfile": "Dockerfile"})
        os.makedirs(self.path("a", "go"))
        os.makedirs(self.path("b", "go"))

    def test_workflows_carry_a_banner_and_check_detects_drift(self):
        write(self.path(".hecatron", "languages", "go", "workflow.yml"), "steps: [a/go, b/go]\n")
        self.assertEqual(self.hecatron("workflows").returncode, 0)
        with open(self.path("ci", "go.yml"), encoding="utf-8") as f:
            content = f.read()
        self.assertIn("DO NOT EDIT", content)
        self.assertEqual(self.hecatron("workflows", "--check").returncode, 0)
        write(self.path("ci", "go.yml"), "edited\n")
        self.assertNotEqual(self.hecatron("workflows", "--check").returncode, 0)

    def test_check_requires_every_component_in_each_workflow(self):
        write(self.path(".hecatron", "languages", "go", "workflow.yml"), "steps: [a/go]\n")
        self.hecatron("workflows")
        self.hecatron("dockerfile")
        result = self.hecatron("check")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("doesn't test b", result.stdout)

    def test_dockerfile_collects_snippets(self):
        self.assertEqual(self.hecatron("dockerfile").returncode, 0)
        with open(self.path("Dockerfile"), encoding="utf-8") as f:
            content = f.read()
        self.assertIn("FROM ubuntu", content)
        self.assertEqual(self.hecatron("dockerfile", "--check").returncode, 0)


class RunTest(Fixture):
    def setUp(self):
        super().setUp()
        self.configure(languages=["go"], components=["a", "b"], implPath="{component}/{lang}",
                       tasks={"test": {"hello": "echo hello-task"}})
        for component in ("a", "b"):
            os.makedirs(self.path(component, "go"))

    def hook(self, body: str) -> None:
        write(self.path(".hecatron", "languages", "go", "test.sh"), f"#!/usr/bin/env bash\n{body}\n", 0o755)

    def test_hook_runs_once_per_component_in_its_directory(self):
        self.hook('echo "ran $1 in $(basename "$(dirname "$PWD")") for $HECATRON_LANG"')
        result = self.hecatron("test", "go")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("ran a in a for go", result.stdout)
        self.assertIn("ran b in b for go", result.stdout)

    def test_a_failing_component_fails_the_run_but_others_still_run(self):
        self.hook('[ "$1" = a ] && exit 1; echo "ran $1"')
        result = self.hecatron("test", "go")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("ran b", result.stdout)

    def test_component_flag_selects_components(self):
        self.hook('echo "ran $1"')
        result = self.hecatron("test", "go", "-c", "b")
        self.assertIn("ran b", result.stdout)
        self.assertNotIn("ran a", result.stdout)

    def test_parallel_run_keeps_the_component_flag(self):
        self.hook('echo "ran $1"')
        result = self.hecatron("test", "-c", "b")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        with open(self.path(".logs", "test", "go.log"), encoding="utf-8") as f:
            log = f.read()
        self.assertIn("ran b", log)
        self.assertNotIn("ran a", log)
        self.assertFalse(os.path.exists(self.path(".logs", "test", "hello.log")))

    def test_project_tasks_run_by_name(self):
        result = self.hecatron("test", "hello")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("hello-task", result.stdout)

    def test_works_from_a_subdirectory(self):
        self.hook('echo "ran $1"')
        result = self.hecatron("test", "go", cwd=self.path("a", "go"))
        self.assertIn("ran a", result.stdout)

    def test_env_gives_project_recipes_the_helpers(self):
        script = 'eval "$(bash %s env)"; echo "$_ADDED_LANGS"; _name rs' % BIN
        result = subprocess.run(["bash", "-c", script], cwd=self.root, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.split(), ["go", "rust"])

    def test_outside_a_project_is_an_error(self):
        with tempfile.TemporaryDirectory() as outside:
            result = subprocess.run(["bash", BIN, "test"], cwd=outside, capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("hecatron init", result.stderr)


if __name__ == "__main__":
    unittest.main()
