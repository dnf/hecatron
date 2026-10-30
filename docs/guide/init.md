# hct init and detection

`hct init [dir]` sets up an existing repository (default: the current
directory). Without it, a git repository is still a project: with no
`.hecatron/config.json` above, its root is the repository's top level and its
configuration what init would write, detected and cached in
`.git/hecatron/detected.json`, keyed on the commit, git's index and
hecatron's version, so a command pays for detection only after the tracked
files change (about 17 µs a file: 4 ms for 100 files, 0.8 s for 47,000), and
nothing appears in the working tree or `git status`. Logs and `local.json`
go in `.git/hecatron/` too. Changing the configuration (`add`, `remove`)
saves it, as init would. Init itself:

1. Refuses if `dir` isn't inside a git repository, and is a no-op on an
   already initialised project unless `--force`.
2. Detects languages from their build files: `go.mod` → go, `Cargo.toml` →
   rust, `tsconfig.json` → typescript, `pyproject.toml` → python, `*.cabal` →
   haskell, and so on (a definition's `detect` globs). A language with no
   build file is detected from its sources (`detectSources`, such as `*.c`),
   keeping only the directories outside every other language's: C files in
   `python/ext/` are Python's extension, and C found only inside `cpp/` isn't
   added. A build file several languages share (`sharedBuildFiles`, such as
   C's and C++'s `CMakeLists.txt`) counts for the one with the most sources
   beside it or under its `src/` and `include/`. A hit is dropped where one
   of the definition's `detectUnless` files sits beside it (`package.json` is
   JavaScript unless a `tsconfig.json` makes it TypeScript), except where
   the other definition's `detectYields` rule gives it back: a
   `tsconfig.json` beside more JavaScript than TypeScript files (`.d.ts`
   declarations aside; tracked files in the directory and below, as
   detection counts them) is a JavaScript package's, as antlr4's
   `runtime/JavaScript` with its type declarations is. Committed build
   output doesn't count, so a TypeScript package that tracks its compiled
   JavaScript stays TypeScript: `dist/`, `build/`, `out/` and `lib/`
   directly below, the `compilerOptions.outDir` its `tsconfig.json` names
   (read with comments and trailing commas), and a `.js` beside a `.ts` of
   the same name. A
   `package.json` counts only with one of the definition's `detectKeys`
   (JavaScript's: runtime `dependencies`, an entry point `main`, `bin`,
   `exports`, `module` or `browser`, `workspaces`, or a `start`, `dev`,
   `serve` or `test` script), so an Express app without tests is
   JavaScript, and a `package.json` carrying only tooling (husky,
   commitlint and Prettier in `devDependencies`, `prepare` and `lint`
   scripts: an iOS SDK's commit hooks, Cucumber's `jsonschema/`) isn't. Some
   build files count only beside the language's code, in their directory or
   below (`detectWithSources`): a `requirements.txt` with `*.py` files is
   Python (an app with no `pyproject.toml`), one for a docs build isn't; an
   Xcode project or workspace (`*.xcodeproj/`, `*.xcworkspace/`) is Swift
   with `*.swift` files and Objective-C with `*.m` or `*.mm`, each source
   counting for the innermost project holding it. A `Gemfile` is Ruby even
   when it only installs fastlane: that is Ruby tooling the project runs.
   Dotfiles aren't sources (a `.nvim.lua` configures an editor).
   Examples, tests and test data, docs, tools, hooks, a generator's
   templates and vendored code (`examples/`, `tests/`, `tests_data/`,
   `tools/`, `hooks/`, `templates/`, `fixtures/`, `android/`,
   `third_party/`, `vendor/`, `vendored/`, `node_modules/`, CocoaPods'
   `Pods/`, Carthage's `Carthage/` and the like, in any case:
   `Examples/`) and hidden directories (`.jenkins/`) are skipped, except a
   definition's `detectTests` files (Bash's `*.bats`) in a `test/` or
   `tests/` directory, which, for a language not otherwise found, make it
   the language of the directory tested (bats-core, Bash-it). A language
   left out is listed with the reason.
3. Detects the layout. A language's directory may carry any of its
   definition's `dirNames` (C#'s `dotnet/` or `netstd/`, Ruby's `rb/`). If
   the directories share a pattern with one part varying (`age/impl/go`,
   `date/impl/go`) and every language has a directory named for it, it
   proposes `components` and `implPath`; otherwise one
   component, at the pattern under `sdks/`, `sdk/`, `lib/`, `lang/`,
   `bindings/`, `ffi/`, `packages/` or the root if there is one, else the
   pattern covering most languages. A language whose directory doesn't fit
   gets its own entry in `implPaths`. A directory the pattern names fits
   without the language's build file only if it holds the language's code:
   one of its `detect` or `detectSources` files, or, for a definition without
   `detectSources`, any file when it sits at the root or under one of the
   parents above (`lib/`, `sdks/`...: Thrift's `lib/go`, whose `go.mod` is at
   the root). So a build file at the root keeps a language there beside a
   `languages/go/` holding no Go. A build file more than three levels below
   a directory doesn't count, since the hooks look no deeper: with only such
   build files the directory doesn't fit, and the language gets the
   directory holding them (antlr4's `runtime/Go/antlr/v4`, not the root). A directory another language was found in
   doesn't fit (Cucumber's `javascript/` is TypeScript's, so JavaScript found
   in `jsonschema/` gets `implPaths` `jsonschema`). `--components` and
   `--impl-path` set the layout explicitly.
4. Writes `.hecatron/config.json`, adds `.hecatron/local.json` and the log
   directory to `.gitignore`.
5. Writes no hooks: defaults apply until the project adds its own. `--hooks`
   copies the defaults in for editing.
6. With `--justfile`, writes `.hecatron/hecatron.just` with recipes that
   delegate to `hecatron` (`test *args: hct test {{args}}`) for a project
   justfile to `import`.
7. Writes a short section for AI coding agents into `AGENTS.md` (the file
   most agents read), between `<!-- hecatron:begin -->` and
   `<!-- hecatron:end -->` so a later init replaces only it, and adds
   `@AGENTS.md` to an existing `CLAUDE.md` (Claude Code reads that instead).
   The section is generic (no languages, which would go stale) and points
   to `hct help agents`, which prints `docs/agents.md`, the compact
   reference for the installed version. `--no-agents` skips this. Only an
   explicit init writes it: a detected project writes nothing.
8. Prints what it found and the next step: `hct doctor`.

It never installs anything or touches CI by itself; `hct workflows` and
`hct dockerfile` are separate, explicit steps.
