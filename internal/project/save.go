package project

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Writing what project.py writes: config.json, local.json, the detection
// cache and .gitignore entries, byte for byte as Python's
// json.dump(data, f, indent=2) and its file writes give them.

// PyJSON is Python's json.dumps(value, indent=2) of a decoded value (nil,
// bool, json.Number, string, []any, *Object, and []string or map-free
// conveniences): ASCII only, non-ASCII escaped as \uXXXX.
func PyJSON(value any) (string, error) {
	var out strings.Builder
	if err := writePyJSON(&out, value, 0); err != nil {
		return "", err
	}
	return out.String(), nil
}

func writePyJSON(out *strings.Builder, value any, depth int) error {
	indent := func(level int) { out.WriteString("\n" + strings.Repeat("  ", level)) }
	switch v := value.(type) {
	case nil:
		out.WriteString("null")
	case bool:
		out.WriteString(strconv.FormatBool(v))
	case json.Number:
		text, err := pyJSONNumber(v)
		if err != nil {
			return err
		}
		out.WriteString(text)
	case int:
		out.WriteString(strconv.Itoa(v))
	case string:
		out.WriteString(pyJSONString(v))
	case []string:
		list := make([]any, len(v))
		for i, s := range v {
			list[i] = s
		}
		return writePyJSON(out, list, depth)
	case []any:
		if len(v) == 0 {
			out.WriteString("[]")
			return nil
		}
		out.WriteString("[")
		for i, item := range v {
			if i > 0 {
				out.WriteString(",")
			}
			indent(depth + 1)
			if err := writePyJSON(out, item, depth+1); err != nil {
				return err
			}
		}
		indent(depth)
		out.WriteString("]")
	case *Object:
		if len(v.Keys) == 0 {
			out.WriteString("{}")
			return nil
		}
		out.WriteString("{")
		for i, key := range v.Keys {
			if i > 0 {
				out.WriteString(",")
			}
			indent(depth + 1)
			out.WriteString(pyJSONString(key) + ": ")
			if err := writePyJSON(out, v.Values[key], depth+1); err != nil {
				return err
			}
		}
		indent(depth)
		out.WriteString("}")
	default:
		return unsupported("writing a %T as JSON", value)
	}
	return nil
}

// pyJSONNumber is how json.dumps writes a number json.loads read: an
// integer as Python's int, anything with a fraction or exponent as its
// float's repr.
func pyJSONNumber(n json.Number) (string, error) {
	text := string(n)
	if !strings.ContainsAny(text, ".eE") {
		if text == "-0" {
			return "0", nil
		}
		return text, nil
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsInf(f, 0) {
		return "", unsupported("the number %s", text) // Python writes Infinity
	}
	return pyFloatRepr(f), nil
}

// pyJSONString is json.dumps' ensure_ascii quoting.
func pyJSONString(s string) string {
	var out strings.Builder
	out.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			out.WriteString(`\"`)
		case r == '\\':
			out.WriteString(`\\`)
		case r == '\n':
			out.WriteString(`\n`)
		case r == '\r':
			out.WriteString(`\r`)
		case r == '\t':
			out.WriteString(`\t`)
		case r == '\b':
			out.WriteString(`\b`)
		case r == '\f':
			out.WriteString(`\f`)
		case r >= ' ' && r <= '~':
			out.WriteRune(r)
		case r > 0xffff:
			r -= 0x10000
			fmt.Fprintf(&out, `\u%04x\u%04x`, 0xd800+(r>>10), 0xdc00+(r&0x3ff))
		default:
			fmt.Fprintf(&out, `\u%04x`, r)
		}
	}
	out.WriteByte('"')
	return out.String()
}

// loneSurrogate is a \uD800-\uDFFF escape: Python keeps a lone one in its
// string and writes it back, where Go's decoder made it U+FFFD.
var loneSurrogate = regexp.MustCompile(`\\u[dD][89a-fA-F][0-9a-fA-F]{2}`)

// writePyJSONFile is project.py's _write_json: the directory made, the
// document, a newline.
func writePyJSONFile(path string, value any) error {
	text, err := PyJSON(value)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text+"\n"), 0o666)
}

// readRewritable reads a JSON file the model will write back, declining
// what it couldn't write back as Python would.
func readRewritable(path string) (any, error) {
	if data, err := os.ReadFile(path); err == nil && loneSurrogate.Match(data) {
		return nil, unsupported("%s holds a \\uD800-\\uDFFF escape", path)
	}
	return readJSON(path)
}

// SaveLanguages is project.py's save_config with "languages" set: written
// sorted and without repeats, the rest of config.json as read. A detected
// project becomes one with its own config, as hecatron init would make it.
func (p *Project) SaveLanguages(languages []string) error {
	if _, err := readRewritable(pyJoin(p.Dir, "config.json")); err != nil {
		return err
	}
	unique := map[string]bool{}
	var sorted []string
	for _, lang := range languages {
		if !unique[lang] {
			unique[lang] = true
			sorted = append(sorted, lang)
		}
	}
	sort.Strings(sorted)
	p.config.set("languages", stringsToAny(sorted))
	if err := writePyJSONFile(pyJoin(p.Dir, "config.json"), p.config); err != nil {
		return err
	}
	p.Languages = sorted
	if p.Detected {
		return IgnoreProjectFiles(p.Root)
	}
	return nil
}

// SaveDisabled is project.py's save_local with "disabled" set, the rest of
// local.json as read.
func (p *Project) SaveDisabled(disabled []string) error {
	if _, err := readRewritable(p.localPath); err != nil {
		return err
	}
	if p.local == nil {
		p.local = &Object{Values: map[string]any{}}
	}
	p.local.set("disabled", stringsToAny(disabled))
	if err := writePyJSONFile(p.localPath, p.local); err != nil {
		return err
	}
	p.Disabled = append([]string(nil), disabled...)
	sort.Strings(p.Disabled)
	return nil
}

func stringsToAny(list []string) []any {
	result := make([]any, len(list))
	for i, s := range list {
		result[i] = s
	}
	return result
}

// IgnoreProjectFiles is project.py's ignore_project_files: add what a
// project writes but doesn't commit to its .gitignore.
func IgnoreProjectFiles(root string) error {
	path := pyJoin(root, ".gitignore")
	existing := ""
	if isFile(path) {
		text, err := readPyTextStrict(path)
		if err != nil {
			return err
		}
		existing = text
	}
	lines := strings.Split(existing, "\n")
	var missing []string
	for _, entry := range []string{".hecatron/local.json", ".logs/"} {
		if !contains(lines, entry) {
			missing = append(missing, entry)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	separator := ""
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		separator = "\n"
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o666)
	if err != nil {
		return err
	}
	_, err = f.WriteString(separator + strings.Join(missing, "\n") + "\n")
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

// ReadPyText is a text file as Python's open(path, encoding="utf-8")
// reads it: universal newlines, and text that isn't UTF-8 declined (Python
// raises UnicodeDecodeError).
func ReadPyText(path string) (string, error) { return readPyTextStrict(path) }

func readPyTextStrict(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", unsupported("%v", err)
	}
	if !utf8.Valid(data) {
		return "", unsupported("%s isn't UTF-8", path)
	}
	return pyText(data), nil
}
