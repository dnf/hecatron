# Builds and checks each language directory, like a multi-language project's root Makefile.
cxx: cxx/add_test

cxx/add_test: cxx/add.cpp cxx/add_test.cpp
	$(CXX) -std=c++17 -o $@ $^

check_cxx: cxx
	./cxx/add_test

clean_cxx:
	rm -f cxx/add_test

.PHONY: cxx check_cxx clean_cxx
