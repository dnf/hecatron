- **`hct licenses` and `hct sbom` take `--timeout DURATION`** (and
  `HECATRON_TIMEOUT`), with `hct audit`'s syntax: `90s`, `30m`, `1h30m`, a
  number of seconds, `0` for none. Past it, OSV-Scanner is stopped and the
  command exits 1. There's no limit by default.
- **`hct licenses` lists the project's own packages apart**, out of the
  dependencies, the unknown count and the policy: uv's editable root (a dev
  version like `0.27.0.post18.dev0+g1a2b3c4` used to show as `UNKNOWN`) and
  workspace members, npm workspaces, Cargo workspace members, a
  `Gemfile.lock`'s `PATH` gems, Go's `replace` with a directory, Composer
  path repositories, NuGet project references, Poetry directory
  dependencies, and any package a component's manifest names. JSON: `own`.
- **`hct licenses` classifies copyleft.** Each package's JSON has
  `licenseIds` (the expression's identifiers) and `copyleft` (`none`,
  `weak`, `strong`, `network`, `maximal`, `unclassified`, or `unknown`),
  from Blue Oak Council's copyleft and permissive lists, for the expression
  as a whole (`OR`: the least restrictive choice; `AND`: the most
  restrictive part). Licenses are rolled up per identifier too
  (`licenseIds`, so `Apache-2.0 AND LGPL-3.0-or-later` counts for each),
  every rollup says its class, and the summary counts copyleft packages
  (`copyleft` in JSON).
- **`hct licenses` and `hct sbom` warn of a lock file outside every
  component** of its language (a root `package-lock.json` in a monorepo
  whose components are its subdirectories), whose packages they don't read.
- **The "detected" note is for a person at a terminal.** `... has no
  .hecatron/config.json; detected javascript` is no longer printed when
  stderr isn't a terminal, with `--output` or `--json` or `HECATRON_OUTPUT`
  naming another format than human, in an agent's session or in GitHub
  Actions, as the update notice already wasn't (which now also heeds
  `--json`). `hct status` still says the config is detected.
