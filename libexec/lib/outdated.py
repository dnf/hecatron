#!/usr/bin/env python3
"""Outdated dependencies (docs/outdated.md): read a tool's report into rows
of package, current, wanted (the newest the manifest's constraints allow,
where the tool says) and latest, print them and a summary, and save them as
JSON for --json.

    outdated.py report FORMAT FILE LANG COMPONENT [DIRECT]
        # FORMAT: npm, pnpm, yarn-classic, uv-tree, uv-pip, go, cargo, bundler,
        # composer, dotnet, mix, dart, maven, gradle, cabal. DIRECT, for cargo:
        # a `cargo metadata --no-deps` report naming the direct dependencies.
    outdated.py collect DIR                     # every saved row, as one JSON array

HECATRON_OUTDATED_FAIL=1 makes a component with outdated dependencies fail;
HECATRON_OUTDATED_JSON_DIR is where `report` saves its rows.
"""

from __future__ import annotations

import json
import os
import re
import sys


class Row:
    def __init__(self, package: str, current: str, latest: str, wanted: str = ""):
        self.package = package
        self.current = _bare(current)
        self.wanted = _bare(wanted)
        self.latest = _bare(latest)

    def outdated(self) -> bool:
        return bool(self.latest) and self.latest != self.current

    def as_dict(self, lang: str, component: str) -> dict:
        return {"component": component, "language": lang, "package": self.package,
                "current": self.current, "wanted": self.wanted, "latest": self.latest}


def _bare(version) -> str:
    """A version without the v some tools put in front (v1.2.3)."""
    text = str(version or "").strip()
    return text[1:] if re.match(r"^v\d", text) else text


def _json_values(text: str):
    """Each JSON value in a stream of them (go list -json writes one per
    module, pretty-printed back to back)."""
    decoder = json.JSONDecoder()
    index = 0
    while True:
        while index < len(text) and text[index].isspace():
            index += 1
        if index >= len(text):
            return
        value, index = decoder.raw_decode(text, index)
        yield value


def _object(text: str) -> dict:
    """The report, when it's a JSON object; an empty report is nothing
    outdated (npm and pnpm print nothing then)."""
    if not text.strip():
        return {}
    data = json.loads(text)
    if not isinstance(data, dict):
        raise ValueError("not an outdated report")
    return data


# -- the tools' reports ------------------------------------------------------

def parse_npm(text: str, direct=None) -> list[Row]:
    """npm outdated --json, and pnpm outdated --format json: by package
    name. An entry can be a list when several copies are outdated."""
    rows = []
    for name, entries in _object(text).items():
        for entry in entries if isinstance(entries, list) else [entries]:
            rows.append(Row(name, entry.get("current") or "", entry.get("latest") or "", entry.get("wanted") or ""))
    return rows


def parse_yarn_classic(text: str, direct=None) -> list[Row]:
    """yarn outdated --json (Yarn 1): JSON lines, one a table."""
    rows = []
    for line in text.splitlines():
        try:
            value = json.loads(line)
        except ValueError:
            continue
        if isinstance(value, dict) and value.get("type") == "table":
            head = value["data"]["head"]
            for body in value["data"]["body"]:
                entry = dict(zip(head, body))
                rows.append(Row(entry.get("Package", ""), entry.get("Current", ""), entry.get("Latest", ""),
                                entry.get("Wanted", "")))
    return rows


_UV_TREE = re.compile(r"── (\S+) v(\S+).*\(latest: v?([^)\s]+)\)")


def parse_uv_tree(text: str, direct=None) -> list[Row]:
    """uv tree --outdated --depth 1: `├── six v1.15.0 (latest: v1.17.0)`."""
    return [Row(m.group(1), m.group(2), m.group(3)) for m in map(_UV_TREE.search, text.splitlines()) if m]


def parse_uv_pip(text: str, direct=None) -> list[Row]:
    """uv pip list --outdated --format json."""
    data = json.loads(text or "[]")
    if not isinstance(data, list):
        raise ValueError("not an outdated report")
    return [Row(p["name"], p.get("version", ""), p.get("latest_version", "")) for p in data]


def parse_go(text: str, direct=None) -> list[Row]:
    """go list -u -m -json all: the main module's direct requirements with
    an update."""
    rows = []
    for module in _json_values(text):
        if module.get("Main") or module.get("Indirect") or not module.get("Update"):
            continue
        rows.append(Row(module["Path"], module.get("Version", ""), module["Update"].get("Version", "")))
    return rows


_CARGO = re.compile(r"^\s*(Unchanged|Updating|Adding|Locking)\s+(\S+)\s+v(\S+)(?:\s+->\s+v(\S+))?(?:.*\(available: v([^)]+)\))?")


def parse_cargo(text: str, direct=None) -> list[Row]:
    """cargo update --dry-run --verbose: `Unchanged itoa v1.0.1 (available:
    v1.0.18)`, or `Updating serde v1.0.1 -> v1.0.9 (available: v2.0.0)`
    when the lock is behind its requirements. Only the direct dependencies
    count, as for the other ecosystems."""
    names = _cargo_direct(direct)
    rows = []
    for match in map(_CARGO.match, text.splitlines()):
        if not match or (names is not None and match.group(2) not in names):
            continue
        _, name, current, wanted, available = match.groups()
        if wanted or available:
            rows.append(Row(name, current, available or wanted, wanted or ""))
    return rows


def _cargo_direct(path: str | None):
    if not path:
        return None
    with open(path, encoding="utf-8") as f:
        metadata = json.load(f)
    return {dep["name"] for package in metadata.get("packages", []) for dep in package.get("dependencies", [])}


_BUNDLER = re.compile(r"^(\S+) \(newest ([^,]+), installed ([^,)]+)")


def parse_bundler(text: str, direct=None) -> list[Row]:
    """bundle outdated --parseable: `rack (newest 3.1.8, installed 2.2.3,
    requested = 2.2.3)`."""
    return [Row(m.group(1), m.group(3), m.group(2)) for m in map(_BUNDLER.match, text.splitlines()) if m]


def parse_composer(text: str, direct=None) -> list[Row]:
    """composer outdated --format=json: the installed packages, or with
    --locked, the lock file's."""
    data = _object(text)
    return [Row(p["name"], p.get("version", ""), p.get("latest", ""))
            for p in data.get("installed") or data.get("locked") or [] if p.get("latest-status") != "up-to-date"]


def parse_dotnet(text: str, direct=None) -> list[Row]:
    """dotnet list package --outdated --format json: each project's
    top-level packages, once each."""
    rows = {}
    for project in _object(text).get("projects") or []:
        for framework in project.get("frameworks") or []:
            for package in framework.get("topLevelPackages") or []:
                rows.setdefault(package["id"], Row(package["id"], package.get("resolvedVersion", ""),
                                                   package.get("latestVersion", "")))
    return list(rows.values())


def parse_mix(text: str, direct=None) -> list[Row]:
    """mix hex.outdated: a table of Dependency, Only, Current, Latest and
    Status ("Update possible" when the requirement allows the latest)."""
    rows = []
    inside = False
    for line in text.splitlines():
        if line.startswith("Dependency"):
            inside = True
            continue
        parts = re.split(r"\s{2,}", line.strip())
        if not inside or len(parts) < 4:
            continue
        name, current, latest, status = parts[0], parts[-3], parts[-2], parts[-1]
        if status.lower().startswith("up-to-date"):
            continue
        rows.append(Row(name, current, latest, latest if status.lower().startswith("update possible") else ""))
    return rows


def parse_dart(text: str, direct=None) -> list[Row]:
    """dart pub outdated --json: the direct and dev dependencies; wanted is
    pub's "upgradable"."""
    rows = []
    for package in _object(text).get("packages") or []:
        if package.get("kind") == "transitive":
            continue
        version = lambda key: (package.get(key) or {}).get("version", "")  # noqa: E731
        rows.append(Row(package["package"], version("current"), version("latest"), version("upgradable")))
    return rows


_MAVEN = re.compile(r"^(?:\[INFO\])?\s+([\w.\-]+:[\w.\-]+)\s*\.*\s*(?:(\S+)\s+->\s+(\S+))?\s*$")
_MAVEN_VERSIONS = re.compile(r"^(?:\[INFO\])?\s+(\S+)\s+->\s+(\S+)\s*$")


def parse_maven(text: str, direct=None) -> list[Row]:
    """versions:display-dependency-updates: `group:artifact ..... 3.12.0 ->
    3.17.0`, a long name's versions on the next line."""
    rows = []
    pending = None
    for line in text.splitlines():
        match = _MAVEN.match(line)
        if match and match.group(2):
            rows.append(Row(match.group(1), match.group(2), match.group(3)))
            pending = None
        elif match:
            pending = match.group(1)
        elif pending and (versions := _MAVEN_VERSIONS.match(line)):
            rows.append(Row(pending, versions.group(1), versions.group(2)))
            pending = None
        else:
            pending = None
    return rows


def parse_gradle(text: str, direct=None) -> list[Row]:
    """The Gradle Versions plugin's report.json (dependencyUpdates)."""
    rows = []
    for dep in (_object(text).get("outdated") or {}).get("dependencies") or []:
        available = dep.get("available") or {}
        latest = available.get("release") or available.get("milestone") or available.get("integration") or ""
        rows.append(Row(f"{dep.get('group')}:{dep.get('name')}", dep.get("version", ""), latest))
    return rows


_CABAL = re.compile(r"^(\S+) (.+?) \(latest: ([^)]+)\)")


def parse_cabal(text: str, direct=None) -> list[Row]:
    """cabal outdated: `base >=4 && <4.15 (latest: 4.18.0.0)`; the current
    is the constraint, which cabal reports against."""
    return [Row(m.group(1), m.group(2), m.group(3)) for m in map(_CABAL.match, text.splitlines()) if m]


PARSERS = {
    "npm": parse_npm,
    "pnpm": parse_npm,
    "yarn-classic": parse_yarn_classic,
    "uv-tree": parse_uv_tree,
    "uv-pip": parse_uv_pip,
    "go": parse_go,
    "cargo": parse_cargo,
    "bundler": parse_bundler,
    "composer": parse_composer,
    "dotnet": parse_dotnet,
    "mix": parse_mix,
    "dart": parse_dart,
    "maven": parse_maven,
    "gradle": parse_gradle,
    "cabal": parse_cabal,
}


def parse(fmt: str, text: str, direct: str | None = None) -> list[Row]:
    unique = {}
    for row in PARSERS[fmt](text, direct):
        if row.outdated():
            unique.setdefault((row.package, row.current), row)
    return sorted(unique.values(), key=lambda r: r.package.lower())


# -- output ------------------------------------------------------------------

def table(rows: list[Row]) -> list[str]:
    if not rows:
        return []
    header = ("package", "current", "wanted", "latest")
    cells = [header] + [(r.package, r.current or "-", r.wanted or "-", r.latest) for r in rows]
    widths = [max(len(c[i]) for c in cells) for i in range(3)]
    return ["  " + "  ".join(c[i].ljust(widths[i]) for i in range(3)) + "  " + c[3] for c in cells]


def report(fmt: str, text: str, lang: str, component: str, direct: str | None = None,
           json_dir: str | None = None, fail: bool = False) -> int:
    rows = parse(fmt, text, direct)
    for line in table(rows):
        print(line)
    noun = "dependency" if len(rows) == 1 else "dependencies"
    print(f"{component}: {lang} {len(rows)} outdated {noun}")
    if json_dir:
        os.makedirs(json_dir, exist_ok=True)
        name = f"{lang}-{re.sub(r'[^A-Za-z0-9_.-]', '_', component)}.json"
        with open(os.path.join(json_dir, name), "w", encoding="utf-8") as f:
            json.dump([r.as_dict(lang, component) for r in rows], f)
    return 1 if fail and rows else 0


def collect(directory: str) -> list[dict]:
    rows = []
    for name in sorted(os.listdir(directory)):
        if name.endswith(".json"):
            with open(os.path.join(directory, name), encoding="utf-8") as f:
                rows.extend(json.load(f))
    return rows


def main(argv: list[str]) -> int:
    if len(argv) in (5, 6) and argv[0] == "report":
        fmt, path, lang, component = argv[1:5]
        if fmt not in PARSERS:
            print(f"outdated.py: unknown format {fmt!r}", file=sys.stderr)
            return 2
        with open(path, encoding="utf-8", errors="replace") as f:
            text = f.read()
        try:
            return report(fmt, text, lang, component, argv[5] if len(argv) == 6 else None,
                          os.environ.get("HECATRON_OUTDATED_JSON_DIR") or None,
                          os.environ.get("HECATRON_OUTDATED_FAIL") == "1")
        except (ValueError, KeyError, TypeError, AttributeError) as error:
            print(f"error: {lang}: couldn't read the {fmt} report ({error}); its output was:", file=sys.stderr)
            print(text[:4000], file=sys.stderr)
            return 1
    if len(argv) == 2 and argv[0] == "collect":
        print(json.dumps(collect(argv[1]), indent=2))
        return 0
    print(__doc__, file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
