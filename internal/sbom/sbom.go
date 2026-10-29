// Package sbom is hct sbom (docs/guide/sbom.md): a software bill of
// materials for the whole project, as CycloneDX 1.6 or SPDX 2.3 JSON, from
// the packages hct licenses gathers (internal/licenses): each component of
// the project, the packages it uses with their purls and SPDX licenses,
// and which it depends on directly.
package sbom

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dnf/hecatron/internal/licenses"
)

// Formats are the formats Write writes, the first the default.
var Formats = []string{"cyclonedx", "spdx"}

// Component is one component of the project and the languages it has.
type Component struct {
	Name      string
	Languages []string
}

// Input is what a bill of materials says.
type Input struct {
	Project     string
	Commit      string // the repository's HEAD; "" outside git
	ToolVersion string // hecatron's
	Time        time.Time
	Components  []Component
	Packages    []licenses.Package // one per package, language and component
	NoDev       bool               // leave out dev and test dependencies
}

// Write is the bill of materials in format, indented JSON.
func Write(in Input, format string) ([]byte, error) {
	m := build(in)
	switch format {
	case "cyclonedx":
		return m.cyclonedx(), nil
	case "spdx":
		return m.spdx(), nil
	}
	return nil, fmt.Errorf("unknown SBOM format %q (formats: %s)", format, strings.Join(Formats, ", "))
}

// pkg is one package (an ecosystem's name and version) and every use of it.
type pkg struct {
	purl, ecosystem, name, version string
	license                        string   // a canonical SPDX expression; "" when unknown
	declared                       []string // what the registry or lock file says, when that isn't one
	dependency                     string   // direct if any use is, transitive if every one is, else ""
	dev                            bool     // a dev or test dependency in every use
	components, languages, sources []string
}

// use is a component's use of a package.
type use struct {
	purl, dependency string
	dev              bool
}

type model struct {
	in       Input
	packages []*pkg
	uses     map[string][]use // by component
}

func build(in Input) *model {
	m := &model{in: in, uses: map[string][]use{}}
	byPurl := map[string]*pkg{}
	kinds := map[string]map[string]bool{}
	for _, p := range in.Packages {
		if in.NoDev && p.Dev {
			continue
		}
		purl := Purl(p.Ecosystem, p.Name, p.Version)
		entry := byPurl[purl]
		if entry == nil {
			entry = &pkg{purl: purl, ecosystem: p.Ecosystem, name: p.Name, version: p.Version, dev: true}
			byPurl[purl] = entry
			kinds[purl] = map[string]bool{}
			m.packages = append(m.packages, entry)
		}
		if entry.license == "" && p.License != "" {
			entry.license, entry.declared = p.License, nil
		}
		if entry.license == "" && len(entry.declared) == 0 {
			entry.declared = p.Declared
		}
		entry.dev = entry.dev && p.Dev
		kinds[purl][p.Dependency] = true
		entry.components = addSorted(entry.components, p.Component)
		entry.languages = addSorted(entry.languages, p.Language)
		entry.sources = addSorted(entry.sources, p.Source)
		m.uses[p.Component] = append(m.uses[p.Component], use{purl, p.Dependency, p.Dev})
	}
	for _, entry := range m.packages {
		switch k := kinds[entry.purl]; {
		case k["direct"]:
			entry.dependency = "direct"
		case k["transitive"] && !k[""]:
			entry.dependency = "transitive"
		}
	}
	sort.Slice(m.packages, func(i, j int) bool { return m.packages[i].purl < m.packages[j].purl })
	for component, uses := range m.uses {
		sort.Slice(uses, func(i, j int) bool { return uses[i].purl < uses[j].purl })
		m.uses[component] = uses
	}
	return m
}

func addSorted(list []string, s string) []string {
	i := sort.SearchStrings(list, s)
	if i < len(list) && list[i] == s {
		return list
	}
	return append(list[:i], append([]string{s}, list[i:]...)...)
}

// timestamp is the document's time as both formats want it: UTC, to the
// second.
func (m *model) timestamp() string { return m.in.Time.UTC().Format("2006-01-02T15:04:05Z") }

// uuid is a name-based (version 5) UUID of the document's content: the
// same bill of materials (time included, so SOURCE_DATE_EPOCH makes it
// reproducible) has the same serial number, any other a new one.
func uuid(content []byte) string {
	namespace := []byte{0x6c, 0x1f, 0x0e, 0x5a, 0x93, 0x2b, 0x4d, 0x8e, 0xa1, 0x7c, 0x52, 0xd4, 0x0b, 0x3e, 0x9f, 0x61}
	sum := sha1.Sum(append(namespace, content...))
	b := sum[:16]
	b[6] = b[6]&0x0f | 0x50
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func marshal(document any) []byte {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(document) // maps, slices and strings only: can't fail
	return out.Bytes()
}

// Purl is a package's package URL (github.com/package-url/purl-spec) from
// OSV-Scanner's ecosystem, name and version.
func Purl(ecosystem, name, version string) string {
	kind, namespace := "generic", ""
	switch ecosystem {
	case "npm":
		kind = "npm"
		if strings.HasPrefix(name, "@") {
			namespace, name = splitLast(name, "/")
		}
	case "PyPI":
		kind, name = "pypi", strings.ReplaceAll(strings.ToLower(name), "_", "-")
	case "Go":
		kind = "golang"
		namespace, name = splitLast(name, "/")
	case "crates.io":
		kind = "cargo"
	case "Maven":
		kind = "maven"
		namespace, name = splitLast(name, ":")
	case "NuGet":
		kind = "nuget"
	case "RubyGems":
		kind = "gem"
	case "Packagist":
		kind = "composer"
		namespace, name = splitLast(strings.ToLower(name), "/")
	case "Hex":
		kind, name = "hex", strings.ToLower(name)
	case "Pub":
		kind, name = "pub", strings.ToLower(name)
	case "SwiftURL":
		kind = "swift"
		namespace, name = splitLast(strings.TrimPrefix(strings.TrimPrefix(name, "https://"), "http://"), "/")
	case "CRAN":
		kind = "cran"
	case "Hackage":
		kind = "hackage"
	case "ConanCenter":
		kind = "conan"
	}
	purl := "pkg:" + kind + "/"
	if namespace != "" {
		var segments []string
		for _, segment := range strings.Split(namespace, "/") {
			segments = append(segments, escape(segment))
		}
		purl += strings.Join(segments, "/") + "/"
	}
	purl += escape(name)
	if version != "" {
		purl += "@" + escape(version)
	}
	return purl
}

func splitLast(s, sep string) (string, string) {
	if i := strings.LastIndex(s, sep); i >= 0 {
		return s[:i], s[i+len(sep):]
	}
	return "", s
}

// escape percent-encodes all but the characters a purl's segments may
// hold as they are.
func escape(s string) string {
	var out strings.Builder
	for _, b := range []byte(s) {
		switch {
		case 'a' <= b && b <= 'z', 'A' <= b && b <= 'Z', '0' <= b && b <= '9', b == '.', b == '-', b == '_', b == '~':
			out.WriteByte(b)
		default:
			fmt.Fprintf(&out, "%%%02X", b)
		}
	}
	return out.String()
}
