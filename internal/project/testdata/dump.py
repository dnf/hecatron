"""Print what libexec/lib/project.py makes of the project around the working
directory, as JSON, for the Go model's parity tests (parity_test.go).

    HECATRON_HOME=... python3 dump.py LANG...

With HECATRON_DUMP_VERSIONS=1, each language's toolchain versions and
environment too (which runs its installed.sh).

A value project.py raised on is {"raised": "<exception type>"}, or, for the
errors its command line reports as "hecatron: <message>" (ProjectError and
ValueError), {"error": "<message>"}.
"""

import json
import os
import sys

sys.path.insert(0, os.path.join(os.environ["HECATRON_HOME"], "libexec", "lib"))

import audit  # noqa: E402
import project  # noqa: E402

# The platform project.py takes the path of (HOST_OS), for the parity tests
# of macOS-only behaviour on other platforms.
if os.environ.get("HECATRON_DUMP_HOST_OS"):
    project.HOST_OS = os.environ["HECATRON_DUMP_HOST_OS"]

FILES = ["config.json", "install.sh", "doctor.sh", "workflow.yml", "dockerfile", "installed.sh", *project.HOOKS]
ACTIONS = ["test", "lint", "build", "clean", "audit"]


def field(compute):
    """compute(p) on a freshly loaded project: Project caches what it reads,
    a partly read set of definitions included, and each command reads it
    afresh."""
    try:
        return compute(project.load())
    except (project.ProjectError, ValueError) as e:
        return {"error": str(e)}
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
        "lintFormat": [lang for lang in added
                       if p.resolve(lang, "format.sh") and project.lint_checks_format(p.config, lang)],
        "abbreviations": {lang: d["abbreviation"] for lang, d in defs.items() if d.get("abbreviation")},
    }


def component_version(lang, component):
    """What a component's language resolves to, as the version and env
    queries print it ("" for None)."""
    def compute(p):
        resolved = p.resolve_version(lang, component)
        match = p.installed_version(lang, component)
        return {"spec": resolved["spec"] or "", "source": resolved["source"] or "", "exact": resolved["exact"],
                "selects": p.selects_version(lang, component), "installed": list(match) if match else [],
                "env": project.env_exports(p.language_env(lang, component))}
    return field(compute)


def versions_of(lang, components):
    if not os.environ.get("HECATRON_DUMP_VERSIONS"):
        return {}
    return {
        "default": field(lambda p: p.default_version(lang) or ""),
        "installedAll": field(lambda p: [list(pair) for pair in p.installed(lang)]),
        "version": {c or "(none)": component_version(lang, c) for c in components + [""]},
        "versions": field(lambda p: [[spec or "", sources] for spec, sources in p.versions(lang)]),
        "missing": field(lambda p: [list(m) for m in p.missing_versions(lang)]),
        "notes": field(lambda p: [list(n) for n in p.version_notes(lang)]),
        "staticPath": field(lambda p: p._static_path(lang)),
    }


def language(lang, components):
    return {
        **versions_of(lang, components),
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
    try:
        components = p.components
    except Exception:  # noqa: BLE001 - the components field reports it
        components = []
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
        "configProblems": field(lambda p: audit.config_problems(p.config) + project.lint_config_problems(p.config)),
        "asOf": field(lambda p: p.as_of() or ""),
        "conflicts": field(lambda p: [[lang, c, source, spec or "", wanted]
                                      for lang, c, source, spec, wanted in p.version_conflicts()]),
        "lang": {lang: language(lang, components) for lang in langs},
    }
    print(json.dumps(out))


main()
