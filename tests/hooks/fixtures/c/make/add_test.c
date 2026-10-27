#include <stdio.h>
#include "add.h"

int main(void) {
    if (add(2, 3) != 5) {
        fprintf(stderr, "2 + 3 != 5\n");
        return 1;
    }
    return 0;
}
