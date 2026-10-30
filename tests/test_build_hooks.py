"""The default build hooks' choices for JVM and .NET layouts
(languages/_hooks.sh): each tool is a fake on PATH that prints how it was
called. internal/actions' comparison checks the plain cases against the Go
plans; these are the ones a plan can't show (a retry on the tool's output,
the environment a command gets, installers' view of builds below)."""

from __future__ import annotations

import json
import os
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
                   sdk: str = "iPhoneOS") -> None:
        write(self.path(".list.json"), json.dumps({kind: {"name": "App", "schemes": schemes}}))
        write(self.path(".settings.txt"), self.SETTINGS.format(scheme="App", platforms=platforms,
                                                               platform=sdk.lower(), sdkroot=self.SDKS.format(sdk)))
        body = (f'echo "$*" >>"{self.log}"\n'
                f'case "$*" in *-list*) cat "{self.path(".list.json")}"; exit ;; '
                f'*-showBuildSettings*) cat "{self.path(".settings.txt")}"; exit ;; esac')
        self.fake("xcodebuild", body)

    def calls(self) -> list[str]:
        if not os.path.exists(self.log):
            return []
        with open(self.log) as log:
            return log.read().splitlines()

    def builds(self) -> list[str]:
        return [call for call in self.calls() if call.startswith("build ")]

    def only(self, *languages: str, **config) -> None:
        self.configure(languages=list(languages), implPaths={lang: "." for lang in languages}, **config)

    def destination(self, platforms: str, sdk: str = "iPhoneOS") -> str:
        self.xcodebuild(["App"], platforms=platforms, sdk=sdk)
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
                                             "Simulator -derivedDataPath .build/xcode CODE_SIGNING_ALLOWED=NO"])
            self.assertIn("-list -json -project App.xcodeproj", self.calls())
            self.assertIn("-showBuildSettings -project App.xcodeproj -scheme App", self.calls())

    def test_the_destination_comes_from_the_supported_platforms(self):
        self.only("swift")
        # NetNewsWire: a native Mac app (SDKROOT is macOS's).
        self.assertEqual(self.destination("macosx", sdk="MacOSX"), "generic/platform=macOS")
        self.assertEqual(self.destination("macosx iphoneos iphonesimulator", sdk="MacOSX"), "generic/platform=macOS")
        # IceCubesApp and GoMap: iOS apps supporting Mac Catalyst build for
        # the simulator, not the Catalyst variant.
        self.assertEqual(self.destination("iphoneos iphonesimulator macosx xros xrsimulator"),
                         "generic/platform=iOS Simulator")
        self.assertEqual(self.destination("macosx"), "generic/platform=macOS")
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

    def test_clean_removes_the_derived_data(self):
        write(self.path(".build", "xcode", "Build", "App.o"), "")
        self.only("swift")
        result = self.hook("swift", "clean", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("+ rm -rf .build/xcode", result.stdout)
        self.assertFalse(os.path.exists(self.path(".build")))
        self.assertEqual(self.calls(), [])

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

    def test_without_xcodebuild_the_build_skips(self):
        if shutil.which("xcodebuild"):
            self.skipTest("xcodebuild is installed")
        os.remove(os.path.join(self.bin, "xcodebuild"))
        result = self.hook("swift", "build", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("builds with xcodebuild (Xcode, on macOS), which isn't installed", result.stdout)

    APP_TARGET, FRAMEWORK_TARGET, TESTS_TARGET = "B657A8E51CCC1F9F00A8AEBC", "1164C3D624D9C1C100E0A0F1", "B657A8FD1CCC1F9F00A8AEBC"

    def targets(self, app: str = "App") -> None:
        """A project.pbxproj (trimmed from home-assistant/iOS's) with an app
        target, a framework and a unit test bundle."""
        def target(identifier: str, name: str, product: str) -> str:
            return (f"\t\t{identifier} /* {name} */ = {{\n\t\t\tisa = PBXNativeTarget;\n"
                    f"\t\t\tbuildConfigurationList = 0 /* Build configuration list for PBXNativeTarget \"{name}\" */;\n"
                    f"\t\t\tbuildPhases = (\n\t\t\t\t1 /* Sources */,\n\t\t\t);\n\t\t\tname = {name};\n"
                    f"\t\t\tproductName = {name};\n\t\t\tproductType = \"com.apple.product-type.{product}\";\n\t\t}};\n")
        write(self.path("App.xcodeproj", "project.pbxproj"),
              "// !$*UTF8*$!\n{\n\tarchiveVersion = 1;\n\tobjects = {\n\n/* Begin PBXNativeTarget section */\n"
              + target(self.APP_TARGET, app, "application") + target(self.FRAMEWORK_TARGET, "Shared", "framework")
              + target(self.TESTS_TARGET, f"{app}Tests", "bundle.unit-test")
              + "/* End PBXNativeTarget section */\n\t};\n\trootObject = 2;\n}\n")

    def scheme(self, container: str, name: str, target: str) -> None:
        """A shared scheme whose BuildAction builds the target."""
        write(self.path(container, "xcshareddata", "xcschemes", name + ".xcscheme"),
              '<?xml version="1.0" encoding="UTF-8"?>\n<Scheme LastUpgradeVersion = "1600" version = "1.7">\n'
              '   <BuildAction parallelizeBuildables = "YES" buildImplicitDependencies = "YES">\n'
              '      <BuildActionEntries>\n'
              '         <BuildActionEntry buildForTesting = "YES" buildForRunning = "YES">\n'
              f'            <BuildableReference BuildableIdentifier = "primary" BlueprintIdentifier = "{target}" '
              'BuildableName = "App.app" BlueprintName = "App" ReferencedContainer = "container:App.xcodeproj">\n'
              '            </BuildableReference>\n         </BuildActionEntry>\n      </BuildActionEntries>\n'
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
        # no scheme is named like the project.
        self.only("swift")
        self.targets()
        self.scheme("App.xcodeproj", "App-Release", self.APP_TARGET)
        self.scheme("App.xcodeproj", "App-Debug", self.APP_TARGET)
        self.scheme("App.xcodeproj", "Shared", self.FRAMEWORK_TARGET)
        self.xcodebuild(self.PACKAGES + ["App-Debug", "App-Release", "Shared"])
        self.assertEqual(self.built_scheme(), "App-Debug")
        self.assertIn("swift: build: scheme App-Debug: the first of the shared schemes building the app "
                      "(App-Debug, App-Release)", self.hook("swift", "build", ".").stdout)
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
        self.assertIn("swift: build: the Pods aren't installed: run pod install", result.stdout)
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
                     "firefox-ios/Client/Legacy.m", "focus-ios/Blockzilla/Legacy.m"):
            write(self.path(*path.split("/")), "")
        write(self.path(".list.json"), json.dumps({"project": {"name": "Client", "schemes": ["Blockzilla", "Client"]}}))
        write(self.path(".settings.txt"), XcodeTest.SETTINGS.format(
            scheme="Client", platform="iphoneos", sdkroot="iphoneos", platforms=XcodeTest.IOS_AND_VISION))
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

    def test_clean_cleans_the_package_and_each_app(self):
        result = self.hook("swift", "clean", ".")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("swift package clean", result.stdout)
        self.assertEqual(result.stdout.count("+ rm -rf .build/xcode"), 2)

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
