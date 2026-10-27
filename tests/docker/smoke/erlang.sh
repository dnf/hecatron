# Smoke test for erlang: build a rebar3 application in a scratch directory,
# run a function from it, and run Dialyzer on it as the default lint hook does.
# shellcheck shell=bash
set -euo pipefail
mkdir -p hello/src && cd hello
printf '{erl_opts, [debug_info]}.\n' > rebar.config
cat > src/hello.app.src <<'EOF'
{application, hello, [
    {description, "smoke test"},
    {vsn, "0.1.0"},
    {applications, [kernel, stdlib]},
    {modules, []}
]}.
EOF
cat > src/hello.erl <<'EOF'
-module(hello).
-export([answer/0]).

-spec answer() -> integer().
answer() -> lists:sum(lists:seq(1, 9)).
EOF
rebar3 compile >/dev/null
_out="$(erl -noshell -pa _build/default/lib/hello/ebin -eval 'io:format("~p~n", [hello:answer()]), halt().')"
[ "${_out}" = "45" ] || { echo "expected 45, got: ${_out}" >&2; exit 1; }
echo "${_out}"
rebar3 dialyzer
