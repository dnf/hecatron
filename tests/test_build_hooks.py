"""The default build hooks' choices for JVM and .NET layouts
(languages/_hooks.sh): each tool is a fake on PATH that prints how it was
called. internal/actions' comparison checks the plain cases against the Go
plans; these are the ones a plan can't show (a retry on the tool's output,
the environment a command gets, installers' view of builds below)."""

from __future__ import annotations

import json
import os
import platform
import shutil
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
        # xcodebuild's derived data in the fixture, not ~/Library.
        environment = {**os.environ, "PATH": self.bin + os.pathsep + os.environ["PATH"], "HECATRON_LANG": lang,
                       "HECATRON_ROOT": self.root, "HECATRON_HOME": "",
                       "HECATRON_XCODE_DERIVED_DATA": self.path(".derived-data"), **env}
        # The hooks ask hct's binary when it runs them: a test gives a fake.
        if "HECATRON_BINARY" not in env:
            environment.pop("HECATRON_BINARY", None)
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

    def android_app(self) -> None:
        write(self.path("settings.gradle.kts"), 'include(":app", ":core")\n')
        write(self.path("build.gradle.kts"), "")
        write(self.path("app", "build.gradle.kts"), "plugins { alias(libs.plugins.android.application) }\n")
        write(self.path("app", "src", "main", "AndroidManifest.xml"), "<manifest/>\n")
        write(self.path("core", "build.gradle.kts"), "")

    def test_an_android_app_builds_its_debug_variants(self):
        # Release needs a signing config and services files only the
        # maintainers have (AntennaPod's validateSigningFreeRelease).
        self.android_app()
        self.fake("gradle")
        result = self.hook("kotlin", "build", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("gradle assembleDebug", result.stdout)
        self.assertNotIn("gradle assemble\n", result.stdout)

    def test_an_android_app_runs_its_debug_unit_tests(self):
        self.android_app()
        self.fake("gradle")
        result = self.hook("kotlin", "test", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("gradle testDebugUnitTest", result.stdout)
        self.assertNotIn("gradle test\n", result.stdout)

    def test_an_android_build_without_debug_unit_tests_runs_test(self):
        self.android_app()
        self.fake("gradle", "case \"$1\" in testDebugUnitTest) echo \"Task 'testDebugUnitTest' not found in root project 'x'.\"; exit 1 ;; esac")
        result = self.hook("kotlin", "test", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("no testDebugUnitTest task\n", result.stdout)
        self.assertIn("gradle test\n", result.stdout)

    def test_a_manifest_in_build_output_isnt_android(self):
        write(self.path("build.gradle.kts"), "")
        write(self.path("build", "intermediates", "src", "main", "AndroidManifest.xml"), "<manifest/>\n")
        self.fake("gradle")
        self.assertIn("gradle assemble\n", self.hook("kotlin", "build", ".").stdout)

    def test_an_aosp_layout_android_app_builds_its_debug_variants(self):
        # Lawnchair: Launcher3's manifest beside its build.gradle, sources in src/.
        write(self.path("settings.gradle"), "include ':shared'\n")
        write(self.path("build.gradle"), "")
        write(self.path("AndroidManifest.xml"), "<manifest/>\n")
        write(self.path("docs", "AndroidManifest.xml"), "<manifest/>\n")
        self.fake("gradle")
        self.assertIn("gradle assembleDebug", self.hook("java", "build", ".").stdout)

    def test_a_manifest_away_from_a_build_file_isnt_android(self):
        write(self.path("build.gradle"), "")
        write(self.path("docs", "AndroidManifest.xml"), "<manifest/>\n")
        self.fake("gradle")
        self.assertIn("gradle assemble\n", self.hook("java", "build", ".").stdout)

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

    def test_one_gradle_run_at_a_time_in_a_build(self):
        # Lawnchair's java and kotlin jobs ran one Gradle build at once and
        # overwrote each other's generated sources.
        write(self.path("build.gradle"), "")
        write(self.path("gradlew"), '#!/bin/sh\n'
              'if [ -e running ]; then echo overlap; exit 1; fi\n'
              'touch running; sleep 1; rm running; echo "gradlew $*"\n', 0o755)
        environment = {**os.environ, "HECATRON_ROOT": self.root, "HECATRON_HOME": ""}
        jobs = [subprocess.Popen(["bash", os.path.join(HOME, "languages", lang, "hooks", "build.sh")],
                                 cwd=self.root, env={**environment, "HECATRON_LANG": lang},
                                 stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
                for lang in ("java", "kotlin")]
        outputs = [job.communicate()[0] for job in jobs]
        self.assertEqual([job.returncode for job in jobs], [0, 0], outputs)
        self.assertIn("waiting for another Gradle run in " + os.path.realpath(self.root), "".join(outputs))
        self.assertFalse(os.path.exists(self.path(".gradle", "hecatron.lock")))

    def test_a_lock_left_by_a_killed_run_is_taken_over(self):
        write(self.path("build.gradle"), "")
        write(self.path(".gradle", "hecatron.lock", "pid"), "999999\n")
        self.fake("gradle")
        result = self.hook("java", "build", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertNotIn("waiting", result.stdout)

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

    def java_on_path(self, version: str) -> None:
        write(os.path.join(self.bin, "java"), f"#!/bin/sh\necho 'openjdk version \"{version}\"' >&2\n", 0o755)
        write(os.path.join(self.bin, "javac"), "#!/bin/sh\n", 0o755)

    def test_a_build_for_a_newer_java_runs_on_a_jdk_that_compiles_it(self):
        # nextcloud's app compiles for Java 21: on JDK 17, "invalid source release: 21".
        self.java_on_path("17.0.11")
        write(self.path("build.gradle.kts"), "android { compileOptions { sourceCompatibility = JavaVersion.VERSION_21 } }\n")
        result = self.hook("kotlin", "test", ".", HECATRON_HOME=self.home, JAVA_HOME="")
        self.assertIn("it compiles for Java 21 (build.gradle.kts), which needs JDK 21 or later.", result.stdout)
        self.assertIn("JAVA_HOME=" + self.jdk, result.stdout)

    def test_java_home_is_the_jdk_the_build_runs_on(self):
        # A runner's JAVA_HOME names JDK 17 while PATH finds a newer java:
        # Gradle runs on JAVA_HOME's (Lawnchair's java build).
        self.java_on_path("25.0.1")
        old = self.path(".jdk17")
        write(os.path.join(old, "bin", "java"), "#!/bin/sh\necho 'openjdk version \"17.0.11\"' >&2\n", 0o755)
        write(os.path.join(old, "bin", "javac"), "#!/bin/sh\n", 0o755)
        write(self.path("build.gradle.kts"), "java { sourceCompatibility = JavaVersion.VERSION_21 }\n")
        result = self.hook("kotlin", "test", ".", HECATRON_HOME=self.home, JAVA_HOME=old)
        self.assertIn("JAVA_HOME=" + self.jdk, result.stdout)

    def test_a_pinned_jdk_is_used(self):
        self.java_on_path("17.0.11")
        write(self.path(".java-version"), "21.0\n")
        result = self.hook("kotlin", "build", ".", HECATRON_HOME=self.home, JAVA_HOME="")
        self.assertIn("it pins JDK 21 (.java-version), which needs JDK 21 or later.", result.stdout)
        self.assertIn("JAVA_HOME=" + self.jdk, result.stdout)

    def test_gradle_reruns_on_the_jdk_a_plugin_needs(self):
        # Metro's Gradle plugin needs JDK 21 to run (kotlinconf-app, Mihon);
        # nothing in the build files says so.
        self.java_on_path("17.0.11")
        self.fake("gradle", f'if [ "${{JAVA_HOME:-}}" != "{self.jdk}" ]; then\n'
                  "  echo '      > Dependency requires at least JVM runtime version 21. This build uses a Java 17 JVM.'\n"
                  "  exit 1\nfi")
        result = self.hook("kotlin", "build", ".", HECATRON_HOME=self.home, JAVA_HOME="")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("Using JDK 21.0.2 (" + self.jdk + "): Gradle says a plugin the build applies needs a newer JVM, "
                      "which needs JDK 21 or later.", result.stdout)
        self.assertEqual(result.stdout.count("+ gradle assemble"), 2)

    def test_gradle_isnt_rerun_without_a_newer_jdk(self):
        self.java_on_path("21.0.4")
        write(os.path.join(self.bin, "javac"), "#!/bin/sh\n", 0o755)
        self.fake("gradle", "echo '> Dependency requires at least JVM runtime version 25. This build uses a Java 21 JVM.'; exit 1")
        result = self.hook("kotlin", "build", ".", HECATRON_HOME=self.home, JAVA_HOME="")
        self.assertEqual(result.returncode, 1)
        self.assertEqual(result.stdout.count("+ gradle assemble"), 1)
        self.assertIn("needs JDK 25 or later, and no such JDK is installed", result.stderr)

    def test_a_jdk_at_the_floor_or_later_is_kept(self):
        self.java_on_path("21.0.4")
        write(self.path("build.gradle.kts"), "java { sourceCompatibility = JavaVersion.VERSION_17 }\n")
        result = self.hook("kotlin", "test", ".", HECATRON_HOME=self.home, JAVA_HOME="")
        self.assertNotIn("Using JDK", result.stdout)
        self.assertIn("JAVA_HOME=\n", result.stdout)

    def test_no_jdk_new_enough_warns(self):
        self.java_on_path("21.0.4")
        write(self.path("build.gradle.kts"), "java { sourceCompatibility = JavaVersion.VERSION_25 }\n")
        result = self.hook("kotlin", "test", ".", HECATRON_HOME=self.home, JAVA_HOME="")
        self.assertIn("which needs JDK 25 or later, and no such JDK is installed (hecatron install kotlin)\n", result.stderr)
        self.assertIn("JAVA_HOME=\n", result.stdout)

    def test_without_a_jdk_new_enough_the_newest_installed_is_used(self):
        # Quran's CI pins a JDK hecatron can't install yet; its build runs on
        # 21, and Metro's plugin won't run on the runner's 17.
        self.java_on_path("17.0.11")
        write(self.path(".java-version"), "27\n")
        result = self.hook("kotlin", "build", ".", HECATRON_HOME=self.home, JAVA_HOME="")
        self.assertIn("it pins JDK 27 (.java-version), which needs JDK 27 or later, and no such JDK is installed "
                      f"(hecatron install kotlin): using the newest installed, JDK 21.0.2 ({self.jdk})", result.stderr)
        self.assertIn("JAVA_HOME=" + self.jdk, result.stdout)

    def test_a_jdk_with_javac_is_kept(self):
        write(os.path.join(self.bin, "java"), "#!/bin/sh\n", 0o755)
        write(os.path.join(self.bin, "javac"), "#!/bin/sh\n", 0o755)
        result = self.hook("kotlin", "test", ".", HECATRON_HOME=self.home, JAVA_HOME="")
        self.assertNotIn("Using JDK", result.stdout)
        self.assertIn("JAVA_HOME=\n", result.stdout)


class JdksMissingTest(BuildHookTest):
    """jvm_jdks_missing (languages/_common.sh): the JDKs hct install adds."""

    def missing(self, installed: str, build: str) -> list[str]:
        home = self.path(".fakehome")
        os.makedirs(os.path.join(home, "libexec", "lib"), exist_ok=True)
        if not os.path.exists(os.path.join(home, "libexec", "lib", "jvm.py")):
            os.symlink(os.path.join(HOME, "libexec", "lib", "jvm.py"), os.path.join(home, "libexec", "lib", "jvm.py"))
        write(os.path.join(home, "languages", "java", "installed.sh"), f'printf "{installed}"\n')
        write(self.path("java", "build.gradle.kts"), build)
        script = f'source {COMMON}\nimpl_dirs() {{ echo "{self.path("java")}"; }}\njvm_jdks_missing java'
        env = {**os.environ, "HECATRON_HOME": home, "HECATRON_ROOT": self.root}
        result = subprocess.run(["bash", "-c", script], cwd=self.root, env=env, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        return result.stdout.split()

    def test_a_build_for_a_newer_java_gets_the_oldest_lts_that_compiles_it(self):
        self.assertEqual(self.missing("17.0.11\\t/jdk17\\n", "java { sourceCompatibility = JavaVersion.VERSION_19 }\n"), ["21"])

    def test_nothing_when_an_installed_jdk_compiles_it(self):
        self.assertEqual(self.missing("21.0.2\\t/jdk21\\n", "java { sourceCompatibility = JavaVersion.VERSION_21 }\n"), [])
        self.assertEqual(self.missing("17.0.11\\t/jdk17\\n", ""), [])


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


class PythonRequirementsTest(BuildHookTest):
    """An application with only a requirements.txt: detection takes it for
    Python, so its tests run with those packages, or skip without tests."""

    def test_its_tests_run_with_the_requirements(self):
        write(self.path("requirements.txt"), "flask\n")
        write(self.path("requirements-dev.txt"), "pytest-mock\n")
        write(self.path("app.py"), "")
        write(self.path("tests", "test_app.py"), "")
        self.fake("uv")
        result = self.hook("python", "test", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("uv run --with-requirements requirements-dev.txt --with-requirements requirements.txt"
                      " --with pytest pytest", result.stdout)

    def test_without_test_files_it_skips(self):
        write(self.path("requirements.txt"), "flask\n")
        write(self.path("app.py"), "")
        self.fake("uv")
        result = self.hook("python", "test", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("python: test: no test files", result.stdout)
        self.assertNotIn("uv run", result.stdout)


class XcodeTest(BuildHookTest):
    """An app with an Xcode project: xcodebuild builds the scheme named like
    it, for a generic destination its supported platforms give, into
    .build/xcode; its tests, which need a simulator, skip saying so. The
    fake xcodebuild logs each call and answers -list -json and
    -showBuildSettings from fixture text, shaped as Xcode 27's output for
    IceCubesApp (28 schemes: one per Swift package) and NetNewsWire."""

    SETTINGS = ("Build settings for action build and target {scheme}:\n    PLATFORM_NAME = {platform}\n"
                "    SDKROOT = {sdkroot}\n    SUPPORTED_PLATFORMS = {platforms}\n"
                "    SUPPORTS_MACCATALYST = {catalyst}\n"
                "Build settings for action build and target Other:\n    SDKROOT = macosx\n"
                "    SUPPORTED_PLATFORMS = macosx\n")
    SDKS = "/Applications/Xcode.app/Contents/Developer/Platforms/{0}.platform/Developer/SDKs/{0}26.0.sdk"
    PACKAGES = [f"Package{n}" for n in range(26)]
    IOS_AND_VISION = "iphoneos iphonesimulator xros xrsimulator"

    def setUp(self):
        super().setUp()
        write(self.path("App.xcodeproj", "project.pbxproj"), "")
        write(self.path("App.xcodeproj", "project.xcworkspace", "contents.xcworkspacedata"), "")
        write(self.path("App", "AppDelegate.swift"), "")
        write(self.path("App", "Legacy.m"), "")
        self.log = self.path(".xcodebuild.log")
        self.xcodebuild(self.PACKAGES + ["App"])

    def xcodebuild(self, schemes: list[str], platforms: str = IOS_AND_VISION, kind: str = "project",
                   sdk: str = "iPhoneOS", catalyst: str = "NO", version: str = "26.6", status: int = 0) -> None:
        """The fake xcodebuild: of Xcode version, its builds exiting with
        status, and recording the DEVELOPER_DIR they ran with."""
        write(self.path(".list.json"), json.dumps({kind: {"name": "App", "schemes": schemes}}))
        write(self.path(".settings.txt"), self.SETTINGS.format(scheme="App", platforms=platforms, catalyst=catalyst,
                                                               platform=sdk.lower(), sdkroot=self.SDKS.format(sdk)))
        body = (f'[ "$1" != -version ] || {{ printf "Xcode {version}\\nBuild version 1\\n"; exit; }}\n'
                f'echo "$*" >>"{self.log}"\n'
                f'case "$*" in *-list*) cat "{self.path(".list.json")}"; exit ;; '
                f'*-showBuildSettings*) cat "{self.path(".settings.txt")}"; exit ;; esac\n'
                f'echo "${{DEVELOPER_DIR:-}}" >"{self.path(".developer-dir")}"\n'
                f'exit {status}')
        self.fake("xcodebuild", body)

    def derived_data(self, directory: str = ".", parent: str = "") -> str:
        """Where the hook puts the derived data of a build in directory:
        hecatron-<its name>-<cksum of its real path> in parent."""
        real = os.path.realpath(self.path(directory))
        hashed = subprocess.run(["cksum"], input=real, capture_output=True, text=True).stdout.split()[0]
        return os.path.join(parent or self.path(".derived-data"), f"hecatron-{os.path.basename(real)}-{hashed}")

    def calls(self) -> list[str]:
        if not os.path.exists(self.log):
            return []
        with open(self.log) as log:
            return log.read().splitlines()

    def builds(self) -> list[str]:
        return [call for call in self.calls() if call.startswith("build ")]

    def only(self, *languages: str, **config) -> None:
        self.configure(languages=list(languages), implPaths={lang: "." for lang in languages}, **config)

    def destination(self, platforms: str, sdk: str = "iPhoneOS", catalyst: str = "NO") -> str:
        self.xcodebuild(["App"], platforms=platforms, sdk=sdk, catalyst=catalyst)
        open(self.log, "w").close()
        result = self.hook("swift", "build", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        builds = self.builds()
        return builds[0].split("-destination ")[1].split(" -derivedDataPath")[0] if builds else result.stdout

    def test_xcodebuild_builds_the_scheme_named_like_the_project(self):
        for lang in ("swift", "objectivec"):
            self.only(lang)
            open(self.log, "w").close()
            result = self.hook(lang, "build", ".")
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertEqual(self.builds(), ["build -project App.xcodeproj -scheme App -destination generic/platform=iOS "
                                             f"Simulator -derivedDataPath {self.derived_data()} -skipMacroValidation "
                                             "-skipPackagePluginValidation CODE_SIGNING_ALLOWED=NO ARCHS=" + platform.machine()])
            self.assertIn("-list -json -project App.xcodeproj", self.calls())
            self.assertIn("-showBuildSettings -project App.xcodeproj -scheme App", self.calls())

    def test_the_destination_comes_from_the_supported_platforms(self):
        self.only("swift")
        # NetNewsWire: a Mac-only app.
        self.assertEqual(self.destination("macosx", sdk="MacOSX"), "generic/platform=macOS")
        # GoMap, home-assistant and IceCubesApp on Xcode 27: iOS apps
        # supporting Mac Catalyst, whose settings without a destination are
        # the Mac's (SDKROOT is the macOS SDK's path), build for the
        # simulator, not the Catalyst variant.
        self.assertEqual(self.destination("iphoneos iphonesimulator macosx xros xrsimulator", sdk="MacOSX",
                                          catalyst="YES"), "generic/platform=iOS Simulator")
        self.assertEqual(self.destination("macosx iphoneos iphonesimulator", sdk="MacOSX"),
                         "generic/platform=iOS Simulator")
        self.assertEqual(self.destination("iphoneos macosx", sdk="MacOSX", catalyst="YES"),
                         "generic/platform=iOS Simulator")
        self.assertEqual(self.destination("iphoneos iphonesimulator macosx xros xrsimulator"),
                         "generic/platform=iOS Simulator")
        self.assertEqual(self.destination("macosx"), "generic/platform=macOS")
        self.assertEqual(self.destination("macosx appletvos appletvsimulator", sdk="MacOSX"),
                         "generic/platform=tvOS Simulator")
        self.assertEqual(self.destination("xros xrsimulator"), "generic/platform=visionOS Simulator")
        self.assertEqual(self.destination("watchos watchsimulator"), "generic/platform=watchOS Simulator")
        self.assertIn("can't tell a destination for scheme App", self.destination("iphoneos"))
        self.assertIn(".hecatron/languages/swift/build.sh", self.destination(""))

    def test_objective_c_leaves_a_shared_project_to_swift(self):
        self.only("swift", "objectivec")
        skipped = self.path(".skipped")
        for action in ("build", "clean"):
            result = self.hook("objectivec", action, ".", HECATRON_SKIPPED=skipped)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertIn(f"objectivec: {action}: built with swift's hook", result.stdout)
            with open(skipped) as reason:
                self.assertIn("built with swift's hook", reason.read())
        result = self.hook("objectivec", "test", ".")
        self.assertIn("objectivec: test: tested with swift's hook", result.stdout)
        self.assertEqual(self.calls(), [])
        self.assertEqual(self.hook("swift", "build", ".").returncode, 0)
        self.assertEqual(len(self.builds()), 1)

    def test_objective_c_builds_when_swift_is_disabled_or_elsewhere(self):
        self.only("swift", "objectivec")
        write(self.path(".hecatron", "local.json"), json.dumps({"disabled": ["swift"]}))
        self.assertEqual(self.hook("objectivec", "build", ".").returncode, 0)
        self.assertEqual(len(self.builds()), 1)
        os.remove(self.path(".hecatron", "local.json"))
        self.configure(languages=["swift", "objectivec"], implPaths={"swift": "Packages", "objectivec": "."})
        os.makedirs(self.path("Packages"))
        self.assertEqual(self.hook("objectivec", "build", ".").returncode, 0)
        self.assertEqual(len(self.builds()), 2)

    def test_a_root_workspace_builds_through_its_scheme(self):
        write(self.path("App.xcworkspace", "contents.xcworkspacedata"), "")
        self.xcodebuild(["Alamofire", "App", "Pods-App"], kind="workspace")
        self.only("swift")
        result = self.hook("swift", "build", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(len(self.builds()), 1)
        self.assertIn("build -workspace App.xcworkspace -scheme App ", self.builds()[0])

    def test_a_workspace_named_otherwise_builds_the_projects_scheme(self):
        write(self.path("Main.xcworkspace", "contents.xcworkspacedata"), "")
        self.xcodebuild(["Pods-App", "App"], kind="workspace")
        self.only("swift")
        self.assertEqual(self.hook("swift", "build", ".").returncode, 0)
        self.assertIn("build -workspace Main.xcworkspace -scheme App ", self.builds()[0])

    def test_without_a_scheme_named_like_the_project_the_build_skips(self):
        self.xcodebuild(["App-iOS"])
        self.only("swift")
        result = self.hook("swift", "build", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("App.xcodeproj has no shared scheme building its app, nor one named like it (App)", result.stdout)
        self.assertIn(".hecatron/languages/swift/build.sh", result.stdout)
        self.assertIn(".hecatron/languages/swift/test.sh", result.stdout)
        self.assertEqual(self.builds(), [])

    def test_several_projects_build_the_one_named_like_the_directory(self):
        write(self.path("Tools.xcodeproj", "project.pbxproj"), "")
        self.only("swift")
        result = self.hook("swift", "build", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("several Xcode projects here (App.xcodeproj Tools.xcodeproj)", result.stdout)
        self.assertEqual(self.calls(), [])
        os.makedirs(self.path("App", "App.xcodeproj"))
        os.makedirs(self.path("App", "Tools.xcodeproj"))
        self.assertEqual(self.hook("swift", "build", "App").returncode, 0)
        self.assertIn("build -project App.xcodeproj ", self.builds()[0])

    def test_the_derived_data_is_outside_the_repository(self):
        # Gifski's SwiftLint phase linted the package checkouts in
        # .build/xcode. Xcode's own place, named after the directory and
        # its path: ~/Library/Developer/Xcode/DerivedData.
        self.only("swift")
        home = self.path(".home")
        result = self.hook("swift", "build", ".", HOME=home, HECATRON_XCODE_DERIVED_DATA="")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        derived = self.derived_data(parent=os.path.join(home, "Library", "Developer", "Xcode", "DerivedData"))
        self.assertIn(f" -derivedDataPath {derived} ", self.builds()[0])

    def test_clean_removes_the_derived_data(self):
        # And .build/xcode, where earlier versions put it.
        write(os.path.join(self.derived_data(), "Build", "App.o"), "")
        write(self.path(".build", "xcode", "Build", "App.o"), "")
        self.only("swift")
        result = self.hook("swift", "clean", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn(f"+ rm -rf {self.derived_data()}", result.stdout)
        self.assertFalse(os.path.exists(self.derived_data()))
        self.assertIn("+ rm -rf .build/xcode", result.stdout)
        self.assertFalse(os.path.exists(self.path(".build")))
        self.assertEqual(self.calls(), [])
        self.assertNotIn(".build/xcode", self.hook("swift", "clean", ".").stdout)

    def test_tests_skip_saying_how_to_run_them(self):
        for lang in ("swift", "objectivec"):
            self.only(lang)
            result = self.hook(lang, "test", ".")
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertIn(f"{lang}: test: an Xcode project's tests need a scheme and a destination", result.stdout)
            self.assertIn("'platform=macOS' for a Mac app", result.stdout)
            self.assertIn(f".hecatron/languages/{lang}/test.sh", result.stdout)
        self.assertEqual(self.calls(), [])

    def test_swifts_other_hooks_skip(self):
        for action in ("lint", "deps", "docs", "bench"):
            result = self.hook("swift", action, ".")
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertIn(f"an Xcode project has no standard {action} command", result.stdout)

    def test_a_simulator_build_is_for_this_macs_architecture(self):
        # rileytestut/Delta: melonDS' ARM assembly can't compile for x86_64.
        arch = platform.machine()
        result = self.hook("swift", "build", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("generic/platform=iOS Simulator", result.stdout)
        self.assertIn(f"CODE_SIGNING_ALLOWED=NO ARCHS={arch}", result.stdout)
        # A project excluding it builds for what it says.
        write(self.path("Config", "Sim.xcconfig"), f"EXCLUDED_ARCHS[sdk=iphonesimulator*] = {arch}\n")
        result = self.hook("swift", "build", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertNotIn("ARCHS=", result.stdout)

    def test_deps_leaves_the_local_packages_below_to_the_project(self):
        # openhab/openhab-ios: CommonUI's own Package.resolved is stale; the
        # app builds from its workspace's.
        write(self.path("CommonUI", "Package.swift"), "")
        write(self.path("CommonUI", "Package.resolved"), "")
        self.fake("swift", "exit 1")
        self.commit()
        result = self.hook("swift", "deps", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("an Xcode project has no standard deps command", result.stdout)
        self.assertNotIn("swift package", result.stdout)

    def test_without_xcodebuild_the_build_skips(self):
        if shutil.which("xcodebuild"):
            self.skipTest("xcodebuild is installed")
        os.remove(os.path.join(self.bin, "xcodebuild"))
        result = self.hook("swift", "build", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("builds with xcodebuild (Xcode, on macOS), which isn't installed", result.stdout)

    def installed_xcode(self, name: str, version: str) -> str:
        """An Xcode in ~/Applications (the hook's HOME is .home): its
        developer directory."""
        developer = self.path(".home", "Applications", name, "Contents", "Developer")
        write(os.path.join(developer, "usr", "bin", "xcodebuild"), f'#!/bin/sh\nprintf "Xcode {version}\\n"\n')
        os.chmod(os.path.join(developer, "usr", "bin", "xcodebuild"), 0o755)
        return developer

    def developer_dir(self) -> str:
        with open(self.path(".developer-dir")) as f:
            return f.read().strip()

    def test_the_xcode_version_file_selects_the_installed_xcode(self):
        # woocommerce-ios' .xcode-version says 27.0. Versions no real Xcode
        # has, so a Mac's own /Applications doesn't answer.
        self.only("swift")
        os.makedirs(self.path("Sub", "App.xcodeproj"))
        write(self.path("Sub", "App.swift"), "")
        self.installed_xcode("Xcode_98.0.app", "98.0")
        wanted = self.installed_xcode("Xcode_99.1.app", "99.1")
        write(self.path(".xcode-version"), "99.1\n")
        result = self.hook("swift", "build", "Sub", HOME=self.path(".home"))
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("swift: build: Xcode 99.1, as ", result.stdout)
        self.assertEqual(os.path.realpath(self.developer_dir()), os.path.realpath(wanted))
        self.assertEqual(len(self.builds()), 1)

    def test_the_selected_xcode_of_the_version_asked_for_is_kept(self):
        self.only("swift")
        self.installed_xcode("Xcode_99.1.app", "99.1")
        write(self.path(".xcode-version"), "26.6")
        result = self.hook("swift", "build", ".", HOME=self.path(".home"), DEVELOPER_DIR="")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertNotIn("as .xcode-version asks", result.stdout)
        self.assertEqual(self.developer_dir(), "")

    def test_an_xcode_older_than_the_version_asked_for_skips_the_build(self):
        self.only("swift")
        write(self.path(".xcode-version"), "99.2\n")
        result = self.hook("swift", "build", ".", HOME=self.path(".home"))
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("swift: build: the project builds with Xcode 99.2 (.xcode-version), and this is Xcode 26.6: "
                      "install Xcode 99.2 (xcodes install 99.2)", result.stdout)
        self.assertEqual(self.builds(), [])

    def test_an_xcode_newer_than_the_version_asked_for_builds(self):
        self.only("swift")
        write(self.path(".xcode-version"), "1.0\n")
        result = self.hook("swift", "build", ".", HOME=self.path(".home"))
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("the project builds with Xcode 1.0 (.xcode-version), which isn't installed: building with "
                      "Xcode 26.6", result.stdout)
        self.assertEqual(len(self.builds()), 1)

    def test_a_failed_build_of_a_project_upgraded_by_a_newer_xcode_says_so(self):
        # home-assistant/iOS: LastUpgradeCheck 2700, built with Xcode 26.6,
        # failing on an API of the iOS 27 SDK.
        self.only("swift")
        write(self.path("App.xcodeproj", "project.pbxproj"),
              "{\n\tobjects = {\n\t\tP = {\n\t\t\tisa = PBXProject;\n\t\t\tattributes = {\n"
              "\t\t\t\tLastSwiftUpdateCheck = 2700;\n\t\t\t\tLastUpgradeCheck = 2700;\n\t\t\t};\n\t\t};\n\t};\n}\n")
        self.xcodebuild(["App"], status=65)
        result = self.hook("swift", "build", ".")
        self.assertEqual(result.returncode, 65, result.stdout + result.stderr)
        self.assertIn("App.xcodeproj was last upgraded by Xcode 27.0 (its LastUpgradeCheck), newer than this Xcode 26.6",
                      result.stdout)
        self.xcodebuild(["App"], status=65, version="27.1")
        self.assertNotIn("last upgraded", self.hook("swift", "build", ".").stdout)

    APP_TARGET, FRAMEWORK_TARGET, TESTS_TARGET = "B657A8E51CCC1F9F00A8AEBC", "1164C3D624D9C1C100E0A0F1", "B657A8FD1CCC1F9F00A8AEBC"

    EXTENSION_TARGET, HELPER_TARGET, KLAR_TARGET = "B657A9001CCC1F9F00A8AEBC", "B657A9011CCC1F9F00A8AEBC", "B657A9021CCC1F9F00A8AEBC"

    def targets(self, app: str = "App", others: tuple = (), embeds: tuple = ()) -> None:
        """A project.pbxproj (trimmed from home-assistant/iOS's) with an app
        target (depending on the targets embeds names), a framework, a unit
        test bundle, and others: (id, name, product type) each."""
        def target(identifier: str, name: str, product: str, dependencies: tuple = ()) -> str:
            listed = "".join(f"\t\t\t\tD{n} /* PBXTargetDependency */,\n" for n in range(len(dependencies)))
            return (f"\t\t{identifier} /* {name} */ = {{\n\t\t\tisa = PBXNativeTarget;\n"
                    f"\t\t\tbuildConfigurationList = 0 /* Build configuration list for PBXNativeTarget \"{name}\" */;\n"
                    f"\t\t\tbuildPhases = (\n\t\t\t\t1 /* Sources */,\n\t\t\t);\n"
                    f"\t\t\tdependencies = (\n{listed}\t\t\t);\n\t\t\tname = {name};\n"
                    f"\t\t\tproductName = {name};\n\t\t\tproductType = \"com.apple.product-type.{product}\";\n\t\t}};\n")
        dependencies = "".join(f"\t\tD{n} /* PBXTargetDependency */ = {{\n\t\t\tisa = PBXTargetDependency;\n"
                               f"\t\t\ttarget = {identifier} /* embedded */;\n\t\t\ttargetProxy = P{n};\n\t\t}};\n"
                               for n, identifier in enumerate(embeds))
        write(self.path("App.xcodeproj", "project.pbxproj"),
              "// !$*UTF8*$!\n{\n\tarchiveVersion = 1;\n\tobjects = {\n\n/* Begin PBXNativeTarget section */\n"
              + target(self.APP_TARGET, app, "application", embeds) + target(self.FRAMEWORK_TARGET, "Shared", "framework")
              + target(self.TESTS_TARGET, f"{app}Tests", "bundle.unit-test")
              + "".join(target(*other) for other in others)
              + "/* End PBXNativeTarget section */\n\n/* Begin PBXTargetDependency section */\n" + dependencies
              + "/* End PBXTargetDependency section */\n\t};\n\trootObject = 2;\n}\n")

    def scheme(self, container: str, name: str, *targets: str) -> None:
        """A shared scheme whose BuildAction builds the targets, in order."""
        entries = "".join(
            '         <BuildActionEntry buildForTesting = "YES" buildForRunning = "YES">\n'
            f'            <BuildableReference BuildableIdentifier = "primary" BlueprintIdentifier = "{target}" '
            'BuildableName = "App.app" BlueprintName = "App" ReferencedContainer = "container:App.xcodeproj">\n'
            '            </BuildableReference>\n         </BuildActionEntry>\n' for target in targets)
        write(self.path(container, "xcshareddata", "xcschemes", name + ".xcscheme"),
              '<?xml version="1.0" encoding="UTF-8"?>\n<Scheme LastUpgradeVersion = "1600" version = "1.7">\n'
              '   <BuildAction parallelizeBuildables = "YES" buildImplicitDependencies = "YES">\n'
              f'      <BuildActionEntries>\n{entries}      </BuildActionEntries>\n'
              '   </BuildAction>\n</Scheme>\n')

    def built_scheme(self) -> str:
        open(self.log, "w").close()
        result = self.hook("swift", "build", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        builds = self.builds()
        self.assertEqual(len(builds), 1, result.stdout)
        return builds[0].split("-scheme ")[1].split(" ")[0]

    def test_the_shared_scheme_building_the_app_target_is_built(self):
        # home-assistant/iOS: App-Debug and App-Release build the app, and
        # no scheme is named like the project. Without hct's binary to
        # read the schemes' configurations, which to build is the
        # project's to say: no pick by name.
        self.only("swift")
        self.targets()
        self.scheme("App.xcodeproj", "App-Release", self.APP_TARGET)
        self.scheme("App.xcodeproj", "App-Debug", self.APP_TARGET)
        self.scheme("App.xcodeproj", "Shared", self.FRAMEWORK_TARGET)
        self.xcodebuild(self.PACKAGES + ["App-Debug", "App-Release", "Shared"])
        result = self.hook("swift", "build", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("swift: build: App.xcodeproj: several shared schemes build the app (App-Debug, App-Release): "
                      "write .hecatron/languages/swift/build.sh", result.stdout)
        self.assertEqual(self.builds(), [])
        os.remove(self.path("App.xcodeproj", "xcshareddata", "xcschemes", "App-Debug.xcscheme"))
        self.assertEqual(self.built_scheme(), "App-Release")
        # One named like the project comes first, when it builds the app.
        self.scheme("App.xcodeproj", "App", self.APP_TARGET)
        self.xcodebuild(["App", "App-Release", "Shared"])
        self.assertEqual(self.built_scheme(), "App")
        self.scheme("App.xcodeproj", "App", self.FRAMEWORK_TARGET)
        self.assertEqual(self.built_scheme(), "App-Release")

    def test_without_shared_schemes_the_only_app_targets_scheme_is_built(self):
        # Firefox's projects without shared schemes: xcodebuild lists one
        # per target.
        self.only("swift")
        self.targets(app="Client")
        self.xcodebuild(["Client", "ClientTests", "Shared"])
        self.assertEqual(self.built_scheme(), "Client")
        self.assertIn("the scheme of the project's only app target", self.hook("swift", "build", ".").stdout)

    def test_an_extensions_scheme_isnt_the_apps(self):
        # focus-ios: ContentBlocker's scheme builds the extension, then its
        # host app; Focus' builds the app. Klar is a second app.
        self.only("swift")
        self.targets(app="Focus", others=((self.EXTENSION_TARGET, "ContentBlocker", "app-extension"),
                                          (self.KLAR_TARGET, "Klar", "application")))
        self.scheme("App.xcodeproj", "ContentBlocker", self.EXTENSION_TARGET, self.APP_TARGET)
        self.scheme("App.xcodeproj", "Focus", self.APP_TARGET, self.EXTENSION_TARGET)
        self.xcodebuild(self.PACKAGES + ["ContentBlocker", "Focus"])
        self.assertEqual(self.built_scheme(), "Focus")
        self.assertIn("scheme Focus: the shared scheme for the app no other app embeds",
                      self.hook("swift", "build", ".").stdout)
        # Two apps' schemes: the project's to choose, not the first by name.
        self.scheme("App.xcodeproj", "Klar", self.KLAR_TARGET)
        self.xcodebuild(self.PACKAGES + ["ContentBlocker", "Focus", "Klar"])
        open(self.log, "w").close()
        result = self.hook("swift", "build", ".")
        self.assertIn("several shared schemes build the app (Focus, Klar)", result.stdout)
        self.assertEqual(self.builds(), [])

    def test_a_helper_apps_scheme_isnt_the_apps(self):
        # home-assistant: the app embeds its watchOS app, which has a
        # scheme of its own.
        self.only("swift")
        self.targets(app="Main", others=((self.HELPER_TARGET, "Watch", "application"),), embeds=(self.HELPER_TARGET,))
        self.scheme("App.xcodeproj", "Main", self.APP_TARGET)
        self.scheme("App.xcodeproj", "Watch", self.HELPER_TARGET)
        self.xcodebuild(self.PACKAGES + ["Main", "Watch"])
        self.assertEqual(self.built_scheme(), "Main")

    def test_a_script_target_listed_first_doesnt_hide_the_app(self):
        # LoopKit/Loop: its Loop scheme runs the Cartfile aggregate target
        # before the app; WatchApp's scheme is for the app Loop embeds.
        self.only("swift")
        self.targets(app="Main", others=((self.HELPER_TARGET, "Watch", "application"),), embeds=(self.HELPER_TARGET,))
        cartfile = "C0FFEE000000000000000001"
        with open(self.path("App.xcodeproj", "project.pbxproj")) as f:
            text = f.read()
        write(self.path("App.xcodeproj", "project.pbxproj"), text.replace(
            "/* Begin PBXNativeTarget section */\n",
            f"/* Begin PBXAggregateTarget section */\n\t\t{cartfile} /* Cartfile */ = {{\n"
            "\t\t\tisa = PBXAggregateTarget;\n\t\t\tname = Cartfile;\n\t\t};\n"
            "/* End PBXAggregateTarget section */\n\n/* Begin PBXNativeTarget section */\n"))
        self.scheme("App.xcodeproj", "Main", cartfile, self.APP_TARGET)
        self.scheme("App.xcodeproj", "Watch", self.HELPER_TARGET)
        self.xcodebuild(self.PACKAGES + ["Main", "Watch"])
        self.assertEqual(self.built_scheme(), "Main")

    def test_a_workspaces_shared_scheme_builds_its_projects_app(self):
        self.only("swift")
        self.targets()
        write(self.path("App.xcworkspace", "contents.xcworkspacedata"),
              '<?xml version="1.0" encoding="UTF-8"?>\n<Workspace version = "1.0">\n'
              '   <FileRef location = "group:App.xcodeproj">\n   </FileRef>\n</Workspace>\n')
        self.scheme("App.xcworkspace", "Main", self.APP_TARGET)
        self.xcodebuild(["Main", "Shared"], kind="workspace")
        self.assertEqual(self.built_scheme(), "Main")
        self.assertIn("build -workspace App.xcworkspace -scheme Main ", self.builds()[0])

    def plan(self, *lines: str, status: int = 0, directory: str = ".") -> dict:
        """A fake hct binary whose _xcode plan prints the lines (each
        "field<TAB>value") in the directory, and "unsure" elsewhere; the
        environment that gives it to the hooks."""
        write(self.path(directory, ".plan"), "".join(line + "\n" for line in lines))
        binary = self.path(".hct", "hct")
        write(binary, f'#!/bin/sh\necho "$*" >>"{self.path(".hct.log")}"\n[ "$1 $2" = "_xcode plan" ] || exit 2\n'
                      f'[ -f .plan ] || {{ printf "unsure\\tno plan here\\n"; exit 0; }}\ncat .plan\nexit {status}\n', 0o755)
        return {"HECATRON_BINARY": binary}

    def test_the_plan_gives_the_container_scheme_and_destination(self):
        # GoMap: an iOS app supporting Mac Catalyst, whose project says
        # SDKROOT = iphoneos; xcodebuild's settings would say the Mac's.
        self.only("swift")
        self.xcodebuild(["App"], platforms="iphoneos iphonesimulator macosx", sdk="MacOSX", catalyst="YES")
        env = self.plan("kind\t-project", "container\tApp.xcodeproj", "scheme\tApp-Debug",
                        "destination\tgeneric/platform=iOS Simulator")
        result = self.hook("swift", "build", ".", **env)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.builds(), ["build -project App.xcodeproj -scheme App-Debug -destination generic/platform=iOS "
                                         f"Simulator -derivedDataPath {self.derived_data()} -skipMacroValidation "
                                         "-skipPackagePluginValidation CODE_SIGNING_ALLOWED=NO ARCHS=" + platform.machine()])
        self.assertEqual(len(self.calls()), 1)
        self.assertIn("scheme App-Debug: the scheme building the app, from the project's files", result.stdout)
        with open(self.path(".hct.log")) as log:
            self.assertIn(f"_xcode plan . {os.path.basename(self.root)} ", log.read())

    def test_a_plan_missing_files_skips_saying_what_makes_them(self):
        # wikipedia-ios: an .xcconfig its own setup makes; eidolon: Pods.
        self.only("swift")
        env = self.plan("kind\t-project", "container\tApp.xcodeproj", "scheme\tApp",
                        "destination\tgeneric/platform=iOS Simulator",
                        "setup\tthe project's own setup (see its README)", "missing\tConfig/Secrets.xcconfig")
        result = self.hook("swift", "build", ".", **env)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("swift: build: Config/Secrets.xcconfig is missing: the project's own setup (see its README)",
                      result.stdout)
        write(self.path("App.xcworkspace", "contents.xcworkspacedata"), self.PODS_WORKSPACE)
        env = self.plan("kind\t-workspace", "container\tApp.xcworkspace", "scheme\tApp",
                        "destination\tgeneric/platform=iOS Simulator", "setup\tpod install",
                        "missing\tPods/Pods.xcodeproj", "missing\tPods/Target Support Files/Pods-App/Pods-App.debug.xcconfig")
        result = self.hook("swift", "build", ".", **env)
        self.assertIn("swift: build: the Pods aren't installed (Pods/Pods.xcodeproj Pods/Target Support Files/Pods-App/"
                      "Pods-App.debug.xcconfig are missing): run pod install", result.stdout)
        self.assertEqual(self.calls(), [])

    def test_what_the_plan_is_unsure_of_xcodebuild_says(self):
        # focus-ios: two apps' schemes; the scheme named like the project
        # is xcodebuild's to find.
        self.only("swift")
        env = self.plan("kind\t-project", "container\tApp.xcodeproj",
                        "unsure\tseveral schemes build an app (Focus, Klar)")
        result = self.hook("swift", "build", ".", **env)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("the project's files leave it unsure (several schemes build an app (Focus, Klar)): asking xcodebuild",
                      result.stdout)
        self.assertIn("-list -json -project App.xcodeproj", self.calls())
        self.assertIn("build -project App.xcodeproj -scheme App ", self.builds()[0])
        # A scheme without a destination: only the scheme's settings.
        open(self.log, "w").close()
        env = self.plan("kind\t-project", "container\tApp.xcodeproj", "scheme\tApp-Debug",
                        "unsure\ttarget App doesn't set SDKROOT in the files hecatron reads")
        self.assertEqual(self.hook("swift", "build", ".", **env).returncode, 0)
        self.assertEqual(self.calls()[0], "-showBuildSettings -project App.xcodeproj -scheme App-Debug")
        self.assertIn("-scheme App-Debug -destination generic/platform=iOS Simulator ", self.builds()[0])
        # Files hct can't read (it says why): xcodebuild decides it all.
        open(self.log, "w").close()
        env = self.plan(status=1)
        self.assertEqual(self.hook("swift", "build", ".", **env).returncode, 0)
        self.assertIn("-list -json -project App.xcodeproj", self.calls())
        self.assertEqual(len(self.builds()), 1)

    def test_an_app_beside_its_package_builds_with_xcodebuild(self):
        # isowords' App/: isowords.xcodeproj and the Package.swift of its
        # modules. The app is the product; SwiftPM keeps the tests.
        self.only("swift")
        self.targets()
        write(self.path("Package.swift"), self.IOS_PACKAGE)
        self.fake("swift")
        self.fake("uname", 'echo Linux; exit')
        result = self.hook("swift", "build", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(len(self.builds()), 1)
        self.assertNotIn("swift build", result.stdout)
        result = self.hook("swift", "test", ".")
        self.assertIn("+ swift test", result.stdout)
        self.assertEqual(len(self.builds()), 1)
        # Objective-C leaves the app to Swift.
        self.only("swift", "objectivec")
        self.assertIn("built with swift's hook", self.hook("objectivec", "build", ".").stdout)
        # A project without an app (a framework's) leaves the build to SwiftPM.
        self.only("swift")
        write(self.path("App.xcodeproj", "project.pbxproj"), "")
        self.assertIn("+ swift build", self.hook("swift", "build", ".").stdout)
        self.assertEqual(len(self.builds()), 1)

    PODS_WORKSPACE = ('<?xml version="1.0" encoding="UTF-8"?>\n<Workspace version = "1.0">\n'
                      '   <FileRef location = "group:App.xcodeproj">\n   </FileRef>\n'
                      '   <FileRef location = "group:Pods/Pods.xcodeproj">\n   </FileRef>\n</Workspace>\n')

    def test_a_workspace_without_its_pods_skips_saying_to_install_them(self):
        # artsy/eidolon: Kiosk.xcworkspace and Podfile.lock, Pods/ not
        # committed; xcodebuild -list lists only Kiosk.
        self.only("swift")
        write(self.path("App.xcworkspace", "contents.xcworkspacedata"), self.PODS_WORKSPACE)
        write(self.path("Podfile"), "target 'App' do\n  pod 'Alamofire'\nend\n")
        write(self.path("Podfile.lock"), "PODS:\n  - Alamofire (5.9.0)\n")
        self.xcodebuild(["App"], kind="workspace")
        skipped = self.path(".skipped")
        result = self.hook("swift", "build", ".", HECATRON_SKIPPED=skipped)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("swift: build: the Pods aren't installed (Pods/Pods.xcodeproj is missing): run pod install",
                      result.stdout)
        with open(skipped) as reason:
            self.assertIn("run pod install", reason.read())
        self.assertEqual(self.calls(), [])
        # A workspace naming no Pods project, beside a Podfile.lock.
        write(self.path("App.xcworkspace", "contents.xcworkspacedata"), self.PODS_WORKSPACE.replace(
            '   <FileRef location = "group:Pods/Pods.xcodeproj">\n   </FileRef>\n', ""))
        self.assertIn("the Pods aren't installed", self.hook("swift", "build", ".").stdout)
        write(self.path("App.xcworkspace", "contents.xcworkspacedata"), self.PODS_WORKSPACE)
        write(self.path("Pods", "Pods.xcodeproj", "project.pbxproj"), "")
        self.assertEqual(self.built_scheme(), "App")

    def test_a_workspace_missing_another_project_skips(self):
        self.only("swift")
        write(self.path("App.xcworkspace", "contents.xcworkspacedata"),
              '<?xml version="1.0" encoding="UTF-8"?>\n<Workspace version = "1.0">\n'
              '   <FileRef location = "group:App.xcodeproj">\n   </FileRef>\n'
              '   <Group location = "container:Vendor" name = "Vendor">\n'
              '      <FileRef location = "group:Lib/Lib.xcodeproj">\n      </FileRef>\n   </Group>\n</Workspace>\n')
        result = self.hook("swift", "build", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("App.xcworkspace refers to projects that aren't here (Vendor/Lib/Lib.xcodeproj)", result.stdout)
        self.assertEqual(self.calls(), [])

    def test_deps_installs_an_apps_pods(self):
        write(self.path("Podfile"), "target 'App' do\n  pod 'Alamofire'\nend\n")
        self.fake("pod")
        self.fake("swift")
        for lang in ("swift", "objectivec"):
            self.only(lang)
            result = self.hook(lang, "deps", ".")
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertIn("+ pod install", result.stdout)
            self.assertNotIn("swift package", result.stdout)
        # Swift's hook installs the pods of a project the two share.
        self.only("swift", "objectivec")
        result = self.hook("objectivec", "deps", ".")
        self.assertIn("installed with swift's hook", result.stdout)
        self.assertNotIn("+ pod install", result.stdout)

    def test_deps_without_cocoapods_skips(self):
        if shutil.which("pod"):
            self.skipTest("CocoaPods is installed")
        write(self.path("Podfile"), "target 'App' do\nend\n")
        self.only("swift")
        result = self.hook("swift", "deps", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("the Podfile's pods install with pod install (CocoaPods), which isn't installed", result.stdout)
        os.remove(self.path("Podfile"))
        self.only("objectivec")
        self.assertIn("no Podfile", self.hook("objectivec", "deps", ".").stdout)

    IOS_PACKAGE = ('// swift-tools-version:6.0\nlet package = Package(name: "Core", platforms: [.iOS(.v17), .visionOS(.v1)],\n'
                   '  targets: [.target(name: "Core"), .testTarget(name: "CoreTests")])\n')

    def test_the_xcode_project_owns_the_tests_of_its_local_packages(self):
        # IceCubesApp: iOS-only packages under Packages/, which swift test
        # on the Mac can't build.
        write(self.path("Packages", "Core", "Package.swift"), self.IOS_PACKAGE)
        self.fake("swift")
        result = self.hook("swift", "test", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("swift: test: an Xcode project's tests need a scheme", result.stdout)
        self.assertNotIn("swift test", result.stdout)

    def test_an_ios_only_package_skips_swiftpm_on_macos(self):
        write(self.path("Packages", "Core", "Package.swift"), self.IOS_PACKAGE)
        self.fake("swift")
        self.fake("uname", 'echo Darwin; exit')
        result = self.hook("swift", "test", "Packages/Core")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("the package's platforms don't include macOS", result.stdout)
        self.assertNotIn("+ swift test", result.stdout)
        write(self.path("Packages", "Core", "Package.swift"), self.IOS_PACKAGE.replace(".iOS(.v17)", ".iOS(.v17), .macOS(.v14)"))
        self.assertIn("+ swift test", self.hook("swift", "test", "Packages/Core").stdout)
        self.fake("uname", 'echo Linux; exit')
        write(self.path("Packages", "Core", "Package.swift"), self.IOS_PACKAGE)
        self.assertIn("+ swift test", self.hook("swift", "test", "Packages/Core").stdout)


class XcodeAppsBelowTest(BuildHookTest):
    """Apps with Xcode projects of their own below a root Package.swift
    (mozilla-mobile/firefox-ios: firefox-ios/Client.xcodeproj and
    focus-ios/Blockzilla.xcodeproj): Swift's build and clean act on the root
    package, then on each app detection finds, never on an example's."""

    def setUp(self):
        super().setUp()
        self.log = self.path(".xcodebuild.log")
        for path in ("Package.swift", "BrowserKit/Sources/Common/a.swift",
                     "firefox-ios/Client.xcodeproj/project.pbxproj", "firefox-ios/Client/AppDelegate.swift",
                     "focus-ios/Blockzilla.xcodeproj/project.pbxproj", "focus-ios/Blockzilla/AppDelegate.swift",
                     "Example/Example.xcodeproj/project.pbxproj", "Example/Example/a.swift",
                     "SampleBrowser/SampleBrowser.xcodeproj/project.pbxproj", "SampleBrowser/SampleBrowser/a.swift",
                     "ThirdParty/Deferred/Deferred.xcodeproj/project.pbxproj", "ThirdParty/Deferred/Deferred/a.swift",
                     "Vendor/Lib/Lib.xcodeproj/project.pbxproj", "Vendor/Lib/a.swift",
                     ".github/Tool/Tool.xcodeproj/project.pbxproj", ".github/Tool/a.swift",
                     "Assets/Assets.xcodeproj/project.pbxproj",
                     "firefox-ios/Client/Legacy.m", "focus-ios/Blockzilla/Legacy.m"):
            write(self.path(*path.split("/")), "")
        write(self.path(".list.json"), json.dumps({"project": {"name": "Client", "schemes": ["Blockzilla", "Client"]}}))
        write(self.path(".settings.txt"), XcodeTest.SETTINGS.format(
            scheme="Client", platform="iphoneos", sdkroot="iphoneos", platforms=XcodeTest.IOS_AND_VISION, catalyst="NO"))
        self.fake("xcodebuild", f'echo "$(basename "$(pwd)") $*" >>"{self.log}"\n'
                                f'case "$*" in *-list*) cat "{self.path(".list.json")}"; exit ;; '
                                f'*-showBuildSettings*) cat "{self.path(".settings.txt")}"; exit ;; esac')
        self.fake("swift")
        self.configure(languages=["objectivec", "swift"], implPath=".")
        self.commit()

    def builds(self) -> list[str]:
        with open(self.log) as log:
            return [" ".join(call.split()[:6]) for call in log.read().splitlines() if call.split()[1] == "build"]

    def test_build_builds_the_package_and_each_app(self):
        result = self.hook("swift", "build", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("swift build --build-tests", result.stdout)
        self.assertEqual(result.stdout.count("+ swift build"), 1)
        self.assertEqual(self.builds(), ["firefox-ios build -project Client.xcodeproj -scheme Client",
                                         "focus-ios build -project Blockzilla.xcodeproj -scheme Blockzilla"])
        self.assertIn("--- swift: build in firefox-ios", result.stdout)

    def test_a_package_the_apps_use_is_left_to_their_builds(self):
        # pointfreeco/isowords: App/isowords.xcodeproj refers to the root
        # package's folder (path = ..), whose iOS modules SwiftPM on the
        # Mac can't build; Xcode 15's projects name it with an
        # XCLocalSwiftPackageReference instead.
        write(self.path("firefox-ios", "Client.xcodeproj", "project.pbxproj"),
              "{\n\tobjects = {\n\t\tF0 /* isowords */ = {isa = PBXFileReference; lastKnownFileType = folder; "
              "name = isowords; path = ..; sourceTree = \"<group>\"; };\n\t};\n}\n")
        result = self.hook("swift", "build", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("swift: build: the package here is built by the apps below that use it (firefox-ios focus-ios)",
                      result.stdout)
        self.assertNotIn("+ swift build", result.stdout)
        self.assertEqual(len(self.builds()), 2)
        self.assertIn("swift package clean", self.hook("swift", "clean", ".").stdout)
        write(self.path("firefox-ios", "Client.xcodeproj", "project.pbxproj"),
              "{\n\tobjects = {\n\t\tL0 /* XCLocalSwiftPackageReference \"..\" */ = {\n"
              "\t\t\tisa = XCLocalSwiftPackageReference;\n\t\t\trelativePath = ..;\n\t\t};\n\t};\n}\n")
        self.assertNotIn("+ swift build", self.hook("swift", "build", ".").stdout)
        # A package of its own beside the app's isn't the root's.
        write(self.path("firefox-ios", "Client.xcodeproj", "project.pbxproj"),
              "{\n\tobjects = {\n\t\tL0 = {isa = XCLocalSwiftPackageReference; relativePath = Packages/Core; };\n\t};\n}\n")
        self.assertIn("+ swift build", self.hook("swift", "build", ".").stdout)

    def test_the_plans_nested_projects_are_the_apps(self):
        # hct's binary lists the projects below; those outside examples
        # and vendored code, with Swift in them, are the apps.
        binary = self.path(".hct", "hct")
        write(binary, '#!/bin/sh\n[ "$(pwd)" = "' + os.path.realpath(self.root) + '" ] || '
                      '{ printf "unsure\\tnot here\\n"; exit 0; }\nprintf "unsure\\tno Xcode project here\\n'
                      'nested\\tExample/Example.xcodeproj\\nnested\\tSampleBrowser/SampleBrowser.xcodeproj\\n'
                      'nested\\tfirefox-ios/Client.xcodeproj\\n'
                      'nested\\tVendor/Lib/Lib.xcodeproj\\nnested\\tAssets/Assets.xcodeproj\\n"\n', 0o755)
        result = self.hook("swift", "build", ".", HECATRON_BINARY=binary)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.builds(), ["firefox-ios build -project Client.xcodeproj -scheme Client"])

    def test_clean_cleans_the_package_and_each_app(self):
        result = self.hook("swift", "clean", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("swift package clean", result.stdout)
        self.assertEqual(result.stdout.count("+ rm -rf " + self.path(".derived-data", "hecatron-")), 2)

    def test_an_app_alone_builds_itself(self):
        result = self.hook("swift", "build", "focus-ios")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.builds(), ["focus-ios build -project Blockzilla.xcodeproj -scheme Blockzilla"])
        self.assertNotIn("swift build", result.stdout)

    def test_objective_c_in_the_apps_is_left_to_swift(self):
        # firefox-ios: its .m files are all in the apps, so the root has no
        # Objective-C build of its own.
        skipped = self.path(".skipped")
        for action in ("build", "clean"):
            result = self.hook("objectivec", action, ".", HECATRON_SKIPPED=skipped)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertIn("firefox-ios: built with swift's hook", result.stdout)
            self.assertIn("focus-ios: built with swift's hook", result.stdout)
        self.assertFalse(os.path.exists(self.log))
        for action in ("test", "lint"):
            result = self.hook("objectivec", action, ".")
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertIn("the Objective-C here is in the Xcode apps below (firefox-ios focus-ios)", result.stdout)

    def test_objective_c_builds_the_apps_when_swift_is_disabled(self):
        write(self.path(".hecatron", "local.json"), json.dumps({"disabled": ["swift"]}))
        result = self.hook("objectivec", "build", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.builds(), ["firefox-ios build -project Client.xcodeproj -scheme Client",
                                         "focus-ios build -project Blockzilla.xcodeproj -scheme Blockzilla"])


if __name__ == "__main__":
    unittest.main()
