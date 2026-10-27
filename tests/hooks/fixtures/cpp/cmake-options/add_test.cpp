#include <iostream>
#include "add.hpp"

int main() {
    if (add(2, 3) != 5) {
        std::cerr << "2 + 3 != 5\n";
        return 1;
    }
    return 0;
}
