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

    def test_a_source_directory_reads_its_gradle_builds_toolchain_from_the_catalog(self):
        write(os.path.join(self.root, "settings.gradle.kts"), 'include(":lib")\n')
        write(os.path.join(self.root, "build.gradle.kts"),
              "java { toolchain { languageVersion.set(libs.versions.jdk.map(JavaLanguageVersion::of)) } }\n"
              "kotlin { jvmToolchain(libs.versions.build.jdk.get().toInt()) }\n")
        write(os.path.join(self.root, "gradle", "libs.versions.toml"),
              '[versions]\njdk = "21"\nbuild-jdk = "17"\nkotlin = "2.1.0"\n[libraries]\njdk = "x:y:9"\n')
        sources = os.path.join(self.root, "lib", "src", "main", "java")
        os.makedirs(sources)
        self.assertEqual(jvm.toolchains(sources, self.root), [17, 21])
        self.assertEqual(jvm.ceiling(sources, self.root), (21, "it compiles with Kotlin 2.1.0"))

    def test_a_kotlin_jvm_target_without_a_java_one_runs_on_that_jdk(self):
        write(os.path.join(self.component, "build.gradle.kts"),
              "kotlin { compilerOptions { jvmTarget.set(JvmTarget.JVM_17) } }\n")
        self.assertEqual(jvm.ceiling(self.component),
                         (17, "its Kotlin targets JVM 17 and nothing sets Java's target to match"))
        write(os.path.join(self.component, "build.gradle.kts"),
              'kotlin { jvmTarget = "17" }\njava { targetCompatibility = JavaVersion.VERSION_17 }\n')
        self.assertIsNone(jvm.ceiling(self.component))
        write(os.path.join(self.component, "build.gradle.kts"), "kotlin { jvmTarget.set(JvmTarget.JVM_1_8) }\n")
        self.assertIsNone(jvm.ceiling(self.component))

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

    def test_the_java_a_build_compiles_for_is_its_floor(self):
        # nextcloud's Android app: compileOptions and Kotlin's jvmTarget.
        write(os.path.join(self.component, "app", "build.gradle.kts"),
              "android { compileOptions { sourceCompatibility = JavaVersion.VERSION_21\n"
              "targetCompatibility = JavaVersion.VERSION_21 } }\n"
              "kotlin.compilerOptions.jvmTarget.set(JvmTarget.JVM_17)\n")
        write(os.path.join(self.component, "settings.gradle.kts"), 'include(":app")\n')
        self.assertEqual(jvm.floor(self.component, self.root)[0], 21)
        self.assertIn("it compiles for Java 21 (lang/java/app/build.gradle.kts)", jvm.floor(self.component, self.root)[1])

    def test_a_floor_from_the_version_catalog(self):
        # Lawnchair: sourceCompatibility = libs.versions.jdkRelease.get().
        write(os.path.join(self.component, "build.gradle"),
              "compileOptions {\n  sourceCompatibility = libs.versions.jdkRelease.get()\n}\n")
        write(os.path.join(self.component, "gradle", "libs.versions.toml"), '[versions]\njdkRelease = "21"\nagp = "8"\n')
        self.assertEqual(jvm.floor(self.component)[0], 21)

    def test_a_floor_set_in_a_convention_plugin(self):
        # Now in Android's build-logic sets it in Kotlin source.
        write(os.path.join(self.component, "build.gradle.kts"), "")
        write(os.path.join(self.component, "build-logic", "convention", "src", "main", "kotlin", "KotlinAndroid.kt"),
              "compileOptions { sourceCompatibility = JavaVersion.VERSION_17 }\n")
        self.assertEqual(jvm.floor(self.component)[0], 17)

    def test_a_pinned_jdk_is_a_floor(self):
        # pocket-casts' Android app targets 17 and pins 21: a dependency's
        # Java 21 class files don't load in javac on JDK 17.
        write(os.path.join(self.component, "build.gradle.kts"), 'java { sourceCompatibility = "17" }\n')
        write(os.path.join(self.component, ".java-version"), "21.0\n")
        self.assertEqual(jvm.floor(self.component, self.root), (21, "it pins JDK 21 (lang/java/.java-version)"))
        os.remove(os.path.join(self.component, ".java-version"))
        for name, text, version in ((".sdkmanrc", "# sdk env\njava=22.0.2-tem\n", 22),
                                    (".tool-versions", "nodejs 22\njava temurin-23.0.1+11\n", 23),
                                    (".java-version", "corretto64-11.0.2\n", 17),
                                    (".java-version", "1.8\n", 17)):
            write(os.path.join(self.root, name), text)
            self.assertEqual(jvm.floor(self.component, self.root)[0], version, name)
            os.remove(os.path.join(self.root, name))

    def test_maven_and_old_targets(self):
        write(os.path.join(self.component, "pom.xml"),
              "<properties><maven.compiler.release>17</maven.compiler.release></properties>\n")
        self.assertEqual(jvm.floor(self.component, self.root)[0], 17)
        write(os.path.join(self.component, "pom.xml"),
              "<properties><maven.compiler.source>1.8</maven.compiler.source></properties>\n")
        self.assertIsNone(jvm.floor(self.component, self.root))  # Java 8: any JDK hecatron runs compiles it
        write(os.path.join(self.component, "pom.xml"), "<project/>\n")
        write(os.path.join(self.component, "build.gradle"), "java { sourceCompatibility = '11' }\n")
        self.assertEqual(jvm.floor(self.component, self.root)[0], 11)

    def test_a_wrapper_properties_file_without_gradlew_is_not_run(self):
        write(os.path.join(self.component, "gradle", "wrapper", "gradle-wrapper.properties"),
              "distributionUrl=https\\://services.gradle.org/distributions/gradle-5.2.1-bin.zip\n")
        self.assertIsNone(jvm.ceiling(self.component))

    def test_a_project_of_a_multi_project_build_runs_the_wrapper_above(self):
        write(os.path.join(self.root, "gradlew"), "#!/bin/sh\n")
        write(os.path.join(self.root, "gradle", "wrapper", "gradle-wrapper.properties"),
              "distributionUrl=https\\://services.gradle.org/distributions/gradle-8.14.3-bin.zip\n")
        self.assertIsNone(jvm.ceiling(self.component))
        self.assertEqual(jvm.ceiling(self.component, self.root)[0], 21)

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
