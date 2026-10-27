# Smoke test for clojure: run a hello world with the Clojure CLI in a scratch
# directory, and lint it with clj-kondo, which the default lint hook fetches
# through the CLI at the version languages/_hooks.sh pins.
# shellcheck shell=bash
set -euo pipefail
_hooks="$(dirname "${BASH_SOURCE[0]}")/../../../languages/_hooks.sh"
_kondo_version="$(sed -n 's/^_CLJ_KONDO_VERSION="\(.*\)"$/\1/p' "${_hooks}")"
[ -n "${_kondo_version}" ] || { echo "no _CLJ_KONDO_VERSION in ${_hooks}" >&2; exit 1; }
mkdir -p src/hello
printf '{:paths ["src"]}\n' > deps.edn
cat > src/hello/core.clj <<'EOF'
(ns hello.core)

(defn -main [& _args]
  (println (reduce + (range 1 10))))
EOF
_out="$(clojure -M -m hello.core)"
[ "${_out}" = "45" ] || { echo "expected 45, got: ${_out}" >&2; exit 1; }
echo "${_out}"
clojure -Sdeps "{:deps {clj-kondo/clj-kondo {:mvn/version \"${_kondo_version}\"}}}" \
    -M -m clj-kondo.main --lint src
