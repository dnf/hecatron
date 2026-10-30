# Languages

Every language hecatron supports, what it is, and how hecatron builds,
tests, lints and formats it by default. A project overrides any default with its own
hook (`.hecatron/languages/<lang>/<action>.sh`).

Generated from each language's `config.json` by
`libexec/codegen/languages_doc.py`; don't edit by hand.
[coverage.md](coverage.md) has which platforms each one installs and passes
on, and [adding-a-language.md](../contributing/adding-a-language.md) covers adding one.

## Popular languages

The languages at least 5% of developers use, from the
[Stack Overflow Developer Survey 2025](https://survey.stackoverflow.co/2025/technology).

| Language | What it is | Build systems | Tests | Linting | Formatting | Default version |
| --- | --- | --- | --- | --- | --- | --- |
| [Bash](https://www.gnu.org/software/bash/) | The Unix shell and scripting language. | – | [Bats](https://github.com/bats-core/bats-core) | [ShellCheck](https://www.shellcheck.net) | – | – |
| [C](https://www.open-std.org/jtc1/sc22/wg14/) | The portable systems language most others build on. | [CMake](https://cmake.org), [Meson](https://mesonbuild.com), [Autotools](https://www.gnu.org/software/automake/), [make](https://www.gnu.org/software/make/) | [CTest](https://cmake.org/cmake/help/latest/manual/ctest.1.html), [Meson](https://mesonbuild.com) test, [make](https://www.gnu.org/software/make/) check | – | [clang-format](https://clang.llvm.org/docs/ClangFormat.html)[^1] | – |
| [C#](https://learn.microsoft.com/dotnet/csharp/) | Microsoft's general-purpose language for .NET. | [.NET SDK](https://dotnet.microsoft.com) (dotnet) | [dotnet test](https://learn.microsoft.com/dotnet/core/tools/dotnet-test) | [dotnet format](https://learn.microsoft.com/dotnet/core/tools/dotnet-format) | [dotnet format](https://learn.microsoft.com/dotnet/core/tools/dotnet-format) | 10.0.403 (LTS) |
| [C++](https://isocpp.org) | Zero-overhead systems language with templates and RAII. | [CMake](https://cmake.org), [Meson](https://mesonbuild.com), [Autotools](https://www.gnu.org/software/automake/), [make](https://www.gnu.org/software/make/) | [CTest](https://cmake.org/cmake/help/latest/manual/ctest.1.html), [Meson](https://mesonbuild.com) test, [make](https://www.gnu.org/software/make/) check | – | [clang-format](https://clang.llvm.org/docs/ClangFormat.html)[^1] | – |
| [Dart](https://dart.dev) | Client-optimized language behind Flutter. | [pub](https://dart.dev/tools/pub/cmd) (dart, flutter) | [package:test](https://pub.dev/packages/test) | [dart analyze](https://dart.dev/tools/dart-analyze) | [dart format](https://dart.dev/tools/dart-format) | – |
| [Go](https://go.dev) | Simple, fast-compiling language with built-in concurrency. | [go modules](https://go.dev/ref/mod) | [go test](https://pkg.go.dev/cmd/go#hdr-Test_packages) | [golangci-lint](https://golangci-lint.run) | [gofmt](https://pkg.go.dev/cmd/gofmt) | 1.27.3 |
| [Java](https://dev.java) | The JVM's flagship language. | [Maven](https://maven.apache.org), [Gradle](https://gradle.org) | [JUnit](https://junit.org) | [Gradle](https://gradle.org) check | [Spotless](https://github.com/diffplug/spotless), [fmt-maven-plugin](https://github.com/spotify/fmt-maven-plugin)[^2] | 25.0.5 (LTS) |
| [JavaScript](https://developer.mozilla.org/docs/Web/JavaScript) | The web's language, on Node.js. | [npm](https://docs.npmjs.com/cli), [pnpm](https://pnpm.io), [Yarn](https://yarnpkg.com) or [Bun](https://bun.sh) | [npm](https://docs.npmjs.com/cli) test ([node:test](https://nodejs.org/api/test.html), [Jest](https://jestjs.io)...) | [npm](https://docs.npmjs.com/cli) run lint ([ESLint](https://eslint.org)...) | [Prettier](https://prettier.io), [Biome](https://biomejs.dev)[^3] | 24.22.1 (LTS) |
| [Kotlin](https://kotlinlang.org) | Concise, null-safe language for the JVM, Android and beyond. | [Gradle](https://gradle.org), [Maven](https://maven.apache.org), [kotlinc](https://kotlinlang.org/docs/command-line.html) | [JUnit](https://junit.org), [kotlin.test](https://kotlinlang.org/api/core/kotlin-test/) | [Gradle](https://gradle.org) check ([detekt](https://detekt.dev), [ktlint](https://github.com/pinterest/ktlint)) | [Spotless](https://github.com/diffplug/spotless), [ktlint-gradle](https://github.com/JLLeitschuh/ktlint-gradle), [Kotlinter](https://github.com/jeremymailen/kotlinter-gradle)[^2] | – |
| [Lua](https://www.lua.org) | Lightweight embeddable scripting language. | [LuaRocks](https://luarocks.org) | [busted](https://lunarmodules.github.io/busted/) | [luacheck](https://github.com/lunarmodules/luacheck) | [StyLua](https://github.com/JohnnyMorganz/StyLua)[^4] | – |
| [PHP](https://www.php.net) | The web's server-side workhorse. | [Composer](https://getcomposer.org) | [PHPUnit](https://phpunit.de) | [PHPStan](https://phpstan.org), [PHP_CodeSniffer](https://github.com/PHPCSStandards/PHP_CodeSniffer), [PHP CS Fixer](https://cs.symfony.com) | [PHP CS Fixer](https://cs.symfony.com), [Laravel Pint](https://laravel.com/docs/pint)[^5] | 8.5.12 |
| [PowerShell](https://learn.microsoft.com/powershell/) | Cross-platform shell and scripting language built on .NET. | PowerShell modules | [Pester](https://pester.dev) | [PSScriptAnalyzer](https://github.com/PowerShell/PSScriptAnalyzer) | – | – |
| [Python](https://www.python.org) | Readable general-purpose language with a vast ecosystem. | [uv](https://docs.astral.sh/uv/) (pyproject.toml, requirements.txt), [setuptools](https://setuptools.pypa.io) | [pytest](https://docs.pytest.org) | [Ruff](https://docs.astral.sh/ruff/) | [Ruff](https://docs.astral.sh/ruff/), [Black](https://black.readthedocs.io)[^6] | 3.15.1 |
| [Ruby](https://www.ruby-lang.org) | Expressive, object-oriented language behind Rails. | [Bundler](https://bundler.io), [Rake](https://ruby.github.io/rake/), [RubyGems](https://rubygems.org) | [RSpec](https://rspec.info), [Minitest](https://github.com/minitest/minitest) | [RuboCop](https://rubocop.org), [Standard](https://github.com/standardrb/standard) | [RuboCop](https://rubocop.org), [Standard](https://github.com/standardrb/standard)[^7] | 4.0.9 |
| [Rust](https://www.rust-lang.org) | Memory safety without garbage collection, and fearless concurrency. | [Cargo](https://doc.rust-lang.org/cargo/) | [cargo test](https://doc.rust-lang.org/cargo/commands/cargo-test.html) | [Clippy](https://doc.rust-lang.org/clippy/), [rustfmt](https://github.com/rust-lang/rustfmt) | [rustfmt](https://github.com/rust-lang/rustfmt) | 1.99.1 |
| [Swift](https://www.swift.org) | Apple's safe, fast language, also on Linux. | [Swift Package Manager](https://www.swift.org/documentation/package-manager/), [Xcode](https://developer.apple.com/xcode/) (xcodebuild build and clean) | [XCTest](https://developer.apple.com/documentation/xctest), [Swift Testing](https://developer.apple.com/xcode/swift-testing/) | – | [swift-format](https://github.com/swiftlang/swift-format) | – |
| [TypeScript](https://www.typescriptlang.org) | JavaScript with static types. | [npm](https://docs.npmjs.com/cli), [pnpm](https://pnpm.io), [Yarn](https://yarnpkg.com) or [Bun](https://bun.sh) ([tsc](https://www.typescriptlang.org/docs/handbook/compiler-options.html)) | [npm](https://docs.npmjs.com/cli) test | [npm](https://docs.npmjs.com/cli) run lint, [tsc](https://www.typescriptlang.org/docs/handbook/compiler-options.html) --noEmit | [Prettier](https://prettier.io), [Biome](https://biomejs.dev)[^3] | 24.22.1 (LTS) |

## Additional languages

| Language | What it is | Build systems | Tests | Linting | Formatting | Default version |
| --- | --- | --- | --- | --- | --- | --- |
| [Ada](https://ada-lang.io) | Safety-critical systems language with strong typing and contracts (SPARK). | [Alire](https://alire.ada.dev), [GPRbuild](https://github.com/AdaCore/gprbuild) | [Alire](https://alire.ada.dev), [GPRbuild](https://github.com/AdaCore/gprbuild) test projects | – | – | – |
| [Clojure](https://clojure.org) | A Lisp on the JVM built around immutable data. | [Clojure CLI](https://clojure.org/reference/clojure_cli) (deps.edn), [Leiningen](https://leiningen.org) | [clojure.test](https://clojure.github.io/clojure/clojure.test-api.html) | [clj-kondo](https://github.com/clj-kondo/clj-kondo) | [cljfmt](https://github.com/weavejester/cljfmt)[^8] | – |
| [COBOL](https://gnucobol.sourceforge.io) | The business language of record, compiled with GnuCOBOL. | [CMake](https://cmake.org), [make](https://www.gnu.org/software/make/) | [CTest](https://cmake.org/cmake/help/latest/manual/ctest.1.html), [make](https://www.gnu.org/software/make/) check | – | – | – |
| [Common Lisp](https://common-lisp.net) | The ANSI Lisp, interactive and extensible, on SBCL. | [ASDF](https://asdf.common-lisp.dev) | [ASDF](https://asdf.common-lisp.dev) test-op | – | – | – |
| [Crystal](https://crystal-lang.org) | Ruby-like syntax, statically typed and compiled to native code. | [Shards](https://github.com/crystal-lang/shards) | [crystal spec](https://crystal-lang.org/reference/latest/guides/testing.html) | [crystal tool format](https://crystal-lang.org/reference/latest/man/crystal/index.html) | [crystal tool format](https://crystal-lang.org/reference/latest/man/crystal/index.html) | – |
| [D](https://dlang.org) | C-family systems language with garbage collection and metaprogramming. | [dub](https://dub.pm) | unittest blocks ([dub](https://dub.pm) test) | [D-Scanner](https://github.com/dlang-community/D-Scanner) | – | – |
| [Elixir](https://elixir-lang.org) | Functional, concurrent language on the Erlang VM. | [Mix](https://hexdocs.pm/mix/) | [ExUnit](https://hexdocs.pm/ex_unit/) | [mix format](https://hexdocs.pm/mix/Mix.Tasks.Format.html), [Credo](https://github.com/rrrene/credo) | [mix format](https://hexdocs.pm/mix/Mix.Tasks.Format.html) | 1.20.4 |
| [Elm](https://elm-lang.org)[^9] | Pure functional language for web front ends, with no runtime exceptions. | elm | [elm-test](https://github.com/elm-explorations/test) | [elm-review](https://package.elm-lang.org/packages/jfmengels/elm-review/latest/) | [elm-format](https://github.com/avh4/elm-format) | – |
| [Erlang](https://www.erlang.org) | Fault-tolerant concurrency on the BEAM, born at Ericsson. | [rebar3](https://rebar3.org) | [EUnit](https://www.erlang.org/doc/apps/eunit/), [Common Test](https://www.erlang.org/doc/apps/common_test/) | [Dialyzer](https://www.erlang.org/doc/apps/dialyzer/) | [erlfmt](https://github.com/WhatsApp/erlfmt)[^10] | 29.1.2 |
| [F#](https://fsharp.org) | Functional-first language for .NET. | [.NET SDK](https://dotnet.microsoft.com) (dotnet) | [dotnet test](https://learn.microsoft.com/dotnet/core/tools/dotnet-test) | [Fantomas](https://fsprojects.github.io/fantomas/) | [Fantomas](https://fsprojects.github.io/fantomas/)[^11] | 10.0.403 (LTS) |
| [Fortran](https://fortran-lang.org) | The original high-performance numerical language, still current. | [fpm](https://fpm.fortran-lang.org), [CMake](https://cmake.org), [Meson](https://mesonbuild.com), [make](https://www.gnu.org/software/make/) | [fpm](https://fpm.fortran-lang.org) test, [CTest](https://cmake.org/cmake/help/latest/manual/ctest.1.html) | – | – | – |
| [Gleam](https://gleam.run) | Friendly, type-safe language for the Erlang VM and JavaScript. | gleam | [gleeunit](https://github.com/lpil/gleeunit) ([gleam test](https://gleam.run/command-line-reference/)) | [gleam format](https://gleam.run/command-line-reference/) | [gleam format](https://gleam.run/command-line-reference/) | 1.14.0 |
| [Goose](https://aardappel.github.io/goose/)[^9] | Memory-safe systems language with no heap allocator, garbage collector or lifetime annotations. | goose | test programs | goose check | – | – |
| [Groovy](https://groovy-lang.org) | Dynamic, Java-compatible language for the JVM. | [Gradle](https://gradle.org), [Maven](https://maven.apache.org), [groovyc](https://groovy-lang.org/groovyc.html) | [JUnit](https://junit.org), [Spock](https://spockframework.org) | [Gradle](https://gradle.org) check | [Spotless](https://github.com/diffplug/spotless)[^12] | – |
| [Haskell](https://www.haskell.org) | Lazy, purely functional language with a powerful type system. | [Stack](https://docs.haskellstack.org), [cabal](https://www.haskell.org/cabal/) | test suites | [HLint](https://github.com/ndmitchell/hlint), [cabal](https://www.haskell.org/cabal/) check | [Fourmolu](https://fourmolu.github.io), [Ormolu](https://github.com/tweag/ormolu)[^13] | – |
| [Haxe](https://haxe.org) | One language that compiles to many targets: JS, C++, JVM, Python... | [haxelib](https://lib.haxe.org) (.hxml) | [utest](https://github.com/haxe-utest/utest) | – | – | – |
| [Janet](https://janet-lang.org) | Small, embeddable Lisp-like scripting language. | [jpm](https://github.com/janet-lang/jpm) | [jpm](https://github.com/janet-lang/jpm) test | – | – | – |
| [Julia](https://julialang.org) | Fast dynamic language for numerical and scientific computing. | [Pkg](https://pkgdocs.julialang.org) | Test (Pkg.test) | – | [JuliaFormatter](https://github.com/domluna/JuliaFormatter.jl)[^14] | – |
| [Mojo](https://www.modular.com/mojo)[^15] | Python-like syntax with systems-level performance, for AI. | mojo package | test programs | mojo format | mojo format | – |
| [Nim](https://nim-lang.org) | Python-like syntax compiled to C, with powerful macros. | [Nimble](https://github.com/nim-lang/nimble) | nimble test | – | – | – |
| [Objective-C](https://developer.apple.com/documentation/objectivec) | Smalltalk-style messaging on C; Apple's language before Swift. | [Xcode](https://developer.apple.com/xcode/), [GNUstep make](https://github.com/gnustep/tools-make), [CMake](https://cmake.org), [make](https://www.gnu.org/software/make/) | [XCTest](https://developer.apple.com/documentation/xctest), test programs | – | [clang-format](https://clang.llvm.org/docs/ClangFormat.html)[^1] | – |
| [OCaml](https://ocaml.org) | Fast functional language with type inference and modules. | [dune](https://dune.build) | [dune](https://dune.build) runtest | [ocamlformat](https://github.com/ocaml-ppx/ocamlformat), [dune](https://dune.build) @check | [ocamlformat](https://github.com/ocaml-ppx/ocamlformat)[^16] | – |
| [Octave](https://octave.org) | Free MATLAB-compatible numerical language. | [Octave packages](https://gnu-octave.github.io/packages/) | %!test blocks | – | – | – |
| [Odin](https://odin-lang.org) | Data-oriented systems language, a C alternative. | odin | @(test) procedures | odin vet | – | – |
| [Pascal](https://www.freepascal.org) | Structured, strongly typed language, with Free Pascal. | [Free Pascal](https://www.freepascal.org), [make](https://www.gnu.org/software/make/) | test programs, [make](https://www.gnu.org/software/make/) check | – | – | – |
| [Perl](https://www.perl.org) | Text-processing veteran with CPAN, the first great package archive. | [ExtUtils::MakeMaker](https://metacpan.org/pod/ExtUtils::MakeMaker), [Module::Build](https://metacpan.org/pod/Module::Build), [Carton](https://metacpan.org/pod/Carton) | [prove](https://metacpan.org/pod/prove) ([Test::More](https://perldoc.perl.org/Test::More)) | [Perl::Critic](https://metacpan.org/pod/Perl::Critic) | [perltidy](https://metacpan.org/pod/Perl::Tidy)[^17] | – |
| [Pharo](https://pharo.org) | Live, image-based Smalltalk environment. | [Metacello](https://github.com/Metacello/metacello) (Tonel) | SUnit | – | – | – |
| [Prolog](https://www.swi-prolog.org) | Logic programming, with SWI-Prolog. | [SWI-Prolog packs](https://www.swi-prolog.org/pack/list) | [plunit](https://www.swi-prolog.org/pldoc/package/plunit.html) | – | – | – |
| [R](https://www.r-project.org) | The statistics and data analysis language. | [R packages](https://r-pkgs.org) (DESCRIPTION) | [testthat](https://testthat.r-lib.org) | [lintr](https://lintr.r-lib.org) | [Air](https://posit-dev.github.io/air/)[^18] | – |
| [Racket](https://racket-lang.org) | A Lisp for making languages. | [raco](https://docs.racket-lang.org/raco/) | [rackunit](https://docs.racket-lang.org/rackunit/) ([raco](https://docs.racket-lang.org/raco/) test) | – | – | – |
| [Raku](https://raku.org) | Perl's expressive sister language, formerly Perl 6. | [zef](https://github.com/ugexe/zef) | Test (prove6, [zef](https://github.com/ugexe/zef) test) | – | – | – |
| [Scala](https://www.scala-lang.org) | Functional and object-oriented language for the JVM. | [sbt](https://www.scala-sbt.org), [Gradle](https://gradle.org), [Maven](https://maven.apache.org), scalac | [MUnit](https://scalameta.org/munit/), [ScalaTest](https://www.scalatest.org) | – | [sbt-scalafmt](https://scalameta.org/scalafmt/), [Spotless](https://github.com/diffplug/spotless)[^2] | – |
| [Tcl](https://www.tcl-lang.org) | Embeddable command language, with Tk. | TEA ([make](https://www.gnu.org/software/make/)) | [tcltest](https://wiki.tcl-lang.org/page/tcltest) | – | – | – |
| [V](https://vlang.io) | Simple, fast-compiling language in the spirit of Go. | v | [v test](https://docs.vlang.io/tools.html) | [v fmt](https://docs.vlang.io/tools.html), [v vet](https://docs.vlang.io/tools.html) | [v fmt](https://docs.vlang.io/tools.html) | – |
| [VB.NET](https://learn.microsoft.com/dotnet/visual-basic/) | Visual Basic for .NET. | [.NET SDK](https://dotnet.microsoft.com) (dotnet) | [dotnet test](https://learn.microsoft.com/dotnet/core/tools/dotnet-test) | [dotnet format](https://learn.microsoft.com/dotnet/core/tools/dotnet-format) | [dotnet format](https://learn.microsoft.com/dotnet/core/tools/dotnet-format) | 10.0.403 (LTS) |
| [Wren](https://wren.io) | Small, fast, class-based scripting language for embedding. | [wren_cli](https://github.com/wren-lang/wren-cli) | test scripts | – | – | – |
| [Zig](https://ziglang.org) | Simple systems language and C toolchain, with comptime. | [zig build](https://ziglang.org/learn/build-system/) | [zig build](https://ziglang.org/learn/build-system/) test | zig fmt | zig fmt | 0.17.0 |

## API docs and benchmarks

What `hct docs` and `hct bench` run by default ([commands](commands.md#run-commands)).
Most run only where the project has the tool already: a dependency, a
configuration file (`docs/conf.py`, a `Doxyfile`), a build step or script
named for it. A language not listed, or a component without the tool, has
nothing to run and is skipped.

| Language | API docs | Benchmarks |
| --- | --- | --- |
| C | [Doxygen](https://www.doxygen.nl) | [Google Benchmark](https://github.com/google/benchmark), [Meson](https://mesonbuild.com) benchmarks, [make](https://www.gnu.org/software/make/) bench |
| C# | [DocFX](https://dotnet.github.io/docfx/) | [BenchmarkDotNet](https://benchmarkdotnet.org) |
| C++ | [Doxygen](https://www.doxygen.nl) | [Google Benchmark](https://github.com/google/benchmark), [Meson](https://mesonbuild.com) benchmarks, [make](https://www.gnu.org/software/make/) bench |
| Clojure | [Codox](https://github.com/weavejester/codox) | – |
| Crystal | [crystal docs](https://crystal-lang.org/reference/latest/syntax_and_semantics/documenting_code.html) | – |
| D | [dub](https://dub.pm) build -b docs | – |
| Dart | [dart doc](https://dart.dev/tools/dart-doc) | – |
| Elixir | [ExDoc](https://hexdocs.pm/ex_doc/) | [Benchee](https://github.com/bencheeorg/benchee) |
| Erlang | [EDoc](https://www.erlang.org/doc/apps/edoc/) | – |
| F# | [fsdocs](https://fsprojects.github.io/FSharp.Formatting/) | [BenchmarkDotNet](https://benchmarkdotnet.org) |
| Gleam | [gleam docs](https://gleam.run/command-line-reference/) | – |
| Go | – | [go test](https://pkg.go.dev/cmd/go#hdr-Test_packages) -bench |
| Groovy | [Groovydoc](https://groovy-lang.org/groovydoc.html), [Javadoc](https://docs.oracle.com/en/java/javase/21/docs/specs/man/javadoc.html) | [JMH](https://github.com/openjdk/jmh) |
| Haskell | [Haddock](https://haskell-haddock.readthedocs.io) | [cabal](https://www.haskell.org/cabal/) bench, [Stack](https://docs.haskellstack.org) bench |
| Java | [Javadoc](https://docs.oracle.com/en/java/javase/21/docs/specs/man/javadoc.html) | [JMH](https://github.com/openjdk/jmh) |
| JavaScript | [npm](https://docs.npmjs.com/cli) run docs, [TypeDoc](https://typedoc.org) | [npm](https://docs.npmjs.com/cli) run bench |
| Julia | [Documenter](https://documenter.juliadocs.org) | [BenchmarkTools](https://juliaci.github.io/BenchmarkTools.jl/) |
| Kotlin | [Dokka](https://kotlinlang.org/docs/dokka-introduction.html) | [JMH](https://github.com/openjdk/jmh), [kotlinx-benchmark](https://github.com/Kotlin/kotlinx-benchmark) |
| Nim | [nim doc](https://nim-lang.org/docs/docgen.html) | – |
| Objective-C | [Doxygen](https://www.doxygen.nl) | – |
| OCaml | [odoc](https://ocaml.github.io/odoc/) | [dune](https://dune.build) build @bench |
| PHP | – | [PHPBench](https://phpbench.readthedocs.io) |
| Python | [Sphinx](https://www.sphinx-doc.org), [pdoc](https://pdoc.dev) | [pytest-benchmark](https://pytest-benchmark.readthedocs.io) |
| Ruby | [YARD](https://yardoc.org), [RDoc](https://ruby.github.io/rdoc/) | [Rake](https://ruby.github.io/rake/) bench task |
| Rust | [cargo doc](https://doc.rust-lang.org/cargo/commands/cargo-doc.html) | [cargo bench](https://doc.rust-lang.org/cargo/commands/cargo-bench.html) |
| Scala | [Scaladoc](https://docs.scala-lang.org/scala3/guides/scaladoc/) | [JMH](https://github.com/openjdk/jmh) |
| Swift | [DocC](https://www.swift.org/documentation/docc/) | [package-benchmark](https://github.com/ordo-one/package-benchmark) |
| TypeScript | [npm](https://docs.npmjs.com/cli) run docs, [TypeDoc](https://typedoc.org) | [npm](https://docs.npmjs.com/cli) run bench |
| VB.NET | – | [BenchmarkDotNet](https://benchmarkdotnet.org) |
| Zig | [zig build](https://ziglang.org/learn/build-system/) docs | [zig build](https://ziglang.org/learn/build-system/) bench |

[^1]: Only when the project has a `.clang-format` file: C-family code has no single standard style.
[^2]: Only when the build applies one; hecatron runs it through the build.
[^3]: Only when the project depends on one, at the version it locks.
[^4]: Only when the project has a `stylua.toml`.
[^5]: Only when the project installs one with Composer.
[^6]: Black only where the project configures it; otherwise Ruff.
[^7]: Only when one is in the project's Gemfile.
[^8]: Only when the project has a `cljfmt.edn`.
[^9]: Not on Linux arm64: no toolchain upstream.
[^10]: As a rebar3 plugin, when the project's rebar.config has it.
[^11]: Only when Fantomas is in the project's .NET tool manifest.
[^12]: Only when the build applies it; hecatron runs it through the build.
[^13]: Only when the project configures one.
[^14]: Only when the project has a `.JuliaFormatter.toml`.
[^15]: Not on Intel Macs: no toolchain upstream.
[^16]: Only when the project has a `.ocamlformat`.
[^17]: Only when the project has a `.perltidyrc`.
[^18]: Only when the project has an `air.toml`.
