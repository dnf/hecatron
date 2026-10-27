// Package spdx reads SPDX license expressions ("MIT", "Apache-2.0 OR
// MIT", "GPL-2.0-only WITH Classpath-exception-2.0") into a canonical form
// and decides whether a set of allowed or denied licenses permits one, for
// hct licenses (docs/licenses.md).
package spdx

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed licenses.txt
var licenseList string

//go:embed exceptions.txt
var exceptionList string

var (
	licenses   = map[string]string{} // lower case to the identifier
	deprecated = map[string]bool{}
	exceptions = map[string]string{}
)

func init() {
	read := func(list string, into map[string]string) {
		for _, line := range strings.Split(list, "\n") {
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			id := strings.TrimPrefix(line, "!")
			into[strings.ToLower(id)] = id
			if id != line {
				deprecated[id] = true
			}
		}
	}
	read(licenseList, licenses)
	read(exceptionList, exceptions)
}

// replacements are deprecated identifiers whose meaning has a current one:
// the GNU licenses without -only or -or-later, and with a +.
var replacements = map[string]string{
	"AGPL-1.0": "AGPL-1.0-only", "AGPL-3.0": "AGPL-3.0-only",
	"GFDL-1.1": "GFDL-1.1-only", "GFDL-1.2": "GFDL-1.2-only", "GFDL-1.3": "GFDL-1.3-only",
	"GPL-1.0": "GPL-1.0-only", "GPL-1.0+": "GPL-1.0-or-later",
	"GPL-2.0": "GPL-2.0-only", "GPL-2.0+": "GPL-2.0-or-later",
	"GPL-3.0": "GPL-3.0-only", "GPL-3.0+": "GPL-3.0-or-later",
	"LGPL-2.0": "LGPL-2.0-only", "LGPL-2.0+": "LGPL-2.0-or-later",
	"LGPL-2.1": "LGPL-2.1-only", "LGPL-2.1+": "LGPL-2.1-or-later",
	"LGPL-3.0": "LGPL-3.0-only", "LGPL-3.0+": "LGPL-3.0-or-later",
}

// aliases are names registries use that aren't SPDX identifiers but name
// exactly one.
var aliases = map[string]string{
	"apache 2.0": "Apache-2.0", "apache-2": "Apache-2.0", "apache2": "Apache-2.0",
	"apache license 2.0": "Apache-2.0", "apache license, version 2.0": "Apache-2.0",
	"the apache software license, version 2.0": "Apache-2.0", "the apache license, version 2.0": "Apache-2.0",
	"mit license": "MIT", "the mit license": "MIT", "expat": "MIT",
	"bsd 3-clause": "BSD-3-Clause", "new bsd license": "BSD-3-Clause", "bsd-3": "BSD-3-Clause",
	"bsd 2-clause": "BSD-2-Clause", "simplified bsd license": "BSD-2-Clause", "bsd-2": "BSD-2-Clause",
	"isc license": "ISC", "mozilla public license 2.0": "MPL-2.0", "mpl 2.0": "MPL-2.0",
	"gplv2": "GPL-2.0-only", "gplv3": "GPL-3.0-only", "lgplv3": "LGPL-3.0-only", "agplv3": "AGPL-3.0-only",
	"unlicense": "Unlicense", "public domain": "", "unknown": "", "non-standard": "", "noassertion": "", "none": "",
}

// Canonical is a license identifier as the SPDX list spells it, with a
// deprecated GNU one replaced by its -only or -or-later form; ok is false
// for what isn't on the list (LicenseRef-... is kept as written).
func Canonical(id string) (string, bool) {
	if strings.HasPrefix(id, "LicenseRef-") || strings.HasPrefix(id, "DocumentRef-") {
		return id, true
	}
	plus := ""
	if found, ok := licenses[strings.ToLower(id)]; ok {
		id = found
	} else if strings.HasSuffix(id, "+") {
		found, ok := licenses[strings.ToLower(strings.TrimSuffix(id, "+"))]
		if !ok {
			return id, false
		}
		id, plus = found, "+"
	} else {
		return id, false
	}
	if replacement, ok := replacements[id+plus]; ok {
		return replacement, true
	}
	return id + plus, true
}

// Expression is a parsed license expression: a license (ID, with its
// exception), or an AND or OR of others.
type Expression struct {
	Op        string // "", "AND" or "OR"
	ID        string // for a license
	Exception string // its WITH exception, if any
	Terms     []*Expression
}

// String is the expression in canonical form: identifiers as SPDX spells
// them, AND binding tighter than OR, parentheses only where needed.
func (e *Expression) String() string { return e.format(false) }

func (e *Expression) format(nested bool) string {
	if e.Op == "" {
		if e.Exception != "" {
			return e.ID + " WITH " + e.Exception
		}
		return e.ID
	}
	parts := make([]string, len(e.Terms))
	for i, term := range e.Terms {
		parts[i] = term.format(e.Op == "AND" && term.Op == "OR")
	}
	text := strings.Join(parts, " "+e.Op+" ")
	if nested {
		return "(" + text + ")"
	}
	return text
}

// IDs are the expression's license identifiers, each once, in order.
func (e *Expression) IDs() []string {
	if e.Op == "" {
		return []string{e.ID}
	}
	var ids []string
	seen := map[string]bool{}
	for _, term := range e.Terms {
		for _, id := range term.IDs() {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids
}

// Parse reads an SPDX license expression, or a name a registry uses for
// one license ("Apache 2.0"). An error for anything else: an identifier
// not on the SPDX list, a placeholder ("UNKNOWN", "non-standard",
// "NOASSERTION"), bad syntax.
func Parse(text string) (*Expression, error) {
	text = strings.TrimSpace(text)
	if alias, ok := aliases[strings.ToLower(text)]; ok {
		if alias == "" {
			return nil, fmt.Errorf("%q names no license", text)
		}
		text = alias
	}
	p := &parser{tokens: tokenize(text)}
	if len(p.tokens) == 0 {
		return nil, fmt.Errorf("no license")
	}
	e, err := p.or()
	if err != nil {
		return nil, err
	}
	if p.pos < len(p.tokens) {
		return nil, fmt.Errorf("unexpected %q in %q", p.tokens[p.pos], text)
	}
	return e, nil
}

func tokenize(text string) []string {
	var tokens []string
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}
	for _, r := range text {
		switch {
		case r == '(' || r == ')':
			flush()
			tokens = append(tokens, string(r))
		case r == ' ' || r == '\t' || r == '\n':
			flush()
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return tokens
}

type parser struct {
	tokens []string
	pos    int
}

func (p *parser) peek() string {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return ""
}

func (p *parser) operator(op string) bool {
	if strings.EqualFold(p.peek(), op) {
		p.pos++
		return true
	}
	return false
}

func (p *parser) or() (*Expression, error) {
	return p.join("OR", p.and)
}

func (p *parser) and() (*Expression, error) {
	return p.join("AND", p.term)
}

// join reads terms separated by op, flattening nested ones of the same op.
func (p *parser) join(op string, next func() (*Expression, error)) (*Expression, error) {
	first, err := next()
	if err != nil {
		return nil, err
	}
	terms := []*Expression{first}
	for p.operator(op) {
		term, err := next()
		if err != nil {
			return nil, err
		}
		terms = append(terms, term)
	}
	if len(terms) == 1 {
		return first, nil
	}
	var flat []*Expression
	for _, term := range terms {
		if term.Op == op {
			flat = append(flat, term.Terms...)
		} else {
			flat = append(flat, term)
		}
	}
	return &Expression{Op: op, Terms: flat}, nil
}

func (p *parser) term() (*Expression, error) {
	token := p.peek()
	switch {
	case token == "":
		return nil, fmt.Errorf("a license expression ends early")
	case token == "(":
		p.pos++
		e, err := p.or()
		if err != nil {
			return nil, err
		}
		if p.peek() != ")" {
			return nil, fmt.Errorf("no closing parenthesis")
		}
		p.pos++
		return e, nil
	case token == ")" || strings.EqualFold(token, "AND") || strings.EqualFold(token, "OR") || strings.EqualFold(token, "WITH"):
		return nil, fmt.Errorf("unexpected %q", token)
	}
	p.pos++
	id, ok := Canonical(token)
	if !ok {
		return nil, fmt.Errorf("%q isn't an SPDX license identifier", token)
	}
	e := &Expression{ID: id}
	if p.operator("WITH") {
		exception, ok := exceptions[strings.ToLower(p.peek())]
		if !ok {
			return nil, fmt.Errorf("%q isn't an SPDX license exception", p.peek())
		}
		p.pos++
		e.Exception = exception
	}
	return e, nil
}

// Policy is which licenses a project accepts: with Allow, only those; never
// those in Deny. Identifiers are canonical (Canonical).
type Policy struct {
	Allow, Deny []string
}

// Permits says whether a package under the expression may be used: an OR
// needs one permitted choice, an AND every part; a license WITH an
// exception is judged by the license.
func (p Policy) Permits(e *Expression) bool {
	switch e.Op {
	case "OR":
		for _, term := range e.Terms {
			if p.Permits(term) {
				return true
			}
		}
		return false
	case "AND":
		for _, term := range e.Terms {
			if !p.Permits(term) {
				return false
			}
		}
		return true
	}
	if contains(p.Deny, e.ID) {
		return false
	}
	return len(p.Allow) == 0 || contains(p.Allow, e.ID)
}

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}
