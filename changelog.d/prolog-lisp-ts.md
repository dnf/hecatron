- **Current SWI-Prolog on Ubuntu:** `hct install prolog` adds SWI-Prolog's
  PPA (`ppa:swi-prolog/stable`, its key checked against its fingerprint)
  and installs or upgrades SWI-Prolog from it, so packs that need 9.2 or
  later (lsp_server) work; Debian and macOS keep their own. `hct doctor`
  reports the PPA missing, and `hct uninstall prolog` removes it. An
  `install.json` can name such a repository for any language
  (`aptSources`).
- **Quicklisp for Common Lisp:** `hct install lisp` installs Quicklisp into
  `~/.local/share/hecatron/quicklisp` (no root; its quickstart pinned by
  checksum), and the Common Lisp hooks fetch the dependencies of the
  systems they build and test (their test systems' too) with it, so a
  project such as cl-ppcre, whose tests need cl-unicode, tests without
  setup. `hct doctor` reports Quicklisp missing; `hct uninstall lisp`
  removes it. A Quicklisp the user's init file loads is used instead.
- **JavaScript or TypeScript below:** without a `package.json` in a
  component's directory, the Node.js hooks pick the packages below as
  detection does, so a mostly-JavaScript package with a `tsconfig.json` is
  JavaScript's, not TypeScript's.
