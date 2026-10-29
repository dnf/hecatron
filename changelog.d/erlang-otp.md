- **A current Erlang/OTP on Ubuntu:** Erlang's installer installs
  builds.hex.pm's OTP 29.1.2 (the default `version` in its definition) into
  `~/.local/share/hecatron/toolchains/erlang`, with its commands and
  rebar3's release (3.27.3, checked against its SHA-256) in
  `~/.local/share/hecatron/bin`, without root, instead of Ubuntu's own
  Erlang: 24.04's OTP 25 is too old for current libraries (hackney) and
  Gleam's standard library, which need OTP 26. Debian, which builds.hex.pm
  has no builds for, still gets its distribution's Erlang and rebar3, and
  macOS Homebrew's. The generated Dockerfile installs the same OTP and
  rebar3. Gleam installs Erlang's OTP when the `erl` on PATH is older than
  26, and its doctor reports one that is; `hct uninstall erlang` removes
  the OTP build and rebar3 it installed (#206).
