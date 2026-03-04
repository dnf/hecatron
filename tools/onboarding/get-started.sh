#!/usr/bin/env bash
# Interactive onboarding: shows project info, installed languages, and install menu.
# Usage: get-started.sh [--install-all | --install LANG... | --status | --check]
set -euo pipefail

# --- Helpers ----------------------------------------------------------------

BOLD='\033[1m'
DIM='\033[2m'
GREEN='\033[32m'
CYAN='\033[36m'
RESET='\033[0m'

_hide_cursor() { printf '\033[?25l'; }
_show_cursor() { printf '\033[?25h'; }
_cursor_up() { printf '\033[%dA' "$1"; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../../languages/_common.sh
source "$SCRIPT_DIR/../../languages/_common.sh"

# Language → detection tool (matches justfile _tool())
_tool() {
    case "$1" in
        ada)         echo gnat    ;;
        c|cpp)       echo cmake   ;;
        clojure)     echo clojure ;;
        csharp)      echo dotnet  ;;
        dart)        echo dart    ;;
        elixir)      echo mix     ;;
        elm)         echo elm     ;;
        erlang)      echo rebar3  ;;
        fsharp)      echo dotnet  ;;
        fortran)     echo gfortran ;;
        go)          echo go      ;;
        haskell)     echo cabal   ;;
        groovy|java|kotlin|scala) echo java ;;
        ocaml)       echo opam    ;;
        perl)        echo perl    ;;
        php)         echo php     ;;
        python)      echo uv      ;;
        ruby)        echo ruby    ;;
        rust)        echo cargo   ;;
        swift)       echo swift   ;;
        typescript)  echo npm     ;;
        zig)         echo zig     ;;
    esac
}

ALL_LANGS=(ada c clojure cpp csharp dart elixir elm erlang fsharp fortran gleam go groovy haskell java kotlin ocaml perl php python ruby rust scala swift typescript zig)

# Languages unavailable on certain architectures
_unavailable() {
    local lang="$1"
    # Elm has no arm64 Linux binary (https://github.com/elm/compiler/issues/2292)
    if [ "$lang" = "elm" ] && [ "$OS" = "linux" ]; then
        local arch; arch="$(uname -m)"
        if [ "$arch" = "aarch64" ] || [ "$arch" = "arm64" ]; then
            return 0
        fi
    fi
    return 1
}

# Display name (for the status table)
_display() {
    case "$1" in
        cpp)        echo "C++"        ;;
        csharp)     echo "C#"         ;;
        fsharp)     echo "F#"         ;;
        ocaml)      echo "OCaml"      ;;
        php)        echo "PHP"        ;;
        typescript) echo "TypeScript" ;;
        *)          # Capitalize first letter
            first="$(echo "${1:0:1}" | tr '[:lower:]' '[:upper:]')"
            echo "${first}${1:1}"
            ;;
    esac
}

# --- Detect languages -------------------------------------------------------

installed=()
missing=()
unavailable=()
for lang in "${ALL_LANGS[@]}"; do
    if _unavailable "$lang"; then
        unavailable+=("$lang")
    else
        tool=$(_tool "$lang")
        if has "$tool"; then
            installed+=("$lang")
        else
            missing+=("$lang")
        fi
    fi
done
available_total=$(( ${#ALL_LANGS[@]} - ${#unavailable[@]} ))

# --- CLI flags ---------------------------------------------------------------

_usage() {
    cat <<'EOF'
Usage: get-started.sh [OPTION]

Interactive onboarding (default when run with no options).

Options:
  --status        Print language status table and exit
  --check         Exit 0 if all languages installed, 1 if any missing
  --install-all   Install all missing languages (non-interactive)
  --install LANG  Install specific languages (non-interactive)
  -h, --help      Show this help
EOF
}

_print_intro() {
    echo ""
    echo -e "${BOLD}sayable-age${RESET}"
    echo "Formats ages the way parents talk — \"6 weeks\", \"14 months\", \"3½ years\"."
    echo "23 identical implementations, 71 locales, zero dependencies."
    echo ""
}

_print_status() {
    local col_w=16
    local cols=4

    echo -e "${BOLD}Language toolchains:${RESET}"
    echo ""

    local i=0
    for lang in "${ALL_LANGS[@]}"; do
        local display
        display=$(_display "$lang")
        if _unavailable "$lang"; then
            printf "  ${DIM}%-2s %-${col_w}s${RESET}" "–" "$display"
        else
            local tool
            tool=$(_tool "$lang")
            if has "$tool"; then
                printf "  ${GREEN}%-2s${RESET} %-${col_w}s" "✓" "$display"
            else
                printf "  ${DIM}%-2s %-${col_w}s${RESET}" "✗" "$display"
            fi
        fi
        i=$((i + 1))
        if (( i % cols == 0 )); then
            echo ""
        fi
    done
    if (( i % cols != 0 )); then
        echo ""
    fi

    echo ""
    if [ ${#unavailable[@]} -gt 0 ]; then
        echo -e "${BOLD}${#installed[@]}/${available_total}${RESET} installed (${#unavailable[@]} unavailable on $(uname -m))"
    else
        echo -e "${BOLD}${#installed[@]}/${#ALL_LANGS[@]}${RESET} installed"
    fi
    echo ""
}

_print_quickstart() {
    echo "  just test           Run all test suites in parallel"
    echo "  just test go        Run a single language's tests"
    echo "  just build          Build all compiled languages"
    echo "  just lint           Run all linters"
    echo "  just --list         See all available commands"
    echo ""
}

# Handle CLI flags
if [ $# -gt 0 ]; then
    case "$1" in
        -h|--help)
            _usage
            exit 0
            ;;
        --status)
            _print_intro
            _print_status
            exit 0
            ;;
        --check)
            rc=0
            if [ ${#missing[@]} -gt 0 ]; then
                echo "missing: ${missing[*]}"
                rc=1
            fi
            if [ ${#unavailable[@]} -gt 0 ]; then
                echo "unavailable on $(uname -m): ${unavailable[*]}"
            fi
            exit $rc
            ;;
        --install-all)
            if [ ${#missing[@]} -eq 0 ]; then
                echo "All available languages already installed."
                if [ ${#unavailable[@]} -gt 0 ]; then
                    echo "Unavailable on $(uname -m): ${unavailable[*]}"
                fi
                exit 0
            fi
            echo -e "Installing ${BOLD}${#missing[@]}${RESET} languages: ${missing[*]}"
            echo ""
            just install "${missing[@]}"
            exit 0
            ;;
        --install)
            shift
            if [ $# -eq 0 ]; then
                echo "error: --install requires at least one language" >&2
                exit 1
            fi
            just install "$@"
            exit 0
            ;;
        *)
            echo "error: unknown option '$1'" >&2
            echo "Run with --help for usage." >&2
            exit 1
            ;;
    esac
fi

# --- Interactive mode --------------------------------------------------------

_print_intro
_print_status

# All available languages installed → done
if [ ${#missing[@]} -eq 0 ]; then
    echo -e "${GREEN}You're all set!${RESET} Quick-start commands:"
    echo ""
    _print_quickstart
    exit 0
fi

# --- Arrow-key menu for action choice ----------------------------------------

actions=("Install all missing languages" "Choose which languages to install" "Continue without installing")
n_actions=${#actions[@]}
cursor=0

_draw_menu() {
    if [ "${1:-}" = "redraw" ]; then
        _cursor_up "$n_actions"
    fi
    for (( i=0; i<n_actions; i++ )); do
        if [ "$i" -eq "$cursor" ]; then
            printf "  ${CYAN}▸ %s${RESET}\033[K\n" "${actions[$i]}"
        else
            printf "    %s\033[K\n" "${actions[$i]}"
        fi
    done
}

echo "Some toolchains are missing. What would you like to do?"
echo -e "${DIM}  ↑/↓ navigate  ·  enter select${RESET}"
echo ""

_hide_cursor
_cleanup() { _show_cursor; }
trap _cleanup EXIT

_draw_menu first

while true; do
    IFS= read -rsn1 key
    case "$key" in
        $'\x1b')
            read -rsn2 seq
            case "$seq" in
                '[A') (( cursor > 0 )) && cursor=$((cursor - 1)) ;;
                '[B') (( cursor < n_actions - 1 )) && cursor=$((cursor + 1)) ;;
            esac
            ;;
        '') break ;;  # Enter
    esac
    _draw_menu redraw
done

_show_cursor
trap - EXIT
echo ""

case "$cursor" in
    0)  # Install all
        echo -e "Installing ${BOLD}${#missing[@]}${RESET} languages..."
        echo ""
        just install "${missing[@]}"
        ;;
    1)  # Choose languages
        n=${#missing[@]}
        selected=()
        for (( i=0; i<n; i++ )); do
            selected+=(0)
        done
        cursor=0

        _draw_select() {
            if [ "${1:-}" = "redraw" ]; then
                _cursor_up "$n"
            fi
            for (( i=0; i<n; i++ )); do
                local display
                display=$(_display "${missing[$i]}")
                if [ "$i" -eq "$cursor" ]; then
                    if [ "${selected[$i]}" -eq 1 ]; then
                        printf "  ${CYAN}▸ [✓] %s${RESET}\033[K\n" "$display"
                    else
                        printf "  ${CYAN}▸ [ ] %s${RESET}\033[K\n" "$display"
                    fi
                else
                    if [ "${selected[$i]}" -eq 1 ]; then
                        printf "    ${GREEN}[✓]${RESET} %s\033[K\n" "$display"
                    else
                        printf "    [ ] %s\033[K\n" "$display"
                    fi
                fi
            done
        }

        echo "Select languages to install:"
        echo -e "${DIM}  ↑/↓ navigate  ·  space toggle  ·  a all  ·  n none  ·  enter confirm${RESET}"
        echo ""

        _hide_cursor
        trap _cleanup EXIT

        _draw_select first

        while true; do
            IFS= read -rsn1 key
            case "$key" in
                $'\x1b')
                    read -rsn2 seq
                    case "$seq" in
                        '[A') (( cursor > 0 )) && cursor=$((cursor - 1)) ;;
                        '[B') (( cursor < n - 1 )) && cursor=$((cursor + 1)) ;;
                    esac
                    ;;
                ' ')
                    if [ "${selected[$cursor]}" -eq 1 ]; then
                        selected[$cursor]=0
                    else
                        selected[$cursor]=1
                    fi
                    ;;
                a|A) for (( i=0; i<n; i++ )); do selected[$i]=1; done ;;
                n|N) for (( i=0; i<n; i++ )); do selected[$i]=0; done ;;
                '') break ;;
            esac
            _draw_select redraw
        done

        _show_cursor
        trap - EXIT
        echo ""

        to_install=()
        for (( i=0; i<n; i++ )); do
            if [ "${selected[$i]}" -eq 1 ]; then
                to_install+=("${missing[$i]}")
            fi
        done

        if [ ${#to_install[@]} -eq 0 ]; then
            echo "No languages selected."
        else
            echo -e "Installing ${BOLD}${#to_install[@]}${RESET} languages: ${to_install[*]}"
            echo ""
            just install "${to_install[@]}"
        fi
        ;;
    2)  # Continue without installing
        echo "Quick-start commands (for installed languages):"
        echo ""
        echo "  just test           Run all test suites in parallel"
        echo "  just test go        Run a single language's tests"
        echo "  just install <lang> Install a specific toolchain later"
        echo "  just --list         See all available commands"
        echo ""
        ;;
esac
