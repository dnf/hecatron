CFLAGS ?= -Wall -Wextra -O2

all: libadd.a

libadd.a: add.o
	ar rcs $@ $^

add_test: add_test.o libadd.a
	$(CC) $(CFLAGS) -o $@ add_test.o libadd.a

check: add_test
	./add_test

clean:
	rm -f *.o *.a add_test

.PHONY: all check clean
