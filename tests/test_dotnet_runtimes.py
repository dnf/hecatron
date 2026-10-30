"""The .NET runtimes the doctor and installer want (dotnet_target_majors in
languages/_common.sh): those the projects that run target, test projects and
executables, not a class library's frameworks, whose targeting packs the SDK
restores (#256)."""

from __future__ import annotations

import os
import subprocess
import sys
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(HOME, "tests"))

from test_hecatron import Fixture, write  # noqa: E402

COMMON = os.path.join(HOME, "languages", "_common.sh")


def project(frameworks: str = "", body: str = "", sdk: str = "Microsoft.NET.Sdk") -> str:
    tag = "TargetFrameworks" if ";" in frameworks else "TargetFramework"
    group = f"<{tag}>{frameworks}</{tag}>" if frameworks else ""
    return f'<Project Sdk="{sdk}">\n  <PropertyGroup>\n    {group}\n{body}  </PropertyGroup>\n</Project>\n'


class DotnetRuntimesTest(Fixture):
    def majors(self) -> list[str]:
        self.configure(languages=["csharp"], implPath="{lang}")
        env = {**os.environ, "HECATRON_HOME": HOME, "HECATRON_ROOT": self.root}
        result = subprocess.run(["bash", "-c", f"source {COMMON}\ndotnet_target_majors"],
                                cwd=self.root, env=env, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return result.stdout.split()

    def test_a_library_needs_no_runtime_and_its_tests_do(self):
        # durabuilt's: a library multi-targeting net8.0, tested and run on net10.0.
        write(self.path("csharp", "src", "Lib", "Lib.csproj"), project("net8.0;netstandard2.1"))
        write(self.path("csharp", "tests", "Lib.Tests", "Lib.Tests.csproj"),
              project("net10.0", '    <PackageReference Include="Microsoft.NET.Test.Sdk" Version="17.0.0" />\n'))
        write(self.path("csharp", "src", "App", "App.csproj"),
              project("net10.0", "    <OutputType>Exe</OutputType>\n"))
        self.assertEqual(self.majors(), ["10"])

    def test_each_kind_of_project_that_runs(self):
        write(self.path("csharp", "a", "a.csproj"), project("net6.0", "    <IsTestProject>true</IsTestProject>\n"))
        write(self.path("csharp", "b", "b.csproj"), project("net7.0", "    <OutputType>WinExe</OutputType>\n"))
        write(self.path("csharp", "c", "c.csproj"), project("net8.0", sdk="Microsoft.NET.Sdk.Web"))
        write(self.path("csharp", "d", "d.fsproj"), project("net9.0;netstandard2.0", "    <OutputType>exe</OutputType>\n"))
        write(self.path("csharp", "e", "e.csproj"), project("net11.0", "    <OutputType>Library</OutputType>\n"))
        self.assertEqual(self.majors(), ["6", "7", "8", "9"])

    def test_frameworks_from_the_nearest_directory_build_props(self):
        write(self.path("csharp", "Directory.Build.props"), project("net9.0"))
        write(self.path("csharp", "tests", "Directory.Build.props"), "<Project />\n")
        write(self.path("csharp", "tests", "T", "T.csproj"),
              project(body='    <PackageReference Include="Microsoft.NET.Test.Sdk" />\n'))
        write(self.path("csharp", "src", "L", "L.csproj"), project())
        self.assertEqual(self.majors(), ["9"])

    def test_nothing_that_runs(self):
        write(self.path("csharp", "L.csproj"), project("net8.0"))
        self.assertEqual(self.majors(), [])


if __name__ == "__main__":
    unittest.main()
