#!/usr/bin/env bash
# Check each component's Elixir: that it runs on its OTP (install.sh pairs
# each version with one it supports), saying which, and that its Hex, which
# mix needs to fetch dependencies, loads.
source "$(dirname "$0")/../_common.sh"

[ -n "${HECATRON_HOME:-}" ] || doctor_done
_project() { (cd "${ROOT}" && python3 "${HECATRON_HOME}/libexec/lib/project.py" "$@" </dev/null); }
# "version<TAB>OTP version<TAB>OTP root" of the elixir on PATH.
_pairing() {
    elixir -e 'root = :code.root_dir()
release = System.otp_release()
otp = case File.read(Path.join([root, "releases", release, "OTP_VERSION"])) do
  {:ok, text} -> String.trim(text)
  _ -> release
end
IO.puts(Enum.join([System.version(), otp, root], "\t"))' </dev/null 2>/dev/null
}

# Each component and its directory; without components, the language's
# own version, with no directory.
_targets() {
    local found
    found="$(_project impl-dirs elixir)"
    if [ -n "${found}" ]; then echo "${found}"; else printf '\t\n'; fi
}
_env() { if [ -n "$1" ]; then _project env elixir "$1"; else _project env elixir; fi; }

_seen=""
while IFS=$'\t' read -r _component _dir; do
    _label="${_component:+${_component}: }"
    _found="$(eval "$(_env "${_component}")" && has elixir && _pairing)" || _found=""
    if [ -z "${_found}" ]; then
        has elixir && problem "${_label}elixir doesn't start (its OTP may be missing or unsupported; hecatron doctor --fix elixir)"
        continue
    fi
    IFS=$'\t' read -r _version _otp _root <<<"${_found}"
    # Once per pairing: components on the same Elixir share it.
    if ! grep -qxF "${_found}" <<<"${_seen}"; then
        _seen="${_seen}${_found}"$'\n'
        note "${_label}Elixir ${_version} on OTP ${_otp} (${_root})"
    fi
    if [ -n "${_dir}" ] && [ -f "${_dir}/mix.exs" ] &&
        ! (eval "$(_env "${_component}")" && cd "${_dir}" && mix hex.info </dev/null >/dev/null 2>&1); then
        problem "${_label}Elixir ${_version}'s Hex is not installed or doesn't load"
    fi
done < <(_targets)
doctor_done
