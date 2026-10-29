# Contributor guide

Working on hecatron itself. Start with [CONTRIBUTING.md](../../CONTRIBUTING.md)
and the conventions in [CLAUDE.md](../../CLAUDE.md); what hecatron does for
users is in the [user guide](../guide/README.md).

| Page | Contents |
| --- | --- |
| [Architecture](architecture.md) | The two implementations (scripts and the Go core), the pieces, and the rules they rest on |
| [Modules](modules.md) | What each file and package does: `bin/`, `cmd/hecatron`, `internal/*`, `libexec/`, `languages/` |
| [Command flow](command-flow.md) | A command from `bin/hecatron` through the binary or the scripts, the runner, the project model and the hooks |
| [The install engine](install-engine.md) | Installs as data: `install.json`, the executor, release downloads, receipts, parity |
| [Installer scripts](installer-scripts.md) | `install.sh`, `uninstall.sh`, `doctor.sh` and their helpers |
| [Adding a language](adding-a-language.md) | A language definition's files and checklist |
| [Build conventions](build-conventions.md) | Where each language's build artifacts go |
| [Testing](testing.md) | The test suites, Docker tests, parity tests and what CI runs |
| [Releasing](releasing.md) | Changelog fragments, `release.sh`, the release workflow |
| [Design record](design.md) | Goals, concepts, the owner's decisions and their history, the Go core's design and plans |

The bash runner's internals have their own docs beside it:
[libexec/runner/README.md](../../libexec/runner/README.md) and
[ARCHITECTURE.md](../../libexec/runner/ARCHITECTURE.md).
