#!/usr/bin/env bash
# Check which .NET SDK each component runs, against what hecatron resolves:
# 1. b's workflow says dotnet-version 9.0.x, which says what to install, not
#    what runs (setup-dotnet's SDKs sit beside the runner's, and dotnet runs
#    the newest): 9.0 is installed, and a and b both run the newest SDK.
# 2. The project's own config says version 9.0: both run exactly that SDK,
#    from its view (languages/csharp/install.sh), seeing no other.
# Installs SDKs, so run it in a throwaway container:
#   tests/docker/run.sh ubuntu:24.04 bash /hecatron/tests/docker/dotnet-sdks.sh
set -euo pipefail

_project="$(mktemp -d)"
cd "${_project}"
git init -q
mkdir -p a b/.github/workflows .hecatron/languages/csharp
cat > b/.github/workflows/test.yml <<'YML'
jobs:
  test:
    steps:
      - uses: actions/setup-dotnet@v4
        with:
          dotnet-version: 9.0.x
YML
cat > .hecatron/config.json <<'JSON'
{"schema": 1, "components": ["a", "b"], "implPath": "{component}", "languages": ["csharp"]}
JSON
cat > .hecatron/languages/csharp/test.sh <<'SH'
#!/usr/bin/env bash
echo "$(basename "$(pwd)") $(dotnet --version) $(dotnet --list-sdks | wc -l)" >> "${HECATRON_ROOT}/ran"
SH
chmod +x .hecatron/languages/csharp/test.sh
git add -A && git -c user.email=t@example.com -c user.name=t commit -qm fixture

_failed=0
_check() {  # _check <component> <SDK it should run> <how many SDKs it should see>
    local ran sdks
    read -r _ ran sdks < <(grep "^$1 " ran)
    if [ "${ran}" = "$2" ] && [ "${sdks}" = "$3" ]; then
        echo "PASS $1: runs SDK ${ran}, seeing ${sdks}"
    else
        echo "FAIL $1: should run $2 seeing $3, ran ${ran} seeing ${sdks}"
        _failed=1
    fi
}
_installed() { bash /hecatron/languages/csharp/installed.sh | cut -f1 | grep "^$1" | sort -V | tail -1; }

echo "== 1. a workflow's dotnet-version: installed, the newest runs"
hct install csharp
newest="$(bash /hecatron/languages/csharp/installed.sh | cut -f1 | sort -V | tail -1)"
nine="$(_installed 9.0.)"
[ -n "${nine}" ] && echo "PASS 9.0 is installed (${nine})" || { echo "FAIL no 9.0 SDK installed"; _failed=1; }
rm -f ran && hct test csharp >/dev/null
_check a "${newest}" 1
_check b "${newest}" 1
hct doctor csharp | tee doctor.txt
grep -q "b: csharp 9.0 installed for b/.github/workflows/test.yml; runs the newest, ${newest}" doctor.txt &&
    echo "PASS doctor says what runs" || { echo "FAIL doctor's note"; _failed=1; }

echo "== 2. the project's version: exactly that SDK runs"
echo '{"version": "9.0"}' > .hecatron/languages/csharp/config.json
rm -f ran && hct test csharp >/dev/null
_check a "${nine}" 1
_check b "${nine}" 1
exit "${_failed}"
