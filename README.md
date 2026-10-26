# hecatron

Toolchains, tests and CI for polyglot repositories: one project implemented in
many programming languages, possibly as several components each implemented
in every language.

Hecatron installs, checks and repairs each language's toolchain on macOS and
Linux, runs a project's tests, linters, builds and cleans per language or for
everything in parallel with a live progress display, and keeps generated CI
workflows and a Dockerfile in sync.

> **Status:** early (0.1). Split out of [dnf/sayable](https://github.com/dnf/sayable);
> the command works, Homebrew packaging and default hooks are next. The design
> is in [docs/design.md](docs/design.md).

## Try it

```bash
git clone https://github.com/dnf/hecatron && export PATH="$PWD/hecatron/bin:$PATH"
cd your-repo
hecatron init          # detects languages and layout, writes .hecatron/config.json
hecatron doctor        # checks each toolchain; --fix installs and repairs
hecatron test          # every language's tests, in parallel
hct test go -c api     # one language, one component (hct is the short form)
```

Projects describe their layout in `.hecatron/config.json` and put per-language
hooks (`test.sh`, `lint.sh`, `build.sh`, `clean.sh`) in
`.hecatron/languages/<lang>/`; `hecatron help` lists every command.

## What's here

| Path | Contents |
| --- | --- |
| [`docs/design.md`](docs/design.md) | The design for the standalone tool |
| `languages/<lang>/` | Toolchain definitions for 51 languages: `config.json`, `install.sh`, `uninstall.sh`, `doctor.sh`, Dockerfile snippet ([adding a language](docs/adding-a-language.md)) |
| `languages/_common.sh` | Shared installer helpers |
| `bin/hecatron`, `bin/hct` | The command |
| `libexec/lib/` | The project model (`project.py`), shell helpers, per-component dispatch |
| `libexec/commands/` | The commands (`run.sh` for test/lint/build/clean, `manage.py` for configuration) |
| `libexec/runner/` | The parallel runner and its docs |
| `libexec/codegen/` | DO NOT EDIT banners and the Dockerfile generator |
| `libexec/{metadata,onboarding,bootstrap,lint}/` | Metadata updates, onboarding, machine bootstrap, shellcheck |
| `tests/` | Unit and command tests against fixture repositories |
| [`docs/`](docs/) | Installer reference, adding a language, build conventions |
| `examples/sayable/` | Sayable's project hooks, CI workflow templates and config at the time of the split |

## License

[MIT](LICENSE.md)
