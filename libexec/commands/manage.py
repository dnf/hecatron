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

HOME = os.environ.get("HECATRON_HOME", os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
sys.path.insert(0, os.path.join(HOME, "libexec", "lib"))
sys.path.insert(0, os.path.join(HOME, "libexec", "codegen"))

import project as projectlib  # noqa: E402
from banner import make_banner  # noqa: E402
import dockerfile as dockerfilelib  # noqa: E402


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


def run_definition_script(project, lang: str, script: str) -> int:
    path = project.resolve(lang, script)
    if path is None:
        print(f"{lang}: no {script}", file=sys.stderr)
        return 1
    env = hook_env(project)
    environment = project.language_env(lang)
    env.update(environment["vars"])
    env["PATH"] = os.pathsep.join(environment["path"] + [env.get("PATH", "")])
    return subprocess.run(["bash", path], env=env, cwd=project.root).returncode


def cmd_install(args) -> int:
    project = load_project()
    install_all = not args.languages
    langs = project.languages if install_all else resolve_all(project, args.languages, added_only=True)
    failed = []
    for lang in langs:
        if run_definition_script(project, lang, "install.sh") != 0:
            if not install_all:
                return 1
            failed.append(lang)
    if failed:
        print(f"\nFailed to install: {' '.join(failed)}", file=sys.stderr)
        return 1
    return 0


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
    return banner + body


def write_workflows(project, *, check: bool) -> bool:
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
    ok = write_workflows(load_project(), check=args.check)
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
        matrix_path = project.impl_path.replace("{component}", "${{ matrix.component }}").replace("{lang}", lang)
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
    return 0 if ok else 1


def cmd_banner(args) -> int:
    rows = ["This file is auto-generated by:", f"  {args.generator}", ""]
    rows.append(f"Source: {args.source}" if args.source else "Changes will be overwritten when it regenerates.")
    rows.append(f"To regenerate: {args.regenerate}" if args.regenerate else "Edit the generator or its input instead.")
    sys.stdout.write(make_banner(rows, line_prefix=args.prefix, line_suffix=args.suffix))
    return 0


# -- init -----------------------------------------------------------------------


def detect_languages(root: str) -> dict[str, set[str]]:
    """Language → directories holding one of its `detect` manifests (tracked files only)."""
    defs = {}
    for lang in sorted(os.listdir(projectlib.BUILTIN_LANGUAGES)):
        cfg = os.path.join(projectlib.BUILTIN_LANGUAGES, lang, "config.json")
        if os.path.isfile(cfg):
            with open(cfg, encoding="utf-8") as f:
                defs[lang] = json.load(f).get("detect", [])
    try:
        files = subprocess.run(["git", "ls-files"], cwd=root, capture_output=True, text=True,
                               check=True).stdout.splitlines()
    except (OSError, subprocess.CalledProcessError):
        files = []
    found: dict[str, set[str]] = {}
    for path in files:
        base = os.path.basename(path)
        for lang, patterns in defs.items():
            if any(fnmatch.fnmatch(base, pattern) for pattern in patterns):
                found.setdefault(lang, set()).add(os.path.dirname(path) or ".")
    return found


def infer_layout(found: dict[str, set[str]]) -> tuple[list[str], str]:
    """Components and implPath from where each language's manifests sit.

    `age/impl/go` and `date/impl/go` give components [age, date] and implPath
    `{component}/impl/{lang}`; `go/` and `rust/` give one component at `{lang}`.
    """
    templates: dict[str, set[str]] = {}
    for lang, directories in found.items():
        for directory in directories:
            parts = directory.split("/")
            if lang in parts:
                # A language's root is the path up to its own name (age/impl/c, not age/impl/c/src).
                index = parts.index(lang)
                templates.setdefault("/".join(parts[:index] + ["{lang}"]), set()).add(lang)
    if not templates:
        return ["."], "."
    if len(templates) == 1:
        return ["."], next(iter(templates))
    split = [t.split("/") for t in templates]
    if len({len(parts) for parts in split}) == 1:
        varying = [i for i in range(len(split[0])) if len({parts[i] for parts in split}) > 1]
        if len(varying) == 1:
            i = varying[0]
            pattern = split[0][:i] + ["{component}"] + split[0][i + 1:]
            return sorted({parts[i] for parts in split}), "/".join(pattern)
    return ["."], max(templates, key=lambda t: len(templates[t]))


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
    found = detect_languages(root)
    components, impl_path = infer_layout(found)
    if args.components:
        components = [c for c in args.components.split(",") if c]
    if args.impl_path:
        impl_path = args.impl_path
    languages = sorted(args.languages.split(",")) if args.languages else sorted(found)
    config = {"schema": projectlib.SCHEMA, "hecatron": f">={projectlib.hecatron_version()}",
              "project": os.path.basename(root), "components": components, "implPath": impl_path,
              "languages": languages, "tools": {}, "generated": {}, "logs": ".logs"}
    if components == ["."]:
        del config["components"]
    projectlib._write_json(config_path, config)
    ignore_path = os.path.join(root, ".gitignore")
    existing = open(ignore_path, encoding="utf-8").read() if os.path.isfile(ignore_path) else ""
    missing = [entry for entry in (".hecatron/local.json", ".logs/") if entry not in existing.split("\n")]
    if missing:
        with open(ignore_path, "a", encoding="utf-8") as f:
            f.write(("" if not existing or existing.endswith("\n") else "\n") + "\n".join(missing) + "\n")
    if args.justfile:
        with open(os.path.join(root, ".hecatron", "hecatron.just"), "w", encoding="utf-8") as f:
            f.write(JUSTFILE)
    print(f"Initialised {root}")
    print(f"  languages:  {', '.join(languages) or '(none detected; add with hecatron add <lang>)'}")
    print(f"  components: {', '.join(components)}   implPath: {impl_path}")
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
    p.set_defaults(func=cmd_init)

    for name, func in (("install", cmd_install), ("uninstall", cmd_uninstall),
                       ("add", cmd_add), ("remove", cmd_remove),
                       ("enable", cmd_enable), ("disable", cmd_disable)):
        p = sub.add_parser(name)
        p.add_argument("languages", nargs="*")
        if name == "enable":
            p.add_argument("--all", action="store_true", help="re-enable every language")
        p.set_defaults(func=func)

    p = sub.add_parser("list")
    p.add_argument("what", nargs="?", choices=["languages"], default="languages")
    p.add_argument("--all", action="store_true")
    p.set_defaults(func=cmd_list)

    for name, func in (("dockerfile", cmd_dockerfile), ("workflows", cmd_workflows)):
        p = sub.add_parser(name)
        p.add_argument("--check", action="store_true")
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
