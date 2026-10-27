#!/usr/bin/env python3
"""Dependency audit findings (docs/audit.md): read an audit tool's report,
apply the project's accepted risks, print the findings and a summary, and
fail when one is at or above the threshold.

    audit.py report FORMAT FILE LANG COMPONENT   # FORMAT: npm, yarn-classic, yarn-berry,
                                                 # bun, pip-audit, govulncheck, composer,
                                                 # bundle-audit, dotnet, osv
    audit.py eol LANG VERSION < releases         # warn when VERSION's line is past its end of life

The threshold is HECATRON_AUDIT_FAIL_ON (critical, high, moderate or low;
default high). A finding whose tool gives no severity counts as high.
Accepted risks are .hecatron/config.json's "audit": {"ignore": [{"id",
"reason", "until"}]}; an entry past its "until" date no longer applies.
"""

from __future__ import annotations

import datetime
import json
import os
import re
import sys

SEVERITIES = ("critical", "high", "moderate", "low")
_ALIASES = {"medium": "moderate", "important": "high", "info": "low", "informational": "low", "none": "low"}
_ADVISORY_ID = re.compile(r"\b(GHSA(?:-[23456789cfghjmpqrvwx]{4}){3}|CVE-\d{4}-\d{4,}|RUSTSEC-\d{4}-\d{4}|GO-\d{4}-\d{4,}|PYSEC-\d{4}-\d+)\b")


class Finding:
    def __init__(self, id: str, package: str, version: str = "", severity: str | None = None,
                 title: str = "", aliases: list[str] | None = None, url: str = ""):
        self.id = id
        self.package = package
        self.version = version
        self.severity = normalize_severity(severity)
        self.title = title
        self.aliases = sorted({a for a in (aliases or []) if a and a != id})
        self.url = url

    @property
    def ids(self) -> set[str]:
        return {self.id, *self.aliases}

    def rank(self) -> int:
        """0 for critical ... 3 for low; unknown ranks as high."""
        return SEVERITIES.index(self.severity) if self.severity else 1

    def key(self) -> tuple:
        return (self.id, self.package, self.version)


def normalize_severity(value) -> str | None:
    if value is None:
        return None
    text = str(value).strip().lower()
    text = _ALIASES.get(text, text)
    if text in SEVERITIES:
        return text
    try:
        score = float(text)
    except ValueError:
        return None
    return cvss_severity(score)


def cvss_severity(score: float) -> str | None:
    if score >= 9.0:
        return "critical"
    if score >= 7.0:
        return "high"
    if score >= 4.0:
        return "moderate"
    if score > 0:
        return "low"
    return None


def _first_sentence(text: str) -> str:
    """An advisory's description cut to its first line of prose (pip-audit
    gives no title), skipping Markdown headings."""
    for line in (text or "").splitlines():
        line = line.strip()
        if line and not line.startswith("#"):
            return line if len(line) <= 100 else line[:97].rstrip() + "..."
    return ""


def advisory_ids(*texts: str) -> list[str]:
    found = []
    for text in texts:
        for match in _ADVISORY_ID.findall(text or ""):
            if match not in found:
                found.append(match)
    return found


def _json_values(text: str):
    """Each JSON value in a stream of them (one per line, or pretty-printed
    back to back, as govulncheck writes)."""
    decoder = json.JSONDecoder()
    index = 0
    while True:
        while index < len(text) and text[index].isspace():
            index += 1
        if index >= len(text):
            return
        value, index = decoder.raw_decode(text, index)
        yield value


# -- the tools' reports ------------------------------------------------------

def _expect(data, *keys: str) -> dict:
    """The report, when it's a JSON object with one of the keys the tool
    always writes; anything else (an error message, nothing) is no report."""
    if not isinstance(data, dict) or not any(key in data for key in keys):
        raise ValueError("not an audit report")
    return data


def parse_npm(text: str) -> list[Finding]:
    """npm audit --json (npm 7 and later), or the older advisories layout
    pnpm audit --json still writes."""
    data = _expect(json.loads(text), "vulnerabilities", "advisories", "auditReportVersion")
    if "advisories" in data:
        return _npm_advisories(data["advisories"].values())
    findings = []
    for name, entry in (data.get("vulnerabilities") or {}).items():
        for via in entry.get("via") or []:
            # A string names another vulnerable package this one depends on;
            # that package's own entry holds the advisory.
            if not isinstance(via, dict):
                continue
            ids = advisory_ids(via.get("url", ""))
            # The range is the advisory's, not the installed version (npm's
            # report doesn't give that).
            findings.append(Finding(ids[0] if ids else str(via.get("source")), via.get("name") or name,
                                    "", via.get("severity"), via.get("title", ""), ids[1:], via.get("url", "")))
    return findings


def _npm_advisories(advisories) -> list[Finding]:
    findings = []
    for advisory in advisories:
        ghsa = advisory.get("github_advisory_id") or ""
        aliases = list(advisory.get("cves") or [])
        versions = sorted({f.get("version", "") for f in advisory.get("findings") or []} - {""})
        findings.append(Finding(ghsa or str(advisory.get("id")), advisory.get("module_name", ""),
                                ", ".join(versions), advisory.get("severity"), advisory.get("title", ""),
                                aliases, advisory.get("url", "")))
    return findings


def parse_yarn_classic(text: str) -> list[Finding]:
    """yarn audit --json (Yarn 1): one JSON object a line."""
    advisories = [value["data"]["advisory"] for value in _json_values(text)
                  if isinstance(value, dict) and value.get("type") == "auditAdvisory"]
    unique = {a.get("id"): a for a in advisories}
    return _npm_advisories(unique.values())


def parse_yarn_berry(text: str) -> list[Finding]:
    """yarn npm audit --json (Yarn 2 and later): one object a line, a package
    and its advisory's fields."""
    findings = []
    for value in _json_values(text):
        if not isinstance(value, dict) or not isinstance(value.get("children"), dict):
            continue
        child = value["children"]
        url = str(child.get("URL", ""))
        ids = advisory_ids(url) or [str(child.get("ID", ""))]
        versions = child.get("Tree Versions") or []
        findings.append(Finding(ids[0], str(value.get("value", "")), ", ".join(map(str, versions)),
                                child.get("Severity"), str(child.get("Issue", "")), ids[1:], url))
    return findings


def parse_bun(text: str) -> list[Finding]:
    """bun audit --json: each package's advisories, as npm's bulk advisory
    endpoint returns them."""
    data = json.loads(text)
    findings = []
    for name, advisories in data.items():
        for advisory in advisories if isinstance(advisories, list) else []:
            url = advisory.get("url", "")
            ids = advisory_ids(url) or [str(advisory.get("id"))]
            findings.append(Finding(ids[0], name, advisory.get("vulnerable_versions", ""),
                                    advisory.get("severity"), advisory.get("title", ""), ids[1:], url))
    return findings


def parse_pip_audit(text: str) -> list[Finding]:
    """pip-audit -f json: no severities, so each counts as high."""
    dependencies = _expect(json.loads(text), "dependencies")["dependencies"]
    findings = []
    for dependency in dependencies:
        for vuln in dependency.get("vulns") or []:
            findings.append(Finding(vuln["id"], dependency.get("name", ""), dependency.get("version", ""),
                                    None, _first_sentence(vuln.get("description", "")), vuln.get("aliases") or []))
    return findings


def parse_govulncheck(text: str) -> list[Finding]:
    """govulncheck -json: the vulnerabilities the code calls (a finding whose
    trace reaches a function), as govulncheck itself reports by default.
    The Go database has no severities, so each counts as high."""
    entries = {}
    called = {}
    values = list(_json_values(text))
    if not any(isinstance(value, dict) and "config" in value for value in values):
        raise ValueError("not a govulncheck report")
    for value in values:
        if not isinstance(value, dict):
            continue
        if "osv" in value and isinstance(value["osv"], dict):
            entries[value["osv"]["id"]] = value["osv"]
        finding = value.get("finding")
        if isinstance(finding, dict):
            trace = finding.get("trace") or [{}]
            if trace[0].get("function"):
                called.setdefault(finding["osv"], trace[0])
    findings = []
    for osv_id, frame in sorted(called.items()):
        entry = entries.get(osv_id, {})
        findings.append(Finding(osv_id, frame.get("module", ""), frame.get("version", ""), None,
                                entry.get("summary", ""), entry.get("aliases") or []))
    return findings


def parse_composer(text: str) -> list[Finding]:
    """composer audit --format=json."""
    data = _expect(json.loads(text), "advisories")
    advisories = data.get("advisories") or {}
    findings = []
    for name, entries in advisories.items():
        for entry in entries.values() if isinstance(entries, dict) else entries:
            ids = advisory_ids(entry.get("advisoryId", ""), entry.get("link", ""))
            primary = ids[0] if ids else str(entry.get("advisoryId"))
            aliases = [entry.get("cve")] + ids[1:] if entry.get("cve") else ids[1:]
            findings.append(Finding(primary, name, entry.get("affectedVersions", ""), entry.get("severity"),
                                    entry.get("title", ""), aliases, entry.get("link", "")))
    return findings


def parse_bundle_audit(text: str) -> list[Finding]:
    """bundle-audit check's text report: a block of "Name:", "Version:",
    "CVE:" or "GHSA:", "Criticality:"... lines per advisory."""
    if not re.search(r"^(No vulnerabilities found|Vulnerabilities found!|Name:)", text, re.MULTILINE):
        raise ValueError("not a bundle-audit report")
    findings = []
    block: dict[str, str] = {}

    def flush():
        if block.get("name"):
            ids = advisory_ids(block.get("cve", ""), block.get("ghsa", ""), block.get("advisory", ""),
                               block.get("url", ""))
            primary = ids[0] if ids else block.get("advisory") or block.get("name")
            severity = block.get("criticality")
            findings.append(Finding(primary, block["name"], block.get("version", ""),
                                    None if severity in (None, "Unknown") else severity,
                                    block.get("title", ""), ids[1:], block.get("url", "")))
        block.clear()

    for line in text.splitlines():
        match = re.match(r"^(Name|Version|CVE|GHSA|Advisory|Criticality|URL|Title|Solution):\s*(.*)$", line)
        if not match:
            continue
        field, value = match.group(1).lower(), match.group(2).strip()
        if field == "name":
            flush()
        block[field] = value
    flush()
    return findings


def parse_dotnet(text: str) -> list[Finding]:
    """dotnet list package --vulnerable --include-transitive --format json."""
    data = _expect(json.loads(text), "projects", "version")
    findings = []
    for project in data.get("projects") or []:
        for framework in project.get("frameworks") or []:
            for kind in ("topLevelPackages", "transitivePackages"):
                for package in framework.get(kind) or []:
                    for vuln in package.get("vulnerabilities") or []:
                        url = vuln.get("advisoryurl", "")
                        ids = advisory_ids(url) or [url]
                        findings.append(Finding(ids[0], package.get("id", ""), package.get("resolvedVersion", ""),
                                                vuln.get("severity"), "", ids[1:], url))
    return findings


def parse_osv(text: str) -> list[Finding]:
    """osv-scanner --format json: one finding per group of aliases."""
    data = _expect(json.loads(text), "results") if text.strip() else {}
    findings = []
    for result in data.get("results") or []:
        for package in result.get("packages") or []:
            info = package.get("package") or {}
            vulns = {v["id"]: v for v in package.get("vulnerabilities") or []}
            for group in package.get("groups") or [{"ids": [i]} for i in vulns]:
                ids = list(group.get("ids") or [])
                primary = next((i for i in ids if i.startswith("GHSA-")), ids[0])
                severity = normalize_severity(group.get("max_severity")) if group.get("max_severity") else None
                vuln = vulns.get(primary) or vulns.get(ids[0]) or {}
                if severity is None:
                    severity = normalize_severity((vuln.get("database_specific") or {}).get("severity"))
                aliases = ids + list(group.get("aliases") or []) + list(vuln.get("aliases") or [])
                findings.append(Finding(primary, info.get("name", ""), info.get("version", ""), severity,
                                        vuln.get("summary", ""), aliases))
    return findings


PARSERS = {
    "npm": parse_npm,
    "pnpm": parse_npm,
    "yarn-classic": parse_yarn_classic,
    "yarn-berry": parse_yarn_berry,
    "bun": parse_bun,
    "pip-audit": parse_pip_audit,
    "govulncheck": parse_govulncheck,
    "composer": parse_composer,
    "bundle-audit": parse_bundle_audit,
    "dotnet": parse_dotnet,
    "osv": parse_osv,
}


def parse(fmt: str, text: str) -> list[Finding]:
    unique = {}
    for finding in PARSERS[fmt](text):
        unique.setdefault(finding.key(), finding)
    return sorted(unique.values(), key=lambda f: (f.rank(), f.package, f.id))


# -- accepted risks ----------------------------------------------------------

def config_problems(config: dict) -> list[str]:
    """What's wrong with config.json's "audit" section, for hecatron check."""
    audit = config.get("audit")
    if audit is None:
        return []
    if not isinstance(audit, dict):
        return ['"audit" must be an object']
    problems = []
    for index, entry in enumerate(audit.get("ignore") or []):
        where = f"audit.ignore[{index}]"
        if not isinstance(entry, dict) or not entry.get("id"):
            problems.append(f"{where} needs an \"id\"")
            continue
        where = f"audit.ignore {entry['id']}"
        if not str(entry.get("reason", "")).strip():
            problems.append(f"{where} needs a \"reason\"")
        if not re.match(r"^\d{4}-\d{2}-\d{2}$", str(entry.get("until", ""))):
            problems.append(f"{where} needs an \"until\" date (YYYY-MM-DD)")
    fail_on = audit.get("failOn")
    if fail_on is not None and fail_on not in SEVERITIES:
        problems.append(f"audit.failOn must be one of {', '.join(SEVERITIES)}")
    return problems


def split_ignored(findings: list[Finding], ignore: list[dict], today: str):
    """(kept, ignored, expired): findings an entry accepts until its date, and
    those an expired entry no longer covers, with the entry."""
    kept, ignored, expired = [], [], []
    for finding in findings:
        entry = next((e for e in ignore if isinstance(e, dict) and e.get("id") in finding.ids), None)
        if entry is None:
            kept.append(finding)
        elif str(entry.get("until", "")) >= today:
            ignored.append((finding, entry))
        else:
            expired.append((finding, entry))
            kept.append(finding)
    return kept, ignored, expired


def threshold(config: dict) -> str:
    value = os.environ.get("HECATRON_AUDIT_FAIL_ON") or (config.get("audit") or {}).get("failOn") or "high"
    if value not in SEVERITIES:
        raise ValueError(f"--fail-on must be one of {', '.join(SEVERITIES)}, not {value!r}")
    return value


def summary(findings: list[Finding]) -> str:
    counts = {s: 0 for s in SEVERITIES}
    unknown = 0
    for finding in findings:
        if finding.severity:
            counts[finding.severity] += 1
        else:
            unknown += 1
    parts = [f"{s} {counts[s]}" for s in SEVERITIES]
    if unknown:
        parts.append(f"unrated {unknown}")
    noun = "vulnerability" if len(findings) == 1 else "vulnerabilities"
    return f"{len(findings)} {noun} ({', '.join(parts)})"


def _line(finding: Finding) -> str:
    severity = finding.severity or "unrated"
    version = f" {finding.version}" if finding.version else ""
    aliases = f" ({', '.join(finding.aliases[:3])})" if finding.aliases else ""
    title = f": {finding.title}" if finding.title else ""
    return f"  {severity:<9} {finding.package}{version}  {finding.id}{aliases}{title}"


def report(fmt: str, text: str, lang: str, component: str, config: dict, today: str) -> int:
    findings = parse(fmt, text)
    fail_on = threshold(config)
    kept, ignored, expired = split_ignored(findings, (config.get("audit") or {}).get("ignore") or [], today)
    for finding in kept:
        print(_line(finding))
    for finding, entry in ignored:
        print(_line(finding) + f"  [accepted until {entry.get('until')}: {entry.get('reason', '')}]")
    for finding, entry in expired:
        print(f"  note: the accepted risk {entry.get('id')} expired on {entry.get('until')}")
    line = f"{component}: {lang} {summary(kept)}"
    if ignored:
        line += f", {len(ignored)} accepted"
    print(line)
    limit = SEVERITIES.index(fail_on)
    failing = [f for f in kept if f.rank() <= limit]
    if failing:
        print(f"{component}: {lang}: {len(failing)} at or above {fail_on}")
        return 1
    return 0


def eol_warning(lang: str, version: str, releases: str, line_parts: int | None = None) -> str | None:
    """A warning when the installed version's line is past its end of life,
    from a definition's releases.sh output (version<TAB>flags, the newest
    release of each line). A line is the definition's "releaseLine" leading
    parts (1 for Node.js: 16 for 16.20.2), else all but the last (3.9 for
    3.9.25), as for versions (docs/design.md §18), and never the whole
    version: .NET's releaseLine of 3 names a feature band, but support ends
    by channel (10.0)."""
    parts = version.lstrip("v").split(".")
    for line in releases.splitlines():
        latest, _, flags = line.partition("\t")
        latest_parts = latest.split(".")
        whole = max(len(latest_parts) - 1, 1)
        prefix = latest_parts[:min(line_parts or whole, whole)]
        if parts[:len(prefix)] == prefix:
            if "eol" in flags.split(","):
                return f"warning: {lang} {version} is past its end of life (line {'.'.join(prefix)}); upgrade it"
            return None
    return None


def _release_line_parts(lang: str) -> int | None:
    path = os.path.join(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))),
                        "languages", lang, "config.json")
    try:
        with open(path, encoding="utf-8") as f:
            return json.load(f).get("releaseLine")
    except (OSError, ValueError):
        return None


def _project_config() -> dict:
    sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
    import project as projectlib  # noqa: E402
    try:
        root = projectlib.find_root(os.environ.get("HECATRON_ROOT") or None)
    except Exception:  # noqa: BLE001 - outside a project there's nothing to apply
        return {}
    path = os.path.join(root, ".hecatron", "config.json")
    try:
        with open(path, encoding="utf-8") as f:
            return json.load(f)
    except (OSError, ValueError):
        return {}


def main(argv: list[str]) -> int:
    if len(argv) == 5 and argv[0] == "report":
        fmt, path, lang, component = argv[1:]
        if fmt not in PARSERS:
            print(f"audit.py: unknown format {fmt!r}", file=sys.stderr)
            return 2
        with open(path, encoding="utf-8", errors="replace") as f:
            text = f.read()
        try:
            return report(fmt, text, lang, component, _project_config(), datetime.date.today().isoformat())
        except (ValueError, KeyError, TypeError, AttributeError) as error:
            print(f"error: {lang}: couldn't read the {fmt} report ({error}); its output was:", file=sys.stderr)
            print(text[:4000], file=sys.stderr)
            return 1
    if len(argv) == 3 and argv[0] == "eol":
        warning = eol_warning(argv[1], argv[2], sys.stdin.read(), _release_line_parts(argv[1]))
        if warning:
            print(warning)
        return 0
    print(__doc__, file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
