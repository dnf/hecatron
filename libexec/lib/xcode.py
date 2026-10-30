"""What the Xcode hooks (xcode_hook in languages/_hooks.sh) read from an
Xcode project's files, run in the directory being built.

    xcode.py apps <lang> [<container>...]
                                    the apps below this directory: the
                                    directories holding the containers given
                                    (hct _xcode plan's nested lines), else
                                    the Xcode projects and workspaces found
                                    below as plan finds them; outside
                                    vendored code, examples and tests, with
                                    the language's sources; as ./paths, one
                                    per line
    xcode.py has-app                exits 0 when an Xcode project here (or a
                                    workspace's) has an application target
    xcode.py swift-builds           exits 0 when Swift's hooks build the
                                    Xcode project here (a directory of
                                    Swift's, or an app below one)
    xcode.py scheme <container> [<project>] [<name>...]
                                    the scheme to build, from xcodebuild
                                    -list -json's report (stdin), and why,
                                    tab-separated; nothing when none fits,
                                    and only the why when several do
    xcode.py missing <workspace>    the projects a workspace refers to that
                                    aren't there, one per line

This is the route without hct's binary, or where its reader of the files
(hct _xcode plan, internal/xcode) is unsure. Only a few facts come from
the project files here: which native targets are applications
(project.pbxproj's productType) and which targets an app depends on, and
which targets a shared scheme builds (its .xcscheme's BuildAction).
Everything else (the schemes, the build settings) comes from xcodebuild,
which reads the files as Xcode does.
"""

from __future__ import annotations

import fnmatch
import glob
import json
import os
import re
import sys
import xml.etree.ElementTree as ElementTree

HOME = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
# project.py finds hecatron's files through HECATRON_HOME as
# they load: the hooks' tests run them with it empty.
os.environ["HECATRON_HOME"] = os.environ.get("HECATRON_HOME") or HOME
sys.path.insert(0, os.path.join(HOME, "libexec", "lib"))
import project  # noqa: E402

APPLICATION = "com.apple.product-type.application"
# A PBXNativeTarget (and a PBXTargetDependency) object: its id, then its
# body up to the line closing it (a body holds lists, never an object).
NATIVE_TARGET = re.compile(
    r"^\s*([0-9A-Za-z_]+)\s*(?:/\*[^\n]*?\*/)?\s*=\s*\{\s*isa\s*=\s*PBXNativeTarget\s*;(.*?)^\s*\};", re.M | re.S)
TARGET_DEPENDENCY = re.compile(
    r"^\s*([0-9A-Za-z_]+)\s*(?:/\*[^\n]*?\*/)?\s*=\s*\{\s*isa\s*=\s*PBXTargetDependency\s*;(.*?)^\s*\};", re.M | re.S)
IDENTIFIER = re.compile(r"[0-9A-Za-z_]+")
# Directories whose Xcode projects aren't the project's own apps: those
# detection skips (vendored code, examples, tests: project.SKIPPED_DIRS),
# and dependencies and build output, as internal/xcode's Below() skips
# (firefox-ios/ThirdParty/Deferred). Compared lowercased.
NOT_APPS = {"build", "carthage", "deriveddata", "external", "node_modules", "pods", "third-party", "third_party",
            "thirdparty", "vendor"}
# How many levels below a directory its apps' containers can be
# (internal/xcode's Below()).
APPS_DEPTH = 5
CONTAINERS = (".xcodeproj", ".xcworkspace")


def _setting(body: str, key: str) -> str:
    match = re.search(rf"^\s*{key}\s*=\s*(\"(?:[^\"\\]|\\.)*\"|[^;]*?)\s*;", body, re.M)
    return match.group(1).strip('"') if match else ""


def _pbxproj(project: str) -> str:
    try:
        with open(os.path.join(project, "project.pbxproj"), encoding="utf-8", errors="replace") as f:
            return f.read()
    except OSError:
        return ""


def application_targets(project: str) -> dict[str, str]:
    """The project's application targets: object id → target name."""
    return {match.group(1): _setting(match.group(2), "name") for match in NATIVE_TARGET.finditer(_pbxproj(project))
            if _setting(match.group(2), "productType") == APPLICATION}


def embedded_targets(project: str) -> set[str]:
    """The targets an application target depends on: the extensions and
    helper apps (a Mac login item, a watchOS app) it carries."""
    text = _pbxproj(project)
    dependency_target = {match.group(1): _identifier(_setting(match.group(2), "target"))
                         for match in TARGET_DEPENDENCY.finditer(text)}
    embedded = set()
    for match in NATIVE_TARGET.finditer(text):
        if _setting(match.group(2), "productType") != APPLICATION:
            continue
        listed = re.search(r"^\s*dependencies\s*=\s*\((.*?)\)\s*;", match.group(2), re.M | re.S)
        dependencies = IDENTIFIER.findall(re.sub(r"/\*.*?\*/", "", listed.group(1), flags=re.S)) if listed else []
        embedded |= {dependency_target[d] for d in dependencies
                     if dependency_target.get(d) and dependency_target[d] != match.group(1)}
    return embedded


def _identifier(value: str) -> str:
    """An object reference's id, without the comment naming it."""
    match = IDENTIFIER.match(value)
    return match.group(0) if match else ""


def built_targets(scheme: str) -> list[str]:
    """The BlueprintIdentifiers a scheme's BuildAction builds, in order."""
    try:
        tree = ElementTree.parse(scheme)
    except (OSError, ElementTree.ParseError):
        return []
    return [reference.get("BlueprintIdentifier", "")
            for reference in tree.getroot().findall("./BuildAction/BuildActionEntries/BuildActionEntry/BuildableReference")]


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
    embedded: set[str] = set()
    for path in projects:
        apps.update(application_targets(path))
        embedded |= embedded_targets(path)
    shared = [container] + [p for p in projects if p != container]
    app_schemes: dict[str, list[str]] = {}
    for directory in shared:
        for path in sorted(glob.glob(os.path.join(directory, "xcshareddata", "xcschemes", "*.xcscheme"))):
            name, built = os.path.basename(path)[:-len(".xcscheme")], built_targets(path)
            if name in listed and set(built) & set(apps):
                app_schemes[name] = built
    if app_schemes:
        named = next((name for name in names if name in app_schemes), None)
        if named:
            return named, "the shared scheme building the app, named like the project"
        if len(app_schemes) == 1:
            return next(iter(app_schemes)), "the shared scheme building the app"
        # An extension's scheme builds its host app too, but lists the
        # extension first (focus-ios' ContentBlocker), and a helper app's
        # is for an app another embeds. Several schemes for the main app
        # (home-assistant's App-Debug and App-Release, focus-ios' Focus
        # and Klar) are the project's to choose between.
        main = sorted(name for name, built in app_schemes.items() if built[0] in apps and built[0] not in embedded)
        if len(main) == 1:
            return main[0], "the shared scheme for the app no other app embeds"
        return "", f"several shared schemes build the app ({', '.join(main or sorted(app_schemes))})"
    app_names = sorted(set(apps.values()))
    if len(app_names) == 1 and app_names[0] in listed:
        return app_names[0], "the scheme of the project's only app target"
    named = next((name for name in names if name in listed), None)
    if named:
        return named, "the scheme named like the project"
    return "", ""


def language_sources(lang: str) -> list[str]:
    """The language's "detectWithSources" "sources" patterns (*.swift)."""
    try:
        with open(os.path.join(HOME, "languages", lang, "config.json"), encoding="utf-8") as f:
            rule = json.load(f).get("detectWithSources") or {}
    except (OSError, ValueError):
        return []
    return rule.get("sources", [])


def _skipped(name: str) -> bool:
    lowered = name.lower()
    return name.startswith(".") or lowered in NOT_APPS or lowered in project.SKIPPED_DIRS


def containers_below(start: str) -> list[str]:
    """The Xcode projects and workspaces below start (not in it), the
    outermost of each tree, outside hidden and skipped directories, as hct
    _xcode plan's nested lines give them (internal/xcode's Below()): a
    directory scan, not detection's pass over every tracked file, which
    took seconds on firefox-ios."""
    found = []

    def walk(path: str, depth: int) -> None:
        try:
            directories = sorted((entry for entry in os.scandir(path) if entry.is_dir()), key=lambda e: e.name)
        except OSError:
            return
        here = [entry.path for entry in directories if entry.name.endswith(CONTAINERS)]
        if depth > 0 and here:
            found.extend(here)
            return
        if depth < APPS_DEPTH - 1:
            for entry in directories:
                if not entry.name.endswith(CONTAINERS) and not _skipped(entry.name):
                    walk(entry.path, depth + 1)

    walk(start, 0)
    return found


def has_sources(directory: str, patterns: list[str]) -> bool:
    """Whether a file matching one of the patterns is in the directory or
    below it, outside hidden and skipped directories."""
    for _, subdirectories, files in os.walk(directory):
        subdirectories[:] = [name for name in subdirectories if not _skipped(name)]
        if any(fnmatch.fnmatch(name, pattern) for name in files if not name.startswith(".") for pattern in patterns):
            return True
    return False


def apps(lang: str, start: str = ".", containers: list[str] | None = None) -> list[str]:
    """The directories below start holding the containers given (relative
    to start), else those containers_below finds, outside skipped
    directories (an example's app), with the language's sources, as paths
    relative to start."""
    patterns = language_sources(lang)
    below: list[str] = []
    for container in containers_below(start) if containers is None else containers:
        directory = os.path.dirname(os.path.relpath(os.path.join(start, container), start))
        if (directory and directory not in below and not any(_skipped(part) for part in directory.split(os.sep))
                and has_sources(os.path.join(start, directory), patterns)):
            below.append(directory)
    return below


def swift_builds() -> bool:
    """Whether Swift's hooks build this directory's Xcode project: Swift is
    added and enabled, and this is one of its directories, or one of the
    apps below one (firefox-ios' focus-ios, below the root's Package.swift)."""
    model = project.load()
    if "swift" not in model.languages or "swift" in model.disabled:
        return False
    here = os.path.realpath(".")
    directories = [os.path.realpath(d) for _, d in model.impl_dirs("swift")]
    if here in directories:
        return True
    return any(here == os.path.realpath(os.path.join(d, app))
               for d in directories if here.startswith(d + os.sep) for app in apps("swift", d))


def has_app() -> bool:
    """Whether an Xcode project here, or one a workspace here refers to, has
    an application target."""
    projects = glob.glob("*.xcodeproj")
    for workspace in glob.glob("*.xcworkspace"):
        projects += workspace_projects(workspace)
    return any(application_targets(path) for path in projects)


def main(argv: list[str]) -> int:
    if len(argv) >= 2 and argv[0] == "apps":
        for directory in apps(argv[1], ".", argv[2:] or None):
            print("./" + directory)
        return 0
    if len(argv) == 1 and argv[0] == "has-app":
        return 0 if has_app() else 1
    if argv[:1] == ["swift-builds"]:
        return 0 if swift_builds() else 1
    if len(argv) >= 2 and argv[0] == "scheme":
        try:
            listing = json.load(sys.stdin)
        except ValueError:
            listing = {}
        name, why = scheme(listing if isinstance(listing, dict) else {}, argv[1], argv[2] if len(argv) > 2 else "",
                           argv[3:])
        if name or why:
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
