#import <Foundation/Foundation.h>
#import "Adder.h"

// An explicit pool rather than @autoreleasepool, which GCC's Objective-C
// (what GNUstep is built with on Debian and Ubuntu) lacks.
int main(void) {
    NSAutoreleasePool *pool = [[NSAutoreleasePool alloc] init];
    NSInteger sum = [Adder add:2 to:3];
    int status = 0;
    if (sum != 5) {
        NSLog(@"2 + 3 = %ld, not 5", (long)sum);
        status = 1;
    }
    [pool drain];
    return status;
}
