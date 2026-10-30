# Build directory conventions

Each language's build artifacts land in directories named by its toolchain. These are not our choices — they're ecosystem conventions we match in each `impl/.gitignore`.

## Build output directories

| Convention | Build tool | Languages |
|------------|-----------|-----------|
| `build/` | CMake, Gradle, Gleam | Ada, C, COBOL, Fortran, Groovy, Java, Kotlin, Scala |
| `_build/` | Mix, rebar3, dune | Elixir, Erlang, OCaml |
| `.build/` | Swift Package Manager; xcodebuild's derived data in `.build/xcode/` (the default Xcode hook's `-derivedDataPath`) | Swift, Objective-C |
| `target/` | Cargo | Rust |
| `dist-newstyle/` | Cabal | Haskell |
| `zig-out/` | Zig build system | Zig |

The three `build/` variants reflect different ecosystem norms:

- **`build/`** — the most common convention, used by CMake and Gradle (JVM languages)
- **`_build/`** — BEAM ecosystem (Elixir, Erlang) and OCaml's dune. The underscore signals "generated, not source"
- **`.build/`** — Swift Package Manager. The dot makes it hidden on Unix, following Apple's convention of hiding tooling directories

## Cache and intermediate directories

| Directory | Tool | Languages |
|-----------|------|-----------|
| `.cache/` | clangd, ccls | C, C++ |
| `.cpcache/` | Clojure CLI tools | Clojure |
| `.dart_tool/` | Dart SDK | Dart |
| `.gradle/` | Gradle | Groovy, Java, Kotlin, Scala |
| `.phpunit.cache/` | PHPUnit | PHP |
| `.phpstan.cache/` | PHPStan | PHP |
| `.php-cs-fixer.cache` | PHP CS Fixer | PHP |
| `.zig-cache/` | Zig build system | Zig |
| `elm-stuff/` | Elm compiler | Elm |
| `node_modules/` | npm/pnpm | TypeScript |

## Dependency directories

| Directory | Tool | Languages |
|-----------|------|-----------|
| `vendor/` | Composer, Bundler | PHP, Ruby |
| `.bundle/` | Bundler | Ruby |
| `luarocks/` | LuaRocks | Lua |
| `_opam/` | opam (local switch) | OCaml |
| `local/` | cpanm/Carton | Perl |

## Compiled output subdirectories

Some languages split build output from object/intermediate files:

| Directory | Purpose | Languages |
|-----------|---------|-----------|
| `obj/` | Intermediate object files | Ada (GPRbuild), C#/.NET, F#/.NET |
| `bin/` | Compiled binaries | C#/.NET, F#/.NET, Go |

Ada produces both `build/` (final output) and `obj/` (ALI files, object files). .NET projects produce both `bin/` (assemblies) and `obj/` (intermediate build state) under each project directory.

## Lock files

| File | Tool | Languages |
|------|------|-----------|
| `composer.lock` | Composer | PHP |
| `Gemfile.lock` | Bundler | Ruby |
| `manifest.toml` | Gleam package manager | Gleam |
| `Package.resolved` | Swift Package Manager | Swift |
| `pubspec.lock` | Dart pub | Dart |
| `rebar.lock` | rebar3 | Erlang |

Lock files are gitignored for library implementations since they pin versions that downstream consumers shouldn't inherit.
