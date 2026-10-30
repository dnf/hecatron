// Package xcode reads Xcode's project files without Xcode: a project's
// project.pbxproj (its targets, build configurations and the .xcconfig files
// they're based on), its shared schemes, a workspace's projects, and
// .xcconfig files. It reads structure, not every build setting: settings
// that depend on a configuration's conditions or on Xcode's defaults are
// left to xcodebuild, which on a Mac stays the authority for a build. See
// docs/contributing/design.md §21.
package xcode

import (
	"fmt"
	"strconv"
	"strings"
)

// parsePlist reads a property list in the old-style (OpenStep) text format
// project.pbxproj is written in: dictionaries { key = value; }, arrays
// ( a, b, ), strings quoted or bare, and comments. A value is a string, a
// map[string]any or a []any; <hex> data is kept as its text.
func parsePlist(text string) (any, error) {
	p := &plistParser{text: text}
	value, err := p.value()
	if err != nil {
		return nil, err
	}
	p.skip()
	if p.pos < len(p.text) {
		return nil, p.errorf("unexpected %q after the top-level value", p.text[p.pos])
	}
	return value, nil
}

type plistParser struct {
	text string
	pos  int
}

func (p *plistParser) errorf(format string, args ...any) error {
	line := 1 + strings.Count(p.text[:p.pos], "\n")
	return fmt.Errorf("line %d: %s", line, fmt.Sprintf(format, args...))
}

// skip passes whitespace and comments.
func (p *plistParser) skip() {
	for p.pos < len(p.text) {
		switch {
		case strings.ContainsRune(" \t\r\n", rune(p.text[p.pos])):
			p.pos++
		case strings.HasPrefix(p.text[p.pos:], "//"):
			end := strings.IndexByte(p.text[p.pos:], '\n')
			if end < 0 {
				p.pos = len(p.text)
			} else {
				p.pos += end + 1
			}
		case strings.HasPrefix(p.text[p.pos:], "/*"):
			end := strings.Index(p.text[p.pos+2:], "*/")
			if end < 0 {
				p.pos = len(p.text)
			} else {
				p.pos += end + 4
			}
		default:
			return
		}
	}
}

func (p *plistParser) value() (any, error) {
	p.skip()
	if p.pos >= len(p.text) {
		return nil, p.errorf("unexpected end of file")
	}
	switch p.text[p.pos] {
	case '{':
		return p.dict()
	case '(':
		return p.array()
	case '"', '\'':
		return p.quoted()
	case '<':
		end := strings.IndexByte(p.text[p.pos:], '>')
		if end < 0 {
			return nil, p.errorf("unterminated data")
		}
		data := p.text[p.pos : p.pos+end+1]
		p.pos += end + 1
		return data, nil
	}
	return p.bare()
}

func (p *plistParser) dict() (map[string]any, error) {
	p.pos++ // {
	dict := map[string]any{}
	for {
		p.skip()
		if p.pos < len(p.text) && p.text[p.pos] == '}' {
			p.pos++
			return dict, nil
		}
		key, err := p.value()
		if err != nil {
			return nil, err
		}
		name, ok := key.(string)
		if !ok {
			return nil, p.errorf("a dictionary key must be a string")
		}
		p.skip()
		if p.pos >= len(p.text) || p.text[p.pos] != '=' {
			return nil, p.errorf("expected = after %q", name)
		}
		p.pos++
		value, err := p.value()
		if err != nil {
			return nil, err
		}
		dict[name] = value
		p.skip()
		if p.pos >= len(p.text) || p.text[p.pos] != ';' {
			return nil, p.errorf("expected ; after the value of %q", name)
		}
		p.pos++
	}
}

func (p *plistParser) array() ([]any, error) {
	p.pos++ // (
	array := []any{}
	for {
		p.skip()
		if p.pos < len(p.text) && p.text[p.pos] == ')' {
			p.pos++
			return array, nil
		}
		value, err := p.value()
		if err != nil {
			return nil, err
		}
		array = append(array, value)
		p.skip()
		if p.pos < len(p.text) && p.text[p.pos] == ',' {
			p.pos++
		} else if p.pos >= len(p.text) || p.text[p.pos] != ')' {
			return nil, p.errorf("expected , or ) in an array")
		}
	}
}

// isBare is a character an unquoted string may hold.
func isBare(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("_$+/:.-", c) >= 0
}

func (p *plistParser) bare() (string, error) {
	start := p.pos
	for p.pos < len(p.text) && isBare(p.text[p.pos]) {
		p.pos++
	}
	if p.pos == start {
		return "", p.errorf("unexpected %q", p.text[p.pos])
	}
	return p.text[start:p.pos], nil
}

// quoted reads a quoted string with its backslash escapes: \n, \t, \", \\,
// \Uxxxx and octal \ooo, as Xcode writes them.
func (p *plistParser) quoted() (string, error) {
	start, quote := p.pos, p.text[p.pos]
	p.pos++
	var b strings.Builder
	for p.pos < len(p.text) {
		c := p.text[p.pos]
		switch {
		case c == quote:
			p.pos++
			return b.String(), nil
		case c != '\\':
			b.WriteByte(c)
			p.pos++
			continue
		}
		p.pos++ // the backslash
		if p.pos >= len(p.text) {
			break
		}
		e := p.text[p.pos]
		p.pos++
		switch e {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'a':
			b.WriteByte('\a')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		case 'U':
			if p.pos+4 > len(p.text) {
				return "", p.errorf("short \\U escape")
			}
			code, err := strconv.ParseUint(p.text[p.pos:p.pos+4], 16, 32)
			if err != nil {
				return "", p.errorf("bad \\U escape")
			}
			b.WriteRune(rune(code))
			p.pos += 4
		case '0', '1', '2', '3', '4', '5', '6', '7':
			end := p.pos - 1
			for end < len(p.text) && end < p.pos+2 && p.text[end] >= '0' && p.text[end] <= '7' {
				end++
			}
			code, _ := strconv.ParseUint(p.text[p.pos-1:end], 8, 8)
			b.WriteByte(byte(code))
			p.pos = end
		default:
			b.WriteByte(e)
		}
	}
	p.pos = start
	return "", p.errorf("unterminated string")
}
