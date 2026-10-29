# Shared helpers for the default bench hooks (languages/<lang>/hooks/bench.sh;
# sourced after _hooks.sh). Each runs the component's benchmarks with its
# ecosystem's standard harness, where the component has some: a component
# without benchmarks, or without the harness among its dependencies, skips
# (docs/guide/commands.md). The numbers are the tools' own output.
# shellcheck shell=bash
# shellcheck disable=SC2154 # build_dir is set by find_build in _hooks.sh

set -euo pipefail

# Directories a search for benchmarks leaves out: version control, build
# output, dependencies.
_bench_skipped=(.git target node_modules vendor _build build dist-newstyle .stack-work bin obj)
# Whether a file below this directory (or the one given) matches the
# extended regular expression, among the files named like the glob.
bench_grep() {
    local _pattern="$1" _glob="$2" _dir="${3:-.}" _prune=() _skip _file
    # find, not grep's --include and --exclude-dir: BusyBox's grep (the
    # bash:3.2 image's) has neither.
    for _skip in "${_bench_skipped[@]}"; do _prune+=(-o -name "${_skip}"); done
    while IFS= read -r _file; do
        if grep -qsE -- "${_pattern}" "${_file}"; then return 0; fi
    done < <(find "${_dir}" \( -name .git "${_prune[@]}" \) -prune -o -type f -name "${_glob}" -print 2>/dev/null)
    return 1
}

# Rust: cargo bench, where the package or workspace has benchmarks (benches/,
# or a [[bench]] target).
bench_rust() {
    find_build --above-first Cargo.toml || unknown "no Cargo.toml here, below, or above it in the project"
    local _where=.
    [ "${build_dir}" = . ] || _where="${build_dir}"
    if [ -z "$(find "${_where}" -maxdepth 4 \( -name target -o -name .git \) -prune -o -type d -name benches -print 2>/dev/null | head -1)" ] &&
        ! bench_grep '^\[\[bench\]\]' Cargo.toml "${_where}"; then
        skip "no benches/ directory or [[bench]] target"
    fi
    run cargo bench
}

# Go: the module's Benchmark functions, without its tests.
bench_go() {
    find_build --above-first go.mod || unknown "no go.mod here, below, or above it in the project"
    bench_grep '^func Benchmark' '*_test.go' || skip "no Benchmark functions in the tests"
    run go test -run '^$' -bench . ./...
}

# Python: pytest-benchmark's benchmarks only, where it's a dependency, in the
# project's environment as the test hook runs pytest.
bench_python() {
    has_any pyproject.toml setup.py setup.cfg || run_below_with pyproject.toml setup.py setup.cfg
    has uv || { echo "error: python: uv isn't installed (hecatron install python installs it)" >&2; exit 1; }
    grep -qsiE 'pytest[-_]benchmark' pyproject.toml setup.cfg setup.py requirements*.txt uv.lock ||
        skip "pytest-benchmark isn't among the dependencies"
    if has_any pyproject.toml setup.py setup.cfg; then
        run uv run --all-extras --all-groups --with pytest --with pytest-benchmark pytest --benchmark-only
    else
        run uv run --with pytest --with pytest-benchmark pytest --benchmark-only
    fi
}

# .NET: each project of the language that references BenchmarkDotNet, built
# for release and run with every benchmark selected (BenchmarkSwitcher would
# otherwise ask which).
bench_dotnet() {
    local _extension _projects=() _project _failed=()
    case "${HECATRON_LANG:-}" in
        fsharp) _extension=fsproj ;;
        vbnet) _extension=vbproj ;;
        *) _extension=csproj ;;
    esac
    while IFS= read -r -d '' _project; do
        if grep -qs 'Include="BenchmarkDotNet"' "${_project}"; then _projects+=("${_project}"); fi
    done < <(find . \( -name bin -o -name obj -o -name .git -o -name node_modules \) -prune -o \
        -type f -name "*.${_extension}" -print0 2>/dev/null | sort -z)
    [ "${#_projects[@]}" -gt 0 ] || skip "no project references BenchmarkDotNet"
    export DOTNET_ROLL_FORWARD="${DOTNET_ROLL_FORWARD:-Major}"
    for _project in "${_projects[@]}"; do
        run dotnet run -c Release --project "${_project}" -- --filter '*' || _failed+=("${_project#./}")
    done
    if [ "${#_failed[@]}" -gt 0 ]; then
        echo "error: ${HECATRON_LANG:-}: bench failed in ${_failed[*]}" >&2
        exit 1
    fi
}

# Haskell: every benchmark suite, through Stack or cabal.
bench_haskell() {
    haskell_project
    bench_grep '^benchmark[[:space:]]' '*.cabal' || bench_grep '^benchmarks:' package.yaml ||
        skip "no benchmark suite in the .cabal or package.yaml files"
    if is_stack; then run stack bench; else run cabal bench all; fi
}

# Elixir: Benchee scripts in bench/, where Benchee is a dependency, each
# through mix run.
bench_elixir() {
    [ -f mix.exs ] || unknown "no mix.exs"
    grep -qs ':benchee' mix.exs || skip "Benchee isn't among mix.exs's dependencies"
    exists 'bench/*.exs' || skip "no bench/*.exs scripts"
    mix_deps
    local _script
    for _script in bench/*.exs; do
        run mix run "${_script}"
    done
}

# Swift: package-benchmark's plugin, when the package depends on it.
bench_swift() {
    swift_package
    grep -qs 'package-benchmark' "${build_dir}"/Package*.swift ||
        skip "the package doesn't depend on package-benchmark"
    run swift package ${swift_package_args[@]+"${swift_package_args[@]}"} benchmark
}

# Zig: the build's bench (or benchmark) step, when it defines one.
bench_zig() {
    [ -f build.zig ] || unknown "no build.zig"
    local _step
    for _step in bench benchmark; do
        if grep -qsE "\\.step\\([[:space:]]*\"${_step}\"" build.zig; then
            run zig build "${_step}"
            return
        fi
    done
    skip "build.zig defines no bench or benchmark step"
}

# Julia: BenchmarkTools' SUITE in benchmark/benchmarks.jl (PkgBenchmark's
# layout), in the benchmark environment with the package developed into it.
bench_julia() {
    [ -f Project.toml ] || unknown "no Project.toml"
    [ -f benchmark/benchmarks.jl ] || skip "no benchmark/benchmarks.jl"
    [ -f benchmark/Project.toml ] || skip "no benchmark/Project.toml for the benchmarks' environment"
    run julia --project=benchmark -e 'using Pkg; Pkg.develop(PackageSpec(path=pwd())); Pkg.instantiate()'
    run julia --project=benchmark -e 'using BenchmarkTools; include("benchmark/benchmarks.jl"); display(run(SUITE; verbose=true))'
}

# OCaml: dune's bench alias, where a dune file defines it.
bench_ocaml() {
    bench_grep '\(alias[[:space:]]+bench\)|\(name[[:space:]]+bench\)' dune ||
        skip "no dune file defines a bench alias"
    run dune build @bench
}

# Ruby: the Rakefile's bench or benchmark task.
bench_ruby() {
    ruby_project
    [ -f "${ruby_root}/Rakefile" ] || skip "no Rakefile"
    bundle_deps
    local _task
    for _task in bench benchmark; do
        if rake_has "${_task}"; then
            ruby_run rake "${_task}"
            return
        fi
    done
    skip "the Rakefile has no bench or benchmark task"
}

# PHP: PHPBench, where composer.json requires it.
bench_php() {
    [ -f composer.json ] || unknown "no composer.json"
    grep -qs '"phpbench/phpbench"' composer.json || skip "PHPBench isn't among composer.json's dependencies"
    composer_deps
    run vendor/bin/phpbench run --report=default
}
