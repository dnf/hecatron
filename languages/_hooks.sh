# Shared helpers for the built-in default hooks (languages/<lang>/hooks/*.sh;
# sourced, not executed). A hook runs inside one component's directory for
# its language (each-component.sh), picks the command from the build files it
# finds there, and says so. A project replaces any hook with its own
# .hecatron/languages/<lang>/<action>.sh.
# shellcheck shell=bash

set -euo pipefail

_hook_action="$(basename "$0" .sh)"

has() { command -v "$1" &>/dev/null; }

# Downloads retry transient failures, as in the installers (_common.sh).
curl() {
    command curl --retry 4 --retry-delay 3 --retry-connrefused --connect-timeout 30 \
        --speed-limit 1024 --speed-time 60 "$@"
}

# Whether any file matches a glob in this directory, e.g. `exists '*.csproj'`.
exists() {
    local _match
    for _match in $1; do
        [ -e "${_match}" ] && return 0
    done
    return 1
}

# The files a find expression matches below this directory, sorted, into the
# array found_files, e.g. `find_files -name '*.sh'`. Version control and
# dependency directories aren't searched.
found_files=()
find_files() {
    local _file
    found_files=()
    while IFS= read -r -d '' _file; do
        found_files+=("${_file}")
    done < <(find . \( -name .git -o -name node_modules \) -prune -o -type f \( "$@" \) -print0 | sort -z)
}

# Run a command, showing it first, so a log says which build system ran.
run() {
    echo "+ $*"
    "$@"
}

# The hook has nothing to do here: say why, and succeed. For an action the
# directory's build system doesn't have (no linter configured, nothing to
# build for an interpreted library).
skip() {
    echo "${HECATRON_LANG:-}: ${_hook_action}: $* (skipping)"
    exit 0
}

# The hook can't tell how to act on this directory: fail, saying what it
# looked for and how to override it.
unknown() {
    echo "error: ${HECATRON_LANG:-}: no ${_hook_action} command for $(pwd): $*" >&2
    echo "Write .hecatron/languages/${HECATRON_LANG:-<lang>}/${_hook_action}.sh to say how." >&2
    exit 1
}

# Whether a format hook only checks (hecatron format --check, and hecatron
# lint): it changes nothing and fails, listing what it would change.
format_checking() { [ "${HECATRON_FORMAT_CHECK:-}" = 1 ]; }

# For a lint hook whose linter checks formatting itself (cargo fmt --check,
# dotnet format --verify-no-changes): `hecatron lint` then doesn't run the
# format hook's check as well (each-component.sh). Say so before the check,
# which exits the hook when it fails.
checks_format() {
    if [ -n "${HECATRON_FORMAT_CHECKED:-}" ]; then echo "${HECATRON_LANG:-}" >>"${HECATRON_FORMAT_CHECKED}"; fi
}

# A language's directory isn't always where its build file is. The build can
# be above it (pulumi's sdk/go is in the module sdk/go.mod; FlatBuffers'
# swift/ is built by the root's Package.swift), or in directories below it
# (FlatBuffers' rust/ holds three crates). find_build looks here, then below,
# then above, never leaving the project (HECATRON_ROOT).

# This hook and its arguments, to run it again in another directory.
_hook_script="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"
_hook_args=("$@")

# Whether this directory has a file matching one of the globs.
has_any() {
    local _glob
    for _glob in "$@"; do
        exists "${_glob}" && return 0
    done
    return 1
}

# The directories above this one, nearest first, up to the project root.
dirs_above() {
    local _root _dir
    _root="$(cd "${HECATRON_ROOT:-.}" 2>/dev/null && pwd -P)" || return 0
    _dir="$(pwd -P)"
    case "${_dir}/" in "${_root}/"?*) ;; *) return 0 ;; esac
    while [ "${_dir}" != "${_root}" ]; do
        _dir="$(dirname "${_dir}")"
        echo "${_dir}"
    done
}

# The nearest directory above this one with a file matching one of the globs.
dir_above_with() {
    local _dir
    while IFS= read -r _dir; do
        if (cd "${_dir}" && has_any "$@"); then
            echo "${_dir}"
            return 0
        fi
    done < <(dirs_above)
    return 1
}

# The nearest directory, this one or above it in the project, with one of
# the files: where a formatter that looks upward finds its configuration.
nearest_with() {
    if has_any "$@"; then pwd; else dir_above_with "$@"; fi
}

# The directories below this one, at most three levels down, with a file
# matching one of the globs: only the shallowest that have one (csharp/src/
# with the solution, not each project under it), as ./paths. Vendored code,
# test data and build output aren't searched.
dirs_below_with() {
    local _names=() _skip=() _glob
    for _glob in "$@"; do _names+=(-o -name "${_glob}"); done
    for _glob in '.*' node_modules vendor third_party third-party external \
        testdata test-data test_data fixtures build target out dist bin obj; do
        _skip+=(-o -name "${_glob}")
    done
    find . -maxdepth 4 \( -type d ! -path . \( "${_skip[@]:1}" \) -prune \) -o \
        -type f \( "${_names[@]:1}" \) -print 2>/dev/null |
        sed 's|/[^/]*$||' | sort -u |
        awk -F/ 'NF > 1 { depth[NR] = NF; dir[NR] = $0; if (!least || NF < least) least = NF }
            END { for (i = 1; i <= NR; i++) if (depth[i] == least) print dir[i] }'
}

# Run this hook again in each directory, and exit: failing if any fails.
run_hook_in() {
    local _dir _failed_dirs=()
    for _dir in "$@"; do
        echo "--- ${HECATRON_LANG:-}: ${_hook_action} in ${_dir#./}"
        (cd "${_dir}" && bash "${_hook_script}" ${_hook_args[@]+"${_hook_args[@]}"}) || _failed_dirs+=("${_dir#./}")
    done
    if [ "${#_failed_dirs[@]}" -gt 0 ]; then
        echo "error: ${HECATRON_LANG:-}: ${_hook_action} failed in ${_failed_dirs[*]}" >&2
        exit 1
    fi
    exit 0
}

# Where the build files matching the globs are: here (build_dir is .), or the
# nearest directory above (build_dir is its path); when only directories
# below have them, the hook runs in each and exits. Fails when none has.
# --above-first looks above before below, for a tool that itself finds its
# build file upward (go, cargo, bundler, gradle): one above means this
# directory belongs to that build, and the tool acts on it from here.
build_dir=""
find_build() {
    local _above_first=0 _dirs=() _dir
    if [ "$1" = --above-first ]; then
        _above_first=1
        shift
    fi
    build_dir=.
    has_any "$@" && return 0
    if [ "${_above_first}" -eq 1 ] && build_dir="$(dir_above_with "$@")"; then return 0; fi
    while IFS= read -r _dir; do _dirs+=("${_dir}"); done < <(dirs_below_with "$@")
    [ "${#_dirs[@]}" -eq 0 ] || run_hook_in "${_dirs[@]}"
    if [ "${_above_first}" -eq 0 ] && build_dir="$(dir_above_with "$@")"; then return 0; fi
    build_dir=""
    return 1
}
# Say where the build is, when it isn't here.
note_build_dir() {
    [ "${build_dir}" = . ] || echo "${HECATRON_LANG:-}: ${_hook_action}: the build for $(pwd) is in ${build_dir}"
}

# A Maven or Gradle wrapper when the project ships one, else the tool. A
# Gradle subproject's wrapper is usually in the build's root, above it.
maven() { if [ -x ./mvnw ]; then run ./mvnw -B "$@"; else run mvn -B "$@"; fi; }
gradle_() {
    local _wrapper=./gradlew
    [ -x "${_wrapper}" ] || _wrapper="$(dir_above_with gradlew || true)/gradlew"
    if [ "${_wrapper}" != /gradlew ] && [ -x "${_wrapper}" ]; then run "${_wrapper}" "$@"; else run gradle "$@"; fi
}
is_gradle() { exists 'build.gradle' || exists 'build.gradle.kts'; }

# The one solution or project file `dotnet` should act on, when a directory
# has several (dotnet refuses to choose); empty when it can choose itself.
dotnet_target() {
    local _solutions
    _solutions="$(ls -- *.sln *.slnx 2>/dev/null | head -2 || true)"
    if [ "$(printf '%s' "${_solutions}" | grep -c .)" -eq 1 ]; then
        echo "${_solutions}"
    fi
}

# How `dotnet test` takes a target: with Microsoft.Testing.Platform selected in
# global.json (.NET 10), as --solution or --project; otherwise positionally.
dotnet_test_target() {
    [ -n "$1" ] || return 0
    if [ -f global.json ] && grep -q '"Microsoft.Testing.Platform"' global.json; then
        case "$1" in
            *.sln | *.slnx) echo "--solution $1" ;;
            *) echo "--project $1" ;;
        esac
    else
        echo "$1"
    fi
}

# .NET (C#, F#, VB.NET): the solution or project in the directory, else in
# each directory below with one (protobuf's csharp/src/), else the nearest
# above, acting there. `dotnet format` checks C# and VB.NET only, so F#'s lint
# hook is its own.
dotnet_hook() {
    find_build '*.sln' '*.slnx' '*.csproj' '*.fsproj' '*.vbproj' ||
        unknown "no .sln, .slnx or project file here, below, or above it in the project"
    note_build_dir
    cd "${build_dir}"
    local _target
    _target="$(dotnet_target)"
    case "$1" in
        test)
            local _args=()
            read -r -a _args <<<"$(dotnet_test_target "${_target}")"
            run dotnet test ${_args[@]+"${_args[@]}"} ;;
        build) run dotnet build ${_target:+"${_target}"} ;;
        clean) run dotnet clean ${_target:+"${_target}"} ;;
        lint)
            checks_format
            run dotnet format ${_target:+"${_target}"} --verify-no-changes ;;
    esac
}

# SwiftPM: the package here, each package below, or the one above whose
# targets include this directory's sources (FlatBuffers' Package.swift at the
# root builds swift/Sources), which swift_package_args points SwiftPM at.
# shellcheck disable=SC2034 # read by the swift hooks
swift_package_args=()
swift_package() {
    find_build Package.swift || unknown "no Package.swift here, below, or above it in the project"
    note_build_dir
    # shellcheck disable=SC2034 # read by the swift hooks
    [ "${build_dir}" = . ] || swift_package_args=(--package-path "${build_dir}")
}

# Janet's jpm, in the project's local tree (jpm -l) when it keeps its
# dependencies in one.
jpm_() { if [ -d jpm_tree ]; then run jpm -l "$@"; else run jpm "$@"; fi; }

# Common Lisp: ASDF systems on SBCL, found from this directory's .asd files
# (each a system named after its file; foo-test.asd and foo-tests.asd are
# left to foo's test-op, unless they are all there is). The test passes when
# the system's test-op signals no error. The user's init file still loads,
# so a Quicklisp it sets up finds dependencies.
asdf_hook() {
    exists '*.asd' || unknown "no .asd file"
    local _asd _system _systems=() _tests=()
    for _asd in *.asd; do
        _system="${_asd%.asd}"
        case "${_system}" in
            *-test | *-tests) _tests+=("${_system}") ;;
            *) _systems+=("${_system}") ;;
        esac
    done
    [ "${#_systems[@]}" -gt 0 ] || _systems=("${_tests[@]}")
    case "$1" in
        test) for _system in "${_systems[@]}"; do asdf_run "(asdf:test-system \"${_system}\")"; done ;;
        build) for _system in "${_systems[@]}"; do asdf_run "(asdf:load-system \"${_system}\" :force t)"; done ;;
        lint) skip "Common Lisp has no standard linter" ;;
        clean) asdf_clean ;;
    esac
}
asdf_run() {
    run sbcl --noinform --non-interactive \
        --eval '(require :asdf)' \
        --eval '(push (uiop:getcwd) asdf:*central-registry*)' \
        --eval "$1"
}
# The fasls compiled in place, and ASDF's cache of this directory's
# (~/.cache/common-lisp/<implementation>/<this path>), never this directory
# itself if output translations are off.
asdf_clean() {
    local _cache
    _cache="$(sbcl --noinform --non-interactive --eval '(require :asdf)' \
        --eval '(format t "~&cache: ~a~%" (namestring (asdf:apply-output-translations (uiop:getcwd))))' |
        sed -n 's/^cache: //p')"
    find_files -name '*.fasl'
    if [ "${#found_files[@]}" -gt 0 ]; then run rm -f "${found_files[@]}"; fi
    case "${_cache}" in
        "" | "$(pwd)" | "$(pwd)/") ;;
        */common-lisp/*) [ ! -d "${_cache}" ] || run rm -rf "${_cache}" ;;
    esac
}

# Pharo: a Tonel repository (its directory from .project's srcDirectory,
# else src/) with a BaselineOf package, loaded with Metacello into a copy of
# a fresh Pharo image in build/, whose packages' tests then run. The fresh
# image is downloaded once, for the Pharo version the installer's VM runs.
_PHARO_VERSION="120"
pharo_hook() {
    local _src="src" _baseline _packages=() _package
    if [ -f .project ]; then
        _src="$(sed -n "s/.*'srcDirectory'[[:space:]]*:[[:space:]]*'\([^']*\)'.*/\1/p" .project)"
        _src="${_src:-.}"
    fi
    _baseline="$(cd "${_src}" 2>/dev/null && ls -d BaselineOf* 2>/dev/null | head -1 || true)"
    case "$1" in
        lint) skip "Pharo has no standard command-line linter" ;;
        clean) run rm -rf build; return ;;
    esac
    [ -n "${_baseline}" ] || unknown "no Tonel repository with a BaselineOf package (in .project's srcDirectory, or src/)"
    pharo_image
    run pharo build/Pharo.image metacello install "tonel://$(cd "${_src}" && pwd)" "${_baseline}"
    [ "$1" = test ] || return 0
    for _package in "${_src}"/*/; do
        _package="$(basename "${_package}")"
        case "${_package}" in BaselineOf*) ;; *) _packages+=("${_package}") ;; esac
    done
    [ "${#_packages[@]}" -gt 0 ] || unknown "no packages besides ${_baseline} in ${_src}/"
    run pharo build/Pharo.image test --fail-on-failure "${_packages[@]}"
}
pharo_image() {
    local _fresh="${XDG_CACHE_HOME:-${HOME}/.cache}/hecatron/pharo/${_PHARO_VERSION}"
    [ -f build/Pharo.image ] && return
    if [ ! -f "${_fresh}/Pharo.image" ]; then
        echo "+ downloading a fresh Pharo ${_PHARO_VERSION} image into ${_fresh}"
        mkdir -p "${_fresh}"
        (cd "${_fresh}" && curl -fsSL "https://get.pharo.org/64/${_PHARO_VERSION}" | bash)
    fi
    mkdir -p build
    run cp "${_fresh}"/Pharo.image "${_fresh}"/Pharo.changes "${_fresh}"/*.sources build/
}

# Haxe: the project's .hxml build files. The test hook compiles and runs
# test.hxml (or tests.hxml), which runs its tests itself (--interp, or a
# --cmd after the build); the build hook build.hxml, else every other .hxml;
# both install the haxelib libraries the file names first. Clean removes
# the outputs the .hxml files name, inside this directory.
haxe_hook() {
    local _hxml _builds=()
    case "$1" in
        test)
            for _hxml in test.hxml tests.hxml; do
                [ -f "${_hxml}" ] || continue
                haxe_build "${_hxml}"
                return
            done
            unknown "no test.hxml or tests.hxml" ;;
        build)
            if [ -f build.hxml ]; then
                _builds=(build.hxml)
            else
                for _hxml in *.hxml; do
                    case "${_hxml}" in test.hxml | tests.hxml | "*.hxml") ;; *) _builds+=("${_hxml}") ;; esac
                done
            fi
            [ "${#_builds[@]}" -gt 0 ] || skip "no build.hxml or other .hxml besides the tests'"
            for _hxml in "${_builds[@]}"; do haxe_build "${_hxml}"; done ;;
        lint) skip "Haxe has no standard linter" ;;
        clean) haxe_clean ;;
    esac
}
haxe_build() {
    haxe_fetch "$1"
    run haxe "$1"
}
# The haxelib libraries an .hxml file names.
haxe_fetch() {
    if grep -qE '^[[:space:]]*(-lib|-L|--library)[[:space:]]' "$1"; then
        run haxelib install --always "$1"
    fi
}
# Each target's output path in the .hxml files here (--js out.js, --cpp
# dir...), when it is relative and stays inside this directory.
haxe_outputs() {
    local _hxml _line _words _flag _index
    for _hxml in *.hxml; do
        [ -f "${_hxml}" ] || continue
        while IFS= read -r _line || [ -n "${_line}" ]; do
            read -r -a _words <<<"${_line}"
            for ((_index = 0; _index + 1 < ${#_words[@]}; _index++)); do
                _flag="${_words[${_index}]}"
                case "${_flag}" in
                    -js | --js | -lua | --lua | -python | --python | -neko | --neko | -hl | --hl | -swf | --swf | \
                        -php | --php | -cpp | --cpp | -cppia | --cppia | -cs | --cs | -java | --java | -jvm | --jvm) ;;
                    *) continue ;;
                esac
                case "${_words[$((_index + 1))]}" in
                    /* | ..* | */..* | . | ./) ;;
                    *) echo "${_words[$((_index + 1))]}" ;;
                esac
            done
        done <"${_hxml}"
    done
}
haxe_clean() {
    local _outputs=() _output
    while IFS= read -r _output; do
        if [ -e "${_output}" ]; then _outputs+=("${_output}"); fi
    done < <(haxe_outputs | sort -u)
    [ "${#_outputs[@]}" -gt 0 ] || skip "no build outputs from the .hxml files here"
    run rm -rf "${_outputs[@]}"
}

# Whether package.json defines an npm script, e.g. `npm_script test`.
npm_script() {
    [ -f package.json ] && python3 -c '
import json, sys
sys.exit(0 if sys.argv[1] in (json.load(open("package.json")).get("scripts") or {}) else 1)
' "$1"
}
# Each ecosystem's dependencies come from one function, *_fetch, which
# `hecatron deps` runs (languages/<lang>/hooks/deps.sh) and the hooks that
# need them run first when they aren't there yet (*_deps). Each installs
# what the lock file says when there is one, and fails rather than change it.

# Node.js: with the package's own package manager (node_pm_command first).
node_fetch() {
    node_install_command
    run "${node_pm[@]}" "${node_install_args[@]}"
}
# When they aren't installed yet (in node_modules, or Yarn Plug'n'Play's
# .pnp.cjs).
npm_deps() {
    { [ -d node_modules ] || [ -f .pnp.cjs ]; } && return
    node_fetch
}

# Mix: deps.get, which installs what mix.lock locks.
mix_fetch() { run mix deps.get; }
# A locked project's dependencies, when they aren't fetched yet.
mix_deps() {
    if [ -f mix.lock ] && [ ! -d deps ]; then mix_fetch; fi
}

# Whether composer.json defines a script, e.g. `composer_script test`.
composer_script() {
    [ -f composer.json ] && python3 -c '
import json, sys
sys.exit(0 if sys.argv[1] in (json.load(open("composer.json")).get("scripts") or {}) else 1)
' "$1"
}
# Composer: install, which installs what composer.lock locks (and without
# one, resolves and writes it).
composer_fetch() { run composer install --no-interaction; }
# When they aren't installed yet.
composer_deps() {
    if [ ! -d vendor ]; then composer_fetch; fi
}

# Ruby: the bundle and Rakefile here, else the nearest above (Bundler and
# Rake look upward themselves: gRPC's src/ruby is in the root's bundle), in
# ruby_root; else each bundle below, running the hook there.
ruby_root=.
ruby_project() {
    has_any Gemfile Rakefile '*.gemspec' && return 0
    if find_build --above-first Gemfile Rakefile; then
        ruby_root="${build_dir}"
        note_build_dir
        if [ -f "${ruby_root}/Gemfile" ]; then export BUNDLE_GEMFILE="${ruby_root}/Gemfile"; fi
    fi
}
# Bundler: the bundle, frozen to Gemfile.lock when there is one.
bundle_fetch() {
    if [ -f "${ruby_root}/Gemfile.lock" ]; then BUNDLE_FROZEN=true run bundle install; else run bundle install; fi
}
# The bundle, when it isn't installed yet; then a command through it (or
# directly, without a Gemfile).
bundle_deps() {
    if [ -f "${ruby_root}/Gemfile" ] && ! bundle check >/dev/null 2>&1; then bundle_fetch; fi
}
ruby_run() {
    if [ -f "${ruby_root}/Gemfile" ]; then run bundle exec "$@"; else run "$@"; fi
}
# Whether the bundle (or the gemspec) names a gem, e.g. `gem_listed rubocop`.
gem_listed() {
    grep -qsE "^ {4}$1 \(|['\"]$1['\"]" "${ruby_root}/Gemfile.lock" "${ruby_root}/Gemfile" "${ruby_root}"/*.gemspec
}
# Whether the Rakefile defines a task, described or not, e.g. `rake_has test`.
rake_has() {
    local _tasks
    if [ -f "${ruby_root}/Gemfile" ]; then _tasks="$(bundle exec rake -P 2>/dev/null)"; else _tasks="$(rake -P 2>/dev/null)"; fi
    grep -qx "rake $1" <<<"${_tasks}"
}

# Perl: configure once (Makefile.PL writes a Makefile, Build.PL a Build script).
makemaker_configure() { [ -f Makefile ] || run perl Makefile.PL; }
module_build_configure() { [ -x Build ] || run perl Build.PL; }

# Dart: the flutter tool for a package on the Flutter SDK, else dart; and
# the package's dependencies (pub get, held to pubspec.lock), first when they
# aren't fetched yet.
dart_tool() {
    if grep -qE 'sdk:[[:space:]]*flutter([^[:alnum:]_]|$)' pubspec.yaml; then echo flutter; else echo dart; fi
}
dart_fetch() {
    if [ -f pubspec.lock ]; then run "$(dart_tool)" pub get --enforce-lockfile; else run "$(dart_tool)" pub get; fi
}
dart_deps() {
    if [ ! -f .dart_tool/package_config.json ]; then dart_fetch; fi
}

# Node.js packages (TypeScript, JavaScript): the package's own scripts, run
# by the package manager it declares (typescript/package-manager.sh).
node_hook() {
    [ -f package.json ] || unknown "no package.json"
    # shellcheck source=languages/typescript/package-manager.sh
    source "$(dirname "${BASH_SOURCE[0]}")/typescript/package-manager.sh"
    node_pm_command
    case "$1" in
        test)
            npm_deps
            npm_script test || unknown "package.json has no test script"
            run "${node_pm[@]}" run test ;;
        lint)
            npm_deps
            if npm_script lint; then run "${node_pm[@]}" run lint
            elif [ -f tsconfig.json ] && [ -x node_modules/.bin/tsc ]; then run node_modules/.bin/tsc --noEmit
            else skip "package.json has no lint script"; fi ;;
        build)
            npm_deps
            if npm_script build; then run "${node_pm[@]}" run build
            elif [ -f tsconfig.json ] && [ -x node_modules/.bin/tsc ]; then run node_modules/.bin/tsc
            else skip "package.json has no build script"; fi ;;
        clean)
            if npm_script clean; then run "${node_pm[@]}" run clean
            else skip "package.json has no clean script"; fi ;;
    esac
}

# JVM builds (Java, Kotlin, Scala, Groovy): Maven, Gradle or sbt, whichever
# the directory has, through its wrapper when it ships one. Gradle's `check`
# without tests runs whatever checkers the build configures (Checkstyle,
# Spotless, detekt...).
#
# Without a build file here: a Gradle settings file above makes this
# directory one of that build's projects (Beam's sdks/java, whose tasks run
# for every project under it), and Gradle, run from here, acts on it. Else
# the builds in the directories below, each; else a Makefile above with a
# target for this directory (make_target_above).
jvm_hook() {
    local _gradle_above=""
    if ! has_any pom.xml build.gradle build.gradle.kts build.sbt; then
        if _gradle_above="$(dir_above_with settings.gradle settings.gradle.kts)"; then
            echo "${HECATRON_LANG:-}: ${_hook_action}: $(pwd) is a project of the Gradle build in ${_gradle_above}"
        else
            find_build pom.xml build.gradle build.gradle.kts build.sbt || true
            make_target_above "$1"
            unknown "no pom.xml, build.gradle(.kts) or build.sbt here or below, no Gradle settings above, and no Makefile above with a target for $(basename "$(pwd)")"
        fi
    fi
    jvm_select_jdk
    if [ -f pom.xml ]; then
        case "$1" in
            test) maven test ;;
            build) maven package -DskipTests ;;
            clean) maven clean ;;
            lint) skip "Maven has no standard lint goal" ;;
            format) jvm_format_maven ;;
            deps) maven dependency:go-offline ;;
        esac
    elif is_gradle || [ -n "${_gradle_above}" ]; then
        case "$1" in
            test) gradle_ test ;;
            build) gradle_ assemble ;;
            clean) gradle_ clean ;;
            lint) gradle_ check -x test ;;
            format) jvm_format_gradle "${_gradle_above:-.}" ;;
            # Resolving the dependency report downloads what it lists
            # (held to gradle.lockfile where the build locks).
            deps) gradle_ dependencies ;;
        esac
    elif [ -f build.sbt ]; then
        case "$1" in
            test) run sbt test ;;
            build) run sbt compile ;;
            clean) run sbt clean ;;
            lint) skip "sbt has no standard lint task" ;;
            format) jvm_format_sbt ;;
            deps) run sbt update ;;
        esac
    else
        unknown "no pom.xml, build.gradle(.kts) or build.sbt"
    fi
}

# Formatting is a plugin the build configures, run through the build so it
# formats as the project's CI does: Spotless, or fmt-maven-plugin (Maven);
# Spotless, ktlint-gradle or Kotlinter (Gradle); sbt-scalafmt (sbt). None is
# a default: each JVM language has several styles.
jvm_format_maven() {
    local _poms=(pom.xml) _dir
    while IFS= read -r _dir; do
        if [ -f "${_dir}/pom.xml" ]; then _poms+=("${_dir}/pom.xml"); fi
    done < <(dirs_above)
    if grep -qs 'spotless-maven-plugin' "${_poms[@]}"; then
        if format_checking; then maven spotless:check; else maven spotless:apply; fi
    elif grep -qs 'fmt-maven-plugin' "${_poms[@]}"; then
        if format_checking; then maven com.spotify.fmt:fmt-maven-plugin:check; else maven com.spotify.fmt:fmt-maven-plugin:format; fi
    else
        skip "no Spotless or fmt-maven-plugin in pom.xml"
    fi
}
# The build's own files: this project's, and the root's (its settings file,
# version catalog and buildSrc convention plugins) when that is above.
jvm_format_gradle() {
    local _root _build_files=() _file
    _root="$(nearest_with settings.gradle settings.gradle.kts || echo "$1")"
    while IFS= read -r -d '' _file; do _build_files+=("${_file}"); done < <(
        find . "${_root}" -maxdepth 3 \( -name build -o -name .gradle -o -name node_modules \) -prune -o \
            -type f \( -name '*.gradle' -o -name '*.gradle.kts' -o -name '*.versions.toml' \) -print0 2>/dev/null)
    if grep -qs 'spotless' ${_build_files[@]+"${_build_files[@]}"}; then
        if format_checking; then gradle_ spotlessCheck; else gradle_ spotlessApply; fi
    elif grep -qs 'org.jlleitschuh.gradle.ktlint' ${_build_files[@]+"${_build_files[@]}"}; then
        if format_checking; then gradle_ ktlintCheck; else gradle_ ktlintFormat; fi
    elif grep -qs 'org.jmailen.kotlinter' ${_build_files[@]+"${_build_files[@]}"}; then
        if format_checking; then gradle_ lintKotlin; else gradle_ formatKotlin; fi
    else
        skip "no Spotless, ktlint-gradle or Kotlinter in the Gradle build"
    fi
}
jvm_format_sbt() {
    grep -qs 'sbt-scalafmt' project/plugins.sbt || skip "no sbt-scalafmt in project/plugins.sbt"
    if format_checking; then run sbt scalafmtCheckAll scalafmtSbtCheck; else run sbt scalafmtAll scalafmtSbt; fi
}

# A build whose Gradle wrapper or Kotlin compiler predates the JDK it would
# run on fails to start on it, so it runs on the newest installed JDK it
# supports instead (libexec/lib/jvm.py; the JVM installers install one).
jvm_select_jdk() {
    [ -n "${HECATRON_HOME:-}" ] || return 0
    local _ceiling="" _why _current _version _home
    IFS=$'\t' read -r _ceiling _why < <(python3 "${HECATRON_HOME}/libexec/lib/jvm.py" ceiling . "${HECATRON_ROOT:-.}") || true
    [ -n "${_ceiling}" ] || return 0
    _current="$(java_feature "$( (java -version 2>&1 || true) | sed -n 's/.* version "\([^"]*\)".*/\1/p' | head -1)")"
    { [ -n "${_current}" ] && [ "${_current}" -le "${_ceiling}" ]; } && return 0
    while IFS=$'\t' read -r _version _home; do
        [ "$(java_feature "${_version}")" -le "${_ceiling}" ] && break
        _home=""
    done < <(bash "${HECATRON_HOME}/languages/java/installed.sh" 2>/dev/null | sort -t. -k1,1nr -k2,2nr -k3,3nr)
    if [ -z "${_home:-}" ]; then
        echo "warning: ${_why}, which runs on JDK ${_ceiling} at the newest, and no such JDK is installed (hecatron install ${HECATRON_LANG:-})" >&2
        return 0
    fi
    echo "Using JDK ${_version} (${_home}): ${_why}, which runs on JDK ${_ceiling} at the newest."
    export JAVA_HOME="${_home}"
    export PATH="${_home}/bin:${PATH}"
}
# A Java version's feature release: 1.8.0_412 is 8, 21.0.2 is 21.
java_feature() { echo "$1" | sed 's/^1\.\([0-9]*\).*/\1/; s/[^0-9].*//'; }

# Clojure: the Clojure CLI (deps.edn) or Leiningen (project.clj). The CLI
# runs the :test alias as a function (-X, :exec-fn) or a main (-M,
# :main-opts), whichever it defines, and builds through tools.build's
# build.clj. Lint is clj-kondo, which the CLI fetches at a pinned version when
# it isn't installed.
_CLJ_KONDO_VERSION="2026.08.04"
clojure_hook() {
    if [ -f deps.edn ]; then
        case "$1" in
            test)
                case "$(clojure_alias_kind test)" in
                    X) run clojure -X:test ;;
                    M) run clojure -M:test ;;
                    *) unknown "deps.edn has no :test alias with :exec-fn or :main-opts" ;;
                esac ;;
            build)
                [ -f build.clj ] || skip "no build.clj (tools.build)"
                run clojure -T:build jar ;;
            clean) run rm -rf target .cpcache ;;
            lint) clj_kondo ;;
        esac
    elif [ -f project.clj ]; then
        case "$1" in
            test) run lein test ;;
            build) run lein jar ;;
            clean) run lein clean ;;
            lint) clj_kondo ;;
        esac
    else
        unknown "no deps.edn or project.clj"
    fi
}
# How deps.edn's alias runs: X (:exec-fn), M (:main-opts) or nothing.
clojure_alias_kind() {
    clojure -M -e "(require 'clojure.edn)
(let [alias (-> \"deps.edn\" slurp clojure.edn/read-string :aliases (get :$1))]
  (print (cond (:exec-fn alias) \"X\" (:main-opts alias) \"M\" :else \"\")))"
}
clj_kondo() {
    local _dirs=() _dir
    for _dir in src test; do
        if [ -d "${_dir}" ]; then _dirs+=("${_dir}"); fi
    done
    [ "${#_dirs[@]}" -gt 0 ] || skip "no src or test directory"
    if has clj-kondo; then
        run clj-kondo --lint "${_dirs[@]}"
    else
        run clojure -Sdeps "{:deps {clj-kondo/clj-kondo {:mvn/version \"${_CLJ_KONDO_VERSION}\"}}}" \
            -M -m clj-kondo.main --lint "${_dirs[@]}"
    fi
}

# Native builds (C, C++, Objective-C): CMake, Meson, Autotools or a Makefile.
# CMake and Meson build out of tree in build/ (native_build), which clean
# removes.
native_hook() {
    native_build="$(native_build_dir)"
    has_any CMakeLists.txt meson.build configure configure.ac Makefile makefile GNUmakefile ||
        native_elsewhere "$1"
    if [ -f CMakeLists.txt ]; then
        case "$1" in
            test)
                cmake_build
                cmake_enable_tests
                cmake_test ;;
            build) cmake_build ;;
            clean) run rm -rf "${native_build}" ;;
            lint) skip "no standard linter for CMake builds" ;;
        esac
    elif [ -f meson.build ]; then
        case "$1" in
            test) meson_setup; run meson test -C "${native_build}" --print-errorlogs ;;
            build) meson_setup; run meson compile -C "${native_build}" ;;
            clean) run rm -rf "${native_build}" ;;
            lint) skip "no standard linter for Meson builds" ;;
        esac
    elif [ -f configure ] || [ -f configure.ac ] || [ -f Makefile ] || [ -f makefile ] || [ -f GNUmakefile ]; then
        [ -f Makefile ] || [ -f makefile ] || [ -f GNUmakefile ] || autotools_configure
        case "$1" in
            test)
                if make_has check; then run make check
                elif make_has test; then run make test
                else unknown "the Makefile has no check or test target"; fi ;;
            build) run make ;;
            clean) if make_has clean; then run make clean; else skip "the Makefile has no clean target"; fi ;;
            lint) if make_has lint; then run make lint; else skip "the Makefile has no lint target"; fi ;;
        esac
    else
        unknown "no CMakeLists.txt, meson.build, configure or Makefile"
    fi
}
# Without a native build here: the CMake or Meson projects in the
# directories below, each; else the nearest one above, which the hook then
# acts on, building in its build/. A project above builds more than this
# directory, but it is this directory's build: gRPC's src/cpp has no build of
# its own, and the root's CMakeLists.txt is gRPC's C++ build. Only a file that
# declares a project() counts, not one that a parent's add_subdirectory()
# reads. Else a Makefile above with a target for this directory.
native_elsewhere() {
    local _dirs=() _dir
    while IFS= read -r _dir; do
        if is_native_project "${_dir}"; then _dirs+=("${_dir}"); fi
    done < <(dirs_below_with CMakeLists.txt meson.build)
    [ "${#_dirs[@]}" -eq 0 ] || run_hook_in "${_dirs[@]}"
    while IFS= read -r _dir; do
        if is_native_project "${_dir}"; then
            echo "${HECATRON_LANG:-}: ${_hook_action}: the build for $(pwd) is in ${_dir}"
            cd "${_dir}"
            return 0
        fi
    done < <(dirs_above)
    make_target_above "$1"
    unknown "no CMakeLists.txt, meson.build, configure or Makefile here, no CMake or Meson project below or above it, and no Makefile above with a target for $(basename "$(pwd)")"
}
# Where CMake and Meson build: build/, unless the project keeps files of its
# own there (Zstandard's CMake build is in build/cmake/), which configuring
# into it would mix with build output and clean would delete: then _build/.
native_build_dir() {
    if [ -d build ] && build_dir_has_sources; then echo _build; else echo build; fi
}
build_dir_has_sources() {
    if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
        [ -n "$(git ls-files -- build | head -n 1)" ]
    else
        [ ! -f build/CMakeCache.txt ] && [ ! -d build/meson-private ] && [ -n "$(ls -A build)" ]
    fi
}
is_native_project() {
    grep -qsiE '^[[:space:]]*project[[:space:]]*\(' "$1/CMakeLists.txt" "$1/meson.build"
}

# A Makefile above this directory with a target named for it or for the
# language (check_cxx, test-java...), which runs and exits: a root Makefile
# that builds every language's directory (Snowball's check_cxx for cxx/). Only
# a rule the Makefile defines counts, not a file or directory make finds.
make_target_above() {
    local _dir _name _target _targets
    _dir="$(dir_above_with Makefile makefile GNUmakefile)" || return 0
    for _name in "$(basename "$(pwd)")" "${HECATRON_LANG:-}"; do
        [ -n "${_name}" ] || continue
        case "$1" in
            test) _targets="check_${_name} check-${_name} test_${_name} test-${_name}" ;;
            build) _targets="${_name} build_${_name} build-${_name}" ;;
            clean) _targets="clean_${_name} clean-${_name}" ;;
            lint) _targets="lint_${_name} lint-${_name}" ;;
            format) _targets="format_${_name} format-${_name}" ;;
            deps) _targets="deps_${_name} deps-${_name}" ;;
        esac
        for _target in ${_targets}; do
            if make_defines "${_dir}" "${_target}"; then
                echo "${HECATRON_LANG:-}: ${_hook_action}: the build for $(pwd) is the ${_dir} Makefile's ${_target}"
                run make -C "${_dir}" "${_target}"
                exit 0
            fi
        done
    done
    case "$1" in
        clean | lint | format | deps) skip "the Makefile in ${_dir} has no ${1} target for $(basename "$(pwd)")" ;;
    esac
}
make_defines() {
    local _database
    # make -q exits 1 when the default goal is out of date: the database still prints.
    _database="$(make -C "$1" -qp 2>/dev/null || true)"
    grep -q "^$2:" <<<"${_database}"
}

# Whether the Makefile has a target (make -q exits 2 only for an unknown one).
make_has() {
    local _status=0
    make -q "$1" >/dev/null 2>&1 || _status=$?
    [ "${_status}" -ne 2 ]
}
autotools_configure() {
    [ -f configure ] || run autoreconf -fi
    run ./configure
}
meson_setup() { [ -d "${native_build}" ] || run meson setup "${native_build}"; }

# Many projects build their tests only when asked, through an option of their
# own (FOO_BUILD_TESTS, ENABLE_TESTING...) that defaults to off: turn on the
# boolean cache options that name tests, then build again. Only the project's
# own options: a dependency fetched into the build tree (FetchContent) brings
# options of its own, whose tests need what the project never set up for
# them. Not the ones that run tests under valgrind or need large memory. An
# option can need setup the project documents (a path to another library):
# when configuring with them all fails, each is tried alone, and the ones
# that fail stay off.
cmake_enable_tests() {
    local _options=() _option _kept=() _source
    _source="$(cmake_source_dir)"
    while IFS= read -r _option; do
        [ -n "${_option}" ] && _options+=("${_option}")
    done < <(cmake -N -LA "${native_build}" 2>/dev/null |
        sed -n 's/^\([A-Za-z0-9_]*\(TEST\|TESTS\|TESTING\)[A-Za-z0-9_]*\):BOOL=OFF$/\1/p' |
        grep -Eiv 'COVERAGE|FUZZ|BENCH|INSTALL|DISABLE|SKIP|(^|_)NO_|MEMCHECK|VALGRIND|LARGE_MEMORY' |
        cmake_own_options "${_source}" || true)
    [ "${#_options[@]}" -gt 0 ] || return 0
    if ! run cmake -S "${_source}" -B "${native_build}" "${_options[@]/%/=ON}"; then
        echo "Configuring with ${_options[*]} failed: leaving them off."
        run cmake -S "${_source}" -B "${native_build}" "${_options[@]/%/=OFF}"
        if [ "${#_options[@]}" -gt 1 ]; then
            echo "Trying each alone."
            for _option in "${_options[@]}"; do
                if run cmake -S "${_source}" -B "${native_build}" ${_kept[@]+"${_kept[@]/%/=ON}"} "${_option}=ON"; then
                    _kept+=("${_option}")
                else
                    run cmake -S "${_source}" -B "${native_build}" ${_kept[@]+"${_kept[@]/%/=ON}"} "${_option}=OFF"
                fi
            done
        fi
    fi
    cmake_compile
}
# Of the option names on stdin, the ones the project's own CMake files name
# (as whole words), outside its build trees and fetched dependencies.
cmake_own_options() {
    local _names
    _names="$(cat)"
    [ -n "${_names}" ] || return 0
    find "$1" \( -name _deps -o -name CMakeFiles -o -type d -exec test -f '{}/CMakeCache.txt' \; \) -prune -o \
        \( -name CMakeLists.txt -o -name '*.cmake' \) -type f -print0 2>/dev/null |
        xargs -0 grep -ohwF "${_names}" 2>/dev/null | sort -u | sed 's/^/-D/'
}

# CMake: configure once into build/, then build. A CMakeLists.txt that calls
# no project() is a subdirectory a parent's add_subdirectory() pulls in, and
# can't be configured alone: configure the nearest directory above (within
# the project) whose CMakeLists.txt calls project(), into this directory's
# build/, then build and test only this directory's part of it (its
# directory in the build tree, whose targets and tests are the ones defined
# here and below), not the rest of the project's languages.
cmake_build() {
    run cmake -S "$(cmake_source_dir)" -B "${native_build}" -DCMAKE_BUILD_TYPE=Release
    cmake_compile
}
cmake_compile() {
    local _part
    _part="$(cmake_binary_dir)"
    if [ "${_part}" = "${native_build}" ]; then
        run cmake --build "${native_build}" --parallel "$(build_jobs)"
    elif grep -q '^CMAKE_GENERATOR:INTERNAL=Ninja' "${native_build}/CMakeCache.txt"; then
        run cmake --build "${native_build}" --parallel "$(build_jobs)" --target "${_part#"${native_build}"/}/all"
    else
        run make -C "${_part}" -j "$(build_jobs)"
    fi
}
# CTest's tests; or, for a project that registers none and runs its tests
# from a custom target instead (add_custom_target(check ...) or tests), that
# target. A project whose CMake files never mention a test has none to run
# (a tree-sitter grammar's C library, tested by the grammar's own tooling):
# it built, and that is all there is. One that does, but registered none,
# still fails: its tests were left out.
cmake_test() {
    local _part _target
    _part="$(cmake_binary_dir)"
    if ! ctest --test-dir "${_part}" -N 2>/dev/null | grep -q '^Total Tests: [1-9]'; then
        _target="$(cmake_test_target)"
        if [ -n "${_target}" ]; then
            run cmake --build "${native_build}" --parallel "$(build_jobs)" --target "${_target}"
            return
        fi
        cmake_mentions_tests "$(cmake_source_dir)" ||
            skip "the CMake project defines no tests (no add_test, enable_testing or CTest)"
    fi
    run ctest --test-dir "${_part}" --output-on-failure --no-tests=error
}
# Whether the project's own CMake files (outside its build trees and
# fetched dependencies) set up any test.
cmake_mentions_tests() {
    [ -n "$(find "$1" \( -name _deps -o -name CMakeFiles -o -type d -exec test -f '{}/CMakeCache.txt' \; \) -prune -o \
        \( -name CMakeLists.txt -o -name '*.cmake' \) -type f -print0 2>/dev/null |
        xargs -0 grep -lsiE 'add_test|enable_testing|include[[:space:]]*\([[:space:]]*CTest|_discover_tests|_add_tests' 2>/dev/null |
        head -n 1)" ]
}
cmake_test_target() {
    local _target
    for _target in check tests; do
        if grep -qsiE "add_custom_target[[:space:]]*\([[:space:]]*${_target}([[:space:]]|\)|$)" CMakeLists.txt; then
            echo "${_target}"
            return
        fi
    done
}
cmake_source_dir() {
    local _dir
    _dir="$(pwd)"
    while ! cmake_has_project "${_dir}"; do
        if [ "${_dir}" = "${HECATRON_ROOT:-/}" ] || [ "${_dir}" = / ]; then
            pwd
            return 0
        fi
        _dir="$(dirname "${_dir}")"
    done
    echo "${_dir}"
}
cmake_has_project() { grep -qiE '^[[:space:]]*project[[:space:]]*\(' "$1/CMakeLists.txt" 2>/dev/null; }
cmake_binary_dir() {
    local _source _here
    _source="$(cmake_source_dir)"
    _here="$(pwd)"
    echo "${native_build}${_here#"${_source}"}"
}

# How many compile jobs to run at once: one per CPU the machine (or the
# container's CPU quota) gives, but at most one per 2 GB of memory (the
# container's limit, when it has one), which a large C++ file can take to
# compile. Bare `cmake --build --parallel` lets make start any number, and on
# a big project the kernel then kills compilers for memory.
# CMAKE_BUILD_PARALLEL_LEVEL, when set, decides.
build_jobs() {
    local _cpus _quota _period _memory_kb _limit
    if [ -n "${CMAKE_BUILD_PARALLEL_LEVEL:-}" ]; then
        echo "${CMAKE_BUILD_PARALLEL_LEVEL}"
        return
    fi
    _cpus="$(getconf _NPROCESSORS_ONLN 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 1)"
    if read -r _quota _period </sys/fs/cgroup/cpu.max 2>/dev/null && [ "${_quota}" != max ]; then
        _quota=$(((_quota + _period - 1) / _period))
        [ "${_quota}" -ge "${_cpus}" ] || _cpus="${_quota}"
    fi
    if [ -r /proc/meminfo ]; then
        _memory_kb="$(sed -n 's/^MemTotal:[[:space:]]*\([0-9]*\).*/\1/p' /proc/meminfo)"
    else
        _memory_kb=$(($(sysctl -n hw.memsize 2>/dev/null || echo 0) / 1024))
    fi
    _limit="$(cat /sys/fs/cgroup/memory.max 2>/dev/null || true)"
    case "${_limit}" in
        "" | max | *[!0-9]*) ;;
        *) [ $((_limit / 1024)) -ge "${_memory_kb:-0}" ] || _memory_kb=$((_limit / 1024)) ;;
    esac
    _memory_kb="${_memory_kb:-0}"
    if [ "${_memory_kb}" -gt 0 ] && [ $((_memory_kb / 2097152)) -lt "${_cpus}" ]; then
        _cpus=$((_memory_kb / 2097152))
    fi
    [ "${_cpus}" -ge 1 ] || _cpus=1
    echo "${_cpus}"
}

# Objective-C: xcodebuild for an Xcode project on macOS (untested: no macOS
# here; it acts on the project's default scheme and target), GNUstep make for
# a GNUmakefile that includes GNUstep's common.make (which needs
# GNUSTEP_MAKEFILES, from gnustep-config), else CMake, Meson or make as for C.
objc_hook() {
    if exists '*.xcodeproj' && has xcodebuild; then
        case "$1" in
            test) run xcodebuild test ;;
            build) run xcodebuild build ;;
            clean) run xcodebuild clean ;;
            lint) skip "xcodebuild has no standard lint action" ;;
        esac
        return
    fi
    if is_gnustep_make && [ -z "${GNUSTEP_MAKEFILES:-}" ]; then
        has gnustep-config || unknown "the GNUmakefile uses GNUstep make, and gnustep-config is not installed"
        GNUSTEP_MAKEFILES="$(gnustep-config --variable=GNUSTEP_MAKEFILES)"
        export GNUSTEP_MAKEFILES
    fi
    native_hook "$1"
}
is_gnustep_make() { [ -f GNUmakefile ] && grep -q 'GNUSTEP_MAKEFILES' GNUmakefile; }
