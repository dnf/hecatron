<!-- Generated from languages/*/config.json by libexec/codegen/skill.py. -->

# Each language's default tools

What `hct test`, `lint`, `format` and `build` run for a language when the
project has no hook of its own (`.hecatron/languages/<lang>/<action>.sh`).
The key is what to type (`hct test csharp`).

| Language | Key | Build | Test | Lint | Format |
| --- | --- | --- | --- | --- | --- |
| Ada | `ada` | Alire, GPRbuild | Alire, GPRbuild test projects | – | – |
| Bash | `bash` | – | Bats | ShellCheck | – |
| C | `c` | CMake, Meson, Autotools, make | CTest, Meson test, make check | – | clang-format |
| C# | `csharp` | .NET SDK (dotnet) | dotnet test | dotnet format | dotnet format |
| C++ | `cpp` | CMake, Meson, Autotools, make | CTest, Meson test, make check | – | clang-format |
| Clojure | `clojure` | Clojure CLI (deps.edn), Leiningen | clojure.test | clj-kondo | cljfmt |
| COBOL | `cobol` | CMake, make | CTest, make check | – | – |
| Common Lisp | `lisp` | ASDF | ASDF test-op | – | – |
| Crystal | `crystal` | Shards | crystal spec | crystal tool format | crystal tool format |
| D | `d` | dub | unittest blocks (dub test) | D-Scanner | – |
| Dart | `dart` | pub (dart, flutter) | package:test | dart analyze | dart format |
| Elixir | `elixir` | Mix | ExUnit | mix format, Credo | mix format |
| Elm | `elm` | elm | elm-test | elm-review | elm-format |
| Erlang | `erlang` | rebar3 | EUnit, Common Test | Dialyzer | erlfmt |
| F# | `fsharp` | .NET SDK (dotnet) | dotnet test | Fantomas | Fantomas |
| Fortran | `fortran` | fpm, CMake, Meson, make | fpm test, CTest | – | – |
| Gleam | `gleam` | gleam | gleeunit (gleam test) | gleam format | gleam format |
| Go | `go` | go modules | go test | golangci-lint | gofmt |
| Goose | `goose` | goose | test programs | goose check | – |
| Groovy | `groovy` | Gradle, Maven, groovyc | JUnit, Spock | Gradle check | Spotless |
| Haskell | `haskell` | Stack, cabal | test suites | HLint, cabal check | Fourmolu, Ormolu |
| Haxe | `haxe` | haxelib (.hxml) | utest | – | – |
| Janet | `janet` | jpm | jpm test | – | – |
| Java | `java` | Maven, Gradle | JUnit | Gradle check | Spotless, fmt-maven-plugin |
| JavaScript | `javascript` | npm, pnpm, Yarn or Bun | npm test (node:test, Jest...) | npm run lint (ESLint...) | Prettier, Biome |
| Julia | `julia` | Pkg | Test (Pkg.test) | – | JuliaFormatter |
| Kotlin | `kotlin` | Gradle, Maven, kotlinc | JUnit, kotlin.test | Gradle check (detekt, ktlint) | Spotless, ktlint-gradle, Kotlinter |
| Lua | `lua` | LuaRocks | busted | luacheck | StyLua |
| Mojo | `mojo` | mojo package | test programs | mojo format | mojo format |
| Nim | `nim` | Nimble | nimble test | – | – |
| Objective-C | `objectivec` | Xcode, GNUstep make, CMake, make | XCTest, test programs | – | clang-format |
| OCaml | `ocaml` | dune | dune runtest | ocamlformat, dune @check | ocamlformat |
| Octave | `octave` | Octave packages | %!test blocks | – | – |
| Odin | `odin` | odin | @(test) procedures | odin vet | – |
| Pascal | `pascal` | Free Pascal, make | test programs, make check | – | – |
| Perl | `perl` | ExtUtils::MakeMaker, Module::Build, Carton | prove (Test::More) | Perl::Critic | perltidy |
| Pharo | `pharo` | Metacello (Tonel) | SUnit | – | – |
| PHP | `php` | Composer | PHPUnit | PHPStan, PHP_CodeSniffer, PHP CS Fixer | PHP CS Fixer, Laravel Pint |
| PowerShell | `powershell` | PowerShell modules | Pester | PSScriptAnalyzer | – |
| Prolog | `prolog` | SWI-Prolog packs | plunit | – | – |
| Python | `python` | uv (pyproject.toml, requirements.txt), setuptools | pytest | Ruff | Ruff, Black |
| R | `r` | R packages (DESCRIPTION) | testthat | lintr | Air |
| Racket | `racket` | raco | rackunit (raco test) | – | – |
| Raku | `raku` | zef | Test (prove6, zef test) | – | – |
| Ruby | `ruby` | Bundler, Rake, RubyGems | RSpec, Minitest | RuboCop, Standard | RuboCop, Standard |
| Rust | `rust` | Cargo | cargo test | Clippy, rustfmt | rustfmt |
| Scala | `scala` | sbt, Gradle, Maven, scalac | MUnit, ScalaTest | – | sbt-scalafmt, Spotless |
| Swift | `swift` | Swift Package Manager, Xcode (xcodebuild build and clean) | XCTest, Swift Testing | – | swift-format |
| Tcl | `tcl` | TEA (make) | tcltest | – | – |
| TypeScript | `typescript` | npm, pnpm, Yarn or Bun (tsc) | npm test | npm run lint, tsc --noEmit | Prettier, Biome |
| V | `v` | v | v test | v fmt, v vet | v fmt |
| VB.NET | `vbnet` | .NET SDK (dotnet) | dotnet test | dotnet format | dotnet format |
| Wren | `wren` | wren_cli | test scripts | – | – |
| Zig | `zig` | zig build | zig build test | zig fmt | zig fmt |
