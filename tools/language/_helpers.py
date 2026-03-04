"""Shared helpers for language management scripts."""

import json
import os
import subprocess
import sys


def project_root():
    """Return the project root (three levels up from this file)."""
    return os.path.dirname(
        os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
    )


def config_path(root):
    """Return the path to .hecatron/config/languages.json."""
    return os.path.join(root, ".hecatron", "config", "languages.json")


def load_config(path):
    """Read and return the languages config JSON."""
    with open(path) as f:
        return json.load(f)


def save_config(path, config):
    """Write the languages config JSON."""
    with open(path, "w") as f:
        json.dump(config, f, indent=2)
        f.write("\n")


def load_languages(root):
    """Discover languages from .hecatron/languages/*/config.json."""
    langs_dir = os.path.join(root, ".hecatron", "languages")
    languages = {}
    for entry in sorted(os.listdir(langs_dir)):
        cfg = os.path.join(langs_dir, entry, "config.json")
        if os.path.isfile(cfg):
            with open(cfg) as f:
                languages[entry] = json.load(f)
    return languages


def added_path(root):
    """Return the path to .hecatron/config/languages/added.json."""
    return os.path.join(root, ".hecatron", "config", "languages", "added.json")


def load_added(root):
    """Read the added languages list."""
    with open(added_path(root)) as f:
        return json.load(f)


def save_added(root, langs):
    """Write the added languages list."""
    with open(added_path(root), "w") as f:
        json.dump(sorted(set(langs)), f, indent=2)
        f.write("\n")


def disabled_path(root):
    """Return the path to .hecatron/config/languages/disabled.json."""
    return os.path.join(root, ".hecatron", "config", "languages", "disabled.json")


def read_disabled(path):
    """Read the disabled languages file, returning a sorted list."""
    if not os.path.isfile(path):
        return []
    with open(path) as f:
        return sorted(json.load(f))


def write_disabled(path, langs):
    """Write the disabled languages file (empty array when none disabled)."""
    with open(path, "w") as f:
        json.dump(sorted(set(langs)), f, indent=2)
        f.write("\n")


def resolve_name(lang, languages):
    """Resolve a language name or abbreviation to the canonical name."""
    lang = lang.lower()
    if lang in languages:
        return lang
    for name, data in languages.items():
        if data.get("abbreviation") == lang:
            return name
    return None


def workflow_banner(name):
    """Return a DO NOT EDIT banner for a copied workflow file."""
    # Import make_banner from the codegen util (canonical location)
    root = project_root()
    sys.path.insert(0, os.path.join(root, ".hecatron", "tools", "codegen"))
    from util import make_banner

    source = f".hecatron/languages/{name}/workflow.yml"
    return make_banner([
        f"Source: {source}",
        "Copied by: .hecatron/tools/language/_helpers.py",
        "",
        f"To update: edit the source, then rerun just add {name}",
        "Copied automatically by: just add",
    ])


def sync_workflow(name, root, action):
    """Copy or remove a CI workflow for a language.

    On add: copy .hecatron/languages/<name>/workflow.yml to
            .github/workflows/<name>.yml with a generated-file banner.
    On remove: delete .github/workflows/<name>.yml.
    """
    src = os.path.join(root, ".hecatron", "languages", name, "workflow.yml")
    dst = os.path.join(root, ".github", "workflows", f"{name}.yml")
    if action == "add":
        if os.path.isfile(src):
            with open(src) as f:
                content = f.read()
            with open(dst, "w") as f:
                f.write(workflow_banner(name))
                f.write(content)
    elif action == "remove":
        if os.path.isfile(dst):
            os.remove(dst)


def regenerate_dockerfile(root):
    """Regenerate Dockerfile from per-language snippets."""
    generator = os.path.join(root, ".hecatron", "tools", "codegen", "dockerfile.py")
    if os.path.isfile(generator):
        subprocess.run([sys.executable, generator], check=True)
