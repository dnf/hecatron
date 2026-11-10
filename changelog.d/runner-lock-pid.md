- **A long run keeps its lock:** a second run of the same command took the
  lock of any run going for over two minutes, as stale, and ran beside it;
  the lock now holds the run's pid, and is stale only once that process has
  exited.
