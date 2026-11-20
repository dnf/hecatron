#!/usr/bin/env python3
"""The agent comparison's chart for the README: median tokens per run, per
repository, without and with hecatron, from summarize.py --json. A static
SVG (a README can't run script), so every value is labelled on its bar and
the write-up has the table; light and dark from the palette's selected
steps.

  chart.py SUMMARY.json OUT.svg
"""

from __future__ import annotations

import json
import sys
from html import escape

ARMS = (("plain", "Without hecatron"), ("hecatron", "With hecatron"))
WIDTH = 720
LEFT = 170          # repository labels
RIGHT = 150         # room for the value labels past the longest bar
BAR = 18
GAP = 2             # the surface gap between a row's two bars
ROW = 2 * BAR + GAP + 22
TOP = 116
STYLE = """
  .surface { fill: #fcfcfb; } .plain { fill: #eb6834; } .hecatron { fill: #2a78d6; }
  .grid { stroke: #e1e0d9; } .axis { stroke: #c3c2b7; }
  .title { fill: #0b0b0b; } .label { fill: #0b0b0b; }
  .subtitle, .value, .legend { fill: #52514e; } .sublabel, .tick { fill: #898781; }
  @media (prefers-color-scheme: dark) {
    .surface { fill: #1a1a19; } .plain { fill: #d95926; } .hecatron { fill: #3987e5; }
    .grid { stroke: #2c2c2a; } .axis { stroke: #383835; }
    .title, .label { fill: #f0efec; } .subtitle, .value, .legend { fill: #c3c2b7; }
  }
  text { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif; }
  .title { font-size: 20px; font-weight: 600; } .subtitle { font-size: 13px; }
  .label { font-size: 14px; font-weight: 600; } .sublabel, .value, .legend { font-size: 12px; }
  .tick { font-size: 11px; }
"""


def kilo(n: float) -> str:
    return f"{n / 1e6:.1f}M" if n >= 1e6 else f"{n / 1e3:.0f}k"


def nice_step(top: float) -> float:
    for step in (50e3, 100e3, 200e3, 250e3, 500e3, 1e6, 2e6, 2.5e6, 5e6):
        if top / step <= 5:
            return step
    return 10e6


def bar(x: float, y: float, width: float, color: str) -> str:
    """A bar anchored on the baseline with a 4px rounded data end."""
    width = max(width, 2)
    r = min(4, width / 2, BAR / 2)
    return (f'<path d="M{x},{y} h{width - r} a{r},{r} 0 0 1 {r},{r} v{BAR - 2 * r} a{r},{r} 0 0 1 {-r},{r} '
            f'h{-(width - r)} z" class="{color}"/>')


def chart(summary: dict, repos: list[dict]) -> str:
    rows = [r for r in repos if r["name"] in summary["repos"]]
    top = max(summary["repos"][r["name"]][arm]["tokens"] for r in rows for arm, _ in ARMS
              if arm in summary["repos"][r["name"]])
    step = nice_step(top)
    scale_max = step * -(-top // step)
    plot = WIDTH - LEFT - RIGHT
    height = TOP + len(rows) * ROW + 40
    overall = summary["overall"]
    p, h = overall["plain"], overall["hecatron"]
    tokens = (h["tokens"] - p["tokens"]) / p["tokens"] * 100
    cost = (h["cost"] - p["cost"]) / p["cost"] * 100
    runs = p["runs"] // max(len(rows), 1)
    headline = (f"Agents used {abs(tokens):.0f}% {'fewer' if tokens < 0 else 'more'} tokens with hecatron"
                if abs(tokens) >= 1 else "Agents used about the same tokens with hecatron")
    out = [f'<svg xmlns="http://www.w3.org/2000/svg" width="{WIDTH}" height="{height}" '
           f'viewBox="0 0 {WIDTH} {height}" role="img" aria-labelledby="t d">',
           f"<style>{STYLE}</style>",
           f'<rect class="surface" width="{WIDTH}" height="{height}" rx="8"/>',
           f'<title id="t">{escape(headline)}</title>',
           f'<desc id="d">Median tokens per run for Claude Code to get each repository\'s tests running on a fresh '
           f'machine, without and with hecatron. ' + "; ".join(
               f"{r['name']}: " + ", ".join(f"{label.lower()} {kilo(summary['repos'][r['name']][arm]['tokens'])}"
                                             for arm, label in ARMS if arm in summary["repos"][r["name"]])
               for r in rows) + "</desc>",
           f'<text class="title" x="24" y="38">{escape(headline)}</text>',
           f'<text class="subtitle" x="24" y="60">Median tokens per run to get a repository\'s tests running on a '
           f'fresh machine.</text>',
           f'<text class="subtitle" x="24" y="78">Claude Code, Sonnet 5.5, {runs} runs each. Cost {cost:+.0f}%. '
           f'Tests passing: {p["passed"]}/{p["runs"]} without, {h["passed"]}/{h["runs"]} with.</text>']
    # Legend, above the plot.
    x = LEFT
    for arm, label in ARMS:
        out.append(f'<rect class="{arm}" x="{x}" y="92" width="12" height="12" rx="3"/>')
        out.append(f'<text class="legend" x="{x + 18}" y="102">{label}</text>')
        x += 18 + 7.2 * len(label) + 24
    # Recessive gridlines and ticks on the one value axis.
    plot_bottom = TOP + len(rows) * ROW - 22
    tick = 0.0
    while tick <= scale_max + 1:
        gx = LEFT + plot * tick / scale_max
        out.append(f'<line x1="{gx:.1f}" y1="{TOP - 6}" x2="{gx:.1f}" y2="{plot_bottom}" '
                   f'class="{"axis" if tick == 0 else "grid"}" stroke-width="1"/>')
        out.append(f'<text class="tick" x="{gx:.1f}" y="{plot_bottom + 16}" text-anchor="middle">{kilo(tick) if tick else "0"}</text>')
        tick += step
    for i, repo in enumerate(rows):
        y = TOP + i * ROW
        data = summary["repos"][repo["name"]]
        out.append(f'<text class="label" x="{LEFT - 14}" y="{y + BAR + 2}" text-anchor="end">{escape(repo["name"])}</text>')
        out.append(f'<text class="sublabel" x="{LEFT - 14}" y="{y + BAR + 18}" text-anchor="end">{escape(repo["language"])}</text>')
        for j, (arm, _) in enumerate(ARMS):
            if arm not in data:
                continue
            by = y + j * (BAR + GAP)
            width = plot * data[arm]["tokens"] / scale_max
            out.append(bar(LEFT, by, width, arm))
            passed = f' · {data[arm]["passed"]}/{data[arm]["runs"]} passed'
            out.append(f'<text class="value" x="{LEFT + width + 6:.1f}" y="{by + 13}">{kilo(data[arm]["tokens"])}{passed}</text>')
    out.append("</svg>")
    return "\n".join(out) + "\n"


def main(argv: list[str]) -> int:
    if len(argv) != 2:
        print(__doc__, file=sys.stderr)
        return 2
    import os
    with open(argv[0], encoding="utf-8") as f:
        summary = json.load(f)
    with open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "repos.json"), encoding="utf-8") as f:
        repos = json.load(f)
    with open(argv[1], "w", encoding="utf-8") as f:
        f.write(chart(summary, repos))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
