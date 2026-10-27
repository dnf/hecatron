package deps

import (
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/dnf/hecatron/internal/project"
)

// Row is one outdated dependency: its current version, the newest its
// manifest's constraints allow where the tool says (wanted), and the
// newest (latest).
type Row struct {
	Package                 any
	Current, Wanted, Latest string
}

func newRow(pkg, current, latest, wanted any) Row {
	return Row{Package: pkg, Current: bare(current), Wanted: bare(wanted), Latest: bare(latest)}
}

// Outdated says whether the tool names a newer version than the current.
func (r Row) Outdated() bool { return r.Latest != "" && r.Latest != r.Current }

var leadingV = project.PyRegexp(`^v\d`)

// bare is a version without the v some tools put in front (v1.2.3).
func bare(version any) string {
	text := strings.TrimFunc(str(or(version, "")), project.PyIsSpace)
	if leadingV.MatchString(text) {
		_, size := utf8.DecodeRuneInString(text)
		return text[size:]
	}
	return text
}

// object is outdated.py's _object: the report, when it's a JSON object; an
// empty report is nothing outdated (npm and pnpm print nothing then).
func object(text string) (*Object, error) {
	if strings.TrimFunc(text, project.PyIsSpace) == "" {
		return newObject(), nil
	}
	data, err := jsonLoads(text)
	if err != nil {
		return nil, err
	}
	o, ok := data.(*Object)
	if !ok {
		return nil, errorf("not an outdated report")
	}
	return o, nil
}

// parseNPMOutdated reads npm outdated --json and pnpm outdated --format
// json: by package name. An entry can be a list when several copies are
// outdated.
func parseNPMOutdated(text string, _ string) ([]Row, error) {
	data, err := object(text)
	if err != nil {
		return nil, err
	}
	var rows []Row
	for _, name := range data.Keys {
		entries, ok := data.Values[name].([]any)
		if !ok {
			entries = []any{data.Values[name]}
		}
		for _, entry := range entries {
			current, err := get(entry, "current", nil)
			if err != nil {
				return nil, err
			}
			e := entry.(*Object)
			rows = append(rows, newRow(name, or(current, ""), or(e.Values["latest"], ""), or(e.Values["wanted"], "")))
		}
	}
	return rows, nil
}

// parseYarnClassicOutdated reads yarn outdated --json (Yarn 1): JSON lines,
// one a table.
func parseYarnClassicOutdated(text string, _ string) ([]Row, error) {
	var rows []Row
	for _, line := range project.PySplitLines(text) {
		value, err := jsonLoads(line)
		if err != nil {
			continue
		}
		o, ok := value.(*Object)
		if !ok || o.Values["type"] != "table" {
			continue
		}
		data, err := item(o, "data")
		if err != nil {
			return nil, err
		}
		headValue, err := item(data, "head")
		if err != nil {
			return nil, err
		}
		bodyValue, err := item(data, "body")
		if err != nil {
			return nil, err
		}
		head, err := iterate(headValue)
		if err != nil {
			return nil, err
		}
		bodies, err := iterate(bodyValue)
		if err != nil {
			return nil, err
		}
		for _, bodyItem := range bodies {
			body, err := iterate(bodyItem)
			if err != nil {
				return nil, err
			}
			entry := newObject()
			for i := 0; i < len(head) && i < len(body); i++ {
				if _, err := hashKey(head[i]); err != nil {
					return nil, err
				}
				if key, ok := head[i].(string); ok {
					entry.set(key, body[i])
				}
			}
			rows = append(rows, newRow(getOr(entry, "Package", ""), getOr(entry, "Current", ""),
				getOr(entry, "Latest", ""), getOr(entry, "Wanted", "")))
		}
	}
	return rows, nil
}

// Python's \s and \w are spelled out inside brackets, where PyRegexp
// leaves them.
var uvTree = project.PyRegexp(`── (\S+) v(\S+).*\(latest: v?([^)\t\n\x0b\x0c\r \x1c-\x1f\x{85}\p{Z}]+)\)`)

// parseUVTree reads uv tree --outdated --depth 1: `├── six v1.15.0
// (latest: v1.17.0)`.
func parseUVTree(text string, _ string) ([]Row, error) {
	var rows []Row
	for _, line := range project.PySplitLines(text) {
		if m := uvTree.FindStringSubmatch(line); m != nil {
			rows = append(rows, newRow(m[1], m[2], m[3], ""))
		}
	}
	return rows, nil
}

// parseUVPip reads uv pip list --outdated --format json.
func parseUVPip(text string, _ string) ([]Row, error) {
	if text == "" {
		text = "[]"
	}
	data, err := jsonLoads(text)
	if err != nil {
		return nil, err
	}
	list, ok := data.([]any)
	if !ok {
		return nil, errorf("not an outdated report")
	}
	var rows []Row
	for _, p := range list {
		name, err := item(p, "name")
		if err != nil {
			return nil, err
		}
		o := p.(*Object)
		rows = append(rows, newRow(name, getOr(o, "version", ""), getOr(o, "latest_version", ""), ""))
	}
	return rows, nil
}

// parseGoOutdated reads go list -u -m -json all: the main module's direct
// requirements with an update.
func parseGoOutdated(text string, _ string) ([]Row, error) {
	modules, err := jsonValues(text)
	if err != nil {
		return nil, err
	}
	var rows []Row
	for _, module := range modules {
		main, err := get(module, "Main", nil)
		if err != nil {
			return nil, err
		}
		m := module.(*Object)
		if truthy(main) || truthy(m.Values["Indirect"]) || !truthy(m.Values["Update"]) {
			continue
		}
		path, err := item(m, "Path")
		if err != nil {
			return nil, err
		}
		update, err := get(m.Values["Update"], "Version", "")
		if err != nil {
			return nil, err
		}
		rows = append(rows, newRow(path, getOr(m, "Version", ""), update, ""))
	}
	return rows, nil
}

var cargoLine = project.PyRegexp(`^\s*(Unchanged|Updating|Adding|Locking)\s+(\S+)\s+v(\S+)(?:\s+->\s+v(\S+))?(?:.*\(available: v([^)]+)\))?`)

// parseCargo reads cargo update --dry-run --verbose: `Unchanged itoa
// v1.0.1 (available: v1.0.18)`, or `Updating serde v1.0.1 -> v1.0.9
// (available: v2.0.0)` when the lock is behind its requirements. Only the
// direct dependencies count, as for the other ecosystems: direct is a
// `cargo metadata --no-deps` report naming them.
func parseCargo(text string, direct string) ([]Row, error) {
	names, err := cargoDirect(direct)
	if err != nil {
		return nil, err
	}
	var rows []Row
	for _, line := range project.PySplitLines(text) {
		m := cargoLine.FindStringSubmatchIndex(line)
		if m == nil {
			continue
		}
		group := func(i int) string {
			if m[2*i] < 0 {
				return ""
			}
			return line[m[2*i]:m[2*i+1]]
		}
		if names != nil && !names[group(2)] {
			continue
		}
		name, current, wanted, available := group(2), group(3), group(4), group(5)
		if wanted != "" || available != "" {
			latest := available
			if latest == "" {
				latest = wanted
			}
			rows = append(rows, newRow(name, current, latest, wanted))
		}
	}
	return rows, nil
}

func cargoDirect(path string) (map[string]bool, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errorf("[Errno 2] No such file or directory: %s", project.PyStringRepr(path))
	}
	if !utf8.Valid(data) {
		return nil, errorf("'utf-8' codec can't decode the direct dependencies' file")
	}
	metadata, err := jsonLoads(project.PyText(data))
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	packages, err := get(metadata, "packages", []any{})
	if err != nil {
		return nil, err
	}
	list, err := iterate(packages)
	if err != nil {
		return nil, err
	}
	for _, pkg := range list {
		deps, err := get(pkg, "dependencies", []any{})
		if err != nil {
			return nil, err
		}
		depList, err := iterate(deps)
		if err != nil {
			return nil, err
		}
		for _, dep := range depList {
			name, err := item(dep, "name")
			if err != nil {
				return nil, err
			}
			if _, err := hashKey(name); err != nil {
				return nil, err
			}
			if s, ok := name.(string); ok {
				names[s] = true
			}
		}
	}
	return names, nil
}

var bundlerLine = project.PyRegexp(`^(\S+) \(newest ([^,]+), installed ([^,)]+)`)

// parseBundler reads bundle outdated --parseable: `rack (newest 3.1.8,
// installed 2.2.3, requested = 2.2.3)`.
func parseBundler(text string, _ string) ([]Row, error) {
	var rows []Row
	for _, line := range project.PySplitLines(text) {
		if m := bundlerLine.FindStringSubmatch(line); m != nil {
			rows = append(rows, newRow(m[1], m[3], m[2], ""))
		}
	}
	return rows, nil
}

// parseComposerOutdated reads composer outdated --format=json: the
// installed packages, or with --locked, the lock file's.
func parseComposerOutdated(text string, _ string) ([]Row, error) {
	data, err := object(text)
	if err != nil {
		return nil, err
	}
	list, err := iterate(or(or(data.Values["installed"], data.Values["locked"]), []any{}))
	if err != nil {
		return nil, err
	}
	var rows []Row
	for _, p := range list {
		status, err := get(p, "latest-status", nil)
		if err != nil {
			return nil, err
		}
		if status == "up-to-date" {
			continue
		}
		name, err := item(p, "name")
		if err != nil {
			return nil, err
		}
		o := p.(*Object)
		rows = append(rows, newRow(name, getOr(o, "version", ""), getOr(o, "latest", ""), ""))
	}
	return rows, nil
}

// parseDotnetOutdated reads dotnet list package --outdated --format json:
// each project's top-level packages, once each.
func parseDotnetOutdated(text string, _ string) ([]Row, error) {
	data, err := object(text)
	if err != nil {
		return nil, err
	}
	rows := newObject()
	byKey := map[string]Row{}
	err = eachIn(data, "projects", func(p any) error {
		return eachIn(p, "frameworks", func(framework any) error {
			return eachIn(framework, "topLevelPackages", func(pkg any) error {
				id, err := item(pkg, "id")
				if err != nil {
					return err
				}
				key, err := hashKey(id)
				if err != nil {
					return err
				}
				o := pkg.(*Object)
				if _, seen := byKey[key]; !seen {
					rows.set(key, nil)
					byKey[key] = newRow(id, getOr(o, "resolvedVersion", ""), getOr(o, "latestVersion", ""), "")
				}
				return nil
			})
		})
	})
	if err != nil {
		return nil, err
	}
	var list []Row
	for _, key := range rows.Keys {
		list = append(list, byKey[key])
	}
	return list, nil
}

var mixColumns = project.PyRegexp(`\s{2,}`)

// parseMix reads mix hex.outdated: a table of Dependency, Only, Current,
// Latest and Status ("Update possible" when the requirement allows the
// latest).
func parseMix(text string, _ string) ([]Row, error) {
	var rows []Row
	inside := false
	for _, line := range project.PySplitLines(text) {
		if strings.HasPrefix(line, "Dependency") {
			inside = true
			continue
		}
		parts := mixColumns.Split(strings.TrimFunc(line, project.PyIsSpace), -1)
		if !inside || len(parts) < 4 {
			continue
		}
		n := len(parts)
		name, current, latest, status := parts[0], parts[n-3], parts[n-2], strings.ToLower(parts[n-1])
		if strings.HasPrefix(status, "up-to-date") {
			continue
		}
		wanted := ""
		if strings.HasPrefix(status, "update possible") {
			wanted = latest
		}
		rows = append(rows, newRow(name, current, latest, wanted))
	}
	return rows, nil
}

// parseDart reads dart pub outdated --json: the direct and dev
// dependencies; wanted is pub's "upgradable".
func parseDart(text string, _ string) ([]Row, error) {
	data, err := object(text)
	if err != nil {
		return nil, err
	}
	var rows []Row
	err = eachIn(data, "packages", func(p any) error {
		kind, err := get(p, "kind", nil)
		if err != nil {
			return err
		}
		if kind == "transitive" {
			return nil
		}
		name, err := item(p, "package")
		if err != nil {
			return err
		}
		o := p.(*Object)
		version := func(key string) (any, error) {
			return get(or(o.Values[key], newObject()), "version", "")
		}
		var versions [3]any
		for i, key := range []string{"current", "latest", "upgradable"} {
			if versions[i], err = version(key); err != nil {
				return err
			}
		}
		rows = append(rows, newRow(name, versions[0], versions[1], versions[2]))
		return nil
	})
	return rows, err
}

var (
	mavenLine     = project.PyRegexp(`^(?:\[INFO\])?\s+([\p{L}\p{N}_.\-]+:[\p{L}\p{N}_.\-]+)\s*\.*\s*(?:(\S+)\s+->\s+(\S+))?\s*$`)
	mavenVersions = project.PyRegexp(`^(?:\[INFO\])?\s+(\S+)\s+->\s+(\S+)\s*$`)
)

// parseMaven reads versions:display-dependency-updates: `group:artifact
// ..... 3.12.0 -> 3.17.0`, a long name's versions on the next line.
func parseMaven(text string, _ string) ([]Row, error) {
	var rows []Row
	pending := ""
	for _, line := range project.PySplitLines(text) {
		m := mavenLine.FindStringSubmatch(line)
		switch {
		case m != nil && m[2] != "":
			rows = append(rows, newRow(m[1], m[2], m[3], ""))
			pending = ""
		case m != nil:
			pending = m[1]
		case pending != "":
			if v := mavenVersions.FindStringSubmatch(line); v != nil {
				rows = append(rows, newRow(pending, v[1], v[2], ""))
			}
			pending = ""
		}
	}
	return rows, nil
}

// parseGradle reads the Gradle Versions plugin's report.json
// (dependencyUpdates).
func parseGradle(text string, _ string) ([]Row, error) {
	data, err := object(text)
	if err != nil {
		return nil, err
	}
	var rows []Row
	err = eachIn(or(data.Values["outdated"], newObject()), "dependencies", func(dep any) error {
		available, err := get(dep, "available", nil)
		if err != nil {
			return err
		}
		available = or(available, newObject())
		var latest any = ""
		for _, key := range []string{"release", "milestone", "integration"} {
			value, err := get(available, key, nil)
			if err != nil {
				return err
			}
			if truthy(value) {
				latest = value
				break
			}
		}
		o := dep.(*Object)
		name := str(o.Values["group"]) + ":" + str(o.Values["name"])
		rows = append(rows, newRow(name, getOr(o, "version", ""), latest, ""))
		return nil
	})
	return rows, err
}

var cabalLine = project.PyRegexp(`^(\S+) (.+?) \(latest: ([^)]+)\)`)

// parseCabal reads cabal outdated: `base >=4 && <4.15 (latest:
// 4.18.0.0)`; the current is the constraint, which cabal reports against.
func parseCabal(text string, _ string) ([]Row, error) {
	var rows []Row
	for _, line := range project.PySplitLines(text) {
		if m := cabalLine.FindStringSubmatch(line); m != nil {
			rows = append(rows, newRow(m[1], m[2], m[3], ""))
		}
	}
	return rows, nil
}

var outdatedParsers = map[string]func(string, string) ([]Row, error){
	"npm":          parseNPMOutdated,
	"pnpm":         parseNPMOutdated,
	"yarn-classic": parseYarnClassicOutdated,
	"uv-tree":      parseUVTree,
	"uv-pip":       parseUVPip,
	"go":           parseGoOutdated,
	"cargo":        parseCargo,
	"bundler":      parseBundler,
	"composer":     parseComposerOutdated,
	"dotnet":       parseDotnetOutdated,
	"mix":          parseMix,
	"dart":         parseDart,
	"maven":        parseMaven,
	"gradle":       parseGradle,
	"cabal":        parseCabal,
}

// IsOutdatedFormat says whether ParseOutdated reads a format.
func IsOutdatedFormat(format string) bool { _, ok := outdatedParsers[format]; return ok }

// ParseOutdated reads a tool's report into its outdated rows, each
// (package, current) once, by package name ignoring case. direct is the
// file naming the direct dependencies, for cargo ("" for none).
func ParseOutdated(format, text, direct string) ([]Row, error) {
	parser, ok := outdatedParsers[format]
	if !ok {
		return nil, fmt.Errorf("unknown outdated format %q", format)
	}
	rows, err := parser(text, direct)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var unique []Row
	for _, row := range rows {
		if !row.Outdated() {
			continue
		}
		key, err := hashKey(row.Package)
		if err != nil {
			return nil, err
		}
		if key += "\x00" + row.Current; !seen[key] {
			seen[key] = true
			unique = append(unique, row)
		}
	}
	for _, row := range unique {
		if _, ok := row.Package.(string); !ok {
			return nil, errorf("'%s' object has no attribute 'lower'", typeName(row.Package))
		}
	}
	sort.SliceStable(unique, func(i, j int) bool {
		return strings.ToLower(unique[i].Package.(string)) < strings.ToLower(unique[j].Package.(string))
	})
	return unique, nil
}

// Table is the rows as outdated prints them, under a header.
func Table(rows []Row) []string {
	if len(rows) == 0 {
		return nil
	}
	dash := func(s string) string {
		if s == "" {
			return "-"
		}
		return s
	}
	cells := [][4]string{{"package", "current", "wanted", "latest"}}
	for _, r := range rows {
		cells = append(cells, [4]string{r.Package.(string), dash(r.Current), dash(r.Wanted), r.Latest})
	}
	var widths [3]int
	for _, c := range cells {
		for i := 0; i < 3; i++ {
			if n := utf8.RuneCountInString(c[i]); n > widths[i] {
				widths[i] = n
			}
		}
	}
	lines := make([]string, len(cells))
	for k, c := range cells {
		lines[k] = "  " + ljust(c[0], widths[0]) + "  " + ljust(c[1], widths[1]) + "  " + ljust(c[2], widths[2]) + "  " + c[3]
	}
	return lines
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9_.-]`)

// OutdatedReport prints a report's rows and summary as outdated.py's report
// does, saves them as JSON in jsonDir (when not ""), and returns the exit
// status: 1 with fail when anything is outdated.
func OutdatedReport(w io.Writer, format, text, lang, component, direct, jsonDir string, fail bool) (int, error) {
	rows, err := ParseOutdated(format, text, direct)
	if err != nil {
		return 1, err
	}
	for _, line := range Table(rows) {
		fmt.Fprintln(w, line)
	}
	noun := "dependencies"
	if len(rows) == 1 {
		noun = "dependency"
	}
	fmt.Fprintf(w, "%s: %s %d outdated %s\n", component, lang, len(rows), noun)
	if jsonDir != "" {
		if err := os.MkdirAll(jsonDir, 0o777); err != nil {
			return 1, err
		}
		name := fmt.Sprintf("%s-%s.json", lang, unsafeName.ReplaceAllString(component, "_"))
		if err := os.WriteFile(filepath.Join(jsonDir, name), []byte(rowsJSON(rows, lang, component)), 0o666); err != nil {
			return 1, err
		}
	}
	if fail && len(rows) > 0 {
		return 1, nil
	}
	return 0, nil
}

// rowsJSON is json.dump's output for the rows as dicts.
func rowsJSON(rows []Row, lang, component string) string {
	parts := make([]string, len(rows))
	for i, r := range rows {
		parts[i] = fmt.Sprintf(`{"component": %s, "language": %s, "package": %s, "current": %s, "wanted": %s, "latest": %s}`,
			pyJSONString(component), pyJSONString(lang), pyJSONString(r.Package.(string)), pyJSONString(r.Current),
			pyJSONString(r.Wanted), pyJSONString(r.Latest))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// pyJSONString is a str as Python's json module writes it (ensure_ascii).
func pyJSONString(s string) string {
	var out strings.Builder
	out.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			out.WriteString(`\"`)
		case r == '\\':
			out.WriteString(`\\`)
		case r == '\n':
			out.WriteString(`\n`)
		case r == '\r':
			out.WriteString(`\r`)
		case r == '\t':
			out.WriteString(`\t`)
		case r == '\b':
			out.WriteString(`\b`)
		case r == '\f':
			out.WriteString(`\f`)
		case r >= 0x20 && r <= 0x7e:
			out.WriteRune(r)
		case r > 0xffff:
			r -= 0x10000
			fmt.Fprintf(&out, `\u%04x\u%04x`, 0xd800+(r>>10), 0xdc00+(r&0x3ff))
		default:
			fmt.Fprintf(&out, `\u%04x`, r)
		}
	}
	out.WriteByte('"')
	return out.String()
}

// Collect is every row saved in a directory, as one JSON array the way
// outdated.py's collect prints it (json.dumps, indent=2).
func Collect(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var names []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	var rows []any
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "", err
		}
		value, err := jsonLoads(project.PyText(data))
		if err != nil {
			return "", err
		}
		list, err := iterate(value)
		if err != nil {
			return "", err
		}
		rows = append(rows, list...)
	}
	return pyJSONIndent(rows, ""), nil
}

// pyJSONIndent is json.dumps(value, indent=2).
func pyJSONIndent(value any, indent string) string {
	inner := indent + "  "
	switch v := value.(type) {
	case nil:
		return "null"
	case bool:
		if v {
			return "true"
		}
		return "false"
	case Number:
		switch f := v.float(); {
		case !v.Float:
			return repr(v)
		case f != f:
			return "NaN"
		case math.IsInf(f, 1):
			return "Infinity"
		case math.IsInf(f, -1):
			return "-Infinity"
		}
		return repr(v)
	case string:
		return pyJSONString(v)
	case []any:
		if len(v) == 0 {
			return "[]"
		}
		parts := make([]string, len(v))
		for i, item := range v {
			parts[i] = inner + pyJSONIndent(item, inner)
		}
		return "[\n" + strings.Join(parts, ",\n") + "\n" + indent + "]"
	case *Object:
		if len(v.Keys) == 0 {
			return "{}"
		}
		parts := make([]string, len(v.Keys))
		for i, key := range v.Keys {
			parts[i] = inner + pyJSONString(key) + ": " + pyJSONIndent(v.Values[key], inner)
		}
		return "{\n" + strings.Join(parts, ",\n") + "\n" + indent + "}"
	}
	return "null"
}
