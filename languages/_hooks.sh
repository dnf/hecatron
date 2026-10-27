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
