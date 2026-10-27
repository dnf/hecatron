#!/usr/bin/env bash
# Shared helpers for hecatron's shell commands (sourced, not executed).
#
# Loads the project model from libexec/lib/project.py: the language lists
# (_ALL_LANGS, _ADDED_LANGS, _COMPILED_LANGS, _CLEAN_LANGS, _LINT_LANGS,
# _TEST_LANGS, _AUDIT_LANGS, _FORMAT_LANGS, _LINT_FORMAT_LANGS,
# _NO_BUILD_LANGS, _DISABLED_LANGS, _TOOLS), the lookups _name, _abbr, _dep,
# _lint_dep and _is_disabled, and HECATRON_ROOT, _COMPONENTS, _IMPL_PATH and
# _LOGS. Requires HECATRON_HOME (set by bin/hecatron).

_HECATRON_LIB="${HECATRON_HOME}/libexec/lib"
_env="$(python3 "${_HECATRON_LIB}/project.py" shell-env)" || exit 1
eval "${_env}"
unset _env
_ROOT="${HECATRON_ROOT}"

# True when a name is a language hecatron knows (built in or project-defined).
_is_known() {
    printf '%s\n' ${_ALL_LANGS} | grep -qx "$1"
}

# True when a name is a project tool (config.json "tools"), e.g. shell.
_is_tool() {
    printf '%s\n' ${_TOOLS} | grep -qx "$1"
}

# A known language the project doesn't implement (not in config.json "languages").
_is_removed() {
    _is_known "$1" || return 1
    printf '%s\n' ${_ADDED_LANGS} | grep -qx "$1" && return 1
    return 0
}

# ---------------------------------------------------------------------------
# Environment setup for non-standard tool locations
# ---------------------------------------------------------------------------

# Put a language's own toolchain directories (config.json "path": ~ expanded,
# globs allowed, existing ones only) at the front of PATH. Only for that
# language's work: Alire's GNAT directory, for one, also has gcc/g++, which
# would shadow the system compilers other languages build with.
_use_lang_path() {
    local pattern dir
    for pattern in $(_lang_path_patterns "$1"); do
        pattern="${pattern/#\~/${HOME}}"
        for dir in ${pattern}; do
            [ -d "${dir}" ] && PATH="${dir}:${PATH}"
        done
    done
    export PATH
}

# Kept for scripts written before "path" existed.
_ada_env() { _use_lang_path ada; }

# True when a language's (or tool's) dependency is installed, counting the
# language's own "path" directories.
_dep_available() {
    local dep
    dep=$(_dep "$1" 2>/dev/null || true)
    [ -z "${dep}" ] && return 0
    ( _use_lang_path "$1"; command -v "${dep}" >/dev/null 2>&1 )
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
# Also refuses languages the project doesn't implement.
# Usage: name=$(_check "$lang" "install")
_check() {
    local lang="$1" help_cmd="${2:-help}" name suggest answer
    name=$(_name "$lang")
    if _is_known "${name}"; then
        if _is_removed "${name}"; then
            echo "Language '${name}' is not added. Run 'hecatron add ${name}' to add it." >&2
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
        echo "Unknown language: $lang (try 'hecatron $help_cmd help')" >&2
    fi
    exit 1
}
