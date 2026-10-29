"""hct docs and hct bench: the default docs and bench hooks' choices (each tool
a fake on PATH that prints how it was called), the real ones for Rust and
Go where their toolchains are on this machine, and a language with nothing
to run."""

from __future__ import annotations

import os
import re
import shutil
import subprocess
import sys
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(HOME, "tests"))

from test_hecatron import Fixture, write  # noqa: E402

FIXTURES = os.path.join(HOME, "tests", "hooks", "fixtures")


class HookTest(Fixture):
    def setUp(self):
        super().setUp()
        self.bin = self.path(".fakebin")
        os.makedirs(self.bin)

    def fake(self, tool: str, body: str = "") -> None:
        write(os.path.join(self.bin, tool), f'#!/bin/sh\n{body}\necho "{tool} $*"\n', 0o755)

    def hook(self, lang: str, action: str, directory: str = ".", path: str | None = None,
             **env) -> subprocess.CompletedProcess:
        environment = {**os.environ, "PATH": path or self.bin + os.pathsep + os.environ["PATH"],
                       "HECATRON_LANG": lang, "HECATRON_ROOT": self.root, "HECATRON_HOME": "", **env}
        return subprocess.run(["bash", os.path.join(HOME, "languages", lang, "hooks", action + ".sh")],
                              cwd=self.path(directory), env=environment, capture_output=True, text=True,
                              stdin=subprocess.DEVNULL)

    def passes(self, result: subprocess.CompletedProcess) -> str:
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return result.stdout

    def skips(self, result: subprocess.CompletedProcess, why: str) -> None:
        output = self.passes(result)
        self.assertIn(why, output)
        self.assertIn("(skipping)", output)


class DocsHookTest(HookTest):
    def test_python_builds_sphinx_docs_into_their_build_directory(self):
        write(self.path("pyproject.toml"), '[project]\nname = "x"\n')
        write(self.path("docs", "conf.py"), "")
        write(self.path("docs", "requirements.txt"), "furo\n")
        self.fake("uv")
        output = self.passes(self.hook("python", "docs"))
        self.assertIn("uv run --all-extras --all-groups --with sphinx --with-requirements docs/requirements.txt "
                      "sphinx-build -M html docs docs/_build", output)
        self.assertIn("python: docs: written to docs/_build/html", output)

    def test_python_sphinx_with_separate_source_and_build(self):
        write(self.path("setup.py"), "")
        write(self.path("docs", "source", "conf.py"), "")
        self.fake("uv")
        output = self.passes(self.hook("python", "docs"))
        self.assertIn("sphinx-build -M html docs/source docs/build", output)
        self.assertIn("written to docs/build/html", output)

    def test_python_pdoc_documents_the_packages_where_it_is_a_dependency(self):
        write(self.path("pyproject.toml"), '[dependency-groups]\ndocs = ["pdoc>=15"]\n')
        write(self.path("src", "addlib", "__init__.py"), "")
        write(self.path("tests", "__init__.py"), "")
        self.fake("uv")
        output = self.passes(self.hook("python", "docs"))
        self.assertIn("--with pdoc pdoc -o build/pdoc src/addlib\n", output)
        self.assertIn("written to build/pdoc", output)

    def test_python_without_a_docs_tool_skips(self):
        write(self.path("pyproject.toml"), '[project]\ndependencies = ["pdocs-helper"]\n')
        self.fake("uv")
        self.skips(self.hook("python", "docs"), "no docs/conf.py for Sphinx, and pdoc isn't among the dependencies")

    def test_gradle_javadoc_and_dokka(self):
        write(self.path("build.gradle"), "")
        self.fake("gradle")
        output = self.passes(self.hook("java", "docs"))
        self.assertIn("gradle javadoc", output)
        self.assertIn("written to build/docs/javadoc (in each project)", output)
        self.skips(self.hook("kotlin", "docs"), "no Dokka in the Gradle build")

    def test_dokka_1_runs_where_dokka_2s_task_is_missing(self):
        write(self.path("build.gradle.kts"), 'plugins { id("org.jetbrains.dokka") version "1.9.20" }\n')
        self.fake("gradle", "case \"$1\" in dokkaGenerate) echo \"Task 'dokkaGenerate' not found in root project 'x'.\"; exit 1 ;; esac")
        output = self.passes(self.hook("kotlin", "docs"))
        self.assertIn("gradle dokkaHtml", output)
        self.assertIn("written to build/dokka/html", output)

    def test_maven_javadoc(self):
        write(self.path("pom.xml"), "<project/>")
        self.fake("mvn", "mkdir -p target/reports/apidocs")
        output = self.passes(self.hook("java", "docs"))
        self.assertIn("mvn -B javadoc:javadoc", output)
        self.assertIn("written to target/reports/apidocs (in each module)", output)

    def test_typedoc_where_it_is_a_dependency(self):
        write(self.path("package.json"), '{"devDependencies": {"typedoc": "^0.28"}}')
        write(self.path("package-lock.json"), "{}")
        write(self.path("node_modules", ".bin", "typedoc"), '#!/bin/sh\necho "typedoc $*"\n', 0o755)
        output = self.passes(self.hook("typescript", "docs"))
        self.assertIn("typedoc \n", output)
        self.assertIn("typescript: docs: written to docs", output)

    def test_a_package_without_docs_skips(self):
        write(self.path("package.json"), '{"scripts": {"test": "x"}}')
        self.skips(self.hook("javascript", "docs"), "package.json has no docs script")

    def test_doxygen_from_the_doxyfile_above(self):
        write(self.path("Doxyfile"), 'PROJECT_NAME = x\nOUTPUT_DIRECTORY = "build/doc"\n')
        os.makedirs(self.path("src", "c"))
        self.fake("doxygen", "pwd")
        output = self.passes(self.hook("c", "docs", "src/c"))
        self.assertIn(self.root + "\ndoxygen Doxyfile", output)
        self.assertIn("c: docs: written to build/doc/html", output)

    def test_doxygen_missing_fails_where_there_is_a_doxyfile(self):
        write(self.path("Doxyfile"), "")
        minimal = self.path(".minbin")
        os.makedirs(minimal)
        for tool in ("bash", "dirname", "basename", "sed", "tail", "pwd"):
            found = shutil.which(tool)
            if found:
                os.symlink(found, os.path.join(minimal, tool))
        result = self.hook("cpp", "docs", path=minimal)
        self.assertEqual(result.returncode, 1, result.stdout)
        self.assertIn("Doxygen isn't installed", result.stderr)

    def test_nothing_to_document_skips(self):
        self.skips(self.hook("cpp", "docs"), "no Doxyfile here or above it in the project")
        write(self.path("build.zig"), "")
        self.skips(self.hook("zig", "docs"), "build.zig defines no docs step")
        write(self.path("mix.exs"), "")
        self.skips(self.hook("elixir", "docs"), "ExDoc isn't among mix.exs's dependencies")

    def test_zig_docs_step(self):
        write(self.path("build.zig"), 'const docs_step = b.step("docs", "Build the docs");\n')
        self.fake("zig")
        self.assertIn("zig build docs", self.passes(self.hook("zig", "docs")))

    def test_docfx_where_the_project_has_it_as_a_tool(self):
        write(self.path(".config", "dotnet-tools.json"), '{"tools": {"docfx": {"version": "2.78.3"}}}')
        write(self.path("docs", "docfx.json"), "{}")
        os.makedirs(self.path("src"))
        self.fake("dotnet")
        output = self.passes(self.hook("csharp", "docs", "src"))
        self.assertIn("dotnet tool restore", output)
        self.assertIn(f"dotnet docfx {self.root}/docs/docfx.json", output)
        self.assertIn("csharp: docs: written to docs/_site", output)


class BenchHookTest(HookTest):
    def test_python_runs_only_benchmarks_where_pytest_benchmark_is_a_dependency(self):
        write(self.path("pyproject.toml"), '[dependency-groups]\ndev = ["pytest-benchmark"]\n')
        self.fake("uv")
        output = self.passes(self.hook("python", "bench"))
        self.assertIn("uv run --all-extras --all-groups --with pytest --with pytest-benchmark pytest --benchmark-only",
                      output)

    def test_python_without_pytest_benchmark_skips(self):
        write(self.path("pyproject.toml"), "")
        self.fake("uv")
        self.skips(self.hook("python", "bench"), "pytest-benchmark isn't among the dependencies")

    def test_go_without_benchmarks_skips(self):
        write(self.path("go.mod"), "module m\n")
        write(self.path("a_test.go"), "package m\n\nfunc TestA(t *testing.T) {}\n")
        self.fake("go")
        self.skips(self.hook("go", "bench"), "no Benchmark functions in the tests")
        write(self.path("b_test.go"), "package m\n\nfunc BenchmarkB(b *testing.B) {}\n")
        self.assertIn("go test -run ^$ -bench . ./...", self.passes(self.hook("go", "bench")))

    def test_rust_without_benchmarks_skips(self):
        write(self.path("Cargo.toml"), "[package]\nname = \"x\"\n")
        self.fake("cargo")
        self.skips(self.hook("rust", "bench"), "no benches/ directory or [[bench]] target")
        os.makedirs(self.path("crates", "core", "benches"))
        self.assertIn("cargo bench", self.passes(self.hook("rust", "bench")))

    def test_benchmarkdotnet_projects_of_the_language(self):
        write(self.path("App.sln"), "")
        write(self.path("bench", "Bench.csproj"), '<PackageReference Include="BenchmarkDotNet" Version="0.15.4" />')
        write(self.path("bench", "Bench.fsproj"), '<PackageReference Include="BenchmarkDotNet" />')
        write(self.path("src", "App.csproj"), "")
        self.fake("dotnet")
        output = self.passes(self.hook("csharp", "bench"))
        self.assertIn("dotnet run -c Release --project ./bench/Bench.csproj -- --filter *", output)
        self.assertNotIn("fsproj", output)
        self.assertNotIn("App.csproj", output)
        self.skips(self.hook("vbnet", "bench"), "no project references BenchmarkDotNet")

    def test_gradle_jmh(self):
        write(self.path("settings.gradle.kts"), 'include(":bench")\n')
        write(self.path("bench", "build.gradle.kts"), 'plugins { id("me.champeau.jmh") version "0.7.3" }\n')
        self.fake("gradle")
        self.assertIn("gradle jmh", self.passes(self.hook("java", "bench", "bench")))
        write(self.path("bench", "build.gradle.kts"), "")
        self.skips(self.hook("kotlin", "bench", "bench"), "no JMH or kotlinx-benchmark plugin")

    def test_a_package_bench_script(self):
        write(self.path("package.json"), '{"scripts": {"benchmark": "node bench.js"}}')
        os.makedirs(self.path("node_modules"))
        self.fake("npm")
        self.assertIn("npm run benchmark", self.passes(self.hook("javascript", "bench")))

    def test_makefile_bench_target(self):
        write(self.path("Makefile"), "all:\n\ttrue\nbench:\n\ttrue\n")
        self.assertIn("+ make bench", self.passes(self.hook("c", "bench")))
        write(self.path("Makefile"), "all:\n\ttrue\n")
        self.skips(self.hook("c", "bench"), "the Makefile has no bench or benchmark target")

    def test_zig_bench_step(self):
        write(self.path("build.zig"), 'const s = b.step("benchmark", "Run the benchmarks");\n')
        self.fake("zig")
        self.assertIn("zig build benchmark", self.passes(self.hook("zig", "bench")))


def copy_fixture(test: Fixture, lang: str, name: str) -> str:
    destination = test.path(lang)
    shutil.copytree(os.path.join(FIXTURES, lang, name), destination)
    return destination


class RealToolsTest(HookTest):
    """The hooks with the real toolchains, where this machine has them."""

    def real(self, lang: str, action: str) -> str:
        return self.passes(self.hook(lang, action, lang, path=os.environ["PATH"]))

    @unittest.skipUnless(shutil.which("cargo"), "no cargo")
    def test_rust_docs_and_bench(self):
        copy_fixture(self, "rust", "cargo")
        target = self.path(".target")
        output = self.passes(self.hook("rust", "docs", "rust", path=os.environ["PATH"], CARGO_TARGET_DIR=target))
        written = re.search(r"rust: docs: written to (.+)", output).group(1)
        self.assertEqual(written, ".target/doc")
        self.assertTrue(os.path.isfile(self.path(".target", "doc", "add", "index.html")))
        output = self.passes(self.hook("rust", "bench", "rust", path=os.environ["PATH"], CARGO_TARGET_DIR=target))
        self.assertIn("add: 1000000 calls in", output)

    @unittest.skipUnless(shutil.which("go"), "no go")
    def test_go_bench(self):
        copy_fixture(self, "go", "module")
        cache = self.path(".gocache")
        output = self.passes(self.hook("go", "bench", "go", path=os.environ["PATH"], GOCACHE=cache,
                                       GOFLAGS="-mod=mod", GOTOOLCHAIN="local"))
        self.assertRegex(output, r"BenchmarkAdd\S*\s+\d+\s+[\d.]+ ns/op")
        self.assertNotIn("--- PASS: TestAdd", output)


class CommandTest(Fixture):
    """The commands, through bin/hecatron."""

    def setUp(self):
        super().setUp()
        write(self.path("go", "go.mod"), "module m\n")
        write(self.path("elm", "elm.json"), "{}")
        self.configure(languages=["go", "elm"])

    def test_a_language_without_a_docs_tool_says_so(self):
        result = self.hecatron("docs", "go")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("No docs tool for go (skipping)", result.stdout)
        result = self.hecatron("bench", "elm")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("No benchmark runner for elm (skipping)", result.stdout)

    def test_with_no_targets_only_languages_with_a_hook_run(self):
        result = self.hecatron("docs")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("Nothing to docs.", result.stdout)

    def test_help_lists_the_languages_with_a_hook(self):
        result = self.hecatron("bench", "help")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("Run each language's benchmarks for every component", result.stdout)
        self.assertIn("go", result.stdout)
        self.assertNotIn("elm", result.stdout.split("Flags:")[0])


if __name__ == "__main__":
    unittest.main()
