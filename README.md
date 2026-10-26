# hecatron

Toolchains, tests and CI for polyglot repositories: one project implemented in
many programming languages, possibly as several components each implemented
in every language.

Hecatron installs, checks and repairs each language's toolchain on macOS and
Linux, runs a project's tests, linters, builds and cleans per language or for
everything in parallel with a live progress display, and keeps generated CI
workflows and a Dockerfile in sync.

> **Status:** split out of [dnf/sayable](https://github.com/dnf/sayable) and
> not yet usable on its own. The plan, including `brew install hecatron` and
> `hecatron init`, is in [docs/design.md](docs/design.md).

## What's here

| Path | Contents |
| --- | --- |
| [`docs/design.md`](docs/design.md) | The design for the standalone tool |
| `languages/<lang>/` | Toolchain definitions for 51 languages: `config.json`, `install.sh`, `uninstall.sh`, `doctor.sh`, Dockerfile snippet |
| `languages/_common.sh` | Shared installer helpers |
| `tools/runner/` | The parallel runner and its docs |
| `tools/language/` | Language configuration, management and per-component dispatch |
| `tools/codegen/` | DO NOT EDIT banners and the Dockerfile generator |
| `tools/metadata/`, `tools/onboarding/`, `tools/bootstrap/`, `tools/lint/` | Metadata updates, onboarding, machine bootstrap, shellcheck |
| [`docs/`](docs/) | Installer reference, adding a language, build conventions |
| `examples/sayable/` | Sayable's project hooks, CI workflow templates and config at the time of the split |

## License

[MIT](LICENSE.md)
