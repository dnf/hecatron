# What the Crystal hooks read from shard.yml (sourced by them, after
# _hooks.sh).
# shellcheck shell=bash

# The shard's dependencies, frozen to shard.lock when there is one.
shards_fetch() {
    if [ -f shard.lock ]; then run shards install --frozen; else run shards install; fi
}
# When it has some and they aren't installed in lib/.
shards_deps() {
    grep -Eq '^(development_)?dependencies:' shard.yml || return 0
    shards check >/dev/null 2>&1 || shards_fetch
}

shard_name() { sed -n 's/^name:[[:space:]]*//p' shard.yml | head -1; }

# The executables `shards build` makes (into bin/), one per line: the keys
# one level under `targets:`.
shard_targets() {
    awk '
        /^targets:/ { inside = 1; next }
        /^[^[:space:]#]/ { inside = 0 }
        inside && /^[[:space:]]+[^[:space:]#][^:]*:/ {
            match($0, /^[[:space:]]+/)
            if (indent == 0) indent = RLENGTH
            if (RLENGTH == indent) { key = substr($0, RLENGTH + 1); sub(/:.*/, "", key); print key }
        }
    ' shard.yml
}
