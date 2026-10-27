package licenses

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Write prints the report as human (also for CI), agent or json output.
func Write(w io.Writer, r *Report, format string) error {
	var out strings.Builder
	switch format {
	case "json":
		return writeJSON(w, r)
	case "agent":
		writeAgent(&out, r)
	default:
		writeHuman(&out, r)
	}
	_, err := io.WriteString(w, out.String())
	return err
}

func writeJSON(w io.Writer, r *Report) error {
	out := *r
	if out.Packages == nil {
		out.Packages = []Package{}
	}
	for i := range out.Packages {
		if out.Packages[i].Declared == nil {
			out.Packages[i].Declared = []string{}
		}
	}
	if out.Licenses == nil {
		out.Licenses = []Rollup{}
	}
	if out.NotCovered == nil {
		out.NotCovered = []NotCovered{}
	}
	if out.Warnings == nil {
		out.Warnings = []string{}
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(out)
}

// Summary is the report's first line: packages, licenses, unknowns and the
// policy's verdict.
func (r *Report) Summary() string {
	unknown := len(r.Unknown())
	line := fmt.Sprintf("hct licenses: %s, %s, %d unknown", plural(len(r.Packages), "package"),
		plural(len(r.Licenses), "license"), unknown)
	if r.Policy != nil {
		line += "; policy: " + plural(len(r.Policy.Violations), "violation")
		if len(r.Policy.Accepted) > 0 {
			line += fmt.Sprintf(", %d accepted", len(r.Policy.Accepted))
		}
	}
	return line
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func where(p Package) string { return languageIn(p.Language, p.Component) }

// languageIn is a language and a component, or the language alone in a
// project whose one component is the root (".").
func languageIn(lang, component string) string {
	if component == "." {
		return lang
	}
	return lang + ", " + component
}

func dependencyText(p Package) string {
	parts := []string{}
	if p.Dependency != "" {
		parts = append(parts, p.Dependency)
	}
	if p.Dev {
		parts = append(parts, "dev")
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

func declaredText(p Package) string {
	if len(p.Declared) == 0 {
		return "no license metadata"
	}
	return "declared: " + strings.Join(p.Declared, ", ")
}

func writeAgent(w *strings.Builder, r *Report) {
	fmt.Fprintln(w, r.Summary())
	if r.Policy != nil {
		for _, v := range r.Policy.Violations {
			fmt.Fprintf(w, "VIOLATION %s %s %s: %s (%s)\n", v.Package, v.Version, orUnknown(v.License), v.Reason,
				languageIn(v.Language, v.Component))
		}
		for _, note := range r.Policy.Expired {
			fmt.Fprintln(w, "NOTE "+note)
		}
	}
	for _, p := range r.Unknown() {
		fmt.Fprintf(w, "UNKNOWN %s %s (%s; %s)\n", p.Key(), p.Version, declaredText(p), where(p))
	}
	for _, n := range r.NotCovered {
		fmt.Fprintf(w, "NOT COVERED %s: %s\n", notCoveredWhere(n), n.Reason)
	}
	for _, warning := range r.Warnings {
		fmt.Fprintln(w, "WARNING "+warning)
	}
	if len(r.Licenses) > 0 {
		var parts []string
		for _, l := range r.Licenses {
			parts = append(parts, fmt.Sprintf("%s %d", l.License, l.Packages))
		}
		fmt.Fprintln(w, "Licenses: "+strings.Join(parts, ", "))
	}
	if len(r.Packages) > 0 {
		fmt.Fprintln(w, "Every package: hct licenses --output json")
	}
}

func orUnknown(license string) string {
	if license == "" {
		return "unknown"
	}
	return license
}

func notCoveredWhere(n NotCovered) string {
	if n.Component == "" {
		return n.Language
	}
	return n.Language + " (" + n.Component + ")"
}

// table writes rows with each column but the last padded to its widest.
func table(w *strings.Builder, indent string, rows [][]string) {
	if len(rows) == 0 {
		return
	}
	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, cell := range row {
			if n := utf8.RuneCountInString(cell); n > widths[i] {
				widths[i] = n
			}
		}
	}
	for _, row := range rows {
		var b strings.Builder
		b.WriteString(indent)
		for i, cell := range row {
			if i == len(row)-1 {
				b.WriteString(cell)
				break
			}
			b.WriteString(cell + strings.Repeat(" ", widths[i]-utf8.RuneCountInString(cell)+2))
		}
		fmt.Fprintln(w, strings.TrimRight(b.String(), " "))
	}
}

func writeHuman(w *strings.Builder, r *Report) {
	fmt.Fprintln(w, r.Summary())
	if len(r.Licenses) > 0 {
		fmt.Fprintln(w)
		rows := [][]string{{"license", "packages", "languages", "components"}}
		for _, l := range r.Licenses {
			rows = append(rows, []string{l.License, fmt.Sprint(l.Packages), strings.Join(l.Languages, ", "),
				strings.Join(l.Components, ", ")})
		}
		table(w, "  ", rows)
	}
	if unknown := r.Unknown(); len(unknown) > 0 {
		fmt.Fprintln(w, "\nUnknown licenses (no metadata, or not an SPDX expression: worth a look):")
		var rows [][]string
		for _, p := range unknown {
			rows = append(rows, []string{p.Key(), p.Version, declaredText(p), where(p), dependencyText(p)})
		}
		table(w, "  ", rows)
	}
	if len(r.NotCovered) > 0 {
		fmt.Fprintln(w, "\nNot covered:")
		for _, n := range r.NotCovered {
			fmt.Fprintf(w, "  %s: %s\n", notCoveredWhere(n), n.Reason)
		}
	}
	for i, warning := range r.Warnings {
		if i == 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w, "warning: "+warning)
	}
	if len(r.Packages) > 0 {
		fmt.Fprintln(w, "\nPackages:")
		rows := [][]string{{"package", "version", "license", "dependency", "language, component"}}
		for _, p := range r.Packages {
			rows = append(rows, []string{p.Key(), p.Version, orUnknown(p.License), dependencyText(p), where(p)})
		}
		table(w, "  ", rows)
	}
	if r.Policy == nil {
		return
	}
	counted := "dev and test dependencies not counted"
	if r.Policy.IncludeDev {
		counted = "dev and test dependencies counted"
	}
	fmt.Fprintf(w, "\nPolicy (%s): %s\n", counted, plural(len(r.Policy.Violations), "violation"))
	var rows [][]string
	for _, v := range r.Policy.Violations {
		rows = append(rows, []string{v.Package, v.Version, orUnknown(v.License), v.Reason, languageIn(v.Language, v.Component)})
	}
	table(w, "  ", rows)
	for _, v := range r.Policy.Accepted {
		fmt.Fprintf(w, "  %s %s %s: accepted until %s: %s\n", v.Package, v.Version, orUnknown(v.License), v.Until, v.Reason)
	}
	for _, note := range r.Policy.Expired {
		fmt.Fprintln(w, "  note: "+note)
	}
}
