#!/usr/bin/env bash
# Shared helpers for justfile recipes.
# Sourced (not executed) — provides shared functions and language lists.
#
# Language data (lists, abbreviations, dependencies, disabled set) is loaded
# from per-language config files via .hecatron/tools/language/emit-config.py.

# ---------------------------------------------------------------------------
# Load language configuration
# ---------------------------------------------------------------------------

_HELPERS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
_ROOT="$(cd "${_HELPERS_DIR}/../../.." && pwd)"
eval "$(python3 "${_HELPERS_DIR}/emit-config.py")"

# ---------------------------------------------------------------------------
# Removed languages (not in .hecatron/config/languages/added.json)
# ---------------------------------------------------------------------------

_is_removed() {
    local lang="$1" known=false added
    # Only check known languages — tools (cli, codegen) are never "removed"
    for added in ${_ALL_LANGS}; do
        [ "${added}" = "${lang}" ] && known=true && break
    done
    ${known} || return 1
    for added in ${_ADDED_LANGS}; do
        [ "${added}" = "${lang}" ] && return 1
    done
    return 0
}

# ---------------------------------------------------------------------------
# Environment setup for non-standard tool locations
# ---------------------------------------------------------------------------

# Add Alire-managed GNAT + GPRbuild to PATH (macOS installs via Alire).
# No-op on Linux (apt puts them on PATH) or if already found.
_ada_env() {
    command -v gprbuild >/dev/null 2>&1 && return
    local dir
    for dir in "${HOME}/.local/share/alire/toolchains"/*/bin; do
        [ -d "${dir}" ] && export PATH="${dir}:${PATH}"
    done
}

# True when a language's (or tool's) dependency is installed. Ada's GNAT may
# live only in Alire's toolchain directory, which _ada_env adds to PATH for
# Ada recipes alone: that directory also has gcc/g++, so putting it on PATH
# everywhere would shadow the system compilers other languages build with.
_dep_available() {
    local dep
    dep=$(_dep "$1" 2>/dev/null || true)
    [ -z "${dep}" ] && return 0
    command -v "${dep}" >/dev/null 2>&1 && return 0
    [ "$1" = "ada" ] && compgen -G "${HOME}/.local/share/alire/toolchains/*/bin/${dep}" >/dev/null
}

# ---------------------------------------------------------------------------
# Formatting helpers
# ---------------------------------------------------------------------------

# Format a one-per-line language list into a multi-column grid with abbreviations.
# Usage: _lang_grid "Languages" "$_ALL_LANGS"
_lang_grid() {
    local label="$1"
    local list="$2"
    local items=() lang abbr
    while read -r lang; do
        [ -z "$lang" ] && continue
        abbr=$(_abbr "$lang")
        if [ -n "$abbr" ]; then
            items+=("$lang ($abbr)")
        else
            items+=("$lang")
        fi
    done <<< "$list"
    local count=${#items[@]}
    echo "$label ($count):"
    local cols=5 col_w=16 i=0
    for item in "${items[@]}"; do
        if (( i % cols == 0 )); then printf "  "; fi
        printf "%-${col_w}s" "$item"
        i=$((i + 1))
        if (( i % cols == 0 )); then printf '\n'; fi
    done
    if (( i % cols != 0 )); then printf '\n'; fi
}

# Format a one-per-line language list as a comma-separated string.
# Usage: _lang_csv "$_NO_BUILD_LANGS"
_lang_csv() {
    local list="$1" result="" lang
    while read -r lang; do
        [ -z "$lang" ] && continue
        if [ -n "$result" ]; then
            result="$result, $lang"
        else
            result="$lang"
        fi
    done <<< "$list"
    echo "$result"
}

# ---------------------------------------------------------------------------
# Misspelling suggestions (one entry per line for merge safety)
# ---------------------------------------------------------------------------

_suggest() {
    local input; input="$(echo "$1" | tr '[:upper:]' '[:lower:]')"
    case "$input" in
        adda|ade)
            echo ada ;;
        darrt|drt)
            echo dart ;;
        elixer|elixr|exlir|exilir)
            echo elixir ;;
        fortan|frotran|fortarn|fotran)
            echo fortran ;;
        groovie|grovy|grovvy)
            echo groovy ;;
        haskel|haskle|haslkell)
            echo haskell ;;
        cobbol|coball|cbl|cobel)
            echo cobol ;;
        closure|clojur|clojre|cojure)
            echo clojure ;;
        erland|erlng|earland|earlang)
            echo erlang ;;
        f-sharp|f-\#)
            echo fsharp ;;
        gleema|gleem|glam|gelam)
            echo gleam ;;
        typscript|tyepscript|typesript|tsc)
            echo typescript ;;
        golang|goland)
            echo go ;;
        scalar|scalla)
            echo scala ;;
        kotlint|koltin|kotln)
            echo kotlin ;;
        pyton|pyhton|pythn)
            echo python ;;
        phpp)
            echo php ;;
        rubby)
            echo ruby ;;
        swft|siwft)
            echo swift ;;
        ocmal|ocamal|ocmla)
            echo ocaml ;;
        *)
            if [ ${#input} -ge 3 ]; then
                for name in $_ALL_LANGS; do
                    if [[ "$name" == "$input"* || "$name" == *"$input" ]]; then
                        echo "$name"; return
                    fi
                done
            fi ;;
    esac
}

# Validate a language name (abbreviation or full), with fuzzy suggestion on miss.
# Also refuses removed languages (not in added.json).
# Usage: name=$(_check "$lang" "install")
_check() {
    local lang="$1" help_cmd="${2:-help}" name suggest
    name=$(_name "$lang")
    if [ -d ".hecatron/languages/${name}" ]; then
        if _is_removed "${name}"; then
            echo "Language '${name}' is not added. Run 'just add ${name}' to add it." >&2
            exit 1
        fi
        echo "$name"
        return
    fi
    suggest=$(_suggest "$lang")
    if [ -n "$suggest" ]; then
        if [ -t 0 ]; then
            read -rp "Unknown language '$lang'. Did you mean '$suggest'? [Y/n] " answer </dev/tty
            case "$(echo "$answer" | tr '[:upper:]' '[:lower:]')" in
                ""|y|yes) echo "$suggest"; return ;;
            esac
        else
            echo "Unknown language '$lang'. Did you mean '$suggest'?" >&2
        fi
    else
        echo "Unknown language: $lang (try 'just $help_cmd help')" >&2
    fi
    exit 1
}
