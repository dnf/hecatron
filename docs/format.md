# Formatting

`hct format` rewrites every component's code with its language's
formatter, using the project's own configuration (`rustfmt.toml`,
`.editorconfig`, `pyproject.toml`, `.prettierrc`, `.clang-format`...).

```
hct format                # every language, in parallel
hct format go python      # these languages, every component
hct format --check        # change nothing; fail, listing what would change
```

## The check in `hct lint`

`hct lint` runs the format check after each language's linter, so CI fails
on unformatted code. Where the linter checks formatting itself (Clippy's
lint runs `cargo fmt --check`; C#'s is `dotnet format --verify-no-changes`;
RuboCop includes its layout cops) it isn't run twice.

A project that isn't formatted yet can turn the check off, for every
language or some, in `.hecatron/config.json`:

```json
"lint": {"format": false}
"lint": {"format": {"haskell": false, "cpp": false}}
```

`hct check` fails on any other value. Languages whose linter already
checked formatting before `hct format` existed (the ones listed above, and
Crystal, Gleam, V, Zig, Mojo, F#, Swift, and OCaml and Elixir where they
configure their formatter) keep checking it.

## What each language uses

A formatter that ships with the toolchain, or that nearly all of a
language's code follows, runs by default. Elsewhere there is no one style,
and a default would rewrite every file of a project that doesn't use it,
so hecatron runs the formatter the project shows it uses, and skips
otherwise.

| Language | Formatter | Runs |
| --- | --- | --- |
| Go | `gofmt` | always |
| Rust | `cargo fmt` (rustfmt) | always |
| Python | [Ruff](https://docs.astral.sh/ruff/formatter/), or Black where `pyproject.toml` configures Black and not Ruff | always |
| C#, VB.NET | `dotnet format` | always |
| Dart | `dart format` | always |
| Elixir | `mix format` (a new Mix project's inputs without `.formatter.exs`) | always |
| Zig, Gleam, Crystal, V, Mojo | `zig fmt`, `gleam format`, `crystal tool format`, `v fmt`, `mojo format` | always |
| Swift | swift-format (part of the toolchain) | always |
| Elm | [elm-format](https://github.com/avh4/elm-format) | always |
| TypeScript, JavaScript | the package's own Prettier, else Biome | when a dependency |
| Ruby | RuboCop's layout cops (`--fix-layout`), else Standard | when in the bundle |
| PHP | PHP CS Fixer, else Laravel Pint, from `vendor/bin` | when configured or installed |
| Java, Kotlin, Scala, Groovy | the build's plugin: Spotless or fmt-maven-plugin (Maven); Spotless, ktlint-gradle or Kotlinter (Gradle); sbt-scalafmt | when the build has one |
| F# | Fantomas | when the dotnet tool manifest lists it |
| C, C++, Objective-C | clang-format | with a `.clang-format` |
| OCaml | ocamlformat, through dune | with a `.ocamlformat` |
| Haskell | Fourmolu, or Ormolu | with a `fourmolu.yaml`, or `.ormolu` |
| Lua | [StyLua](https://github.com/JohnnyMorganz/StyLua) | with a `stylua.toml` |
| R | [Air](https://posit-dev.github.io/air/) | with an `air.toml` |
| Perl | perltidy | with a `.perltidyrc` |
| Clojure | cljfmt | with a `cljfmt.edn` |
| Erlang | erlfmt (`rebar3 fmt`) | with its plugin in `rebar.config` |
| Julia | JuliaFormatter | with a `.JuliaFormatter.toml` |

Other languages have no formatter hecatron runs: `hct format <lang>` says
so and succeeds. A project adds or replaces one with its own
`.hecatron/languages/<lang>/format.sh`, which formats when run and only
checks when `HECATRON_FORMAT_CHECK=1`.

## Versions

Formatters change their output between releases, so hecatron runs one
version: the project's where it pins one, else hecatron's pinned default.
Prettier, Biome, RuboCop, PHP CS Fixer and the JVM plugins come from the
project's own dependencies. Ruff and Black run at the version `uv.lock`
locks or `.pre-commit-config.yaml` pins, else hecatron's, through `uvx`;
clang-format likewise, from LLVM's PyPI build. `hct install <lang>`
installs the others at pinned versions where a component uses them
(elm-format, StyLua, Air, Fourmolu, Ormolu, perltidy, JuliaFormatter), and
`hct doctor` reports any missing.
