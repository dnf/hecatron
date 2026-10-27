package deps

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dnf/hecatron/internal/project"
)

// The tools' reports are read as libexec/lib/audit.py and outdated.py read
// them, with Python's json module and Python's notion of a value: a
// report's odd shapes (a null where a string belongs, a string where a
// list does) give the same findings, or the same error, in both. Values
// decode to nil, bool, Number, string, []any and *Object.

// Object is a JSON object in its keys' order, as Python's dict keeps it: a
// repeated key keeps its first position and its last value.
type Object struct {
	Keys   []string
	Values map[string]any
}

func newObject() *Object { return &Object{Values: map[string]any{}} }

func (o *Object) set(key string, value any) {
	if _, seen := o.Values[key]; !seen {
		o.Keys = append(o.Keys, key)
	}
	o.Values[key] = value
}

// Number is a JSON number as written; Float says Python reads it as a
// float (a fraction, an exponent, NaN or Infinity) rather than an int.
type Number struct {
	Text  string
	Float bool
}

func (n Number) float() float64 {
	switch n.Text {
	case "NaN":
		return math.NaN()
	case "Infinity":
		return math.Inf(1)
	case "-Infinity":
		return math.Inf(-1)
	}
	f, _ := strconv.ParseFloat(n.Text, 64)
	return f
}

// pyError is a Python exception's message, str(error), as audit.py prints
// it in "couldn't read the ... report (...)".
type pyError struct{ message string }

func (e *pyError) Error() string { return e.message }

func errorf(format string, args ...any) error { return &pyError{fmt.Sprintf(format, args...)} }

// jsonDecodeError is json.JSONDecodeError's message: what, and where in
// characters.
func jsonDecodeError(message string, doc []rune, pos int) error {
	line := 1
	lastNewline := -1
	for i := 0; i < pos && i < len(doc); i++ {
		if doc[i] == '\n' {
			line++
			lastNewline = i
		}
	}
	return errorf("%s: line %d column %d (char %d)", message, line, pos-lastNewline, pos)
}

type jsonScanner struct {
	doc []rune
}

func isJSONSpace(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' }

func (s *jsonScanner) skipSpace(i int) int {
	for i < len(s.doc) && isJSONSpace(s.doc[i]) {
		i++
	}
	return i
}

// errNoValue is the C scanner's StopIteration: no value starts at pos.
type errNoValue struct{ pos int }

func (e errNoValue) Error() string { return "no value" }

func (s *jsonScanner) noValue(err error) error {
	if stop, ok := err.(errNoValue); ok {
		return jsonDecodeError("Expecting value", s.doc, stop.pos)
	}
	return err
}

func (s *jsonScanner) hasPrefix(i int, word string) bool {
	runes := []rune(word)
	if i+len(runes) > len(s.doc) {
		return false
	}
	for k, r := range runes {
		if s.doc[i+k] != r {
			return false
		}
	}
	return true
}

// scan is the C scanner's scan_once: the value at i and where it ends.
func (s *jsonScanner) scan(i int) (any, int, error) {
	if i >= len(s.doc) {
		return nil, i, errNoValue{i}
	}
	switch c := s.doc[i]; {
	case c == '"':
		return s.scanString(i + 1)
	case c == '{':
		return s.scanObject(i + 1)
	case c == '[':
		return s.scanArray(i + 1)
	case c == 'n' && s.hasPrefix(i, "null"):
		return nil, i + 4, nil
	case c == 't' && s.hasPrefix(i, "true"):
		return true, i + 4, nil
	case c == 'f' && s.hasPrefix(i, "false"):
		return false, i + 5, nil
	case c == 'N' && s.hasPrefix(i, "NaN"):
		return Number{"NaN", true}, i + 3, nil
	case c == 'I' && s.hasPrefix(i, "Infinity"):
		return Number{"Infinity", true}, i + 8, nil
	case c == '-' && s.hasPrefix(i, "-Infinity"):
		return Number{"-Infinity", true}, i + 9, nil
	}
	return s.scanNumber(i)
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

func (s *jsonScanner) scanNumber(start int) (any, int, error) {
	i := start
	if i < len(s.doc) && s.doc[i] == '-' {
		i++
	}
	switch {
	case i < len(s.doc) && s.doc[i] == '0':
		i++
	case i < len(s.doc) && s.doc[i] >= '1' && s.doc[i] <= '9':
		for i < len(s.doc) && isDigit(s.doc[i]) {
			i++
		}
	default:
		return nil, start, errNoValue{start}
	}
	float := false
	if i+1 < len(s.doc) && s.doc[i] == '.' && isDigit(s.doc[i+1]) {
		i += 2
		for i < len(s.doc) && isDigit(s.doc[i]) {
			i++
		}
		float = true
	}
	if i < len(s.doc) && (s.doc[i] == 'e' || s.doc[i] == 'E') {
		j := i + 1
		if j < len(s.doc) && (s.doc[j] == '+' || s.doc[j] == '-') {
			j++
		}
		if j < len(s.doc) && isDigit(s.doc[j]) {
			for j < len(s.doc) && isDigit(s.doc[j]) {
				j++
			}
			i, float = j, true
		}
	}
	return Number{string(s.doc[start:i]), float}, i, nil
}

func (s *jsonScanner) scanString(i int) (any, int, error) {
	begin := i - 1
	var out strings.Builder
	for {
		if i >= len(s.doc) {
			return nil, i, jsonDecodeError("Unterminated string starting at", s.doc, begin)
		}
		c := s.doc[i]
		switch {
		case c == '"':
			return out.String(), i + 1, nil
		case c < 0x20:
			return nil, i, jsonDecodeError("Invalid control character at", s.doc, i)
		case c != '\\':
			out.WriteRune(c)
			i++
			continue
		}
		i++
		if i >= len(s.doc) {
			return nil, i, jsonDecodeError("Unterminated string starting at", s.doc, begin)
		}
		escapes := map[rune]rune{'"': '"', '\\': '\\', '/': '/', 'b': '\b', 'f': '\f', 'n': '\n', 'r': '\r', 't': '\t'}
		if r, ok := escapes[s.doc[i]]; ok {
			out.WriteRune(r)
			i++
			continue
		}
		if s.doc[i] != 'u' {
			return nil, i, jsonDecodeError("Invalid \\escape", s.doc, i-1)
		}
		r, ok := s.hex4(i + 1)
		if !ok {
			return nil, i, jsonDecodeError("Invalid \\uXXXX escape", s.doc, i)
		}
		i += 5
		if r >= 0xd800 && r <= 0xdbff && i+1 < len(s.doc) && s.doc[i] == '\\' && s.doc[i+1] == 'u' {
			if low, ok := s.hex4(i + 2); ok && low >= 0xdc00 && low <= 0xdfff {
				r = 0x10000 + (r-0xd800)<<10 + (low - 0xdc00)
				i += 6
			}
		}
		if r >= 0xd800 && r <= 0xdfff {
			r = utf8.RuneError // a lone surrogate, which Go strings can't hold
		}
		out.WriteRune(r)
	}
}

func (s *jsonScanner) hex4(i int) (rune, bool) {
	if i+4 > len(s.doc) {
		return 0, false
	}
	value, err := strconv.ParseUint(string(s.doc[i:i+4]), 16, 32)
	if err != nil || strings.ContainsAny(string(s.doc[i:i+4]), "+-_xX") {
		return 0, false
	}
	return rune(value), true
}

func (s *jsonScanner) scanObject(i int) (any, int, error) {
	object := newObject()
	i = s.skipSpace(i)
	if i < len(s.doc) && s.doc[i] == '}' {
		return object, i + 1, nil
	}
	for {
		if i >= len(s.doc) || s.doc[i] != '"' {
			return nil, i, jsonDecodeError("Expecting property name enclosed in double quotes", s.doc, i)
		}
		key, end, err := s.scanString(i + 1)
		if err != nil {
			return nil, end, err
		}
		i = s.skipSpace(end)
		if i >= len(s.doc) || s.doc[i] != ':' {
			return nil, i, jsonDecodeError("Expecting ':' delimiter", s.doc, i)
		}
		i = s.skipSpace(i + 1)
		value, end, err := s.scan(i)
		if err != nil {
			return nil, end, s.noValue(err)
		}
		object.set(key.(string), value)
		i = s.skipSpace(end)
		if i < len(s.doc) && s.doc[i] == '}' {
			return object, i + 1, nil
		}
		if i >= len(s.doc) || s.doc[i] != ',' {
			return nil, i, jsonDecodeError("Expecting ',' delimiter", s.doc, i)
		}
		comma := i
		i = s.skipSpace(i + 1)
		if i < len(s.doc) && s.doc[i] == '}' {
			return nil, i, jsonDecodeError("Illegal trailing comma before end of object", s.doc, comma)
		}
	}
}

func (s *jsonScanner) scanArray(i int) (any, int, error) {
	list := []any{}
	i = s.skipSpace(i)
	if i < len(s.doc) && s.doc[i] == ']' {
		return list, i + 1, nil
	}
	for {
		value, end, err := s.scan(i)
		if err != nil {
			return nil, end, s.noValue(err)
		}
		list = append(list, value)
		i = s.skipSpace(end)
		if i < len(s.doc) && s.doc[i] == ']' {
			return list, i + 1, nil
		}
		if i >= len(s.doc) || s.doc[i] != ',' {
			return nil, i, jsonDecodeError("Expecting ',' delimiter", s.doc, i)
		}
		comma := i
		i = s.skipSpace(i + 1)
		if i < len(s.doc) && s.doc[i] == ']' {
			return nil, i, jsonDecodeError("Illegal trailing comma before end of array", s.doc, comma)
		}
	}
}

// jsonLoads is json.loads.
func jsonLoads(text string) (any, error) {
	s := &jsonScanner{doc: []rune(text)}
	if len(s.doc) > 0 && s.doc[0] == '\ufeff' {
		return nil, jsonDecodeError("Unexpected UTF-8 BOM (decode using utf-8-sig)", s.doc, 0)
	}
	value, end, err := s.scan(s.skipSpace(0))
	if err != nil {
		return nil, s.noValue(err)
	}
	if end = s.skipSpace(end); end != len(s.doc) {
		return nil, jsonDecodeError("Extra data", s.doc, end)
	}
	return value, nil
}

// jsonValues is audit.py's _json_values: each JSON value in a stream of
// them (one a line, or pretty-printed back to back), with Python's
// whitespace between them.
func jsonValues(text string) ([]any, error) {
	s := &jsonScanner{doc: []rune(text)}
	var values []any
	i := 0
	for {
		for i < len(s.doc) && project.PyIsSpace(s.doc[i]) {
			i++
		}
		if i >= len(s.doc) {
			return values, nil
		}
		value, end, err := s.scan(i)
		if err != nil {
			return nil, s.noValue(err)
		}
		values = append(values, value)
		i = end
	}
}

// -- Python's operations on the decoded values -------------------------------

// typeName is a value's Python type name, for Python's error messages.
func typeName(value any) string {
	switch v := value.(type) {
	case nil:
		return "NoneType"
	case bool:
		return "bool"
	case Number:
		if v.Float {
			return "float"
		}
		return "int"
	case string:
		return "str"
	case []any:
		return "list"
	case *Object:
		return "dict"
	}
	return fmt.Sprintf("%T", value)
}

// truthy is Python's bool(value).
func truthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case Number:
		f := v.float()
		return f != 0 || math.IsNaN(f)
	case string:
		return v != ""
	case []any:
		return len(v) > 0
	case *Object:
		return len(v.Keys) > 0
	}
	return true
}

// or is Python's `a or b`.
func or(a, b any) any {
	if truthy(a) {
		return a
	}
	return b
}

// str is Python's str(value).
func str(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return repr(value)
}

// repr is Python's repr(value).
func repr(value any) string {
	switch v := value.(type) {
	case nil:
		return "None"
	case bool:
		if v {
			return "True"
		}
		return "False"
	case Number:
		if !v.Float {
			text := strings.TrimPrefix(v.Text, "-")
			if strings.Trim(text, "0") == "" {
				return "0"
			}
			return v.Text
		}
		f := v.float()
		if math.IsNaN(f) {
			return "nan"
		}
		return project.PyFloatRepr(f)
	case string:
		return project.PyStringRepr(v)
	case []any:
		parts := make([]string, len(v))
		for i, item := range v {
			parts[i] = repr(item)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case *Object:
		parts := make([]string, len(v.Keys))
		for i, key := range v.Keys {
			parts[i] = project.PyStringRepr(key) + ": " + repr(v.Values[key])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return fmt.Sprint(value)
}

// get is Python's value.get(key, default).
func get(value any, key string, def any) (any, error) {
	object, ok := value.(*Object)
	if !ok {
		return nil, errorf("'%s' object has no attribute 'get'", typeName(value))
	}
	if v, present := object.Values[key]; present {
		return v, nil
	}
	return def, nil
}

// item is Python's value[key] with a str key.
func item(value any, key string) (any, error) {
	switch v := value.(type) {
	case *Object:
		if found, present := v.Values[key]; present {
			return found, nil
		}
		return nil, errorf("%s", project.PyStringRepr(key))
	case []any:
		return nil, errorf("list indices must be integers or slices, not str")
	case string:
		return nil, errorf("string indices must be integers, not 'str'")
	}
	return nil, errorf("'%s' object is not subscriptable", typeName(value))
}

// index is Python's value[0]-style subscription with an int.
func index(value any, i int) (any, error) {
	switch v := value.(type) {
	case []any:
		if i < len(v) {
			return v[i], nil
		}
		return nil, errorf("list index out of range")
	case string:
		runes := []rune(v)
		if i < len(runes) {
			return string(runes[i]), nil
		}
		return nil, errorf("string index out of range")
	case *Object:
		return nil, errorf("%d", i)
	}
	return nil, errorf("'%s' object is not subscriptable", typeName(value))
}

// iterate is Python's iteration over a value: a list's items, a dict's
// keys, a str's characters.
func iterate(value any) ([]any, error) {
	switch v := value.(type) {
	case []any:
		return v, nil
	case *Object:
		keys := make([]any, len(v.Keys))
		for i, key := range v.Keys {
			keys[i] = key
		}
		return keys, nil
	case string:
		var chars []any
		for _, r := range v {
			chars = append(chars, string(r))
		}
		return chars, nil
	}
	return nil, errorf("'%s' object is not iterable", typeName(value))
}

// items is Python's dict.items().
func items(value any) (*Object, error) {
	object, ok := value.(*Object)
	if !ok {
		return nil, errorf("'%s' object has no attribute 'items'", typeName(value))
	}
	return object, nil
}

// values is Python's dict.values().
func values(value any) ([]any, error) {
	object, ok := value.(*Object)
	if !ok {
		return nil, errorf("'%s' object has no attribute 'values'", typeName(value))
	}
	list := make([]any, len(object.Keys))
	for i, key := range object.Keys {
		list[i] = object.Values[key]
	}
	return list, nil
}

// text is a value given where Python wants a str (a regex's subject, after
// `value or ""`): its own text, else Python's TypeError.
func text(value any) (string, error) {
	if !truthy(value) {
		return "", nil
	}
	if s, ok := value.(string); ok {
		return s, nil
	}
	return "", errorf("expected string or bytes-like object, got '%s'", typeName(value))
}

// hashKey is a value as a dict key or set member, for the hashable values
// Python allows: equal numbers are one key (1 == 1.0 == True), and lists
// and dicts are unhashable.
func hashKey(value any) (string, error) {
	switch v := value.(type) {
	case []any, *Object:
		return "", errorf("unhashable type: '%s'", typeName(v))
	case string:
		return "s" + v, nil
	case bool:
		if v {
			return "n1", nil
		}
		return "n0", nil
	case Number:
		f := v.float()
		if f == math.Trunc(f) && !math.IsInf(f, 0) {
			return "n" + strconv.FormatFloat(f, 'f', -1, 64), nil
		}
		return "n" + strconv.FormatFloat(f, 'g', -1, 64), nil
	case nil:
		return "None", nil
	}
	return fmt.Sprint(value), nil
}

// joinStrings is Python's ", ".join(values): every item a str.
func joinStrings(separator string, list []any) (string, error) {
	parts := make([]string, len(list))
	for i, item := range list {
		s, ok := item.(string)
		if !ok {
			return "", errorf("sequence item %d: expected str instance, %s found", i, typeName(item))
		}
		parts[i] = s
	}
	return strings.Join(parts, separator), nil
}

// sortedValues is Python's sorted() of str or number values; others, or
// a mix, fail as Python's comparison does (which pair it names can differ).
func sortedValues(list []any) ([]any, error) {
	sorted := append([]any(nil), list...)
	var sortErr error
	sort.SliceStable(sorted, func(i, j int) bool {
		less, err := pyLess(sorted[i], sorted[j])
		if err != nil && sortErr == nil {
			sortErr = err
		}
		return less
	})
	return sorted, sortErr
}
