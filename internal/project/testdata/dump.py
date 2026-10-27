"""Print what libexec/lib/project.py makes of the project around the working
directory, as JSON, for the Go model's parity tests (parity_test.go).

    HECATRON_HOME=... python3 dump.py LANG...

A value project.py raised on is {"raised": "<exception type>"}.
"""

import json
import os
import sys

sys.path.insert(0, os.path.join(os.environ["HECATRON_HOME"], "libexec", "lib"))

import audit  # noqa: E402
import project  # noqa: E402

FILES = ["config.json", "install.sh", "doctor.sh", "workflow.yml", "dockerfile", "installed.sh", *project.HOOKS]
ACTIONS = ["test", "lint", "build", "clean", "audit"]


def field(compute):
    """compute(p) on a freshly loaded project: Project caches what it reads,
    a partly read set of definitions included, and each command reads it
    afresh."""
    try:
        return compute(project.load())
    except Exception as e:  # noqa: BLE001 - which exception is the point
        return {"raised": type(e).__name__}


def shell(p):
    project.shell_env(p)  # raises where helpers.sh would fail
    defs = p.definitions
    added = [lang for lang in p.languages if lang in defs]
    return {
        "exports": [["HECATRON_ROOT", p.root], ["HECATRON_PROJECT", p.name],
                    ["HECATRON_COMPONENTS", " ".join(p.components)], ["HECATRON_IMPL_PATH", p.impl_path],
                    ["HECATRON_LOGS", p.logs],
                    ["HECATRON_METADATA", os.path.join(p.root, p.config["metadata"]) if p.config.get("metadata") else ""]],
        "known": sorted(defs),
        "added": added,
        "hooks": {hook: [lang for lang in added if p.resolve(lang, hook)] for hook in project.HOOKS},
        "abbreviations": {lang: d["abbreviation"] for lang, d in defs.items() if d.get("abbreviation")},
    }


def language(lang):
    return {
        "implTemplate": field(lambda p: p.impl_template(lang)),
        "dirNames": field(lambda p: p.dir_names(lang)),
        "implDir": field(lambda p: {c: p.impl_dir(c, lang) for c in p.components}),
        "implDirs": field(lambda p: [list(pair) for pair in p.impl_dirs(lang)]),
        "resolve": field(lambda p: {f: p.resolve(lang, f) or "" for f in FILES}),
        "projectDefinition": field(lambda p: os.path.isdir(os.path.join(p.dir, "languages", lang))),
        "resolveName": field(lambda p: p.resolve_name(lang)),
    }


def main():
    try:
        p = project.load()
    except project.ProjectError as e:
        print(json.dumps({"error": str(e)}))
        return
    except Exception as e:  # noqa: BLE001
        print(json.dumps({"crash": type(e).__name__}))
        return
    langs = sys.argv[1:]
    out = {
        "root": p.root,
        "name": field(lambda p: p.name),
        "components": field(lambda p: p.components),
        "implPath": field(lambda p: p.impl_path),
        "languages": field(lambda p: p.languages),
        "disabled": field(lambda p: p.disabled),
        "logs": field(lambda p: p.logs),
        "generated": field(lambda p: {k: p.generated(k) or "" for k in ("dockerfile", "workflows")}),
        "tasks": field(lambda p: {a: [[n, c] for n, c in p.tasks(a).items()] for a in ACTIONS}),
        "tools": field(lambda p: {t: d.get("dependency", "") or "" for t, d in p.tools.items()}),
        "definitions": field(lambda p: p.definitions),
        "shell": field(shell),
        "configProblems": field(lambda p: audit.config_problems(p.config)),
        "lang": {lang: language(lang) for lang in langs},
    }
    print(json.dumps(out))


main()
