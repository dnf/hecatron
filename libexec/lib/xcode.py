"""What the Xcode hooks (xcode_hook in languages/_hooks.sh) read from an
Xcode project's files, run in the directory being built.

    xcode.py apps <lang> <root>     the directories below this one holding an
                                    Xcode project that detection takes for
                                    the language, as ./paths, one per line
    xcode.py swift-builds <root>    exits 0 when Swift's hooks build the
                                    Xcode project here (a directory of
                                    Swift's, or an app below one)
    xcode.py scheme <container> [<project>] [<name>...]
                                    the scheme to build, from xcodebuild
                                    -list -json's report (stdin), and why,
                                    tab-separated; nothing when none fits
    xcode.py missing <workspace>    the projects a workspace refers to that
                                    aren't there, one per line

Only two facts come from the project files themselves: which native targets
are applications (project.pbxproj's productType), and which target a shared
scheme builds (its .xcscheme's BuildAction). Everything else (the schemes,
the build settings) comes from xcodebuild, which reads the files as Xcode
does.
"""

from __future__ import annotations

import glob
import json
import os
import re
import sys
import xml.etree.ElementTree as ElementTree

HOME = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
# project.py and manage.py find hecatron's files through HECATRON_HOME as
# they load: the hooks' tests run them with it empty.
os.environ["HECATRON_HOME"] = os.environ.get("HECATRON_HOME") or HOME

APPLICATION = "com.apple.product-type.application"
# A PBXNativeTarget object: its id, then its body up to the line closing it
# (a target's body holds lists, never another object).
NATIVE_TARGET = re.compile(r"^\s*([0-9A-Za-z_]+)\s*(?:/\*.*?\*/)?\s*=\s*\{\s*isa\s*=\s*PBXNativeTarget\s*;(.*?)^\s*\};",
                           re.M | re.S)


def _setting(body: str, key: str) -> str:
    match = re.search(rf"^\s*{key}\s*=\s*(\"(?:[^\"\\]|\\.)*\"|[^;]*?)\s*;", body, re.M)
    return match.group(1).strip('"') if match else ""


def application_targets(project: str) -> dict[str, str]:
    """The project's application targets: object id → target name."""
    try:
        with open(os.path.join(project, "project.pbxproj"), encoding="utf-8", errors="replace") as f:
            text = f.read()
    except OSError:
        return {}
    return {match.group(1): _setting(match.group(2), "name") for match in NATIVE_TARGET.finditer(text)
            if _setting(match.group(2), "productType") == APPLICATION}


def built_targets(scheme: str) -> set[str]:
    """The BlueprintIdentifiers a scheme's BuildAction builds."""
    try:
        tree = ElementTree.parse(scheme)
    except (OSError, ElementTree.ParseError):
        return set()
    return {reference.get("BlueprintIdentifier", "")
            for reference in tree.getroot().findall("./BuildAction/BuildActionEntries/BuildActionEntry/BuildableReference")}


def workspace_projects(workspace: str) -> list[str]:
    """The .xcodeproj paths a workspace's contents.xcworkspacedata refers to,
    relative to this directory: group: locations are relative to their
    enclosing group's, container: ones to the workspace's directory."""
    try:
        root = ElementTree.parse(os.path.join(workspace, "contents.xcworkspacedata")).getroot()
    except (OSError, ElementTree.ParseError):
        return []
    base = os.path.dirname(workspace)
    found = []

    def resolve(location: str, group: str) -> str | None:
        kind, _, path = location.partition(":")
        if kind == "group":
            return os.path.normpath(os.path.join(group, path))
        if kind == "container":
            return os.path.normpath(os.path.join(base, path))
        if kind == "absolute":
            return path
        return None  # self: (a project's own workspace), developer:

    def walk(element, group: str) -> None:
        for child in element:
            path = resolve(child.get("location", ""), group)
            if child.tag == "Group":
                walk(child, path if path is not None else group)
            elif child.tag == "FileRef" and path and path.endswith(".xcodeproj"):
                found.append(path)

    walk(root, base)
    return found


def scheme(listing: dict, container: str, project: str, names: list[str]) -> tuple[str, str]:
    """The scheme to build and why, or ("", "")."""
    info = listing.get("workspace") or listing.get("project") or {}
    listed = info.get("schemes") or []
    projects = [project] if project else []
    if container.endswith(".xcworkspace"):
        projects += [p for p in workspace_projects(container) if p not in projects]
    apps: dict[str, str] = {}
    for path in projects:
        apps.update(application_targets(path))
    shared = [container] + [p for p in projects if p != container]
    app_schemes = sorted({os.path.basename(path)[:-len(".xcscheme")]
                          for directory in shared
                          for path in glob.glob(os.path.join(directory, "xcshareddata", "xcschemes", "*.xcscheme"))
                          if built_targets(path) & set(apps)} & set(listed))
    if app_schemes:
        named = next((name for name in names if name in app_schemes), None)
        if named:
            return named, "the shared scheme building the app, named like the project"
        if len(app_schemes) == 1:
            return app_schemes[0], "the shared scheme building the app"
        # Several (home-assistant's App-Debug and App-Release, Firefox's
        # Fennec, Firefox and FirefoxBeta): the first by name, a Release
        # one only when all are. A check builds the debug configuration.
        first = min(app_schemes, key=lambda name: ("release" in name.lower(), name))
        return first, f"the first of the shared schemes building the app ({', '.join(app_schemes)})"
    app_names = sorted(set(apps.values()))
    if len(app_names) == 1 and app_names[0] in listed:
        return app_names[0], "the scheme of the project's only app target"
    named = next((name for name in names if name in listed), None)
    if named:
        return named, "the scheme named like the project"
    return "", ""


def detected(root: str) -> dict[str, set[str]]:
    """Where detection (manage.py's detect_languages) finds each language."""
    sys.path.insert(0, os.path.join(HOME, "libexec", "commands"))
    import manage  # noqa: E402

    return manage.detect_languages(root)[0]


def apps(lang: str, root: str, start: str = ".", found: dict[str, set[str]] | None = None) -> list[str]:
    """The directories below start that detection finds the language in,
    each holding an Xcode project or workspace itself, as absolute paths."""
    root = os.path.realpath(root)
    here = os.path.relpath(os.path.realpath(start), root)
    below = []
    for directory in sorted((detected(root) if found is None else found).get(lang, ())):
        if directory == here or here != "." and not directory.startswith(here + "/"):
            continue
        path = os.path.join(root, directory)
        if glob.glob(os.path.join(path, "*.xcodeproj")) or glob.glob(os.path.join(path, "*.xcworkspace")):
            below.append(path)
    return below


def swift_builds(root: str) -> bool:
    """Whether Swift's hooks build this directory's Xcode project: Swift is
    added and enabled, and this is one of its directories, or one of the
    apps below one (firefox-ios' focus-ios, below the root's Package.swift)."""
    sys.path.insert(0, os.path.join(HOME, "libexec", "lib"))
    import project  # noqa: E402

    model = project.load()
    if "swift" not in model.languages or "swift" in model.disabled:
        return False
    here = os.path.realpath(".")
    directories = [os.path.realpath(d) for _, d in model.impl_dirs("swift")]
    if here in directories:
        return True
    found = detected(root)
    return any(here == os.path.realpath(app) for d in directories for app in apps("swift", root, d, found))


def main(argv: list[str]) -> int:
    if len(argv) >= 3 and argv[0] == "apps":
        for directory in apps(argv[1], argv[2]):
            print("./" + os.path.relpath(directory))
        return 0
    if len(argv) == 2 and argv[0] == "swift-builds":
        return 0 if swift_builds(argv[1]) else 1
    if len(argv) >= 2 and argv[0] == "scheme":
        try:
            listing = json.load(sys.stdin)
        except ValueError:
            listing = {}
        name, why = scheme(listing if isinstance(listing, dict) else {}, argv[1], argv[2] if len(argv) > 2 else "",
                           argv[3:])
        if name:
            print(f"{name}\t{why}")
        return 0
    if len(argv) == 2 and argv[0] == "missing":
        for path in workspace_projects(argv[1]):
            if not os.path.isdir(path):
                print(path)
        return 0
    print(__doc__, file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
