#!/usr/bin/env python3
"""Caches each language's toolchain install between real-project jobs.

Each job runs in a fresh container, so without a cache every job installs
every toolchain from scratch. A language's cache is a directory,
<cache>/<lang>, holding what installing that language left in the home
directory (home/: hecatron's toolchains and bin directories, and the
directories the language's definition caches as its toolchain, as
"ci": {"cache": true} does) and the Debian packages apt downloaded (apt/).
Its key is hecatron's version and a hash of the language's definition, as
generated CI keys a toolchain cache on what the executor would install.

Only a job that tests one language, installs it cleanly and restored no
cache saves one, so a cache holds that language's install and nothing
else. Every job restores the caches of the languages it's likely to find
(the last run's, else the expected ones) and merges them before installing;
the installers check what's there and install only what's missing.

  cache.py plan REPO                on the runner: the job's caches, as
                                    GITHUB_OUTPUT lines (lang1, key1, ...)
  cache.py order                    on the runner: the projects, those
                                    with one language first (they save)
  cache.py run [run.sh options] OWNER/REPO
                                    in the container: restore what's in
                                    /cache, run run.sh, save the language's
                                    cache there when this job should
"""

import hashlib
import json
import os
import platform
import shutil
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
HOME = os.path.dirname(os.path.dirname(HERE))
CACHE = "/cache"
# Most languages a job restores; the steps in real-projects.yml match.
SLOTS = 8
# Hecatron's own directories every install writes to. The download cache
# (~/.cache/hecatron/downloads) is left out: it holds the archives the
# toolchains directory has unpacked, so it would double a cache's size
# and save nothing once the toolchain is restored.
HECATRON_DIRS = [".local/share/hecatron/toolchains", ".local/share/hecatron/bin"]
APT_ARCHIVES = "/var/cache/apt/archives"
# Ubuntu's images delete downloaded packages after each install.
APT_CLEAN = "/etc/apt/apt.conf.d/docker-clean"
INSTALL_DONE = "### done install rc="


def _load(path: str) -> dict:
    with open(path) as f:
        return json.load(f)


def definition_hash(lang: str, home: str = HOME) -> str:
    """A short hash of the language's definition: every file of it."""
    digest = hashlib.sha256()
    root = os.path.join(home, "languages", lang)
    for directory, dirs, files in sorted(os.walk(root)):
        dirs.sort()
        for name in sorted(files):
            path = os.path.join(directory, name)
            digest.update(os.path.relpath(path, root).encode() + b"\0")
            with open(path, "rb") as f:
                digest.update(f.read() + b"\0")
    return digest.hexdigest()[:16]


def key(lang: str, home: str = HOME) -> str:
    with open(os.path.join(home, "VERSION")) as f:
        version = f.read().strip()
    return f"real-projects-{lang}-{platform.system()}-{platform.machine()}-{version}-{definition_hash(lang, home)}"


def toolchain_dirs(lang: str, home: str = HOME) -> list[str]:
    """The home directories a language's cache holds, relative to ~."""
    config = _load(os.path.join(home, "languages", lang, "config.json"))
    own = [d[2:] for d in (config.get("cache") or {}).get("toolchain") or [] if d.startswith("~/")]
    return HECATRON_DIRS + [d for d in own if d not in HECATRON_DIRS]


def languages(repo: str, home: str = HOME) -> list[str]:
    """The languages a project is likely to have: the last run's, else the expected ones."""
    data = os.path.join(home, "tests", "real-projects")
    found: list[str] = []
    for run in _load(os.path.join(data, "results.json"))["results"]:
        if run["repo"].lower() == repo.lower():
            found = [lang["language"] for lang in run["languages"]]
    if not found:
        for project in _load(os.path.join(data, "projects.json"))["projects"]:
            if project["repo"].lower() == repo.lower():
                found = list(project["languages"])
    known = set(os.listdir(os.path.join(home, "languages")))
    return [lang for lang in found if lang in known]


def plan(repo: str, home: str = HOME) -> list[str]:
    lines = []
    for slot, lang in enumerate(languages(repo, home)[:SLOTS], 1):
        lines += [f"lang{slot}={lang}", f"key{slot}={key(lang, home)}"]
    return lines


def order(home: str = HOME) -> list[str]:
    """Every project, those with one language first, so their caches are
    saved before the polyglot projects that restore them start."""
    data = os.path.join(home, "tests", "real-projects")
    count = {run["repo"].lower(): len(run["languages"]) for run in _load(os.path.join(data, "results.json"))["results"]}
    projects = _load(os.path.join(data, "projects.json"))["projects"]
    sizes = {p["repo"]: count.get(p["repo"].lower()) or len(p["languages"]) for p in projects}
    return sorted(sizes, key=lambda repo: sizes[repo] != 1)


def _copy(source: str, target: str) -> None:
    """Merges directory source into target, keeping links and modes."""
    os.makedirs(target, exist_ok=True)
    subprocess.run(["cp", "-a", source.rstrip("/") + "/.", target], check=True)


def restore(cache: str, user_home: str) -> list[str]:
    restored = []
    for lang in sorted(os.listdir(cache)):
        if not os.path.isfile(os.path.join(cache, lang, "complete")):
            continue
        if os.path.isdir(os.path.join(cache, lang, "home")):
            _copy(os.path.join(cache, lang, "home"), user_home)
        debs = os.path.join(cache, lang, "apt")
        if os.path.isdir(debs) and os.listdir(debs):
            subprocess.run(["sudo", "cp", "-a", debs + "/.", APT_ARCHIVES], check=True)
        restored.append(lang)
    return restored


def save(cache: str, user_home: str, lang: str, home: str = HOME, archives: str = APT_ARCHIVES) -> None:
    """Writes the language's cache, whole or not at all."""
    partial = os.path.join(cache, f".{lang}.partial")
    shutil.rmtree(partial, ignore_errors=True)
    for relative in toolchain_dirs(lang, home):
        source = os.path.join(user_home, relative)
        if os.path.isdir(source):
            _copy(source, os.path.join(partial, "home", relative))
    debs = [name for name in os.listdir(archives) if name.endswith(".deb")] if os.path.isdir(archives) else []
    os.makedirs(os.path.join(partial, "apt"), exist_ok=True)
    for name in debs:
        shutil.copy2(os.path.join(archives, name), os.path.join(partial, "apt", name))
    with open(os.path.join(partial, "complete"), "w") as f:
        f.write(key(lang, home) + "\n")
    os.rename(partial, os.path.join(cache, lang))
    with open(os.path.join(cache, "saved"), "w") as f:
        f.write(f"lang={lang}\nkey={key(lang, home)}\n")


def run(args: list[str]) -> int:
    enabled = os.path.isdir(CACHE)
    user_home = os.path.expanduser("~")
    restored = []
    if enabled:
        subprocess.run(["sudo", "rm", "-f", APT_CLEAN], check=True)
        restored = restore(CACHE, user_home)
    print("### cache " + json.dumps({"enabled": enabled, "restored": restored}), flush=True)

    install_rc = None
    with subprocess.Popen(["bash", os.path.join(HERE, "run.sh"), *args], stdin=subprocess.DEVNULL,
                          stdout=subprocess.PIPE, stderr=subprocess.STDOUT) as child:
        assert child.stdout is not None
        for line in child.stdout:
            sys.stdout.buffer.write(line)
            sys.stdout.buffer.flush()
            text = line.decode(errors="replace")
            if text.startswith(INSTALL_DONE):
                install_rc = int(text[len(INSTALL_DONE):].split()[0])
    rc = child.returncode

    if not enabled or restored or install_rc != 0:
        return rc
    repo = args[-1]
    config = os.path.join(user_home, repo.replace("/", "_"), ".hecatron", "config.json")
    try:
        found = _load(config).get("languages") or []
    except (OSError, ValueError):
        return rc
    if len(found) == 1:
        save(CACHE, user_home, found[0])
        print(f"### cache saved {found[0]}", flush=True)
    return rc


def main(argv: list[str]) -> int:
    if len(argv) >= 2 and argv[0] == "plan":
        print("\n".join(plan(argv[1])))
        return 0
    if argv == ["order"]:
        print(json.dumps(order()))
        return 0
    if len(argv) >= 2 and argv[0] == "run":
        return run(argv[1:])
    print(__doc__, file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
