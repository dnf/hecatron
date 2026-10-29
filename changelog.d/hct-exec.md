- **`hct exec LANG [-c COMPONENT] -- CMD...`** runs a command with a
  language's environment for a component, what `hct env LANG -c COMPONENT`
  prints, without an `eval` first: useful where components use different
  versions of one toolchain. Without `-c`, the component whose directory
  holds the working directory, else the only one in the language. CMD runs
  in the working directory and its exit status is hct's; a version not
  installed is reported with the `hct install` to run (#225). Needs the
  binary.
