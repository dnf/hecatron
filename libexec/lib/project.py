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
    tasks <action>              print the project's extra tasks for an action
    task <action> <name>        print one task's command
"""

from __future__ import annotations

import glob
import json
import os
import shlex
import subprocess
import sys

import versions

SCHEMA = 1
HECATRON_HOME = os.environ.get(
    "HECATRON_HOME", os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
# HECATRON_BUILTIN_LANGUAGES points the tests at fixture definitions.
BUILTIN_LANGUAGES = os.environ.get("HECATRON_BUILTIN_LANGUAGES") or os.path.join(HECATRON_HOME, "languages")
# Built-in default hooks live in languages/<lang>/hooks/ (docs/design.md §9).
HOOKS = ("test.sh", "lint.sh", "build.sh", "clean.sh")


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

    def impl_dir(self, component: str, lang: str) -> str:
        relative = self.impl_path.replace("{component}", component).replace("{lang}", lang)
        return os.path.normpath(os.path.join(self.root, relative))

    def impl_dirs(self, lang: str) -> list[tuple[str, str]]:
        """(component, directory) for each component implemented in `lang`."""
        return [(c, self.impl_dir(c, lang)) for c in self.components
                if os.path.isdir(self.impl_dir(c, lang))]

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

    def _default_version(self, lang: str) -> str | None:
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

    def _pin(self, lang: str, component: str):
        """(spec, source) from the nearest pin file, searching from the
        component's implementation directory up to the project root."""
        entries = self._pin_files(lang)
        if not entries:
            return versions.NOT_PINNED, None
        directory = self.impl_dir(component, lang)
        while True:
            for entry in entries:
                path = os.path.join(directory, entry["file"])
                if os.path.isfile(path):
                    with open(path, encoding="utf-8") as f:
                        spec = versions.read_pin(entry["format"], f.read(), entry.get("key"))
                    if spec is not versions.NOT_PINNED:
                        return spec, "pin:" + os.path.relpath(path, self.root)
            if os.path.normpath(directory) == os.path.normpath(self.root) or len(directory) <= len(self.root):
                return versions.NOT_PINNED, None
            directory = os.path.dirname(directory)

    def resolve_version(self, lang: str, component: str | None = None) -> dict:
        """The version a component uses: its nearest pin file, else the
        project's "version", else the definition's default. spec is None
        when a pin file names no version ("stable"), or nothing gives one."""
        component = component or self._first_component(lang)
        default = self._default_version(lang)
        spec, source = self._pin(lang, component)
        if spec is versions.NOT_PINNED:
            spec, source = self._project_version(lang), "project"
            if spec is None:
                spec, source = default, "default" if default is not None else None
        if spec is not None:
            versions.Spec(spec)
        return {"spec": spec, "source": source, "exact": spec is not None and spec.startswith("=")}

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

    def installed_version(self, lang: str, component: str | None = None) -> tuple[str, str] | None:
        """The newest installed (version, home) matching the component's spec."""
        spec = self.resolve_version(lang, component)["spec"]
        if spec is None:
            return None
        wanted = versions.Spec(spec)
        matching = {v: home for v, home in self.installed(lang) if wanted.matches(v)}
        best = versions.newest(list(matching))
        return (best, matching[best]) if best else None

    def missing_versions(self, lang: str) -> list[tuple[str, str, str]]:
        """(component, spec, source) for each component whose spec has no
        installed match. Empty without installed.sh: nothing to check against."""
        if not self.resolve(lang, "installed.sh"):
            return []
        missing = []
        for component in [c for c, _ in self.impl_dirs(lang)] or [self.components[0]]:
            resolved = self.resolve_version(lang, component)
            if resolved["spec"] is not None and self.installed_version(lang, component) is None:
                missing.append((component, resolved["spec"], resolved["source"]))
        return missing

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
