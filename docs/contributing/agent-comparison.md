# The agent comparison

How much does hecatron save an AI agent? `tests/agent-comparison/` measures it:
Claude Code gets the same prompt in two fresh containers, one without
hecatron (`plain`) and one with its latest release and Agent Skill
installed (`hecatron`). The prompt is to get a repository's tests running,
install whatever they need, and record the command that runs them. Each run
reports Claude Code's own token use, cost, turns and time, whether the skill
fired and `hct` was used, and whether the recorded test command passes when
the harness runs it again in that container.

It is never run in CI: it needs an agent's credentials, and every run costs
money. #171 tracks the runs and the write-up.

## Files

| File | What it is |
| --- | --- |
| `repos.json` | The repositories, each at a pinned commit, with its language and an optional scope added to the prompt |
| `Dockerfile` | `plain`: Ubuntu 24.04 with sudo, network and Claude Code, and no toolchains; `hecatron`: the same, plus hecatron's latest release and its skill |
| `run.sh` | Builds both images and runs every repository in each arm, several times, a few at a time |
| `summarize.py` | Per run and per repository and arm (medians): a Markdown table, and with `--json` the data `chart.py` reads |
| `chart.py` | A static SVG of median tokens per repository and arm, for the README |

`tests/test_agent_comparison.py` checks `summarize.py` and `chart.py` on a
made-up result. It needs neither Docker nor credentials.

## Running it

Put the credentials in a file only Docker reads. Run `claude setup-token`
in your own terminal (not through an agent, so the token stays out of its
transcript), then save the token it prints:

```bash
mkdir -p ~/.config/hecatron-bench && chmod 700 ~/.config/hecatron-bench
stty -echo; printf 'Paste the token, then press Enter and Ctrl-D: '; t=$(tr -d '[:space:]'); stty echo; echo
(umask 077; printf 'CLAUDE_CODE_OAUTH_TOKEN=%s\n' "$t" > ~/.config/hecatron-bench/env); unset t
```

The token is long enough that the terminal wraps it, and copying it can
bring a line break along; reading until Ctrl-D and dropping whitespace
keeps all of it (a prompt that stops at the first line break can save part
of it, which the API rejects as `authentication_failed`). It isn't echoed or kept
in the shell's history, and `umask 077` makes the file readable only by
you from the start.

(`ANTHROPIC_API_KEY=...` works too.) Then:

```bash
ENV_FILE=~/.config/hecatron-bench/env tests/agent-comparison/run.sh RESULTS 3 3
tests/agent-comparison/summarize.py RESULTS --json RESULTS/summary.json
tests/agent-comparison/chart.py RESULTS/summary.json RESULTS/tokens.svg
```

`run.sh RESULTS [RUNS] [PARALLEL]` runs each repository `RUNS` times per arm
(default 3), `PARALLEL` containers at a time (default 3). Each run writes
`RESULTS/<repo>/<arm>-<n>/`. `MODEL` picks the model (default
`claude-sonnet-5-5`), `ONLY` runs one repository by name, and `TIMEOUT`
limits a run (default `45m`). `SKILL=plugins/hecatron/skills/hecatron`
puts a checkout's Agent Skill in the hecatron arm in place of the
release's, to measure a change to the skill before releasing it (say so
with the numbers: that arm is then not quite what a newcomer installs).
A rerun with the same RESULTS skips the runs that finished, so an
interrupted batch picks up where it stopped; to rerun one arm only, copy
the other arm's run directories into a new RESULTS first.

## Keeping it fair

- Both arms have Python 3 and perl, because hecatron's installer adds them.
  The only difference is hecatron and its skill.
- The `hecatron` arm installs the latest release as a newcomer would
  (`install.sh`, then `hct skill install`). A new release rebuilds that
  layer.
- Repositories are pinned to a commit, so reruns measure the same task.
- A run passes only if the harness can rerun the agent's recorded test
  command afterwards; the agent's own report of success isn't used.
