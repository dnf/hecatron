package project

import (
	"regexp"
	"strings"
)

// A pinned hecatron (docs/design.md, "A project pins its hecatron"):
// config.json's "hecatron" holds a version, "0.6" (any 0.6.x) or "0.6.1",
// where ">=0.6.0" sets only a minimum. Only the Go core reads the pin;
// project.py reads either form as a minimum.

var pinPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+){0,2}$`)

// PinProblem is hecatron check's message for a "hecatron" setting that is
// neither a pin nor a minimum.
const PinProblem = `"hecatron" must pin a version ("0.6", or "0.6.1" exactly) or set a minimum (">=0.6.0")`

// HecatronPin is the version config.json's "hecatron" pins, "" when it
// pins none (no setting, or a minimum), and the problem with a setting
// that is neither.
func (p *Project) HecatronPin() (pin, problem string) {
	value, _ := p.config.Get("hecatron")
	if value == nil {
		return "", ""
	}
	text, ok := value.(string)
	if !ok {
		return "", PinProblem
	}
	text = pyStrip(text)
	switch {
	case text == "", strings.HasPrefix(text, ">="):
		return "", ""
	case pinPattern.MatchString(text):
		return text, ""
	}
	return "", PinProblem
}

// PinMatches is whether version is one pin allows: the pin itself, or a
// version it's a prefix of on a dot boundary ("0.6" takes 0.6.2, not
// 0.60.0).
func PinMatches(pin, version string) bool {
	return version == pin || strings.HasPrefix(version, pin+".")
}

// ExactPin is whether pin names one release (X.Y.Z) rather than a series.
func ExactPin(pin string) bool { return strings.Count(pin, ".") == 2 }
