use strict;
use warnings;

use Module::Build;

Module::Build->new(
    module_name => 'Add',
    dist_abstract => 'Adds two numbers',
    license     => 'perl',
)->create_build_script;
