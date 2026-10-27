package project

// Python's path and string behaviour, for the packages that write what
// hecatron's Python writes (internal/generate).

// PyJoin is Python's os.path.join, which doesn't clean the result.
func PyJoin(base string, parts ...string) string { return pyJoin(base, parts...) }

// PyRelpath is Python's os.path.relpath for two absolute paths.
func PyRelpath(path, start string) string { return pyRelpath(path, start) }

// PyStrip is Python's str.strip().
func PyStrip(s string) string { return pyStrip(s) }

// PyFields is Python's str.split() without arguments.
func PyFields(s string) []string { return pyFields(s) }
