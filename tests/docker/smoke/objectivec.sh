# Smoke test for objectivec: build and run a small Foundation program in a
# scratch directory, with GNUstep's flags on Linux and Foundation.framework
# on macOS.
# shellcheck shell=bash
set -euo pipefail
cat > hello.m <<'EOF'
#import <Foundation/Foundation.h>

int main(void) {
    NSAutoreleasePool *pool = [[NSAutoreleasePool alloc] init];
    NSString *greeting = [NSString stringWithFormat:@"hello from %@", @"Foundation"];
    NSLog(@"%@", greeting);
    printf("%s\n", [greeting UTF8String]);
    [pool drain];
    return 0;
}
EOF
if [ "$(uname -s)" = "Darwin" ]; then
    clang -Wall -Werror -o hello hello.m -framework Foundation
else
    # shellcheck disable=SC2046 # gnustep-config prints several flags, split on purpose
    gcc $(gnustep-config --objc-flags) -Werror -o hello hello.m $(gnustep-config --base-libs)
fi
./hello | grep -qx "hello from Foundation"
