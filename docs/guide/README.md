# User guide

Using hecatron in a project. `hct` is short for `hecatron`; `hct help`
lists the commands and `hct <command> help` a command's options.

```bash
cd your-repo
hct status        # each language, its toolchain, its last runs and changes
hct doctor --fix  # install and repair every language's toolchain
hct test          # every language's tests, in parallel
```

## Concepts

- **Project:** a git repository, configured by its `.hecatron/` directory
  or, without one, detected. Commands find it by walking up from the
  working directory, like git.
- **Language:** one the project is implemented in. Its **definition**
  (built in, or the project's own) says how to install, check, build and
  test it.
- **Component:** a separately buildable unit implemented in each language
  (a single-component project has one). Each language's code for a
  component lives at a path built from a template, e.g.
  `{component}/impl/{lang}`.
- **Action:** `test`, `lint`, `build`, `clean`, `deps`, `format`, `audit`,
  `outdated`, `docs`, `bench`. A **hook** script implements it, run once per component.

## Pages

| Page | Contents |
| --- | --- |
| [Installing hecatron](installing-hecatron.md) | Installing, updating and removing hecatron, and pinning a project's version of it |
| [Commands](commands.md) | Every command: what it does, its options, examples |
| [Configuration](configuration.md) | `.hecatron/config.json`, `local.json`, and overriding a language's files |
| [Init and detection](init.md) | `hct init`, and how a repository's languages and layout are detected |
| [Hooks](hooks.md) | The hook contract, what the default hooks do, dependencies, formatting, trust |
| [Toolchain versions](versions.md) | Pin files, `version`, defaults, version syntax, old checkouts |
| [Installing toolchains](installing-toolchains.md) | `install`, `uninstall`, `doctor`, shared toolchains, platforms |
| [The parallel runner](runner.md) | Concurrency, scheduling, the live display, time limits, `--changed`, `hct sync`, reports |
| [Output](output.md) | Human, agent, JSON and CI output; exit codes |
| [Generated files](generated-files.md) | The Dockerfile, CI workflows, the GitLab pipeline, caching, Dependabot and Renovate |
| [Hecatron in CI](ci.md) | What works on each forge today |
| [Formatting](format.md) | `hct format` and the format check in `hct lint` |
| [Auditing dependencies](audit.md) | `hct audit`: auditors, severities, accepted risks, CI |
| [Outdated dependencies](outdated.md) | `hct outdated` |
| [Dependency licenses](licenses.md) | `hct licenses` and a license policy |
| [hct advisor](advisor.md) | Each ecosystem's good practices, and the rules |
| [Languages](languages.md) | Every language and its default tools |
| [Coverage](coverage.md) | Which languages install and work on which platforms |
| [Reference for AI agents](../agents.md) | The compact reference `hct help agents` prints |

Background: [why one repository](why-one-repository.md) and
[a ten-language library, hecatron or ad hoc](hecatron-vs-ad-hoc.md).
