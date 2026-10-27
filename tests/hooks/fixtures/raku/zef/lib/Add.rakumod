unit module Add;

#| Add two numbers.
sub add(Int $a, Int $b --> Int) is export {
    $a + $b
}
