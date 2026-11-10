- **One run of a command at a time, on Ubuntu too:** two runs of the same
  command started together could both get past the lock where `mkdir`
  isn't atomic (Ubuntu's uutils coreutils); the lock is now a file created
  exclusively.
