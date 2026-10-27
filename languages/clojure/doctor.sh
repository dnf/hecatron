#!/usr/bin/env bash
# Check for the JDK the Clojure CLI runs on, and for Leiningen where a
# component builds with project.clj (see install.sh).
source "$(dirname "$0")/../_common.sh"

has java || problem "java not installed (the Clojure CLI runs on the JDK)"
while IFS= read -r _dir; do
    [ -f "${_dir}/project.clj" ] || continue
    has lein || problem "lein not installed (${_dir#"${ROOT}/"} builds with project.clj)"
    break
done < <(impl_dirs clojure)
doctor_done
