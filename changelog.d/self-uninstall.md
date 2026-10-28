- **`hct self uninstall`** removes hecatron: `~/.hecatron`, its lines in
  your shell's startup files and fish's conf.d, and its Agent Skill. It
  asks first (`--yes` skips the question). The toolchains it installed
  stay; `--all` also removes those it unpacked (`~/.local/share/hecatron`)
  and its download cache (`~/.cache/hecatron`). Run from a checkout, it
  keeps the checkout.
