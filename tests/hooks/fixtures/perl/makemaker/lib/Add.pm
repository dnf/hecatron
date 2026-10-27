package Add;

use strict;
use warnings;

use Exporter 'import';

our $VERSION   = '0.1.0';
our @EXPORT_OK = qw(add);

sub add {
    my ( $first, $second ) = @_;
    return $first + $second;
}

1;
