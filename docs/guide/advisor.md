# hct advisor

`hct doctor` keeps the **machine** working; `hct advisor` reads the
**project** and recommends what each language's ecosystem considers good
practice: lock files committed, toolchains pinned and still supported, a
formatter and linter configured, CI that runs every language. Each finding
says what to change, why in one sentence, and links to the ecosystem's own
documentation, so a repository in an unfamiliar language (or fifty of them)
gets the advice an expert in each would give.

It reads files only. It needs no network and no toolchain, runs no build
tool or auditor, and installs nothing; it answers in milliseconds. It needs
hecatron's binary (`bin/hecatron` without it says so).

```bash
hct advisor                       # every language and the repository
hct advisor go python             # some languages only
hct advisor --output json         # for scripts; agent output for AI agents
hct advisor --fail-on recommended # exit 1 if there is a recommended finding (CI)
hct advisor --fix                 # make the safe, mechanical changes
```

## Findings

Each finding has:

- an **id**, `<set>/<rule>`: the language (`go/sum-committed`), a set
  several languages share (`node/` for JavaScript and TypeScript, `dotnet/`,
  `gradle/`, `maven/`, `jvm/`, `native/` for C, C++, Fortran and
  Objective-C), or the repository (`repo/`, `ci/`, `audit/`);
- a **level**: `recommended` is what the ecosystem asks for; `suggestion` is
  good practice a project may have reasons to skip;
- a **category**: reproducibility, currency, tooling, security,
  performance or ci;
- the **directory** and the languages and components it is about; a finding
  two languages share in one directory (a `package.json` both JavaScript
  and TypeScript use) is listed once;
- what to change, **why**, a **link** to the ecosystem's documentation, and
  the **fix** `--fix` would make, if there is one.

`--output` is chosen as for `hct test`: the flag, else `HECATRON_OUTPUT`,
else agent output when an AI agent runs it (`CLAUDECODE=1` and no
terminal), else human. Agent output is a summary line and one line per
finding. JSON is one document: `root`, `config`, `languages`, `noAdvice`
(languages with no rules yet), `findings`, `ignored` and `fixed`.

The exit status is 0 whatever is found, unless `--fail-on recommended`
(any recommended finding) or `--fail-on suggestion` (any finding) says
otherwise; 2 is a usage error.

## --fix

`--fix` makes only changes that are safe and mechanical, and lists each:
today, creating a missing toolchain pin file at the version the component
runs now (`.python-version`, `.nvmrc`, `rust-toolchain.toml`,
`global.json`, `.ruby-version`), so nothing changes but that the version is
written down. It never edits code or an existing file, never installs
anything and never runs an ecosystem's tool; a missing lock file is for
`npm install`, `uv lock`, `cargo generate-lockfile` and the like, run by
you.

## Silencing a rule

In `.hecatron/config.json`:

```json
"advisor": {
  "ignore": [
    {"id": "rust/toolchain-pinned", "reason": "a library: CI tests every supported Rust"},
    {"id": "toolchain-pinned", "path": "tools", "reason": "scripts run with whatever is installed"}
  ]
}
```

An id with no slash silences the rule for every set; `path` limits an entry
to one directory (relative to the root). Every entry needs a reason. `hct
check` reports an entry without an id or a reason, or whose id names no
rule. Ignored findings are still listed, with their reasons, at the end
(and under `ignored` in JSON).

## The rules

Rules are data where they can be: `languages/<lang>/advice.json`, and
`languages/_advice.json` for the sets several languages include and the
repository's own rules. A project's `.hecatron/languages/<lang>/advice.json`
replaces the built-in one. Rules needing logic are Go code
(`internal/advisor`), shared by every language and parameterised by the
same files:

| Rule | What it judges |
| --- | --- |
| `<set>/toolchain-pinned` | The component's version comes from none of the pin files its definition reads (`pinFiles`); a version the project sets in `.hecatron` counts as chosen |
| `<set>/toolchain-end-of-life` | The version the project chose (a pin file, its `.hecatron` version, or the newest its classifiers or CI workflows support) is on a release line past its end of life, from the line's first release in `releases.json` and the set's policy: N newer lines, or M months, separately for LTS lines. hecatron's own default is never judged |
| `<set>/toolchain-behind` | That version is more than N release lines behind the newest |
| `<set>/lock-matches-manifest` | The lock file no longer records the manifest's dependencies (npm: `package-lock.json`'s entry for the package, as `npm ci` checks) |
| `<set>/ci-covers` | No CI configuration (GitHub Actions, GitLab CI, CircleCI, Azure Pipelines, Travis CI, Jenkins, Buildkite, Woodpecker, AppVeyor, Cirrus, Drone) mentions the language's tools or runs it through `hct`; a workflow calling another repository's reusable workflow isn't judged. With hecatron's generated workflows, `ci/workflow-current` instead: the workflow is written, current and covers every component, as `hct check` checks |
| `ci/configured` | The repository has no CI configuration at all |
| `audit/failed` | The last `hct audit` of the language failed (from its log; the advisor runs no auditor) |
| `audit/acceptance-expired` | An `audit.ignore` entry's `until` date has passed |

Each rule over files has an `id`, `category`, `level`, an optional `scope`
(`component`, the default: each component's directory; or `root`), an
optional `when` and an `expect` condition, a `message`, a one-sentence
`why` and a `link` to the ecosystem's own documentation. Where `when` holds
and `expect` doesn't, the advisor reports the message. Conditions are a
definition's step conditions ([design record §19](../contributing/design.md#language-definitions-data-families-scripts)): `file`, `missing`,
`tracked` (committed to git), `contains` (`[glob, regex]`), with `"up":
true` to look in each directory up to the root as tools that search upward
do, and `all`, `any`, `not`. For example:

```json
{
  "id": "sum-committed",
  "category": "reproducibility",
  "level": "recommended",
  "when": {"contains": ["go.mod", "(?m)^require\\b"]},
  "expect": {"tracked": "go.sum"},
  "message": "Commit go.sum beside go.mod",
  "why": "go.sum holds the checksums the go command verifies every module download against; Go's documentation says to commit it with go.mod.",
  "link": "https://go.dev/ref/mod#go-sum-files"
}
```

A set's `pin` (`when`, `level`, `create`, `content` with `{version}` and
`{line}`, `lineParts`, `minParts`), `endOfLife` (`lineParts`, `lines`,
`months`, `ltsLines`, `ltsMonths`), `behind` (`lineParts`, `lines`),
`lockFresh` (`manifest`, `lock`, `format`) and `ci` (`markers`: regexes)
parameterise the Go rules; each also carries its `why` and `link`. Every
rule's link is checked weekly with the documentation's
(`libexec/maintenance/check_links.py`), and every rule over files has a
fixture that triggers it and one that doesn't
(`internal/advisor/testdata/rules.json`).

Languages with rules: Bash, C, C++, C#, Dart, F#, Fortran, Go, Groovy,
Java, JavaScript, Kotlin, Lua, Objective-C, PHP, PowerShell, Python, Ruby,
Rust, Scala, Swift, TypeScript and VB.NET (Bash and Swift: CI coverage
only, so far). The advisor names the others ("No advice yet for: ...").

| Rule | Level | Category | Checks | Source |
| --- | --- | --- | --- | --- |
| `repo/branch-pinned-action` | recommended | security | Pin each GitHub Action to a release tag or a commit, not a branch (a uses: line names @main, @master or another branch) | [docs.github.com](https://docs.github.com/en/actions/reference/security/secure-use#using-third-party-actions) |
| `node/lock-committed` | recommended | reproducibility | Commit the package manager's lock file (package-lock.json, pnpm-lock.yaml, yarn.lock or bun.lock) | [docs.npmjs.com](https://docs.npmjs.com/cli/configuring-npm/package-lock-json) |
| `node/npm-ci` | suggestion | performance | Install with npm ci in CI, not npm install | [docs.npmjs.com](https://docs.npmjs.com/cli/commands/npm-ci) |
| `node/linter` | suggestion | tooling | Configure a linter (ESLint's eslint.config.js, or Biome's biome.json) | [eslint.org](https://eslint.org/docs/latest/use/getting-started) |
| `node/formatter` | suggestion | tooling | Configure a formatter (Prettier's .prettierrc, or Biome's biome.json) | [prettier.io](https://prettier.io/docs/install) |
| `node/toolchain-pinned` | suggestion | reproducibility | Pin the toolchain in `.nvmrc` (`--fix` creates it) | [github.com](https://github.com/nvm-sh/nvm#nvmrc) |
| `node/toolchain-end-of-life` | recommended | currency | The version the project chose is past its line's end of life (8 months, LTS: 37 months) | [nodejs.org](https://nodejs.org/en/about/previous-releases) |
| `node/lock-matches-manifest` | recommended | reproducibility | `package-lock.json` records the dependencies `package.json` declares | [docs.npmjs.com](https://docs.npmjs.com/cli/commands/npm-ci) |
| `node/ci-covers` | suggestion | ci | Some CI configuration runs the language | [docs.github.com](https://docs.github.com/en/actions/tutorials/build-and-test-code/nodejs) |
| `dotnet/editorconfig` | suggestion | tooling | Add an .editorconfig with the code style and analyzer settings | [learn.microsoft.com](https://learn.microsoft.com/dotnet/fundamentals/code-analysis/configuration-files) |
| `dotnet/toolchain-pinned` | suggestion | reproducibility | Pin the toolchain in `global.json` (`--fix` creates it) | [learn.microsoft.com](https://learn.microsoft.com/dotnet/core/tools/global-json) |
| `dotnet/toolchain-end-of-life` | recommended | currency | The version the project chose is past its line's end of life (24 months, LTS: 36 months) | [dotnet.microsoft.com](https://dotnet.microsoft.com/platform/support/policy/dotnet-core) |
| `dotnet/ci-covers` | suggestion | ci | Some CI configuration runs the language | [docs.github.com](https://docs.github.com/en/actions/tutorials/build-and-test-code/net) |
| `gradle/wrapper` | recommended | reproducibility | Commit the Gradle wrapper (gradlew and gradle/wrapper/), made with gradle wrapper | [docs.gradle.org](https://docs.gradle.org/current/userguide/gradle_wrapper.html) |
| `gradle/build-cache` | suggestion | performance | Turn on Gradle's build cache (org.gradle.caching=true in gradle.properties) | [docs.gradle.org](https://docs.gradle.org/current/userguide/build_cache.html) |
| `gradle/configuration-cache` | suggestion | performance | Turn on Gradle's configuration cache (org.gradle.configuration-cache=true in gradle.properties) | [docs.gradle.org](https://docs.gradle.org/current/userguide/configuration_cache.html) |
| `maven/wrapper` | suggestion | reproducibility | Commit the Maven wrapper (mvnw and .mvn/wrapper/), made with mvn wrapper:wrapper | [maven.apache.org](https://maven.apache.org/tools/wrapper/) |
| `jvm/toolchain-end-of-life` | recommended | currency | The version the project chose is past its line's end of life (1 newer line(s)) | [adoptium.net](https://adoptium.net/support/) |
| `jvm/ci-covers` | suggestion | ci | Some CI configuration runs the language | [docs.github.com](https://docs.github.com/en/actions/tutorials/build-and-test-code/java-with-gradle) |
| `native/cmake-minimum-version` | recommended | currency | Raise cmake_minimum_required in CMakeLists.txt to 3.5 or later (or give a range, such as 3.5...3.31) | [cmake.org](https://cmake.org/cmake/help/latest/command/cmake_minimum_required.html) |
| `native/clang-format` | suggestion | tooling | Add a .clang-format file with the project's style | [clang.llvm.org](https://clang.llvm.org/docs/ClangFormatStyleOptions.html) |
| `native/ci-covers` | suggestion | ci | Some CI configuration runs the language | [docs.github.com](https://docs.github.com/en/actions/get-started/understand-github-actions) |
| `bash/ci-covers` | suggestion | ci | Some CI configuration runs the language | [docs.github.com](https://docs.github.com/en/actions/get-started/understand-github-actions) |
| `dart/lock-committed` | recommended | reproducibility | Commit pubspec.lock (this is an application package: publish_to: none) | [dart.dev](https://dart.dev/tools/pub/private-files) |
| `dart/analysis-options` | suggestion | tooling | Add analysis_options.yaml, including a lint set such as package:lints/recommended.yaml | [dart.dev](https://dart.dev/tools/analysis) |
| `dart/ci-covers` | suggestion | ci | Some CI configuration runs the language | [docs.github.com](https://docs.github.com/en/actions/get-started/understand-github-actions) |
| `go/sum-committed` | recommended | reproducibility | Commit go.sum beside go.mod | [go.dev](https://go.dev/ref/mod#go-sum-files) |
| `go/toolchain-end-of-life` | recommended | currency | The version the project chose is past its line's end of life (2 newer line(s)) | [go.dev](https://go.dev/doc/devel/release#policy) |
| `go/ci-covers` | suggestion | ci | Some CI configuration runs the language | [docs.github.com](https://docs.github.com/en/actions/tutorials/build-and-test-code/go) |
| `lua/luacheck-config` | suggestion | tooling | Add a .luacheckrc saying which Lua version and globals the code uses | [luacheck.readthedocs.io](https://luacheck.readthedocs.io/en/stable/config.html) |
| `lua/ci-covers` | suggestion | ci | Some CI configuration runs the language | [docs.github.com](https://docs.github.com/en/actions/get-started/understand-github-actions) |
| `php/lock-committed` | recommended | reproducibility | Commit composer.lock (this is a project, "type": "project", not a library) | [getcomposer.org](https://getcomposer.org/doc/01-basic-usage.md#commit-your-composer-lock-file-to-version-control) |
| `php/static-analysis` | suggestion | tooling | Configure a static analyser or style checker (phpstan.neon, psalm.xml, phpcs.xml or .php-cs-fixer.dist.php) | [phpstan.org](https://phpstan.org/user-guide/getting-started) |
| `php/toolchain-end-of-life` | recommended | currency | The version the project chose is past its line's end of life (50 months) | [php.net](https://www.php.net/supported-versions.php) |
| `php/ci-covers` | suggestion | ci | Some CI configuration runs the language | [docs.github.com](https://docs.github.com/en/actions/get-started/understand-github-actions) |
| `powershell/module-manifest` | suggestion | tooling | Add a module manifest (.psd1, made with New-ModuleManifest) beside the .psm1 | [learn.microsoft.com](https://learn.microsoft.com/powershell/scripting/developer/module/how-to-write-a-powershell-module-manifest) |
| `powershell/ci-covers` | suggestion | ci | Some CI configuration runs the language | [docs.github.com](https://docs.github.com/en/actions/tutorials/build-and-test-code/powershell) |
| `python/lock-committed` | recommended | reproducibility | Commit the lock file (uv.lock, poetry.lock, pdm.lock or pylock.toml): it is here but not in git | [docs.astral.sh](https://docs.astral.sh/uv/concepts/projects/layout/#the-lockfile) |
| `python/pyproject` | recommended | currency | Add a pyproject.toml declaring the build system ([build-system]), and move the metadata from setup.py into it | [packaging.python.org](https://packaging.python.org/en/latest/guides/modernize-setup-py-project/) |
| `python/lint-config` | suggestion | tooling | Configure a linter and formatter, such as Ruff ([tool.ruff] in pyproject.toml, or ruff.toml) | [docs.astral.sh](https://docs.astral.sh/ruff/configuration/) |
| `python/toolchain-pinned` | suggestion | reproducibility | Pin the toolchain in `.python-version` (`--fix` creates it) | [docs.astral.sh](https://docs.astral.sh/uv/concepts/python-versions/#python-version-files) |
| `python/toolchain-end-of-life` | recommended | currency | The version the project chose is past its line's end of life (61 months) | [devguide.python.org](https://devguide.python.org/versions/) |
| `python/ci-covers` | suggestion | ci | Some CI configuration runs the language | [docs.github.com](https://docs.github.com/en/actions/tutorials/build-and-test-code/python) |
| `ruby/lock-committed` | recommended | reproducibility | Commit Gemfile.lock (this is an application: it has no .gemspec) | [guides.rubygems.org](https://guides.rubygems.org/faqs/) |
| `ruby/lint-config` | suggestion | tooling | Configure RuboCop (.rubocop.yml) or Standard (.standard.yml) | [docs.rubocop.org](https://docs.rubocop.org/rubocop/configuration.html) |
| `ruby/toolchain-pinned` | suggestion | reproducibility | Pin the toolchain in `.ruby-version` (`--fix` creates it) | [guides.rubygems.org](https://guides.rubygems.org/gemfile_ruby/) |
| `ruby/toolchain-end-of-life` | recommended | currency | The version the project chose is past its line's end of life (40 months) | [ruby-lang.org](https://www.ruby-lang.org/en/downloads/branches/) |
| `ruby/ci-covers` | suggestion | ci | Some CI configuration runs the language | [docs.github.com](https://docs.github.com/en/actions/tutorials/build-and-test-code/ruby) |
| `rust/lock-committed` | recommended | reproducibility | Commit Cargo.lock (this package builds a binary and has dependencies) | [doc.rust-lang.org](https://doc.rust-lang.org/cargo/faq.html#why-have-cargolock-in-version-control) |
| `rust/edition` | recommended | currency | Set edition in Cargo.toml's [package] (edition = "2024", or edition.workspace = true) | [doc.rust-lang.org](https://doc.rust-lang.org/cargo/reference/manifest.html#the-edition-field) |
| `rust/old-edition` | suggestion | currency | Move to a newer Rust edition (cargo fix --edition migrates the code) | [doc.rust-lang.org](https://doc.rust-lang.org/edition-guide/editions/transitioning-an-existing-project-to-a-new-edition.html) |
| `rust/toolchain-pinned` | suggestion | reproducibility | Pin the toolchain in `rust-toolchain.toml` (`--fix` creates it) | [rust-lang.github.io](https://rust-lang.github.io/rustup/overrides.html#the-toolchain-file) |
| `rust/toolchain-behind` | suggestion | currency | The version the project chose is more than 8 release lines behind the newest | [rust-lang.org](https://www.rust-lang.org/policies/security) |
| `rust/ci-covers` | suggestion | ci | Some CI configuration runs the language | [docs.github.com](https://docs.github.com/en/actions/tutorials/build-and-test-code/rust) |
| `swift/ci-covers` | suggestion | ci | Some CI configuration runs the language | [docs.github.com](https://docs.github.com/en/actions/tutorials/build-and-test-code/swift) |
| `typescript/strict` | suggestion | tooling | Turn on "strict": true in tsconfig.json's compilerOptions | [typescriptlang.org](https://www.typescriptlang.org/tsconfig/#strict) |

The thresholds are the ecosystems' published policies, rounded so a line
is reported a little after its end of life rather than before: Node.js
supports an odd-numbered line about eight months and an LTS line about
three years; Python five years; Ruby about three years and three months;
PHP four years to the end of a year; .NET three years (LTS) or two (STS);
Go until two newer lines exist; a non-LTS Java line until the next one.
Rust supports only its newest release, so a Rust toolchain is reported
when it is more than eight lines (about a year) behind, as a suggestion.
