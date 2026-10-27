# Shared helpers for the default format hooks (languages/<lang>/hooks/format.sh;
# sourced after _hooks.sh). A hook rewrites the component's files with the
# language's formatter, using the project's own configuration; with
# HECATRON_FORMAT_CHECK=1 (hecatron format --check, and hecatron lint) it
# changes nothing and fails, listing the files the formatter would change
# (docs/format.md).
# shellcheck shell=bash
# shellcheck disable=SC2154 # build_dir is set by find_build in _hooks.sh

set -euo pipefail

# Formatter versions hecatron runs when the project pins none (a lock file
# or .pre-commit-config.yaml), so two machines format alike. Each is fetched
# on first use (uvx, the Clojure CLI) rather than installed.
_RUFF_VERSION="0.17.3"
_BLACK_VERSION="26.10.1"
_CLANG_FORMAT_VERSION="23.1.3"
_CLJFMT_VERSION="0.16.6"

# Fail, listing the files the formatter would change (given, or one per line
# on stdin).
format_unformatted() {
    local _files=() _file
    if [ "$#" -gt 0 ]; then _files=("$@"); else while IFS= read -r _file; do [ -z "${_file}" ] || _files+=("${_file}"); done; fi
    [ "${#_files[@]}" -gt 0 ] || return 0
    echo "error: ${HECATRON_LANG:-}: not formatted (hecatron format ${HECATRON_LANG:-} formats them):" >&2
    printf '  %s\n' "${_files[@]#./}" >&2
    exit 1
}

# Run a formatter that lists the files it changes (or, checking, would),
# one per line, and exits 0 either way: checking, any listed fails.
format_listing() {
    local _listed
    echo "+ $*"
    _listed="$("$@")"
    if format_checking; then format_unformatted <<<"${_listed}"; elif [ -n "${_listed}" ]; then echo "${_listed}"; fi
}

# The files below this directory with one of the extensions (c, h...), one
# per line, outside version control, dependencies, build output and vendored
# code: what a formatter given every file would wrongly touch.
format_files() {
    local _names=() _ext
    for _ext in "$@"; do _names+=(-o -name "*.${_ext}"); done
    find . \( -type d ! -path . \( -name '.*' -o -name node_modules -o -name vendor -o -name third_party \
        -o -name third-party -o -name external -o -name build -o -name target -o -name out -o -name dist \
        -o -name obj -o -name _build -o -name dist-newstyle -o -name .stack-work \) -prune \) \
        -o -type f \( "${_names[@]:1}" \) -print 2>/dev/null | sort | format_not_excluded
}

# The paths read from stdin (./relative to this directory, one per line)
# that the project's "format": {"exclude": [...]} (HECATRON_FORMAT_EXCLUDE,
# relative to the project root; a trailing "/" is everything below) doesn't
# name, such as a fixture misformatted on purpose.
format_not_excluded() {
    local _root _here _path _relative _pattern _excluded
    if [ -z "${HECATRON_FORMAT_EXCLUDE:-}" ] || [ -z "${HECATRON_ROOT:-}" ]; then
        cat
        return
    fi
    _root="$(cd "${HECATRON_ROOT}" && pwd -P)"
    _here="$(pwd -P)"
    while IFS= read -r _path; do
        _relative="${_here}/${_path#./}"
        _relative="${_relative#"${_root}"/}"
        _excluded=0
        while IFS= read -r _pattern; do
            [ -n "${_pattern}" ] && format_path_matches "${_relative}" "${_pattern}" && _excluded=1
        done <<<"${HECATRON_FORMAT_EXCLUDE}"
        [ "${_excluded}" -eq 1 ] || printf '%s\n' "${_path}"
    done
}

# Whether a root-relative path is the pattern's: under it when it ends in
# "/", else matching it as a glob (*.pb.go, gen/*.go).
# shellcheck disable=SC2053 # the pattern is a glob on purpose
format_path_matches() {
    if [[ "$2" == */ ]]; then
        [ "${1#"$2"}" != "$1" ]
    else
        [[ "$1" == $2 ]]
    fi
}
format_files_into() {
    local _file
    found_files=()
    while IFS= read -r _file; do found_files+=("${_file}"); done < <(format_files "$@")
}

# The version a project's .pre-commit-config.yaml pins for a hook
# repository (the rev after a repo: line matching the pattern), without a
# leading v.
precommit_rev() {
    local _dir
    _dir="$(nearest_with .pre-commit-config.yaml)" || return 1
    awk -v pattern="$1" '
        /repo:/ { inside = ($0 ~ pattern) }
        inside && /rev:/ { sub(/.*rev:[[:space:]]*/, ""); gsub(/["'\''[:space:]]/, ""); sub(/^v/, ""); print; exit }
    ' "${_dir}/.pre-commit-config.yaml" | grep .
}

# The version of a package a uv.lock here or above locks.
uv_locked() {
    local _dir
    _dir="$(nearest_with uv.lock)" || return 1
    awk -v name="$1" '
        $0 == "name = \"" name "\"" { found = 1; next }
        found && /^version = / { gsub(/version = |"/, ""); print; exit }
        /^\[\[package\]\]/ { found = 0 }
    ' "${_dir}/uv.lock" | grep .
}

# uvx: on PATH, or where uv's installer puts it (uv_install in _common.sh),
# which isn't on PATH for languages other than Python.
uvx_path() {
    if has uvx; then command -v uvx
    elif [ -x "${HOME}/.local/bin/uvx" ]; then echo "${HOME}/.local/bin/uvx"
    else return 1; fi
}

# Go: gofmt over the module's packages here (not vendor/ or testdata/, nor
# directories the go command ignores).
format_go() {
    local _files=() _file
    while IFS= read -r _file; do _files+=("${_file}"); done < <(
        find . \( -type d ! -path . \( -name vendor -o -name testdata -o -name '.*' -o -name '_*' \) -prune \) \
            -o -type f -name '*.go' -print | sort | format_not_excluded)
    [ "${#_files[@]}" -gt 0 ] || skip "no .go files"
    if format_checking; then format_listing gofmt -l "${_files[@]}"; else format_listing gofmt -l -w "${_files[@]}"; fi
}

# Rust: rustfmt through Cargo, for the package or workspace (rustfmt.toml).
format_rust() {
    find_build --above-first Cargo.toml || unknown "no Cargo.toml here, below, or above it in the project"
    if format_checking; then run cargo fmt --all -- --check; else run cargo fmt --all; fi
}

# Python: Ruff's formatter (its configuration in pyproject.toml or
# ruff.toml), or Black for a project that configures Black and not Ruff; at
# the version the project locks (uv.lock) or pins for pre-commit, else
# hecatron's.
format_python() {
    has uvx || { echo "error: python: uv isn't installed (hecatron install python installs it)" >&2; exit 1; }
    local _version
    if [ -f pyproject.toml ] && grep -q '^\[tool\.black\]' pyproject.toml && ! grep -q '^\[tool\.ruff' pyproject.toml &&
        [ ! -f ruff.toml ] && [ ! -f .ruff.toml ]; then
        _version="$(uv_locked black || precommit_rev 'psf/black' || echo "${_BLACK_VERSION}")"
        if format_checking; then run uvx "black@${_version}" --check .; else run uvx "black@${_version}" .; fi
        return
    fi
    _version="$(uv_locked ruff || precommit_rev 'ruff-pre-commit' || echo "${_RUFF_VERSION}")"
    if format_checking; then run uvx "ruff@${_version}" format --check .; else run uvx "ruff@${_version}" format .; fi
}

# Node.js (TypeScript, JavaScript): the Prettier or Biome the package (or
# its workspace) depends on, at the version it locks. Neither is a default:
# a project that doesn't use one would have every file rewritten.
format_node() {
    [ -f package.json ] || unknown "no package.json"
    local _tool
    if node_depends_on prettier; then _tool=prettier
    elif node_depends_on @biomejs/biome; then _tool=biome
    else skip "the package depends on neither Prettier nor Biome"; fi
    # shellcheck source=languages/typescript/package-manager.sh
    source "$(dirname "${BASH_SOURCE[0]}")/typescript/package-manager.sh"
    node_pm_command
    npm_deps
    local _bin
    _bin="$(nearest_with "node_modules/.bin/${_tool}")/node_modules/.bin/${_tool}" ||
        { echo "error: ${HECATRON_LANG:-}: ${_tool} isn't in node_modules after installing the dependencies" >&2; exit 1; }
    case "${_tool}" in
        prettier) if format_checking; then run "${_bin}" --check .; else run "${_bin}" --write --list-different .; fi ;;
        biome) if format_checking; then run "${_bin}" format .; else run "${_bin}" format --write .; fi ;;
    esac
}
# Whether package.json, here or in the workspace above, lists a package.
node_depends_on() {
    local _above
    _above="$(dir_above_with package.json || true)"
    python3 -c '
import json, sys
for path in sys.argv[2:]:
    try:
        data = json.load(open(path))
    except (OSError, ValueError):
        continue
    for key in ("dependencies", "devDependencies", "optionalDependencies"):
        if sys.argv[1] in (data.get(key) or {}):
            sys.exit(0)
sys.exit(1)
' "$1" package.json ${_above:+"${_above}/package.json"}
}

# .NET (C#, VB.NET): dotnet format, with the project's .editorconfig.
format_dotnet() {
    find_build '*.sln' '*.slnx' '*.csproj' '*.vbproj' ||
        unknown "no .sln, .slnx or project file here, below, or above it in the project"
    note_build_dir
    cd "${build_dir}"
    local _target
    _target="$(dotnet_target)"
    if format_checking; then
        run dotnet format ${_target:+"${_target}"} --verify-no-changes
    else
        run dotnet format ${_target:+"${_target}"}
    fi
}

# Ruby: RuboCop's layout cops, or Standard, whichever the bundle has. Neither
# is a default: their style is the project's choice.
format_ruby() {
    ruby_project
    if gem_listed rubocop; then
        bundle_deps
        if format_checking; then ruby_run rubocop --only Layout; else ruby_run rubocop --fix-layout; fi
    elif gem_listed standard; then
        bundle_deps
        if format_checking; then ruby_run standardrb; else ruby_run standardrb --fix; fi
    else
        skip "neither RuboCop nor Standard is in the bundle"
    fi
}

# PHP: PHP CS Fixer where the project configures it, else Laravel Pint where
# it installs it; each from the project's vendor/bin.
format_php() {
    [ -f composer.json ] || unknown "no composer.json"
    composer_deps
    if [ -x vendor/bin/php-cs-fixer ] && exists '.php-cs-fixer.php .php-cs-fixer.dist.php'; then
        if format_checking; then run vendor/bin/php-cs-fixer fix --dry-run --diff; else run vendor/bin/php-cs-fixer fix; fi
    elif [ -x vendor/bin/pint ]; then
        if format_checking; then run vendor/bin/pint --test; else run vendor/bin/pint; fi
    else
        skip "no configured PHP CS Fixer and no Laravel Pint in vendor/bin"
    fi
}

# Elixir: mix format, with .formatter.exs; without one, on the files a new
# Mix project's .formatter.exs lists. Its plugins and import_deps need the
# dependencies.
format_elixir() {
    [ -f mix.exs ] || unknown "no mix.exs"
    mix_deps
    local _inputs=()
    [ -f .formatter.exs ] || _inputs=("{mix,.formatter}.exs" "{config,lib,test}/**/*.{ex,exs}")
    if format_checking; then
        run mix format --check-formatted ${_inputs[@]+"${_inputs[@]}"}
    else
        run mix format ${_inputs[@]+"${_inputs[@]}"}
    fi
}

# Dart: dart format (also for Flutter packages), with the page width in
# analysis_options.yaml.
format_dart() {
    [ -f pubspec.yaml ] || unknown "no pubspec.yaml"
    if format_checking; then run dart format --output=none --set-exit-if-changed .; else run dart format .; fi
}

# C, C++, Objective-C: clang-format, where the project has a .clang-format
# (here or above): C has no one style, and clang-format's default would
# rewrite every file. At the version the project pins for pre-commit, else
# hecatron's, through uvx (the LLVM release's PyPI build); else the
# clang-format on PATH.
format_clang() {
    nearest_with .clang-format _clang-format >/dev/null || skip "no .clang-format here or above"
    format_files_into "$@"
    [ "${#found_files[@]}" -gt 0 ] || skip "no .$(echo "$@" | sed 's/ /, ./g') files"
    local _clang_format=(clang-format) _version _uvx
    if _uvx="$(uvx_path)"; then
        _version="$(precommit_rev 'mirrors-clang-format' || echo "${_CLANG_FORMAT_VERSION}")"
        _clang_format=("${_uvx}" --from "clang-format==${_version}" clang-format)
    elif ! has clang-format; then
        echo "error: ${HECATRON_LANG:-}: neither uv nor clang-format is installed (hecatron install ${HECATRON_LANG:-})" >&2
        exit 1
    fi
    if format_checking; then
        run "${_clang_format[@]}" --dry-run -Werror "${found_files[@]}"
    else
        run "${_clang_format[@]}" -i "${found_files[@]}"
    fi
}

# Swift: swift-format (part of the toolchain), with the package's
# .swift-format. It has no check mode of its own (its lint adds rules beyond
# layout), so a check compares each file with its formatted output.
format_swift() {
    swift_package
    local _file _unformatted=()
    format_files_into swift
    [ "${#found_files[@]}" -gt 0 ] || skip "no .swift files"
    if ! format_checking; then
        run swift format --in-place --parallel "${found_files[@]}"
        return
    fi
    echo "+ swift format (checking ${#found_files[@]} files)"
    for _file in "${found_files[@]}"; do
        swift format "${_file}" | cmp -s "${_file}" - || _unformatted+=("${_file}")
    done
    format_unformatted ${_unformatted[@]+"${_unformatted[@]}"}
}

# Clojure: cljfmt, where the project configures it (cljfmt.edn), at a
# pinned version the Clojure CLI fetches.
format_clojure() {
    nearest_with cljfmt.edn .cljfmt.edn cljfmt.clj .cljfmt.clj >/dev/null || skip "no cljfmt configuration (cljfmt.edn)"
    local _dirs=() _dir
    for _dir in src test; do
        if [ -d "${_dir}" ]; then _dirs+=("${_dir}"); fi
    done
    [ "${#_dirs[@]}" -gt 0 ] || skip "no src or test directory"
    local _cljfmt=(clojure -Sdeps "{:deps {dev.weavejester/cljfmt {:mvn/version \"${_CLJFMT_VERSION}\"}}}" -M -m cljfmt.main)
    if format_checking; then run "${_cljfmt[@]}" check "${_dirs[@]}"; else run "${_cljfmt[@]}" fix "${_dirs[@]}"; fi
}

# Erlang: erlfmt, where rebar.config has its plugin (rebar3 fmt).
format_erlang() {
    [ -f rebar.config ] || unknown "no rebar.config"
    grep -q erlfmt rebar.config || skip "no erlfmt plugin in rebar.config"
    if format_checking; then run rebar3 fmt --check; else run rebar3 fmt --write; fi
}

# Haskell: Fourmolu (fourmolu.yaml) or Ormolu (.ormolu), whichever the
# project configures.
format_haskell() {
    local _tool
    if nearest_with fourmolu.yaml >/dev/null; then _tool=fourmolu
    elif nearest_with .ormolu >/dev/null; then _tool=ormolu
    else skip "no fourmolu.yaml or .ormolu"; fi
    has "${_tool}" || { echo "error: haskell: ${_tool} isn't installed (hecatron install haskell installs it)" >&2; exit 1; }
    format_files_into hs
    [ "${#found_files[@]}" -gt 0 ] || skip "no .hs files"
    if format_checking; then run "${_tool}" --mode check "${found_files[@]}"; else run "${_tool}" --mode inplace "${found_files[@]}"; fi
}

# Lua: StyLua, where the project configures it (stylua.toml).
format_lua() {
    nearest_with stylua.toml .stylua.toml >/dev/null || skip "no stylua.toml"
    has stylua || { echo "error: lua: StyLua isn't installed (hecatron install lua installs it)" >&2; exit 1; }
    if format_checking; then run stylua --check .; else run stylua .; fi
}

# Perl: perltidy, where the project configures it (.perltidyrc).
format_perl() {
    [ -f .perltidyrc ] || skip "no .perltidyrc"
    format_files_into pl pm t
    [ "${#found_files[@]}" -gt 0 ] || skip "no .pl, .pm or .t files"
    if ! format_checking; then
        run perltidy --profile=.perltidyrc -b -bext=/ "${found_files[@]}"
        return
    fi
    local _file _unformatted=()
    echo "+ perltidy --profile=.perltidyrc (checking ${#found_files[@]} files)"
    for _file in "${found_files[@]}"; do
        perltidy --profile=.perltidyrc -st -se "${_file}" 2>/dev/null | cmp -s "${_file}" - || _unformatted+=("${_file}")
    done
    format_unformatted ${_unformatted[@]+"${_unformatted[@]}"}
}

# R: Air, where the project configures it (air.toml).
format_r() {
    nearest_with air.toml .air.toml >/dev/null || skip "no air.toml"
    has air || { echo "error: r: Air isn't installed (hecatron install r installs it)" >&2; exit 1; }
    if format_checking; then run air format --check .; else run air format .; fi
}

# Julia: JuliaFormatter, where the project configures it
# (.JuliaFormatter.toml), from the shared environment the installer puts it
# in.
format_julia() {
    nearest_with .JuliaFormatter.toml >/dev/null || skip "no .JuliaFormatter.toml"
    local _overwrite=true
    if format_checking; then _overwrite=false; fi
    run julia --project=@hecatron-format -e "using JuliaFormatter
exit(format(\".\"; overwrite=${_overwrite}, verbose=true) ? 0 : 1)"
}

# Elm: elm-format, which nearly all Elm code follows, over the source
# directories elm.json names and tests/.
format_elm() {
    [ -f elm.json ] || unknown "no elm.json"
    has elm-format || { echo "error: elm: elm-format isn't installed (hecatron install elm installs it)" >&2; exit 1; }
    local _dirs=() _dir
    while IFS= read -r _dir; do
        if [ -d "${_dir}" ]; then _dirs+=("${_dir}"); fi
    done < <(python3 -c '
import json
data = json.load(open("elm.json"))
for d in data.get("source-directories") or ["src"]:
    print(d)
print("tests")
')
    [ "${#_dirs[@]}" -gt 0 ] || skip "no source directories"
    if format_checking; then run elm-format --validate "${_dirs[@]}"; else run elm-format --yes "${_dirs[@]}"; fi
}

# OCaml: dune's formatting rules, where the project configures ocamlformat
# (.ocamlformat): ocamlformat refuses to run without one.
format_ocaml() {
    [ -f .ocamlformat ] || skip "no .ocamlformat"
    if format_checking; then
        run dune build @fmt
    else
        # Promoting the formatted files exits non-zero for having had diffs;
        # a second build says whether anything is left.
        run dune build @fmt --auto-promote >/dev/null 2>&1 || true
        run dune build @fmt
    fi
}
