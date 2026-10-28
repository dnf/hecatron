- **Root needed less:** OSV-Scanner, formatters' release builds (StyLua,
  Air, Fourmolu, Ormolu), V, Goose, Elm's tools and bundler-audit now go
  in hecatron's bin directory, `~/.local/share/hecatron/bin`, rather than
  `/usr/local/bin` or `/opt`, so installing them needs no password. `hct`
  puts that directory first on PATH for everything it runs. On macOS only
  Perl's and Dart's installers still ask for administrator rights.
