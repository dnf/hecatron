<?php

use Calc\Calc;
use PHPUnit\Framework\TestCase;

final class CalcTest extends TestCase
{
    public function testAdds(): void
    {
        $this->assertSame(5, Calc::add(2, 3));
    }

    public function testAddsZero(): void
    {
        $this->assertSame(0, Calc::add(0, 0));
    }

    public function testRejectsNull(): void
    {
        Calc::add(null, 1);
    }
}
