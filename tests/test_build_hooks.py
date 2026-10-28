"""The default build hooks' choices for JVM and .NET layouts
(languages/_hooks.sh): each tool is a fake on PATH that prints how it was
called. internal/actions' comparison checks the plain cases against the Go
plans; these are the ones a plan can't show (a retry on the tool's output,
the environment a command gets, installers' view of builds below)."""

from __future__ import annotations

import os
import subprocess
import sys
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(HOME, "tests"))

from test_hecatron import Fixture, write  # noqa: E402

COMMON = os.path.join(HOME, "languages", "_common.sh")


class BuildHookTest(Fixture):
    def setUp(self):
        super().setUp()
        self.bin = self.path(".fakebin")

    def fake(self, tool: str, body: str = "") -> None:
        write(os.path.join(self.bin, tool), f'#!/bin/sh\n{body}\necho "{tool} $*"\n', 0o755)

    def hook(self, lang: str, action: str, directory: str, **env) -> subprocess.CompletedProcess:
        environment = {**os.environ, "PATH": self.bin + os.pathsep + os.environ["PATH"], "HECATRON_LANG": lang,
                       "HECATRON_ROOT": self.root, "HECATRON_HOME": "", **env}
        return subprocess.run(["bash", os.path.join(HOME, "languages", lang, "hooks", action + ".sh")],
                              cwd=self.path(directory), env=environment, capture_output=True, text=True)


class GradleTest(BuildHookTest):
    def test_a_multiplatform_build_without_a_test_task_runs_jvm_tests(self):
        write(self.path("settings.gradle.kts"), 'include(":lib")\n')
        write(self.path("lib", "build.gradle.kts"), "")
        self.fake("gradle", "case \"$1\" in test) echo \"Task 'test' not found in root project 'x'.\"; exit 1 ;; esac")
        result = self.hook("kotlin", "test", "lib")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("no test task (a Kotlin Multiplatform build)", result.stdout)
        self.assertIn("gradle jvmTest", result.stdout)
        self.assertNotIn("allTests", result.stdout)

    def test_a_multiplatform_build_without_a_jvm_target_runs_all_tests(self):
        write(self.path("build.gradle.kts"), "")
        self.fake("gradle", "case \"$1\" in test|jvmTest) echo \"Task '$1' not found in root project 'x'.\"; exit 1 ;; esac")
        result = self.hook("kotlin", "test", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("gradle allTests", result.stdout)

    def test_another_failure_is_not_retried(self):
        write(self.path("build.gradle"), "")
        self.fake("gradle", "exit 1")
        result = self.hook("java", "test", ".")
        self.assertEqual(result.returncode, 1)
        self.assertNotIn("allTests", result.stdout)

    def test_a_source_directory_runs_its_project(self):
        write(self.path("settings.gradle"), "include 'core'\n")
        write(self.path("core", "build.gradle"), "")
        os.makedirs(self.path("core", "src", "main", "scala"))
        self.fake("gradle", 'pwd')
        result = self.hook("scala", "test", "core/src/main/scala")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("is in the Gradle project " + os.path.realpath(self.path("core")), result.stdout)
        self.assertIn(os.path.realpath(self.path("core")) + "\n", result.stdout)

    def test_the_jvm_gets_a_utf8_locale_when_none_is_set(self):
        write(self.path("build.gradle"), "")
        self.fake("gradle", 'echo "LANG=${LANG:-}"')
        env = {key: "" for key in ("LANG", "LC_ALL", "LC_CTYPE")}
        self.assertIn("LANG=C.UTF-8", self.hook("java", "test", ".", **env).stdout)
        self.assertIn("LANG=en_GB.UTF-8", self.hook("java", "test", ".", LANG="en_GB.UTF-8", LC_ALL="").stdout)


class JdkTest(BuildHookTest):
    """jvm_select_jdk, with a fake hecatron home listing the installed JDKs."""

    def setUp(self):
        super().setUp()
        self.home = self.path(".fakehome")
        os.makedirs(os.path.join(self.home, "libexec", "lib"))
        os.symlink(os.path.join(HOME, "libexec", "lib", "jvm.py"), os.path.join(self.home, "libexec", "lib", "jvm.py"))
        self.jdk = self.path(".jdk21")
        write(os.path.join(self.jdk, "bin", "javac"), "#!/bin/sh\n", 0o755)
        write(os.path.join(self.home, "languages", "java", "installed.sh"), f'printf "21.0.2\\t{self.jdk}\\n"\n')
        write(self.path("build.gradle.kts"), "")
        self.fake("gradle", 'echo "JAVA_HOME=${JAVA_HOME:-}"')

    def test_a_runtime_without_javac_gives_way_to_an_installed_jdk(self):
        write(os.path.join(self.bin, "java"), "#!/bin/sh\necho 'openjdk version \"21.0.12\"' >&2\n", 0o755)
        result = self.hook("kotlin", "test", ".", HECATRON_HOME=self.home, JAVA_HOME="")
        self.assertIn("has no javac", result.stdout)
        self.assertIn("JAVA_HOME=" + self.jdk, result.stdout)

    def test_a_jdk_with_javac_is_kept(self):
        write(os.path.join(self.bin, "java"), "#!/bin/sh\n", 0o755)
        write(os.path.join(self.bin, "javac"), "#!/bin/sh\n", 0o755)
        result = self.hook("kotlin", "test", ".", HECATRON_HOME=self.home, JAVA_HOME="")
        self.assertNotIn("Using JDK", result.stdout)
        self.assertIn("JAVA_HOME=\n", result.stdout)


class DotnetTest(BuildHookTest):
    TEST_PROJECT = '<Project><ItemGroup><PackageReference Include="Microsoft.NET.Test.Sdk" /></ItemGroup></Project>'

    def setUp(self):
        super().setUp()
        self.fake("dotnet", 'echo "roll=${DOTNET_ROLL_FORWARD:-}"')
        os.makedirs(self.path("cs"))

    def test_several_projects_without_a_solution_test_each_test_project(self):
        write(self.path("cs", "lib.csproj"), "<Project />")
        write(self.path("cs", "lib.Tests.csproj"), self.TEST_PROJECT)
        result = self.hook("csharp", "test", "cs")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("dotnet test lib.Tests.csproj", result.stdout)
        self.assertNotIn("dotnet test lib.csproj", result.stdout)
        result = self.hook("csharp", "build", "cs")
        self.assertIn("dotnet build lib.Tests.csproj", result.stdout)
        self.assertIn("dotnet build lib.csproj", result.stdout)

    def test_several_projects_none_a_test_project_skip_test(self):
        write(self.path("cs", "a.csproj"), "<Project />")
        write(self.path("cs", "b.csproj"), "<Project />")
        result = self.hook("csharp", "test", "cs")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("no test project among a.csproj b.csproj (skipping)", result.stdout)

    def test_one_project_lets_dotnet_choose(self):
        write(self.path("cs", "lib.csproj"), "<Project />")
        result = self.hook("csharp", "test", "cs")
        self.assertIn("+ dotnet test\n", result.stdout)

    def test_tests_roll_forward_to_the_installed_runtime(self):
        write(self.path("cs", "lib.csproj"), "<Project />")
        self.assertIn("roll=Major", self.hook("csharp", "test", "cs").stdout)
        self.assertIn("roll=LatestMinor", self.hook("csharp", "test", "cs", DOTNET_ROLL_FORWARD="LatestMinor").stdout)


class ClojureTest(BuildHookTest):
    def test_leiningen_tests_when_deps_edn_has_no_test_alias(self):
        write(self.path("deps.edn"), "{:deps {}}\n")
        write(self.path("project.clj"), "(defproject x \"1\")\n")
        self.fake("clojure", "exit 0")
        self.fake("lein")
        result = self.hook("clojure", "test", ".")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("lein test", result.stdout)

    def test_without_project_clj_it_is_unknown(self):
        write(self.path("deps.edn"), "{:deps {}}\n")
        self.fake("clojure", "exit 0")
        result = self.hook("clojure", "test", ".")
        self.assertEqual(result.returncode, 1)
        self.assertIn("no :test alias", result.stderr)


class JvmBuildToolsTest(BuildHookTest):
    def missing(self, lang: str) -> str:
        script = f"source {COMMON}\nimpl_dirs() {{ echo \"{self.path(lang)}\"; }}\njvm_build_tools_missing {lang}"
        self.fake("mvn", "exit 1")  # no Maven of 3.9 or later, whatever this machine has
        env = {**os.environ, "PATH": self.bin + os.pathsep + os.environ["PATH"], "HECATRON_HOME": "", "ROOT": self.root}
        result = subprocess.run(["bash", "-c", script], cwd=self.root, env=env, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        return result.stdout

    def test_a_build_below_the_language_directory_needs_its_tool(self):
        write(self.path("java", "packages", "pom.xml"), "<project />")
        self.assertEqual(self.missing("java"), "maven\n")

    def test_a_build_with_its_wrapper_needs_none(self):
        write(self.path("java", "packages", "pom.xml"), "<project />")
        write(self.path("java", "packages", "mvnw"), "#!/bin/sh\n", 0o755)
        self.assertEqual(self.missing("java"), "")


if __name__ == "__main__":
    unittest.main()
