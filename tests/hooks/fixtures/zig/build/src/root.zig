const std = @import("std");

/// The sum of two integers.
pub fn add(a: i32, b: i32) i32 {
    return a + b;
}

test "add" {
    try std.testing.expectEqual(@as(i32, 5), add(2, 3));
}
