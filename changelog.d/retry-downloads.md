- **Downloads through a tool's own client are retried:** Go's toolchain
  downloads and `go install`, `rustup`, `uv`, `opam`, `cpanm`, `npm -g`,
  `mix local.rebar` and `raco pkg install` in the installers are tried up
  to five times, 3 to 24 s apart (`retry` in `languages/_common.sh`), as
  curl's downloads already were; a dropped Go toolchain download
  ("stream error") no longer fails `hct install`. The binary's release
  downloads now also retry a transfer that breaks partway, not only a
  failed connection or a 5xx answer.
