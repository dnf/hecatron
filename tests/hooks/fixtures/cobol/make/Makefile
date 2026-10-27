COBC ?= cobc
COBFLAGS ?= -free -Wall -fstatic-call

all: libadd.a

libadd.a: add.o
	ar rcs $@ $^

%.o: %.cbl
	$(COBC) $(COBFLAGS) -c -o $@ $<

add_test: add_test.cbl libadd.a
	$(COBC) $(COBFLAGS) -x -o $@ add_test.cbl libadd.a

check: add_test
	./add_test

clean:
	rm -f *.o *.a add_test

.PHONY: all check clean

