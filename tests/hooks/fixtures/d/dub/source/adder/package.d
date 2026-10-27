module adder;

/// The sum of two integers.
int add(int a, int b) @safe pure nothrow @nogc
{
    return a + b;
}

@safe unittest
{
    assert(add(2, 3) == 5);
}
