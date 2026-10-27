package advisor

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dnf/hecatron/internal/project"
)

// usedVersion is the version a component uses and where the project says
// so ("pin", "project", "supports"); kind is "" when the version is only
// hecatron's default (or none), which is no choice of the project's.
type usedVersion struct {
	spec  string // without "="
	kind  string
	where string // the pin or support file, relative to the root
}

func (r *run) usedVersion(lang, component string) (usedVersion, bool) {
	resolved, err := r.p.ResolveVersion(lang, component)
	if err != nil {
		return usedVersion{}, false
	}
	source := resolved.Source
	if inner, ok := strings.CutPrefix(source, "as-of:"); ok {
		// "as-of:2021-03-04 (pin:x/.nvmrc)": judged by what it came from
		if open := strings.Index(inner, "("); open >= 0 {
			source = strings.TrimSuffix(inner[open+1:], ")")
			source, _, _ = strings.Cut(source, ";")
		}
	}
	v := usedVersion{spec: strings.TrimPrefix(resolved.Spec, "=")}
	switch {
	case strings.HasPrefix(source, "pin:"):
		v.kind, v.where = "pin", strings.TrimPrefix(source, "pin:")
	case strings.HasPrefix(source, "supports:"):
		v.kind, v.where = "supports", strings.TrimPrefix(source, "supports:")
	case source == "project":
		v.kind = "project"
	}
	return v, true
}

// describe says where a version comes from, for a message.
func (v usedVersion) describe() string {
	switch v.kind {
	case "pin":
		return "pinned in " + v.where
	case "supports":
		return "the newest " + v.where + " supports"
	case "project":
		return "the project's version in .hecatron"
	}
	return "hecatron's default"
}

// toolchain runs the pin, end-of-life and behind rules of a set for one
// component.
func (r *run) toolchain(set *Set, lang string, d project.ComponentDir) error {
	if set.Pin == nil && set.EndOfLife == nil && set.Behind == nil {
		return nil
	}
	v, ok := r.usedVersion(lang, d.Component)
	if !ok {
		return nil
	}
	name := set.Toolchain
	if name == "" {
		name = r.name(lang)
	}
	if pin := set.Pin; pin != nil && v.kind != "pin" && v.kind != "project" && r.files.eval(pin.When, d.Dir) {
		f := Finding{ID: set.name + "/" + idPinned, Level: pin.Level, Category: "reproducibility",
			Message: fmt.Sprintf("Pin %s's version in %s", name, pin.Create), Why: pin.Why, Link: pin.Link}
		f = r.at(f, d.Dir)
		if v.spec != "" {
			f.Message += fmt.Sprintf(" (it runs %s now, %s)", v.spec, v.describe())
		}
		if v.spec != "" && len(strings.Split(v.spec, ".")) >= pin.MinParts {
			content := strings.ReplaceAll(pin.Content, "{version}", v.spec)
			content = strings.ReplaceAll(content, "{line}", line(v.spec, pin.LineParts))
			f.Fix = &Fix{Create: joinRel(f.Dir, pin.Create), Content: content}
		}
		r.add(lang, d.Component, f)
	}
	if v.kind == "" || v.spec == "" {
		return nil
	}
	releases, err := r.p.Releases(lang)
	if err != nil || len(releases) == 0 {
		return nil
	}
	if eol := set.EndOfLife; eol != nil {
		if past, newest := endOfLife(eol, releases, v.spec, r.opts.Now); past {
			f := Finding{ID: set.name + "/" + idEndOfLife, Level: Recommended, Category: "currency",
				Message: fmt.Sprintf("Move off %s %s (%s): that line is past its end of life; the newest is %s",
					name, line(v.spec, eol.LineParts), v.describe(), newest),
				Why: eol.Why, Link: eol.Link}
			r.add(lang, d.Component, r.at(f, d.Dir))
		}
	}
	if b := set.Behind; b != nil {
		if behind, newest := linesBehind(releases, v.spec, b.LineParts); behind > b.Lines {
			f := Finding{ID: set.name + "/" + idBehind, Level: Suggestion, Category: "currency",
				Message: fmt.Sprintf("Update %s %s (%s): it is %d releases behind %s",
					name, line(v.spec, b.LineParts), v.describe(), behind, newest),
				Why: b.Why, Link: b.Link}
			r.add(lang, d.Component, r.at(f, d.Dir))
		}
	}
	return nil
}

// releaseLine is one line's releases, from the table.
type releaseLine struct {
	key   []int
	name  string
	start string // its first release's date
	lts   bool
	first bool // it holds the table's first row: its start may be older
}

// lines groups a table's releases by their first parts, oldest line first.
func lines(releases []project.Release, parts int) []releaseLine {
	var result []releaseLine
	index := map[string]int{}
	for i, rel := range releases {
		name := line(rel.Version, parts)
		key, ok := numbers(name)
		if !ok {
			continue
		}
		at, seen := index[name]
		if !seen {
			at = len(result)
			index[name] = at
			result = append(result, releaseLine{key: key, name: name, start: rel.Date, first: i == 0})
		}
		l := &result[at]
		if rel.Date < l.start {
			l.start = rel.Date
		}
		l.lts = l.lts || rel.LTS
	}
	return result
}

// endOfLife is whether a version's line is past its end of life, and the
// newest line.
func endOfLife(eol *EndOfLife, releases []project.Release, version string, now time.Time) (bool, string) {
	all := lines(releases, eol.LineParts)
	target, ok := find(all, line(version, eol.LineParts))
	if !ok {
		return false, ""
	}
	newer, newest := newerLines(all, target)
	maxLines, months := eol.Lines, eol.Months
	if target.lts {
		maxLines, months = eol.LTSLines, eol.LTSMonths
	}
	if maxLines > 0 && newer >= maxLines {
		return true, newest
	}
	if months > 0 && !target.first {
		start, err := time.Parse("2006-01-02", target.start)
		if err == nil && now.After(start.AddDate(0, months, 0)) {
			return true, newest
		}
	}
	return false, newest
}

// linesBehind is how many lines are newer than the version's, and the
// newest line.
func linesBehind(releases []project.Release, version string, parts int) (int, string) {
	all := lines(releases, parts)
	target, ok := find(all, line(version, parts))
	if !ok {
		return 0, ""
	}
	return newerLines(all, target)
}

func find(all []releaseLine, name string) (releaseLine, bool) {
	key, ok := numbers(name)
	if !ok {
		return releaseLine{}, false
	}
	for _, l := range all {
		if compare(l.key, key) == 0 {
			return l, true
		}
	}
	return releaseLine{}, false
}

func newerLines(all []releaseLine, target releaseLine) (int, string) {
	count, newest := 0, target
	for _, l := range all {
		if compare(l.key, target.key) > 0 {
			count++
		}
		if compare(l.key, newest.key) > 0 {
			newest = l
		}
	}
	return count, newest.name
}

// line is a version's first parts ("3.12.4", 2: "3.12"); a shorter
// version is its own line.
func line(version string, parts int) string {
	fields := strings.Split(version, ".")
	if parts > 0 && len(fields) > parts {
		fields = fields[:parts]
	}
	return strings.Join(fields, ".")
}

func numbers(version string) ([]int, bool) {
	var result []int
	for _, field := range strings.Split(version, ".") {
		n, err := strconv.Atoi(field)
		if err != nil {
			return nil, false
		}
		result = append(result, n)
	}
	return result, true
}

func compare(a, b []int) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		x, y := 0, 0
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

func joinRel(dir, file string) string {
	if dir == "." {
		return file
	}
	return dir + "/" + file
}
