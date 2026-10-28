- **One password prompt, up front:** `hct install` and `hct doctor --fix`
  name the languages whose install may need administrator rights and say
  why, then ask for the password once (`sudo -v`) before any installer
  runs, keeping it valid until they finish, instead of sudo prompting
  halfway through. Nothing is asked as root, in CI or off a terminal.
