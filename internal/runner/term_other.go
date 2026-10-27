//go:build !unix

package runner

import "os"

// TerminalSize is a terminal's columns and rows; off Unix the size isn't
// read, and a character device is taken for an 80x24 terminal.
func TerminalSize(f *os.File) (columns, rows int, ok bool) {
	if !IsTerminal(f) {
		return 0, 0, false
	}
	return 80, 24, true
}

// IsTerminal reports whether f is a terminal (a character device).
func IsTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
