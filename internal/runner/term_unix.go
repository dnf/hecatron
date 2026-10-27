//go:build unix

package runner

import (
	"os"
	"syscall"
	"unsafe"
)

// TerminalSize is a terminal's columns and rows; ok is false when f isn't a
// terminal.
func TerminalSize(f *os.File) (columns, rows int, ok bool) {
	var size struct{ rows, columns, x, y uint16 }
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&size)))
	if errno != 0 {
		return 0, 0, false
	}
	return int(size.columns), int(size.rows), true
}

// IsTerminal reports whether f is a terminal.
func IsTerminal(f *os.File) bool {
	_, _, ok := TerminalSize(f)
	return ok
}
