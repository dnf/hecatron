- **Default hooks find more layouts:** PHP's test and deps hooks use the
  nearest `composer.json` above (a root one covering `lang/php`, running
  its PHPUnit on the directory's `phpunit.xml`); Dart's test hook tests the
  packages below with a `test/` (a pub workspace's root, `pkgs/*`); R's
  tests each package below with `tests/testthat/` (`R-package/`), and its
  deps hook installs a `DESCRIPTION`'s missing dependencies from CRAN
  (`hct install r` now fetches them); Ruby compiles a gem's native
  extension (`rake compile`) before testing; Racket links sibling packages
  (each with its `info.rkt`) before `raco test`; Elm runs
  `elm-explorations/test` 1.x on the last elm-test supporting it, with
  Elm 0.19.1.

- **Installs:** PHP installs the extensions a `composer.json` or its
  dependencies require (`ext-curl`, `ext-mongodb`...) where packages
  exist; opam installs the system packages dependencies need; Tcl installs
  `make` for TEA extensions' tests.
