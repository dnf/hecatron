# Shared helpers for the default docs hooks (languages/<lang>/hooks/docs.sh;
# sourced after _hooks.sh). Each builds the component's API documentation
# with its ecosystem's standard tool, one that comes with the toolchain or
# that the project already depends on (hecatron installs none), into the
# tool's default directory, and says where (docs_written). A component with
# nothing to build docs from, or without the tool among its dependencies,
# skips (docs/guide/commands.md).
# shellcheck shell=bash
# shellcheck disable=SC2154 # build_dir is set by find_build in _hooks.sh

set -euo pipefail

# Rust: rustdoc through Cargo, for the workspace's own crates, into the
# target directory's doc/.
docs_rust() {
    find_build --above-first Cargo.toml || unknown "no Cargo.toml here, below, or above it in the project"
    run cargo doc --no-deps
    docs_written "$(cargo_target_dir)/doc"
}
cargo_target_dir() {
    cargo metadata --format-version 1 --no-deps 2>/dev/null |
        python3 -c 'import json, sys; print(json.load(sys.stdin)["target_directory"])' 2>/dev/null || echo target
}

# Python: Sphinx where the project has a Sphinx configuration (docs/conf.py,
# or docs/source/conf.py beside docs/build), into its _build/html (or
# build/html); else pdoc where it's a dependency, into build/pdoc (pdoc has
# no default: it serves the docs instead). Both run in the project's
# environment through uv, as the test hook does.
docs_python() {
    has_any pyproject.toml setup.py setup.cfg || run_below_with pyproject.toml setup.py setup.cfg
    local _conf="" _candidate _source _out
    for _candidate in docs/conf.py docs/source/conf.py doc/conf.py doc/source/conf.py; do
        if [ -f "${_candidate}" ]; then _conf="${_candidate}"; break; fi
    done
    if [ -n "${_conf}" ]; then
        _source="$(dirname "${_conf}")"
        case "${_source}" in
            */source) _out="$(dirname "${_source}")/build" ;;
            *) _out="${_source}/_build" ;;
        esac
        python_run sphinx sphinx-build -M html "${_source}" "${_out}"
        docs_written "${_out}/html"
    elif python_lists pdoc; then
        local _modules=()
        while IFS= read -r _candidate; do _modules+=("${_candidate}"); done < <(python_modules)
        [ "${#_modules[@]}" -gt 0 ] || skip "no package or module for pdoc to document"
        python_run pdoc pdoc -o build/pdoc "${_modules[@]}"
        docs_written build/pdoc
    else
        skip "no docs/conf.py for Sphinx, and pdoc isn't among the dependencies"
    fi
}
# A command in the project's environment (every extra and group) with a
# tool added, and the docs' own requirements (docs/requirements.txt, as Read
# the Docs reads them).
python_run() {
    local _tool="$1" _args=() _candidate
    shift
    has uv || { echo "error: python: uv isn't installed (hecatron install python installs it)" >&2; exit 1; }
    if has_any pyproject.toml setup.py setup.cfg; then _args+=(--all-extras --all-groups); fi
    _args+=(--with "${_tool}")
    for _candidate in docs/requirements.txt doc/requirements.txt; do
        if [ -f "${_candidate}" ]; then _args+=(--with-requirements "${_candidate}"); fi
    done
    run uv run "${_args[@]}" "$@"
}
# Whether the project's dependencies name a package, e.g. `python_lists pdoc`
# (a regular expression, matched as a whole name).
python_lists() {
    grep -qsiE "(^|[^[:alnum:]_.-])$1([^[:alnum:]_.-]|$)" pyproject.toml setup.cfg setup.py requirements*.txt \
        docs/requirements.txt uv.lock
}
# The packages and modules to document: those in src/, else at the top,
# leaving out tests, docs and setup scripts.
python_modules() {
    local _base=. _entry
    if [ -d src ]; then _base=src; fi
    for _entry in "${_base}"/*; do
        case "$(basename "${_entry}")" in
            test | tests | docs | doc | setup.py | conftest.py | noxfile.py | _* | .*) continue ;;
        esac
        if [ -f "${_entry}/__init__.py" ] || { [ -f "${_entry}" ] && [ "${_entry%.py}" != "${_entry}" ]; }; then
            echo "${_entry#./}"
        fi
    done
}

# .NET: the project's own local tools (.config/dotnet-tools.json, here or
# above): DocFX for C# (docfx.json, here or in docs/, into its _site), the
# F# Formatting tool for F# (fsdocs, into output/).
docs_dotnet() {
    local _manifest_dir _config=""
    _manifest_dir="$(nearest_with .config/dotnet-tools.json)" || skip "no .config/dotnet-tools.json with a docs tool"
    case "${HECATRON_LANG:-}" in
        fsharp)
            grep -qs '"fsdocs-tool"' "${_manifest_dir}/.config/dotnet-tools.json" ||
                skip "fsdocs-tool isn't in .config/dotnet-tools.json"
            cd "${_manifest_dir}"
            run dotnet tool restore
            run dotnet fsdocs build
            docs_written output ;;
        *)
            grep -qs '"docfx"' "${_manifest_dir}/.config/dotnet-tools.json" ||
                skip "DocFX isn't in .config/dotnet-tools.json"
            local _dir
            for _dir in . docs "${_manifest_dir}" "${_manifest_dir}/docs"; do
                if [ -f "${_dir}/docfx.json" ]; then _config="${_dir}/docfx.json"; break; fi
            done
            [ -n "${_config}" ] || skip "no docfx.json here, in docs/, or beside .config/dotnet-tools.json"
            run dotnet tool restore
            run dotnet docfx "${_config}"
            docs_written "$(dirname "${_config}")/_site" ;;
    esac
}

# Elixir: ExDoc (mix docs), when it's a dependency, into doc/.
docs_elixir() {
    [ -f mix.exs ] || unknown "no mix.exs"
    grep -qs ':ex_doc' mix.exs || skip "ExDoc isn't among mix.exs's dependencies"
    mix_deps
    run mix docs
    docs_written doc
}

# Erlang: EDoc through rebar3, into doc/.
docs_erlang() {
    [ -f rebar.config ] || unknown "no rebar.config"
    run rebar3 edoc
    docs_written doc
}

# Gleam: gleam docs build, into build/dev/docs/<name>.
docs_gleam() {
    [ -f gleam.toml ] || unknown "no gleam.toml"
    run gleam docs build
    docs_written "build/dev/docs/$(sed -n 's/^name[[:space:]]*=[[:space:]]*"\(.*\)".*/\1/p' gleam.toml | head -1)"
}

# Swift: DocC through swift-docc-plugin, when the package has it, into
# SwiftPM's plugin outputs.
docs_swift() {
    swift_package
    grep -qs 'swift-docc-plugin' "${build_dir}"/Package*.swift ||
        skip "the package doesn't depend on swift-docc-plugin"
    run swift package ${swift_package_args[@]+"${swift_package_args[@]}"} generate-documentation
    docs_written "${build_dir}/.build/plugins/Swift-DocC/outputs"
}

# Haskell: Haddock through Stack or cabal, which print each package's
# directory.
docs_haskell() {
    haskell_project
    if is_stack; then
        run stack haddock
        docs_written .stack-work "each package's, as Stack says above"
    else
        run cabal haddock all
        docs_written dist-newstyle "each package's, as cabal says above"
    fi
}

# OCaml: odoc through dune (@doc), into _build/default/_doc/_html. odoc is
# an opam package of its own, which hecatron doesn't install.
docs_ocaml() {
    has odoc || skip "odoc isn't installed (opam install odoc)"
    run dune build @doc
    docs_written _build/default/_doc/_html
}

# Dart: dart doc, into doc/api.
docs_dart() {
    [ -f pubspec.yaml ] || unknown "no pubspec.yaml"
    dart_deps
    run dart doc .
    docs_written doc/api
}

# Julia: Documenter's docs/make.jl, in the docs environment with the package
# developed into it (Documenter's own instructions), into docs/build.
docs_julia() {
    [ -f Project.toml ] || unknown "no Project.toml"
    [ -f docs/make.jl ] || skip "no docs/make.jl"
    run julia --project=docs -e 'using Pkg; Pkg.develop(PackageSpec(path=pwd())); Pkg.instantiate()'
    run julia --project=docs docs/make.jl
    docs_written docs/build
}

# Ruby: YARD when the bundle has it, else RDoc (a default gem), into doc/.
docs_ruby() {
    ruby_project
    bundle_deps
    if gem_listed yard; then
        ruby_run yard doc
    elif [ -d lib ] && has rdoc; then
        run rdoc lib
    else
        skip "no YARD in the bundle, and no lib/ for RDoc"
    fi
    docs_written doc
}

# Zig: the build's docs step, when it defines one (usually installing into
# zig-out/docs).
docs_zig() {
    [ -f build.zig ] || unknown "no build.zig"
    grep -qsE '\.step\([[:space:]]*"docs"' build.zig || skip "build.zig defines no docs step"
    run zig build docs
    docs_written zig-out "where the build installs them, usually zig-out/docs"
}

# Crystal: crystal docs, into docs/.
docs_crystal() {
    [ -f shard.yml ] || unknown "no shard.yml"
    shards_deps
    run crystal docs
    docs_written docs
}

# Nim: nim doc for the package's main module and those it imports, into
# htmldocs/.
docs_nim() {
    exists '*.nimble' || unknown "no .nimble file"
    local _name _module="" _candidate
    _name="$(basename ./*.nimble .nimble)"
    for _candidate in "src/${_name}.nim" "${_name}.nim"; do
        if [ -f "${_candidate}" ]; then _module="${_candidate}"; break; fi
    done
    [ -n "${_module}" ] || skip "no src/${_name}.nim or ${_name}.nim to document"
    run nim doc --project --index:on --outdir:htmldocs "${_module}"
    docs_written htmldocs
}

# D: dub's docs build type, into docs/.
docs_d() {
    { [ -f dub.json ] || [ -f dub.sdl ]; } || unknown "no dub.json or dub.sdl"
    run dub build -b docs
    docs_written docs
}

# C, C++, Objective-C: Doxygen, where the project has a Doxyfile (here or
# above), from its directory, into its OUTPUT_DIRECTORY's HTML_OUTPUT.
docs_doxygen() {
    local _dir _out _html
    _dir="$(nearest_with Doxyfile)" || skip "no Doxyfile here or above it in the project"
    if ! has doxygen; then
        echo "error: ${HECATRON_LANG:-}: the project has a Doxyfile (${_dir}), and Doxygen isn't installed" >&2
        exit 1
    fi
    cd "${_dir}"
    _out="$(doxyfile_value OUTPUT_DIRECTORY)"
    _html="$(doxyfile_value HTML_OUTPUT)"
    run doxygen Doxyfile
    docs_written "${_out:-.}/${_html:-html}"
}
# A Doxyfile setting's value, without quotes; empty when it isn't set.
doxyfile_value() {
    sed -n "s/^[[:space:]]*$1[[:space:]]*=[[:space:]]*//p" Doxyfile | tail -1 | sed 's/^"\(.*\)"[[:space:]]*$/\1/; s/[[:space:]]*$//'
}

# Clojure: Codox, through its Leiningen plugin or a deps.edn :codox alias,
# into target/doc.
docs_clojure() {
    if [ -f project.clj ] && grep -qs 'lein-codox' project.clj; then
        run lein codox
    elif [ -f deps.edn ] && grep -qs ':codox' deps.edn; then
        run clojure -X:codox
    else
        skip "no lein-codox plugin in project.clj, and no :codox alias in deps.edn"
    fi
    docs_written target/doc
}
