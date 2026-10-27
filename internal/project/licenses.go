package project

import (
	"fmt"

	"github.com/dnf/hecatron/internal/spdx"
)

// LicensePolicy is config.json's "licenses" section (hct licenses,
// docs/licenses.md): the licenses allowed (none listed: any) and denied,
// accepted packages, and whether dev and test dependencies count.
type LicensePolicy struct {
	Allow, Deny []string // canonical SPDX identifiers
	Ignore      []LicenseIgnore
	IncludeDev  bool
	Set         bool // the project has a "licenses" section
}

// LicenseIgnore accepts one package (ecosystem:name, optionally @version)
// whatever its license, for a reason, until a date (YYYY-MM-DD).
type LicenseIgnore struct {
	Package, Reason, Until string
}

// LicensesProblems is what's wrong with config.json's "licenses" section,
// for hecatron check. Only the binary reads the section (hct licenses is
// native only), so only its check reports these.
func (p *Project) LicensesProblems() []string {
	_, problems := p.licensePolicy()
	return problems
}

// LicensePolicy is the project's license policy, its problems left out.
func (p *Project) LicensePolicy() LicensePolicy {
	policy, _ := p.licensePolicy()
	return policy
}

func (p *Project) licensePolicy() (LicensePolicy, []string) {
	var policy LicensePolicy
	section, present := p.config.Get("licenses")
	if !present || section == nil {
		return policy, nil
	}
	policy.Set = true
	object, ok := section.(*Object)
	if !ok {
		return policy, []string{`"licenses" must be an object`}
	}
	var problems []string
	for _, key := range object.Keys {
		switch key {
		case "allow", "deny", "ignore", "includeDev":
		default:
			problems = append(problems, fmt.Sprintf("licenses.%s isn't a setting (allow, deny, ignore, includeDev)", key))
		}
	}
	ids := func(key string) []string {
		value, _ := object.Get(key)
		if value == nil {
			return nil
		}
		list, ok := value.([]any)
		if !ok {
			problems = append(problems, fmt.Sprintf("licenses.%s must be a list of SPDX license identifiers", key))
			return nil
		}
		var out []string
		for _, item := range list {
			text, isString := item.(string)
			id, valid := spdx.Canonical(text)
			if !isString || !valid {
				problems = append(problems, fmt.Sprintf("licenses.%s: %s isn't an SPDX license identifier (https://spdx.org/licenses/)",
					key, describeValue(item)))
				continue
			}
			out = append(out, id)
		}
		return out
	}
	policy.Allow, policy.Deny = ids("allow"), ids("deny")
	if value, _ := object.Get("includeDev"); value != nil {
		include, ok := value.(bool)
		if !ok {
			problems = append(problems, "licenses.includeDev must be true or false")
		}
		policy.IncludeDev = include
	}
	ignore, _ := object.Get("ignore")
	entries, ok := ignore.([]any)
	if ignore != nil && !ok {
		problems = append(problems, "licenses.ignore must be a list")
	}
	for index, raw := range entries {
		entry, isObject := raw.(*Object)
		pkg, _ := entry.Get("package")
		name, isString := pkg.(string)
		if !isObject || !isString || name == "" {
			problems = append(problems, fmt.Sprintf(`licenses.ignore[%d] needs a "package" (ecosystem:name, e.g. npm:caniuse-lite)`, index))
			continue
		}
		where := "licenses.ignore " + name
		reason, _ := entry.Get("reason")
		until, _ := entry.Get("until")
		reasonText, _ := reason.(string)
		untilText, _ := until.(string)
		if reasonText == "" || pyStrip(reasonText) == "" {
			problems = append(problems, where+` needs a "reason"`)
		}
		if !isISODate(until) {
			problems = append(problems, where+` needs an "until" date (YYYY-MM-DD)`)
		}
		policy.Ignore = append(policy.Ignore, LicenseIgnore{Package: name, Reason: reasonText, Until: untilText})
	}
	return policy, problems
}

// describeValue is a config value in a message: a string quoted, anything
// else as JSON would write it.
func describeValue(value any) string {
	switch v := value.(type) {
	case string:
		return fmt.Sprintf("%q", v)
	case nil:
		return "null"
	case *Object:
		return "an object"
	case []any:
		return "a list"
	}
	return fmt.Sprint(value)
}
