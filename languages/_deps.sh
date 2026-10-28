# Shared helpers for the default deps hooks (languages/<lang>/hooks/deps.sh;
# sourced after _hooks.sh): each fetches or restores a component's
# dependencies with its ecosystem's own tool, held to the lock file when
# there is one. Ecosystems whose test and build hooks also install
# dependencies when they're missing (Node.js, Ruby, PHP, Elixir, Dart,
# Crystal, Haxe) share their *_fetch function with those hooks, in
# _hooks.sh; the rest are here, and their tools fetch implicitly when they
# build (go, cargo, dotnet, uv run, SwiftPM...).
# shellcheck shell=bash
# shellcheck disable=SC2154 # build_dir is set by find_build in _hooks.sh

set -euo pipefail

# Node.js (TypeScript, JavaScript): the package manager the package declares.
deps_node() {
    [ -f package.json ] || skip "no package.json"
    # shellcheck source=languages/typescript/package-manager.sh
    source "$(dirname "${BASH_SOURCE[0]}")/typescript/package-manager.sh"
    node_pm_command
    node_fetch
}

# Python: uv sync (--locked with a uv.lock, which then must match
# pyproject.toml), every extra and dependency group, as the test hook runs;
# a requirements-only directory gets a .venv with them. A .venv moved from
# another checkout, whose scripts can't start, is made again.
deps_python() {
    has uv || { echo "error: python: uv isn't installed (hecatron install python installs it)" >&2; exit 1; }
    if [ -f .venv/bin/pytest ] && ! head -1 .venv/bin/pytest | grep -qF "${PWD}/.venv/"; then
        echo "python: .venv was moved from another checkout: making it again"
        rm -rf .venv
    fi
    if [ -f pyproject.toml ]; then
        if nearest_with uv.lock >/dev/null; then run uv sync --locked --all-extras --all-groups; else run uv sync --all-extras --all-groups; fi
    elif exists 'requirements*.txt'; then
        local _args=() _file
        for _file in requirements*.txt; do _args+=(-r "${_file}"); done
        run uv venv --allow-existing
        run uv pip install "${_args[@]}"
    else
        skip "no pyproject.toml or requirements file"
    fi
}

# Ruby: Bundler, frozen to Gemfile.lock.
deps_ruby() {
    ruby_project
    [ -f "${ruby_root}/Gemfile" ] || skip "no Gemfile"
    bundle_fetch
}

# PHP: composer install, from composer.lock: the composer.json here, else
# the nearest above (a root composer.json covering lang/php), else each below.
deps_php() {
    find_build --above-first composer.json || skip "no composer.json"
    note_build_dir
    cd "${build_dir}"
    composer_fetch
}

# Elixir: mix deps.get, from mix.lock.
deps_elixir() {
    [ -f mix.exs ] || skip "no mix.exs"
    mix_fetch
}

# Dart: pub get (flutter pub get for a Flutter package), held to
# pubspec.lock.
deps_dart() {
    [ -f pubspec.yaml ] || skip "no pubspec.yaml"
    dart_fetch
}

# Go: the module's dependencies into the module cache, as go.sum records.
deps_go() {
    find_build --above-first go.mod || skip "no go.mod"
    run go mod download
}

# Rust: cargo fetch, --locked to Cargo.lock when the package or workspace
# has one.
deps_rust() {
    find_build --above-first Cargo.toml || skip "no Cargo.toml"
    if nearest_with Cargo.lock >/dev/null; then run cargo fetch --locked; else run cargo fetch; fi
}

# .NET (C#, F#, VB.NET): dotnet restore, --locked-mode where the projects
# have lock files (packages.lock.json).
deps_dotnet() {
    find_build '*.sln' '*.slnx' '*.csproj' '*.fsproj' '*.vbproj' || skip "no .sln, .slnx or project file"
    note_build_dir
    cd "${build_dir}"
    local _target _locked=()
    _target="$(dotnet_target)"
    if [ -n "$(find . -maxdepth 4 -name packages.lock.json -print -quit 2>/dev/null)" ]; then _locked=(--locked-mode); fi
    run dotnet restore ${_target:+"${_target}"} ${_locked[@]+"${_locked[@]}"}
}

# Swift: SwiftPM's resolve, held to Package.resolved when there is one.
deps_swift() {
    swift_package
    local _resolved=()
    if [ -f "${build_dir}/Package.resolved" ]; then _resolved=(--force-resolved-versions); fi
    run swift package ${swift_package_args[@]+"${swift_package_args[@]}"} ${_resolved[@]+"${_resolved[@]}"} resolve
}

# Crystal: shards install, --frozen to shard.lock.
deps_crystal() {
    [ -f shard.yml ] || skip "no shard.yml"
    shards_fetch
}

# Haskell: the dependencies built, through Stack or cabal (whose package
# index is updated first); a cabal.project.freeze holds cabal to its versions.
deps_haskell() {
    if is_stack; then
        run stack build --only-dependencies --test --no-run-tests
    elif is_cabal; then
        run cabal update
        run cabal build all --only-dependencies --enable-tests
    else
        skip "no stack.yaml, *.cabal or cabal.project"
    fi
}

# Clojure: the CLI's prepare (-P) for the project and its :test alias, or
# Leiningen's deps.
deps_clojure() {
    if [ -f deps.edn ]; then
        run clojure -P
        if grep -q ':test' deps.edn; then run clojure -P -M:test; fi
    elif [ -f project.clj ]; then
        run lein deps
    else
        skip "no deps.edn or project.clj"
    fi
}

# Erlang: rebar3, from rebar.lock.
deps_erlang() {
    [ -f rebar.config ] || skip "no rebar.config"
    run rebar3 get-deps
}

# Gleam: from manifest.toml.
deps_gleam() {
    [ -f gleam.toml ] || skip "no gleam.toml"
    run gleam deps download
}

# Julia: Pkg.instantiate, which installs what Manifest.toml records.
deps_julia() {
    [ -f Project.toml ] || skip "no Project.toml"
    run julia --project=. -e 'using Pkg; Pkg.instantiate()'
}

# Nim: nimble's dependencies only, from nimble.lock when there is one.
deps_nim() {
    exists '*.nimble' || skip "no .nimble file"
    run nimble install --depsOnly -y
}

# Zig: the packages build.zig.zon names, into the package cache.
deps_zig() {
    [ -f build.zig.zon ] || skip "no build.zig.zon"
    run zig build --fetch
}

# OCaml: opam's dependencies of the packages here, test ones included.
# Packages here that require each other at {= version} need one version, and
# opam 2.1 gives a local package without a version field the newest in its
# repository (ocaml-community/yojson's yojson 3.0.0, but yojson-bench 2.0.2,
# so no solution), so several are pinned as <name>.dev while their
# dependencies install, and unpinned after.
deps_ocaml() {
    exists '*.opam' || skip "no .opam file"
    # The system packages the dependencies need (libgmp-dev, pkg-config...),
    # through apt or Homebrew: --yes alone answers no to installing them.
    export OPAMCONFIRMLEVEL=unsafe-yes
    local opams=(*.opam) names=() name status=0
    if [ "${#opams[@]}" -eq 1 ]; then
        run opam install . --deps-only --with-test --yes
        return
    fi
    for name in "${opams[@]}"; do names+=("${name%.opam}"); done
    for name in "${names[@]}"; do
        run opam pin add --no-action --yes --kind=path "${name}.dev" .
    done
    run opam install --deps-only --with-test --yes "${names[@]}" || status=$?
    opam pin remove --no-action --yes "${names[@]}" >/dev/null
    return "${status}"
}

# D: dub's packages, as dub.selections.json selects.
deps_d() {
    { [ -f dub.json ] || [ -f dub.sdl ]; } || skip "no dub.json or dub.sdl"
    run dub upgrade --missing-only
}

# Raku: zef, for a distribution.
deps_raku() {
    [ -f META6.json ] || skip "no META6.json"
    run zef install --deps-only .
}

# Janet: jpm, into the local tree when the project keeps one.
deps_janet() {
    [ -f project.janet ] || skip "no project.janet"
    jpm_ deps
}

# R: renv's restore, from renv.lock; else a package's DESCRIPTION
# dependencies (Depends, Imports, LinkingTo and Suggests, which the tests
# use) that aren't installed, from CRAN into the user's library. Here, else
# each package below.
deps_r() {
    find_build renv.lock DESCRIPTION || skip "no renv.lock or DESCRIPTION"
    note_build_dir
    cd "${build_dir}"
    if [ -f renv.lock ]; then
        run Rscript -e 'renv::restore(prompt = FALSE)'
        return
    fi
    echo "+ Rscript: install DESCRIPTION's missing dependencies from CRAN"
    Rscript -e '
fields <- read.dcf("DESCRIPTION", fields = c("Depends", "Imports", "LinkingTo", "Suggests"))
named <- unlist(strsplit(fields[!is.na(fields)], ","))
named <- trimws(sub("[(].*", "", named))
installed <- rownames(installed.packages())
missing <- setdiff(unique(named[nzchar(named)]), c("R", installed))
if (length(missing) > 0) {
    lib <- Sys.getenv("R_LIBS_USER")
    dir.create(lib, recursive = TRUE, showWarnings = FALSE)
    install.packages(missing, lib = lib, repos = "https://cloud.r-project.org",
                     Ncpus = max(1L, parallel::detectCores()))
    still <- setdiff(missing, rownames(installed.packages(lib.loc = c(lib, .libPaths()))))
    if (length(still) > 0) message("r: deps: not on CRAN or failed to install: ", paste(still, collapse = ", "))
}'
}

# Perl: Carton's deployment install from cpanfile.snapshot, into local/
# (where the test hook looks). A cpanfile's modules without a snapshot go
# where hecatron install puts them, system-wide.
deps_perl() {
    [ -f cpanfile.snapshot ] || skip "no cpanfile.snapshot (Carton); hecatron install perl installs a cpanfile's modules"
    has carton || { echo "error: perl: Carton isn't installed (cpanm Carton)" >&2; exit 1; }
    run carton install --deployment
}

# Haxe: the haxelib libraries each .hxml names.
deps_haxe() {
    local _hxml _found=0
    for _hxml in *.hxml; do
        [ -f "${_hxml}" ] || continue
        haxe_fetch "${_hxml}"
        _found=1
    done
    [ "${_found}" -eq 1 ] || skip "no .hxml files"
}
