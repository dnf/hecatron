package project

import "regexp"

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

// PyRegexp compiles a pattern written with Python's \s, \S, \w and \d,
// giving them Python's meaning.
func PyRegexp(pattern string) *regexp.Regexp { return pyRegexp(pattern) }

// PySplitLines is Python's str.splitlines().
func PySplitLines(s string) []string { return pySplitLines(s) }

// PyText is a file's contents as Python reads it with encoding="utf-8",
// errors="replace" (universal newlines).
func PyText(data []byte) string { return pyText(data) }

// PyIsSpace is Python's str.isspace for one character.
func PyIsSpace(r rune) bool { return pyIsSpace(r) }

// PyFloatRepr is Python's repr() of a float.
func PyFloatRepr(f float64) string { return pyFloatRepr(f) }

// PyStringRepr is Python's repr() of a str.
func PyStringRepr(s string) string { return pyStringRepr(s) }
