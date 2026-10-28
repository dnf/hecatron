#!/usr/bin/env python3
"""Check the backlog (projects.json) against the GitHub API and refresh each
project's metadata.

    verify.py            report the projects that fail the criteria
    verify.py --write    also save the fresh metadata and today's date

A project passes when its repository exists under that name, isn't
archived, was pushed to within two years, and is under 1.5 GB. Needs `gh`
(logged in, or GH_TOKEN set).
"""

from __future__ import annotations

import argparse
import datetime
import json
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
PROJECTS = os.path.join(HERE, "projects.json")
MAX_SIZE_KB = 1_500_000
MAX_AGE_DAYS = 730
CRITERIA = ("Exists under this name, not archived, pushed to within two years, "
            "and under 1.5 GB, per the GitHub API (tests/real-projects/verify.py).")


def gh_api(path: str) -> dict:
    out = subprocess.run(["gh", "api", path], check=True, capture_output=True, text=True).stdout
    return json.loads(out)


def metadata(repo: dict, languages: dict, fetched: str) -> dict:
    """A project's "github" entry from repos/O/R and repos/O/R/languages."""
    license_ = repo.get("license") or {}
    return {
        "fetched": fetched,
        "description": repo.get("description"),
        "topics": repo.get("topics") or [],
        "license": license_.get("spdx_id"),
        "stars": repo.get("stargazers_count", 0),
        "forks": repo.get("forks_count", 0),
        "sizeKB": repo.get("size", 0),
        "defaultBranch": repo.get("default_branch", ""),
        "pushedAt": repo.get("pushed_at", ""),
        "archived": bool(repo.get("archived")),
        "primaryLanguage": repo.get("language"),
        "languageBytes": languages,
        "homepage": repo.get("homepage") or None,
    }


def problems(name: str, repo: dict, today: datetime.date) -> list[str]:
    found = []
    if repo.get("full_name", "").lower() != name.lower():
        found.append(f"renamed to {repo.get('full_name')}")
    if repo.get("archived"):
        found.append("archived")
    pushed = datetime.date.fromisoformat(repo["pushed_at"][:10])
    if (today - pushed).days > MAX_AGE_DAYS:
        found.append(f"last pushed {pushed}")
    if repo.get("size", 0) > MAX_SIZE_KB:
        found.append(f"{repo['size'] // 1000} MB")
    return found


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--write", action="store_true", help="save the metadata and the verification date")
    args = parser.parse_args()
    with open(PROJECTS, encoding="utf-8") as f:
        data = json.load(f)
    today = datetime.date.today()
    failed = 0
    for project in data["projects"]:
        name = project["repo"]
        try:
            repo = gh_api(f"repos/{name}")
            languages = gh_api(f"repos/{name}/languages")
        except subprocess.CalledProcessError as e:
            print(f"{name}: {e.stderr.strip() or 'not found'}")
            failed += 1
            continue
        found = problems(name, repo, today)
        if found:
            print(f"{name}: {', '.join(found)}")
            failed += 1
        project["github"] = metadata(repo, languages, today.isoformat())
    if args.write:
        data["verified"] = today.isoformat()
        data["criteria"] = CRITERIA
        with open(PROJECTS, "w", encoding="utf-8") as f:
            json.dump(data, f, indent=1, ensure_ascii=False)
            f.write("\n")
    print(f"{len(data['projects']) - failed} of {len(data['projects'])} projects pass")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
