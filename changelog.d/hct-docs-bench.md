- **`hct docs` and `hct bench`:** two new run commands. `hct docs` builds
  each component's API documentation with its ecosystem's tool into the
  tool's own directory (`cargo doc --no-deps`, Sphinx with a
  `docs/conf.py` or pdoc, Javadoc, Dokka, Scaladoc, DocFX, ExDoc, EDoc,
  DocC, Haddock, odoc, `dart doc`, Documenter, TypeDoc, YARD or RDoc,
  Doxygen with a `Doxyfile`...) and ends saying where (also in agent and
  JSON output). `hct bench` runs benchmarks (`cargo bench`, `go test -run
  '^$' -bench . ./...`, pytest-benchmark, JMH, BenchmarkDotNet,
  `cabal bench`, package-benchmark, a CMake bench target or Google
  Benchmark programs...). Both run only tools a component already has;
  a language or component with none says so and passes. A project's
  `docs.sh` or `bench.sh` replaces the default
  ([languages](docs/guide/languages.md#api-docs-and-benchmarks)) (#227, #228).
