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

# .NET (C#, F#, VB.NET): the solution or project in the directory. `dotnet
# format` checks C# and VB.NET only, so F#'s lint hook is its own.
dotnet_hook() {
    exists '*.sln' || exists '*.slnx' || exists '*.csproj' || exists '*.fsproj' || exists '*.vbproj' ||
        unknown "no .sln, .slnx or project file"
    local _target
    _target="$(dotnet_target)"
    case "$1" in
        test)
            local _args=()
            read -r -a _args <<<"$(dotnet_test_target "${_target}")"
            run dotnet test ${_args[@]+"${_args[@]}"} ;;
        build) run dotnet build ${_target:+"${_target}"} ;;
        clean) run dotnet clean ${_target:+"${_target}"} ;;
        lint) run dotnet format ${_target:+"${_target}"} --verify-no-changes ;;
    esac
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
    if grep -qE '^[[:space:]]*(-lib|-L|--library)[[:space:]]' "$1"; then
        run haxelib install --always "$1"
    fi
    run haxe "$1"
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
# Dependencies first, when they aren't installed yet.
npm_deps() {
    [ -d node_modules ] && return
    if [ -f package-lock.json ]; then run npm ci; else run npm install; fi
}

# Mix: a locked project's dependencies first, when they aren't fetched yet.
mix_deps() {
    if [ -f mix.lock ] && [ ! -d deps ]; then run mix deps.get; fi
}

# Whether composer.json defines a script, e.g. `composer_script test`.
composer_script() {
    [ -f composer.json ] && python3 -c '
import json, sys
sys.exit(0 if sys.argv[1] in (json.load(open("composer.json")).get("scripts") or {}) else 1)
' "$1"
}
# Composer: dependencies first, when they aren't installed yet.
composer_deps() {
    if [ ! -d vendor ]; then run composer install --no-interaction; fi
}

# Bundler: the bundle first, when it isn't installed yet; then a command
# through it (or directly, without a Gemfile).
bundle_deps() {
    if [ -f Gemfile ] && ! bundle check >/dev/null 2>&1; then run bundle install; fi
}
ruby_run() {
    if [ -f Gemfile ]; then run bundle exec "$@"; else run "$@"; fi
}
# Whether the bundle (or the gemspec) names a gem, e.g. `gem_listed rubocop`.
gem_listed() {
    grep -qsE "^ {4}$1 \(|['\"]$1['\"]" Gemfile.lock Gemfile ./*.gemspec
}
# Whether the Rakefile defines a task, described or not, e.g. `rake_has test`.
rake_has() {
    local _tasks
    if [ -f Gemfile ]; then _tasks="$(bundle exec rake -P 2>/dev/null)"; else _tasks="$(rake -P 2>/dev/null)"; fi
    grep -qx "rake $1" <<<"${_tasks}"
}

# Perl: configure once (Makefile.PL writes a Makefile, Build.PL a Build script).
makemaker_configure() { [ -f Makefile ] || run perl Makefile.PL; }
module_build_configure() { [ -x Build ] || run perl Build.PL; }

# Dart: the flutter tool for a package on the Flutter SDK, else dart; and
# the package's dependencies first, when they aren't fetched yet.
dart_tool() {
    if grep -qE 'sdk:[[:space:]]*flutter([^[:alnum:]_]|$)' pubspec.yaml; then echo flutter; else echo dart; fi
}
dart_deps() {
    if [ ! -f .dart_tool/package_config.json ]; then run "$(dart_tool)" pub get; fi
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
    jvm_select_jdk
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

# A build whose Gradle wrapper or Kotlin compiler predates the JDK it would
# run on fails to start on it, so it runs on the newest installed JDK it
# supports instead (libexec/lib/jvm.py; the JVM installers install one).
jvm_select_jdk() {
    [ -n "${HECATRON_HOME:-}" ] || return 0
    local _ceiling="" _why _current _version _home
    IFS=$'\t' read -r _ceiling _why < <(python3 "${HECATRON_HOME}/libexec/lib/jvm.py" ceiling .) || true
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
# CMake and Meson build out of tree in build/, which clean removes.
native_hook() {
    if [ -f CMakeLists.txt ]; then
        case "$1" in
            test)
                cmake_build
                cmake_enable_tests
                cmake_test ;;
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
    done < <(cmake -N -LA build 2>/dev/null |
        sed -n 's/^\([A-Za-z0-9_]*\(TEST\|TESTS\|TESTING\)[A-Za-z0-9_]*\):BOOL=OFF$/\1/p' |
        grep -Eiv 'COVERAGE|FUZZ|BENCH|INSTALL|DISABLE|SKIP|(^|_)NO_|MEMCHECK|VALGRIND|LARGE_MEMORY' |
        cmake_own_options "${_source}" || true)
    [ "${#_options[@]}" -gt 0 ] || return 0
    if ! run cmake -S "${_source}" -B build "${_options[@]/%/=ON}"; then
        echo "Configuring with ${_options[*]} failed: leaving them off."
        run cmake -S "${_source}" -B build "${_options[@]/%/=OFF}"
        if [ "${#_options[@]}" -gt 1 ]; then
            echo "Trying each alone."
            for _option in "${_options[@]}"; do
                if run cmake -S "${_source}" -B build ${_kept[@]+"${_kept[@]/%/=ON}"} "${_option}=ON"; then
                    _kept+=("${_option}")
                else
                    run cmake -S "${_source}" -B build ${_kept[@]+"${_kept[@]/%/=ON}"} "${_option}=OFF"
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
    run cmake -S "$(cmake_source_dir)" -B build -DCMAKE_BUILD_TYPE=Release
    cmake_compile
}
cmake_compile() {
    local _part
    _part="$(cmake_binary_dir)"
    if [ "${_part}" = build ]; then
        run cmake --build build --parallel "$(build_jobs)"
    elif grep -q '^CMAKE_GENERATOR:INTERNAL=Ninja' build/CMakeCache.txt; then
        run cmake --build build --parallel "$(build_jobs)" --target "${_part#build/}/all"
    else
        run make -C "${_part}" -j "$(build_jobs)"
    fi
}
# CTest's tests; or, for a project that registers none and runs its tests
# from a custom target instead (add_custom_target(check ...) or tests), that
# target.
cmake_test() {
    local _part _target
    _part="$(cmake_binary_dir)"
    if ! ctest --test-dir "${_part}" -N 2>/dev/null | grep -q '^Total Tests: [1-9]'; then
        _target="$(cmake_test_target)"
        if [ -n "${_target}" ]; then
            run cmake --build build --parallel "$(build_jobs)" --target "${_target}"
            return
        fi
    fi
    run ctest --test-dir "${_part}" --output-on-failure --no-tests=error
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
    echo "build${_here#"${_source}"}"
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
