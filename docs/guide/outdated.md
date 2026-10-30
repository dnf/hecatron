# Outdated dependencies

`hct outdated` lists each component's direct dependencies that have a newer
version, with each ecosystem's own report. It only reports: it changes
nothing and passes unless a tool fails.

```
hct outdated                  # every language, in parallel
hct outdated go rust          # these languages, every component
hct outdated --json           # one JSON array on stdout
hct outdated --fail           # fail when anything is outdated
```

Each component prints a table, then one summary line:

```
  package   current  wanted  latest
  left-pad  1.2.0    1.2.0   1.3.0
.: typescript 1 outdated dependency
```

`wanted` is the newest version the manifest's constraints allow, where the
tool says (npm, pnpm, Yarn, Cargo, pub, Hex); `-` elsewhere. `--json`
prints the same rows as objects with `component`, `language`, `package`,
`current`, `wanted` and `latest`, running the languages one at a time with
their logs on stderr.

## What each language uses

| Language | Report |
| --- | --- |
| TypeScript, JavaScript | `npm outdated --json`, `pnpm outdated --format json`, `yarn outdated --json` (Yarn 1); for Yarn 2+ and Bun, npm's report on their `node_modules` |
| Python | `uv tree --outdated --depth 1` against `uv.lock`; without one, `uv pip list --outdated` on the `.venv` |
| Go | `go list -u -m -json all`, the direct requirements |
| Rust | `cargo update --dry-run --verbose` (Cargo names the newest version of each locked crate; no plugin), the direct dependencies |
| Ruby | `bundle outdated --parseable --only-explicit` |
| PHP | `composer outdated --locked --direct --format=json` |
| C#, F#, VB.NET | `dotnet list package --outdated --format json` |
| Elixir | `mix hex.outdated` |
| Dart | `dart pub outdated --json` (direct and dev dependencies) |
| Java, Kotlin, Scala, Groovy | Maven: the versions plugin's `display-dependency-updates`, at a pinned version. Gradle has no report of its own: where the build applies the [Versions plugin](https://github.com/ben-manes/gradle-versions-plugin), its `dependencyUpdates`; otherwise it says so. sbt has none. |
| Haskell | `cabal outdated` (Stack has none) |

Other languages have no report: `hct outdated <lang>` says so and
succeeds. Dependencies are installed first where the report reads them
(npm's, Bundler's, Hex's, pub's), which runs the packages' install
scripts as `hct deps` would. `HECATRON_OUTDATED_REPORT_DIR=<dir>`
saves each tool's raw report there.
