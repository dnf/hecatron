package deps

import (
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The versions that fix each finding, for hecatron's own reports (hct
// audit --output json, SARIF): what the tool says, or what its advisory's
// range implies. Read apart from ParseAudit, with Go's JSON rules and
// never an error, since the scripts print no fixed versions and a report
// they read must not fail here.

// Fixes are a report's fixed versions by package and advisory ID (each of
// an advisory's IDs and aliases).
type Fixes map[string][]string

func fixKey(pkg, id string) string { return pkg + "\x00" + id }

// For is the versions that fix a finding: those listed for its ID, else
// for one of its aliases, at its version where the report says per
// version; nil when the report gives none.
func (fixes Fixes) For(f Finding) []string {
	pkg := Text(f.Package)
	ids := append([]any{f.ID}, f.Aliases...)
	for _, key := range []string{pkg + "@" + Text(f.Version), pkg} {
		for _, id := range ids {
			if versions, ok := fixes[fixKey(key, Text(id))]; ok {
				return versions
			}
		}
	}
	return nil
}

// addVersion adds fixes for a package at a version, and for the package.
func (fixes Fixes) addVersion(pkg, version string, ids []string, versions []string) {
	if version != "" {
		fixes.add(pkg+"@"+version, ids, versions)
	}
	fixes.add(pkg, ids, versions)
}

func (fixes Fixes) add(pkg string, ids []string, versions []string) {
	if len(versions) == 0 {
		return
	}
	for _, id := range ids {
		if id == "" {
			continue
		}
		key := fixKey(pkg, id)
		fixes[key] = appendUnique(fixes[key], versions...)
	}
}

func appendUnique(list []string, values ...string) []string {
	for _, value := range values {
		if value != "" && !containsString(list, value) {
			list = append(list, value)
		}
	}
	return list
}

var (
	// The upper bound of an npm or Composer range part (<1.2.3, not <=).
	exclusiveUpper = regexp.MustCompile(`<\s*v?([0-9][^\s,|<>=]*)`)
	// The lower bound of a patched range (>=1.2.3).
	inclusiveLower = regexp.MustCompile(`>=\s*v?([0-9][^\s,|<>=]*)`)
	// A version in a requirement (bundler-audit's '~> 2.0.9, >= 2.0.9.1').
	requirement = regexp.MustCompile(`(~>|>=|>|=)?\s*([0-9][0-9A-Za-z.\-]*)`)
)

// fixedByRange is the version each part of a vulnerable range ends below
// ("<4.17.21", ">=3.7.0 <4.17.19 || >=5.0.0 <5.0.2"; Composer's "|" and ","
// too): the first version outside it.
func fixedByRange(vulnerable string) []string {
	var fixed []string
	for _, m := range exclusiveUpper.FindAllStringSubmatch(vulnerable, -1) {
		fixed = appendUnique(fixed, m[1])
	}
	return fixed
}

// fixedByPatched is the lowest version of each part of a patched range
// (">=4.17.21"); none for npm's "<0.0.0", which means no fix.
func fixedByPatched(patched string) []string {
	var fixed []string
	for _, m := range inclusiveLower.FindAllStringSubmatch(patched, -1) {
		fixed = appendUnique(fixed, m[1])
	}
	return fixed
}

// fixedByRequirements is a version from each of bundler-audit's quoted
// requirements ('~> 2.0.9, >= 2.0.9.1', '>= 2.2.3.1'): its >= bound,
// else its first version.
func fixedByRequirements(solution string) []string {
	var fixed []string
	parts := strings.Split(solution, "'")
	for i := 1; i < len(parts); i += 2 {
		best := ""
		for _, m := range requirement.FindAllStringSubmatch(parts[i], -1) {
			if m[1] == ">=" || best == "" {
				best = m[2]
			}
		}
		fixed = appendUnique(fixed, best)
	}
	return fixed
}

func idsOf(texts ...string) []string {
	var ids []string
	for _, text := range texts {
		ids = appendUnique(ids, findAdvisoryIDs(text)...)
	}
	return ids
}

// FixedVersions reads a report's fixed versions; an empty set for a
// report it can't read, or a tool that gives none (dotnet).
func FixedVersions(format, text string) Fixes {
	fixes := Fixes{}
	switch format {
	case "npm", "pnpm":
		npmFixes(fixes, text)
	case "yarn-classic":
		for _, line := range strings.Split(text, "\n") {
			var entry struct {
				Type string `json:"type"`
				Data struct {
					Advisory npmAdvisory `json:"advisory"`
				} `json:"data"`
			}
			if json.Unmarshal([]byte(line), &entry) == nil && entry.Type == "auditAdvisory" {
				entry.Data.Advisory.addTo(fixes)
			}
		}
	case "yarn-berry":
		for _, line := range strings.Split(text, "\n") {
			var entry struct {
				Value    string `json:"value"`
				Children struct {
					ID         any    `json:"ID"`
					URL        string `json:"URL"`
					Vulnerable string `json:"Vulnerable Versions"`
					Patched    string `json:"Patched Versions"`
				} `json:"children"`
			}
			if json.Unmarshal([]byte(line), &entry) != nil {
				continue
			}
			c := entry.Children
			fixed := fixedByPatched(c.Patched)
			if len(fixed) == 0 {
				fixed = fixedByRange(c.Vulnerable)
			}
			fixes.add(entry.Value, append(idsOf(c.URL), idText(c.ID)), fixed)
		}
	case "bun":
		var report map[string][]struct {
			ID         any    `json:"id"`
			URL        string `json:"url"`
			Vulnerable string `json:"vulnerable_versions"`
		}
		if json.Unmarshal([]byte(text), &report) == nil {
			for name, advisories := range report {
				for _, a := range advisories {
					fixes.add(name, append(idsOf(a.URL), idText(a.ID)), fixedByRange(a.Vulnerable))
				}
			}
		}
	case "pip-audit":
		var report struct {
			Dependencies []struct {
				Name  string `json:"name"`
				Vulns []struct {
					ID      string   `json:"id"`
					Aliases []string `json:"aliases"`
					Fixed   []string `json:"fix_versions"`
				} `json:"vulns"`
			} `json:"dependencies"`
		}
		if json.Unmarshal([]byte(text), &report) == nil {
			for _, d := range report.Dependencies {
				for _, v := range d.Vulns {
					fixes.add(d.Name, append([]string{v.ID}, v.Aliases...), v.Fixed)
				}
			}
		}
	case "govulncheck":
		govulncheckFixes(fixes, text)
	case "composer":
		var report struct {
			Advisories map[string]json.RawMessage `json:"advisories"`
		}
		if json.Unmarshal([]byte(text), &report) == nil {
			for name, raw := range report.Advisories {
				for _, a := range composerAdvisories(raw) {
					ids := append(idsOf(a.AdvisoryID, a.Link), a.AdvisoryID, a.CVE)
					fixes.add(name, ids, fixedByRange(a.Affected))
				}
			}
		}
	case "bundle-audit":
		bundleAuditFixes(fixes, text)
	case "osv":
		osvFixes(fixes, text)
	}
	return fixes
}

// idText is an ID a report gives as a number or a string.
func idText(id any) string {
	switch v := id.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return ""
}

type npmAdvisory struct {
	ID         any      `json:"id"`
	GHSA       string   `json:"github_advisory_id"`
	CVEs       []string `json:"cves"`
	Module     string   `json:"module_name"`
	URL        string   `json:"url"`
	Vulnerable string   `json:"vulnerable_versions"`
	Patched    string   `json:"patched_versions"`
}

func (a npmAdvisory) addTo(fixes Fixes) {
	fixed := fixedByPatched(a.Patched)
	if len(fixed) == 0 && a.Patched == "" {
		fixed = fixedByRange(a.Vulnerable)
	}
	ids := append(append([]string{a.GHSA, idText(a.ID)}, a.CVEs...), idsOf(a.URL)...)
	fixes.add(a.Module, ids, fixed)
}

// npmFixes reads npm audit --json: each advisory's vulnerable range (npm 7
// and later), or the advisories layout's patched versions (pnpm).
func npmFixes(fixes Fixes, text string) {
	var report struct {
		Advisories      map[string]npmAdvisory `json:"advisories"`
		Vulnerabilities map[string]struct {
			Via []json.RawMessage `json:"via"`
		} `json:"vulnerabilities"`
	}
	if json.Unmarshal([]byte(text), &report) != nil {
		return
	}
	for _, a := range report.Advisories {
		a.addTo(fixes)
	}
	for name, v := range report.Vulnerabilities {
		for _, raw := range v.Via {
			var via struct {
				Name   string `json:"name"`
				Source any    `json:"source"`
				URL    string `json:"url"`
				Range  string `json:"range"`
			}
			if json.Unmarshal(raw, &via) != nil {
				continue // a string: another package's entry holds the advisory
			}
			pkg := via.Name
			if pkg == "" {
				pkg = name
			}
			fixes.add(pkg, append(idsOf(via.URL), idText(via.Source)), fixedByRange(via.Range))
		}
	}
}

func govulncheckFixes(fixes Fixes, text string) {
	decoder := json.NewDecoder(strings.NewReader(text))
	for {
		var message struct {
			OSV *struct {
				ID      string   `json:"id"`
				Aliases []string `json:"aliases"`
			} `json:"osv"`
			Finding *struct {
				OSV   string `json:"osv"`
				Fixed string `json:"fixed_version"`
				Trace []struct {
					Module string `json:"module"`
				} `json:"trace"`
			} `json:"finding"`
		}
		if decoder.Decode(&message) != nil {
			return
		}
		if f := message.Finding; f != nil && f.Fixed != "" && len(f.Trace) > 0 {
			fixes.add(f.Trace[0].Module, []string{f.OSV}, []string{f.Fixed})
		}
	}
}

type composerAdvisory struct {
	AdvisoryID string `json:"advisoryId"`
	CVE        string `json:"cve"`
	Link       string `json:"link"`
	Affected   string `json:"affectedVersions"`
}

// composerAdvisories are a package's advisories, a list or an object.
func composerAdvisories(raw json.RawMessage) []composerAdvisory {
	var list []composerAdvisory
	if json.Unmarshal(raw, &list) == nil {
		return list
	}
	var object map[string]composerAdvisory
	_ = json.Unmarshal(raw, &object)
	for _, a := range object {
		list = append(list, a)
	}
	return list
}

func bundleAuditFixes(fixes Fixes, text string) {
	block := map[string]string{}
	flush := func() {
		if block["name"] != "" {
			ids := append(idsOf(block["cve"], block["ghsa"], block["advisory"], block["url"]),
				block["advisory"], block["name"])
			fixes.add(block["name"], ids, fixedByRequirements(block["solution"]))
		}
		block = map[string]string{}
	}
	for _, line := range strings.Split(text, "\n") {
		match := bundleAuditField.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if match == nil {
			continue
		}
		field := strings.ToLower(match[1])
		if field == "name" {
			flush()
		}
		block[field] = strings.TrimSpace(match[2])
	}
	flush()
}

// osvFixes reads OSV-Scanner's report: each vulnerability's fixed events
// for the package it was found in.
func osvFixes(fixes Fixes, text string) {
	var report struct {
		Results []struct {
			Packages []struct {
				Package struct {
					Name      string `json:"name"`
					Version   string `json:"version"`
					Ecosystem string `json:"ecosystem"`
				} `json:"package"`
				Vulnerabilities []struct {
					ID       string   `json:"id"`
					Aliases  []string `json:"aliases"`
					Affected []struct {
						Package struct {
							Name      string `json:"name"`
							Ecosystem string `json:"ecosystem"`
						} `json:"package"`
						Ranges []struct {
							Events []osvEvent `json:"events"`
						} `json:"ranges"`
					} `json:"affected"`
				} `json:"vulnerabilities"`
				Groups []struct {
					IDs     []string `json:"ids"`
					Aliases []string `json:"aliases"`
				} `json:"groups"`
			} `json:"packages"`
		} `json:"results"`
	}
	if json.Unmarshal([]byte(text), &report) != nil {
		return
	}
	// Each advisory's own record first: two databases' records of one
	// vulnerability (RUSTSEC's and GitHub's) can name different fixes, and
	// a finding's are its own ID's; an alias gets them only where it has
	// no record of its own.
	aliases := Fixes{}
	for _, result := range report.Results {
		for _, p := range result.Packages {
			for _, v := range p.Vulnerabilities {
				var fixed []string
				for _, affected := range v.Affected {
					if affected.Package.Name != p.Package.Name ||
						(affected.Package.Ecosystem != "" && p.Package.Ecosystem != "" &&
							!sameEcosystem(affected.Package.Ecosystem, p.Package.Ecosystem)) {
						continue
					}
					for _, r := range affected.Ranges {
						fixed = appendUnique(fixed, fixedAfter(r.Events, p.Package.Version)...)
					}
				}
				fixes.addVersion(p.Package.Name, p.Package.Version, []string{v.ID}, fixed)
				aliases.addVersion(p.Package.Name, p.Package.Version, v.Aliases, fixed)
			}
		}
	}
	for key, versions := range aliases {
		if _, own := fixes[key]; !own {
			fixes[key] = versions
		}
	}
}

type osvEvent struct {
	Introduced string `json:"introduced"`
	Fixed      string `json:"fixed"`
}

// fixedAfter is a range's fixed version for the installed one: the fix
// ending the stretch it's in (introduced 0.0.0, fixed 0.2.0 ... for
// 0.1.43: 0.2.0); every fix when the version isn't known or isn't in one.
func fixedAfter(events []osvEvent, installed string) []string {
	var all []string
	introduced := ""
	for _, event := range events {
		switch {
		case event.Introduced != "":
			introduced = event.Introduced
		case event.Fixed != "":
			all = appendUnique(all, event.Fixed)
			if installed != "" && (introduced == "" || introduced == "0" || compareVersions(introduced, installed) <= 0) &&
				compareVersions(installed, event.Fixed) < 0 {
				return []string{event.Fixed}
			}
			introduced = ""
		}
	}
	return all
}

// compareVersions orders versions by their parts, numbers as numbers
// (1.10 after 1.9), whatever the ecosystem's finer rules; a pre-release
// (1.0.0-rc1, 0.0.0-0) comes before its release.
func compareVersions(a, b string) int {
	x, y := versionParts(a), versionParts(b)
	for i := 0; i < len(x) && i < len(y); i++ {
		if c := comparePart(x[i], y[i]); c != 0 {
			return c
		}
	}
	switch {
	case len(x) == len(y):
		return 0
	case len(x) > len(y):
		return -tailOrder(x[len(y)])
	}
	return tailOrder(y[len(x)])
}

// tailOrder is +1 when a longer version's next part makes it newer (1.0.1
// after 1.0), -1 when it marks a pre-release (1.0-rc1 before 1.0).
func tailOrder(part string) int {
	if part != preRelease && part != "" && part[0] >= '0' && part[0] <= '9' {
		return -1
	}
	return 1
}

func versionParts(v string) []string {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	var parts []string
	current := ""
	digits := false
	for _, r := range v {
		isDigit := r >= '0' && r <= '9'
		isLetter := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
		switch {
		case !isDigit && !isLetter:
			if current != "" {
				parts = append(parts, current)
			}
			current = ""
			if r == '-' {
				parts = append(parts, preRelease)
			}
		case current != "" && isDigit != digits:
			parts = append(parts, current)
			current = string(r)
		default:
			current += string(r)
		}
		digits = isDigit
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// preRelease is the part a "-" makes: what follows is a pre-release, before
// the release and anything else in its place.
const preRelease = "-"

func comparePart(a, b string) int {
	switch {
	case a == b:
		return 0
	case a == preRelease:
		return -1
	case b == preRelease:
		return 1
	}
	an, aErr := strconv.ParseUint(a, 10, 64)
	bn, bErr := strconv.ParseUint(b, 10, 64)
	switch {
	case aErr == nil && bErr == nil:
		switch {
		case an < bn:
			return -1
		case an > bn:
			return 1
		}
		return 0
	case aErr == nil:
		return 1 // a number after a pre-release tag (1.0.1 after 1.0.rc)
	case bErr == nil:
		return -1
	}
	return strings.Compare(a, b)
}

// sameEcosystem compares OSV ecosystems without their release suffix
// (Debian:12, Alpine:v3.20).
func sameEcosystem(a, b string) bool {
	a, _, _ = strings.Cut(a, ":")
	b, _, _ = strings.Cut(b, ":")
	return strings.EqualFold(a, b)
}

// -- npm's installed versions ------------------------------------------------

// npmLockFiles are the lock files npm audit reads, in the directory it
// ran in.
var npmLockFiles = []string{"package-lock.json", "npm-shrinkwrap.json"}

// npmInstalled are the versions each package has in the npm lock file in
// the working directory (lockfileVersion 2 and 3: "packages"), sorted and
// joined as audit.py's _npm_installed does: npm audit names the
// vulnerable range, not the version installed.
func npmInstalled() map[string]string {
	for _, name := range npmLockFiles {
		data, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		var lock struct {
			Packages map[string]json.RawMessage `json:"packages"`
		}
		if json.Unmarshal(data, &lock) != nil {
			return nil
		}
		versions := map[string][]string{}
		for path, raw := range lock.Packages {
			var entry struct {
				Version any `json:"version"`
			}
			_ = json.Unmarshal(raw, &entry) // not an object: no version
			_, pkg, found := cutLast(path, "node_modules/")
			version, ok := entry.Version.(string)
			if !found || !ok || pkg == "" || version == "" {
				continue
			}
			versions[pkg] = appendUnique(versions[pkg], version)
		}
		installed := map[string]string{}
		for pkg, list := range versions {
			sort.Strings(list)
			installed[pkg] = strings.Join(list, ", ")
		}
		return installed
	}
	return nil
}

func cutLast(s, sep string) (before, after string, found bool) {
	if i := strings.LastIndex(s, sep); i >= 0 {
		return s[:i], s[i+len(sep):], true
	}
	return s, "", false
}

// withInstalled fills in the installed version of npm's findings, which
// its report leaves out, from the lock file it read.
func withInstalled(format string, findings []Finding) []Finding {
	if format != "npm" {
		return findings
	}
	installed := npmInstalled()
	if len(installed) == 0 {
		return findings
	}
	for i, f := range findings {
		if name, ok := f.Package.(string); ok && !truthy(f.Version) {
			if version, ok := installed[name]; ok {
				findings[i].Version = version
			}
		}
	}
	return findings
}
