#!/usr/bin/env python3
"""Real-project test results: collect run.sh logs, render the page.

    report.py collect LOG...   add each log's run (tests/real-projects/run.sh)
                               to results.json, replacing that project's
                               previous one
    report.py render           write docs/real-projects.md,
                               results-by-language.json and a badge per
                               tested project (badges/) from projects.json
                               and results.json
    report.py summary LOG      one log's results, in Markdown (a CI job summary)
    report.py validate         check the data files against schema.json
    report.py categories LOG...
                               failures by category in the logs, parsed
                               afresh: tries a change to the categories on
                               saved logs without rerunning the projects

The data files are described in schema.json; docs/real-projects.md is one
view of them.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(os.path.dirname(HERE))
PROJECTS = os.path.join(HERE, "projects.json")
RESULTS = os.path.join(HERE, "results.json")
BY_LANGUAGE = os.path.join(HERE, "results-by-language.json")
SCHEMA = os.path.join(HERE, "schema.json")
DOC = os.path.join(ROOT, "docs", "real-projects.md")
BADGES = os.path.join(HERE, "badges")
SCHEMA_VERSION = 1
REPO_URL = "https://github.com/dnf/hecatron"
# shields.io's endpoint badge reads each project's badge file from main.
BADGE_FILES_URL = "https://raw.githubusercontent.com/dnf/hecatron/main/tests/real-projects/badges"
PAGE_URL = f"{REPO_URL}/blob/main/docs/real-projects.md"

ANSI = re.compile(r"\x1b\[[0-9;?]*[A-Za-z]|\x1b\][^\x07]*\x07")
STEP = re.compile(r"^### step (\S+): .*?\n(.*?)^### done \1 rc=(-?\d+) secs=(\d+)\n?", re.M | re.S)
JSON_LINE = re.compile(r"^### json (\S+) (.*)$", re.M)
JUNIT_LINE = re.compile(r"^### junit (\S+) (\{.*\})$", re.M)
DISK_LINE = re.compile(r"^### disk (\S+) (\{.*\})$", re.M)
# Free space under this, at any point of a run, is close to running out.
DISK_LOW_KB = 2 * 1024 * 1024
MARKER = re.compile(r"^### (meta|project|cache|end) (\{.*\})$", re.M)
NOT_ADDED = re.compile(r"^\s*not added: (\S+) \((.*)\)\s*$", re.M)

# The tools a hook runs, as its "+ <command>" lines name them: anything else
# after a "+ " (dart pub's "+ yaml 3.1.4") isn't a build system.
BUILD_TOOLS = {
    "alr", "bazel", "bun", "bundle", "busted", "cabal", "cargo", "clj", "clojure", "cmake", "cobc",
    "composer", "crystal", "ctest", "dart", "deno", "dotnet", "dub", "dune", "elm", "elm-test", "fpc",
    "fpm", "flutter", "gleam", "go", "goose", "gprbuild", "gradle", "gradlew", "haxe", "haxelib",
    "janet", "jpm", "julia", "lazbuild", "lein", "lua", "luarocks", "make", "meson", "mill", "mix",
    "mojo", "mvn", "mvnw", "nim", "nimble", "ninja", "npm", "npx", "octave", "odin", "opam", "perl",
    "phpunit", "php", "pixi", "pnpm", "poetry", "prove", "pwsh", "pytest", "python", "python3", "R",
    "raco", "raku", "rake", "rebar3", "Rscript", "rspec", "ruby", "sbt", "scala-cli", "shards",
    "stack", "swift", "swipl", "tox", "uv", "v", "wren", "yarn", "zef", "zig",
}
COMMAND = re.compile(r"^\+ (?:corepack )?(\S+)", re.M)


def julia_counts(text: str) -> list[dict[str, int]]:
    """Julia's Test Summary tables: a header naming the columns (Pass, Fail,
    Error, Broken, Total, Time, each shown only when used) and the top-level
    testset's row under it, numbers right-aligned to their column's name."""
    found = []
    for m in JULIA_SUMMARY.finditer(text):
        head, row = m.group("head"), m.group("row")
        bar = head.index("|")
        if row.find("|") != bar:
            continue
        columns = {c.end(): c.group() for c in re.finditer(r"\w+", head[bar:])}
        values: dict[str, int] = {}
        for number in re.finditer(r"\d+(?!\S)", row[bar:]):
            key = JULIA_COLUMNS.get(columns.get(number.end(), ""))
            if key:
                values[key] = values.get(key, 0) + int(number.group())
        if values:
            found.append(values)
    return found


JULIA_SUMMARY = re.compile(r"^(?P<head>Test Summary:[^\n]*\|[^\n]*)\n(?P<row>[^\n]*\|[^\n]*)$", re.M)
JULIA_COLUMNS = {"Pass": "passed", "Fail": "failed", "Error": "errors", "Broken": "skipped", "Total": "total"}

# Each test runner's summary, and the counts in it. Every match counts: a
# language with several components runs its tool once per component. An
# entry's patterns are one regular expression, a list of them (summed: a
# summary spread over lines that may be missing), or a function returning
# each summary's values.
COUNT_PATTERNS = [
    ("cargo", re.compile(r"test result: \w+\. (?P<passed>\d+) passed; (?P<failed>\d+) failed; (?P<skipped>\d+) ignored")),
    ("pytest", re.compile(r"^=+ (?P<summary>(?:\d+ \w+(?:, )?)+) in [\d.]+s", re.M)),
    ("jest", re.compile(r"^Tests:\s+(?P<summary>.*?), (?P<total>\d+) total", re.M)),
    ("vitest", re.compile(r"^\s*Tests\s+(?P<summary>(?:\d+ \w+(?: \| )?)+) \((?P<total>\d+)\)", re.M)),
    ("dotnet", re.compile(r"(?:Passed|Failed)!\s+-\s+Failed:\s+(?P<failed>\d+), Passed:\s+(?P<passed>\d+), Skipped:\s+(?P<skipped>\d+), Total:\s+(?P<total>\d+)")),
    ("dotnet", re.compile(r"^\s*total: (?P<total>\d+)\n\s*failed: (?P<failed>\d+)\n\s*succeeded: (?P<passed>\d+)\n\s*skipped: (?P<skipped>\d+)", re.M)),
    # Surefire's totals (a module's, or the build's); a class's own line ends
    # in its time, and surefire 2 prints no [INFO] before it.
    ("maven", re.compile(r"^(?:\[(?:INFO|WARNING|ERROR)\] )?Tests run: (?P<total>\d+), Failures: (?P<failed>\d+), Errors: (?P<errors>\d+), Skipped: (?P<skipped>\d+)\s*$", re.M)),
    # Gradle prints counts only when a test fails; otherwise its JUnit XML
    # reports give them (run.sh's "### junit" line).
    ("gradle", re.compile(r"^(?P<total>\d+) tests? completed, (?P<failed>\d+) failed(?:, (?P<skipped>\d+) skipped)?", re.M)),
    ("sbt", re.compile(r"(?:Passed|Failed): Total (?P<total>\d+), Failed (?P<failed>\d+), Errors (?P<errors>\d+), Passed (?P<passed>\d+)")),
    ("scalatest", re.compile(r"Tests: succeeded (?P<passed>\d+), failed (?P<failed>\d+), canceled \d+, ignored (?P<skipped>\d+)")),
    ("julia", julia_counts),
    ("zig", re.compile(r"Build Summary: [^\n]*?(?P<passed>\d+)/(?P<total>\d+) tests passed(?: \((?P<skipped>\d+) skipped\))?(?:; \d+ skipped)?(?:; (?P<failed>\d+) failed)?")),
    ("gtest", [re.compile(r"^\[  PASSED  \] (?P<passed>\d+) tests?\.", re.M),
               re.compile(r"^\[  FAILED  \] (?P<failed>\d+) tests?, listed below", re.M),
               re.compile(r"^\[  SKIPPED \] (?P<skipped>\d+) tests?, listed below", re.M)]),
    ("phpunit", re.compile(r"^OK \((?P<total>\d+) tests?,", re.M)),
    ("phpunit", re.compile(r"^Tests: (?P<total>\d+), Assertions: \d+(?:, (?:Errors: (?P<errors>\d+)|Failures: (?P<failed>\d+)|Skipped: (?P<skipped>\d+)|[\w ]+: \d+))*\.", re.M)),
    ("dart", re.compile(r"\+(?P<passed>\d+)(?: ~(?P<skipped>\d+))?(?: -(?P<failed>\d+))?: (?:All tests passed|Some tests failed)")),
    ("ctest", re.compile(r"\d+% tests passed, (?P<failed>\d+) tests? failed out of (?P<total>\d+)")),
    ("rspec", re.compile(r"^(?P<total>\d+) examples?, (?P<failed>\d+) failures?(?:, (?P<skipped>\d+) pending)?", re.M)),
    ("minitest", re.compile(r"(?P<total>\d+) (?:runs|tests), \d+ assertions, (?P<failed>\d+) failures, (?P<errors>\d+) errors, (?P<skipped>\d+) skips")),
    ("test-unit", re.compile(r"^(?P<total>\d+) tests, \d+ assertions, (?P<failed>\d+) failures, (?P<errors>\d+) errors, (?P<skipped>\d+) pendings", re.M)),
    ("exunit", re.compile(r"^(?:\d+ doctests?, )?(?:\d+ propert(?:y|ies), )?(?P<total>\d+) tests?, (?P<failed>\d+) failures?(?:, (?P<skipped>\d+) (?:skipped|excluded))?", re.M)),
    ("clojure.test", re.compile(r"^Ran (?P<total>\d+) tests containing \d+ assertions\.\n(?P<failed>\d+) failures, (?P<errors>\d+) errors\.", re.M)),
    ("mocha", re.compile(r"^\s+(?P<passed>\d+) passing", re.M)),
    ("node:test", re.compile(r"^[#ℹ] pass (?P<passed>\d+)\n[#ℹ] fail (?P<failed>\d+)", re.M)),
    ("ava", [re.compile(r"^\s+(?P<passed>\d+) tests? passed$", re.M),
             re.compile(r"^\s+(?P<failed>\d+) tests? failed$", re.M),
             re.compile(r"^\s+(?P<skipped>\d+) tests? skipped$", re.M)]),
    ("elm-test", re.compile(r"^Passed:\s+(?P<passed>\d+)\nFailed:\s+(?P<failed>\d+)", re.M)),
    ("xctest", re.compile(r"Executed (?P<total>\d+) tests?, with (?P<failed>\d+) failures?")),
    ("busted", re.compile(r"(?P<passed>\d+) successes? / (?P<failed>\d+) failures? / (?P<errors>\d+) errors? / (?P<skipped>\d+) pending")),
    ("testthat", re.compile(r"\[ FAIL (?P<failed>\d+) \| WARN \d+ \| SKIP (?P<skipped>\d+) \| PASS (?P<passed>\d+) \]")),
    ("pester", re.compile(r"Tests Passed: (?P<passed>\d+), Failed: (?P<failed>\d+), Skipped: (?P<skipped>\d+)")),
    ("pharo", re.compile(r"^(?P<total>\d+) run, (?P<passed>\d+) passes, (?P<failed>\d+) failures, (?P<errors>\d+) errors", re.M)),
    ("unit-threaded", re.compile(r"^(?P<total>\d+) test\(s\) run, (?P<failed>\d+) failed", re.M)),
    ("alcotest", [re.compile(r"Test Successful in [\d.]+s\. (?P<passed>\d+) tests? run"),
                  re.compile(r"^(?P<failed>\d+) failures?! in [\d.]+s\. (?P<total>\d+) tests? run", re.M)]),
    ("ounit", [re.compile(r"^Ran: (?P<total>\d+) tests? in: [\d.]+ seconds\.", re.M),
               re.compile(r"^FAILED: Cases: \d+ Tried: \d+ Errors: (?P<errors>\d+) Failures: (?P<failed>\d+)", re.M)]),
    ("plunit", [re.compile(r"^% (?:All )?(?P<passed>\d+) tests? passed", re.M),
                re.compile(r"^% (?P<failed>\d+) tests? failed", re.M)]),
    ("raco", [re.compile(r"^(?P<failed>\d+)/(?P<total>\d+) test failures$", re.M),
              re.compile(r"^(?P<passed>\d+) tests? passed$", re.M)]),
    ("eunit", re.compile(r"All (?P<passed>\d+) tests passed")),
    ("prove", re.compile(r"Files=\d+, Tests=(?P<total>\d+)")),
]
# go test's package lines, tab-separated (jest's "FAIL  path" and TAP's
# "ok 1 name" aren't).
GO_OK = re.compile(r"^ok  \t\S+\s", re.M)
GO_FAIL = re.compile(r"^FAIL\t\S+\s", re.M)
TAP_OK = re.compile(r"^ok \d+ ", re.M)
TAP_NOT_OK = re.compile(r"^not ok \d+ ", re.M)

# Checked in order, after the exit code and the counts: the first that
# matches names the cause. The causes outside the project (disk, network,
# the toolchain, the system) come first, as they set off errors that look
# like the project's own.
FAILURE_PATTERNS = [
    ("disk", re.compile(r"No space left on device|ENOSPC|Disk quota exceeded")),
    # A tool hecatron's own hooks run, missing: hecatron didn't install it.
    ("toolchain", re.compile(r"/languages/\S+: line \d+: \S+: command not found")),
    ("network", re.compile(
        r"Couldn't resolve host|Could not resolve host|Temporary failure in name resolution|"
        r"Connection timed out|Connection reset by peer|ETIMEDOUT|ECONNRESET|EAI_AGAIN|"
        r"TLS handshake timeout|429 Too Many Requests|rate limit exceeded|503 Service Unavailable")),
    ("toolchain-version", re.compile(
        r"OTP release \d+ or later is required|is newer than \S+ maximum supported version|"
        r"requires (?:a )?(?:newer|older) version of|Unsupported class file major version|"
        r"requires rustc \d|is not compatible with the current Python|"
        r"uninitialized constant DidYouMean::SPELL_CHECKERS|"
        # Zig's build API changes from release to release: a build.zig that
        # doesn't compile was written for another Zig.
        r"^build\.zig:\d+:\d+: error:|"
        # A plugin compiled against another JDK's internals.
        r"NoSuchMethodError: '[^']*com\.sun\.tools\.javac", re.M)),
    ("system", re.compile(
        r"^(?!#)(?!.*/languages/).*: (?:line \d+: )?\S+: command not found$|^/bin/sh: \d+: \S+: not found$|"
        r"Make sure that '[^']+' is installed|"
        r"\[Errno 2\] No such file or directory: '[\w.+-]+'|Cannot run program \"|"
        r"binary not found|not found in PATH|on your PATH to run|did you install it\?|"
        r"executable could not be found|Unknown compiler\(s\)|Dependency \"[^\"]+\" not found|"
        r"Gem::Ext::BuildError|headers or library files could not be found|"
        r"Package '[^']+',? (?:required by '[^']+', )?not found|No package '[^']+' found|"
        r"fatal error: [\w/.+-]+\.h(?:pp)?: No such file or directory|"
        r"Could NOT find \w+|cannot find -l\w+|Could not find a package configuration file provided by|"
        r"Could not find \S+ using the following names|not found, \(required dependency", re.M)),
    ("dependencies", re.compile(
        r"Could not resolve|could not resolve|Failed to download|failed to download|unable to access|"
        r"Could not find artifact|Could not transfer artifact|npm ERR! (?:code E404|network)|ERR_PNPM_FETCH|"
        r"No matching distribution|failed to fetch|version solving failed|Unable to find package|"
        r"error NU1101|failed to select a version|No solution found when resolving|"
        r"incompatible with the declared conflicts|ERR_PNPM_IGNORED_BUILDS|MISSING-DEPENDENCY|"
        r"An error occurred while installing|\[Cabal-7107\]|Bundler::GemNotFound")),
    # The project expects a step of its own first: generated code, a
    # service, a native library built by another part of the repository.
    ("setup", re.compile(
        r"Child module \S+ of \S+ does not exist|error MSB3202: The project file .* was not found|"
        r"failed to load manifest for workspace member|can't find include file \"gen-|"
        r"go: updates to go\.mod needed|no required module provides package|"
        r"cannot load such file|could not find module|parameter not set|"
        r"Unable to contact \S+ server|No such file or directory \(os error 2\)|"
        r"header '[^']+' not found|Class \"[^\"]+\" not found|NoClassDefFoundError")),
    ("build", re.compile(
        r"error\[E\d+\]|error CS\d+|error FS\d+|COMPILATION ERROR|Compilation failure|BUILD FAILED|"
        r"Build FAILED|cannot find symbol|undefined reference|fatal error:|make(?:\[\d+\])?: \*\*\*|"
        r"CMake Error|ninja: build stopped|SyntaxError|ModuleNotFoundError|ImportError|"
        r"error TS\d+|Could not find a package|Cannot find module|compilation terminated|"
        r"error: SwiftCompile|Fatal: Compilation aborted|error MSB\d+|failed to run custom build command|"
        r"Failed to build `|\.mojo:\d+:\d+: error:|\.hx:\d+: characters|error: Build failed|"
        r"Compiling \S+ failed|ERR_PNPM_RECURSIVE_RUN_FIRST_FAIL")),
    ("no-tests", re.compile(
        r"no tests ran in|No tests were found|No test is available|No tests found|"
        r"Total number of tests run: 0\b|0 examples, 0 failures")),
    ("tests", re.compile(
        r"^--- FAIL|test result: FAILED|Some tests failed|AssertionError|Tests? failed|^FAILED |"
        r"\b[1-9]\d* (?:failed|failures?|failing)\b|There (?:was|were) \d+ failures?|"
        r"There are test failures|^not ok \d+|Failing test scripts|\d+/\d+ test failures|"
        r"Tests Passed: \d+, Failed: [1-9]|^ℹ fail [1-9]", re.M)),
]
NO_COMMAND = re.compile(r"no (?:test|lint|build) command for")
NOT_ON_PATH_EXIT = 3
TIMEOUT_EXITS = {124, 137}
NOISE = re.compile(r"^\s*$|^###|Downloading|Downloaded|^\s*Compiling|━|^\[INFO\] ?-*$|^go: downloading|^\s*\|")


# ----- collecting -----------------------------------------------------------

def _json(text: str):
    try:
        return json.loads(text)
    except ValueError:
        return None


def _step(match) -> dict:
    return {"rc": int(match.group(3)), "seconds": int(match.group(4))}


def counts(text: str) -> dict | None:
    """The test runner's own summary in a step's output, totalled."""
    text = ANSI.sub("", text)
    for tool, patterns in COUNT_PATTERNS:
        found = _summaries(patterns, text)
        if found:
            return totals(tool, found)
    ok, fail = len(GO_OK.findall(text)), len(GO_FAIL.findall(text))
    if ok or fail:
        return {"tool": "go (packages)", "passed": ok, "failed": fail}
    ok, fail = len(TAP_OK.findall(text)), len(TAP_NOT_OK.findall(text))
    if ok or fail:
        return {"tool": "tap", "passed": ok, "failed": fail}
    return None


SUMMARY_WORDS = {"passed": "passed", "failed": "failed", "skipped": "skipped", "error": "errors",
                 "errors": "errors", "todo": "skipped", "xfailed": "skipped"}


def _summaries(patterns, text: str) -> list[dict[str, int]]:
    """Each summary's values ({"passed": 3, ...}) the patterns find in text."""
    if callable(patterns):
        return patterns(text)
    found = []
    for pattern in patterns if isinstance(patterns, list) else [patterns]:
        for m in pattern.finditer(text):
            values = {k: int(v) for k, v in m.groupdict().items() if v and k != "summary" and v.isdigit()}
            for n, word in re.findall(r"(\d+) (\w+)", m.groupdict().get("summary") or ""):
                key = SUMMARY_WORDS.get(word)
                if key:
                    values[key] = values.get(key, 0) + int(n)
            found.append(values)
    return found


def totals(tool: str, found: list[dict[str, int]]) -> dict:
    """The summaries' values added up, as results.json records them."""
    total: dict[str, int] = {}
    for values in found:
        for k, v in values.items():
            total[k] = total.get(k, 0) + v
    failed = total.pop("failed", 0) + total.pop("errors", 0)
    result: dict = {"tool": tool}
    if "passed" in total:
        result["passed"] = total["passed"]
    if failed or "passed" in total or "total" in total:
        result["failed"] = failed
    if "skipped" in total:
        result["skipped"] = total["skipped"]
    if "total" in total:
        result["total"] = total["total"]
        if "passed" not in result:
            result["passed"] = max(0, total["total"] - failed - total.get("skipped", 0))
    return result


def junit_counts(junit: dict | None) -> dict | None:
    """Counts from the JUnit XML reports a step wrote (run.sh's "### junit"
    line): Gradle's, and Maven's or sbt's when their output says nothing."""
    if not isinstance(junit, dict) or not junit.get("files"):
        return None
    return totals("junit-xml", [{"total": junit.get("tests", 0), "failed": junit.get("failures", 0),
                                 "errors": junit.get("errors", 0), "skipped": junit.get("skipped", 0)}])


def build_systems(text: str) -> list[str]:
    tools = []
    for command in COMMAND.findall(text):
        tool = os.path.basename(command)
        if tool in BUILD_TOOLS and tool not in tools:
            tools.append(tool)
    return tools


def excerpt(text: str, job: dict | None) -> list[str]:
    """What went wrong: the error lines hecatron picked, else the log's end."""
    errors = [e.get("text", "") for e in (job or {}).get("errors") or []]
    lines = errors or [line for line in ANSI.sub("", text).splitlines() if not NOISE.search(line)][-6:]
    return [line.strip()[:300] for line in lines[:10]]


def failure_category(rc: int, text: str, job: dict | None, install_failed: bool, found: dict | None,
                     action: str = "test") -> str:
    text = ANSI.sub("", text)
    if rc in TIMEOUT_EXITS or (job or {}).get("status") == "timedOut":
        return "timeout"
    if (job or {}).get("exitCode") == NOT_ON_PATH_EXIT or install_failed and re.search(r"command not found|not found on PATH|not installed", text):
        return "toolchain"
    if NO_COMMAND.search(text):
        return "no-command"
    # Failures beside passes are the tests' own; with no test passing, an
    # error before the tests ran (an import, a fetch) explains them better.
    if found and found.get("failed") and found.get("passed"):
        return "tests"
    for category, pattern in FAILURE_PATTERNS:
        if category in ACTION_ONLY_CATEGORIES and action not in ACTION_ONLY_CATEGORIES[category]:
            continue
        if pattern.search(text):
            return category
    if found and found.get("failed"):
        return "tests"
    if install_failed:
        return "toolchain"
    # A linter that ran and exited non-zero found problems.
    return "lint" if action == "lint" else "other"


# Categories that only explain a failure of some actions: a lint or build
# step runs no tests.
ACTION_ONLY_CATEGORIES = {"no-tests": {"test"}, "tests": {"test"}}


def install_failures(text: str, what: str = "install") -> set[str]:
    """The languages install said it couldn't install (what: "install"), or
    couldn't fetch the dependencies of ("fetch dependencies")."""
    failed = set()
    for line in ANSI.sub("", text).splitlines():
        m = re.match(rf"\s*Failed to {what}:? ([a-z ]*)", line)
        if m:
            failed.update(m.group(1).split())
    return failed


def parse_log(text: str) -> dict | None:
    """One run record from a run.sh log, or None when it has no meta line."""
    markers = {m.group(1): _json(m.group(2)) for m in MARKER.finditer(text)}
    meta = markers.get("meta")
    if not meta:
        return None
    jsons = {m.group(1): _json(m.group(2)) for m in JSON_LINE.finditer(text)}
    junits = {m.group(1): _json(m.group(2)) for m in JUNIT_LINE.finditer(text)}
    steps = {m.group(1): m for m in STEP.finditer(text)}
    config = jsons.get("config") if isinstance(jsons.get("config"), dict) else None
    status = jsons.get("status") if isinstance(jsons.get("status"), dict) else None
    status_by_lang = {lang["name"]: lang for lang in (status or {}).get("languages") or []}

    init = None
    if "init" in steps:
        components: dict[str, dict] = {}
        for lang in status_by_lang.values():
            for comp in lang.get("components") or []:
                components.setdefault(comp["name"], {})[lang["name"]] = comp.get("dir", "")
        init = {
            **_step(steps["init"]),
            "languages": (config or {}).get("languages") or [],
            "components": [{"name": name, "dirs": dirs} for name, dirs in sorted(components.items())],
            "notAdded": [{"language": lang, "reason": reason} for lang, reason in NOT_ADDED.findall(steps["init"].group(2))],
            "config": config,
        }
    root = (markers.get("project") or {}).get("root") or ""
    install_text = steps["install"].group(2) if "install" in steps else ""
    not_installed = install_failures(install_text)
    deps_failed = install_failures(install_text, "fetch dependencies")
    actions = [a for a in (meta.get("actions") or "test").split(",") if a]

    languages = []
    for lang in (init or {}).get("languages") or []:
        per_action = {}
        tools: list[str] = []
        components = []
        for action in actions:
            m = steps.get(f"{action}:{lang}")
            if not m:
                continue
            body = m.group(2)
            job = None
            document = jsons.get(f"{action}:{lang}")
            if isinstance(document, dict):
                job = next((j for j in document.get("jobs") or [] if j.get("language") == lang), None)
            found = counts(body) or junit_counts(junits.get(f"{action}:{lang}"))
            rc = int(m.group(3))
            per_action[action] = {
                **_step(m),
                "counts": found,
                "failure": None if rc == 0 else {
                    "category": failure_category(rc, body, job, lang in not_installed, found, action),
                    "excerpt": excerpt(body, job),
                },
            }
            tools += [t for t in build_systems(body) if t not in tools]
            if job and not components:
                components = [{"name": c.get("name", ""), "dir": relative(c.get("dir", ""), root), "failed": bool(c.get("failed"))}
                              for c in job.get("components") or []]
        info = status_by_lang.get(lang) or {}
        languages.append({
            "language": lang,
            "result": language_result(per_action, actions),
            "toolchain": {"versions": info.get("versions") or [], "state": info.get("toolchain")},
            "install": {"toolchainFailed": lang in not_installed, "depsFailed": lang in deps_failed},
            "buildSystems": tools,
            "components": components,
            "steps": per_action,
        })

    end = markers.get("end")
    return {
        "actions": actions,
        "repo": meta["repo"],
        "date": meta["date"],
        "commit": (markers.get("project") or {}).get("commit"),
        "hecatron": meta["hecatron"],
        "runner": meta["runner"],
        "wallSeconds": (end or {}).get("wallSeconds", sum(int(m.group(4)) for m in steps.values())),
        "cache": markers.get("cache"),
        "complete": end is not None,
        "clone": _step(steps["clone"]) if "clone" in steps else None,
        "init": init,
        "install": _step(steps["install"]) if "install" in steps else None,
        "doctor": _step(steps["doctor"]) if "doctor" in steps else None,
        "status": status,
        "disk": disk_summary([(m.group(1), _json(m.group(2))) for m in DISK_LINE.finditer(text)]),
        "languages": languages,
    }


def language_result(per_action: dict, actions: list[str]) -> str:
    """pass, fail or not-run: the test step's result when the run tested
    (lint and build are recorded beside it), else every step's."""
    deciding = ["test"] if "test" in actions else actions
    ran = [per_action[a] for a in deciding if a in per_action]
    if any(step["rc"] != 0 for step in ran):
        return "fail"
    return "pass" if ran and len(ran) == len(deciding) else "not-run"


def disk_summary(samples: list[tuple[str, dict | None]]) -> dict | None:
    """Free space on the workspace's disk: at the start, the end and its
    lowest, from run.sh's "### disk LABEL {...}" lines."""
    valid = [(label, d) for label, d in samples if isinstance(d, dict) and isinstance(d.get("freeKB"), int)]
    if not valid:
        return None
    low_label, low = min(valid, key=lambda s: s[1]["freeKB"])
    return {
        "sizeKB": valid[0][1].get("sizeKB"),
        "freeBeforeKB": valid[0][1]["freeKB"],
        "freeAfterKB": valid[-1][1]["freeKB"],
        "minFreeKB": low["freeKB"],
        "minFreeAfter": low_label,
        "low": low["freeKB"] < DISK_LOW_KB,
    }


def relative(directory: str, root: str) -> str:
    if root and (directory == root or directory.startswith(root + "/")):
        return os.path.relpath(directory, root)
    return directory


def load(path: str, default: dict) -> dict:
    if not os.path.exists(path):
        return default
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def save(path: str, data: dict) -> None:
    with open(path, "w", encoding="utf-8") as f:
        json.dump(data, f, indent=1, ensure_ascii=False)
        f.write("\n")


def collect(logs: list[str]) -> None:
    results = {r["repo"].lower(): r for r in load(RESULTS, {"results": []})["results"]}
    for path in logs:
        with open(path, encoding="utf-8", errors="replace") as f:
            run = parse_log(f.read())
        if run is None:
            print(f"{path}: no run in it", file=sys.stderr)
            continue
        results[run["repo"].lower()] = run
    save(RESULTS, {"schemaVersion": SCHEMA_VERSION, "results": sorted(results.values(), key=lambda r: r["repo"].lower())})


# ----- rendering ------------------------------------------------------------

MARKS = [
    ("pass", "✅", "Its tests pass: `hct test <language>` exits 0"),
    ("fail", "❌", "Fails: install, build or the tests"),
    ("not-run", "⏱️", "Not run: the job ran out of time before it"),
    ("not-detected", "➖", "Expected, but `hct init` didn't add it"),
    ("untested", "❔", "Not tested yet"),
]
MARK = {key: mark for key, mark, _ in MARKS}
CATEGORIES = [
    ("polyglot", "Polyglot", "One project in several languages: the case hecatron is for."),
    ("single", "Single language", "Well-known projects in one or two languages, at least one for each language hecatron supports."),
    ("stress", "Stress", "Monorepos, several build systems in one repository, native extensions."),
]
FAILURES = {
    "toolchain": "Toolchain not installed or not found",
    "toolchain-version": "Toolchain too new or too old for the project",
    "system": "A system tool or library missing",
    "network": "Network error",
    "disk": "Out of disk space",
    "dependencies": "Fetching or resolving dependencies failed",
    "setup": "Needs a setup step of the project's own (generated code, a service, a native library)",
    "build": "Build or compile error",
    "no-tests": "No tests found",
    "tests": "Tests failed",
    "lint": "The linter reported problems",
    "no-command": "No command for the layout",
    "timeout": "Timed out",
    "other": "Other",
}


def supported_languages() -> list[str]:
    base = os.path.join(ROOT, "languages")
    return sorted(d for d in os.listdir(base) if os.path.isdir(os.path.join(base, d)) and not d.startswith("_"))


def rows(projects: dict, results: dict) -> list[dict]:
    """One row per backlog project and language: results-by-language.json."""
    out = []
    for project in projects["projects"]:
        run = results.get(project["repo"].lower())
        detected = {lang["language"]: lang for lang in (run or {}).get("languages") or []}
        detected_names = set((run or {}).get("init") and run["init"]["languages"] or [])
        for language in sorted(set(project["languages"]) | detected_names):
            entry = detected.get(language)
            if run is None:
                result = "untested"
            elif entry:
                result = entry["result"]
            elif language in detected_names:
                result = "not-run"
            else:
                result = "not-detected"
            steps = (entry or {}).get("steps", {})
            test = steps.get("test") or {}
            row = {
                "repo": project["repo"],
                "category": project["category"],
                "language": language,
                "expected": language in project["languages"],
                "detected": None if run is None else language in detected_names,
                "result": result,
                "seconds": test.get("seconds"),
                "failureCategory": (test.get("failure") or {}).get("category"),
                "commit": (run or {}).get("commit"),
                "date": (run or {}).get("date"),
                "hecatronVersion": (run or {}).get("hecatron", {}).get("version"),
            }
            others = {action: "pass" if step["rc"] == 0 else "fail" for action, step in steps.items() if action != "test"}
            if others:
                row["actions"] = others
            out.append(row)
    return out


def _minutes(step: dict | None) -> str:
    if not step:
        return "–"
    secs = step["seconds"]
    text = f"{secs}s" if secs < 60 else f"{secs // 60}m {secs % 60:02d}s"
    return text if step["rc"] == 0 else f"{text} ❌"


def _project_state(project_rows: list[dict]) -> str | None:
    """all, some or none of a tested project's detected languages pass."""
    tested = [r for r in project_rows if r["detected"]]
    if not project_rows or project_rows[0]["result"] == "untested":
        return None
    passed = sum(r["result"] == "pass" for r in tested)
    if tested and passed == len(tested):
        return "all"
    return "some" if passed else "none"


def badge_name(repo: str) -> str:
    return repo.replace("/", "_") + ".json"


def badge(project_rows: list[dict]) -> dict | None:
    """A shields.io endpoint badge: whether the project's detected languages pass."""
    state = _project_state(project_rows)
    if state is None:
        return None
    tested = [r for r in project_rows if r["detected"]]
    passed = sum(r["result"] == "pass" for r in tested)
    message, color = {
        "all": ("passing", "brightgreen"),
        "some": (f"{passed}/{len(tested)} languages", "yellow"),
        "none": ("failing", "red"),
    }[state]
    return {"schemaVersion": 1, "label": "hecatron", "message": message, "color": color}


def badges(projects: dict, results: dict) -> dict[str, dict]:
    by_repo: dict[str, list[dict]] = {}
    for r in rows(projects, results):
        by_repo.setdefault(r["repo"], []).append(r)
    found = {badge_name(repo): badge(rs) for repo, rs in by_repo.items()}
    return {name: data for name, data in found.items() if data is not None}


def badge_markdown(repo: str) -> str:
    image = f"https://img.shields.io/endpoint?url={BADGE_FILES_URL}/{badge_name(repo)}"
    return f"[![hecatron]({image})]({PAGE_URL})"


def render_text(projects: dict, results: dict) -> str:
    table = rows(projects, results)
    by_repo: dict[str, list[dict]] = {}
    for r in table:
        by_repo.setdefault(r["repo"], []).append(r)
    tested_rows = [r for r in table if r["detected"]]
    states = {repo: _project_state(rs) for repo, rs in by_repo.items()}

    out = [
        "# Real projects",
        "",
        "Hecatron tried on open-source repositories as a newcomer would use it:",
        "a fresh Ubuntu container clones the project, then runs `hct init`,",
        "`hct install`, `hct doctor` and `hct test <language>` for each language",
        "init found, with no configuration and no hooks. A language passes when",
        "its tests pass. The workflow `.github/workflows/real-projects.yml` runs",
        "every project monthly on GitHub's runners, one job each, with an hour",
        "per project. Generated by `tests/real-projects/report.py render`; don't",
        "edit by hand. The data behind it, for other uses:",
        "[`projects.json`](../tests/real-projects/projects.json) (the backlog, with",
        "GitHub metadata), [`results.json`](../tests/real-projects/results.json)",
        "(each run in full) and",
        "[`results-by-language.json`](../tests/real-projects/results-by-language.json)",
        "(one row per project and language), described by",
        "[`schema.json`](../tests/real-projects/schema.json). Projects not run yet",
        "show as ❔: the page is the backlog too.",
        "",
        "| Status | Meaning |",
        "| --- | --- |",
    ]
    out += [f"| {mark} | {meaning} |" for _, mark, meaning in MARKS]

    def summary(repos: list[str]) -> list[str]:
        rs = [r for r in tested_rows if r["repo"] in repos]
        tested = [repo for repo in repos if states[repo] is not None]
        return [
            str(len(repos)),
            str(len(tested)),
            str(sum(states[repo] == "all" for repo in tested)),
            str(sum(states[repo] == "some" for repo in tested)),
            str(sum(states[repo] == "none" for repo in tested)),
            f"{sum(r['result'] == 'pass' for r in rs)}/{len(rs)}",
        ]

    out += ["", "## Totals", "",
            "| Category | Projects | Tested | All languages pass | Some pass | None pass | Languages passing |",
            "| --- | --: | --: | --: | --: | --: | --: |"]
    for key, name, _ in CATEGORIES:
        repos = [p["repo"] for p in projects["projects"] if p["category"] == key]
        out.append(f"| {name} | " + " | ".join(summary(repos)) + " |")
    out.append("| **All** | " + " | ".join(f"**{v}**" for v in summary([p["repo"] for p in projects["projects"]])) + " |")
    out += ["", "A language counts when `hct init` added it; one expected but not",
            "detected (➖) is listed with its project but not in these counts."]

    failures: dict[str, int] = {}
    for r in tested_rows:
        if r["result"] == "fail":
            key = r["failureCategory"] or "other"
            failures[key] = failures.get(key, 0) + 1
    if failures:
        out += ["", "### Why languages fail", "", "| Cause | Languages |", "| --- | --: |"]
        for key, n in sorted(failures.items(), key=lambda kv: (-kv[1], kv[0])):
            out.append(f"| {FAILURES[key]} | {n} |")

    out += _other_actions_text(tested_rows)
    out += _disk_text(results)

    dates = sorted({r["date"] for r in results.values() if r.get("date")})
    if dates:
        out += ["", f"Runs from {dates[0]} to {dates[-1]}." if len(dates) > 1 else f"Run on {dates[0]}."]
    out += [f"Backlog verified against the GitHub API on {projects['verified']}."]

    for key, name, blurb in CATEGORIES:
        out += ["", f"## {name}", "", blurb, "",
                "| Project | Languages | Commit | hecatron | init | install | Date |",
                "| --- | --- | --- | --- | --: | --: | --- |"]
        for project in (p for p in projects["projects"] if p["category"] == key):
            repo = project["repo"]
            run = results.get(repo.lower())
            cells = " ".join(f"{r['language']} {MARK[r['result']]}" for r in by_repo.get(repo, []))
            if run is None:
                out.append(f"| [{repo}](https://github.com/{repo}) | {cells} | | | | | |")
                continue
            commit = run.get("commit") or ""
            hecatron = run["hecatron"]
            version = hecatron["version"]
            if hecatron.get("commit"):
                version = f"[{version}]({REPO_URL}/commit/{hecatron['commit']})"
            commit_cell = f"[{commit[:7]}](https://github.com/{repo}/commit/{commit})" if commit else ""
            init = run.get("init")
            out.append(f"| [{repo}](https://github.com/{repo}) | {cells} | {commit_cell} | {version} "
                       f"| {_minutes(init)} | {_minutes(run.get('install'))} | {run['date']} |")

    out += ["", "## By language", "",
            "For each language hecatron supports, the projects it was tested in.", "",
            "| Language | Passing | Projects |", "| --- | --: | --- |"]
    for language in supported_languages():
        rs = [r for r in table if r["language"] == language and r["result"] != "untested"]
        tested = [r for r in rs if r["detected"]]
        passing = f"{sum(r['result'] == 'pass' for r in tested)}/{len(tested)}" if tested else "–"
        listed = " ".join(f"[{r['repo']}](https://github.com/{r['repo']}) {MARK[r['result']]}" for r in rs)
        untested = sum(1 for r in table if r["language"] == language and r["result"] == "untested")
        if untested:
            listed = (listed + " · " if listed else "") + f"{untested} untested"
        out.append(f"| {language} | {passing} | {listed or '–'} |")

    passing_repos = [p["repo"] for p in projects["projects"] if states.get(p["repo"]) == "all"]
    out += ["", "## Badge", "",
            "Each tested project has a badge that follows its latest result here:",
            "`passing` when every language `hct init` found passes, `N/M languages`",
            "when some do, `failing` when none do. It updates with each monthly run.",
            "To show it in a project's README, copy its line:", ""]
    if passing_repos:
        out += ["| Project | Markdown |", "| --- | --- |"]
        out += [f"| {repo} | `{badge_markdown(repo)}` |" for repo in passing_repos]
    else:
        out.append("No project passes in every language yet.")
    out += ["", "Projects not listed have a badge too, at",
            "`tests/real-projects/badges/OWNER_REPO.json`."]
    return "\n".join(out) + "\n"


def _other_actions_text(tested_rows: list[dict]) -> list[str]:
    """How lint and build did, for runs that asked for them (--actions)."""
    tally: dict[str, dict[str, int]] = {}
    for r in tested_rows:
        for action, result in (r.get("actions") or {}).items():
            tally.setdefault(action, {"pass": 0, "fail": 0})[result] += 1
    if not tally:
        return []
    out = ["", "### Lint and build", "",
           "For the runs that asked for them (`run.sh --actions`); a language's",
           "result above is its tests'.", "",
           "| Action | Languages passing |", "| --- | --: |"]
    for action, n in sorted(tally.items()):
        out.append(f"| {action} | {n['pass']}/{n['pass'] + n['fail']} |")
    return out


def _gb(kb: int | None) -> str:
    return "?" if kb is None else f"{kb / 1024 / 1024:.1f} GB"


def _disk_text(results: dict) -> list[str]:
    """The projects whose run came close to filling the disk."""
    if not any(r.get("disk") for r in results.values()):
        return []
    low = sorted((r for r in results.values() if (r.get("disk") or {}).get("low")), key=lambda r: r["disk"]["minFreeKB"])
    out = ["", "### Disk space", "",
           f"Projects whose run left under {_gb(DISK_LOW_KB)} free on the runner's disk at some point:", ""]
    if not low:
        return out + ["None."]
    out += ["| Project | Lowest free | After | Start | Disk |", "| --- | --: | --- | --: | --: |"]
    for r in low:
        d = r["disk"]
        out.append(f"| [{r['repo']}](https://github.com/{r['repo']}) | {_gb(d['minFreeKB'])} | {d['minFreeAfter']} "
                   f"| {_gb(d['freeBeforeKB'])} | {_gb(d['sizeKB'])} |")
    return out


def by_language_data(projects: dict, results: dict) -> dict:
    return {"schemaVersion": SCHEMA_VERSION, "rows": rows(projects, results)}


def load_all() -> tuple[dict, dict]:
    projects = load(PROJECTS, {"projects": [], "verified": "never"})
    results = {r["repo"].lower(): r for r in load(RESULTS, {"results": []})["results"]}
    return projects, results


def render() -> None:
    projects, results = load_all()
    backlog = {p["repo"].lower() for p in projects["projects"]}
    for repo in sorted(set(results) - backlog):
        print(f"{repo}: has results but isn't in projects.json; left out", file=sys.stderr)
    with open(DOC, "w", encoding="utf-8") as f:
        f.write(render_text(projects, results))
    save(BY_LANGUAGE, by_language_data(projects, results))
    write_badges(badges(projects, results))


def write_badges(wanted: dict[str, dict]) -> None:
    os.makedirs(BADGES, exist_ok=True)
    for name in os.listdir(BADGES):
        if name.endswith(".json") and name not in wanted:
            os.remove(os.path.join(BADGES, name))
    for name, data in wanted.items():
        save(os.path.join(BADGES, name), data)


# ----- validating -----------------------------------------------------------

TYPES = {"object": dict, "array": list, "string": str, "boolean": bool, "null": type(None)}


def validate(value, schema: dict, root: dict, path: str = "$") -> list[str]:
    """Errors in value against the JSON Schema subset schema.json uses."""
    if "$ref" in schema:
        target = root
        for part in schema["$ref"].lstrip("#/").split("/"):
            target = target[part]
        errors = validate(value, target, root, path)
        rest = {k: v for k, v in schema.items() if k not in ("$ref", "description")}
        return errors + (validate(value, rest, root, path) if rest else [])
    for key in ("anyOf", "oneOf"):
        if key in schema:
            matches = [not validate(value, option, root, path) for option in schema[key]]
            if (key == "anyOf" and not any(matches)) or (key == "oneOf" and sum(matches) != 1):
                return [f"{path}: matches {sum(matches)} of {key}'s schemas"]
    if "type" in schema:
        types = schema["type"] if isinstance(schema["type"], list) else [schema["type"]]

        def is_type(t: str) -> bool:
            if t == "integer":
                return isinstance(value, int) and not isinstance(value, bool)
            if t == "number":
                return isinstance(value, (int, float)) and not isinstance(value, bool)
            return isinstance(value, TYPES[t])
        if not any(is_type(t) for t in types):
            return [f"{path}: not {' or '.join(types)}"]
    errors = []
    if "enum" in schema and value not in schema["enum"]:
        errors.append(f"{path}: {value!r} not one of {schema['enum']}")
    if "pattern" in schema and isinstance(value, str) and not re.search(schema["pattern"], value):
        errors.append(f"{path}: {value!r} doesn't match {schema['pattern']}")
    if "minimum" in schema and isinstance(value, (int, float)) and value < schema["minimum"]:
        errors.append(f"{path}: {value} < {schema['minimum']}")
    if isinstance(value, dict):
        for key in schema.get("required", []):
            if key not in value:
                errors.append(f"{path}: missing {key}")
        properties = schema.get("properties", {})
        extra = schema.get("additionalProperties", True)
        for key, item in value.items():
            if key in properties:
                errors += validate(item, properties[key], root, f"{path}.{key}")
            elif extra is False:
                errors.append(f"{path}: unexpected {key}")
            elif isinstance(extra, dict):
                errors += validate(item, extra, root, f"{path}.{key}")
    if isinstance(value, list) and "items" in schema:
        for i, item in enumerate(value):
            errors += validate(item, schema["items"], root, f"{path}[{i}]")
    return errors


def validate_files() -> list[str]:
    with open(SCHEMA, encoding="utf-8") as f:
        schema = json.load(f)
    errors = []
    for path, definition in ((PROJECTS, "projectsFile"), (RESULTS, "resultsFile"), (BY_LANGUAGE, "byLanguageFile")):
        if os.path.exists(path):
            with open(path, encoding="utf-8") as f:
                data = json.load(f)
            errors += [f"{os.path.basename(path)} {e}" for e in validate(data, schema["$defs"][definition], schema)]
    return errors


def summary(path: str) -> str:
    with open(path, encoding="utf-8", errors="replace") as f:
        run = parse_log(f.read())
    if run is None:
        return f"{path}: no run in it\n"
    disk = run.get("disk")
    out = [f"### {run['repo']} at {(run['commit'] or '?')[:7]}", "",
           f"init {_minutes(run['init'])}, install {_minutes(run['install'])}, doctor {_minutes(run['doctor'])}"
           + ("" if run["complete"] else " (incomplete)")
           + (f" (restored from the cache: {', '.join(run['cache']['restored'])})" if (run["cache"] or {}).get("restored") else "")]
    if disk:
        out[-1] += (f"; disk free {_gb(disk['freeBeforeKB'])} at the start, lowest {_gb(disk['minFreeKB'])}"
                    f" after {disk['minFreeAfter']}" + (" ⚠️" if disk["low"] else ""))
    others = [a for a in run.get("actions") or ["test"] if a != "test"]
    out += ["", "| Language | Result | Seconds | Tests | Failure |" + "".join(f" {a} |" for a in others),
            "| --- | :-: | --: | --- | --- |" + " --- |" * len(others)]
    for lang in run["languages"]:
        test = lang["steps"].get("test") or {}
        found = test.get("counts") or {}
        tests = ", ".join(f"{found[k]} {k}" for k in ("passed", "failed", "skipped") if k in found)
        failure = test.get("failure") or {}
        cells = ""
        for action in others:
            step = lang["steps"].get(action)
            cells += " – |" if step is None else f" {'✅' if step['rc'] == 0 else '❌ ' + step['failure']['category']} |"
        out.append(f"| {lang['language']} | {MARK[lang['result']]} | {test.get('seconds', '')} | {tests} "
                   f"| {failure.get('category', '')} |" + cells)
    return "\n".join(out) + "\n"


def category_counts(logs: list[str]) -> dict[str, int]:
    """Failures by category across logs, parsed afresh: to try a change to
    the categories on saved logs without rerunning the projects."""
    found: dict[str, int] = {}
    for path in logs:
        with open(path, encoding="utf-8", errors="replace") as f:
            run = parse_log(f.read())
        for lang in (run or {}).get("languages") or []:
            for action, step in lang["steps"].items():
                if step["failure"]:
                    key = step["failure"]["category"] if action == "test" else f"{action}: {step['failure']['category']}"
                    found[key] = found.get(key, 0) + 1
    return dict(sorted(found.items(), key=lambda kv: (-kv[1], kv[0])))


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    sub = parser.add_subparsers(dest="command", required=True)
    c = sub.add_parser("collect")
    c.add_argument("logs", nargs="+")
    sub.add_parser("render")
    s = sub.add_parser("summary")
    s.add_argument("log")
    sub.add_parser("validate")
    k = sub.add_parser("categories")
    k.add_argument("logs", nargs="+")
    args = parser.parse_args()
    if args.command == "collect":
        collect(args.logs)
    elif args.command == "render":
        render()
    elif args.command == "summary":
        print(summary(args.log), end="")
        return 0
    elif args.command == "categories":
        for key, n in category_counts(args.logs).items():
            print(f"{n:5d}  {key}")
        return 0
    errors = validate_files()
    for error in errors[:50]:
        print(error, file=sys.stderr)
    return 1 if errors else 0


if __name__ == "__main__":
    sys.exit(main())
