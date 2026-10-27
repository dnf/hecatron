package project

import (
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
)

// pyJoin is Python's os.path.join, which, unlike filepath.Join, doesn't
// clean the result: a "logs" of "../logs" stays as written in
// HECATRON_LOGS, as project.py prints it.
func pyJoin(base string, parts ...string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(append([]string{base}, parts...)...)
	}
	path := base
	for _, part := range parts {
		switch {
		case strings.HasPrefix(part, "/"):
			path = part
		case path == "" || strings.HasSuffix(path, "/"):
			path += part
		default:
			path += "/" + part
		}
	}
	return path
}

// pyBasename is Python's os.path.basename: "" for "/".
func pyBasename(path string) string {
	if runtime.GOOS == "windows" {
		return filepath.Base(path)
	}
	return path[strings.LastIndex(path, "/")+1:]
}

var (
	fnmatchCache   = map[string]*regexp.Regexp{}
	fnmatchCacheMu sync.Mutex
)

// fnmatch is Python's fnmatch.fnmatch on POSIX: "*" matches any text,
// slashes and leading dots included; "[!...]" negates a set; a "[" with no
// closing "]" is literal. A set Go's regexp rejects (a reversed range) is
// unsupported.
func fnmatch(name, pattern string) (bool, error) {
	fnmatchCacheMu.Lock()
	defer fnmatchCacheMu.Unlock()
	re, ok := fnmatchCache[pattern]
	if !ok {
		var err error
		if re, err = regexp.Compile(translateGlob(pattern)); err != nil {
			return false, unsupported("pattern %q: %v", pattern, err)
		}
		fnmatchCache[pattern] = re
	}
	return re.MatchString(name), nil
}

func translateGlob(pattern string) string {
	var out strings.Builder
	out.WriteString(`(?s)\A`)
	runes := []rune(pattern)
	for i := 0; i < len(runes); i++ {
		switch c := runes[i]; c {
		case '*':
			out.WriteString(".*")
		case '?':
			out.WriteString(".")
		case '[':
			end := i + 1
			if end < len(runes) && runes[end] == '!' {
				end++
			}
			if end < len(runes) && runes[end] == ']' {
				end++
			}
			for end < len(runes) && runes[end] != ']' {
				end++
			}
			if end >= len(runes) {
				out.WriteString(`\[`)
				continue
			}
			set := string(runes[i+1 : end])
			i = end
			negate := strings.HasPrefix(set, "!")
			set = strings.TrimPrefix(set, "!")
			set = strings.ReplaceAll(set, `\`, `\\`)
			if strings.HasPrefix(set, "^") {
				set = `\` + set
			}
			if negate {
				out.WriteString("[^" + set + "]")
			} else {
				out.WriteString("[" + set + "]")
			}
		default:
			out.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	out.WriteString(`\z`)
	return out.String()
}
