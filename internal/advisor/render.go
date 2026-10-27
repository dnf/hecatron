package advisor

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Write renders a report: "agent" as one line per finding, "json" for
// scripts, anything else as the human format.
func Write(w io.Writer, r *Report, format string) error {
	switch format {
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(r)
	case "agent":
		_, err := io.WriteString(w, agent(r))
		return err
	}
	_, err := io.WriteString(w, human(r))
	return err
}

func counts(r *Report) string {
	rec, sug := r.Count(Recommended), r.Count(Suggestion)
	if rec+sug == 0 {
		return "no findings"
	}
	return fmt.Sprintf("%s, %s", plural(rec, "recommended", "recommended"), plural(sug, "suggestion", "suggestions"))
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// dirText is a finding's directory, for a heading.
func dirText(f Finding) string {
	if f.Dir == "." {
		return "the repository"
	}
	return f.Dir + "/"
}

// owners are the languages (and components, when there are several) a
// finding is about: "go: age, date"; "" for the repository's.
func owners(f Finding) string {
	text := strings.Join(f.Languages, ", ")
	if len(f.Components) > 0 && !(len(f.Components) == 1 && f.Components[0] == ".") {
		text += ": " + strings.Join(f.Components, ", ")
	}
	return text
}

// where is a finding's directory and owners, on one line.
func where(f Finding) string {
	if o := owners(f); o != "" {
		return dirText(f) + " (" + o + ")"
	}
	return dirText(f)
}

func fixText(f Finding) string {
	if f.Fix == nil {
		return ""
	}
	if f.Fix.Applied {
		return "created " + f.Fix.Create
	}
	return "hct advisor --fix creates " + f.Fix.Create
}

// agent: a summary line, then a line per finding.
func agent(r *Report) string {
	var out strings.Builder
	fmt.Fprintf(&out, "advisor: %s (%s)\n", counts(r), strings.Join(r.Languages, ", "))
	for _, f := range r.Findings {
		fmt.Fprintf(&out, "%s %s %s: %s. Why: %s See %s", f.Level, f.ID, where(f), strings.TrimSuffix(f.Message, "."), f.Why, f.Link)
		if fix := fixText(f); fix != "" {
			out.WriteString(" Fix: " + fix)
		}
		out.WriteString("\n")
	}
	tail(&out, r, " · ")
	return out.String()
}

// human: findings grouped by directory, each with why and where to read
// more.
func human(r *Report) string {
	var out strings.Builder
	config := "config saved"
	if r.Config == "detected" {
		config = "config detected"
	}
	fmt.Fprintf(&out, "hct advisor  %s (%s): %s\n", r.Root, config, counts(r))
	group := ""
	for _, f := range r.Findings {
		if g := dirText(f); g != group {
			group = g
			fmt.Fprintf(&out, "\n%s\n", g)
		}
		about := ""
		if o := owners(f); o != "" {
			about = "  (" + o + ")"
		}
		fmt.Fprintf(&out, "  %-11s  %s  [%s]%s\n", f.Level, f.ID, f.Category, about)
		fmt.Fprintf(&out, "      %s\n", f.Message)
		fmt.Fprintf(&out, "      Why: %s\n", f.Why)
		fmt.Fprintf(&out, "      See: %s\n", f.Link)
		if fix := fixText(f); fix != "" {
			fmt.Fprintf(&out, "      Fix: %s\n", fix)
		}
	}
	if len(r.Findings) > 0 {
		out.WriteString("\n")
	}
	tail(&out, r, "\n")
	return out.String()
}

// tail says which languages have no advice yet, what the config ignores,
// and what --fix changed.
func tail(out *strings.Builder, r *Report, sep string) {
	var lines []string
	if len(r.NoAdvice) > 0 {
		lines = append(lines, "No advice yet for: "+strings.Join(r.NoAdvice, ", "))
	}
	if len(r.Ignored) > 0 {
		var ignored []string
		for _, f := range r.Ignored {
			ignored = append(ignored, fmt.Sprintf("%s in %s (%s)", f.ID, f.Dir, f.Reason))
		}
		lines = append(lines, "Ignored by config: "+strings.Join(ignored, "; "))
	}
	if len(r.Fixed) > 0 {
		lines = append(lines, "Fixed: created "+strings.Join(r.Fixed, ", "))
	} else if fixable(r) > 0 {
		lines = append(lines, fmt.Sprintf("hct advisor --fix would create %s", plural(fixable(r), "file", "files")))
	}
	if len(lines) > 0 {
		out.WriteString(strings.Join(lines, sep) + "\n")
	}
}

func fixable(r *Report) int {
	n := 0
	for _, f := range r.Findings {
		if f.Fix != nil && !f.Fix.Applied {
			n++
		}
	}
	return n
}
