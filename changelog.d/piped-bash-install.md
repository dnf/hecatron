- **`curl … | sh` on macOS** says how to put `hct` on PATH in the current
  shell, instead of "Ready: try 'hct help'": macOS's `sh` is bash, and
  install.sh took bash reading it from a pipe for being sourced.
