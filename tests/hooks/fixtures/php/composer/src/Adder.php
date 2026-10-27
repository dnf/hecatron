<?php

declare(strict_types=1);

namespace Example\Add;

final class Adder
{
    public static function add(int $a, int $b): int
    {
        return $a + $b;
    }
}
