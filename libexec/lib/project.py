#!/usr/bin/env python3
"""The hecatron project model: configuration, languages and file resolution.

A project is a directory with `.hecatron/config.json` (docs/guide/configuration.md).
Everything that needs to know about the project reads it through this module,
from Python (`import project`) or from shell (`python3 project.py <query>`).

Shell queries:
    root                        print the project root
    shell-env                   print shell variables and functions for eval
    resolve <lang> <file>       print the path a language's file resolves to
    impl-dirs <lang>            print each component's directory for a language
    version <lang> [component]  print a language's version spec, its source and
                                the installed match (docs/guide/versions.md)
    versions <lang>             print each distinct version spec the components
                                resolve to, with its sources
    env <lang> [component]      print shell exports for the language's environment
    missing-versions <lang>     print each component whose version isn't installed
    version-notes <lang>        print each component on the newest version it
                                supports rather than the default
    tasks <action>              print the project's extra tasks for an action
    task <action> <name>        print one task's command
"""

from __future__ import annotations

import fnmatch
import functools
import glob
import json
import os
import posixpath
import re
import shlex
import subprocess
import sys
from datetime import date, timedelta

import versions

SCHEMA = 1
# How old a checkout's last commit is before its toolchains are the releases
# current then (Project.as_of).
# Build output and dependencies, which floorFiles aren't read from.
FLOOR_FILES_SKIPPED = {"bin", "obj", "node_modules", "build", "target", "packages"}

AS_OF_AFTER_DAYS = 365
HECATRON_HOME = os.environ.get(
    "HECATRON_HOME", os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
# HECATRON_BUILTIN_LANGUAGES points the tests at fixture definitions.
BUILTIN_LANGUAGES = os.environ.get("HECATRON_BUILTIN_LANGUAGES") or os.path.join(HECATRON_HOME, "languages")
# Built-in default hooks live in languages/<lang>/hooks/ (docs/guide/hooks.md).
HOOKS = ("test.sh", "lint.sh", "build.sh", "clean.sh", "audit.sh", "format.sh", "deps.sh", "outdated.sh", "docs.sh", "bench.sh")


class ProjectError(Exception):
    """A project that can't be found or read."""


def find_root(start: str | None = None) -> str:
    """The nearest directory at or above `start` holding .hecatron/config.json;
    else the top of the git repository around it, which hecatron reads as
    detected (detected_config)."""
    directory = os.path.abspath(start or os.getcwd())
    while True:
        if os.path.isfile(os.path.join(directory, ".hecatron", "config.json")):
            return directory
        parent = os.path.dirname(directory)
        if parent == directory:
            break
        directory = parent
    top = _git(os.path.abspath(start or os.getcwd()), "rev-parse", "--show-toplevel")
    if top is None:
        raise ProjectError(
            "not a hecatron project (no .hecatron/config.json here or above) and not inside "
            "a git repository; hecatron init sets up a git repository")
    return top


def ignore_project_files(root: str) -> None:
    """Add what a project writes but doesn't commit to its .gitignore."""
    path = os.path.join(root, ".gitignore")
    existing = open(path, encoding="utf-8").read() if os.path.isfile(path) else ""
    missing = [entry for entry in (".hecatron/local.json", ".logs/") if entry not in existing.split("\n")]
    if missing:
        with open(path, "a", encoding="utf-8") as f:
            f.write(("" if not existing or existing.endswith("\n") else "\n") + "\n".join(missing) + "\n")


# Trees holding code that isn't the project's own implementation: vendored
# dependencies, samples, tests and test data, docs, tooling (hooks/: git
# hooks, build hooks) and a generator's templates, whatever their case
# (Examples/), and hidden directories (.jenkins/, .github/): detection
# doesn't count them (manage.py).
SKIPPED_DIRS = {"android", "benchmarks", "buildsrc", "carthage", "demo", "demos", "doc", "docs", "example", "examples",
                "external", "fixtures", "hooks", "node_modules", "playground", "pods", "sample", "samples", "templates",
                "test", "test-data", "test_data", "testdata", "tests", "tests_data", "third-party", "third_party", "thirdparty",
                "tools", "tutorial", "tutorials", "vendor", "vendored"}


def yields(rule: dict, paths, build_file: str | None = None) -> bool:
    """Whether a definition's "detectYields" rule holds for a directory, from
    the paths of the files in it and below (relative to it): more match its
    "where" than its "outnumber" (less those matching its "except"), by file
    name, dotfiles left out. A tsconfig.json beside mostly JavaScript
    (antlr4's runtime/JavaScript, with type declarations) is a JavaScript
    package's. Build output doesn't count, so a TypeScript package that
    commits its compiled JavaScript stays TypeScript's: the directories
    directly below named in "skipDirs" (dist/, lib/), the one the build
    file names at "outputDir" (tsconfig.json's compilerOptions.outDir), and,
    with "twins", a "where" file whose stem an "outnumber" file shares (the
    compiled index.js of index.ts)."""
    output = _output_dir(build_file, rule.get("outputDir")) if build_file else None
    skipped = set(rule.get("skipDirs", []))
    names = []
    for path in paths:
        parts = path.split("/")
        if len(parts) > 1 and parts[0] in skipped or output and (path == output or path.startswith(output + "/")):
            continue
        if not parts[-1].startswith("."):
            names.append(parts[-1])

    def matches(name: str, patterns: list[str]) -> bool:
        return any(fnmatch.fnmatch(name, p) for p in patterns)
    own = [name for name in names if matches(name, rule.get("outnumber", [])) and not matches(name, rule.get("except", []))]
    stems = {os.path.splitext(name)[0] for name in own} if rule.get("twins") else set()
    others = [name for name in names if matches(name, rule.get("where", [])) and os.path.splitext(name)[0] not in stems]
    return len(others) > len(own)


def yields_in(definition: dict, directory: str, listing) -> bool:
    """Whether one of a definition's build files is in the directory (whose
    names are listing) and its "detectYields" rule holds there, as detection
    decides: the directory is the other language's. The Node hooks ask it
    too (node_below_dirs in languages/_hooks.sh)."""
    rule = definition.get("detectYields")
    if not rule:
        return False
    build_file = build_file_in(directory, listing, definition.get("detect", []))
    return build_file is not None and yields(rule, tracked_paths_below(directory), build_file)


def read_definition(lang: str, root: str | None = None) -> dict:
    """A language's config.json, the built-in one's keys overlaid by the
    project's at root (as Project.definitions merges them), without reading
    the project's config."""
    merged = _read_json(os.path.join(BUILTIN_LANGUAGES, lang, "config.json"), {})
    if root:
        merged.update(_read_json(os.path.join(root, ".hecatron", "languages", lang, "config.json"), {}))
    return merged


def _output_dir(build_file: str, keys) -> str | None:
    """The directory a build file names at a key path (["compilerOptions",
    "outDir"]), relative to the build file's directory and normalized; None
    when it names none inside it, or the file can't be read."""
    if not isinstance(keys, list) or not keys:
        return None
    try:
        with open(build_file, encoding="utf-8") as f:
            value = json.loads(loose_json(f.read()))
    except (OSError, UnicodeDecodeError, ValueError):
        return None
    for key in keys:
        if not isinstance(value, dict) or not isinstance(key, str):
            return None
        value = value.get(key)
    if not isinstance(value, str):
        return None
    directory = posixpath.normpath(value)
    if directory in (".", "..") or directory.startswith(("/", "../")):
        return None
    return directory


def loose_json(text: str) -> str:
    """JSON with comments (// and /* */) and trailing commas, as tsconfig.json
    allows, made strict JSON; strings are kept as written."""
    out, i, n = [], 0, len(text)
    while i < n:
        c = text[i]
        if c == '"':
            j = i + 1
            while j < n and text[j] != '"':
                j += 2 if text[j] == "\\" else 1
            out.append(text[i:j + 1])
            i = j + 1
        elif text.startswith("//", i):
            j = text.find("\n", i)
            i = n if j < 0 else j
        elif text.startswith("/*", i):
            j = text.find("*/", i + 2)
            i = n if j < 0 else j + 2
        else:
            out.append(c)
            i += 1
    strict, i, text = [], 0, "".join(out)
    n = len(text)
    while i < n:
        c = text[i]
        if c == '"':
            j = i + 1
            while j < n and text[j] != '"':
                j += 2 if text[j] == "\\" else 1
            strict.append(text[i:j + 1])
            i = j + 1
            continue
        if c == ",":
            j = i + 1
            while j < n and text[j] in " \t\r\n":
                j += 1
            if j < n and text[j] in "}]":
                i += 1
                continue
        strict.append(c)
        i += 1
    return "".join(strict)


def build_file_in(directory: str, names, patterns: list[str]) -> str | None:
    """The first file, by pattern and then by name, of the names in a
    directory matching the patterns: the build file a hit was found by."""
    for pattern in patterns:
        for name in sorted(names):
            if fnmatch.fnmatch(name, pattern):
                return os.path.join(directory, name)
    return None


@functools.lru_cache(maxsize=None)
def tracked_paths_below(directory: str) -> tuple[str, ...]:
    """The paths, relative to it, of the files git tracks in a directory and
    below it, outside SKIPPED_DIRS and hidden directories, as detection
    counts them."""
    listing = _git(directory, "ls-files")
    paths = []
    for path in (listing or "").splitlines():
        parts = path.split("/")
        if not any(part.lower() in SKIPPED_DIRS or part.startswith(".") for part in parts[:-1]):
            paths.append(path)
    return tuple(paths)


def _git(directory: str, *args: str) -> str | None:
    """A git command's output in directory, or None when it fails."""
    try:
        result = subprocess.run(["git", *args], cwd=directory, capture_output=True, text=True)
    except OSError:
        return None
    return result.stdout.strip() if result.returncode == 0 else None


def hecatron_dir_in_git(root: str) -> str:
    """Where hecatron keeps what it detects and records about a repository with
    no .hecatron/ of its own: inside .git, so nothing appears in the working
    tree or in git status."""
    return os.path.join(_git(root, "rev-parse", "--absolute-git-dir") or os.path.join(root, ".git"), "hecatron")


def detected_config(root: str) -> dict:
    """The config.json hecatron init would write for the repository, cached
    in .git/hecatron/ until the commit, git's index (what's tracked) or
    hecatron changes. Said on stderr the first time."""
    cache = os.path.join(hecatron_dir_in_git(root), "detected.json")
    index = os.path.join(_git(root, "rev-parse", "--absolute-git-dir") or "", "index")
    stat = os.stat(index) if os.path.isfile(index) else None
    key = [_git(root, "rev-parse", "-q", "--verify", "HEAD") or "", stat.st_mtime_ns if stat else 0,
           stat.st_size if stat else 0, hecatron_version()]
    cached = _read_json(cache, None) if os.path.isfile(cache) else None
    if cached and cached.get("key") == key:
        return cached["config"]
    sys.path.insert(0, os.path.join(HECATRON_HOME, "libexec", "commands"))
    import manage  # noqa: E402 - only when detecting: it is the init command's module
    config, _ = manage.detected_config(root)
    del config["logs"]
    if cached is None and _for_a_person():
        print(f"hecatron: {root} has no .hecatron/config.json; detected "
              f"{', '.join(config['languages']) or 'no languages'} (hecatron init saves this)", file=sys.stderr)
    _write_json(cache, {"key": key, "config": config})
    return config


def _for_a_person() -> bool:
    """Whether a note meant for a person may go to stderr: stderr a terminal
    and human output (the binary's runner.ForAPerson): no --output, --json
    or HECATRON_OUTPUT naming another format, not an AI agent's session,
    not GitHub Actions."""
    if not sys.stderr.isatty():
        return False
    output = os.environ.get("HECATRON_OUTPUT", "")
    args = sys.argv[1:]
    for i, arg in enumerate(args):
        if arg == "--":
            break
        if arg == "--output" and i + 1 < len(args):
            output = args[i + 1]
        elif arg.startswith("--output="):
            output = arg[len("--output="):]
        elif arg == "--json":
            output = "json"
    if output:
        return output == "human"
    agent = os.environ.get("CLAUDECODE") == "1" and not sys.stdout.isatty()
    return not agent and os.environ.get("GITHUB_ACTIONS") != "true"


def _read_json(path: str, default):
    if not os.path.isfile(path):
        return default
    with open(path, encoding="utf-8") as f:
        try:
            return json.load(f)
        except json.JSONDecodeError as e:
            raise ProjectError(f"{path}: {e}") from e


def _write_json(path: str, data) -> None:
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        json.dump(data, f, indent=2)
        f.write("\n")


def _version_tuple(version: str) -> tuple[int, ...]:
    return tuple(int(part) for part in version.split(".") if part.isdigit())


def hecatron_version() -> str:
    path = os.path.join(HECATRON_HOME, "VERSION")
    if os.path.isfile(path):
        with open(path, encoding="utf-8") as f:
            return f.read().strip()
    return "0.0.0"


class Project:
    def __init__(self, root: str):
        self.root = root
        self.dir = os.path.join(root, ".hecatron")
        self.config_path = os.path.join(self.dir, "config.json")
        self.local_path = os.path.join(self.dir, "local.json")
        # A git repository with no config of its own: detected, and its local
        # settings and logs kept inside .git, so the working tree stays as it is.
        self.detected = not os.path.exists(self.config_path)
        if self.detected:
            self.config = detected_config(root)
            self.local_path = os.path.join(hecatron_dir_in_git(root), "local.json")
        else:
            self.config = _read_json(self.config_path, None)
        if self.config is None:
            raise ProjectError(f"{self.config_path} not found")
        schema = self.config.get("schema")
        if schema != SCHEMA:
            raise ProjectError(
                f"{self.config_path}: schema {schema!r} is not supported "
                f"(this hecatron reads schema {SCHEMA})")
        needed = str(self.config.get("hecatron", "")).lstrip(">=").strip()
        if needed and _version_tuple(hecatron_version()) < _version_tuple(needed):
            raise ProjectError(
                f"this project needs hecatron {needed} or newer (installed: {hecatron_version()})")
        self.local = _read_json(self.local_path, {})
        self._definitions: dict[str, dict] | None = None
        self._installed: dict[str, list[tuple[str, str]]] = {}

    # -- configuration ---------------------------------------------------

    @property
    def name(self) -> str:
        return self.config.get("project") or os.path.basename(self.root)

    @property
    def components(self) -> list[str]:
        return list(self.config.get("components") or ["."])

    @property
    def impl_path(self) -> str:
        if self.config.get("implPath"):
            return self.config["implPath"]
        return "{lang}" if self.components == ["."] else "{component}/{lang}"

    def impl_template(self, lang: str) -> str:
        """A language's implPath: the project's "implPaths" entry for it, else
        the shared one."""
        return (self.config.get("implPaths") or {}).get(lang) or self.impl_path

    def dir_names(self, lang: str) -> list[str]:
        """The names a language's directory goes by, its own first (C#'s
        dotnet/, Python's py/): the definition's "dirNames"."""
        names = self.definitions.get(lang, {}).get("dirNames", [])
        return [lang] + [name for name in names if name != lang]

    def _find_impl_dir(self, component: str, lang: str) -> tuple[str | None, str]:
        """(the directory found or None, the one to fall back to). A directory
        found by a language's name is its only when none of its "detectUnless"
        files sit there: a javascript/ holding a tsconfig.json is TypeScript's.
        A "detectYields" rule that holds there, as detection decides, gives
        the directory to the other language: a tsconfig.json beside mostly
        JavaScript is JavaScript's. A template without {lang} names its
        directory outright."""
        template = self.impl_template(lang).replace("{component}", component)
        candidates = [os.path.normpath(os.path.join(self.root, template.replace("{lang}", name)))
                      for name in self.dir_names(lang)]
        by_name = "{lang}" in template
        unless = self.definitions.get(lang, {}).get("detectUnless", []) if by_name else []

        def is_own(directory: str) -> bool:
            if not os.path.isdir(directory):
                return False
            listing = os.listdir(directory)
            if not by_name:
                return True
            if self._yields_in(lang, directory, listing):
                return False
            return not any(fnmatch.fnmatch(name, pattern) for name in listing for pattern in unless) or any(
                self._yields_in(other, directory, listing) for other, d in self.definitions.items()
                if (d.get("detectYields") or {}).get("to") == lang)

        return next((c for c in candidates if is_own(c)), None), candidates[0]

    def _yields_in(self, lang: str, directory: str, listing: list[str]) -> bool:
        """Whether one of the language's build files is in the directory and
        its "detectYields" rule holds there, as detection decides: the
        directory is the other language's."""
        return yields_in(self.definitions.get(lang, {}), directory, listing)

    def impl_dir(self, component: str, lang: str) -> str:
        """Where a component's code in a language lives: the template filled
        with the first of the language's directory names that is the
        language's, else with its own name."""
        found, fallback = self._find_impl_dir(component, lang)
        return found or fallback

    def impl_dirs(self, lang: str) -> list[tuple[str, str]]:
        """(component, directory) for each component implemented in `lang`."""
        return [(c, found) for c in self.components
                for found in [self._find_impl_dir(c, lang)[0]] if found]

    @property
    def languages(self) -> list[str]:
        return sorted(self.config.get("languages", []))

    @property
    def disabled(self) -> list[str]:
        return sorted(self.local.get("disabled", []))

    @property
    def tools(self) -> dict[str, dict]:
        return dict(self.config.get("tools", {}))

    @property
    def logs(self) -> str:
        if self.detected and "logs" not in self.config:
            return os.path.join(hecatron_dir_in_git(self.root), "logs")
        return os.path.join(self.root, self.config.get("logs", ".logs"))

    def tasks(self, action: str) -> dict[str, str]:
        """Project commands run with an action's parallel run (config.json "tasks")."""
        return dict((self.config.get("tasks") or {}).get(action) or {})

    def generated(self, key: str) -> str | None:
        return (self.config.get("generated") or {}).get(key)

    def save_config(self) -> None:
        """Write config.json; a detected project becomes one with its own, as
        hecatron init would make it."""
        self.config["languages"] = sorted(set(self.config.get("languages", [])))
        _write_json(self.config_path, self.config)
        if self.detected:
            ignore_project_files(self.root)

    def save_local(self) -> None:
        _write_json(self.local_path, self.local)

    # -- language definitions -------------------------------------------

    def _definition_dirs(self, lang: str) -> list[str]:
        """Where a language's files may live, project first (docs/guide/configuration.md)."""
        return [os.path.join(self.dir, "languages", lang), os.path.join(BUILTIN_LANGUAGES, lang)]

    @property
    def definitions(self) -> dict[str, dict]:
        """Every known language's config.json, built-in keys overlaid by the project's."""
        if self._definitions is None:
            names = set()
            for base in (BUILTIN_LANGUAGES, os.path.join(self.dir, "languages")):
                if os.path.isdir(base):
                    names.update(entry for entry in os.listdir(base)
                                 if os.path.isfile(os.path.join(base, entry, "config.json")))
            self._definitions = {}
            for lang in sorted(names):
                merged: dict = {}
                for directory in reversed(self._definition_dirs(lang)):
                    merged.update(_read_json(os.path.join(directory, "config.json"), {}))
                self._definitions[lang] = merged
        return self._definitions

    def resolve(self, lang: str, filename: str) -> str | None:
        """The file a language uses for `filename`: the project's, else the built-in one."""
        project_file = os.path.join(self.dir, "languages", lang, filename)
        if os.path.isfile(project_file):
            return project_file
        builtin_dir = os.path.join(BUILTIN_LANGUAGES, lang)
        builtin = os.path.join(builtin_dir, "hooks" if filename in HOOKS else "", filename)
        return builtin if os.path.isfile(builtin) else None

    # -- toolchain versions (docs/guide/versions.md) ---------------------------

    def _layer(self, lang: str, builtin: bool) -> dict:
        directory = BUILTIN_LANGUAGES if builtin else os.path.join(self.dir, "languages")
        return _read_json(os.path.join(directory, lang, "config.json"), {})

    def _pin_files(self, lang: str) -> list[dict]:
        """The definition's pin files, then any the project adds."""
        return list(self._layer(lang, True).get("pinFiles", [])) + list(self._layer(lang, False).get("pinFiles", []))

    def default_version(self, lang: str) -> str | None:
        builtin = self._layer(lang, True)
        if not builtin:
            return None
        version = builtin.get("version")
        if version is not None and not str(version).startswith("="):
            raise ProjectError(f"{lang}: the definition's default version {version!r} must be exact (\"={version}\")")
        return version

    def _project_version(self, lang: str) -> str | None:
        """The project's "version"; for a language only the project defines, its definition's."""
        return self._layer(lang, False).get("version")

    def _support_files(self, lang: str) -> list[dict]:
        """Files saying which versions a component supports, the newest of
        which it gets when the default is newer (docs/guide/versions.md)."""
        return list(self._layer(lang, True).get("supportFiles", [])) + list(self._layer(lang, False).get("supportFiles", []))

    def _range(self, lang: str, component: str) -> tuple[str | None, str | None]:
        """(range, path) from the nearest file in the definition's
        rangeFiles: the versions a component runs on (package.json's
        engines.node), else (None, None)."""
        entries = list(self._layer(lang, True).get("rangeFiles", [])) + list(self._layer(lang, False).get("rangeFiles", []))
        allowed, source, _ = self._pin_entry(lang, component, entries, "range")
        if not isinstance(allowed, str):
            return None, None
        return allowed, source[len("range:"):]

    def _in_range(self, lang: str, component: str | None, resolved: dict) -> tuple[str, str] | None:
        """For a component on the default with none of the default's line
        installed: the newest installed (version, home) in the range its
        rangeFiles give, which it runs instead (a package whose engines say
        >=22, on a machine with Node.js 26 and no 24)."""
        if resolved["source"] != "default" or resolved["spec"] is None:
            return None
        allowed, _ = self._range(lang, component or self._first_component(lang))
        if allowed is None or self._installed_match(lang, resolved["spec"], "default") is not None:
            return None
        return self._installed_where(lang, lambda version: versions.satisfies(allowed, version))

    def _pin(self, lang: str, component: str, entries: list[dict] | None = None, kind: str = "pin"):
        """(spec, source) from the nearest pin file (or other entries),
        searching from the component's implementation directory up to the
        project root."""
        spec, source, _ = self._pin_entry(lang, component, entries, kind)
        return spec, source

    def _pin_entry(self, lang: str, component: str, entries: list[dict] | None = None, kind: str = "pin"):
        """_pin's (spec, source), and the entry that gave them."""
        entries = self._pin_files(lang) if entries is None else entries
        if not entries:
            return versions.NOT_PINNED, None, None
        directory = self.impl_dir(component, lang)
        while True:
            for entry in entries:
                spec, path = self._read_entry(directory, entry)
                if spec is not versions.NOT_PINNED:
                    return spec, f"{kind}:" + os.path.relpath(path, self.root), entry
            if os.path.normpath(directory) == os.path.normpath(self.root) or len(directory) <= len(self.root):
                return versions.NOT_PINNED, None, None
            directory = os.path.dirname(directory)

    @staticmethod
    def _read_entry(directory: str, entry: dict):
        """(spec, path) for one pin or support file entry in a directory. An
        entry may name a pattern (".github/workflows/*.y*ml"): then the
        newest version any matching file gives, and that file."""
        found = []
        strip = entry.get("strip")
        if strip is not None:
            try:
                re.compile(strip)
            except (re.error, TypeError):
                raise ProjectError(f"pin file strip {strip!r} is not a regular expression") from None
        for path in sorted(glob.glob(os.path.join(directory, entry["file"]))):
            if os.path.isfile(path):
                with open(path, encoding="utf-8", errors="replace") as f:
                    spec = versions.strip_pin(versions.read_pin(entry["format"], f.read(), entry.get("key")), strip)
                if spec is not versions.NOT_PINNED:
                    found.append((spec, path))
        if not found:
            return versions.NOT_PINNED, None
        named = [(spec, path) for spec, path in found if spec is not None]
        if not named:
            return found[0]
        return max(named, key=lambda item: versions.sort_key(item[0].lstrip("=")))

    def resolve_version(self, lang: str, component: str | None = None) -> dict:
        """The version a component uses: its nearest pin file, else the
        project's "version", else the definition's default, or the newest
        line the component says it supports when the default is newer
        (Python's classifiers, CI workflows' setup steps, a Maven build's
        source level). In an old checkout, anything short of an exact version
        then becomes the release that was current at its last commit
        (as_of). spec is None when a pin file names no version ("stable"),
        or nothing gives one."""
        return self._resolve(lang, component)[0]

    def selects_version(self, lang: str, component: str | None = None) -> bool:
        """Whether the component's version is also the one that runs. Not
        when it comes from a support file marked "selects": false, which says
        what to install but not what runs: a workflow's dotnet-version, since
        .NET's dotnet runs the newest SDK it has unless a global.json picks
        one, on CI's runners too. Such a component runs what one on the
        default would (installed_version)."""
        return self._resolve(lang, component)[1]

    def _resolve(self, lang: str, component: str | None) -> tuple[dict, bool]:
        """(resolve_version's answer, selects_version's)."""
        component = component or self._first_component(lang)
        default = self.default_version(lang)
        selects = True
        spec, source = self._pin(lang, component)
        if isinstance(spec, str) and spec.startswith(versions.FLOOR):
            spec = self._above_floor(lang, component, spec[len(versions.FLOOR):], default)
        if spec is versions.NOT_PINNED:
            spec, source = self._project_version(lang), "project"
            if spec is None:
                spec, source = default, "default" if default is not None else None
                if default is not None:
                    supported, where, entry = self._pin_entry(lang, component, self._support_files(lang), "supports")
                    if supported not in (None, versions.NOT_PINNED) and self._older_line(lang, supported, default):
                        spec, source = self._installable(lang, supported), where
                        selects = entry.get("selects", True)
        if spec is not None:
            versions.Spec(spec)
            if source == "default" or not spec.startswith("="):
                spec, source = self._as_of_version(lang, spec, source)
        return {"spec": spec, "source": source, "exact": spec is not None and spec.startswith("=")}, selects

    def _above_floor(self, lang: str, component: str, floor: str, default: str | None) -> str:
        """The version for a pin that is a floor (global.json's latestMajor:
        8.0.100 or later): the default when it is that or later, else the
        newest release of the floor's major; raised to the newest major the
        component's floor files need (a project targeting net10.0)."""
        least = versions.Spec(floor).numbers
        spec = default if default is not None and versions.Spec(default).numbers >= least else str(least[0])
        needed = self._floor_files_major(lang, component)
        if needed is not None and needed > versions.Spec(spec).numbers[0]:
            spec = str(needed)
        return spec

    def _floor_files_major(self, lang: str, component: str) -> int | None:
        """The newest major the definition's floorFiles ask for, in the
        component's directory and up to three levels below it."""
        entries = list(self._layer(lang, True).get("floorFiles", [])) + list(self._layer(lang, False).get("floorFiles", []))
        if not entries:
            return None
        top = self.impl_dir(component, lang)
        majors = []
        for current, dirs, _ in os.walk(top):
            dirs[:] = sorted(d for d in dirs if not d.startswith(".") and d not in FLOOR_FILES_SKIPPED)
            if os.path.relpath(current, top).count(os.sep) >= 2 and current != top:
                dirs[:] = []
            for entry in entries:
                spec, _ = self._read_entry(current, entry)
                if isinstance(spec, str) and spec.startswith(versions.FLOOR):
                    majors.append(versions.Spec(spec[len(versions.FLOOR):]).numbers[0])
        return max(majors) if majors else None

    def _older_line(self, lang: str, supported: str, default: str) -> bool:
        """Whether the line a component supports up to (3.14) is older than
        the default's (=3.15.1). Compared by release line: a CI workflow
        naming Ruby 4.0.6 says nothing against the default 4.0.9."""
        parts = len(self._release_line(lang, default).split("."))
        line = versions.Spec(supported).numbers[:parts]
        return line < versions.Spec(default).numbers[:len(line)]

    def _oldest_installable(self, lang: str) -> str | None:
        return self._layer(lang, True).get("oldestInstallable")

    def _below_installable(self, lang: str, version: str) -> bool:
        oldest = self._oldest_installable(lang)
        if oldest is None:
            return False
        numbers = versions.Spec(oldest).numbers
        return versions.Spec(version.lstrip("=")).numbers[:len(numbers)] < numbers

    def _installable(self, lang: str, spec: str) -> str:
        """A supported line older than the installer can install (Java 8:
        Temurin's Java 8 releases aren't named like the rest) becomes the
        oldest it can."""
        return self._oldest_installable(lang) if self._below_installable(lang, spec) else spec

    def as_of(self) -> str | None:
        """The date an old checkout's toolchains should be current at: its
        last commit's, when that is over AS_OF_AFTER_DAYS old. Code from the
        last year built fine with today's releases when it was written, and
        older code may not: a newer compiler rejects what an older one
        accepted, or a release breaks a tool its lock file pins.
        HECATRON_AS_OF=YYYY-MM-DD sets the date, =off turns this off."""
        if not hasattr(self, "_as_of"):
            self._as_of = None
            wanted = os.environ.get("HECATRON_AS_OF", "")
            if wanted == "off":
                return None
            if wanted:
                try:
                    self._as_of = date.fromisoformat(wanted).isoformat()
                except ValueError:
                    raise ProjectError(f"HECATRON_AS_OF={wanted!r} isn't a date (YYYY-MM-DD) or off")
                return self._as_of
            try:
                committed = subprocess.run(["git", "log", "-1", "--format=%cs"], cwd=self.root, capture_output=True,
                                           text=True, check=True, stdin=subprocess.DEVNULL).stdout.strip()
                if date.today() - date.fromisoformat(committed) > timedelta(days=AS_OF_AFTER_DAYS):
                    self._as_of = committed
            except (OSError, ValueError, subprocess.CalledProcessError):
                pass
        return self._as_of

    def release_dates(self, lang: str) -> list[tuple[str, str, bool]]:
        """(version, date, lts) for each of a language's releases, from its
        definition's releases.json (or the language named by "releasesOf",
        whose toolchain it shares); empty without one."""
        owner = self._layer(lang, True).get("releasesOf", lang)
        data = _read_json(os.path.join(BUILTIN_LANGUAGES, owner, "releases.json"), {})
        return [(release[0], release[1], "lts" in release[2:]) for release in data.get("releases", [])]

    def _as_of_version(self, lang: str, spec: str, source: str):
        """(spec, source) as of the checkout's date (as_of): the newest
        release then that the spec matches, or, for the default, the newest
        on the definition's track (stable, or LTS). Unchanged when the
        checkout is recent, the language has no release dates, or nothing
        matched yet then."""
        when = self.as_of()
        releases = self.release_dates(lang) if when else []
        if not releases:
            return spec, source
        builtin = self._layer(lang, True)
        if source == "default":
            lts_only = builtin.get("track") == "lts"
            candidates = [v for v, released, lts in releases if released <= when and (lts or not lts_only)]
        else:
            wanted = versions.Spec(spec)
            candidates = [v for v, released, _ in releases if released <= when and wanted.matches(v)]
        picked = versions.newest(candidates)
        if picked is None:
            return spec, source
        note = ""
        if self._below_installable(lang, picked):
            if source == "project" or source.startswith("pin:"):
                return spec, source  # what the project chose stands, installable or not
            oldest = versions.Spec(self._oldest_installable(lang))
            picked = versions.newest([v for v, released, _ in releases if released <= when and oldest.matches(v)])
            note = f"; raised to {self._oldest_installable(lang)}, the oldest hecatron installs"
            if picked is None:
                return self._oldest_installable(lang), f"as-of:{when} ({source}{note})"
        # Where the installer takes a line, not a release (uv's Pythons, a
        # PHP package), the release's line: the newest patch of it.
        dated = self._release_line(lang, picked) if builtin.get("asOfPrecision") == "line" else "=" + picked
        return dated, f"as-of:{when} ({source}{note})"

    def _first_component(self, lang: str) -> str:
        dirs = self.impl_dirs(lang)
        return dirs[0][0] if dirs else self.components[0]

    def versions(self, lang: str) -> list[tuple[str | None, list[str]]]:
        """Each distinct spec the components resolve to, in component order, with its sources."""
        found: dict[str | None, list[str]] = {}
        components = [c for c, _ in self.impl_dirs(lang)] or [self.components[0]]
        for component in components:
            resolved = self.resolve_version(lang, component)
            sources = found.setdefault(resolved["spec"], [])
            if resolved["source"] and resolved["source"] not in sources:
                sources.append(resolved["source"])
        return list(found.items())

    def version_conflicts(self) -> list[tuple[str, str, str, str | None, str]]:
        """(lang, component, pin source, pin spec, project spec) wherever a pin
        file and the project's "version" both apply and disagree."""
        conflicts = []
        for lang in self.languages:
            wanted = self._layer(lang, False).get("version")
            if wanted is None or not self._layer(lang, True):
                continue
            for component, _ in self.impl_dirs(lang):
                spec, source = self._pin(lang, component)
                if spec is not versions.NOT_PINNED and spec != wanted:
                    conflicts.append((lang, component, source, spec, wanted))
        return conflicts

    def installed(self, lang: str) -> list[tuple[str, str]]:
        """(version, home) for each installed version, from the language's
        installed.sh; empty without one, or when it fails or takes over 30 s (a cold first run of several runtimes on macOS, which scans new binaries, can take seconds)."""
        if lang not in self._installed:
            self._installed[lang] = []
            script = self.resolve(lang, "installed.sh")
            if script:
                env = dict(os.environ, PATH=os.pathsep.join(self._static_path(lang) + [os.environ.get("PATH", "")]))
                try:
                    result = subprocess.run(["bash", script], capture_output=True, text=True,
                                            timeout=30, env=env, stdin=subprocess.DEVNULL, cwd=self.root)
                except subprocess.TimeoutExpired:
                    result = None
                if result is not None and result.returncode == 0:
                    for line in result.stdout.splitlines():
                        version, _, home = line.partition("\t")
                        if version and home and versions.is_version(version):
                            self._installed[lang].append((version.strip(), os.path.expanduser(home.strip())))
        return self._installed[lang]

    def _release_line(self, lang: str, default: str) -> str:
        """The default's release line, as a prefix: its first "releaseLine"
        parts (Node.js 24.22.1 is line 24 with 1), else all but the last
        (Go 1.27.3 is line 1.27)."""
        parts = default.lstrip("=").split(".")
        count = self._layer(lang, True).get("releaseLine", max(len(parts) - 1, 1))
        return ".".join(parts[:count])

    def installed_version(self, lang: str, component: str | None = None) -> tuple[str, str] | None:
        """The newest installed (version, home) matching the component's spec.
        A component on the default takes any version on the default's release
        line, so a machine that already has, say, Node.js 24.19 needs no
        second 24; a version the project chooses must match as written."""
        resolved, selects = self._resolve(lang, component)
        if not selects:
            default = self.default_version(lang)
            return self._installed_match(lang, default, "default") or self._installed_match(lang, "", "newest")
        return (self._installed_match(lang, resolved["spec"], resolved["source"])
                or self._in_range(lang, component, resolved))

    def _installed_match(self, lang: str, spec: str | None, source: str | None) -> tuple[str, str] | None:
        """The newest installed (version, home) a spec matches ("" matches
        any): installed_version's, and missing_versions'."""
        if spec is None:
            return None
        if source == "default":
            spec = self._release_line(lang, spec)
        wanted = versions.Spec(spec) if spec else None
        return self._installed_where(lang, lambda version: wanted is None or wanted.matches(version))

    def _installed_where(self, lang: str, wanted) -> tuple[str, str] | None:
        """The newest installed (version, home) whose version passes wanted."""
        matching = {v: home for v, home in self.installed(lang) if wanted(v)}
        best = versions.newest(list(matching))
        return (best, matching[best]) if best else None

    def missing_versions(self, lang: str) -> list[tuple[str, str, str]]:
        """(component, spec, source) for each component whose spec has no
        installed match. Empty without installed.sh: nothing to check against,
        and for a language the project doesn't use (Node.js's installer asks
        for both TypeScript's and JavaScript's)."""
        if lang not in self.languages or not self.resolve(lang, "installed.sh"):
            return []
        missing = []
        for component in [c for c, _ in self.impl_dirs(lang)] or [self.components[0]]:
            resolved = self.resolve_version(lang, component)
            if (resolved["spec"] is not None and self._installed_match(lang, resolved["spec"], resolved["source"]) is None
                    and self._in_range(lang, component, resolved) is None):
                missing.append((component, resolved["spec"], resolved["source"]))
        return missing

    def version_notes(self, lang: str) -> list[tuple[str, str]]:
        """(component, note) for each component whose version neither a pin
        nor the default chose: what it supports, or its commit's date. Worth
        saying, since nothing the project pinned chose it."""
        notes = []
        for component in [c for c, _ in self.impl_dirs(lang)] or [self.components[0]]:
            resolved, selects = self._resolve(lang, component)
            source, spec = resolved["source"] or "", (resolved["spec"] or "").lstrip("=")
            if not selects:
                where = source.rpartition("supports:")[2].rstrip(")")
                runs = self.installed_version(lang, component)
                pins = " or ".join(f"a {entry['file']}" for entry in self._pin_files(lang))
                notes.append((component, f"{spec} installed for {where}; runs the newest, "
                                         f"{runs[0] if runs else 'once installed'}" + (f", unless {pins} picks one" if pins else "")))
            elif source.startswith("supports:"):
                notes.append((component, f"{spec}, the newest {source[len('supports:'):]} supports"))
            elif runs := self._in_range(lang, component, resolved):
                allowed, where = self._range(lang, component)
                notes.append((component, f"{runs[0]} runs: {where} allows {allowed}, and the default {spec} isn't installed"))
            elif source.startswith("as-of:"):
                when, _, reason = source[len("as-of:"):].partition(" (")
                reason = reason[:-1]
                because = f" (the line from {reason[len('supports:'):]})" if reason.startswith("supports:") else (
                    f" ({reason})" if reason not in ("default", "project") else "")
                notes.append((component, f"{spec}, current on {when}, this commit's date{because}"))
        return notes

    def _static_path(self, lang: str) -> list[str]:
        """"path" entries that need no resolved version."""
        dirs = []
        for pattern in self.definitions.get(lang, {}).get("path", []):
            if "{" not in pattern:
                dirs += sorted(d for d in glob.glob(os.path.expanduser(pattern)) if os.path.isdir(d))
        return dirs

    def _others_static_path(self, lang: str, have: list[str]) -> list[str]:
        """The static "path" directories of the project's other added,
        enabled languages, in config order, leaving out any already in
        have: a build in one language often runs another's tool (a Ruby
        gem's Rakefile or an OCaml dune rule calling cargo), which that
        language's installer put in a directory only its own environment
        has."""
        seen, dirs, disabled = set(have), [], self.disabled
        for other in self.languages:
            if other == lang or other in disabled:
                continue
            for directory in self._static_path(other):
                if directory not in seen:
                    seen.add(directory)
                    dirs.append(directory)
        return dirs

    def language_env(self, lang: str, component: str | None = None) -> dict:
        """PATH entries (to prepend, in order) and variables for a language's
        work in a component. Entries and variables naming {home}, {version},
        {major} or {minor} need an installed version matching the spec;
        without one they're left out and the doctor reports it. On macOS, a
        definition's "sdkrootFromXcrun" adds SDKROOT, the selected Xcode's
        SDK, unless it's set: GNAT's gcc links against the Command Line
        Tools' SDK, which can be newer than the selected Xcode's ld knows
        (clang asks xcrun instead)."""
        definition = self.definitions.get(lang, {})
        match = self.installed_version(lang, component)
        fields = {}
        if match:
            version, home = match
            numbers = version.lstrip("v").split(".")
            fields = {"home": home, "version": version, "major": numbers[0],
                      "minor": numbers[1] if len(numbers) > 1 else "0"}
        expand = lambda text: text.format(**fields)  # noqa: E731
        path = []
        for pattern in definition.get("path", []):
            if "{" in pattern and not fields:
                continue
            path += sorted(d for d in glob.glob(os.path.expanduser(expand(pattern))) if os.path.isdir(d))
        path += self._others_static_path(lang, path)
        variables = {name: expand(value) for name, value in definition.get("env", {}).items()} if fields else {}
        if definition.get("sdkrootFromXcrun") and HOST_OS == "darwin" and "SDKROOT" not in os.environ:
            sdk = xcrun_sdk_path()
            if sdk:
                variables["SDKROOT"] = sdk
        return {"path": path, "vars": variables}

    def language_path(self, lang: str) -> list[str]:
        """PATH entries for a language's work outside any one component (doctor, install)."""
        return self.language_env(lang)["path"]

    def resolve_name(self, name: str) -> str | None:
        """A language's canonical name from its name or abbreviation."""
        name = name.lower()
        if name in self.definitions:
            return name
        for lang, data in self.definitions.items():
            if data.get("abbreviation") == name:
                return lang
        return None


# sys.platform, which the parity tests (internal/project) set to take
# macOS's path elsewhere.
HOST_OS = sys.platform

_xcrun_sdk: list[str | None] = []


def xcrun_sdk_path() -> str | None:
    """The selected Xcode's (or Command Line Tools') SDK, from
    `xcrun --show-sdk-path`, asked once per process: a definition's
    "sdkrootFromXcrun" sets SDKROOT to it. None when xcrun fails."""
    if not _xcrun_sdk:
        try:
            result = subprocess.run(["xcrun", "--show-sdk-path"], capture_output=True, text=True,
                                    timeout=30, stdin=subprocess.DEVNULL)
            _xcrun_sdk.append((result.stdout.strip() or None) if result.returncode == 0 else None)
        except (OSError, subprocess.TimeoutExpired):
            _xcrun_sdk.append(None)
    return _xcrun_sdk[0]


def load(start: str | None = None) -> Project:
    return Project(find_root(start))


# -- shell interface ------------------------------------------------------


def _emit_list(name: str, items: list[str]) -> str:
    return f"{name}={shlex.quote(chr(10).join(items))}"


def _emit_case(function: str, mapping: dict[str, str], passthrough: bool = False) -> list[str]:
    """A shell function mapping its argument; unknown values echo themselves or nothing."""
    lines = [f"{function}() {{", '    case "$1" in']
    for key, value in mapping.items():
        lines.append(f"        {shlex.quote(key)}) echo {shlex.quote(value)} ;;")
    lines.append('        *) echo "$1" ;;' if passthrough else "        *) echo '' ;;")
    lines += ["    esac", "}"]
    return lines


def shell_env(project: Project) -> str:
    """Shell variables and functions describing the project, for `eval`."""
    defs = project.definitions
    known = sorted(defs)
    added = [lang for lang in project.languages if lang in defs]
    has_hook = lambda lang, hook: project.resolve(lang, hook) is not None  # noqa: E731
    out = [
        f"export HECATRON_ROOT={shlex.quote(project.root)}",
        f"export HECATRON_PROJECT={shlex.quote(project.name)}",
        f"export HECATRON_COMPONENTS={shlex.quote(' '.join(project.components))}",
        f"export HECATRON_IMPL_PATH={shlex.quote(project.impl_path)}",
        f"export HECATRON_LOGS={shlex.quote(project.logs)}",
        f"export HECATRON_METADATA={shlex.quote(os.path.join(project.root, project.config['metadata']) if project.config.get('metadata') else '')}",
        f"export HECATRON_FORMAT_EXCLUDE={shlex.quote(chr(10).join(format_exclude(project.config)))}",
        _emit_list("_ALL_LANGS", known),
        _emit_list("_ADDED_LANGS", added),
        _emit_list("_COMPILED_LANGS", [lang for lang in added if has_hook(lang, "build.sh")]),
        _emit_list("_CLEAN_LANGS", [lang for lang in added if has_hook(lang, "clean.sh")]),
        _emit_list("_LINT_LANGS", [lang for lang in added if has_hook(lang, "lint.sh")]),
        _emit_list("_TEST_LANGS", [lang for lang in added if has_hook(lang, "test.sh")]),
        _emit_list("_AUDIT_LANGS", [lang for lang in added if has_hook(lang, "audit.sh")]),
        _emit_list("_OUTDATED_LANGS", [lang for lang in added if has_hook(lang, "outdated.sh")]),
        _emit_list("_DEPS_LANGS", [lang for lang in added if has_hook(lang, "deps.sh")]),
        _emit_list("_DOCS_LANGS", [lang for lang in added if has_hook(lang, "docs.sh")]),
        _emit_list("_BENCH_LANGS", [lang for lang in added if has_hook(lang, "bench.sh")]),
        _emit_list("_FORMAT_LANGS", [lang for lang in added if has_hook(lang, "format.sh")]),
        _emit_list("_LINT_FORMAT_LANGS", [lang for lang in added
                                          if has_hook(lang, "format.sh") and lint_checks_format(project.config, lang)]),
        _emit_list("_NO_BUILD_LANGS", [lang for lang in added if not has_hook(lang, "build.sh")]),
        _emit_list("_DISABLED_LANGS", project.disabled),
        _emit_list("_TOOLS", sorted(project.tools)),
    ]
    disabled = set(project.disabled)
    out += ["_is_disabled() {", '    case "$1" in']
    for lang in sorted(disabled):
        out.append(f"        {shlex.quote(lang)}) return 0 ;;")
    out += ["        *) return 1 ;;", "    esac", "}"]
    abbreviations = {d["abbreviation"]: lang for lang, d in defs.items() if d.get("abbreviation")}
    out += _emit_case("_name", abbreviations, passthrough=True)
    out += _emit_case("_abbr", {lang: d["abbreviation"] for lang, d in defs.items() if d.get("abbreviation")})
    deps = {lang: d.get("dependency", "") for lang, d in defs.items()}
    deps.update({tool: data.get("dependency", "") for tool, data in project.tools.items()})
    out += _emit_case("_dep", {k: v for k, v in deps.items() if v})
    out += _emit_case("_lint_dep", {lang: d["lintDependency"] for lang, d in defs.items() if d.get("lintDependency")})
    # Raw "path" patterns; _use_lang_path expands them when called, so a
    # directory an installer just created counts (doctor --fix re-checks).
    # Entries naming a version ({home}...) need project.py env.
    static = {lang: [p for p in d.get("path", []) if "{" not in p] for lang, d in defs.items()}
    out += _emit_case("_lang_path_patterns", {lang: " ".join(p) for lang, p in static.items() if p})
    return "\n".join(out) + "\n"


def lint_checks_format(config: dict, lang: str) -> bool:
    """Whether `hecatron lint` runs the language's format check: config.json's
    "lint": {"format": ...} is true (the default), false, or per language
    ({"haskell": false}, the rest true)."""
    lint = config.get("lint")
    setting = lint.get("format", True) if isinstance(lint, dict) else True
    if isinstance(setting, dict):
        return setting.get(lang, True) is not False
    return setting is not False


def lint_config_problems(config: dict) -> list[str]:
    """What's wrong with config.json's "lint" section, for hecatron check."""
    lint = config.get("lint")
    if lint is None:
        return []
    if not isinstance(lint, dict):
        return ['"lint" must be an object']
    setting = lint.get("format", True)
    if isinstance(setting, bool):
        return []
    if isinstance(setting, dict) and all(isinstance(v, bool) for v in setting.values()):
        return []
    return ['lint.format must be true, false, or an object of languages to true or false ({"haskell": false})']


def format_exclude(config: dict) -> list[str]:
    """Paths the format check and `hecatron format` leave alone: config.json's
    "format": {"exclude": [...]}, relative to the project root, a trailing
    "/" meaning everything below (a fixture misformatted on purpose)."""
    section = config.get("format")
    paths = section.get("exclude") if isinstance(section, dict) else None
    return [path for path in paths if isinstance(path, str)] if isinstance(paths, list) else []


def format_config_problems(config: dict) -> list[str]:
    """What's wrong with config.json's "format" section, for hecatron check."""
    section = config.get("format")
    if section is None:
        return []
    if not isinstance(section, dict):
        return ['"format" must be an object']
    paths = section.get("exclude", [])
    if not isinstance(paths, list) or not all(isinstance(path, str) and path for path in paths):
        return ['format.exclude must be a list of paths relative to the project root (["tests/fixtures/"])']
    return []


# The time limit `hecatron workflows` gives each generated CI job that sets
# none: GitHub's own is 6 hours, so one hung job holds every merge that long.
DEFAULT_TIMEOUT_MINUTES = 60


def _minutes(value) -> bool:
    return isinstance(value, int) and not isinstance(value, bool) and value >= 1


def ci_timeout_minutes(config: dict, lang: str) -> int:
    """A language's CI job time limit: config.json's "ci": {"languages":
    {lang: N}}, else "ci": {"timeoutMinutes": N}, else 60. A value that isn't
    a whole number of minutes is passed over (hecatron check reports it)."""
    ci = config.get("ci")
    ci = ci if isinstance(ci, dict) else {}
    languages = ci.get("languages")
    languages = languages if isinstance(languages, dict) else {}
    for value in (languages.get(lang), ci.get("timeoutMinutes")):
        if _minutes(value):
            return value
    return DEFAULT_TIMEOUT_MINUTES


# The "ci" section's settings: the time limits, which these scripts write,
# and cache and image, only the Go core's (manage.py's go_only_settings).
CI_KEYS = ("cache", "image", "secrets", "timeoutMinutes", "languages")


def ci_config_problems(config: dict) -> list[str]:
    """What's wrong with config.json's "ci" time limits and unknown keys, for
    hecatron check, as the Go core says it (internal/project/ci.go)."""
    ci = config.get("ci")
    if ci is None:
        return []
    if not isinstance(ci, dict):
        return ['"ci" must be an object']
    problems = []
    for key, value in ci.items():
        if key == "timeoutMinutes" and not _minutes(value):
            problems.append("ci.timeoutMinutes must be a whole number of minutes, 1 or more")
        elif key == "languages" and value is not None:
            if not isinstance(value, dict):
                problems.append('ci.languages must be an object of languages to minutes ({"pascal": 120})')
            else:
                problems += [f"ci.languages.{lang} must be a whole number of minutes, 1 or more"
                             for lang, minutes in value.items() if not _minutes(minutes)]
        elif key not in CI_KEYS:
            problems.append(f"ci.{key} is not a setting ({', '.join(CI_KEYS)})")
    return problems


def env_exports(environment: dict) -> str:
    """Shell exports for a language environment from Project.language_env."""
    out = [f"export {name}={shlex.quote(value)}" for name, value in environment["vars"].items()]
    if environment["path"]:
        out.append(f'export PATH={shlex.quote(os.pathsep.join(environment["path"]))}:"${{PATH}}"')
    return "".join(line + "\n" for line in out)


def main(argv: list[str]) -> int:
    if not argv:
        print(__doc__, file=sys.stderr)
        return 2
    query, args = argv[0], argv[1:]
    try:
        return _query(load(), query, args)
    except (ProjectError, ValueError) as e:
        print(f"hecatron: {e}", file=sys.stderr)
        return 1


def _query(project: Project, query: str, args: list[str]) -> int:
    if query == "root":
        print(project.root)
    elif query == "shell-env":
        sys.stdout.write(shell_env(project))
    elif query == "resolve" and len(args) == 2:
        path = project.resolve(args[0], args[1])
        if path is None:
            return 1
        print(path)
    elif query == "tasks" and len(args) == 1:
        for name in project.tasks(args[0]):
            print(name)
    elif query == "task" and len(args) == 2:
        command = project.tasks(args[0]).get(args[1])
        if command is None:
            return 1
        print(command)
    elif query == "version" and len(args) in (1, 2):
        resolved = project.resolve_version(*args)
        match = project.installed_version(*args)
        print("\t".join([resolved["spec"] or "-", resolved["source"] or "-", *(match or ("-", "-"))]))
    elif query == "missing-versions" and len(args) == 1:
        for component, spec, source in project.missing_versions(args[0]):
            print(f"{component}\t{spec}\t{source}")
    elif query == "version-notes" and len(args) == 1:
        for component, note in project.version_notes(args[0]):
            print(f"{component}\t{note}")
    elif query == "versions" and len(args) == 1:
        for spec, sources in project.versions(args[0]):
            print(f"{spec or '-'}\t{','.join(sources) or '-'}")
    elif query == "env" and len(args) in (1, 2):
        sys.stdout.write(env_exports(project.language_env(*args)))
    elif query == "impl-dirs" and len(args) == 1:
        for component, directory in project.impl_dirs(args[0]):
            print(f"{component}\t{directory}")
    else:
        print(__doc__, file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
