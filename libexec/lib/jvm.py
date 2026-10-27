"""The JDKs a JVM build needs besides the one it runs on, read from its build
files, for the JVM languages' installers and hooks.

    jvm.py toolchains <dir> <root>   the JDK feature versions the build asks
                                     Gradle or Maven to find (its toolchains),
                                     one per line
    jvm.py ceiling <dir>             the newest LTS JDK the build's own tools
                                     run on, with why, when that is older
                                     than the current LTS; nothing otherwise

A Gradle or Maven toolchain names a JDK the build compiles or tests with,
which Gradle and Maven's toolchains plugin find among the installed JDKs
(/usr/lib/jvm and the like): it only has to be installed. A ceiling is
different: a Gradle wrapper or a Kotlin compiler from before a JDK's release
fails on it (Gradle doesn't start; Kotlin 2.1 throws IllegalArgumentException
parsing "25.0.5", and Kotlin 2.2 can't target the JDK's bytecode version that
Java compiles to), so the build has to run on an older JDK.
"""

from __future__ import annotations

import os
import re
import sys

# The JDK feature releases with long-term support, oldest first.
LTS = (8, 11, 17, 21, 25)

# The first Gradle release that runs on each LTS JDK (Gradle's compatibility
# matrix, docs.gradle.org/current/userguide/compatibility.html).
_GRADLE_RUNS_ON = ((9, 1, 25), (8, 5, 21), (7, 3, 17), (5, 0, 11))
# The first Kotlin release that supports each LTS JDK, running on it and
# targeting its bytecode (Kotlin's "what's new" for each release).
_KOTLIN_RUNS_ON = ((2, 3, 0, 25), (1, 9, 20, 21), (1, 6, 20, 17))

_BUILD_FILES = ("build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts")


def _read(path: str) -> str:
    try:
        with open(path, encoding="utf-8", errors="replace") as f:
            return f.read()
    except OSError:
        return ""


def _numbers(version: str) -> tuple[int, ...]:
    return tuple(int(part) for part in re.findall(r"\d+", version)[:3])


def _files(directory: str, names: tuple[str, ...], depth: int = 3) -> list[str]:
    """Files with these names (or, for a name starting with ".", this
    extension) in a directory and its subdirectories (a multi-project
    build's), skipping build output and dependencies."""
    found = []
    base = directory.rstrip(os.sep).count(os.sep)
    for current, dirs, files in os.walk(directory):
        dirs[:] = [d for d in dirs if d not in ("build", "target", "node_modules", ".gradle", ".git")]
        if current.count(os.sep) - base >= depth:
            dirs[:] = []
        found += [os.path.join(current, name) for name in files
                  if name in names or any(n.startswith(".") and name.endswith(n) for n in names)]
    return sorted(found)


def _poms(directory: str, root: str) -> list[str]:
    """The component's POMs, and its parents' up to the project root (a
    module inherits its parent's toolchain configuration)."""
    poms = _files(directory, ("pom.xml",))
    current = os.path.dirname(os.path.abspath(directory))
    root = os.path.abspath(root)
    while current.startswith(root):
        if os.path.isfile(os.path.join(current, "pom.xml")):
            poms.append(os.path.join(current, "pom.xml"))
        if current == root:
            break
        current = os.path.dirname(current)
    return poms


def toolchains(directory: str, root: str) -> list[int]:
    wanted = set()
    for path in _files(directory, (".gradle", ".gradle.kts"), depth=4):
        text = _read(path)
        wanted.update(int(n) for n in re.findall(r"JavaLanguageVersion\.of\(\s*(\d+)\s*\)", text))
        wanted.update(int(n) for n in re.findall(r"jvmToolchain\(\s*(\d+)\s*\)", text))
    for path in _poms(directory, root):
        text = _read(path)
        if "maven-toolchains-plugin" not in text:
            continue
        # select-jdk-toolchain's version range ([11,12)), or a <jdk> requirement.
        wanted.update(int(n) for n in re.findall(r"<version>\s*\[\s*(\d+)\s*,", text))
        for block in re.findall(r"<jdk>(.*?)</jdk>", text, re.S):
            wanted.update(int(n) for n in re.findall(r"<version>\s*(\d+)", block))
    return sorted(n for n in wanted if n >= 8)


def _gradle_wrapper(directory: str) -> str | None:
    if not os.path.isfile(os.path.join(directory, "gradlew")):
        return None
    text = _read(os.path.join(directory, "gradle", "wrapper", "gradle-wrapper.properties"))
    match = re.search(r"distributionUrl=.*gradle-(\d+(?:\.\d+)*)", text)
    return match.group(1) if match else None


def _kotlin(directory: str) -> str | None:
    """The Kotlin version a Maven or Gradle build compiles with, in its usual spellings."""
    patterns = (
        r"<kotlin\.version>\s*(\d[\w.-]*)\s*</kotlin\.version>",
        r"<artifactId>kotlin-maven-plugin</artifactId>\s*<version>\s*(\d[\w.-]*)",
        r"org\.jetbrains\.kotlin[\w.]*\"?\)?\s+version\s+[\"'](\d[\w.-]*)",
        r"kotlin\(\s*\"[\w.-]+\"\s*\)\s+version\s+\"(\d[\w.-]*)",
        r"kotlin_version\s*=\s*[\"'](\d[\w.-]*)",
        r"^\s*kotlin\s*=\s*\"(\d[\w.-]*)\"",
    )
    files = [os.path.join(directory, name) for name in ("pom.xml",) + _BUILD_FILES]
    files.append(os.path.join(directory, "gradle", "libs.versions.toml"))
    for path in files:
        text = _read(path)
        for pattern in patterns:
            match = re.search(pattern, text, re.M)
            if match:
                return match.group(1)
    return None


def _newest_lts(table, version: str) -> int:
    numbers = _numbers(version)
    for row in table:
        if numbers >= row[:-1]:
            return row[-1]
    return LTS[0] if table is _GRADLE_RUNS_ON else table[-1][-1]


def ceiling(directory: str) -> tuple[int, str] | None:
    limits = []
    gradle = _gradle_wrapper(directory)
    if gradle:
        limits.append((_newest_lts(_GRADLE_RUNS_ON, gradle), f"its Gradle wrapper is Gradle {gradle}"))
    kotlin = _kotlin(directory)
    if kotlin:
        limits.append((_newest_lts(_KOTLIN_RUNS_ON, kotlin), f"it compiles with Kotlin {kotlin}"))
    limits = [limit for limit in limits if limit[0] < LTS[-1]]
    return min(limits) if limits else None


def main(argv: list[str]) -> int:
    if len(argv) == 3 and argv[0] == "toolchains":
        for feature in toolchains(argv[1], argv[2]):
            print(feature)
        return 0
    if len(argv) == 2 and argv[0] == "ceiling":
        found = ceiling(argv[1])
        if found:
            print(f"{found[0]}\t{found[1]}")
        return 0
    print(__doc__, file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
