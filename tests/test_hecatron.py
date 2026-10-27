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

    def test_language_path_expands_home_and_globs(self):
        os.makedirs(self.path("tools", "v1", "bin"))
        os.makedirs(self.path("tools", "v2", "bin"))
        write(self.path(".hecatron", "languages", "go", "config.json"),
              json.dumps({"path": [self.path("tools", "*", "bin"), "~/surely-not-a-real-dir-xyz"]}))
        # A language whose definition has no "path" at all, independent of the host.
        write(self.path(".hecatron", "languages", "fixture", "config.json"), json.dumps({"dependency": "bash"}))
        self.configure()
        p = project.load(self.root)
        self.assertEqual(p.language_path("go"), [self.path("tools", "v1", "bin"), self.path("tools", "v2", "bin")])
        self.assertEqual(p.language_path("fixture"), [])

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
        self.assertEqual(manage.infer_layout({"go": {"go"}, "rust": {"rust", "rust/src"}}), (["."], "{lang}", {}))

    def test_infers_components_from_a_varying_segment(self):
        found = {"go": {"age/impl/go", "date/impl/go"}, "rust": {"age/impl/rust", "date/impl/rust/src"}}
        self.assertEqual(manage.infer_layout(found), (["age", "date"], "{component}/impl/{lang}", {}))

    def test_a_directory_may_carry_any_of_a_languages_names(self):
        # Thrift's lib/rb and lib/netstd.
        found = {"ruby": {"lib/rb"}, "csharp": {"lib/netstd/src"}, "go": {"lib/go"}}
        names = {"ruby": ["ruby", "rb"], "csharp": ["csharp", "netstd"], "go": ["go"]}
        self.assertEqual(manage.infer_layout(found, names), (["."], "lib/{lang}", {}))

    def test_a_preferred_parent_beats_covering_more_languages(self):
        # Beam: the SDKs under sdks/, not the katas, which cover one more.
        found = {"go": {"sdks/go", "learning/katas/go"}, "java": {"sdks/java", "learning/katas/java"},
                 "kotlin": {"learning/katas/kotlin"}}
        self.assertEqual(manage.infer_layout(found),
                         (["."], "sdks/{lang}", {"kotlin": "learning/katas/kotlin"}))

    def test_a_language_outside_the_shared_layout_gets_its_own_path(self):
        # Cucumber's TypeScript in javascript/; flatbuffers' C++ at the root.
        found = {"go": {"go"}, "java": {"java"}, "typescript": {"javascript"}, "cpp": {".", "src"}}
        self.assertEqual(manage.infer_layout(found),
                         (["."], "{lang}", {"typescript": "javascript", "cpp": "."}))

    def test_a_directory_fitting_the_layout_counts_without_a_build_file(self):
        # Thrift: go.mod at the root, the code in lib/go.
        found = {"go": {"."}, "java": {"lib/java"}, "ruby": {"lib/rb"}}
        self.assertEqual(manage.infer_layout(found, {"ruby": ["ruby", "rb"]}, {"lib", "lib/go", "lib/java", "lib/rb"}),
                         (["."], "lib/{lang}", {}))

    def test_a_directory_another_language_was_found_in_isnt_borrowed(self):
        # Cucumber messages: javascript/ is TypeScript's (it has a tsconfig.json),
        # so JavaScript's package.json in jsonschema/ gets its own path.
        found = {"go": {"go"}, "java": {"java"}, "typescript": {"javascript"}, "javascript": {"jsonschema"}}
        self.assertEqual(manage.infer_layout(found, {"javascript": ["javascript", "js"]},
                                             {"go", "java", "javascript", "jsonschema"}),
                         (["."], "{lang}", {"javascript": "jsonschema", "typescript": "javascript"}))

    def detect(self, files: dict[str, str], defs: dict[str, dict]):
        for path, content in files.items():
            write(self.path(path), content)
        self.commit()
        return manage.detect_languages(self.root, defs)

    def test_examples_tests_and_vendored_code_are_skipped(self):
        found, skipped = self.detect({"go/go.mod": "", "examples/lua/x.lua": "", "third_party/zlib/x.c": "",
                                      "go/testdata/x.c": ""},
                                     {"go": {"detect": ["go.mod"]}, "lua": {"detect": ["*.lua"]},
                                      "c": {"detect": ["*.c"]}})
        self.assertEqual(found, {"go": {"go"}})
        self.assertEqual(set(skipped), {"lua", "c"})

    def test_build_files_outrank_sources(self):
        found, _ = self.detect({"java/pom.xml": "", "java/src/main/A.java": "", "tools/B.java": "",
                                "kt/x.kt": ""},
                               {"java": {"detect": ["pom.xml"], "detectSources": ["*.java"]},
                                "kotlin": {"detect": ["build.gradle.kts"], "detectSources": ["*.kt"]}})
        self.assertEqual(found, {"java": {"java"}, "kotlin": {"kt"}})

    def test_detect_unless_drops_a_hit_beside_another_languages_file(self):
        found, _ = self.detect({"js/package.json": "", "ts/package.json": "", "ts/tsconfig.json": ""},
                               {"javascript": {"detect": ["package.json"], "detectUnless": ["tsconfig.json"]},
                                "typescript": {"detect": ["tsconfig.json"]}})
        self.assertEqual(found, {"javascript": {"js"}, "typescript": {"ts"}})

    def test_a_shared_build_file_marks_c_and_cpp_at_the_root(self):
        # flatbuffers: CMakeLists.txt at the root and C++ in src/, not the
        # copy nested in another tree.
        defs = {"cpp": {"detect": [], "detectSources": ["*.cpp"], "sharedBuildFiles": ["CMakeLists.txt"]},
                "go": {"detect": ["go.mod"]}}
        found, _ = self.detect({"CMakeLists.txt": "", "src/a.cpp": "", "go/go.mod": "", "go/cgo/cpp/b.cpp": ""}, defs)
        self.assertEqual(found, {"cpp": {"."}, "go": {"go"}})

    def test_a_shared_build_file_without_the_languages_sources_isnt_a_hit(self):
        defs = {"c": {"detect": [], "detectSources": ["*.c"], "sharedBuildFiles": ["Makefile"]}}
        found, _ = self.detect({"Makefile": "", "lib/c/a.c": ""}, defs)
        self.assertEqual(found, {"c": {"lib/c"}})

    def test_a_shared_build_file_goes_to_the_language_with_most_sources(self):
        defs = {lang: {"detect": [], "detectSources": [glob], "sharedBuildFiles": ["CMakeLists.txt"]}
                for lang, glob in (("c", "*.c"), ("cpp", "*.cc"))}
        found, not_added = self.detect({"cpp/CMakeLists.txt": "", "cpp/src/a.cc": "", "cpp/src/b.cc": "",
                                        "cpp/src/utf/x.c": ""}, defs)
        self.assertEqual(found, {"cpp": {"cpp"}})
        self.assertIn("c", not_added)

    def test_sources_inside_another_languages_directory_arent_a_language(self):
        # libphonenumber's C files in the C++ library; diff-match-patch's in objectivec/.
        defs = {"cpp": {"detect": ["CMakeLists.txt"]}, "c": {"detectSources": ["*.c"]},
                "objectivec": {"detectSources": ["*.m"]}, "python": {"detect": ["pyproject.toml"]}}
        found, not_added = self.detect({"cpp/CMakeLists.txt": "", "cpp/src/utf/x.c": "", "objectivec/a.m": "",
                                        "objectivec/b.c": "", "python/pyproject.toml": ""}, defs)
        self.assertEqual(found, {"cpp": {"cpp"}, "objectivec": {"objectivec"}, "python": {"python"}})
        self.assertIn("inside cpp, objectivec", not_added["c"])

    def test_other_languages_native_extensions_dont_place_a_language(self):
        defs = {"python": {"detect": ["pyproject.toml"]}, "c": {"detectSources": ["*.c"]}}
        found, _ = self.detect({"python/pyproject.toml": "", "python/ext/a.c": "", "upb/base/b.c": ""}, defs)
        self.assertEqual(found, {"python": {"python"}, "c": {"upb/base"}})

    def test_sources_of_their_own_are_still_a_language(self):
        defs = {"cpp": {"detect": ["CMakeLists.txt"]}, "c": {"detectSources": ["*.c"]}}
        found, _ = self.detect({"cpp/CMakeLists.txt": "", "cpp/x.c": "", "c/y.c": ""}, defs)
        self.assertEqual(found, {"cpp": {"cpp"}, "c": {"c"}})

    def test_init_writes_a_path_for_a_language_outside_the_layout(self):
        write(self.path("go", "go.mod"), "module x\n")
        write(self.path("java", "pom.xml"), "<project/>\n")
        # Not one of C#'s dirNames, so only an implPaths entry finds it.
        write(self.path("clr-binding", "Lib.csproj"), "<Project/>\n")
        self.commit()
        result = self.hecatron("init")
        self.assertEqual(result.returncode, 0, result.stderr)
        with open(self.path(".hecatron", "config.json"), encoding="utf-8") as f:
            config = json.load(f)
        self.assertEqual(config["implPath"], "{lang}")
        self.assertEqual(config["implPaths"], {"csharp": "clr-binding"})
        self.assertEqual(project.load(self.root).impl_dirs("csharp"), [(".", self.path("clr-binding"))])

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


class LanguageDirectoryTest(Fixture):
    def test_a_language_directory_may_go_by_another_name(self):
        self.configure(languages=["csharp"], implPath="lib/{lang}")
        write(self.path(".hecatron", "languages", "csharp", "config.json"), json.dumps({"dirNames": ["netstd"]}))
        os.makedirs(self.path("lib", "netstd"))
        self.assertEqual(project.load(self.root).impl_dirs("csharp"), [(".", self.path("lib", "netstd"))])

    def test_the_languages_own_name_comes_first(self):
        self.configure(languages=["c"], implPath="{lang}")
        write(self.path(".hecatron", "languages", "c", "config.json"), json.dumps({"dirNames": ["c_glib"]}))
        os.makedirs(self.path("c"))
        os.makedirs(self.path("c_glib"))
        self.assertEqual(project.load(self.root).impl_dirs("c"), [(".", self.path("c"))])

    def test_a_directory_holding_a_detect_unless_file_isnt_the_languages(self):
        self.configure(languages=["javascript", "typescript"], implPath="{lang}")
        write(self.path(".hecatron", "languages", "javascript", "config.json"),
              json.dumps({"dirNames": ["js"], "detectUnless": ["tsconfig.json"]}))
        write(self.path("javascript", "tsconfig.json"), "{}")
        write(self.path("js", "package.json"), "{}")
        self.assertEqual(project.load(self.root).impl_dirs("javascript"), [(".", self.path("js"))])

    def test_a_language_whose_only_directory_holds_a_detect_unless_file_has_none(self):
        self.configure(languages=["javascript"], implPath="{lang}")
        write(self.path(".hecatron", "languages", "javascript", "config.json"),
              json.dumps({"detectUnless": ["tsconfig.json"]}))
        write(self.path("javascript", "tsconfig.json"), "{}")
        self.assertEqual(project.load(self.root).impl_dirs("javascript"), [])

    def test_an_implpaths_entry_is_taken_as_written(self):
        self.configure(languages=["javascript"], implPaths={"javascript": "web"})
        write(self.path(".hecatron", "languages", "javascript", "config.json"),
              json.dumps({"detectUnless": ["tsconfig.json"]}))
        write(self.path("web", "tsconfig.json"), "{}")
        self.assertEqual(project.load(self.root).impl_dirs("javascript"), [(".", self.path("web"))])

    def test_impl_paths_override_the_template_for_one_language(self):
        self.configure(languages=["go", "typescript"], components=["a", "b"], implPath="{component}/{lang}",
                       implPaths={"typescript": "{component}/javascript"})
        for directory in ("a/go", "b/go", "a/javascript", "b/javascript"):
            os.makedirs(self.path(directory))
        p = project.load(self.root)
        self.assertEqual(p.impl_dirs("typescript"), [("a", self.path("a", "javascript")),
                                                     ("b", self.path("b", "javascript"))])
        self.assertEqual(p.impl_dirs("go"), [("a", self.path("a", "go")), ("b", self.path("b", "go"))])

    def test_hooks_run_in_the_overridden_directory(self):
        self.configure(languages=["go"], implPaths={"go": "golang"})
        os.makedirs(self.path("golang"))
        write(self.path(".hecatron", "languages", "go", "test.sh"), "#!/usr/bin/env bash\npwd > ../where\n", 0o755)
        result = self.hecatron("test", "go")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        with open(self.path("where"), encoding="utf-8") as f:
            self.assertEqual(os.path.realpath(f.read().strip()), os.path.realpath(self.path("golang")))


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

    def test_dockerfile_installs_what_hecatron_needs(self):
        # The image must run hecatron itself whatever the languages: the CLI
        # needs python3, and the parallel runner perl's Time::HiRes.
        self.assertEqual(self.hecatron("dockerfile").returncode, 0)
        with open(self.path("Dockerfile"), encoding="utf-8") as f:
            content = f.read()
        for package in ("python3", "perl"):
            self.assertRegex(content, rf"\n\s+{package} \\\n")

    def test_dockerfile_sets_a_utf8_locale(self):
        # ubuntu images ship C.UTF-8 but leave LANG unset, and test suites
        # printing non-ASCII (Haskell, R) then fail.
        self.assertEqual(self.hecatron("dockerfile").returncode, 0)
        with open(self.path("Dockerfile"), encoding="utf-8") as f:
            content = f.read()
        self.assertIn("ENV LANG=C.UTF-8", content)


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

    def test_a_hook_reading_stdin_doesnt_swallow_the_other_components(self):
        # Tools like uv read stdin; the component list must not be on it.
        self.hook('cat >/dev/null; echo "ran $1"')
        result = self.hecatron("test", "go")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("ran a", result.stdout)
        self.assertIn("ran b", result.stdout)

    def test_component_flag_selects_components(self):
        self.hook('echo "ran $1"')
        result = self.hecatron("test", "go", "-c", "b")
        self.assertIn("ran b", result.stdout)
        self.assertNotIn("ran a", result.stdout)

    def test_parallel_run_keeps_the_component_flag(self):
        # A parallel run skips languages whose toolchain is missing, so use a
        # project-defined language that needs only bash.
        write(self.path(".hecatron", "languages", "fixture", "config.json"), json.dumps({"dependency": "bash"}))
        write(self.path(".hecatron", "languages", "fixture", "test.sh"), '#!/usr/bin/env bash\necho "ran $1"\n', 0o755)
        for component in ("a", "b"):
            os.makedirs(self.path(component, "fixture"))
        self.configure(languages=["fixture"], components=["a", "b"], implPath="{component}/{lang}")
        result = self.hecatron("test", "-c", "b")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        with open(self.path(".logs", "test", "fixture.log"), encoding="utf-8") as f:
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

    def test_hooks_see_their_own_language_path_only(self):
        tooldir = self.path("tooldir")
        write(os.path.join(tooldir, "mytool"), "#!/bin/sh\necho tool-ran\n", 0o755)
        write(self.path(".hecatron", "languages", "go", "config.json"), json.dumps({"path": [tooldir]}))
        self.hook('mytool')
        result = self.hecatron("test", "go")
        self.assertIn("tool-ran", result.stdout)
        script = 'eval "$(bash %s env)"; command -v mytool || echo not-on-path; _dep_available go && echo go-ok' % BIN
        out = subprocess.run(["bash", "-c", script], cwd=self.root, capture_output=True, text=True).stdout
        self.assertIn("not-on-path", out)

    def test_dep_available_counts_the_language_path(self):
        tooldir = self.path("tooldir")
        write(os.path.join(tooldir, "fixturedep"), "#!/bin/sh\n", 0o755)
        write(self.path(".hecatron", "languages", "fixture", "config.json"),
              json.dumps({"dependency": "fixturedep", "path": [tooldir]}))
        script = 'eval "$(bash %s env)"; _dep_available fixture && echo found' % BIN
        out = subprocess.run(["bash", "-c", script], cwd=self.root, capture_output=True, text=True).stdout
        self.assertIn("found", out)

    def test_env_gives_project_recipes_the_helpers(self):
        script = 'eval "$(bash %s env)"; echo "$_ADDED_LANGS"; _name rs' % BIN
        result = subprocess.run(["bash", "-c", script], cwd=self.root, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.split(), ["go", "rust"])

    def test_help_prints_the_env_example_literally(self):
        result = self.hecatron("help")
        self.assertEqual(result.stderr, "")
        self.assertIn('eval "$(hecatron env)"', result.stdout)

    def test_outside_a_project_is_an_error(self):
        with tempfile.TemporaryDirectory() as outside:
            result = subprocess.run(["bash", BIN, "test"], cwd=outside, capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("hecatron init", result.stderr)


if __name__ == "__main__":
    unittest.main()
