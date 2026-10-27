use strict;
use warnings;

use Test::More tests => 2;

use Add qw(add);

# Set by the Makefile's test target; a hook that bypasses it fails here.
ok( $ENV{AUTHOR_TESTING}, 'run through the Makefile' );
is( add( 2, 3 ), 5, 'adds two numbers' );
