#!/usr/bin/env python3
"""The agent comparison's numbers (run.sh's RESULTS directory): per run,
Claude Code's own usage and cost, turns, time, whether the hecatron skill
fired and hct was used, and whether the test command the agent recorded
passed when the harness ran it again; then medians per repository and arm.

  summarize.py RESULTS [--json OUT]   a Markdown table; --json writes the data the chart reads
"""

from __future__ import annotations

import json
import os
import statistics
import sys

ARMS = ("plain", "hecatron")


def result_event(path: str) -> dict | None:
    """The stream's final result event (usage, cost, turns, time)."""
    found = None
    try:
        with open(path, encoding="utf-8") as f:
            for line in f:
                try:
                    event = json.loads(line)
                except ValueError:
                    continue
                if event.get("type") == "result":
                    found = event
    except OSError:
        return None
    return found


def tool_uses(path: str):
    try:
        with open(path, encoding="utf-8") as f:
            for line in f:
                try:
                    event = json.loads(line)
                except ValueError:
                    continue
                if event.get("type") != "assistant":
                    continue
                for part in (event.get("message") or {}).get("content") or []:
                    if isinstance(part, dict) and part.get("type") == "tool_use":
                        yield part.get("name"), part.get("input") or {}
    except OSError:
        return


def run(directory: str) -> dict | None:
    stream = os.path.join(directory, "stream.jsonl")
    event = result_event(stream)
    if event is None:
        return None
    usage = event.get("usage") or {}
    tokens = {key: usage.get(key, 0) or 0 for key in
              ("input_tokens", "output_tokens", "cache_creation_input_tokens", "cache_read_input_tokens")}
    skill = hct = False
    for name, arguments in tool_uses(stream):
        if name == "Skill" and "hecatron" in json.dumps(arguments):
            skill = True
        if name == "Bash" and ("hct " in arguments.get("command", "") or "hecatron " in arguments.get("command", "")):
            hct = True
    try:
        with open(os.path.join(directory, "verify.txt"), encoding="utf-8") as f:
            passed = f.read().strip() == "exit=0"
    except OSError:
        passed = False
    return {
        "tokens": sum(tokens.values()),
        **tokens,
        "cost": event.get("total_cost_usd") or 0.0,
        "turns": event.get("num_turns") or 0,
        "minutes": round((event.get("duration_ms") or 0) / 60000, 1),
        "passed": passed,
        "skill": skill,
        "hct": hct,
    }


def gather(results: str) -> dict:
    data = {}
    for repo in sorted(os.listdir(results)):
        path = os.path.join(results, repo)
        if not os.path.isdir(path):
            continue
        for entry in sorted(os.listdir(path)):
            arm = entry.rsplit("-", 1)[0]
            if arm in ARMS:
                r = run(os.path.join(path, entry))
                if r is not None:
                    data.setdefault(repo, {}).setdefault(arm, []).append(r)
    return data


def median(runs: list[dict], key: str) -> float:
    return statistics.median(r[key] for r in runs) if runs else 0.0


def summary(data: dict) -> dict:
    out = {"repos": {}, "overall": {}}
    for repo, arms in data.items():
        out["repos"][repo] = {arm: {
            "runs": len(runs),
            "passed": sum(r["passed"] for r in runs),
            "skill": sum(r["skill"] for r in runs),
            "hct": sum(r["hct"] for r in runs),
            **{key: median(runs, key) for key in ("tokens", "output_tokens", "cost", "turns", "minutes")},
        } for arm, runs in arms.items()}
    for arm in ARMS:
        runs = [r for arms in data.values() for r in arms.get(arm, [])]
        out["overall"][arm] = {
            "runs": len(runs),
            "passed": sum(r["passed"] for r in runs),
            "tokens": sum(r["tokens"] for r in runs),
            "cost": round(sum(r["cost"] for r in runs), 2),
            "turns": sum(r["turns"] for r in runs),
            "minutes": round(sum(r["minutes"] for r in runs), 1),
        }
    return out


def change(before: float, after: float) -> str:
    return f"{(after - before) / before * 100:+.0f}%" if before else "n/a"


def table(s: dict) -> str:
    lines = ["| Repository | Arm | Passed | Median tokens | Median cost | Median turns | Median minutes |",
             "| --- | --- | --- | --- | --- | --- | --- |"]
    for repo, arms in s["repos"].items():
        for arm in ARMS:
            if arm in arms:
                a = arms[arm]
                lines.append(f"| {repo} | {arm} | {a['passed']}/{a['runs']} | {a['tokens']:,.0f} | ${a['cost']:.2f} | "
                             f"{a['turns']:.0f} | {a['minutes']:.1f} |")
    p, h = s["overall"].get("plain", {}), s["overall"].get("hecatron", {})
    if p and h:
        lines += ["", f"All runs: plain {p['passed']}/{p['runs']} passed, {p['tokens']:,} tokens, ${p['cost']:.2f}; "
                      f"hecatron {h['passed']}/{h['runs']} passed, {h['tokens']:,} tokens, ${h['cost']:.2f} "
                      f"(tokens {change(p['tokens'], h['tokens'])}, cost {change(p['cost'], h['cost'])}, "
                      f"turns {change(p['turns'], h['turns'])}, time {change(p['minutes'], h['minutes'])})."]
    return "\n".join(lines)


def main(argv: list[str]) -> int:
    if not argv:
        print(__doc__, file=sys.stderr)
        return 2
    s = summary(gather(argv[0]))
    print(table(s))
    if len(argv) == 3 and argv[1] == "--json":
        with open(argv[2], "w", encoding="utf-8") as f:
            json.dump(s, f, indent=2)
            f.write("\n")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
