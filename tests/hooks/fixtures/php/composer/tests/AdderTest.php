<?php

declare(strict_types=1);

namespace Example\Add\Tests;

use Example\Add\Adder;
use PHPUnit\Framework\TestCase;

final class AdderTest extends TestCase
{
    public function testAddsTwoNumbers(): void
    {
        $this->assertSame(5, Adder::add(2, 3));
    }
}
