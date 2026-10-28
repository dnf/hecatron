#!/usr/bin/env python3
"""Write each versioned language's release dates (languages/<lang>/releases.json),
which version resolution reads to give an old checkout the toolchain that was
current when it was written (docs/design.md §18). Run it when defaults are
updated; resolution itself stays offline.

    python3 libexec/maintenance/release_dates.py            # every language
    python3 libexec/maintenance/release_dates.py node java  # some

A table is {"source": <url>, "releases": [[version, date, "lts"?], ...]},
final releases only, oldest first; "lts" marks a release that was a long-term
support release when it came out, for languages whose default follows LTS.
"""

from __future__ import annotations

import json
import os
import re
import sys
import time
import urllib.request
from datetime import datetime

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
LANGUAGES = os.path.join(ROOT, "languages")
FINAL = re.compile(r"^\d+(\.\d+)*$")


def fetch(url: str) -> str:
    """A URL's body, retried like hecatron's curl wrapper."""
    for attempt in range(5):
        try:
            request = urllib.request.Request(url, headers={"User-Agent": "hecatron-release-dates"})
            with urllib.request.urlopen(request, timeout=60) as response:
                return response.read().decode("utf-8")
        except OSError:
            if attempt == 4:
                raise
            time.sleep(3)
    raise AssertionError("unreachable")


def node():
    url = "https://nodejs.org/dist/index.json"
    for release in json.loads(fetch(url)):
        yield release["version"].lstrip("v"), release["date"], bool(release["lts"])


def python():
    url = "https://www.python.org/api/v2/downloads/release/?is_published=true"
    for release in json.loads(fetch(url)):
        version = release["name"].removeprefix("Python ").strip()
        if not release["pre_release"] and FINAL.match(version):
            yield version, release["release_date"][:10], False


def go():
    # go.dev/dl lists versions without dates; the release history has both.
    text = fetch("https://go.dev/doc/devel/release")
    for version, date in re.findall(r"go(1\.\d+(?:\.\d+)?)\s*\(released\s+(\d{4}-\d{2}-\d{2})\)", text):
        yield version, date, False


def rust():
    text = fetch("https://raw.githubusercontent.com/rust-lang/rust/master/RELEASES.md")
    for version, date in re.findall(r"^Version (\d+\.\d+\.\d+) \((\d{4}-\d{2}-\d{2})\)", text, re.MULTILINE):
        yield version, date, False


def java():
    # Temurin's releases, as the installer names them ("11.0.13" for
    # jdk-11.0.13+8), dated when Adoptium published them.
    info = json.loads(fetch("https://api.adoptium.net/v3/info/available_releases"))
    lts = set(info["available_lts_releases"])
    for feature in info["available_releases"]:
        page = 0
        while True:
            # The API returns at most 20 a page, whatever page_size asks for.
            url = (f"https://api.adoptium.net/v3/assets/feature_releases/{feature}/ga"
                   f"?os=linux&architecture=x64&image_type=jdk&jvm_impl=hotspot&page_size=20&page={page}")
            try:
                releases = json.loads(fetch(url))
            except OSError:
                break  # past the last page: Adoptium answers 404
            for release in releases:
                name = release["release_name"]
                if not name.startswith("jdk-"):
                    continue  # jdk8u312-b07: the installer can't name Java 8 releases
                version = name.removeprefix("jdk-").split("+")[0]
                if FINAL.match(version):
                    yield version, release["timestamp"][:10], feature in lts
            if len(releases) < 20:
                break
            page += 1


def ruby():
    text = fetch("https://raw.githubusercontent.com/ruby/www.ruby-lang.org/master/_data/releases.yml")
    for version, date in re.findall(r"^- version: ([\d.]+)\s*\n\s+date: '?(\d{4}-\d{2}-\d{2})", text, re.MULTILINE):
        if FINAL.match(version):
            yield version, date, False


def php():
    for major in (5, 7, 8):
        releases = json.loads(fetch(f"https://www.php.net/releases/index.php?json&version={major}&max=1000"))
        for version, release in releases.items():
            if FINAL.match(version):
                yield version, _php_date(release["date"]), False


def _php_date(text: str) -> str:
    # "03 Nov 2022", and in older entries "19 July 2012".
    for layout in ("%d %b %Y", "%d %B %Y"):
        try:
            return datetime.strptime(text.strip(), layout).strftime("%Y-%m-%d")
        except ValueError:
            pass
    raise ValueError(f"unrecognised PHP release date {text!r}")


def dotnet():
    # SDK versions, the .NET languages' version, by release date.
    base = "https://builds.dotnet.microsoft.com/dotnet/release-metadata"
    for channel in json.loads(fetch(f"{base}/releases-index.json"))["releases-index"]:
        lts = channel["release-type"] == "lts"
        for release in json.loads(fetch(f"{base}/{channel['channel-version']}/releases.json"))["releases"]:
            for sdk in release.get("sdks") or [release["sdk"]]:
                if FINAL.match(sdk["version"]):
                    yield sdk["version"], release["release-date"], lts


def elixir():
    page = 1
    while True:
        releases = json.loads(fetch(f"https://api.github.com/repos/elixir-lang/elixir/releases?per_page=100&page={page}"))
        if not releases:
            return
        for release in releases:
            version = release["tag_name"].lstrip("v")
            if not release["prerelease"] and FINAL.match(version) and release.get("published_at"):
                yield version, release["published_at"][:10], False
        page += 1


SOURCES = {
    "typescript": (node, "https://nodejs.org/dist/index.json"),
    "python": (python, "https://www.python.org/api/v2/downloads/release/"),
    "go": (go, "https://go.dev/doc/devel/release"),
    "rust": (rust, "https://github.com/rust-lang/rust/blob/master/RELEASES.md"),
    "java": (java, "https://api.adoptium.net/v3/assets/feature_releases"),
    "ruby": (ruby, "https://github.com/ruby/www.ruby-lang.org/blob/master/_data/releases.yml"),
    "php": (php, "https://www.php.net/releases/"),
    "elixir": (elixir, "https://github.com/elixir-lang/elixir/releases"),
    "csharp": (dotnet, "https://builds.dotnet.microsoft.com/dotnet/release-metadata/releases-index.json"),
}
ALIASES = {"node": "typescript", "dotnet": "csharp"}


def table(lang: str) -> dict:
    generate, url = SOURCES[lang]
    found = {}
    for version, date, lts in generate():
        # A version listed twice keeps its first date.
        if version not in found or date < found[version][0]:
            found[version] = (date, lts)
    key = lambda item: (item[1][0], tuple(int(p) for p in item[0].split(".")))  # noqa: E731
    releases = [[version, date] + (["lts"] if lts else []) for version, (date, lts) in sorted(found.items(), key=key)]
    return {"source": url, "releases": releases}


def write(lang: str, data: dict) -> None:
    path = os.path.join(LANGUAGES, lang, "releases.json")
    with open(path, "w", encoding="utf-8") as f:
        f.write('{\n  "source": ' + json.dumps(data["source"]) + ',\n  "releases": [\n')
        f.write(",\n".join("    " + json.dumps(release) for release in data["releases"]))
        f.write("\n  ]\n}\n")


def main(argv: list[str]) -> int:
    langs = [ALIASES.get(name, name) for name in argv] or list(SOURCES)
    for lang in langs:
        if lang not in SOURCES:
            print(f"release_dates: no release source for {lang} (known: {', '.join(SOURCES)})", file=sys.stderr)
            return 2
        data = table(lang)
        write(lang, data)
        print(f"{lang}: {len(data['releases'])} releases, {data['releases'][0][1]} to {data['releases'][-1][1]}")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
