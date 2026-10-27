"""Tests for libexec/lib/jvm.py: the JDKs a JVM build needs, from its build files.

    python3 -m unittest discover -s tests
"""

from __future__ import annotations

import os
import sys
import tempfile
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(HOME, "libexec", "lib"))

import jvm  # noqa: E402


def write(path: str, text: str) -> None:
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(text)


class JvmTest(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.root = self._tmp.name
        self.component = os.path.join(self.root, "lang", "java")
        os.makedirs(self.component)

    def tearDown(self):
        self._tmp.cleanup()

    def wrapper(self, version: str) -> None:
        write(os.path.join(self.component, "gradlew"), "#!/bin/sh\n")
        write(os.path.join(self.component, "gradle", "wrapper", "gradle-wrapper.properties"),
              f"distributionUrl=https\\://services.gradle.org/distributions/gradle-{version}-bin.zip\n")

    def test_a_gradle_toolchain_in_any_gradle_script(self):
        write(os.path.join(self.component, "gradle", "sources.gradle"),
              "java { toolchain { languageVersion = JavaLanguageVersion.of(17) } }\n")
        write(os.path.join(self.component, "build.gradle.kts"), "kotlin { jvmToolchain(21) }\n")
        self.assertEqual(jvm.toolchains(self.component, self.root), [17, 21])

    def test_maven_toolchains_in_a_parent_pom(self):
        write(os.path.join(self.component, "pom.xml"), "<project/>\n")
        write(os.path.join(self.root, "pom.xml"), """<project><build><plugins><plugin>
  <artifactId>maven-toolchains-plugin</artifactId>
  <executions><execution><goals><goal>select-jdk-toolchain</goal></goals>
    <configuration><version>[11,12)</version></configuration></execution></executions>
</plugin></plugins></build></project>
""")
        self.assertEqual(jvm.toolchains(self.component, self.root), [11])

    def test_a_version_range_outside_the_toolchains_plugin_is_not_a_toolchain(self):
        write(os.path.join(self.component, "pom.xml"), "<dependency><version>[1,2)</version></dependency>\n")
        self.assertEqual(jvm.toolchains(self.component, self.root), [])

    def test_an_old_gradle_wrapper_runs_on_the_newest_lts_it_supports(self):
        self.wrapper("5.2.1")
        self.assertEqual(jvm.ceiling(self.component), (11, "its Gradle wrapper is Gradle 5.2.1"))
        self.wrapper("8.0.1")
        self.assertEqual(jvm.ceiling(self.component)[0], 17)
        self.wrapper("8.14.3")
        self.assertEqual(jvm.ceiling(self.component)[0], 21)

    def test_a_current_gradle_wrapper_has_no_ceiling(self):
        self.wrapper("9.1.0")
        self.assertIsNone(jvm.ceiling(self.component))

    def test_a_wrapper_properties_file_without_gradlew_is_not_run(self):
        write(os.path.join(self.component, "gradle", "wrapper", "gradle-wrapper.properties"),
              "distributionUrl=https\\://services.gradle.org/distributions/gradle-5.2.1-bin.zip\n")
        self.assertIsNone(jvm.ceiling(self.component))

    def test_an_old_kotlin_compiler(self):
        write(os.path.join(self.component, "pom.xml"),
              "<properties><kotlin.version>2.1.0</kotlin.version></properties>\n")
        self.assertEqual(jvm.ceiling(self.component), (21, "it compiles with Kotlin 2.1.0"))

    def test_kotlin_before_2_3_targets_no_newer_than_jdk_24(self):
        write(os.path.join(self.component, "build.gradle.kts"),
              'plugins { kotlin("jvm") version "2.2.20" }\n')
        self.assertEqual(jvm.ceiling(self.component)[0], 21)

    def test_kotlin_in_a_version_catalog(self):
        write(os.path.join(self.component, "gradle", "libs.versions.toml"), '[versions]\nkotlin = "1.9.10"\n')
        self.assertEqual(jvm.ceiling(self.component)[0], 17)

    def test_a_current_kotlin_compiler_has_no_ceiling(self):
        write(os.path.join(self.component, "build.gradle.kts"),
              'plugins { kotlin("jvm") version "2.3.0" }\n')
        self.assertIsNone(jvm.ceiling(self.component))


if __name__ == "__main__":
    unittest.main()
