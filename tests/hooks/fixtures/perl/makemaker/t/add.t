use strict;
use warnings;

use Test::More tests => 1;

use Add qw(add);

is( add( 2, 3 ), 5, 'adds two numbers' );
