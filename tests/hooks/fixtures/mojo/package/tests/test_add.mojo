from testing import TestSuite, assert_equal

from adder import add


def test_add():
    assert_equal(add(2, 3), 5)


def main():
    TestSuite.discover_tests[__functions_in_module()]().run()
