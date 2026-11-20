"""The agent comparison's numbers and chart (tests/agent-comparison/): what
summarize.py reads from Claude Code's stream, and that chart.py draws it."""

from __future__ import annotations

import importlib.util
import json
import os
import tempfile
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DIR = os.path.join(HOME, "tests", "agent-comparison")


def load(name: str):
    spec = importlib.util.spec_from_file_location(f"agent_comparison_{name}", os.path.join(DIR, f"{name}.py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


summarize = load("summarize")
chart = load("chart")


def write_run(root: str, repo: str, arm: str, n: int, tokens: int, cost: float, passed: bool, hct: bool) -> None:
    path = os.path.join(root, repo, f"{arm}-{n}")
    os.makedirs(path)
    events = [{"type": "system", "subtype": "init"}]
    if hct:
        events.append({"type": "assistant", "message": {"content": [
            {"type": "tool_use", "name": "Skill", "input": {"skill": "hecatron"}},
            {"type": "tool_use", "name": "Bash", "input": {"command": "hct test"}}]}})
    events.append({"type": "result", "total_cost_usd": cost, "num_turns": 10, "duration_ms": 120000,
                   "usage": {"input_tokens": tokens // 2, "output_tokens": 1000,
                             "cache_creation_input_tokens": 0, "cache_read_input_tokens": tokens // 2 - 1000}})
    with open(os.path.join(path, "stream.jsonl"), "w") as f:
        f.write("not json\n" + "\n".join(json.dumps(e) for e in events) + "\n")
    with open(os.path.join(path, "verify.txt"), "w") as f:
        f.write("exit=0\n" if passed else "exit=1\n")


class SummaryTest(unittest.TestCase):
    def test_medians_totals_and_the_chart(self):
        with tempfile.TemporaryDirectory() as root:
            for n, tokens in enumerate((400_000, 500_000, 900_000), 1):
                write_run(root, "jason", "plain", n, tokens, tokens / 1e6, n != 3, False)
            for n, tokens in enumerate((200_000, 300_000, 250_000), 1):
                write_run(root, "jason", "hecatron", n, tokens, tokens / 1e6, True, True)
            os.makedirs(os.path.join(root, "jason", "plain-4"))  # a run with no result is left out
            s = summarize.summary(summarize.gather(root))
        plain, hecatron = s["repos"]["jason"]["plain"], s["repos"]["jason"]["hecatron"]
        self.assertEqual((plain["runs"], plain["passed"], plain["tokens"], plain["skill"]), (3, 2, 500_000, 0))
        self.assertEqual((hecatron["passed"], hecatron["tokens"], hecatron["skill"], hecatron["hct"]), (3, 250_000, 3, 3))
        self.assertEqual(s["overall"]["plain"]["tokens"], 1_800_000)
        self.assertIn("tokens -58%", summarize.table(s))
        svg = chart.chart(s, [{"name": "jason", "language": "Elixir"}])
        self.assertIn("Agents used 58% fewer tokens with hecatron", svg)
        self.assertIn("500k · 2/3 passed", svg)
        self.assertIn("250k · 3/3 passed", svg)
        self.assertIn("prefers-color-scheme: dark", svg)


if __name__ == "__main__":
    unittest.main()
