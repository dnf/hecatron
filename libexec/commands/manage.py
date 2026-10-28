#!/usr/bin/env python3
"""hecatron's configuration-centred commands.

    init [dir] [--components a,b] [--impl-path TEMPLATE] [--languages a,b] [--justfile] [--force]
    install [lang...] | uninstall lang...
    list [languages] [--all]
    add lang... | remove lang... | enable [lang...|--all] | disable [lang...]
    dockerfile [--check] | workflows [--check] | check
    banner --generator PATH [--source PATH] [--regenerate CMD] [--prefix P] [--suffix S]
"""

from __future__ import annotations

import argparse
import fnmatch
import json
import os
import subprocess
import sys
import tempfile

HOME = os.environ.get("HECATRON_HOME", os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
sys.path.insert(0, os.path.join(HOME, "libexec", "lib"))
sys.path.insert(0, os.path.join(HOME, "libexec", "codegen"))

import project as projectlib  # noqa: E402
from banner import make_banner  # noqa: E402
import dockerfile as dockerfilelib  # noqa: E402
import audit as auditlib  # noqa: E402


def die(message: str, status: int = 1) -> None:
    print(f"hecatron: {message}", file=sys.stderr)
    sys.exit(status)


def load_project() -> projectlib.Project:
    try:
        return projectlib.load()
    except projectlib.ProjectError as e:
        die(str(e))
        raise  # unreachable


def resolve_all(project: projectlib.Project, names: list[str], *, added_only: bool) -> list[str]:
    resolved = []
    for name in names:
        lang = project.resolve_name(name)
        if lang is None:
            die(f"unknown language: {name} (try 'hecatron list --all')")
        if added_only and lang not in project.languages:
            die(f"{lang} is not added; run 'hecatron add {lang}' first")
        resolved.append(lang)
    return resolved


def hook_env(project: projectlib.Project) -> dict[str, str]:
    """The environment installers and hooks see (docs/design.md §9)."""
    env = dict(os.environ)
    env.update({
        "HECATRON_HOME": HOME,
        "HECATRON_ROOT": project.root,
        "HECATRON_PROJECT": project.name,
        "HECATRON_COMPONENTS": " ".join(project.components),
        "HECATRON_IMPL_PATH": project.impl_path,
    })
    return env


# -- install / uninstall ------------------------------------------------------


def run_definition_script(project, lang: str, script: str, extra_env: dict[str, str] | None = None) -> int:
    path = project.resolve(lang, script)
    if path is None:
        print(f"{lang}: no {script}", file=sys.stderr)
        return 1
    env = hook_env(project)
    env.update(extra_env or {})
    environment = project.language_env(lang)
    env.update(environment["vars"])
    env["PATH"] = os.pathsep.join(environment["path"] + [env.get("PATH", "")])
    return subprocess.run(["bash", path], env=env, cwd=project.root).returncode


def cmd_install(args) -> int:
    """Installing everything fetches dependencies after every toolchain is
    in, since one language's can need another's toolchain (cucumber/messages'
    npm ci runs a generator needing Ruby): each installer lists the languages
    it would fetch them for in the HECATRON_DEFER_DEPS file. Installing named
    languages fetches theirs as each installs."""
    if args.plan:
        needs_binary("install --plan")
    project = load_project()
    install_all = not args.languages
    langs = project.languages if install_all else resolve_all(project, args.languages, added_only=True)
    with tempfile.NamedTemporaryFile("r", prefix="hecatron-deps-", suffix=".txt") as deferred:
        failed = []
        for lang in langs:
            env = {"HECATRON_DEFER_DEPS": deferred.name} if install_all else None
            if run_definition_script(project, lang, "install.sh", env) != 0:
                if not install_all:
                    return 1
                failed.append(lang)
        deps_langs = list(dict.fromkeys(deferred.read().split()))
    deps_failed = [lang for lang in deps_langs if lang not in failed and
                   subprocess.run(["bash", os.path.join(HOME, "bin", "hecatron"), "deps", lang],
                                  cwd=project.root).returncode != 0]
    if failed:
        print(f"\nFailed to install: {' '.join(failed)}", file=sys.stderr)
    if deps_failed:
        print(f"{'' if failed else chr(10)}Failed to fetch dependencies: {' '.join(deps_failed)}"
              f" (retry with: hct deps {' '.join(deps_failed)})", file=sys.stderr)
    return 1 if failed or deps_failed else 0


def cmd_uninstall(args) -> int:
    project = load_project()
    if not args.languages:
        die("usage: hecatron uninstall <lang>...", 2)
    for lang in resolve_all(project, args.languages, added_only=True):
        status = run_definition_script(project, lang, "uninstall.sh")
        if status != 0:
            return status
    return 0


# -- list / add / remove / enable / disable ----------------------------------


def cmd_list(args) -> int:
    project = load_project()
    added = set(project.languages)
    if args.all:
        for lang in sorted(project.definitions):
            origin = "project" if os.path.isdir(os.path.join(project.dir, "languages", lang)) else "built-in"
            status = "" if lang in added else "  (not added)"
            print(f"  {lang:<12} {origin}{status}")
        print(f"\n{len(project.definitions)} languages known, {len(added)} added")
    else:
        for lang in project.languages:
            print(f"  {lang}{'  (disabled)' if lang in project.disabled else ''}")
        print(f"\n{len(project.languages)} languages")
    return 0


def cmd_add(args) -> int:
    project = load_project()
    langs = resolve_all(project, args.languages, added_only=False)
    new = [lang for lang in langs if lang not in project.languages]
    for lang in langs:
        if lang not in new:
            print(f"Already added: {lang}")
    if new:
        project.config["languages"] = project.languages + new
        project.save_config()
        for lang in new:
            print(f"Added: {lang}")
        regenerate_after_change(project)
    return 0


def cmd_remove(args) -> int:
    project = load_project()
    langs = resolve_all(project, args.languages, added_only=False)
    gone = [lang for lang in langs if lang in project.languages]
    for lang in langs:
        if lang not in gone:
            print(f"Not added: {lang}")
    if gone:
        project.config["languages"] = [lang for lang in project.languages if lang not in gone]
        project.save_config()
        for lang in gone:
            print(f"Removed: {lang}")
            path = workflow_path(project, lang)
            if path and os.path.isfile(path):
                os.remove(path)
        regenerate_after_change(project)
    return 0


def regenerate_after_change(project) -> None:
    if workflow_template(project):
        write_workflows(project, check=False)
    if project.generated("dockerfile"):
        write_dockerfile(project, check=False)


def cmd_enable(args) -> int:
    project = load_project()
    disabled = project.disabled
    if not args.languages and not args.all:
        if disabled:
            print("Disabled for you: " + ", ".join(disabled))
            print("Use 'hecatron enable <lang>' or 'hecatron enable --all' to re-enable.")
        else:
            print("No languages are disabled.")
        return 0
    if args.all:
        remaining = []
    else:
        names = set(resolve_all(project, args.languages, added_only=False))
        remaining = [lang for lang in disabled if lang not in names]
    project.local["disabled"] = remaining
    project.save_local()
    print(f"Still disabled: {', '.join(remaining)}" if remaining else "All languages enabled.")
    return 0


def cmd_disable(args) -> int:
    project = load_project()
    if not args.languages:
        print("Disabled for you: " + (", ".join(project.disabled) or "none"))
        return 0
    names = resolve_all(project, args.languages, added_only=True)
    project.local["disabled"] = sorted(set(project.disabled) | set(names))
    project.save_local()
    print(f"Disabled (for you, in parallel runs): {', '.join(project.local['disabled'])}")
    return 0


# -- generated files ----------------------------------------------------------


def write_if_changed(path: str, content: str, *, check: bool, label: str) -> bool:
    """Write `content` to `path`, or with check only report whether it's current."""
    current = None
    if os.path.isfile(path):
        with open(path, encoding="utf-8") as f:
            current = f.read()
    if current == content:
        return True
    if check:
        print(f"out of date: {label}")
        return False
    os.makedirs(os.path.dirname(path) or ".", exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(content)
    print(f"wrote {label}")
    return True


def write_dockerfile(project, *, check: bool) -> bool:
    relative = project.generated("dockerfile")
    if not relative:
        if check:
            return True
        die("no Dockerfile configured (config.json generated.dockerfile)")
    try:
        content = dockerfilelib.generate(project)
    except (projectlib.ProjectError, ValueError) as e:
        die(str(e))
    return write_if_changed(os.path.join(project.root, relative), content, check=check, label=relative)


def cmd_dockerfile(args) -> int:
    ok = write_dockerfile(load_project(), check=args.check)
    if args.check and ok:
        print("Dockerfile is up to date.")
    return 0 if ok else 1


def workflow_template(project) -> str | None:
    return project.generated("workflows")


def workflow_path(project, lang: str) -> str | None:
    template = workflow_template(project)
    return os.path.join(project.root, template.replace("{lang}", lang)) if template else None


def workflow_content(project, lang: str) -> str | None:
    source = project.resolve(lang, "workflow.yml")
    if source is None:
        return None
    with open(source, encoding="utf-8") as f:
        body = f.read()
    banner = make_banner([
        f"Source: {os.path.relpath(source, project.root)}",
        "Generated by: hecatron workflows",
        "",
        "To update: edit the source, then run hecatron workflows",
        "(hecatron add and remove regenerate it too)",
    ])
    return banner + with_timeouts(body, projectlib.ci_timeout_minutes(project.config, lang))


def _yaml_content(line: str) -> bool:
    text = line.strip(" \t\r")
    return bool(text) and not text.startswith("#")


def _yaml_indent(line: str) -> int:
    return len(line) - len(line.lstrip(" "))


def with_timeouts(body: str, minutes: int) -> str:
    """The workflow with `timeout-minutes: <minutes>` after the runs-on of
    each job in its top-level jobs: that sets no timeout-minutes. A job
    without runs-on (one calling a reusable workflow) can't have one. The
    text is otherwise left as written."""
    lines = body.split("\n")
    start = next((i for i, line in enumerate(lines) if line.rstrip(" \t\r") == "jobs:"), None)
    if start is None:
        return body
    inserts: list[tuple[int, int]] = []
    job_indent = None
    job: dict = {}

    def finish(job: dict) -> None:
        if job.get("runs_on") is not None and not job.get("timeout"):
            inserts.append((job["runs_on"], job["indent"]))

    for i in range(start + 1, len(lines)):
        line = lines[i]
        if not _yaml_content(line):
            continue
        indent = _yaml_indent(line)
        if indent == 0:
            break
        if job_indent is None:
            job_indent = indent
        if indent <= job_indent:
            finish(job)
            job = {}
            continue
        job.setdefault("indent", indent)
        if indent != job["indent"]:
            continue
        key = line.strip(" \t\r").split(":", 1)[0]
        if key == "timeout-minutes":
            job["timeout"] = True
        elif key == "runs-on":
            end = i
            for j in range(i + 1, len(lines)):
                if not _yaml_content(lines[j]):
                    continue
                if _yaml_indent(lines[j]) <= indent:
                    break
                end = j
            job["runs_on"] = end
    finish(job)
    for index, indent in reversed(inserts):
        lines.insert(index + 1, " " * indent + f"timeout-minutes: {minutes}")
    return "\n".join(lines)


# Settings for generated files only the Go core writes (docs/design.md §11).
GO_ONLY_GENERATED = ("ci", "dependabot", "renovate")
GO_ONLY_SECTIONS = ("ci", "dependabot", "renovate")


# The "ci" settings only the Go core reads; these scripts write the time
# limits (timeoutMinutes, languages) themselves.
GO_ONLY_CI_KEYS = ("cache", "image")


def go_only_settings(project) -> list[str]:
    generated = project.config.get("generated") or {}
    ci = project.config.get("ci")
    return ([f"generated.{key}" for key in GO_ONLY_GENERATED if key in generated]
            + [key for key in GO_ONLY_SECTIONS if key in project.config
               and (key != "ci" or (isinstance(ci, dict) and any(k in ci for k in GO_ONLY_CI_KEYS)))])


def needs_binary(what: str) -> None:
    die(f"{what} needs hecatron's binary, which install.sh downloads with a release")


def write_workflows(project, *, check: bool) -> bool:
    if settings := go_only_settings(project):
        needs_binary(f"config.json's {', '.join(settings)}")
    if not workflow_template(project):
        if check:
            return True
        die("no workflow path configured (config.json generated.workflows)")
    ok = True
    for lang in project.languages:
        content = workflow_content(project, lang)
        if content is None:
            continue
        path = workflow_path(project, lang)
        ok &= write_if_changed(path, content, check=check, label=os.path.relpath(path, project.root))
    return ok


def cmd_workflows(args) -> int:
    if getattr(args, "ci", None) is not None:
        needs_binary("hecatron workflows --ci")
    project = load_project()
    if not go_only_settings(project) and (problems := projectlib.ci_config_problems(project.config)):
        die(".hecatron/config.json: " + "; ".join(problems))
    ok = write_workflows(project, check=args.check)
    if args.check and ok:
        print("Workflows are up to date.")
    return 0 if ok else 1


def workflow_coverage_problems(project) -> list[str]:
    """Each language's workflow must test every component it implements."""
    if project.components == ["."]:
        return []
    problems = []
    for lang in project.languages:
        source = project.resolve(lang, "workflow.yml")
        if source is None:
            continue
        with open(source, encoding="utf-8") as f:
            text = f.read()
        # A matrix over components (`${{ matrix.component }}` in the path) covers them all.
        matrix_path = project.impl_template(lang).replace("{component}", "${{ matrix.component }}").replace("{lang}", lang)
        if matrix_path in text and all(c in text for c in project.components):
            continue
        for component, directory in project.impl_dirs(lang):
            relative = os.path.relpath(directory, project.root)
            if relative not in text:
                problems.append(f"{lang}: workflow doesn't test {component} ({relative})")
    return problems


def cmd_check(args) -> int:
    project = load_project()
    ok = write_dockerfile(project, check=True)
    ok &= write_workflows(project, check=True)
    for problem in workflow_coverage_problems(project):
        print(problem)
        ok = False
    for lang, component, source, pinned, wanted in project.version_conflicts():
        print(f"{lang} in {component}: {source[len('pin:'):]} pins {pinned or 'no version'}, "
              f"but the project's version is {wanted} (the pin file wins; make them agree)")
        ok = False
    if ok:
        print("Generated files are up to date and every workflow covers every component.")
    else:
        print("Run 'hecatron dockerfile' / 'hecatron workflows' to regenerate.", file=sys.stderr)
    config_problems = (auditlib.config_problems(project.config) + projectlib.lint_config_problems(project.config)
                       + projectlib.format_config_problems(project.config) + projectlib.ci_config_problems(project.config))
    for problem in config_problems:
        print(f".hecatron/config.json: {problem}")
    return 0 if ok and not config_problems else 1


def cmd_banner(args) -> int:
    rows = ["This file is auto-generated by:", f"  {args.generator}", ""]
    rows.append(f"Source: {args.source}" if args.source else "Changes will be overwritten when it regenerates.")
    rows.append(f"To regenerate: {args.regenerate}" if args.regenerate else "Edit the generator or its input instead.")
    sys.stdout.write(make_banner(rows, line_prefix=args.prefix, line_suffix=args.suffix))
    return 0


# -- init -----------------------------------------------------------------------


# Trees holding code that isn't the project's own implementation: vendored
# dependencies, samples, tests, docs and tooling (hooks/: git hooks, build
# hooks). A language found only there isn't added, and no language's
# directory is placed there.
SKIPPED_DIRS = {"android", "benchmarks", "buildSrc", "demo", "demos", "doc", "docs", "example", "examples",
                "external", "fixtures", "hooks", "node_modules", "playground", "sample", "samples", "templates", "test",
                "test_data", "testdata", "tests", "tests_data", "third_party", "tools", "tutorial", "tutorials",
                "vendor", "vendored"}

# Where projects keep per-language code; when layouts disagree, one under
# these (or at the root) beats one covering more languages (Beam's sdks/{lang}
# over learning/katas/{lang}).
PREFERRED_PARENTS = {"bindings", "ffi", "lang", "lib", "packages", "sdk", "sdks"}


def _detection_definitions() -> dict[str, dict]:
    defs = {}
    for lang in sorted(os.listdir(projectlib.BUILTIN_LANGUAGES)):
        cfg = os.path.join(projectlib.BUILTIN_LANGUAGES, lang, "config.json")
        if os.path.isfile(cfg):
            with open(cfg, encoding="utf-8") as f:
                defs[lang] = json.load(f)
    return defs


def dir_names(defs: dict[str, dict]) -> dict[str, list[str]]:
    """Language → the names its directory goes by, its own first."""
    return {lang: [lang] + [n for n in d.get("dirNames", []) if n != lang] for lang, d in defs.items()}


def _is_skipped(directory: str) -> bool:
    return any(part in SKIPPED_DIRS for part in directory.split("/"))


def tracked_files(root: str) -> list[str]:
    try:
        return subprocess.run(["git", "ls-files"], cwd=root, capture_output=True, text=True,
                              check=True).stdout.splitlines()
    except (OSError, subprocess.CalledProcessError):
        return []


def _names_by_dir(files: list[str]) -> tuple[dict[str, set[str]], dict[str, set[str]]]:
    """The file names in each directory: outside SKIPPED_DIRS, and inside."""
    by_dir: dict[str, set[str]] = {}
    skipped_dirs: dict[str, set[str]] = {}
    for path in files:
        directory = os.path.dirname(path) or "."
        tree = skipped_dirs if _is_skipped(directory) else by_dir
        tree.setdefault(directory, set()).add(os.path.basename(path))
    return by_dir, skipped_dirs


def _matches(names: set[str], patterns: list[str]) -> bool:
    return any(fnmatch.fnmatch(name, pattern) for name in names for pattern in patterns)


def holds_language(by_dir: dict[str, set[str]], defs: dict[str, dict]):
    """(lang, directory) → whether the directory holds the language's code:
    one of its build or source files ("detect", "detectSources") in it or
    under it, or, for a language without "detectSources", which can't tell
    its sources, any file when the directory sits at the root or under a
    PREFERRED_PARENTS directory (Thrift's lib/go, not a languages/go/
    holding Go's definition). A build file deeper than the hooks look for
    one (HOOK_SEARCH_DEPTH levels below) doesn't count, and with only such
    build files the directory doesn't hold the language: its hooks would
    find no build (antlr4's runtime/Go/antlr/v4/go.mod under the root)."""
    def holds(lang: str, directory: str) -> bool:
        d = defs.get(lang, {})
        inside = {sub: names for sub, names in by_dir.items() if _under(sub, directory)}
        if any(_matches(names, d.get("detectSources", [])) for names in inside.values()):
            return True
        builds = [sub for sub, names in by_dir.items()
                  if (directory == "." or _under(sub, directory)) and _matches(names, d.get("detect", []))]
        if any(_depth_below(sub, directory) <= HOOK_SEARCH_DEPTH for sub in builds):
            return True
        if builds:
            return False
        return not d.get("detectSources") and bool(inside) and _preference(directory)[0] == 0
    return holds


# How many levels below a directory the default hooks look for a build file
# (dirs_below_with in languages/_hooks.sh).
HOOK_SEARCH_DEPTH = 3


def _depth_below(sub: str, directory: str) -> int:
    parts = [] if sub == "." else sub.split("/")
    return len(parts) - (0 if directory == "." else len(directory.split("/")))


def _under(directory: str, parent: str) -> bool:
    return directory == parent or directory.startswith(parent + "/")


def detect_languages(root: str, defs: dict[str, dict] | None = None) -> tuple[dict[str, set[str]], dict[str, str]]:
    """(found, not_added). found maps a language to the directories holding
    one of its build files ("detect"), or, for a language with none, its
    sources ("detectSources"). Tracked files only.

    - A directory holding one of a language's "sharedBuildFiles" (C's and
      C++'s CMakeLists.txt, Makefile) is a build-file hit when the language's
      sources sit in it or under its src/ or include/.
    - A hit is dropped where one of the language's "detectUnless" files sits
      beside it (package.json is JavaScript unless a tsconfig.json says
      TypeScript), and, for a language with a "detectScript", where its
      package.json has no such npm script (Cucumber's jsonschema/, tooling
      with no test script).
    - Nothing under SKIPPED_DIRS counts.
    - A language found only by its sources keeps only the directories
      outside every other language's (protobuf's C in upb/, not the native
      extensions in python/ and ruby/), and isn't added when there are none
      (libphonenumber's C files in cpp/src/ are the C++ library's).

    not_added maps each language left out to why."""
    defs = _detection_definitions() if defs is None else defs
    by_dir, skipped_dirs = _names_by_dir(tracked_files(root))

    def hits(dirs: dict[str, set[str]], patterns: list[str], unless: list[str]) -> set[str]:
        return {directory for directory, names in dirs.items()
                if _matches(names, patterns) and not _matches(names, unless)}

    def count(names: set[str], patterns: list[str]) -> int:
        return sum(1 for name in names if any(fnmatch.fnmatch(name, pattern) for pattern in patterns))

    def shared_sources(directory: str, sources: list[str]) -> int:
        """The language's source files in a directory and under its src/ and include/."""
        nested = [os.path.normpath(os.path.join(directory, sub)) for sub in ("src", "include")]
        return count(by_dir[directory], sources) + sum(
            count(names, sources) for d, names in by_dir.items() if any(_under(d, n) for n in nested))

    # A shared build file goes to the language with the most sources under it
    # (libphonenumber's cpp/CMakeLists.txt is C++'s, not its few C files').
    shared_counts: dict[str, dict[str, int]] = {}
    for lang, d in defs.items():
        for directory in hits(by_dir, d.get("sharedBuildFiles", []), []):
            n = shared_sources(directory, d.get("detectSources", []))
            if n:
                shared_counts.setdefault(directory, {})[lang] = n

    def shared_hits(lang: str) -> set[str]:
        return {directory for directory, counts in shared_counts.items()
                if counts.get(lang) == max(counts.values())}

    found: dict[str, set[str]] = {}
    from_sources: set[str] = set()
    not_added: dict[str, str] = {}
    for lang, d in defs.items():
        unless, sources = d.get("detectUnless", []), d.get("detectSources", [])
        directories = hits(by_dir, d.get("detect", []), unless) | shared_hits(lang)
        script = d.get("detectScript")
        if script and directories:
            directories = {directory for directory in directories if _has_npm_script(root, directory, script)}
            if not directories:
                not_added[lang] = f"no {script} script"
                continue
        if not directories and sources:
            directories = hits(by_dir, sources, unless)
            from_sources.add(lang)
        if directories:
            found[lang] = directories
        elif hits(skipped_dirs, d.get("detect", []) + sources, unless):
            not_added[lang] = "found only in examples, tests or vendored code"

    names = dir_names(defs)
    claims = {lang: {c for c in _language_roots(dirs, names[lang]) | (set() if lang in from_sources else dirs)
                     if c != "."} for lang, dirs in found.items()}
    for lang in sorted(from_sources & set(found)):
        owners = {directory: {other for other, claimed in claims.items()
                              if other != lang and any(_under(directory, c) for c in claimed)}
                  for directory in found[lang]}
        own = {directory for directory, by in owners.items() if not by}
        if own:
            found[lang] = own
        else:
            everyone = sorted(set().union(*owners.values()))
            not_added[lang] = f"its files are all inside {', '.join(everyone)}'s directories"
            del found[lang]
    return found, not_added


def _has_npm_script(root: str, directory: str, script: str) -> bool:
    """Whether the directory's package.json has the npm script; one that
    can't be read counts as having it."""
    try:
        with open(os.path.join(root, directory, "package.json"), encoding="utf-8") as f:
            scripts = json.load(f).get("scripts") or {}
    except (OSError, ValueError, AttributeError):
        return True
    return isinstance(scripts, dict) and script in scripts


def _language_roots(directories: set[str], names: list[str]) -> set[str]:
    """Each directory cut at the first part naming the language (age/impl/c,
    not age/impl/c/src)."""
    roots = set()
    for directory in directories:
        parts = directory.split("/")
        for i, part in enumerate(parts):
            if part in names:
                roots.add("/".join(parts[:i + 1]))
                break
    return roots


def _as_template(root: str) -> str:
    parts = root.split("/")
    return "/".join(parts[:-1] + ["{lang}"])


def _preference(path: str) -> tuple[int, int]:
    """Lower is better: under a PREFERRED_PARENTS directory or at the root
    first, then shallower."""
    parts = path.split("/")
    preferred = len(parts) == 1 or any(part in PREFERRED_PARENTS for part in parts[:-1])
    return (0 if preferred else 1, len(parts))


def _own_directory(directories: set[str]) -> str:
    """A language's directory when none is named after it: the shallowest
    directory holding its files, or the common parent of several equally
    shallow ones."""
    depth = min(len(d.split("/")) for d in directories)
    shallowest = sorted(d for d in directories if len(d.split("/")) == depth)
    common = os.path.commonpath(shallowest) if len(shallowest) > 1 else shallowest[0]
    return common or "."


def infer_layout(found: dict[str, set[str]], names: dict[str, list[str]] | None = None,
                 holds=None) -> tuple[list[str], str, dict[str, str]]:
    """Components, implPath and per-language implPaths from where each
    language's files sit.

    `age/impl/go` and `date/impl/go` give components [age, date] and implPath
    `{component}/impl/{lang}`; `go/` and `rust/` give one component at
    `{lang}`. A directory may carry any of a language's names (dotnet/ for C#).
    With one component, a language whose directory doesn't fit the shared
    implPath gets its own entry in implPaths, unless the shared one names a
    directory holding the language's build or source files (`holds(lang,
    directory)`, see holds_language) that no other language was found in:
    Thrift's lib/go, whose go.mod is at the root, but not Cucumber's
    javascript/, which is TypeScript's, nor a languages/go/ holding no Go.
    """
    names = names or {}
    roots = {lang: _language_roots(dirs, names.get(lang, [lang])) for lang, dirs in found.items()}
    templates: dict[str, set[str]] = {}
    for lang, lang_roots in roots.items():
        for root in lang_roots:
            templates.setdefault(_as_template(root), set()).add(lang)
    if not templates:
        # Every language at the root, unless its hooks would find no build
        # from there: one whose files are all deeper than they look.
        return ["."], ".", {lang: _own_directory(dirs) for lang, dirs in sorted(found.items())
                            if holds and not holds(lang, ".")
                            and all(_depth_below(d, ".") > HOOK_SEARCH_DEPTH for d in dirs)}
    split = [t.split("/") for t in templates]
    if len(templates) > 1 and len({len(parts) for parts in split}) == 1:
        varying = [i for i in range(len(split[0])) if len({parts[i] for parts in split}) > 1]
        if len(varying) == 1:
            i = varying[0]
            pattern = split[0][:i] + ["{component}"] + split[0][i + 1:]
            return sorted({parts[i] for parts in split}), "/".join(pattern), {}
    template = min(templates, key=lambda t: (_preference(t), -len(templates[t]), t))
    overrides = {}
    for lang, dirs in sorted(found.items()):
        lang_names = names.get(lang, [lang])
        others = set().union(*(d for other, d in found.items() if other != lang))
        candidates = [template.replace("{lang}", name) for name in lang_names]
        fitting = [c for c in candidates
                   if c in roots[lang] or (holds and c not in others and holds(lang, c))]
        if fitting:
            continue
        overrides[lang] = (min(roots[lang], key=lambda r: (_preference(r), r)) if roots[lang]
                           else _own_directory(dirs))
    return ["."], template, overrides


JUSTFILE = """\
# Recipes delegating to hecatron (generated by: hecatron init --justfile).
# Import from the project justfile with: import '.hecatron/hecatron.just'

test *args:
    hecatron test {{args}}

lint *args:
    hecatron lint {{args}}

build *args:
    hecatron build {{args}}

clean *args:
    hecatron clean {{args}}

doctor *args:
    hecatron doctor {{args}}

install *args:
    hecatron install {{args}}
"""


AGENTS_BEGIN = "<!-- hecatron:begin -->"
AGENTS_END = "<!-- hecatron:end -->"

# What init writes into AGENTS.md: short, since agents load it every
# session, and nothing about the project's languages, which would go stale.
# `hct help agents` (docs/agents.md) has the rest.
AGENTS_SECTION = f"""{AGENTS_BEGIN}
## Building and testing (hecatron)

This repository uses [hecatron](https://github.com/dnf/hecatron): the same
commands for every language. `hct help agents` is a compact reference.

- Start with `hct status`: each language, its toolchain, its last runs and
  what changed, and what to run next.
- `hct test`, `hct lint`, `hct build`: every language; name one to run only
  it (`hct test go`). `hct format` fixes formatting; `hct deps` fetches
  dependencies.
- Exit status 0 passed, 1 failed. Each language's log is in
  `{{logs}}/<action>/<language>.log`.
- Ask before `hct install` or `hct doctor --fix`: they install toolchains
  on this machine. `hct doctor` alone only checks.
- To change how a language is built or tested, write
  `.hecatron/languages/<lang>/<action>.sh`.
{AGENTS_END}
"""


def write_agent_instructions(root: str, logs: str) -> list[str]:
    """Put hecatron's section in AGENTS.md (replacing an earlier one), and
    have an existing CLAUDE.md load AGENTS.md. The files changed."""
    section = AGENTS_SECTION.replace("{logs}", logs)
    changed = []
    path = os.path.join(root, "AGENTS.md")
    text = open(path, encoding="utf-8").read() if os.path.isfile(path) else ""
    if AGENTS_BEGIN in text and AGENTS_END in text:
        start, end = text.index(AGENTS_BEGIN), text.index(AGENTS_END) + len(AGENTS_END) + 1
        updated = text[:start] + section + text[end:]
    else:
        updated = text + ("\n" if text and not text.endswith("\n") else "") + ("\n" if text else "") + section
    if updated != text:
        with open(path, "w", encoding="utf-8") as f:
            f.write(updated)
        changed.append("AGENTS.md")
    claude = os.path.join(root, "CLAUDE.md")
    if os.path.isfile(claude):
        text = open(claude, encoding="utf-8").read()
        if "@AGENTS.md" not in text and AGENTS_BEGIN not in text:
            with open(claude, "a", encoding="utf-8") as f:
                f.write(("" if text.endswith("\n") or not text else "\n") + "\n@AGENTS.md\n")
            changed.append("CLAUDE.md")
    return changed


def detected_config(root: str, components: str | None = None, impl_path: str | None = None,
                    languages: str | None = None) -> tuple[dict, dict[str, str]]:
    """The config.json init writes for a repository: its languages and layout
    as detected, or as given (comma-separated), and the languages found but
    not added, with why."""
    defs = _detection_definitions()
    found, not_added = detect_languages(root, defs)
    by_dir, _ = _names_by_dir(tracked_files(root))
    found_components, found_impl_path, impl_paths = infer_layout(found, dir_names(defs),
                                                                 holds_language(by_dir, defs))
    chosen_components = [c for c in components.split(",") if c] if components else found_components
    if impl_path:
        found_impl_path, impl_paths = impl_path, {}
    chosen = sorted(languages.split(",")) if languages else sorted(found)
    config = {"schema": projectlib.SCHEMA, "hecatron": f">={projectlib.hecatron_version()}",
              "project": os.path.basename(root), "components": chosen_components, "implPath": found_impl_path,
              "languages": chosen, "tools": {}, "generated": {}, "logs": ".logs"}
    if chosen_components == ["."]:
        del config["components"]
    impl_paths = {lang: path for lang, path in impl_paths.items() if lang in chosen}
    if impl_paths:
        config["implPaths"] = impl_paths
    return config, not_added


def cmd_init(args) -> int:
    root = os.path.abspath(args.dir)
    try:
        subprocess.run(["git", "rev-parse", "--show-toplevel"], cwd=root, capture_output=True, check=True)
    except (OSError, subprocess.CalledProcessError):
        die(f"{root} is not inside a git repository")
    config_path = os.path.join(root, ".hecatron", "config.json")
    if os.path.isfile(config_path) and not args.force:
        print(f"{os.path.relpath(config_path)} already exists (use --force to overwrite).")
        return 0
    config, not_added = detected_config(root, args.components, args.impl_path, args.languages)
    languages, components, impl_path = config["languages"], config.get("components", ["."]), config["implPath"]
    impl_paths = config.get("implPaths", {})
    projectlib._write_json(config_path, config)
    projectlib.ignore_project_files(root)
    if args.justfile:
        with open(os.path.join(root, ".hecatron", "hecatron.just"), "w", encoding="utf-8") as f:
            f.write(JUSTFILE)
    agent_files = [] if args.no_agents else write_agent_instructions(root, config.get("logs", ".logs"))
    print(f"Initialised {root}")
    print(f"  languages:  {', '.join(languages) or '(none detected; add with hecatron add <lang>)'}")
    print(f"  components: {', '.join(components)}   implPath: {impl_path}")
    for lang, path in sorted(impl_paths.items()):
        print(f"  {lang} lives in {path} (implPaths)")
    for lang, reason in sorted(not_added.items()):
        print(f"  not added: {lang} ({reason})")
    if agent_files:
        print(f"  instructions for AI agents: {', '.join(agent_files)} (--no-agents to skip)")
    print("Next: hecatron doctor   (checks each language's toolchain)")
    return 0


# -- main ----------------------------------------------------------------------


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(prog="hecatron", description=__doc__,
                                     formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = parser.add_subparsers(dest="command", required=True)

    p = sub.add_parser("init", help="set up a repository")
    p.add_argument("dir", nargs="?", default=".")
    p.add_argument("--components")
    p.add_argument("--impl-path")
    p.add_argument("--languages")
    p.add_argument("--justfile", action="store_true")
    p.add_argument("--force", action="store_true")
    p.add_argument("--no-agents", action="store_true",
                   help="don't write instructions for AI agents to AGENTS.md")
    p.set_defaults(func=cmd_init)

    for name, func in (("install", cmd_install), ("uninstall", cmd_uninstall),
                       ("add", cmd_add), ("remove", cmd_remove),
                       ("enable", cmd_enable), ("disable", cmd_disable)):
        p = sub.add_parser(name)
        p.add_argument("languages", nargs="*")
        if name == "enable":
            p.add_argument("--all", action="store_true", help="re-enable every language")
        if name == "install":
            p.add_argument("--plan", action="store_true", help="show what would be installed (needs the binary)")
        p.set_defaults(func=func)

    p = sub.add_parser("list")
    p.add_argument("what", nargs="?", choices=["languages"], default="languages")
    p.add_argument("--all", action="store_true")
    p.set_defaults(func=cmd_list)

    for name, func in (("dockerfile", cmd_dockerfile), ("workflows", cmd_workflows)):
        p = sub.add_parser(name)
        p.add_argument("--check", action="store_true")
        if name == "workflows":
            p.add_argument("--ci", help="the CI system to generate for (github, gitlab); needs the binary")
        p.set_defaults(func=func)

    sub.add_parser("check").set_defaults(func=cmd_check)

    p = sub.add_parser("banner")
    p.add_argument("--generator", required=True)
    p.add_argument("--source")
    p.add_argument("--regenerate")
    p.add_argument("--prefix", default="# ")
    p.add_argument("--suffix", default="")
    p.set_defaults(func=cmd_banner)

    if len(argv) >= 2 and argv[1] in ("help", "--help"):
        argv = [argv[0], "--help"]
    args = parser.parse_args(argv)
    if args.command in ("add", "remove") and not args.languages:
        die(f"usage: hecatron {args.command} <lang>...", 2)
    return args.func(args)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
