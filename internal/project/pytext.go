package project

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Version resolution reads ecosystem files with Python's str methods and re
// module, whose notions of whitespace, digits and word characters are
// Unicode's. These are those notions, so the Go model reads a file as
// project.py does; where Python would turn a non-ASCII digit into a number,
// the model declines (checkDigits).

// Regular expression classes equal to Python's on str patterns.
const (
	pySpaceClass    = `[\t\n\x0b\x0c\r \x1c-\x1f\x{85}\p{Z}]`
	pyNonSpaceClass = `[^\t\n\x0b\x0c\r \x1c-\x1f\x{85}\p{Z}]`
	pyWordClass     = `[\p{L}\p{N}_]`
	pyDigitClass    = `\p{Nd}`
)

// pyRegexp compiles a pattern written with Python's \s, \S, \w and \d,
// giving them Python's meaning.
func pyRegexp(pattern string) *regexp.Regexp {
	replacer := strings.NewReplacer(`\s`, pySpaceClass, `\S`, pyNonSpaceClass, `\w`, pyWordClass, `\d`, pyDigitClass)
	return regexp.MustCompile(replacer.Replace(pattern))
}

// pyIsSpace is Python's str.isspace for one character.
func pyIsSpace(r rune) bool { return unicode.IsSpace(r) || r >= 0x1c && r <= 0x1f }

// pyIsWord is a character Python's \w matches.
func pyIsWord(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' }

// pyFields is Python's str.split() without arguments.
func pyFields(s string) []string { return strings.FieldsFunc(s, pyIsSpace) }

// pySplitLines is Python's str.splitlines(): lines without their ends, at
// any of the line boundaries Python knows.
func pySplitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch r {
		case '\n', '\r', '\x0b', '\x0c', '\x1c', '\x1d', '\x1e', '\u0085', ' ', ' ':
			lines = append(lines, s[start:i])
			i += size
			if r == '\r' && i < len(s) && s[i] == '\n' {
				i++
			}
			start = i
		default:
			i += size
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// pyText is a file's contents as Python reads it with encoding="utf-8",
// errors="replace": invalid bytes replaced, and line ends made "\n"
// (universal newlines). How many replacement characters a run of invalid
// bytes becomes differs from Python, which no version a format reads can
// contain anyway.
func pyText(data []byte) string {
	text := strings.ToValidUTF8(string(data), "�")
	return strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
}

// checkDigits declines text in which Python would read digits Go doesn't:
// \d and int() take any Unicode decimal digit ("٣" is 3).
func checkDigits(text string) error {
	for _, r := range text {
		if r > unicode.MaxASCII && unicode.IsDigit(r) {
			return unsupported("non-ASCII digits in %.40q", text)
		}
	}
	return nil
}

// pySplit is Python's os.path.split.
func pySplit(path string) (head, tail string) {
	if runtime.GOOS == "windows" {
		head, tail = filepath.Split(path)
		if trimmed := strings.TrimRight(head, `\/`); trimmed != "" && !strings.HasSuffix(trimmed, ":") {
			head = trimmed
		}
		return head, tail
	}
	i := strings.LastIndex(path, "/") + 1
	head, tail = path[:i], path[i:]
	if head != "" && strings.Trim(head, "/") != "" {
		head = strings.TrimRight(head, "/")
	}
	return head, tail
}

// pyDirname is Python's os.path.dirname.
func pyDirname(path string) string {
	head, _ := pySplit(path)
	return head
}

// pyRelpath is Python's os.path.relpath for two absolute paths.
func pyRelpath(path, start string) string {
	if runtime.GOOS == "windows" {
		// With slashes, as git names files: what it gives is a path within
		// the project, written into generated files and compared with git's.
		if rel, err := filepath.Rel(start, path); err == nil {
			return filepath.ToSlash(rel)
		}
		return path
	}
	split := func(p string) []string {
		var parts []string
		for _, part := range strings.Split(pyNormpath(p), "/") {
			if part != "" {
				parts = append(parts, part)
			}
		}
		return parts
	}
	to, from := split(path), split(start)
	common := 0
	for common < len(to) && common < len(from) && to[common] == from[common] {
		common++
	}
	var rel []string
	for range from[common:] {
		rel = append(rel, "..")
	}
	rel = append(rel, to[common:]...)
	if len(rel) == 0 {
		return "."
	}
	return strings.Join(rel, "/")
}

// pyNormpath is Python's os.path.normpath for an absolute POSIX path, which
// keeps exactly two leading slashes.
func pyNormpath(path string) string {
	if runtime.GOOS == "windows" {
		return filepath.Clean(path)
	}
	prefix := ""
	if strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///") {
		prefix = "/"
	}
	return prefix + cleanSlashPath(path)
}

func cleanSlashPath(path string) string {
	var parts []string
	for _, part := range strings.Split(path, "/") {
		switch part {
		case "", ".":
		case "..":
			if len(parts) > 0 && parts[len(parts)-1] != ".." {
				parts = parts[:len(parts)-1]
			} else if !strings.HasPrefix(path, "/") {
				parts = append(parts, part)
			}
		default:
			parts = append(parts, part)
		}
	}
	joined := strings.Join(parts, "/")
	if strings.HasPrefix(path, "/") {
		return "/" + joined
	}
	if joined == "" {
		return "."
	}
	return joined
}

// pathSeparators are the characters that separate a path's parts: "/",
// and on Windows "\\" too.
var pathSeparators = map[bool]string{true: `/\`, false: "/"}[runtime.GOOS == "windows"]

// homeVariable holds the user's home directory, as expanduser reads it:
// HOME, or USERPROFILE on Windows.
var homeVariable = map[bool]string{true: "USERPROFILE", false: "HOME"}[runtime.GOOS == "windows"]

// pyExpandUser is Python's os.path.expanduser with home as $HOME. "~user"
// is declined: Python looks it up in the password database.
func pyExpandUser(path, home string, haveHome bool) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}
	end := strings.IndexAny(path[1:], pathSeparators) + 1
	if end == 0 {
		end = len(path)
	}
	if end != 1 {
		return "", unsupported("~user in %q", path)
	}
	if !haveHome {
		return "", unsupported("no HOME to expand %q", path)
	}
	expanded := strings.TrimRight(home, pathSeparators) + path[1:]
	if expanded == "" {
		return "/", nil
	}
	return expanded, nil
}

// pyGlob is Python's glob.glob(pathname) (not recursive): "*", "?" and
// "[...]" in any part, names starting with "." matched only by a pattern
// part that does too, in the order directories list them.
func pyGlob(pathname string) ([]string, error) {
	if !utf8.ValidString(pathname) {
		return nil, unsupported("a path that isn't UTF-8")
	}
	return pyIGlob(pathname, false)
}

func pyHasMagic(s string) bool { return strings.ContainsAny(s, "*?[") }

func pyIGlob(pathname string, dirOnly bool) ([]string, error) {
	dirname, basename := pySplit(pathname)
	if !pyHasMagic(pathname) {
		if basename != "" {
			if lexists(pathname) {
				return []string{pathname}, nil
			}
		} else if isDir(dirname) {
			return []string{pathname}, nil
		}
		return nil, nil
	}
	if dirname == "" {
		return nil, unsupported("a relative pattern %q", pathname)
	}
	dirs := []string{dirname}
	if dirname != pathname && pyHasMagic(dirname) {
		var err error
		if dirs, err = pyIGlob(dirname, true); err != nil {
			return nil, err
		}
	}
	var found []string
	for _, dir := range dirs {
		var names []string
		if pyHasMagic(basename) {
			var err error
			if names, err = globInDir(dir, basename, dirOnly); err != nil {
				return nil, err
			}
		} else if lexists(pyJoin(dir, basename)) {
			names = []string{basename}
		}
		for _, name := range names {
			found = append(found, pyJoin(dir, name))
		}
	}
	return found, nil
}

// globInDir is glob's _glob1: the names in dir matching pattern.
func globInDir(dir, pattern string, dirOnly bool) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if !utf8.ValidString(name) {
			return nil, unsupported("a file name that isn't UTF-8 in %s", dir)
		}
		if dirOnly && !isDir(pyJoin(dir, name)) {
			continue
		}
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(pattern, ".") {
			continue
		}
		matched, err := fnmatch(name, pattern)
		if err != nil {
			return nil, err
		}
		if matched {
			names = append(names, name)
		}
	}
	return names, nil
}

func lexists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// sortedDirs are the directories among paths, sorted as Python sorts str.
func sortedDirs(paths []string) []string {
	var dirs []string
	for _, path := range paths {
		if isDir(path) {
			dirs = append(dirs, path)
		}
	}
	sort.Strings(dirs)
	return dirs
}

// shellQuote is Python's shlex.quote.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	for i := 0; i < len(s); i++ {
		if !unquoted(s[i]) {
			return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
		}
	}
	return s
}

// unquoted is whether shlex.quote leaves a byte as it is: an ASCII word
// character or one of @%+=:,./-.
func unquoted(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("_@%+=:,./-", c) >= 0
}
