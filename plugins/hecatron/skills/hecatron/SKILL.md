---
name: hecatron
description: Sets up, builds, tests, lints and formats code in almost any programming language through one CLI, hecatron (`hct`). Use it whenever you are about to get a repository working, build it or run its tests, including one you have just cloned or been asked to set up, before installing compilers, SDKs or package managers yourself; and whenever the user mentions hct, hecatron or a .hecatron directory. `hct status` says which languages a repository uses and which toolchains are missing, `hct install` sets them up, and `hct test` runs every language's tests.
license: MIT
compatibility: Needs the hct command (macOS or Linux), installed with hecatron's install.sh.
metadata:
  hecatron-version: "0.7.0"
allowed-tools: Bash(HECATRON_OUTPUT=agent hct status) Bash(hct status:*) Bash(hct --version) Bash(hct help:*) Bash(hct list:*) Bash(hct check) Bash(hct doctor)
---

# Using hecatron (`hct`)

hecatron runs each language's own tools (cargo, go, gradle, uv, npm, mix...)
behind the same commands in every repository. Use it instead of working out
each language's test, lint or build command.

## First

1. `HECATRON_OUTPUT=agent hct status`. Not found: tell the user hecatron
   isn't installed and that
   `curl -fsSL https://raw.githubusercontent.com/dnf/hecatron/main/install.sh | sh`
   installs it (it changes their shell startup files); don't install it
   yourself unless they say to. Meanwhile, run the language's own command
   if you know it, and say that's what you did.
2. Read it: each language and where its code is, the toolchain version it
   uses and whether that's installed, the last run of each action, the
   uncommitted changes, and the commands worth running next (those marked
   "ask the user first" install something). Any git repository works;
   without `.hecatron/config.json`, hecatron detects the languages and
   writes nothing. If it says status needs hecatron's binary (an older or
   source install), `hct list` gives the languages and `hct doctor` the
   toolchains.
3. Set `HECATRON_OUTPUT=agent` on every `hct` command (or pass
   `--output agent`): one summary line, then only the failures, each with
   its log path and its key error lines.

## The loop

- Tests: `hct test <lang>` while working on one language; before you
  finish, `hct test --changed` and `hct lint --changed` (every language and
  component your changes touch, uncommitted ones included), or `hct test`
  for everything. `-c <component>` narrows to one component.
- Lint: `hct lint <lang>`. It includes the formatter's check; on a
  formatting failure run `hct format <lang>`, then lint again.
- Build: `hct build <lang>`. Dependencies: `hct deps <lang>`.
- After pulling, rebasing or switching branches: `hct sync`. It fetches
  dependencies whose lock files changed, says which toolchain versions are
  now needed (installing one is for the user to approve) and checks the
  config.
- Read the summary; open a log (`<logs>/<action>/<lang>.log`, path in the
  output) only for a failure whose excerpt isn't enough.
- Exit status: 0 passed, 1 failed, 2 usage error, 3 a toolchain is missing.
  A `SKIP` line means that language or component had nothing to run (no
  tests, no linter): it exits 0, but don't report it as tested.

## Ask the user first

`hct install`, `hct doctor --fix` and `hct uninstall` install, repair or
remove toolchains on this machine (system packages, files in the home
directory). Propose the exact command and wait for a yes. `hct doctor`
without `--fix` only checks and is safe.

## When something goes wrong

| You see | Do |
| --- | --- |
| Exit 3, or "not installed" | `hct doctor <lang>` to see what's missing; ask before `hct install <lang>` |
| "no <action> command for <dir>" | `hct list` to check where the language's code is; if the layout is right, the project needs its own command: write `.hecatron/languages/<lang>/<action>.sh` |
| Lint fails on formatting | `hct format <lang>`, then `hct lint <lang>` |
| `hct deps` fails: the lock file is out of date | Update the lock with the ecosystem's tool (`npm install`, `uv lock`, `bundle lock`...), then run `hct deps` again |
| A tool says the toolchain version is wrong | `hct doctor <lang>` says which version each component uses and why; pin one in the ecosystem's own file (`.python-version`, `.nvmrc`, `rust-toolchain.toml`, `go.mod`'s `toolchain`) |
| `hct audit` fails | The findings name each package and advisory; upgrade it, or, if the user accepts the risk, add it to `audit.ignore` with a reason and an expiry date |

## Changing how a language runs

A script at `.hecatron/languages/<lang>/<action>.sh` (`test`, `lint`,
`build`, `clean`, `format`, `deps`, `audit`, `outdated`, `docs`,
`bench`) replaces hecatron's default for that language. It runs once per
component, in the component's directory, with `HECATRON_LANG`,
`HECATRON_COMPONENT` and `HECATRON_ROOT` set; with nothing to run, it
sources `${HECATRON_HOME}/languages/_hooks.sh` and calls `skip "reason"`.
`hct init` saves the
detected layout as `.hecatron/config.json`, to commit and adjust.

## More

- [references/reference.md](references/reference.md): every command, the
  configuration keys, how versions are chosen.
- [references/languages.md](references/languages.md): each language's
  default build, test, lint and format tools.
- `hct help agents` prints the reference for the installed version.
