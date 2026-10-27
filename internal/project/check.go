package project

import (
	"fmt"
	"strings"
	"unicode"
)

// Severities are the audit severities, most severe first.
var Severities = []string{"critical", "high", "moderate", "low"}

// ConfigProblems is what's wrong with config.json's "audit" section, for
// hecatron check: audit.py's config_problems, message for message.
func (p *Project) ConfigProblems() ([]string, error) {
	audit, _ := p.config.Get("audit")
	if audit == nil {
		return nil, nil
	}
	object, ok := audit.(*Object)
	if !ok {
		return []string{`"audit" must be an object`}, nil
	}
	var problems []string
	ignore, _ := object.Get("ignore")
	var entries []any
	if truthy(ignore) {
		if entries, ok = ignore.([]any); !ok {
			return nil, unsupported("audit.ignore is not a list")
		}
	}
	for index, raw := range entries {
		entry, isObject := raw.(*Object)
		id, _ := entry.Get("id")
		if !isObject || !truthy(id) {
			problems = append(problems, fmt.Sprintf(`audit.ignore[%d] needs an "id"`, index))
			continue
		}
		idText, err := pyStr(id)
		if err != nil {
			return nil, err
		}
		where := "audit.ignore " + idText
		if reason, ok := entry.Get("reason"); !ok || isBlankString(reason) {
			problems = append(problems, where+` needs a "reason"`)
		}
		if until, _ := entry.Get("until"); !isISODate(until) {
			problems = append(problems, where+` needs an "until" date (YYYY-MM-DD)`)
		}
	}
	if failOn, ok := object.Get("failOn"); ok && failOn != nil && !isSeverity(failOn) {
		problems = append(problems, "audit.failOn must be one of "+strings.Join(Severities, ", "))
	}
	return problems, nil
}

// isBlankString is Python's `not str(value).strip()`: only a string can be
// blank; str() of anything else ("None", "0", "[]") never is.
func isBlankString(value any) bool {
	s, ok := value.(string)
	return ok && pyStrip(s) == ""
}

// isISODate is Python's re.match(r"^\d{4}-\d{2}-\d{2}$", str(value)): \d is
// any Unicode digit, and $ also matches before a final newline.
func isISODate(value any) bool {
	s, ok := value.(string)
	if !ok {
		return false
	}
	runes := []rune(strings.TrimSuffix(s, "\n"))
	if len(runes) != 10 {
		return false
	}
	for i, r := range runes {
		if i == 4 || i == 7 {
			if r != '-' {
				return false
			}
		} else if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func isSeverity(value any) bool {
	s, ok := value.(string)
	if !ok {
		return false
	}
	for _, severity := range Severities {
		if s == severity {
			return true
		}
	}
	return false
}
