#include <stdio.h>

static int add(int a, int b) {
    int unused;
    return a + b + missing;
}

int main(void) {
    const char *total = add(2, 3);
    printf("%d\n", total);
    return 0;
}
