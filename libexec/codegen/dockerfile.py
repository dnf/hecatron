#!/usr/bin/env python3
"""Generate a project's Dockerfile from its languages' dockerfile snippets.

Collects `# apt:` packages (deduplicated into one apt-get install), honours
`# after: <lang>` ordering, and copies each snippet's RUN/ENV blocks,
repeating an `# each-version` block once per toolchain version. Called
by `hecatron dockerfile` (manage.py), which writes config.json's
generated.dockerfile path.
"""

from __future__ import annotations

import os
import sys
from collections import defaultdict

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from banner import make_banner  # noqa: E402

# Always included regardless of active languages: build tools, what snippets
# download with, and what hecatron itself runs on (python3 for the CLI, perl's
# Time::HiRes for the parallel runner's timing; neither is in the base image).
BASE_PACKAGES = [
    "build-essential",
    "ca-certificates",
    "curl",
    "git",
    "gnupg",
    "perl",
    "python3",
    "unzip",
]


def parse_snippet(project, lang: str) -> tuple[list[str], list[str], str | None]:
    """Parse a language's dockerfile snippet (the project's, else the built-in one).

    Returns (apt_packages, body_lines, after_constraint).
    - apt_packages: list of package names from # apt: directives
    - body_lines: all non-directive lines (RUN, ENV, comments, blanks)
    - after_constraint: language name from # after: directive, or None
    """
    snippet_path = project.resolve(lang, "dockerfile")
    if snippet_path is None:
        return [], [], None
    apt_packages: list[str] = []
    body_lines: list[str] = []
    after: str | None = None
    with open(snippet_path) as f:
        for line in f:
            stripped = line.rstrip("\n")
            if stripped.startswith("# apt:"):
                apt_packages.extend(stripped[len("# apt:"):].strip().split())
            elif stripped.startswith("# after:"):
                after = stripped[len("# after:"):].strip()
            else:
                body_lines.append(stripped)
    return apt_packages, expand_versions(project, lang, body_lines, snippet_path), after


EACH_VERSION = "# each-version"
END_EACH_VERSION = "# end-each-version"


def block_versions(project, lang: str) -> list[str]:
    """The specs an each-version block repeats for: the definition's default
    (what the snippet's ENV lines point at), then every other spec the
    components resolve to, in component order. None (a pin naming no version)
    needs nothing beyond the default."""
    specs = [project.default_version(lang)]
    specs += [spec for spec, _ in project.versions(lang)]
    return list(dict.fromkeys(spec for spec in specs if spec is not None))


def fill_version(line: str, spec: str) -> str:
    """A block line with {version} (the spec without "="), {exact} ("1" for an
    exact spec, else ""), {major} and {minor} filled in. Only these four, so
    shell's ${VAR} and other braces pass through."""
    version = spec.lstrip("=")
    parts = version.split(".")
    values = {
        "version": version,
        "exact": "1" if spec.startswith("=") else "",
        "major": parts[0],
        "minor": parts[1] if len(parts) > 1 else "",
    }
    for name, value in values.items():
        line = line.replace("{" + name + "}", value)
    return line


def expand_versions(project, lang: str, lines: list[str], source: str) -> list[str]:
    """Snippet lines with each `# each-version [LANG]` ... `# end-each-version`
    block written once per spec of LANG (the snippet's own language if none
    is named; Kotlin's snippet repeats over Java's JDKs)."""
    out: list[str] = []
    block: list[str] | None = None
    block_lang = lang
    for line in lines:
        if line == EACH_VERSION or line.startswith(EACH_VERSION + " "):
            if block is not None:
                raise ValueError(f"{source}: each-version blocks don't nest")
            block, block_lang = [], line[len(EACH_VERSION):].strip() or lang
        elif line == END_EACH_VERSION:
            if block is None:
                raise ValueError(f"{source}: {END_EACH_VERSION} without {EACH_VERSION}")
            for spec in block_versions(project, block_lang):
                out.extend(fill_version(block_line, spec) for block_line in block)
            block = None
        elif block is not None:
            block.append(line)
        else:
            out.append(line)
    if block is not None:
        raise ValueError(f"{source}: {EACH_VERSION} without {END_EACH_VERSION}")
    return out


def has_body(lines: list[str]) -> bool:
    """Check if body lines contain any RUN/ENV instructions."""
    return any(
        line.startswith("RUN ") or line.startswith("ENV ")
        for line in lines
    )


def topological_sort(
    languages: list[str],
    after_map: dict[str, str | None],
    active_set: set[str],
) -> list[str]:
    """Sort languages respecting # after: constraints, falling back to alphabetical.

    Constraints referencing inactive languages are ignored.
    """
    # Build adjacency: lang must come after its dependency
    graph: dict[str, list[str]] = defaultdict(list)
    in_degree: dict[str, int] = {lang: 0 for lang in languages}

    for lang in languages:
        dep = after_map.get(lang)
        if dep and dep in active_set and dep in in_degree:
            graph[dep].append(lang)
            in_degree[lang] += 1

    # Kahn's algorithm with alphabetical tie-breaking
    queue = sorted(lang for lang in languages if in_degree[lang] == 0)
    result: list[str] = []

    while queue:
        current = queue.pop(0)
        result.append(current)
        for neighbor in sorted(graph[current]):
            in_degree[neighbor] -= 1
            if in_degree[neighbor] == 0:
                # Insert in sorted position
                idx = 0
                while idx < len(queue) and queue[idx] < neighbor:
                    idx += 1
                queue.insert(idx, neighbor)

    return result


def generate(project) -> str:
    """Generate the complete Dockerfile content for a project."""
    active = project.languages
    active_set = set(active)

    # Parse all snippets
    all_apt: list[str] = list(BASE_PACKAGES)
    body_map: dict[str, list[str]] = {}
    after_map: dict[str, str | None] = {}

    for lang in active:
        apt_pkgs, body, after = parse_snippet(project, lang)
        all_apt.extend(apt_pkgs)
        body_map[lang] = body
        after_map[lang] = after

    # Deduplicate and sort apt packages
    apt_packages = sorted(set(all_apt))

    # Filter to languages that have RUN/ENV blocks
    langs_with_body = [lang for lang in active if has_body(body_map[lang])]

    # Topological sort for emit order
    sorted_langs = topological_sort(langs_with_body, after_map, active_set)

    # Build output
    lines: list[str] = []

    # Banner
    banner = make_banner([
        "Generated by: hecatron dockerfile",
        "from each language's dockerfile snippet",
        "",
        "To regenerate: hecatron dockerfile",
        "Regenerated automatically by: hecatron add / remove",
    ])
    lines.append(banner.rstrip("\n"))
    lines.append("")

    # FROM
    lines.append("FROM ubuntu:24.04")
    lines.append("")
    lines.append('ENV DEBIAN_FRONTEND=noninteractive')
    # The image ships the C.UTF-8 locale but doesn't select it, and tools
    # writing non-ASCII to stdout (GHC's runtime, R) fail under POSIX.
    lines.append('ENV LANG=C.UTF-8')
    lines.append("")

    # Apt batch install
    lines.append("# System packages + language runtimes available from distro")
    lines.append("RUN apt-get update && apt-get install -y --no-install-recommends \\")
    for pkg in apt_packages:
        lines.append(f"    {pkg} \\")
    lines.append("    && rm -rf /var/lib/apt/lists/*")
    lines.append("")

    # Language-specific RUN/ENV blocks
    for lang in sorted_langs:
        body = body_map[lang]
        # Strip leading/trailing blank lines from body
        while body and body[0] == "":
            body.pop(0)
        while body and body[-1] == "":
            body.pop()
        if body:
            for line in body:
                lines.append(line)
            lines.append("")

    # just (task runner), when the project has a justfile
    if os.path.isfile(os.path.join(project.root, "justfile")):
        lines.append("# just (task runner)")
        lines.append(
            "RUN curl --proto '=https' --tlsv1.2 -sSf https://just.systems/install.sh"
            " | bash -s -- --to /usr/local/bin"
        )
        lines.append("")
    lines.append("WORKDIR /workspace")
    lines.append("")
    lines.append('CMD ["bash"]')
    lines.append("")

    return "\n".join(lines)
