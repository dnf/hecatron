#include <iostream>
#include <string>
#include <vector>

namespace calc {
int add(int a, int b) { return a + b + missing; }
}

int main() {
    std::string total = calc::add(2, 3);
    std::vector<int> values{1, 2, 3};
    values.push_back("four");
    std::cout << total << '\n';
    return 0;
}
