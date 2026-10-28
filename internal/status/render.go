package status

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

// Write renders the status: "agent" as a few dense lines, "json" for
// scripts, anything else as the human format.
func Write(w io.Writer, s *Status, format string) error {
	switch format {
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(s)
	case "agent":
		_, err := io.WriteString(w, agent(s))
		return err
	}
	_, err := io.WriteString(w, human(s))
	return err
}

func header(s *Status) string {
	config := "config saved"
	if s.Config == "detected" {
		config = "config detected, not saved (hct init saves it)"
	}
	return fmt.Sprintf("hecatron %s (%s%s) · %s · %s", s.Version, s.Runner, updateText(s), s.Root, config)
}

// agent is one line per language, in columns, then what to run next.
func agent(s *Status) string {
	rows := [][]string{}
	for _, l := range s.Languages {
		name := l.Name
		if l.Disabled {
			name += " (disabled)"
		}
		row := []string{name, dirList(l), versionText(l), toolchainText(l), runsText(l, false)}
		if len(l.Changes) > 0 {
			row = append(row, plural(len(l.Changes), "file")+" changed")
		}
		rows = append(rows, row)
	}
	var out strings.Builder
	out.WriteString(header(s) + "\n")
	out.WriteString(columns(rows))
	if len(s.OtherChanges) > 0 {
		fmt.Fprintf(&out, "Other changes: %s outside the languages' directories\n", plural(len(s.OtherChanges), "file"))
	}
	var next []string
	for _, n := range s.Next {
		why := n.Why
		if n.AskUser {
			why += "; ask the user first"
		}
		next = append(next, fmt.Sprintf("%s (%s)", n.Command, why))
	}
	out.WriteString("Next: " + strings.Join(next, " · ") + "\n")
	return out.String()
}

// human is the same, a block per language.
func human(s *Status) string {
	var out strings.Builder
	fmt.Fprintf(&out, "hecatron %s (%s%s)\n", s.Version, s.Runner, updateText(s))
	config := "config saved in .hecatron/config.json"
	if s.Config == "detected" {
		config = "config detected, not saved: hct init saves it"
	}
	fmt.Fprintf(&out, "Project  %s (%s)\n", s.Root, config)
	if len(s.Languages) == 0 {
		out.WriteString("\nNo languages.\n")
	}
	for _, l := range s.Languages {
		name := l.Name
		if l.Disabled {
			name += " (disabled for you in parallel runs)"
		}
		fmt.Fprintf(&out, "\n%s\n", name)
		fmt.Fprintf(&out, "  directories  %s\n", dirList(l))
		fmt.Fprintf(&out, "  version      %s\n", versionText(l))
		fmt.Fprintf(&out, "  toolchain    %s\n", toolchainText(l))
		fmt.Fprintf(&out, "  last runs    %s\n", runsText(l, true))
		if len(l.Changes) > 0 {
			fmt.Fprintf(&out, "  changes      %s: %s\n", plural(len(l.Changes), "file"), shortList(l.Changes))
		}
	}
	if len(s.OtherChanges) > 0 {
		fmt.Fprintf(&out, "\nOther changes: %s\n", shortList(s.OtherChanges))
	}
	if !s.Git {
		out.WriteString("\nNot in a git repository: changes unknown.\n")
	}
	out.WriteString("\nNext:\n")
	for _, n := range s.Next {
		why := n.Why
		if n.AskUser {
			why += " (ask the user first)"
		}
		fmt.Fprintf(&out, "  %-24s %s\n", n.Command, why)
	}
	return out.String()
}

func dirList(l Language) string {
	if len(l.Components) == 0 {
		return "(no directory found)"
	}
	var dirs []string
	for _, c := range l.Components {
		dir := c.Dir
		if dir != "." {
			dir += "/"
		}
		dirs = append(dirs, dir)
	}
	return strings.Join(dirs, ", ")
}

// versionText is each version and where it comes from.
func versionText(l Language) string {
	if l.VersionError != "" {
		return "? (" + l.VersionError + ")"
	}
	var parts []string
	for _, v := range l.Versions {
		var sources []string
		for _, source := range v.Sources {
			sources = append(sources, sourceText(source, v.Spec == ""))
		}
		switch {
		case v.Spec == "" && len(sources) == 0:
			continue
		case v.Spec == "":
			parts = append(parts, "any ("+strings.Join(sources, ", ")+")")
		default:
			parts = append(parts, v.Spec+" ("+strings.Join(sources, ", ")+")")
		}
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

// sourceText says where a version comes from in words.
func sourceText(source string, noVersion bool) string {
	switch {
	case strings.HasPrefix(source, "pin:"):
		if noVersion {
			return strings.TrimPrefix(source, "pin:") + " names none"
		}
		return strings.TrimPrefix(source, "pin:")
	case source == "project":
		return "project config"
	case strings.HasPrefix(source, "supports:"):
		return "newest " + strings.TrimPrefix(source, "supports:") + " supports"
	case strings.HasPrefix(source, "as-of:"):
		when, _, _ := strings.Cut(strings.TrimPrefix(source, "as-of:"), " ")
		return "as of " + when + ", the last commit's date"
	}
	return source
}

func toolchainText(l Language) string {
	switch l.Toolchain {
	case "installed":
		return "installed"
	case "missing":
		return "missing (" + l.Dependency + ")"
	case "partly":
		var missing []string
		for _, c := range l.Components {
			if !c.Toolchain {
				missing = append(missing, c.Name)
			}
		}
		return "missing in " + strings.Join(missing, ", ") + " (" + l.Dependency + ")"
	}
	return "-"
}

// runsText is the last run of each action that has one, test first, and
// "never run" for test.
func runsText(l Language, long bool) string {
	var parts []string
	for _, action := range Actions {
		run, ok := l.Runs[action]
		if !ok {
			if action == "test" {
				parts = append(parts, "test: never run")
			}
			continue
		}
		text := fmt.Sprintf("%s: %s %s", action, run.Status, ago(run.ago))
		if run.Duration != nil {
			text += ", " + duration(time.Duration(*run.Duration*float64(time.Second)))
		}
		if long && run.Status == "failed" {
			text += " (log " + run.Log + ")"
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "; ")
}

func ago(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d/time.Minute))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d/time.Hour))
	}
	return fmt.Sprintf("%dd ago", int(d/(24*time.Hour)))
}

func duration(d time.Duration) string {
	seconds := int(d.Round(time.Second) / time.Second)
	switch {
	case seconds < 60:
		return fmt.Sprintf("%ds", seconds)
	case seconds < 3600:
		return fmt.Sprintf("%dm %02ds", seconds/60, seconds%60)
	}
	return fmt.Sprintf("%dh %02dm", seconds/3600, seconds%3600/60)
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// shortList is the first few files, and how many more.
func shortList(files []string) string {
	const shown = 5
	if len(files) <= shown {
		return strings.Join(files, ", ")
	}
	return strings.Join(files[:shown], ", ") + fmt.Sprintf(", and %d more", len(files)-shown)
}

// columns aligns rows, each column as wide as its widest cell; the last
// cell isn't padded.
func columns(rows [][]string) string {
	var widths []int
	for _, row := range rows {
		for i, cell := range row {
			if i >= len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], utf8.RuneCountInString(cell))
		}
	}
	var out strings.Builder
	for _, row := range rows {
		for i, cell := range row {
			if i == len(row)-1 {
				out.WriteString(cell)
				break
			}
			out.WriteString(cell + strings.Repeat(" ", widths[i]-utf8.RuneCountInString(cell)+2))
		}
		out.WriteString("\n")
	}
	return out.String()
}

// updateText is the newer release known, after the runner in the header.
func updateText(s *Status) string {
	if s.Update == nil {
		return ""
	}
	return "; " + s.Update.Latest + " is available: " + s.Update.Command
}
