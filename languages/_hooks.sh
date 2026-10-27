# Shared helpers for the built-in default hooks (languages/<lang>/hooks/*.sh;
# sourced, not executed). A hook runs inside one component's directory for
# its language (each-component.sh), picks the command from the build files it
# finds there, and says so. A project replaces any hook with its own
# .hecatron/languages/<lang>/<action>.sh.
# shellcheck shell=bash

set -euo pipefail

_hook_action="$(basename "$0" .sh)"

has() { command -v "$1" &>/dev/null; }

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

# A Maven or Gradle wrapper when the project ships one, else the tool.
maven() { if [ -x ./mvnw ]; then run ./mvnw -B "$@"; else run mvn -B "$@"; fi; }
gradle_() { if [ -x ./gradlew ]; then run ./gradlew "$@"; else run gradle "$@"; fi; }
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

# Whether package.json defines an npm script, e.g. `npm_script test`.
npm_script() {
    [ -f package.json ] && python3 -c '
import json, sys
sys.exit(0 if sys.argv[1] in (json.load(open("package.json")).get("scripts") or {}) else 1)
' "$1"
}
# Dependencies first, when they aren't installed yet.
npm_deps() {
    [ -d node_modules ] && return
    if [ -f package-lock.json ]; then run npm ci; else run npm install; fi
}

# Node.js packages (TypeScript, JavaScript): the package's own npm scripts.
node_hook() {
    [ -f package.json ] || unknown "no package.json"
    case "$1" in
        test)
            npm_deps
            npm_script test || unknown "package.json has no test script"
            run npm test ;;
        lint)
            npm_deps
            if npm_script lint; then run npm run lint
            elif [ -f tsconfig.json ] && [ -x node_modules/.bin/tsc ]; then run node_modules/.bin/tsc --noEmit
            else skip "package.json has no lint script"; fi ;;
        build)
            npm_deps
            if npm_script build; then run npm run build
            elif [ -f tsconfig.json ] && [ -x node_modules/.bin/tsc ]; then run node_modules/.bin/tsc
            else skip "package.json has no build script"; fi ;;
        clean)
            if npm_script clean; then run npm run clean
            else skip "package.json has no clean script"; fi ;;
    esac
}

# JVM builds (Java, Kotlin, Scala, Groovy): Maven, Gradle or sbt, whichever
# the directory has, through its wrapper when it ships one. Gradle's `check`
# without tests runs whatever checkers the build configures (Checkstyle,
# Spotless, detekt...).
jvm_hook() {
    if [ -f pom.xml ]; then
        case "$1" in
            test) maven test ;;
            build) maven package -DskipTests ;;
            clean) maven clean ;;
            lint) skip "Maven has no standard lint goal" ;;
        esac
    elif is_gradle; then
        case "$1" in
            test) gradle_ test ;;
            build) gradle_ assemble ;;
            clean) gradle_ clean ;;
            lint) gradle_ check -x test ;;
        esac
    elif [ -f build.sbt ]; then
        case "$1" in
            test) run sbt test ;;
            build) run sbt compile ;;
            clean) run sbt clean ;;
            lint) skip "sbt has no standard lint task" ;;
        esac
    else
        unknown "no pom.xml, build.gradle(.kts) or build.sbt"
    fi
}

# Native builds (C, C++, Objective-C): CMake, Meson, Autotools or a Makefile.
# CMake and Meson build out of tree in build/, which clean removes.
native_hook() {
    if [ -f CMakeLists.txt ]; then
        case "$1" in
            test) cmake_build; run ctest --test-dir build --output-on-failure ;;
            build) cmake_build ;;
            clean) run rm -rf build ;;
            lint) skip "no standard linter for CMake builds" ;;
        esac
    elif [ -f meson.build ]; then
        case "$1" in
            test) meson_setup; run meson test -C build --print-errorlogs ;;
            build) meson_setup; run meson compile -C build ;;
            clean) run rm -rf build ;;
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
meson_setup() { [ -d build ] || run meson setup build; }

# CMake: configure once into build/, then build.
cmake_build() {
    run cmake -S . -B build -DCMAKE_BUILD_TYPE=Release
    run cmake --build build --parallel
}
