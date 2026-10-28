- **A project pins its hecatron:** `"hecatron": "0.6"` in
  `.hecatron/config.json` takes any 0.6.x, `"0.6.1"` only that release
  (`">=0.6.0"` is still a minimum). Another version warns on stderr and
  fails `hct check` (a dev build only warns), and the generated GitLab
  pipeline installs the pinned version (#167).
- **install.sh takes a series:** `HECATRON_VERSION=v0.6` installs the latest
  0.6.x release (#167).
