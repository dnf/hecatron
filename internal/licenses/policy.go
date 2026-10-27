package licenses

import (
	"fmt"
	"strings"

	"github.com/dnf/hecatron/internal/project"
	"github.com/dnf/hecatron/internal/spdx"
)

// Verdict is the project's policy applied to the packages.
type Verdict struct {
	Allow      []string    `json:"allow"`
	Deny       []string    `json:"deny"`
	IncludeDev bool        `json:"includeDev"`
	Violations []Violation `json:"violations"`
	Accepted   []Violation `json:"accepted"` // packages licenses.ignore accepts
	Expired    []string    `json:"expired"`  // notes on ignore entries past their date
}

// Violation is a package the policy doesn't permit, and why (or, accepted,
// the entry's reason and date).
type Violation struct {
	Package   string `json:"package"` // ecosystem:name
	Version   string `json:"version"`
	License   string `json:"license"`
	Reason    string `json:"reason"`
	Language  string `json:"language"`
	Component string `json:"component"`
	Until     string `json:"until,omitempty"`
}

// Apply checks the packages against a policy (nothing without one): a
// package fails under a denied license, or with an allow list under any
// other or an unknown one; dev and test dependencies count only with
// includeDev. An ignore entry accepts a package until its date (today,
// YYYY-MM-DD, included); after it, the package is checked again and the
// entry noted as expired.
func (r *Report) Apply(policy project.LicensePolicy, today string) {
	if !policy.Set {
		return
	}
	verdict := &Verdict{Allow: nonNil(policy.Allow), Deny: nonNil(policy.Deny), IncludeDev: policy.IncludeDev,
		Violations: []Violation{}, Accepted: []Violation{}, Expired: []string{}}
	rules := spdx.Policy{Allow: policy.Allow, Deny: policy.Deny}
	expiredNoted := map[string]bool{}
	for _, p := range r.Packages {
		if p.Dev && !policy.IncludeDev {
			continue
		}
		reason := violation(p, rules)
		if reason == "" {
			continue
		}
		v := Violation{Package: p.Key(), Version: p.Version, License: p.License, Reason: reason,
			Language: p.Language, Component: p.Component}
		if entry, ok := ignoreEntry(policy.Ignore, p); ok {
			if entry.Until >= today {
				v.Reason, v.Until = entry.Reason, entry.Until
				verdict.Accepted = append(verdict.Accepted, v)
				continue
			}
			if !expiredNoted[entry.Package] {
				expiredNoted[entry.Package] = true
				verdict.Expired = append(verdict.Expired,
					fmt.Sprintf("the accepted package %s expired on %s", entry.Package, entry.Until))
			}
		}
		verdict.Violations = append(verdict.Violations, v)
	}
	r.Policy = verdict
}

func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

// violation is why the policy doesn't permit a package, or "".
func violation(p Package, rules spdx.Policy) string {
	if p.Unknown() {
		if len(rules.Allow) > 0 {
			return "unknown license; licenses.allow lists what's allowed"
		}
		return ""
	}
	if rules.Permits(p.expression) {
		return ""
	}
	var denied, outside []string
	for _, id := range p.expression.IDs() {
		switch {
		case containsString(rules.Deny, id):
			denied = append(denied, id)
		case len(rules.Allow) > 0 && !containsString(rules.Allow, id):
			outside = append(outside, id)
		}
	}
	var parts []string
	if len(denied) > 0 {
		parts = append(parts, strings.Join(denied, ", ")+" denied")
	}
	if len(outside) > 0 {
		parts = append(parts, strings.Join(outside, ", ")+" not allowed")
	}
	return strings.Join(parts, "; ")
}

// ignoreEntry is the licenses.ignore entry naming a package: ecosystem:name,
// or ecosystem:name@version for one version (the ecosystem in any case).
func ignoreEntry(entries []project.LicenseIgnore, p Package) (project.LicenseIgnore, bool) {
	for _, entry := range entries {
		ecosystem, rest, ok := strings.Cut(entry.Package, ":")
		if !ok || !strings.EqualFold(ecosystem, p.Ecosystem) {
			continue
		}
		name, version, versioned := strings.Cut(rest, "@")
		if strings.HasPrefix(rest, "@") { // a scoped npm package: @scope/name[@version]
			scoped, v, hasVersion := strings.Cut(rest[1:], "@")
			name, version, versioned = "@"+scoped, v, hasVersion
		}
		if name == p.Name && (!versioned || version == p.Version) {
			return entry, true
		}
	}
	return project.LicenseIgnore{}, false
}
