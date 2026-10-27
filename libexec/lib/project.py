#!/usr/bin/env python3
"""The hecatron project model: configuration, languages and file resolution.

A project is a directory with `.hecatron/config.json` (docs/design.md §6).
Everything that needs to know about the project reads it through this module,
from Python (`import project`) or from shell (`python3 project.py <query>`).

Shell queries:
    root                        print the project root
    shell-env                   print shell variables and functions for eval
    resolve <lang> <file>       print the path a language's file resolves to
    impl-dirs <lang>            print each component's directory for a language
    version <lang> [component]  print a language's version spec, its source and
                                the installed match (docs/design.md §18)
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
import glob
import json
import os
import shlex
import subprocess
import sys
from datetime import date, timedelta

import versions

SCHEMA = 1
# How old a checkout's last commit is before its toolchains are the releases
# current then (Project.as_of).
AS_OF_AFTER_DAYS = 365
HECATRON_HOME = os.environ.get(
    "HECATRON_HOME", os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
# HECATRON_BUILTIN_LANGUAGES points the tests at fixture definitions.
BUILTIN_LANGUAGES = os.environ.get("HECATRON_BUILTIN_LANGUAGES") or os.path.join(HECATRON_HOME, "languages")
# Built-in default hooks live in languages/<lang>/hooks/ (docs/design.md §9).
HOOKS = ("test.sh", "lint.sh", "build.sh", "clean.sh", "audit.sh", "format.sh", "deps.sh")


class ProjectError(Exception):
    """A project that can't be found or read."""


def find_root(start: str | None = None) -> str:
    """The nearest directory at or above `start` holding .hecatron/config.json."""
    directory = os.path.abspath(start or os.getcwd())
    while True:
        if os.path.isfile(os.path.join(directory, ".hecatron", "config.json")):
            return directory
        parent = os.path.dirname(directory)
        if parent == directory:
            raise ProjectError(
                "not a hecatron project (no .hecatron/config.json here or above); "
                "run 'hecatron init' to set one up")
        directory = parent


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
        A template without {lang} names its directory outright."""
        template = self.impl_template(lang).replace("{component}", component)
        candidates = [os.path.normpath(os.path.join(self.root, template.replace("{lang}", name)))
                      for name in self.dir_names(lang)]
        unless = self.definitions.get(lang, {}).get("detectUnless", []) if "{lang}" in template else []

        def is_own(directory: str) -> bool:
            return os.path.isdir(directory) and not any(
                fnmatch.fnmatch(name, pattern) for name in os.listdir(directory) for pattern in unless)

        return next((c for c in candidates if is_own(c)), None), candidates[0]

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
        return os.path.join(self.root, self.config.get("logs", ".logs"))

    def tasks(self, action: str) -> dict[str, str]:
        """Project commands run with an action's parallel run (config.json "tasks")."""
        return dict((self.config.get("tasks") or {}).get(action) or {})

    def generated(self, key: str) -> str | None:
        return (self.config.get("generated") or {}).get(key)

    def save_config(self) -> None:
        self.config["languages"] = sorted(set(self.config.get("languages", [])))
        _write_json(self.config_path, self.config)

    def save_local(self) -> None:
        _write_json(self.local_path, self.local)

    # -- language definitions -------------------------------------------

    def _definition_dirs(self, lang: str) -> list[str]:
        """Where a language's files may live, project first (docs/design.md §7)."""
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

    # -- toolchain versions (docs/design.md §18) ---------------------------

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
        which it gets when the default is newer (docs/design.md §18)."""
        return list(self._layer(lang, True).get("supportFiles", [])) + list(self._layer(lang, False).get("supportFiles", []))

    def _pin(self, lang: str, component: str, entries: list[dict] | None = None, kind: str = "pin"):
        """(spec, source) from the nearest pin file (or other entries),
        searching from the component's implementation directory up to the
        project root."""
        entries = self._pin_files(lang) if entries is None else entries
        if not entries:
            return versions.NOT_PINNED, None
        directory = self.impl_dir(component, lang)
        while True:
            for entry in entries:
                spec, path = self._read_entry(directory, entry)
                if spec is not versions.NOT_PINNED:
                    return spec, f"{kind}:" + os.path.relpath(path, self.root)
            if os.path.normpath(directory) == os.path.normpath(self.root) or len(directory) <= len(self.root):
                return versions.NOT_PINNED, None
            directory = os.path.dirname(directory)

    @staticmethod
    def _read_entry(directory: str, entry: dict):
        """(spec, path) for one pin or support file entry in a directory. An
        entry may name a pattern (".github/workflows/*.y*ml"): then the
        newest version any matching file gives, and that file."""
        found = []
        for path in sorted(glob.glob(os.path.join(directory, entry["file"]))):
            if os.path.isfile(path):
                with open(path, encoding="utf-8", errors="replace") as f:
                    spec = versions.read_pin(entry["format"], f.read(), entry.get("key"))
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
        component = component or self._first_component(lang)
        default = self.default_version(lang)
        spec, source = self._pin(lang, component)
        if spec is versions.NOT_PINNED:
            spec, source = self._project_version(lang), "project"
            if spec is None:
                spec, source = default, "default" if default is not None else None
                if default is not None:
                    supported, where = self._pin(lang, component, self._support_files(lang), "supports")
                    if supported not in (None, versions.NOT_PINNED) and self._older_line(lang, supported, default):
                        spec, source = self._installable(lang, supported), where
        if spec is not None:
            versions.Spec(spec)
            if source == "default" or not spec.startswith("="):
                spec, source = self._as_of_version(lang, spec, source)
        return {"spec": spec, "source": source, "exact": spec is not None and spec.startswith("=")}

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
        resolved = self.resolve_version(lang, component)
        spec = resolved["spec"]
        if spec is None:
            return None
        if resolved["source"] == "default":
            spec = self._release_line(lang, spec)
        wanted = versions.Spec(spec)
        matching = {v: home for v, home in self.installed(lang) if wanted.matches(v)}
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
            if resolved["spec"] is not None and self.installed_version(lang, component) is None:
                missing.append((component, resolved["spec"], resolved["source"]))
        return missing

    def version_notes(self, lang: str) -> list[tuple[str, str]]:
        """(component, note) for each component whose version neither a pin
        nor the default chose: what it supports, or its commit's date. Worth
        saying, since nothing the project pinned chose it."""
        notes = []
        for component in [c for c, _ in self.impl_dirs(lang)] or [self.components[0]]:
            resolved = self.resolve_version(lang, component)
            source, spec = resolved["source"] or "", (resolved["spec"] or "").lstrip("=")
            if source.startswith("supports:"):
                notes.append((component, f"{spec}, the newest {source[len('supports:'):]} supports"))
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

    def language_env(self, lang: str, component: str | None = None) -> dict:
        """PATH entries (to prepend, in order) and variables for a language's
        work in a component. Entries and variables naming {home}, {version},
        {major} or {minor} need an installed version matching the spec;
        without one they're left out and the doctor reports it."""
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
        variables = {name: expand(value) for name, value in definition.get("env", {}).items()} if fields else {}
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
        _emit_list("_ALL_LANGS", known),
        _emit_list("_ADDED_LANGS", added),
        _emit_list("_COMPILED_LANGS", [lang for lang in added if has_hook(lang, "build.sh")]),
        _emit_list("_CLEAN_LANGS", [lang for lang in added if has_hook(lang, "clean.sh")]),
        _emit_list("_LINT_LANGS", [lang for lang in added if has_hook(lang, "lint.sh")]),
        _emit_list("_TEST_LANGS", [lang for lang in added if has_hook(lang, "test.sh")]),
        _emit_list("_AUDIT_LANGS", [lang for lang in added if has_hook(lang, "audit.sh")]),
        _emit_list("_DEPS_LANGS", [lang for lang in added if has_hook(lang, "deps.sh")]),
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
