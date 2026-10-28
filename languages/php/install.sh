#!/usr/bin/env bash
# Install PHP + Composer: the default version and every version the
# project's components need, side by side, from the channels that package
# every supported PHP line (distributions carry one: Ubuntu 24.04 has 8.3).
# Those channels carry each line's newest release, so a line is what gets
# installed. One Composer serves every version: it runs on the php on PATH.
source "$(dirname "$0")/../_common.sh"

# A directory per line holding bin/php, so a component's PATH can choose one
# (apt names them php8.4, php8.5..., and /usr/bin/php is whichever one
# update-alternatives picked). Homebrew's kegs already are such directories.
_shim_root="/opt/php"

install_macos() {
    brew tap shivammathur/php
    has composer || brew_install composer
    _install_php "=$(default_version)"
}
install_linux() {
    _install_php "=$(default_version)"
    if ! has composer; then
        curl -sS https://getcomposer.org/installer |
            $SUDO "${_shim_root}/$(_line "=$(default_version)")/bin/php" -- --install-dir=/usr/local/bin --filename=composer
    fi
}

# The line (8.4) a spec names; a bare major takes its newest line.
_line() {
    local spec="${1#=}"
    case "${spec}" in
        *.*) echo "${spec}" | cut -d. -f1-2 ;;
        *) bash "$(dirname "$0")/releases.sh" | cut -f1 | newest_match "${spec}" | cut -d. -f1-2 ;;
    esac
}

# Ondřej Surý's packages of every PHP line: a PPA for Ubuntu, his own
# repository for Debian.
_add_php_source() {
    local id codename uri key
    id="$(. /etc/os-release && echo "${ID}")"
    codename="$(. /etc/os-release && echo "${VERSION_CODENAME}")"
    if [ "${id}" = "ubuntu" ]; then
        uri="https://ppa.launchpadcontent.net/ondrej/php/ubuntu"
        key="/etc/apt/keyrings/ondrej-php.asc"
        log "Adding Ondřej Surý's PHP PPA for ${codename}..."
        curl -fsSL "https://keyserver.ubuntu.com/pks/lookup?op=get&search=0x14AA40EC0831756756D7F66C4F4EA0AAE5267A6C" |
            $SUDO install -D -m 644 /dev/stdin "${key}"
    else
        uri="https://packages.sury.org/php"
        key="/etc/apt/keyrings/sury-php.gpg"
        log "Adding packages.sury.org's PHP repository for ${codename}..."
        curl -fsSL https://packages.sury.org/php/apt.gpg | $SUDO install -D -m 644 /dev/stdin "${key}"
    fi
    printf 'Types: deb\nURIs: %s\nSuites: %s\nComponents: main\nSigned-By: %s\n' "${uri}" "${codename}" "${key}" |
        $SUDO tee /etc/apt/sources.list.d/php.sources >/dev/null
    $SUDO apt-get -o Acquire::Retries=3 update -qq
}
_php_source_added() { [ -f /etc/apt/sources.list.d/php.sources ]; }

_install_php() {
    local line
    line="$(_line "$1")"
    if [ -z "${line}" ]; then
        echo "No PHP release matches ${1#=}" >&2
        return 1
    fi
    if [ "${OS}" = "macos" ]; then
        brew_install "shivammathur/php/php@${line}"
        return
    fi
    if ! apt_has "php${line}-cli" && ! _php_source_added; then
        _add_php_source
    fi
    if ! apt_has "php${line}-cli"; then
        echo "No package of PHP ${line} for this distribution (php${line}-cli)" >&2
        return 1
    fi
    apt_install "php${line}-cli" "php${line}-xml" "php${line}-mbstring"
    $SUDO mkdir -p "${_shim_root}/${line}/bin"
    $SUDO ln -sfn "/usr/bin/php${line}" "${_shim_root}/${line}/bin/php"
}

# The PHP extensions (curl, mongodb...) the components' composer.json files
# require, with require-dev's (the tests need them), one per line: each
# component's own composer.json, else the nearest above it in the project
# (Avro's and Thrift's root composer.json covers lang/php). Composer refuses
# to install without them. A dependency's (Monolog's mongodb/mongodb needs
# ext-mongodb) are the ones a dry run of the install names.
_required_extensions() {
    local _dir
    while IFS= read -r _dir; do
        while [ ! -f "${_dir}/composer.json" ] && [ "${_dir}" != "${ROOT}" ] && [ "${_dir}" != / ]; do
            _dir="$(dirname "${_dir}")"
        done
        [ -f "${_dir}/composer.json" ] || continue
        python3 -c '
import json, sys
try:
    with open(sys.argv[1], encoding="utf-8") as f:
        document = json.load(f)
except (OSError, ValueError):
    sys.exit()
for key in ("require", "require-dev"):
    section = document.get(key) if isinstance(document, dict) else None
    for name in section if isinstance(section, dict) else ():
        if name.lower().startswith("ext-"):
            print(name[4:].lower())
' "${_dir}/composer.json"
        if has composer; then
            (cd "${_dir}" && composer install --dry-run --no-interaction --no-plugins --no-scripts 2>&1 </dev/null) |
                grep -oE 'ext-[A-Za-z0-9_]+' | cut -c5- | tr '[:upper:]' '[:lower:]' || true
        fi
    done < <(impl_dirs php) | sort -u
}

# The apt package of a PHP line's extension (php8.5-mysql has pdo_mysql).
_extension_package() {
    local line="$1" ext="$2"
    case "${ext}" in
        pdo_mysql | mysqli | mysqlnd) ext=mysql ;;
        pdo_pgsql) ext=pgsql ;;
        pdo_sqlite) ext=sqlite3 ;;
    esac
    echo "php${line}-${ext}"
}

# The extensions the project requires that a line lacks, from its packages
# where they exist: one that isn't packaged stays the project's to provide.
# Again while that adds some: Composer's dry run names one dependency's
# missing extensions at a time.
_install_extensions() {
    local line="$1" ext package loaded missing round
    for round in 1 2 3 4 5; do
        loaded="$("/usr/bin/php${line}" -m 2>/dev/null | tr '[:upper:]' '[:lower:]')"
        missing=()
        while IFS= read -r ext; do
            [ -n "${ext}" ] || continue
            grep -qx "${ext}" <<<"${loaded}" && continue
            package="$(_extension_package "${line}" "${ext}")"
            if apt_has "${package}"; then missing+=("${package}"); fi
        done < <(_required_extensions)
        [ "${#missing[@]}" -gt 0 ] || return 0
        log "PHP ${line}: installing extensions the project requires (round ${round}): ${missing[*]}"
        apt_install "${missing[@]}"
    done
}

_install_versions() {
    local specs spec
    specs="$(missing_versions php)"
    while IFS= read -r spec; do
        [ -n "${spec}" ] || continue
        _install_php "${spec}"
    done <<<"${specs}"
}

# Only libraries with a composer.json have dependencies to install; each with
# its own PHP, since Composer resolves for the PHP it runs on.
deps() {
    _install_versions
    if [ "${OS}" != "macos" ]; then
        local php
        for php in "${_shim_root}"/*/bin/php; do
            [ -e "${php}" ] || continue
            _install_extensions "$(basename "$(dirname "$(dirname "${php}")")")"
        done
    fi
    impl_deps php
}

log "Installing PHP..."
"install_${OS}"
deps
# OSV-Scanner, which hecatron licenses reads this language's licenses with.
osv_scanner_install
