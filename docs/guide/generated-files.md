# Generated files

Hecatron generates these project files, each with the 9-line DO NOT
EDIT banner naming the generator and its source:

- **Dockerfile** (`generated.dockerfile`): collected from each language's
  `dockerfile` snippet (`# apt:` lines deduplicated into one `apt-get
  install`, `# after:` ordering, literal `RUN`/`ENV` blocks, and a block
  repeated per toolchain version; see [versions](versions.md#installing-and-using-a-version)). Before any snippet it writes a
  curlrc (`/etc/hecatron-curl/.curlrc`, found through `ENV CURL_HOME`, so it
  applies whatever the user) with the installers' retry policy (`retry 4`,
  `retry-delay 3`, `retry-connrefused`, `connect-timeout 30`, `speed-limit
  1024` over `speed-time 60`) plus `retry-all-errors`, so every `curl` in the
  build, including those inside piped install scripts, retries transient
  failures such as a dropped TLS connection. It stays in the image; a `RUN`
  that wants curl's defaults passes `-q` first. No snippet uses wget.
- **CI workflows** (`generated.workflows`): each language's project
  `workflow.yml`, with a banner. `hct check` verifies that each workflow
  mentions every component, generalising sayable's
  `check_workflow_components.py`. Each job under `jobs:` that has a
  `runs-on` and no `timeout-minutes` gets one, after its `runs-on`: 60
  minutes, or config.json's `ci.timeoutMinutes`, or the language's
  `ci.languages` entry. GitHub's own limit is 6 hours, so a hung job (a
  stalled download, a test waiting on input) would hold every merge that
  long; 60 is above every language's slowest run so far. A job calling a
  reusable workflow (`uses:`, no `runs-on`) can't have one and gets none,
  and the rest of the template stays as written. `hct check` reports
  a time limit that isn't a whole number of minutes, and `hct workflows`
  refuses to generate until it's fixed.

- **GitLab pipeline** (`generated.ci: "gitlab"`, or `hct workflows
  --ci gitlab`): `.gitlab-ci.yml`, written from the project's languages
  rather than a template: one job per language that installs hecatron with
  install.sh (`HECATRON_VERSION`: the project's [pinned](installing-hecatron.md#pinning-a-projects-hecatron) version, else
  the generating one) and runs `hct install`, `hct test` and `hct lint`
  for the language, which cover every component. The job's image is
  `ci.image`, and its `timeout` the same limit a GitHub job gets (`60m`, or `ci.timeoutMinutes`, or the
  language's `ci.languages` entry).
- **Caching** (`"ci": {"cache": true}`, off by default so that turning it
  on is what changes existing projects' files): each definition's `cache`
  names the directories its installer puts the toolchain in and its package
  manager downloads to (`~/...`, or relative to each component's
  directory), and its lock files. The toolchain cache is keyed on hecatron's
  version, the project's `config.json`, the language's definition in the
  project, and the definition's pin and support files; the dependency
  cache on the lock files, both matched anywhere in the repository (`**/`),
  so adding a component changes nothing. GitHub workflows get an
  `actions/cache` step per cache after each job's `actions/checkout`, with
  the key's prefix as `restore-keys`; GitLab jobs get `cache:key:files`
  with the same files and a prefix. GitLab caches only paths in the
  project, so each job links every home directory it caches to one under
  `.hecatron-cache/<lang>/` before installing. Toolchains installed by apt
  or into `/opt` and `/usr/local` (most JVM languages', Node.js, .NET and
  Ruby on Linux) aren't cached: restoring them needs root. A toolchain
  hecatron's executor downloads from its `install.json` ([installs as data](../contributing/install-engine.md)) goes to `~/.local/share/hecatron/toolchains/<lang>` instead (Gleam,
  Zig), which needs no root, and that directory is the language's toolchain cache,
  keyed on the language, the platform (`runner.os`, `runner.arch`), a hash
  of the built-in `install.json` and default version (written into the
  key, so a hecatron with different data writes a new one), and the
  project's own `install.json`, config and pin files. A restored toolchain
  passes the executor's checks, so the install does nothing; a partial or
  corrupt one fails them and is installed again. Most languages converted
  so far install system packages, which aren't cached; each language moved
  to a download makes the cache cover more. Since the
  toolchain key names hecatron's version, upgrading hecatron makes `hecatron
  check` ask for the files to be regenerated.

- **Dependency update configuration** (`generated.dependabot`,
  `generated.renovate`), written by `hct workflows` alongside the CI:
  `.github/dependabot.yml` with one `updates` entry per package ecosystem
  and component directory, each with the project's `dependabot` settings
  (and its ecosystem's under `dependabot.ecosystems`), and
  `renovate.json` with the project's `renovate` settings, `enabledManagers`
  for its languages and `includePaths` for their directories (none when a
  component sits at the root). Each definition names its Dependabot
  ecosystems (`"dependabot"`, a name or rules choosing by the files in the
  directory: uv where there's a `uv.lock`, else pip; Maven, Gradle or sbt
  by build file) and its Renovate managers (`"renovate"`). GitHub Actions
  is covered at the root when the project has GitHub workflows (bumps to a
  generated workflow then belong in its template, which `hct check`
  enforces). Toolchain pins (`rust-toolchain`, `dotnet-sdk`, `.nvmrc`)
  aren't updated by the bots: hecatron resolves toolchain versions itself ([versions](versions.md)), and `hct check` would flag a pin that disagrees. Because the
  file lists every directory, adding a language or component makes `hecatron
  check` fail until it is regenerated.

The GitLab pipeline, caching and dependency update configuration are
written only by the Go binary; the scripts say so when a project's config asks for them.

Generators for the project's own code (sayable's locale codegen) stay in the
project. They can embed the same banner with `hct banner --generator
<path> --source <path> --prefix '# '`, so they don't import hecatron's Python
modules by path.
