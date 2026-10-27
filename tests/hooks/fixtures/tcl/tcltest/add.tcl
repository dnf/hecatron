package provide add 0.1

namespace eval ::add {
    namespace export add
}

# Add two numbers.
proc ::add::add {a b} {
    expr {$a + $b}
}
