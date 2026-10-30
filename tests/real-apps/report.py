#!/usr/bin/env python3
"""The real-apps workflow's bookkeeping, for try-next.sh: whose turn it is,
what a run's outcome was, and the issue and job summary that report it. No
network and no judgement: what it writes is what the run recorded.

  report.py pick APPS [--slot N]   the app whose turn it is (slot: the clock's two-hour slot)
  report.py fixups FIXUPS APP      the app's setup steps, one per line
  report.py outcome WORK           kind<TAB>file|quiet<TAB>one line saying what happened
  report.py issue WORK             the issue's title, a line, then its body
  report.py comment WORK           a comment for the app's open issue
  report.py summary WORK           the job summary

WORK is try-next.sh's directory: meta.tsv (key<TAB>value), steps.tsv
(name<TAB>exit status<TAB>seconds<TAB>command), and, when they ran,
plan.json (hct _xcode plan --json), build.json (hct build --output json)
and logs/hct/ (the app's .logs).
"""

from __future__ import annotations

import json
import os
import re
import sys
import time

SLOT_SECONDS = 2 * 60 * 60
LOG_TAIL_LINES = 80
BODY_LIMIT = 60000  # GitHub's limit is 65536 characters


def apps(path: str) -> list[str]:
    """The list's owner/repo entries, in order, without comments."""
    with open(path, encoding="utf-8") as f:
        return [line.split("#", 1)[0].strip() for line in f if line.split("#", 1)[0].strip()]


# A comment: # at the start of a line, or after whitespace and before it
# (so a command can still hold a # of its own, as in a URL's fragment).
COMMENT = re.compile(r"(^|\s)#(\s.*)?$")


def fixups(path: str, app: str) -> list[str]:
    """The app's setup steps in the fixups file, in order."""
    steps = []
    try:
        with open(path, encoding="utf-8") as f:
            for line in f:
                line = COMMENT.sub("", line.rstrip("\n")).strip()
                name, sep, command = line.partition(":")
                if sep and name.strip().lower() == app.lower() and command.strip():
                    steps.append(command.strip())
    except OSError:
        pass
    return steps


def pick(path: str, slot: int | None = None) -> str:
    listed = apps(path)
    if slot is None:
        slot = int(time.time()) // SLOT_SECONDS
    return listed[slot % len(listed)]


def read_tsv(path: str, fields: int) -> list[list[str]]:
    try:
        with open(path, encoding="utf-8") as f:
            return [line.rstrip("\n").split("\t", fields - 1) for line in f if line.strip()]
    except OSError:
        return []


def read_json(path: str):
    try:
        with open(path, encoding="utf-8") as f:
            return json.load(f)
    except (OSError, ValueError):
        return None


class Run:
    def __init__(self, work: str):
        self.work = work
        self.meta = {row[0]: row[1] for row in read_tsv(os.path.join(work, "meta.tsv"), 2) if len(row) == 2}
        self.steps = [row for row in read_tsv(os.path.join(work, "steps.tsv"), 4) if len(row) == 4]
        self.plan = read_json(os.path.join(work, "plan.json"))
        self.build = read_json(os.path.join(work, "build.json"))

    def step(self, name: str) -> list[str] | None:
        return next((row for row in self.steps if row[0] == name), None)

    def failed(self, name: str) -> bool:
        row = self.step(name)
        return row is not None and row[1] != "0"

    def outcome(self) -> tuple[str, bool, str]:
        """The run's kind, whether to file it, and a line saying what
        happened."""
        app = self.meta.get("app", "?")
        if self.meta.get("harness_error"):
            return "harness-error", True, f"the workflow couldn't try {app}: {self.meta['harness_error']}"
        if self.failed("clone"):
            return "clone-failed", True, f"{app} couldn't be cloned"
        if self.failed("setup"):
            return "setup-failed", True, f"{app}'s setup steps (tests/real-apps/fixups.txt) failed"
        if self.failed("init"):
            return "init-failed", True, f"hct init failed on {app}"
        if not self.meta.get("languages"):
            return "not-detected", True, f"hct found no Swift or Objective-C in {app}"
        if self.build is None:
            return "build-crashed", True, f"hct build printed no JSON report on {app}"
        status = self.build.get("status")
        if status == "passed":
            return "passed", False, f"{app} built"
        if status == "skipped":
            # Nothing was built: most often a getting-started step the app's
            # README asks for, which fixups.txt then adds.
            return "nothing-built", True, f"hct built nothing in {app}: " + "; ".join(self.skip_reasons())
        return "build-failed", True, f"hct build {status or 'failed'} on {app}"

    def jobs(self, *statuses: str) -> list[dict]:
        return [job for job in (self.build or {}).get("jobs", []) if job.get("status") in statuses]

    def skip_reasons(self) -> list[str]:
        return [f"{job.get('label')}: {job.get('reason', '')}" for job in self.jobs("skipped")]


def code(text: str, language: str = "") -> str:
    fence = "````" if "```" in text else "```"
    return f"{fence}{language}\n{text.rstrip()}\n{fence}"


def tail(path: str, lines: int = LOG_TAIL_LINES) -> str:
    try:
        with open(path, encoding="utf-8", errors="replace") as f:
            return "".join(f.readlines()[-lines:])
    except OSError:
        return ""


def facts(run: Run) -> str:
    meta = run.meta
    app = meta.get("app", "?")
    sha = meta.get("app_sha", "")
    commit = f"[`{sha[:12]}`](https://github.com/{app}/commit/{sha})" if sha else "?"
    rows = [
        ("App", f"[{app}](https://github.com/{app}) at {commit}"),
        ("hecatron", f"`{meta.get('hecatron_sha', '?')[:12]}`"),
        ("Languages", meta.get("languages") or "none detected"),
        ("macOS", meta.get("macos", "?")),
        ("Xcode", meta.get("xcode", "?")),
    ]
    if meta.get("run_url"):
        rows.append(("Run", f"{meta['run_url']} (logs: the `real-app-logs` artifact)"))
    return "| | |\n| --- | --- |\n" + "\n".join(f"| {k} | {v} |" for k, v in rows)


def steps_table(run: Run) -> str:
    lines = ["| Step | Exit | Seconds | Command |", "| --- | --- | --- | --- |"]
    for name, status, seconds, command in run.steps:
        lines.append(f"| {name} | {status} | {seconds} | `{command}` |")
    return "\n".join(lines)


def details(summary: str, body: str) -> str:
    return f"<details><summary>{summary}</summary>\n\n{body}\n\n</details>"


def body(run: Run) -> str:
    kind, _, line = run.outcome()
    parts = [line + ".", facts(run), "### Steps", steps_table(run)]
    for job in run.jobs("failed", "timed out", "not installed"):
        parts.append(f"### {job.get('label')}: {job.get('status')}")
        if job.get("errors"):
            parts.append(code("\n".join(e if isinstance(e, str) else e.get("text", json.dumps(e))
                                        for e in job["errors"])))
        log = job.get("log", "")
        if log:
            path = os.path.join(run.work, "logs", "hct", os.path.basename(os.path.dirname(log)), os.path.basename(log))
            text = tail(path)
            if text:
                parts.append(details(f"The last {LOG_TAIL_LINES} lines of {os.path.basename(log)}", code(text)))
    if run.skip_reasons():
        parts.append("### Skipped\n" + "\n".join(f"- {reason}" for reason in run.skip_reasons()))
    for name in ("clone", "setup", "init", "deps"):
        if run.failed(name):
            parts.append(details(f"The {name} step's output", code(tail(os.path.join(run.work, "logs", name + ".log")))))
    if run.plan is not None:
        parts.append(details("hct _xcode plan", code(json.dumps(run.plan, indent=2), "json")))
    parts.append(NEXT)
    text = "\n\n".join(parts)
    if len(text) > BODY_LIMIT:
        text = text[:BODY_LIMIT] + "\n\n(cut short: the full logs are in the run's artifact)"
    return text


NEXT = """### Next

Filed by the real-apps workflow (tests/real-apps/), which reports what happened; whether hecatron or the app needs the \
fix is for whoever picks this up. If the app needs a manual getting-started step (its README's: a template to copy, a \
bootstrap script), add it to `tests/real-apps/fixups.txt`; if hecatron needs fixing, fix it. Then label this issue \
`real-app-retry`: the next run tries the app again and says here how it went (closing the issue when it builds)."""


def title(run: Run) -> str:
    return f"Real app: {run.meta.get('app', '?')}"


def summary(run: Run) -> str:
    kind, file, line = run.outcome()
    return "\n\n".join([f"## {line}", f"Outcome: `{kind}`" + (" (filed)" if file else ""), facts(run), steps_table(run)])


def main(argv: list[str]) -> int:
    if len(argv) == 3 and argv[0] == "fixups":
        for step in fixups(argv[1], argv[2]):
            print(step)
        return 0
    if len(argv) >= 2 and argv[0] == "pick":
        slot = int(argv[3]) if len(argv) == 4 and argv[2] == "--slot" else None
        print(pick(argv[1], slot))
        return 0
    if len(argv) == 2 and argv[0] in ("outcome", "issue", "comment", "summary"):
        run = Run(argv[1])
        if argv[0] == "outcome":
            kind, file, line = run.outcome()
            print(f"{kind}\t{'file' if file else 'quiet'}\t{line}")
        elif argv[0] == "issue":
            print(title(run))
            print(body(run))
        elif argv[0] == "comment":
            print(body(run) if run.outcome()[1] else run.outcome()[2] + ".\n\n" + facts(run))
        else:
            print(summary(run))
        return 0
    print(__doc__, file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
