package project

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Object is a JSON object that keeps its keys in order, as Python's dict
// does: the order of a project's "tasks" is the order they are listed and
// run in. A repeated key keeps its first position and its last value, also
// as Python's json module does.
type Object struct {
	Keys   []string
	Values map[string]any
}

// Get returns a key's value and whether it is present.
func (o *Object) Get(key string) (any, bool) {
	if o == nil {
		return nil, false
	}
	value, ok := o.Values[key]
	return value, ok
}

func (o *Object) set(key string, value any) {
	if _, seen := o.Values[key]; !seen {
		o.Keys = append(o.Keys, key)
	}
	o.Values[key] = value
}

// decodeJSON parses a document into nil, bool, json.Number, string, []any
// and *Object. A document Python's json module might read differently (it
// takes NaN and Infinity, and rejects invalid UTF-8 with a traceback rather
// than an error) is unsupported, so the caller leaves it to project.py.
func decodeJSON(data []byte) (any, error) {
	if !utf8.Valid(data) {
		return nil, unsupported("invalid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	value, err := decodeValue(decoder)
	if err != nil {
		return nil, unsupported("%v", err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, unsupported("extra data after the JSON value")
	}
	return value, nil
}

func decodeValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch token {
	case json.Delim('{'):
		object := &Object{Values: map[string]any{}}
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			value, err := decodeValue(decoder)
			if err != nil {
				return nil, err
			}
			object.set(key.(string), value)
		}
		_, err := decoder.Token()
		return object, err
	case json.Delim('['):
		list := []any{}
		for decoder.More() {
			value, err := decodeValue(decoder)
			if err != nil {
				return nil, err
			}
			list = append(list, value)
		}
		_, err := decoder.Token()
		return list, err
	}
	return token, nil
}

// truthy is Python's truth value of a decoded JSON value: config.json keys
// such as "components" fall back to their default when empty or false.
func truthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		return v != ""
	case json.Number:
		f, err := strconv.ParseFloat(string(v), 64)
		return err != nil || f != 0
	case []any:
		return len(v) > 0
	case *Object:
		return len(v.Keys) > 0
	}
	return true
}

// pyRepr is Python's repr() of a decoded scalar, for messages project.py
// writes with !r. Lists and objects are unsupported: their repr depends on
// formatting rules not worth reproducing for a message.
func pyRepr(value any) (string, error) {
	switch v := value.(type) {
	case nil:
		return "None", nil
	case bool:
		if v {
			return "True", nil
		}
		return "False", nil
	case json.Number:
		return pyNumber(v)
	case string:
		return pyStringRepr(v), nil
	}
	return "", unsupported("repr of a JSON list or object")
}

// pyNumber is Python's str() of a JSON number: an integer as written, a
// number with a fraction or exponent as a float's shortest repr.
func pyNumber(n json.Number) (string, error) {
	text := string(n)
	if !strings.ContainsAny(text, ".eE") {
		if text == "-0" {
			return "0", nil
		}
		return text, nil
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return "", unsupported("%v", err)
	}
	return pyFloatRepr(f), nil
}

// pyFloatRepr follows Python's float repr: the shortest digits that round
// trip, positional for exponents from -4 to 15, else scientific with at
// least two exponent digits.
func pyFloatRepr(f float64) string {
	switch {
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	}
	sign := ""
	if math.Signbit(f) {
		sign, f = "-", -f
	}
	scientific := strconv.FormatFloat(f, 'e', -1, 64) // d.ddde±XX
	mantissa, exponentText, _ := strings.Cut(scientific, "e")
	exponent, _ := strconv.Atoi(exponentText)
	digits := strings.Replace(mantissa, ".", "", 1)
	if f == 0 {
		return sign + "0.0"
	}
	if exponent < -4 || exponent >= 16 {
		fraction := ""
		if len(digits) > 1 {
			fraction = "." + digits[1:]
		}
		expSign := "+"
		if exponent < 0 {
			expSign, exponent = "-", -exponent
		}
		return fmt.Sprintf("%s%s%se%s%02d", sign, digits[:1], fraction, expSign, exponent)
	}
	if exponent < 0 {
		return sign + "0." + strings.Repeat("0", -exponent-1) + digits
	}
	if len(digits) <= exponent+1 {
		return sign + digits + strings.Repeat("0", exponent+1-len(digits)) + ".0"
	}
	return sign + digits[:exponent+1] + "." + digits[exponent+1:]
}

// pyStringRepr is Python's repr() of a str: single quotes unless the text
// has a single quote and no double quote, escapes for the quote, backslash
// and unprintable characters.
func pyStringRepr(s string) string {
	quote := '\''
	if strings.ContainsRune(s, '\'') && !strings.ContainsRune(s, '"') {
		quote = '"'
	}
	var out strings.Builder
	out.WriteRune(quote)
	for _, r := range s {
		switch {
		case r == quote || r == '\\':
			out.WriteRune('\\')
			out.WriteRune(r)
		case r == '\n':
			out.WriteString(`\n`)
		case r == '\r':
			out.WriteString(`\r`)
		case r == '\t':
			out.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&out, `\x%02x`, r)
		case r < 0x7f || unicode.IsPrint(r):
			out.WriteRune(r)
		case r <= 0xff:
			fmt.Fprintf(&out, `\x%02x`, r)
		case r <= 0xffff:
			fmt.Fprintf(&out, `\u%04x`, r)
		default:
			fmt.Fprintf(&out, `\U%08x`, r)
		}
	}
	out.WriteRune(quote)
	return out.String()
}

// DecodeJSON parses a JSON document as the model reads config files: into
// nil, bool, json.Number, string, []any and *Object.
func DecodeJSON(data []byte) (any, error) { return decodeJSON(data) }
