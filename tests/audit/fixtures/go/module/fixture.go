// Package auditfixture calls a function with a known vulnerability, so
// govulncheck reports it.
package auditfixture

import "golang.org/x/text/language"

// Tag parses a BCP 47 language tag.
func Tag(s string) (language.Tag, error) {
	return language.Parse(s)
}
