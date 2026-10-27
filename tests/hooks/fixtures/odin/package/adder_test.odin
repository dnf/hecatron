package adder

import "core:testing"

@(test)
test_add :: proc(t: ^testing.T) {
	testing.expect_value(t, add(2, 3), 5)
}
