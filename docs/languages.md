# Languages

Every language hecatron supports, what it is, and how hecatron builds,
tests and lints it by default. A project overrides any default with its own
hook (`.hecatron/languages/<lang>/<action>.sh`).

Generated from each language's `config.json` by
`libexec/codegen/languages_doc.py`; don't edit by hand.
[coverage.md](coverage.md) has which platforms each one installs and passes
on, and [adding-a-language.md](adding-a-language.md) covers adding one.

| Language | What it is | Build systems | Tests | Linting | Default version |
| --- | --- | --- | --- | --- | --- |
| [Ada](https://ada-lang.io) | Safety-critical systems language with strong typing and contracts (SPARK). | Alire, GPRbuild | Alire, GPRbuild test projects | – | – |
| [Bash](https://www.gnu.org/software/bash/) | The Unix shell and scripting language. | – | Bats | ShellCheck | – |
| [C](https://www.open-std.org/jtc1/sc22/wg14/) | The portable systems language most others build on. | CMake, Meson, Autotools, make | CTest, Meson test, make check | – | – |
| [C#](https://learn.microsoft.com/dotnet/csharp/) | Microsoft's general-purpose language for .NET. | .NET SDK (dotnet) | dotnet test | dotnet format | – |
| [C++](https://isocpp.org) | Zero-overhead systems language with templates and RAII. | CMake, Meson, Autotools, make | CTest, Meson test, make check | – | – |
| [Clojure](https://clojure.org) | A Lisp on the JVM built around immutable data. | Clojure CLI (deps.edn), Leiningen | clojure.test | clj-kondo | – |
| [COBOL](https://gnucobol.sourceforge.io) | The business language of record, compiled with GnuCOBOL. | CMake, make | CTest, make check | – | – |
| [Common Lisp](https://common-lisp.net) | The ANSI Lisp, interactive and extensible, on SBCL. | ASDF | ASDF test-op | – | – |
| [Crystal](https://crystal-lang.org) | Ruby-like syntax, statically typed and compiled to native code. | Shards | crystal spec | crystal tool format | – |
| [D](https://dlang.org) | C-family systems language with garbage collection and metaprogramming. | dub | unittest blocks (dub test) | D-Scanner | – |
| [Dart](https://dart.dev) | Client-optimized language behind Flutter. | pub (dart, flutter) | package:test | dart analyze | – |
| [Elixir](https://elixir-lang.org) | Functional, concurrent language on the Erlang VM. | Mix | ExUnit | mix format, Credo | – |
| [Elm](https://elm-lang.org) | Pure functional language for web front ends, with no runtime exceptions. *Not on Linux arm64: no toolchain upstream.* | elm | elm-test | elm-review | – |
| [Erlang](https://www.erlang.org) | Fault-tolerant concurrency on the BEAM, born at Ericsson. | rebar3 | EUnit, Common Test | Dialyzer | – |
| [F#](https://fsharp.org) | Functional-first language for .NET. | .NET SDK (dotnet) | dotnet test | Fantomas | – |
| [Fortran](https://fortran-lang.org) | The original high-performance numerical language, still current. | fpm, CMake, Meson, make | fpm test, CTest | – | – |
| [Gleam](https://gleam.run) | Friendly, type-safe language for the Erlang VM and JavaScript. | gleam | gleeunit (gleam test) | gleam format | – |
| [Go](https://go.dev) | Simple, fast-compiling language with built-in concurrency. | go modules | go test | golangci-lint | 1.27.3 |
| [Goose](https://aardappel.github.io/goose/) | Memory-safe systems language with no heap allocator, garbage collector or lifetime annotations. *Not on Linux arm64: no toolchain upstream.* | goose | test programs | goose check | – |
| [Groovy](https://groovy-lang.org) | Dynamic, Java-compatible language for the JVM. | Gradle, Maven, groovyc | JUnit, Spock | Gradle check | – |
| [Haskell](https://www.haskell.org) | Lazy, purely functional language with a powerful type system. | Stack, cabal | test suites | HLint, cabal check | – |
| [Haxe](https://haxe.org) | One language that compiles to many targets: JS, C++, JVM, Python... | haxelib (.hxml) | utest | – | – |
| [Janet](https://janet-lang.org) | Small, embeddable Lisp-like scripting language. | jpm | jpm test | – | – |
| [Java](https://dev.java) | The JVM's flagship language. | Maven, Gradle | JUnit | Gradle check | 25.0.5 (LTS) |
| [JavaScript](https://developer.mozilla.org/docs/Web/JavaScript) | The web's language, on Node.js. | npm | npm test (node:test, Jest...) | npm run lint (ESLint...) | – |
| [Julia](https://julialang.org) | Fast dynamic language for numerical and scientific computing. | Pkg | Test (Pkg.test) | – | – |
| [Kotlin](https://kotlinlang.org) | Concise, null-safe language for the JVM, Android and beyond. | Gradle, Maven, kotlinc | JUnit, kotlin.test | Gradle check (detekt, ktlint) | – |
| [Lua](https://www.lua.org) | Lightweight embeddable scripting language. | LuaRocks | busted | luacheck | – |
| [Mojo](https://www.modular.com/mojo) | Python-like syntax with systems-level performance, for AI. *Not on Intel Macs: no toolchain upstream.* | mojo package | test programs | mojo format | – |
| [Nim](https://nim-lang.org) | Python-like syntax compiled to C, with powerful macros. | Nimble | nimble test | – | – |
| [Objective-C](https://developer.apple.com/documentation/objectivec) | Smalltalk-style messaging on C; Apple's language before Swift. | Xcode, GNUstep make, CMake, make | XCTest, test programs | – | – |
| [OCaml](https://ocaml.org) | Fast functional language with type inference and modules. | dune | dune runtest | ocamlformat, dune @check | – |
| [Octave](https://octave.org) | Free MATLAB-compatible numerical language. | Octave packages | %!test blocks | – | – |
| [Odin](https://odin-lang.org) | Data-oriented systems language, a C alternative. | odin | @(test) procedures | odin vet | – |
| [Pascal](https://www.freepascal.org) | Structured, strongly typed language, with Free Pascal. | Free Pascal, make | test programs, make check | – | – |
| [Perl](https://www.perl.org) | Text-processing veteran with CPAN, the first great package archive. | ExtUtils::MakeMaker, Module::Build, Carton | prove (Test::More) | Perl::Critic | – |
| [Pharo](https://pharo.org) | Live, image-based Smalltalk environment. | Metacello (Tonel) | SUnit | – | – |
| [PHP](https://www.php.net) | The web's server-side workhorse. | Composer | PHPUnit | PHPStan, PHP_CodeSniffer, PHP CS Fixer | – |
| [PowerShell](https://learn.microsoft.com/powershell/) | Cross-platform shell and scripting language built on .NET. | PowerShell modules | Pester | PSScriptAnalyzer | – |
| [Prolog](https://www.swi-prolog.org) | Logic programming, with SWI-Prolog. | SWI-Prolog packs | plunit | – | – |
| [Python](https://www.python.org) | Readable general-purpose language with a vast ecosystem. | uv (pyproject.toml), setuptools | pytest | Ruff | 3.15.1 |
| [R](https://www.r-project.org) | The statistics and data analysis language. | R packages (DESCRIPTION) | testthat | lintr | – |
| [Racket](https://racket-lang.org) | A Lisp for making languages. | raco | rackunit (raco test) | – | – |
| [Raku](https://raku.org) | Perl's expressive sister language, formerly Perl 6. | zef | Test (prove6, zef test) | – | – |
| [Ruby](https://www.ruby-lang.org) | Expressive, object-oriented language behind Rails. | Bundler, Rake, RubyGems | RSpec, Minitest | RuboCop, Standard | – |
| [Rust](https://www.rust-lang.org) | Memory safety without garbage collection, and fearless concurrency. | Cargo | cargo test | Clippy, rustfmt | 1.99.1 |
| [Scala](https://www.scala-lang.org) | Functional and object-oriented language for the JVM. | sbt, Gradle, Maven, scalac | MUnit, ScalaTest | – | – |
| [Swift](https://www.swift.org) | Apple's safe, fast language, also on Linux. | Swift Package Manager | XCTest, Swift Testing | – | – |
| [Tcl](https://www.tcl-lang.org) | Embeddable command language, with Tk. | TEA (make) | tcltest | – | – |
| [TypeScript](https://www.typescriptlang.org) | JavaScript with static types. | npm (tsc) | npm test | npm run lint, tsc --noEmit | 24.22.1 (LTS) |
| [V](https://vlang.io) | Simple, fast-compiling language in the spirit of Go. | v | v test | v fmt, v vet | – |
| [VB.NET](https://learn.microsoft.com/dotnet/visual-basic/) | Visual Basic for .NET. | .NET SDK (dotnet) | dotnet test | dotnet format | – |
| [Wren](https://wren.io) | Small, fast, class-based scripting language for embedding. | wren_cli | test scripts | – | – |
| [Zig](https://ziglang.org) | Simple systems language and C toolchain, with comptime. | zig build | zig build test | zig fmt | – |
