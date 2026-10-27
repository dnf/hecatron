#!/usr/bin/env python3
"""Check that every external link in hecatron's documentation still works:
the tracked Markdown files (README, docs/, CHANGELOG...), llms.txt, the
tool links docs/languages.md is built from, and the documentation each
advisor rule cites (languages/*/advice.json, languages/_advice.json).

    python3 libexec/maintenance/check_links.py              # report; exit 1 if any is broken
    python3 libexec/maintenance/check_links.py --markdown   # the report as Markdown, for an issue

A link is broken when its page is gone (404, 410 and other client errors),
its host can't be reached, or its server keeps failing (5xx) or timing out
after retries. Answers that only refuse automated clients (401, 402, 403,
429: npmjs.com and metacpan.org do) aren't counted: the page is there for
a person, and reporting them would file the same false ticket every week.
"""

from __future__ import annotations

import concurrent.futures
import json
import os
import re
import subprocess
import sys
import time
import urllib.error
import urllib.request

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
TOOL_LINKS = os.path.join("libexec", "codegen", "tool-links.json")
ADVICE = ["languages/*/advice.json", "languages/_advice.json"]
URL = re.compile(r"https?://[^\s<>()\[\]`'\"|]+")
# Placeholders and examples in the docs, not links to check.
SKIP = re.compile(r"[{}$]|//(localhost|127\.0\.0\.1|example\.(com|org)|[^/]*\.example)(/|:|$)")
BLOCKED = {401, 402, 403, 429}
USER_AGENT = "Mozilla/5.0 (compatible; hecatron-link-check; +https://github.com/dnf/hecatron)"


def documents() -> list[str]:
    tracked = subprocess.run(["git", "ls-files", "*.md", "llms.txt", TOOL_LINKS, *ADVICE], cwd=ROOT,
                             capture_output=True, text=True, check=True).stdout.split()
    return [path for path in tracked if not path.startswith(("examples/", "tests/"))]


def links(paths: list[str]) -> dict[str, list[str]]:
    """Each URL, with where it appears (path:line)."""
    found: dict[str, list[str]] = {}
    for path in paths:
        with open(os.path.join(ROOT, path), encoding="utf-8") as f:
            for number, line in enumerate(f, 1):
                for match in URL.finditer(line):
                    url = match.group(0).rstrip(".,;:!?*_")
                    if not SKIP.search(url):
                        found.setdefault(url, []).append(f"{path}:{number}")
    return found


def verdict(status: int | None) -> str:
    """"ok", "blocked" (refuses automated clients) or "broken", from an HTTP
    status, or None when the host couldn't be reached."""
    if status is None:
        return "broken"
    if status < 400:
        return "ok"
    if status in BLOCKED:
        return "blocked"
    return "broken"


def fetch(url: str) -> tuple[int | None, str]:
    """The final status (after redirects) and a short description, retried
    on server errors and network failures."""
    detail = ""
    for attempt in range(3):
        try:
            request = urllib.request.Request(url, headers={"User-Agent": USER_AGENT})
            with urllib.request.urlopen(request, timeout=30) as response:
                return response.status, str(response.status)
        except urllib.error.HTTPError as error:
            if error.code < 500 or error.code == 501:
                return error.code, str(error.code)
            detail = str(error.code)
        except (urllib.error.URLError, OSError) as error:
            detail = str(getattr(error, "reason", error))
        time.sleep(5 * (attempt + 1))
    return (int(detail) if detail.isdigit() else None), detail


def main(argv: list[str]) -> int:
    found = links(documents())
    with concurrent.futures.ThreadPoolExecutor(8) as pool:
        results = dict(zip(found, pool.map(fetch, found)))
    broken = {url: detail for url, (status, detail) in results.items() if verdict(status) == "broken"}
    if "--markdown" in argv:
        print(f"{len(broken)} of the {len(found)} external links in the docs don't work "
              f"(`libexec/maintenance/check_links.py`):\n")
        for url, detail in sorted(broken.items()):
            print(f"- {url}: {detail}, in " + ", ".join(f"`{where}`" for where in found[url]))
    else:
        for url, detail in sorted(broken.items()):
            print(f"{detail}\t{url}\t{' '.join(found[url])}")
        print(f"{len(found)} links checked, {len(broken)} broken", file=sys.stderr)
    return 1 if broken else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
