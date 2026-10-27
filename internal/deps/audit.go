// Package deps reads the reports of dependency tools: the auditors'
// findings (hct audit, docs/audit.md) and the outdated reports (hct
// outdated, docs/outdated.md), as libexec/lib/audit.py and outdated.py do,
// finding for finding and line for line, so the binary prints what the
// scripts print (docs/design.md §19).
package deps

import (
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dnf/hecatron/internal/project"
)

// Severities are the audit severities, most severe first.
var Severities = []string{"critical", "high", "moderate", "low"}

var severityAliases = map[string]string{
	"medium": "moderate", "important": "high", "info": "low", "informational": "low", "none": "low",
}

// Finding is one advisory affecting one package (at one version).
type Finding struct {
	ID       any    // the advisory's ID: GHSA, CVE, RUSTSEC, GO, PYSEC, or the tool's own
	Package  any    // its name
	Version  any    // the installed version, or the advisory's range where the tool gives no version
	Severity string // critical, high, moderate or low; "" when the tool gives none (unrated)
	Title    any
	Aliases  []any // the advisory's other IDs, sorted
	URL      any
}

// covers says whether an accepted risk's ID is the finding's ID or one of
// its aliases.
func (f Finding) covers(id any) bool {
	key, _ := hashKey(id)
	for _, candidate := range append([]any{f.ID}, f.Aliases...) {
		if k, _ := hashKey(candidate); k == key {
			return true
		}
	}
	return false
}

// Rank is 0 for critical ... 3 for low; an unrated finding ranks as high.
func (f Finding) Rank() int {
	for i, severity := range Severities {
		if severity == f.Severity {
			return i
		}
	}
	return 1
}

func newFinding(id, pkg, version, severity, title any, aliases []any, url any) (Finding, error) {
	f := Finding{ID: id, Package: pkg, Version: version, Severity: NormalizeSeverity(severity), Title: title, URL: url}
	idKey, err := hashKey(id)
	if err != nil {
		return f, err
	}
	seen := map[string]bool{}
	var unique []any
	for _, alias := range aliases {
		if !truthy(alias) {
			continue
		}
		key, err := hashKey(alias)
		if err != nil {
			return f, err
		}
		if key != idKey && !seen[key] {
			seen[key] = true
			unique = append(unique, alias)
		}
	}
	f.Aliases, err = sortedValues(unique)
	return f, err
}

// NormalizeSeverity is a tool's severity as one of Severities: their
// aliases (medium, important...) and CVSS scores mapped, "" for none.
func NormalizeSeverity(value any) string {
	if value == nil {
		return ""
	}
	text := strings.ToLower(strings.TrimFunc(str(value), project.PyIsSpace))
	if alias, ok := severityAliases[text]; ok {
		text = alias
	}
	for _, severity := range Severities {
		if text == severity {
			return text
		}
	}
	score, ok := pyFloat(text)
	if !ok {
		return ""
	}
	return cvssSeverity(score)
}

var pyFloatSyntax = regexp.MustCompile(`^[+-]?(?:(?:[0-9](?:_?[0-9])*)(?:\.(?:[0-9](?:_?[0-9])*)?)?|\.[0-9](?:_?[0-9])*)(?:e[+-]?[0-9](?:_?[0-9])*)?$`)

// pyFloat is Python's float() of lowercased, stripped text.
func pyFloat(text string) (float64, bool) {
	switch strings.TrimLeft(text, "+-") {
	case "inf", "infinity":
		if strings.HasPrefix(text, "-") {
			return math.Inf(-1), true
		}
		return math.Inf(1), true
	case "nan":
		return math.NaN(), true
	}
	if !pyFloatSyntax.MatchString(text) {
		return 0, false
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(text, "_", ""), 64)
	if err != nil && f == 0 {
		return 0, false
	}
	return f, true
}

func cvssSeverity(score float64) string {
	switch {
	case score >= 9.0:
		return "critical"
	case score >= 7.0:
		return "high"
	case score >= 4.0:
		return "moderate"
	case score > 0:
		return "low"
	}
	return ""
}

// firstSentence is an advisory's description cut to its first line of
// prose (pip-audit gives no title), skipping Markdown headings.
func firstSentence(value any) (string, error) {
	if truthy(value) {
		if _, ok := value.(string); !ok {
			return "", errorf("'%s' object has no attribute 'splitlines'", typeName(value))
		}
	}
	for _, line := range project.PySplitLines(str(or(value, ""))) {
		line = strings.TrimFunc(line, project.PyIsSpace)
		if line != "" && !strings.HasPrefix(line, "#") {
			if utf8.RuneCountInString(line) <= 100 {
				return line, nil
			}
			return strings.TrimRightFunc(string([]rune(line)[:97]), project.PyIsSpace) + "...", nil
		}
	}
	return "", nil
}

// The advisory ID patterns, tried in order at each word boundary, as
// audit.py's _ADVISORY_ID does with Python's \b and \d.
var advisoryPatterns = []*regexp.Regexp{
	project.PyRegexp(`^GHSA(?:-[23456789cfghjmpqrvwx]{4}){3}`),
	project.PyRegexp(`^CVE-\d{4}-\d{4,}`),
	project.PyRegexp(`^RUSTSEC-\d{4}-\d{4}`),
	project.PyRegexp(`^GO-\d{4}-\d{4,}`),
	project.PyRegexp(`^PYSEC-\d{4}-\d+`),
}

var wordRune = project.PyRegexp(`\w`)

// advisoryIDs are the advisory IDs in texts, each once, in order.
func advisoryIDs(texts ...any) ([]string, error) {
	var found []string
	for _, value := range texts {
		s, err := text(value)
		if err != nil {
			return nil, err
		}
		for _, id := range findAdvisoryIDs(s) {
			if !containsString(found, id) {
				found = append(found, id)
			}
		}
	}
	return found, nil
}

func findAdvisoryIDs(s string) []string {
	var ids []string
	previousIsWord := false
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		matched := 0
		if !previousIsWord {
			for _, pattern := range advisoryPatterns {
				if m := pattern.FindString(s[i:]); m != "" {
					next, _ := utf8.DecodeRuneInString(s[i+len(m):])
					if i+len(m) == len(s) || !wordRune.MatchString(string(next)) {
						matched = len(m)
						break
					}
				}
			}
		}
		if matched > 0 {
			ids = append(ids, s[i:i+matched])
			last, _ := utf8.DecodeLastRuneInString(s[:i+matched])
			previousIsWord = wordRune.MatchString(string(last))
			i += matched
			continue
		}
		previousIsWord = wordRune.MatchString(string(r))
		i += size
	}
	return ids
}

func containsString(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

// -- the tools' reports ------------------------------------------------------

// expect is the report, when it's a JSON object with one of the keys the
// tool always writes; anything else (an error message, nothing) is no report.
func expect(data any, keys ...string) (*Object, error) {
	if object, ok := data.(*Object); ok {
		for _, key := range keys {
			if _, present := object.Values[key]; present {
				return object, nil
			}
		}
	}
	return nil, errorf("not an audit report")
}

// finder accumulates findings and the first error, so a parser reads as
// audit.py's does.
type finder struct {
	findings []Finding
	err      error
}

func (f *finder) add(id, pkg, version, severity, title any, aliases []any, url any) {
	if f.err != nil {
		return
	}
	finding, err := newFinding(id, pkg, version, severity, title, aliases, url)
	if err != nil {
		f.err = err
		return
	}
	f.findings = append(f.findings, finding)
}

func strings2any(list []string) []any {
	out := make([]any, len(list))
	for i, s := range list {
		out[i] = s
	}
	return out
}

// firstOr is `ids[0] if ids else fallback`, with ids[1:] as aliases.
func firstOr(ids []string, fallback func() any) (any, []any) {
	if len(ids) > 0 {
		return ids[0], strings2any(ids[1:])
	}
	return fallback(), nil
}

// parseNPM reads npm audit --json (npm 7 and later), or the older
// advisories layout pnpm audit --json still writes.
func parseNPM(text string) ([]Finding, error) {
	loaded, err := jsonLoads(text)
	if err != nil {
		return nil, err
	}
	data, err := expect(loaded, "vulnerabilities", "advisories", "auditReportVersion")
	if err != nil {
		return nil, err
	}
	if advisories, present := data.Values["advisories"]; present {
		list, err := values(advisories)
		if err != nil {
			return nil, err
		}
		return npmAdvisories(list)
	}
	f := &finder{}
	vulnerabilities, err := items(or(data.Values["vulnerabilities"], newObject()))
	if err != nil {
		return nil, err
	}
	for _, name := range vulnerabilities.Keys {
		via, err := get(vulnerabilities.Values[name], "via", nil)
		if err != nil {
			return nil, err
		}
		list, err := iterate(or(via, []any{}))
		if err != nil {
			return nil, err
		}
		for _, entry := range list {
			// A string names another vulnerable package this one depends
			// on; that package's own entry holds the advisory.
			object, ok := entry.(*Object)
			if !ok {
				continue
			}
			url := getOr(object, "url", "")
			ids, err := advisoryIDs(url)
			if err != nil {
				return nil, err
			}
			id, aliases := firstOr(ids, func() any { return str(object.Values["source"]) })
			// The range is the advisory's, not the installed version (npm's
			// report doesn't give that).
			f.add(id, or(object.Values["name"], name), "", object.Values["severity"], getOr(object, "title", ""), aliases, url)
		}
	}
	return f.findings, f.err
}

// getOr is dict.get(key, default) on an object known to be one.
func getOr(object *Object, key string, def any) any {
	if value, present := object.Values[key]; present {
		return value
	}
	return def
}

func npmAdvisories(advisories []any) ([]Finding, error) {
	f := &finder{}
	for _, advisory := range advisories {
		ghsa, err := get(advisory, "github_advisory_id", nil)
		if err != nil {
			return nil, err
		}
		ghsa = or(ghsa, "")
		object := advisory.(*Object)
		aliases, err := iterate(or(object.Values["cves"], []any{}))
		if err != nil {
			return nil, err
		}
		entries, err := iterate(or(object.Values["findings"], []any{}))
		if err != nil {
			return nil, err
		}
		seen := map[string]bool{"s": true}
		var versions []any
		for _, entry := range entries {
			version, err := get(entry, "version", "")
			if err != nil {
				return nil, err
			}
			key, err := hashKey(version)
			if err != nil {
				return nil, err
			}
			if !seen[key] {
				seen[key] = true
				versions = append(versions, version)
			}
		}
		sorted, err := sortedValues(versions)
		if err != nil {
			return nil, err
		}
		joined, err := joinStrings(", ", sorted)
		if err != nil {
			return nil, err
		}
		id := ghsa
		if !truthy(ghsa) {
			id = str(object.Values["id"])
		}
		f.add(id, getOr(object, "module_name", ""), joined, object.Values["severity"], getOr(object, "title", ""),
			aliases, getOr(object, "url", ""))
	}
	return f.findings, f.err
}

// parseYarnClassic reads yarn audit --json (Yarn 1): one JSON object a line.
func parseYarnClassic(text string) ([]Finding, error) {
	stream, err := jsonValues(text)
	if err != nil {
		return nil, err
	}
	var advisories []any
	for _, value := range stream {
		object, ok := value.(*Object)
		if !ok || object.Values["type"] != "auditAdvisory" {
			continue
		}
		data, err := item(object, "data")
		if err != nil {
			return nil, err
		}
		advisory, err := item(data, "advisory")
		if err != nil {
			return nil, err
		}
		advisories = append(advisories, advisory)
	}
	unique := newObject()
	for _, advisory := range advisories {
		id, err := get(advisory, "id", nil)
		if err != nil {
			return nil, err
		}
		key, err := hashKey(id)
		if err != nil {
			return nil, err
		}
		unique.set(key, advisory)
	}
	list, _ := values(unique)
	return npmAdvisories(list)
}

// parseYarnBerry reads yarn npm audit --json (Yarn 2 and later): one
// object a line, a package and its advisory's fields.
func parseYarnBerry(text string) ([]Finding, error) {
	stream, err := jsonValues(text)
	if err != nil {
		return nil, err
	}
	f := &finder{}
	for _, value := range stream {
		object, ok := value.(*Object)
		if !ok {
			continue
		}
		child, ok := object.Values["children"].(*Object)
		if !ok {
			continue
		}
		url := str(getOr(child, "URL", ""))
		ids, _ := advisoryIDs(url)
		id, aliases := firstOr(ids, func() any { return str(getOr(child, "ID", "")) })
		versions, err := iterate(or(child.Values["Tree Versions"], []any{}))
		if err != nil {
			return nil, err
		}
		texts := make([]string, len(versions))
		for i, version := range versions {
			texts[i] = str(version)
		}
		f.add(id, str(getOr(object, "value", "")), strings.Join(texts, ", "), child.Values["Severity"],
			str(getOr(child, "Issue", "")), aliases, url)
	}
	return f.findings, f.err
}

// parseBun reads bun audit --json: each package's advisories, as npm's
// bulk advisory endpoint returns them.
func parseBun(text string) ([]Finding, error) {
	loaded, err := jsonLoads(text)
	if err != nil {
		return nil, err
	}
	data, err := items(loaded)
	if err != nil {
		return nil, err
	}
	f := &finder{}
	for _, name := range data.Keys {
		list, ok := data.Values[name].([]any)
		if !ok {
			continue
		}
		for _, advisory := range list {
			url, err := get(advisory, "url", "")
			if err != nil {
				return nil, err
			}
			object := advisory.(*Object)
			ids, err := advisoryIDs(url)
			if err != nil {
				return nil, err
			}
			id, aliases := firstOr(ids, func() any { return str(object.Values["id"]) })
			f.add(id, name, getOr(object, "vulnerable_versions", ""), object.Values["severity"],
				getOr(object, "title", ""), aliases, url)
		}
	}
	return f.findings, f.err
}

// parsePipAudit reads pip-audit -f json: no severities, so each counts
// as high.
func parsePipAudit(text string) ([]Finding, error) {
	loaded, err := jsonLoads(text)
	if err != nil {
		return nil, err
	}
	data, err := expect(loaded, "dependencies")
	if err != nil {
		return nil, err
	}
	dependencies, err := iterate(data.Values["dependencies"])
	if err != nil {
		return nil, err
	}
	f := &finder{}
	for _, dependency := range dependencies {
		vulns, err := get(dependency, "vulns", nil)
		if err != nil {
			return nil, err
		}
		list, err := iterate(or(vulns, []any{}))
		if err != nil {
			return nil, err
		}
		object := dependency.(*Object)
		for _, vuln := range list {
			id, err := item(vuln, "id")
			if err != nil {
				return nil, err
			}
			v := vuln.(*Object)
			title, err := firstSentence(getOr(v, "description", ""))
			if err != nil {
				return nil, err
			}
			aliases, err := iterate(or(v.Values["aliases"], []any{}))
			if err != nil {
				return nil, err
			}
			f.add(id, getOr(object, "name", ""), getOr(object, "version", ""), nil, title, aliases, "")
		}
	}
	return f.findings, f.err
}

// parseGovulncheck reads govulncheck -json: the vulnerabilities the code
// calls (a finding whose trace reaches a function), as govulncheck itself
// reports by default. The Go database has no severities, so each counts as
// high.
func parseGovulncheck(text string) ([]Finding, error) {
	stream, err := jsonValues(text)
	if err != nil {
		return nil, err
	}
	report := false
	for _, value := range stream {
		if object, ok := value.(*Object); ok {
			if _, present := object.Values["config"]; present {
				report = true
			}
		}
	}
	if !report {
		return nil, errorf("not a govulncheck report")
	}
	entries := map[string]*Object{}
	type call struct {
		id    any
		frame any
	}
	called := map[string]call{}
	var order []string
	for _, value := range stream {
		object, ok := value.(*Object)
		if !ok {
			continue
		}
		if osv, ok := object.Values["osv"].(*Object); ok {
			id, err := item(osv, "id")
			if err != nil {
				return nil, err
			}
			key, err := hashKey(id)
			if err != nil {
				return nil, err
			}
			entries[key] = osv
		}
		finding, ok := object.Values["finding"].(*Object)
		if !ok {
			continue
		}
		trace := or(finding.Values["trace"], []any{newObject()})
		first, err := index(trace, 0)
		if err != nil {
			return nil, err
		}
		function, err := get(first, "function", nil)
		if err != nil {
			return nil, err
		}
		if truthy(function) {
			id, err := item(finding, "osv")
			if err != nil {
				return nil, err
			}
			key, err := hashKey(id)
			if err != nil {
				return nil, err
			}
			if _, seen := called[key]; !seen {
				called[key] = call{id, first}
				order = append(order, key)
			}
		}
	}
	calls := make([]call, 0, len(order))
	for _, key := range order {
		calls = append(calls, called[key])
	}
	var sortErr error
	sort.SliceStable(calls, func(i, j int) bool {
		less, err := pyLess(calls[i].id, calls[j].id)
		if err != nil && sortErr == nil {
			sortErr = err
		}
		return less
	})
	if sortErr != nil {
		return nil, sortErr
	}
	f := &finder{}
	for _, c := range calls {
		key, _ := hashKey(c.id)
		entry := entries[key]
		if entry == nil {
			entry = newObject()
		}
		frame := c.frame.(*Object)
		aliases, err := iterate(or(entry.Values["aliases"], []any{}))
		if err != nil {
			return nil, err
		}
		f.add(c.id, getOr(frame, "module", ""), getOr(frame, "version", ""), nil, getOr(entry, "summary", ""), aliases, "")
	}
	return f.findings, f.err
}

// parseComposer reads composer audit --format=json.
func parseComposer(text string) ([]Finding, error) {
	loaded, err := jsonLoads(text)
	if err != nil {
		return nil, err
	}
	data, err := expect(loaded, "advisories")
	if err != nil {
		return nil, err
	}
	advisories, err := items(or(data.Values["advisories"], newObject()))
	if err != nil {
		return nil, err
	}
	f := &finder{}
	for _, name := range advisories.Keys {
		var list []any
		if object, ok := advisories.Values[name].(*Object); ok {
			list, _ = values(object)
		} else if list, err = iterate(advisories.Values[name]); err != nil {
			return nil, err
		}
		for _, entry := range list {
			advisoryID, err := get(entry, "advisoryId", "")
			if err != nil {
				return nil, err
			}
			object := entry.(*Object)
			ids, err := advisoryIDs(advisoryID, getOr(object, "link", ""))
			if err != nil {
				return nil, err
			}
			primary, aliases := firstOr(ids, func() any { return str(object.Values["advisoryId"]) })
			if truthy(object.Values["cve"]) {
				aliases = append([]any{object.Values["cve"]}, aliases...)
			}
			f.add(primary, name, getOr(object, "affectedVersions", ""), object.Values["severity"],
				getOr(object, "title", ""), aliases, getOr(object, "link", ""))
		}
	}
	return f.findings, f.err
}

var (
	bundleAuditReport = regexp.MustCompile(`(?m)^(No vulnerabilities found|Vulnerabilities found!|Name:)`)
	bundleAuditField  = project.PyRegexp(`^(Name|Version|CVE|GHSA|Advisory|Criticality|URL|Title|Solution):\s*(.*)$`)
)

// parseBundleAudit reads bundle-audit check's text report: a block of
// "Name:", "Version:", "CVE:" or "GHSA:", "Criticality:"... lines per
// advisory.
func parseBundleAudit(text string) ([]Finding, error) {
	if !bundleAuditReport.MatchString(text) {
		return nil, errorf("not a bundle-audit report")
	}
	f := &finder{}
	block := map[string]string{}
	flush := func() {
		if block["name"] != "" {
			ids, _ := advisoryIDs(block["cve"], block["ghsa"], block["advisory"], block["url"])
			primary, aliases := firstOr(ids, func() any {
				if block["advisory"] != "" {
					return block["advisory"]
				}
				return block["name"]
			})
			var severity any
			if value, present := block["criticality"]; present && value != "Unknown" {
				severity = value
			}
			f.add(primary, block["name"], block["version"], severity, block["title"], aliases, block["url"])
		}
		block = map[string]string{}
	}
	for _, line := range project.PySplitLines(text) {
		match := bundleAuditField.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		field, value := strings.ToLower(match[1]), strings.TrimFunc(match[2], project.PyIsSpace)
		if field == "name" {
			flush()
		}
		block[field] = value
	}
	flush()
	return f.findings, f.err
}

// parseDotnet reads dotnet list package --vulnerable --include-transitive
// --format json.
func parseDotnet(text string) ([]Finding, error) {
	loaded, err := jsonLoads(text)
	if err != nil {
		return nil, err
	}
	data, err := expect(loaded, "projects", "version")
	if err != nil {
		return nil, err
	}
	f := &finder{}
	err = eachIn(data, "projects", func(projectEntry any) error {
		return eachIn(projectEntry, "frameworks", func(framework any) error {
			for _, kind := range []string{"topLevelPackages", "transitivePackages"} {
				err := eachIn(framework, kind, func(pkg any) error {
					return eachIn(pkg, "vulnerabilities", func(vuln any) error {
						url, err := get(vuln, "advisoryurl", "")
						if err != nil {
							return err
						}
						ids, err := advisoryIDs(url)
						if err != nil {
							return err
						}
						id, aliases := firstOr(ids, func() any { return url })
						p := pkg.(*Object)
						f.add(id, getOr(p, "id", ""), getOr(p, "resolvedVersion", ""), vuln.(*Object).Values["severity"], "",
							aliases, url)
						return nil
					})
				})
				if err != nil {
					return err
				}
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return f.findings, f.err
}

// eachIn is `for x in value.get(key) or []`.
func eachIn(value any, key string, do func(any) error) error {
	found, err := get(value, key, nil)
	if err != nil {
		return err
	}
	list, err := iterate(or(found, []any{}))
	if err != nil {
		return err
	}
	for _, x := range list {
		if err := do(x); err != nil {
			return err
		}
	}
	return nil
}

// parseOSV reads osv-scanner --format json: one finding per group of
// aliases.
func parseOSV(text string) ([]Finding, error) {
	var data any = newObject()
	if strings.TrimFunc(text, project.PyIsSpace) != "" {
		loaded, err := jsonLoads(text)
		if err != nil {
			return nil, err
		}
		if data, err = expect(loaded, "results"); err != nil {
			return nil, err
		}
	}
	f := &finder{}
	err := eachIn(data, "results", func(result any) error {
		return eachIn(result, "packages", func(pkg any) error {
			p, err := items(pkg)
			if err != nil {
				return errorf("'%s' object has no attribute 'get'", typeName(pkg))
			}
			info := or(p.Values["package"], newObject())
			vulns := newObject() // by hashKey
			var vulnIDs []any
			if err := eachIn(p, "vulnerabilities", func(v any) error {
				id, err := item(v, "id")
				if err != nil {
					return err
				}
				key, err := hashKey(id)
				if err != nil {
					return err
				}
				if _, seen := vulns.Values[key]; !seen {
					vulnIDs = append(vulnIDs, id)
				}
				vulns.set(key, v)
				return nil
			}); err != nil {
				return err
			}
			groups := p.Values["groups"]
			if !truthy(groups) {
				var made []any
				for _, id := range vulnIDs {
					group := newObject()
					group.set("ids", []any{id})
					made = append(made, group)
				}
				groups = made
				if made == nil {
					groups = []any{}
				}
			}
			list, err := iterate(groups)
			if err != nil {
				return err
			}
			for _, group := range list {
				idsValue, err := get(group, "ids", nil)
				if err != nil {
					return err
				}
				ids, err := iterate(or(idsValue, []any{}))
				if err != nil {
					return err
				}
				if len(ids) == 0 {
					return errorf("list index out of range")
				}
				var primary any
				for _, id := range ids {
					s, ok := id.(string)
					if !ok {
						return errorf("'%s' object has no attribute 'startswith'", typeName(id))
					}
					if strings.HasPrefix(s, "GHSA-") {
						primary = s
						break
					}
				}
				if primary == nil {
					primary = ids[0]
				}
				g := group.(*Object)
				severity := ""
				if truthy(g.Values["max_severity"]) {
					severity = NormalizeSeverity(g.Values["max_severity"])
				}
				primaryKey, _ := hashKey(primary)
				firstKey, _ := hashKey(ids[0])
				vuln := or(or(vulns.Values[primaryKey], vulns.Values[firstKey]), newObject())
				if severity == "" {
					specific, err := get(vuln, "database_specific", nil)
					if err != nil {
						return err
					}
					value, err := get(or(specific, newObject()), "severity", nil)
					if err != nil {
						return err
					}
					severity = NormalizeSeverity(value)
				}
				aliases := append([]any(nil), ids...)
				more, err := iterate(or(g.Values["aliases"], []any{}))
				if err != nil {
					return err
				}
				aliases = append(aliases, more...)
				v := vuln.(*Object)
				more, err = iterate(or(v.Values["aliases"], []any{}))
				if err != nil {
					return err
				}
				aliases = append(aliases, more...)
				name, err := get(info, "name", "")
				if err != nil {
					return err
				}
				version, _ := get(info, "version", "")
				var sev any
				if severity != "" {
					sev = severity
				}
				f.add(primary, name, version, sev, getOr(v, "summary", ""), aliases, "")
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return f.findings, f.err
}

var auditParsers = map[string]func(string) ([]Finding, error){
	"npm":          parseNPM,
	"pnpm":         parseNPM,
	"yarn-classic": parseYarnClassic,
	"yarn-berry":   parseYarnBerry,
	"bun":          parseBun,
	"pip-audit":    parsePipAudit,
	"govulncheck":  parseGovulncheck,
	"composer":     parseComposer,
	"bundle-audit": parseBundleAudit,
	"dotnet":       parseDotnet,
	"osv":          parseOSV,
}

// IsAuditFormat says whether ParseAudit reads a format.
func IsAuditFormat(format string) bool { _, ok := auditParsers[format]; return ok }

// ParseAudit reads a tool's report into its findings, each (ID, package,
// version) once, most severe first, then by package and ID.
func ParseAudit(format, text string) ([]Finding, error) {
	parser, ok := auditParsers[format]
	if !ok {
		return nil, fmt.Errorf("unknown audit format %q", format)
	}
	findings, err := parser(text)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var unique []Finding
	for _, f := range findings {
		var key []string
		for _, part := range []any{f.ID, f.Package, f.Version} {
			k, err := hashKey(part)
			if err != nil {
				return nil, err
			}
			key = append(key, k)
		}
		joined := strings.Join(key, "\x00")
		if !seen[joined] {
			seen[joined] = true
			unique = append(unique, f)
		}
	}
	var sortErr error
	sort.SliceStable(unique, func(i, j int) bool {
		a, b := unique[i], unique[j]
		if a.Rank() != b.Rank() {
			return a.Rank() < b.Rank()
		}
		less, err := tupleLess([]any{a.Package, a.ID}, []any{b.Package, b.ID})
		if err != nil && sortErr == nil {
			sortErr = err
		}
		return less
	})
	return unique, sortErr
}

// pyLess is Python's a < b for the values a report's names and IDs are.
func pyLess(a, b any) (bool, error) {
	switch x := a.(type) {
	case string:
		if y, ok := b.(string); ok {
			return x < y, nil
		}
	case Number:
		if y, ok := b.(Number); ok {
			return x.float() < y.float(), nil
		}
	}
	return false, errorf("'<' not supported between instances of '%s' and '%s'", typeName(a), typeName(b))
}

func tupleLess(a, b []any) (bool, error) {
	for i := range a {
		ka, _ := hashKey(a[i])
		kb, _ := hashKey(b[i])
		if ka != kb {
			return pyLess(a[i], b[i])
		}
	}
	return false, nil
}

// -- accepted risks and the threshold ------------------------------------

// Accepted is a finding an "audit.ignore" entry covers.
type Accepted struct {
	Finding Finding
	Entry   *Object
}

// SplitIgnored splits findings into those kept, those an entry accepts
// until its date (today, YYYY-MM-DD, included), and those an expired entry
// no longer covers (also kept).
func SplitIgnored(findings []Finding, ignore any, today string) (kept []Finding, ignored, expired []Accepted, err error) {
	for _, finding := range findings {
		var entry *Object
		list, err := iterate(ignore)
		if err != nil {
			return nil, nil, nil, err
		}
		for _, e := range list {
			object, ok := e.(*Object)
			if !ok {
				continue
			}
			id := object.Values["id"]
			if _, err := hashKey(id); err != nil {
				return nil, nil, nil, err
			}
			if finding.covers(id) {
				entry = object
				break
			}
		}
		switch {
		case entry == nil:
			kept = append(kept, finding)
		case str(getOr(entry, "until", "")) >= today:
			ignored = append(ignored, Accepted{finding, entry})
		default:
			expired = append(expired, Accepted{finding, entry})
			kept = append(kept, finding)
		}
	}
	return kept, ignored, expired, nil
}

// Threshold is the severity at which the audit fails: the flag
// (HECATRON_AUDIT_FAIL_ON), else the project's "audit.failOn", else high.
func Threshold(config any, flag string) (string, error) {
	var value any = flag
	if flag == "" {
		audit, err := get(config, "audit", nil)
		if err != nil {
			return "", err
		}
		value, err = get(or(audit, newObject()), "failOn", nil)
		if err != nil {
			return "", err
		}
		value = or(value, "high")
	}
	if s, ok := value.(string); ok && containsString(Severities, s) {
		return s, nil
	}
	return "", errorf("--fail-on must be one of %s, not %s", strings.Join(Severities, ", "), repr(value))
}

// Summary is the count line's tail: "2 vulnerabilities (critical 0, high
// 1, moderate 1, low 0)".
func Summary(findings []Finding) string {
	counts := map[string]int{}
	unrated := 0
	for _, f := range findings {
		if f.Severity != "" {
			counts[f.Severity]++
		} else {
			unrated++
		}
	}
	var parts []string
	for _, severity := range Severities {
		parts = append(parts, fmt.Sprintf("%s %d", severity, counts[severity]))
	}
	if unrated > 0 {
		parts = append(parts, fmt.Sprintf("unrated %d", unrated))
	}
	noun := "vulnerabilities"
	if len(findings) == 1 {
		noun = "vulnerability"
	}
	return fmt.Sprintf("%d %s (%s)", len(findings), noun, strings.Join(parts, ", "))
}

func ljust(s string, width int) string {
	if n := utf8.RuneCountInString(s); n < width {
		return s + strings.Repeat(" ", width-n)
	}
	return s
}

// Line is a finding as the audit prints it.
func Line(f Finding) (string, error) {
	severity := f.Severity
	if severity == "" {
		severity = "unrated"
	}
	version, aliases, title := "", "", ""
	if truthy(f.Version) {
		version = " " + str(f.Version)
	}
	if len(f.Aliases) > 0 {
		shown := f.Aliases
		if len(shown) > 3 {
			shown = shown[:3]
		}
		joined, err := joinStrings(", ", shown)
		if err != nil {
			return "", err
		}
		aliases = " (" + joined + ")"
	}
	if truthy(f.Title) {
		title = ": " + str(f.Title)
	}
	return fmt.Sprintf("  %s %s%s  %s%s%s", ljust(severity, 9), str(f.Package), version, str(f.ID), aliases, title), nil
}

// AuditReport prints a report's findings and summary as audit.py's report
// does and returns the exit status: 1 when a finding not accepted is at or
// above the threshold. config is the project's config.json as decoded
// (nil for none), failOn the flag's value, today YYYY-MM-DD.
func AuditReport(w io.Writer, format, text, lang, component string, config any, failOn, today string) (int, error) {
	if config == nil {
		config = newObject()
	}
	findings, err := ParseAudit(format, text)
	if err != nil {
		return 1, err
	}
	threshold, err := Threshold(config, failOn)
	if err != nil {
		return 1, err
	}
	audit, err := get(config, "audit", nil)
	if err != nil {
		return 1, err
	}
	ignore, err := get(or(audit, newObject()), "ignore", nil)
	if err != nil {
		return 1, err
	}
	kept, ignored, expired, err := SplitIgnored(findings, or(ignore, []any{}), today)
	if err != nil {
		return 1, err
	}
	for _, f := range kept {
		line, err := Line(f)
		if err != nil {
			return 1, err
		}
		_, _ = fmt.Fprintln(w, line)
	}
	for _, a := range ignored {
		line, err := Line(a.Finding)
		if err != nil {
			return 1, err
		}
		_, _ = fmt.Fprintf(w, "%s  [accepted until %s: %s]\n", line, str(a.Entry.Values["until"]), str(getOr(a.Entry, "reason", "")))
	}
	for _, a := range expired {
		_, _ = fmt.Fprintf(w, "  note: the accepted risk %s expired on %s\n", str(a.Entry.Values["id"]), str(a.Entry.Values["until"]))
	}
	line := fmt.Sprintf("%s: %s %s", component, lang, Summary(kept))
	if len(ignored) > 0 {
		line += fmt.Sprintf(", %d accepted", len(ignored))
	}
	_, _ = fmt.Fprintln(w, line)
	limit := 0
	for i, severity := range Severities {
		if severity == threshold {
			limit = i
		}
	}
	failing := 0
	for _, f := range kept {
		if f.Rank() <= limit {
			failing++
		}
	}
	if failing > 0 {
		_, _ = fmt.Fprintf(w, "%s: %s: %d at or above %s\n", component, lang, failing, threshold)
		return 1, nil
	}
	return 0, nil
}

// EOLWarning is a warning when the installed version's line is past its
// end of life, from a definition's releases.sh output (version<TAB>flags,
// the newest release of each line). A line is the definition's
// "releaseLine" leading parts (1 for Node.js: 16 for 16.20.2), else all but
// the last (3.9 for 3.9.25), and never the whole version: .NET's
// releaseLine of 3 names a feature band, but support ends by channel.
// linePart is 0 when the definition has none.
func EOLWarning(lang, version, releases string, lineParts int) string {
	parts := strings.Split(strings.TrimLeft(version, "v"), ".")
	for _, line := range project.PySplitLines(releases) {
		latest, flags, _ := strings.Cut(line, "\t")
		latestParts := strings.Split(latest, ".")
		whole := len(latestParts) - 1
		if whole < 1 {
			whole = 1
		}
		n := whole
		if lineParts != 0 && lineParts < whole {
			n = lineParts
		}
		if n < 0 {
			n = len(latestParts) + n
			if n < 0 {
				n = 0
			}
		}
		prefix := latestParts[:n]
		if len(parts) >= len(prefix) && equalStrings(parts[:len(prefix)], prefix) {
			if containsString(strings.Split(flags, ","), "eol") {
				return fmt.Sprintf("warning: %s %s is past its end of life (line %s); upgrade it", lang, version,
					strings.Join(prefix, "."))
			}
			return ""
		}
	}
	return ""
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
