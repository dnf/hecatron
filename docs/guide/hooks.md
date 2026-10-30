# Hooks

A hook is a bash script run once per component, from inside
`implPath` for that component and language, with the component name as `$1`.
It builds or installs whatever it needs and exits non-zero on failure. This is
the contract sayable's `test.sh`, `lint.sh`, `build.sh` and `clean.sh` already
followed; `libexec/lib/each-component.sh` (sayable's `each-library.sh`) is
hecatron's dispatcher.

A project's `.hecatron/languages/<lang>/<action>.sh` (`test.sh`, `lint.sh`,
`build.sh`, `clean.sh`, `deps.sh`, `format.sh`, `audit.sh`,
`outdated.sh`, `docs.sh`, `bench.sh`) replaces the built-in default for that language
([resolving a language's files](configuration.md#resolving-a-languages-files));
`version.sh` and `metadata.sh` serve `hct version` and `hct metadata`.

## The contract

Rules carried over from sayable:

- Every component runs even after one fails, and the summary names the
  failures.
- `clean` removes build artifacts only, never installed dependencies or
  tracked files.
- A language-specific command acts on every component of that language.
- A hook with nothing to check in a component (an audit with no lock file
  its auditor reads) writes why to the file `HECATRON_SKIPPED` names and
  exits 0: the component is skipped, not passed (`skip_component` in
  `languages/_hooks.sh` does both; [audit](audit.md)).

## Default hooks

Many hooks are the language's standard command (`go test
./...`, `cargo test`, `cargo clippy --all-targets -- -D warnings`, `dotnet
test`). Hecatron ships these as defaults under `languages/<lang>/hooks/`, so a
conventional project needs no hook files at all. Sayable's hooks, kept in
`examples/sayable/`, are the starting point for the defaults; where they name
sayable's files (`test_format_${1}.wren`), the default uses a convention and
the project overrides.

## Where the build is

A language's directory isn't always where its build
file is. When it has none, a default hook looks below it (up to three levels,
skipping vendored code, test data and build output) and runs itself in each
of the shallowest directories that have one, failing if any fails (several
crates under `rust/`, a solution in `csharp/src/`); else it uses the nearest
build file above it, never outside the project (`sdk/go` in `sdk/go.mod`, a
root `Package.swift` or CMake project that builds `swift/` or `src/cpp`).
Where the tool itself finds its build file upward (go, cargo, Bundler and
Rake, a Gradle settings file), above comes first and the tool runs from the
directory, scoped to it as far as the tool allows; elsewhere the hook acts
where the build file is (`swift --package-path`, the CMake project's
`build/`). A CMake or Meson file counts only when it declares a `project()`,
and a root Makefile only through a target named for the directory or language
(`check_cxx` for `cxx/`). A directory inside a Gradle build that isn't
one of its projects runs Gradle from the project it belongs to: the one
whose sources it holds (`moshi` for `moshi/src/main/java`), or the root of a
build of one project. JVM tests run Maven's package phase in a
multi-module build (its modules can need each other's jars), Gradle's
`jvmTest`, else `allTests`, in a Kotlin Multiplatform build (no `test`
task), and Mill's `__.test` where the build is `build.mill`; .NET tests roll forward to the
installed runtime, and a directory with several projects and no solution
tests each test project. CMake and Meson build in `build/`, or `_build/`
when the project keeps files of its own in `build/` (Zstandard's CMake
build); a CMake project whose files never set up a test (no `add_test`,
`enable_testing` or CTest) has nothing to test, and its test hook skips
after building, while one that does but registered none fails, unless
its build made GoogleTest programs CTest doesn't know of (LightGBM's
`testlightgbm`, an executable carrying GoogleTest's flags), which then run
as its tests. The C, C++ and
Objective-C installers install Meson where a component builds with it; where
a `meson.build` asks for a newer Meson (`meson_version: '>=1.8.3'`) than the
distribution has, Linux gets the current release from PyPI as a uv tool.
They also install the common libraries a component's build needs, as
distribution packages: those a CMake build's `find_package()` and
`pkg_check_modules()` name, and, for a build without CMake (a Makefile),
those its sources `#include` (`<zlib.h>` is zlib's) unless the project has
the header itself. CTest runs
from the build directory of the subdirectory that calls `enable_testing()`
when the top level doesn't (Zstandard's `build/cmake/`). A Makefile's test
hook builds the default target before `make check` or `make test`, which
needn't depend on everything the tests run (Redis's). Node.js hooks and installers use the package manager
the package declares (`packageManager`, else its lockfile, here or in the
workspace above; pnpm and Yarn through Corepack, with Corepack's shim on
PATH for scripts that call the package manager by name: in
`~/.cache/hecatron/corepack-shims`, a static `path` entry of both Node.js
languages, so other languages' hooks have it too), installing Corepack
or Bun only when a component needs them. The Node.js test hook runs the
package's `test` script; a workspace root without one runs its packages'
(`workspaces`, `pnpm-workspace.yaml`); a package without one runs its
configured runner (`vitest.config.*`, `jest.config.*`, `.mocharc.*`); else,
and in a directory without a `package.json`, it tests the language's
packages below (TypeScript's have a `tsconfig.json`, but for one beside
mostly JavaScript, which is JavaScript's: detection's `detectYields`, the
same rule through `project.py`). It sets `CI=true` (unless it's set) so watching test
runners run once. A package its tests need built (a native addon, from a `napi`
key, a `binding.gyp` or a build script running napi or node-gyp, whose
`*.node` isn't built; or a `main` or `exports` entry point that isn't
there) is built first: its build script, else `napi build --platform` or
`node-gyp rebuild`. So is its workspace, when any workspace package needs
it (one the tests import may): `pnpm -r run build`, `npm run build
--workspaces`, `yarn workspaces foreach --topological run build`, else
each package in turn. pytest also collects `Test*.py` files where no test
file has its default names (`test_*.py`, `*_test.py`) and the project sets
no `python_files`, and leaves out the Python projects inside the one it tests
(`--ignore=pydantic-core`: their environments aren't the outer one's) unless
the project sets `testpaths`. Where there is no `python`, only `python3`, Python's
installer links one to it in `~/.local/share/hecatron/python/bin`, a static
`path` entry of Python's, so the builds of a project's other languages that
call `python` (a Rust build script, a CMake project's) find it, and nothing
outside hecatron's hooks does.

## Dependencies

A `deps` hook fetches a component's dependencies with its
ecosystem's tool, held to the lock file when there is one (it fails rather
than change it). Hooks that need dependencies installed (Node.js's
`node_modules`, Bundler's bundle, Composer's `vendor/`, Mix's, pub's,
Shards') install them when they're missing through the same function
(`node_fetch`, `bundle_fetch`... in `languages/_hooks.sh`), so `deps` is
never a required first step; where the build tool fetches by itself (go,
cargo, dotnet, `uv run`, SwiftPM, Maven, Gradle) the hooks leave it to the
tool. Installers install each component's dependencies by running
`hct deps`, so a project's own `deps.sh` applies there too.

## Formatting

A `format` hook rewrites files with the language's
formatter, or with `HECATRON_FORMAT_CHECK=1` only checks. `lint` runs that
check after the lint hook, for each component, unless the project turns it
off (`lint.format`) or the lint hook checked formatting itself (it calls
`checks_format`), so a project overriding `lint.sh` still gets it. A
formatter runs by default only where it ships with the toolchain or nearly
all of the language's code follows it (gofmt, rustfmt, `dotnet format`);
elsewhere only where the project configures one (Prettier, clang-format,
Spotless), since a default would rewrite every file of a project that
doesn't use it. (Owner's brief, 2026-10-27; the per-formatter choices are
the implementation's, in [format.md](format.md).)

## Docs and benchmarks

A `docs` hook builds the component's API documentation with the
ecosystem's tool into the tool's default directory (`cargo doc --no-deps`
into the target directory's `doc/`, Sphinx into `docs/_build/html`) and
ends by saying where, with `docs_written <dir>` from `languages/_hooks.sh`:
`hct docs` repeats those lines after a parallel run, and puts them in agent
and JSON output (`"docs"`). A `bench` hook runs the component's benchmarks
(`cargo bench`, `go test -run '^$' -bench . ./...`). Both run only what the
component already has (the toolchain's tool, or one among its dependencies
or configuration) and otherwise `skip`, saying what they looked for. The
shared functions are in `languages/_docs.sh` and `_bench.sh`. A project
hook replaces the default, here MkDocs for Sphinx:

```bash
#!/usr/bin/env bash
# .hecatron/languages/python/docs.sh
source "${HECATRON_HOME}/languages/_hooks.sh"
run uv run --with mkdocs mkdocs build --strict
docs_written site
```

## Environment

Environment for hooks: `HECATRON_ROOT` (project root), `HECATRON_COMPONENT`,
`HECATRON_LANG`, `HECATRON_HOME`, and `_common.sh` helpers available by
sourcing `$HECATRON_HOME/languages/_common.sh`.

## Trust

Installers run with the user's privileges and some pipe a downloaded script
into bash (Homebrew, Pharo's zeroconf, rustup). Hecatron only runs an
installer when asked (`install`, `doctor --fix`) and names each one before it
runs. Hooks are project code: running `hct test` in a repository runs
that repository's scripts, exactly like running its justfile or Makefile, so
the same trust applies. Hecatron never runs hooks on its own (no shell
integration, no git hooks).
