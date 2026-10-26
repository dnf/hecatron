#!/usr/bin/env bash
# The JDKs installed on this machine, as "version<TAB>home" (home is the
# JAVA_HOME, with bin/java): every JDK directory whose release file names its
# version. Offline.
source "$(dirname "$0")/../_common.sh"

_homes() {
    local home
    for home in /usr/lib/jvm/* \
                /Library/Java/JavaVirtualMachines/*/Contents/Home \
                /opt/homebrew/opt/openjdk*/libexec/openjdk.jdk/Contents/Home \
                /usr/local/opt/openjdk*/libexec/openjdk.jdk/Contents/Home; do
        [ -f "${home}/release" ] && [ -x "${home}/bin/javac" ] && echo "${home}"
    done
    return 0
}

_homes | python3 -c '
import os, re, sys
seen = set()
for home in sys.stdin.read().split("\n"):
    if not home:
        continue
    home = os.path.realpath(home)
    match = re.search(r"^JAVA_VERSION=\"([^\"]+)\"", open(os.path.join(home, "release")).read(), re.M)
    if match and home not in seen:
        seen.add(home)
        print(match.group(1).split("+")[0] + "\t" + home)
'
