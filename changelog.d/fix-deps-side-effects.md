- **`hct format --check` installs nothing:** where the formatter comes from
  the project's dependencies (Prettier, Biome, RuboCop, Standard, PHP CS
  Fixer, Pint, `mix format`'s plugins) and they aren't installed, the check
  (and the one in `hct lint`) skips the component, saying it wasn't checked,
  instead of running `npm ci`, `bundle install`, `composer install` or `mix
  deps.get`. `hct format` still installs them first.
- **`--no-deps` for `hct install` and `hct doctor --fix`:** repairs
  toolchains without installing the project's dependencies.
- **`hct doctor --output json`:** each language's problems and notes, for
  scripts and agents.
- **`hct install javascript` installs the Node.js version the project
  resolves**, as TypeScript's already did; on macOS it installed only
  Homebrew's latest, and `hct doctor` went on reporting the version missing.
- **Python: Homebrew's versioned Pythons (`python@3.11`) no longer count as
  installed.** They have no `bin/python3`, so uv, pointed at it, failed with
  "No interpreter found" in `hct outdated`, `hct format` and `hct audit`
  while `hct doctor` said ok; now the installer installs uv's own.
- **`hct outdated` fails when the tool fails without a report**, rather than
  reporting nothing outdated.
- **Ruby's `hct doctor`** says when the Gemfile's gems aren't installed
  (`hct deps ruby`), or Bundler isn't, rather than "bundle not installed".
