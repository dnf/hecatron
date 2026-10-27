package project

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Toolchain versions (docs/design.md §18): which version of a language a
// component uses, as project.py's resolve_version and its helpers decide
// it. Each layer of a definition is read on its own here, as project.py
// reads it: the definition's pin files come before the project's, and only
// the built-in layer gives the default.

// asOfAfterDays is how old a checkout's last commit is before its
// toolchains are the releases current then.
const asOfAfterDays = 365

// Resolved is the version a component's language resolves to.
type Resolved struct {
	Spec    string // "" when a pin file names no version, or nothing gives one
	Source  string // where it came from: "pin:...", "project", "default", "supports:...", "as-of:..."; "" for none
	Selects bool   // false when it says what to install but not what runs ("selects": false)
}

// Exact reports whether the spec names one release ("=4.5").
func (r Resolved) Exact() bool { return strings.HasPrefix(r.Spec, "=") }

// versionCache is what project.py's Project caches, or rereads cheaply,
// while answering: the definitions' layers, the release tables, the
// installed versions and the checkout's date. One Project serves the
// runner's parallel jobs, so it is guarded.
type versionCache struct {
	layers    map[string]*Object
	releases  map[string][]release
	installed map[string][]Installed
	asOf      *string
}

// layer is one definition's config.json as project.py's _layer reads it:
// the built-in one or the project's, empty when there is none.
func (p *Project) layer(lang string, builtin bool) (*Object, error) {
	base := p.projectLanguages()
	if builtin {
		base = p.Home.Languages
	}
	path := pyJoin(base, lang, "config.json")
	p.cacheMu.Lock()
	cached, ok := p.cache.layers[path]
	p.cacheMu.Unlock()
	if ok {
		return cached, nil
	}
	value, err := readJSON(path)
	if err != nil {
		return nil, err
	}
	object, isObject := value.(*Object)
	if value == nil && !isFile(path) {
		object, isObject = &Object{Values: map[string]any{}}, true
	}
	if !isObject {
		return nil, unsupported("%s is not an object", path)
	}
	p.cacheMu.Lock()
	if p.cache.layers == nil {
		p.cache.layers = map[string]*Object{}
	}
	p.cache.layers[path] = object
	p.cacheMu.Unlock()
	return object, nil
}

// fileEntry is a pinFiles or supportFiles entry.
type fileEntry struct {
	file, format string
	key          pinKey
	selects      bool
}

// fileEntries are a definition's pin or support file entries (key
// "pinFiles" or "supportFiles"), the built-in ones first.
func (p *Project) fileEntries(lang, key string) ([]fileEntry, error) {
	var entries []fileEntry
	for _, builtin := range []bool{true, false} {
		layer, err := p.layer(lang, builtin)
		if err != nil {
			return nil, err
		}
		value, present := layer.Get(key)
		if !present {
			continue
		}
		list, ok := value.([]any)
		if !ok {
			return nil, unsupported("%s %s is not a list", lang, key)
		}
		for _, item := range list {
			object, ok := item.(*Object)
			if !ok {
				return nil, unsupported("%s %s holds a non-object", lang, key)
			}
			file, fileOK := object.Values["file"].(string)
			format, formatOK := object.Values["format"].(string)
			if !fileOK || !formatOK {
				return nil, unsupported("%s %s entry without a file and format", lang, key)
			}
			entry := fileEntry{file: file, format: format, selects: true}
			if name, present := object.Get("key"); present && name != nil {
				text, ok := name.(string)
				if !ok {
					return nil, unsupported("%s %s key is not a string", lang, key)
				}
				entry.key = pinKey{text, true}
			}
			if selects, present := object.Get("selects"); present {
				entry.selects = truthy(selects)
			}
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

// DefaultVersion is the definition's default version ("=3.15.1"), "" when
// the language has no built-in definition or it names none.
func (p *Project) DefaultVersion(lang string) (string, error) {
	builtin, err := p.layer(lang, true)
	if err != nil || len(builtin.Keys) == 0 {
		return "", err
	}
	value, _ := builtin.Get("version")
	if value == nil {
		return "", nil
	}
	text, err := pyStr(value)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(text, "=") {
		repr, err := pyRepr(value)
		if err != nil {
			return "", err
		}
		return "", &Error{fmt.Sprintf(`%s: the definition's default version %s must be exact ("=%s")`, lang, repr, text)}
	}
	if _, ok := value.(string); !ok {
		return "", unsupported("%s's default version is not a string", lang)
	}
	return text, checkDigits(text)
}

// projectVersion is the project's "version" (for a language only the
// project defines, its definition's): "" with ok false when it sets none.
func (p *Project) projectVersion(lang string) (string, bool, error) {
	layer, err := p.layer(lang, false)
	if err != nil {
		return "", false, err
	}
	value, _ := layer.Get("version")
	if value == nil {
		return "", false, nil
	}
	text, ok := value.(string)
	if !ok {
		return "", false, unsupported("%s's version is not a string", lang)
	}
	return text, true, checkDigits(text)
}

// builtinString is a string key of the built-in definition, "" when unset.
func (p *Project) builtinString(lang, key string) (string, error) {
	builtin, err := p.layer(lang, true)
	if err != nil {
		return "", err
	}
	value, _ := builtin.Get(key)
	if value == nil {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", unsupported("%s's %s is not a string", lang, key)
	}
	return text, checkDigits(text)
}

// pinEntry is project.py's _pin_entry: the spec from the nearest file an
// entry finds, searching from the component's implementation directory up
// to the project root, with its source ("<kind>:<path>") and entry.
func (p *Project) pinEntry(lang, component string, entries []fileEntry, kind string) (pinned, string, fileEntry, error) {
	if len(entries) == 0 {
		return notPinned, "", fileEntry{}, nil
	}
	directory, err := p.ImplDir(component, lang)
	if err != nil {
		return notPinned, "", fileEntry{}, err
	}
	rootLength := utf8.RuneCountInString(p.Root)
	for {
		for _, entry := range entries {
			value, path, err := readEntry(directory, entry)
			if err != nil {
				return notPinned, "", fileEntry{}, err
			}
			if value.pins {
				return value, kind + ":" + pyRelpath(path, p.Root), entry, nil
			}
		}
		if pyNormpath(directory) == pyNormpath(p.Root) || utf8.RuneCountInString(directory) <= rootLength {
			return notPinned, "", fileEntry{}, nil
		}
		directory = pyDirname(directory)
	}
}

// readEntry is project.py's _read_entry: what one entry's file says in a
// directory, and the file. An entry naming a pattern gives the newest
// version any matching file does.
func readEntry(directory string, entry fileEntry) (pinned, string, error) {
	paths, err := pyGlob(pyJoin(directory, entry.file))
	if err != nil {
		return notPinned, "", err
	}
	sort.Strings(paths)
	type found struct {
		value pinned
		path  string
	}
	var all, named []found
	for _, path := range paths {
		if !isFile(path) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return notPinned, "", unsupported("%v", err)
		}
		value, err := readPin(entry.format, pyText(data), entry.key)
		if err != nil {
			return notPinned, "", err
		}
		if value.pins {
			all = append(all, found{value, path})
			if value.spec != "" {
				named = append(named, found{value, path})
			}
		}
	}
	switch {
	case len(all) == 0:
		return notPinned, "", nil
	case len(named) == 0:
		return all[0].value, all[0].path, nil
	}
	best := named[0]
	for _, candidate := range named[1:] {
		if compareVersions(strings.TrimLeft(candidate.value.spec, "="), strings.TrimLeft(best.value.spec, "=")) > 0 {
			best = candidate
		}
	}
	return best.value, best.path, nil
}

// ResolveVersion is the version a component uses (project.py's
// resolve_version and selects_version): its nearest pin file, else the
// project's "version", else the definition's default, or the newest line
// the component says it supports when the default is newer. In an old
// checkout, anything short of an exact version then becomes the release
// current at its last commit. An empty component means the first one
// implemented in the language.
func (p *Project) ResolveVersion(lang, component string) (Resolved, error) {
	if component == "" {
		var err error
		if component, err = p.firstComponent(lang); err != nil {
			return Resolved{}, err
		}
	}
	defaultVersion, err := p.DefaultVersion(lang)
	if err != nil {
		return Resolved{}, err
	}
	pins, err := p.fileEntries(lang, "pinFiles")
	if err != nil {
		return Resolved{}, err
	}
	value, source, _, err := p.pinEntry(lang, component, pins, "pin")
	if err != nil {
		return Resolved{}, err
	}
	resolved := Resolved{Spec: value.spec, Source: source, Selects: true}
	if !value.pins {
		version, set, err := p.projectVersion(lang)
		if err != nil {
			return Resolved{}, err
		}
		resolved = Resolved{Spec: version, Source: "project", Selects: true}
		if !set {
			resolved = Resolved{Spec: defaultVersion, Selects: true}
			if defaultVersion != "" {
				resolved.Source = "default"
				if resolved, err = p.supportedVersion(lang, component, defaultVersion, resolved); err != nil {
					return Resolved{}, err
				}
			}
		}
	}
	if resolved.Spec == "" && resolved.Source == "project" {
		_, err := parseSpec("") // project.py's Spec("") raises
		return Resolved{}, err
	}
	if resolved.Spec != "" {
		if _, err := parseSpec(resolved.Spec); err != nil {
			return Resolved{}, err
		}
		if resolved.Source == "default" || !resolved.Exact() {
			if resolved.Spec, resolved.Source, err = p.asOfVersion(lang, resolved.Spec, resolved.Source); err != nil {
				return Resolved{}, err
			}
		}
	}
	return resolved, nil
}

// supportedVersion holds a component on the default back to the newest
// line its support files say it supports, when that is older.
func (p *Project) supportedVersion(lang, component, defaultVersion string, resolved Resolved) (Resolved, error) {
	support, err := p.fileEntries(lang, "supportFiles")
	if err != nil {
		return resolved, err
	}
	supported, where, entry, err := p.pinEntry(lang, component, support, "supports")
	if err != nil || supported.spec == "" {
		return resolved, err
	}
	older, err := p.olderLine(lang, supported.spec, defaultVersion)
	if err != nil || !older {
		return resolved, err
	}
	spec, err := p.installable(lang, supported.spec)
	return Resolved{Spec: spec, Source: where, Selects: entry.selects}, err
}

// olderLine is whether the line a component supports up to (3.14) is
// older than the default's (=3.15.1), compared by release line.
func (p *Project) olderLine(lang, supported, defaultVersion string) (bool, error) {
	line, err := p.releaseLine(lang, defaultVersion)
	if err != nil {
		return false, err
	}
	parts := len(strings.Split(line, "."))
	supportedSpec, err := parseSpec(supported)
	if err != nil {
		return false, err
	}
	defaultSpec, err := parseSpec(defaultVersion)
	if err != nil {
		return false, err
	}
	supportedLine := prefixNumbers(supportedSpec.numbers, parts)
	return compareNumbers(supportedLine, prefixNumbers(defaultSpec.numbers, len(supportedLine))) < 0, nil
}

// releaseLine is a version's release line as a prefix: its first
// "releaseLine" parts (Node.js 24.22.1 is line 24 with 1), else all but the
// last (Go 1.27.3 is line 1.27).
func (p *Project) releaseLine(lang, version string) (string, error) {
	parts := strings.Split(strings.TrimLeft(version, "="), ".")
	count := max(len(parts)-1, 1)
	builtin, err := p.layer(lang, true)
	if err != nil {
		return "", err
	}
	if value, present := builtin.Get("releaseLine"); present {
		if value == nil {
			count = len(parts) // parts[:None]
		} else if number, ok := value.(json.Number); !ok || strings.ContainsAny(string(number), ".eE") {
			return "", unsupported("%s's releaseLine is not an integer", lang)
		} else if n, err := strconv.Atoi(string(number)); err != nil {
			return "", unsupported("%s's releaseLine %s", lang, number)
		} else {
			count = pySliceEnd(n, len(parts))
		}
	}
	return strings.Join(parts[:min(count, len(parts))], "."), nil
}

// pySliceEnd is where Python's list[:n] ends for a list of length.
func pySliceEnd(n, length int) int {
	if n < 0 {
		return max(length+n, 0)
	}
	return min(n, length)
}

// belowInstallable is whether a version is older than the oldest the
// definition's installer installs ("oldestInstallable").
func (p *Project) belowInstallable(lang, version string) (bool, error) {
	oldest, err := p.builtinString(lang, "oldestInstallable")
	if err != nil || oldest == "" {
		if err == nil && p.hasBuiltinKey(lang, "oldestInstallable") {
			_, err = parseSpec("")
		}
		return false, err
	}
	oldestSpec, err := parseSpec(oldest)
	if err != nil {
		return false, err
	}
	versionSpec, err := parseSpec(strings.TrimLeft(version, "="))
	if err != nil {
		return false, err
	}
	return compareNumbers(prefixNumbers(versionSpec.numbers, len(oldestSpec.numbers)), oldestSpec.numbers) < 0, nil
}

func (p *Project) hasBuiltinKey(lang, key string) bool {
	builtin, err := p.layer(lang, true)
	if err != nil {
		return false
	}
	value, present := builtin.Get(key)
	return present && value != nil
}

// installable raises a supported line older than the installer can
// install to the oldest it can.
func (p *Project) installable(lang, spec string) (string, error) {
	below, err := p.belowInstallable(lang, spec)
	if err != nil || !below {
		return spec, err
	}
	return p.builtinString(lang, "oldestInstallable")
}

// AsOf is the date an old checkout's toolchains should be current at: its
// last commit's, when that is over a year old, else "".
// HECATRON_AS_OF=YYYY-MM-DD sets the date, =off turns this off.
func (p *Project) AsOf() (string, error) {
	p.cacheMu.Lock()
	cached := p.cache.asOf
	p.cacheMu.Unlock()
	if cached != nil {
		return *cached, nil
	}
	when, err := p.computeAsOf()
	if err != nil {
		return "", err
	}
	p.cacheMu.Lock()
	p.cache.asOf = &when
	p.cacheMu.Unlock()
	return when, nil
}

var isoDate = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)

func (p *Project) computeAsOf() (string, error) {
	wanted, _ := p.lookupEnv("HECATRON_AS_OF")
	switch {
	case wanted == "off":
		return "", nil
	case wanted != "":
		if !isoDate.MatchString(wanted) {
			// Python 3.11's fromisoformat also reads 20210101 and week dates.
			return "", unsupported("HECATRON_AS_OF=%q", wanted)
		}
		if parsed, err := time.Parse("2006-01-02", wanted); err != nil || parsed.Year() < 1 {
			return "", &Error{fmt.Sprintf("HECATRON_AS_OF=%s isn't a date (YYYY-MM-DD) or off", pyStringRepr(wanted))}
		}
		return wanted, nil
	}
	cmd := exec.Command("git", "log", "-1", "--format=%cs")
	cmd.Dir, cmd.Env = p.Root, p.environ()
	out, err := cmd.Output()
	if err != nil {
		return "", nil
	}
	committed := pyStrip(string(out))
	day, err := time.Parse("2006-01-02", committed)
	if err != nil {
		return "", nil
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if today.Sub(day) > asOfAfterDays*24*time.Hour {
		return committed, nil
	}
	return "", nil
}

// release is one row of a releases.json table.
type release struct {
	version, date string
	lts           bool
}

// releaseDates are a language's releases, from its definition's
// releases.json, or that of the language its "releasesOf" names.
func (p *Project) releaseDates(lang string) ([]release, error) {
	builtin, err := p.layer(lang, true)
	if err != nil {
		return nil, err
	}
	owner := lang
	if value, present := builtin.Get("releasesOf"); present {
		text, ok := value.(string)
		if !ok {
			return nil, unsupported("%s's releasesOf is not a string", lang)
		}
		owner = text
	}
	path := pyJoin(p.Home.Languages, owner, "releases.json")
	p.cacheMu.Lock()
	cached, ok := p.cache.releases[path]
	p.cacheMu.Unlock()
	if ok {
		return cached, nil
	}
	releases, err := readReleases(path)
	if err != nil {
		return nil, err
	}
	p.cacheMu.Lock()
	if p.cache.releases == nil {
		p.cache.releases = map[string][]release{}
	}
	p.cache.releases[path] = releases
	p.cacheMu.Unlock()
	return releases, nil
}

func readReleases(path string) ([]release, error) {
	value, err := readJSON(path)
	if err != nil || value == nil && !isFile(path) {
		return nil, err
	}
	object, ok := value.(*Object)
	if !ok {
		return nil, unsupported("%s is not an object", path)
	}
	rows, present := object.Get("releases")
	if !present {
		return nil, nil
	}
	list, ok := rows.([]any)
	if !ok {
		return nil, unsupported("%s releases is not a list", path)
	}
	var releases []release
	for _, row := range list {
		fields, ok := row.([]any)
		if !ok || len(fields) < 2 {
			return nil, unsupported("%s holds a row that isn't [version, date, ...]", path)
		}
		version, versionOK := fields[0].(string)
		date, dateOK := fields[1].(string)
		if !versionOK || !dateOK || checkDigits(version) != nil {
			return nil, unsupported("%s holds a row that isn't [version, date, ...]", path)
		}
		r := release{version: version, date: date}
		for _, flag := range fields[2:] {
			if flag == "lts" {
				r.lts = true
			}
		}
		releases = append(releases, r)
	}
	return releases, nil
}

// asOfVersion is a spec as of the checkout's date: the newest release then
// that the spec matches, or, for the default, the newest on the
// definition's track (stable, or LTS). Unchanged when the checkout is
// recent, the language has no release dates, or nothing matched yet then.
func (p *Project) asOfVersion(lang, specText, source string) (string, string, error) {
	when, err := p.AsOf()
	if err != nil || when == "" {
		return specText, source, err
	}
	releases, err := p.releaseDates(lang)
	if err != nil || len(releases) == 0 {
		return specText, source, err
	}
	var candidates []string
	if source == "default" {
		builtin, err := p.layer(lang, true)
		if err != nil {
			return "", "", err
		}
		track, _ := builtin.Get("track")
		ltsOnly := track == "lts"
		for _, r := range releases {
			if r.date <= when && (r.lts || !ltsOnly) {
				candidates = append(candidates, r.version)
			}
		}
	} else {
		wanted := mustSpec(specText)
		for _, r := range releases {
			if r.date <= when && wanted.matches(r.version) {
				candidates = append(candidates, r.version)
			}
		}
	}
	picked, ok := newest(candidates)
	if !ok {
		return specText, source, nil
	}
	note := ""
	below, err := p.belowInstallable(lang, picked)
	if err != nil {
		return "", "", err
	}
	if below {
		if source == "project" || strings.HasPrefix(source, "pin:") {
			return specText, source, nil // what the project chose stands, installable or not
		}
		oldest, err := p.builtinString(lang, "oldestInstallable")
		if err != nil {
			return "", "", err
		}
		oldestSpec := mustSpec(oldest)
		candidates = nil
		for _, r := range releases {
			if r.date <= when && oldestSpec.matches(r.version) {
				candidates = append(candidates, r.version)
			}
		}
		note = "; raised to " + oldest + ", the oldest hecatron installs"
		if picked, ok = newest(candidates); !ok {
			return oldest, fmt.Sprintf("as-of:%s (%s%s)", when, source, note), nil
		}
	}
	precision, err := p.builtinValue(lang, "asOfPrecision")
	if err != nil {
		return "", "", err
	}
	dated := "=" + picked
	if precision == "line" {
		// Where the installer takes a line, not a release (uv's Pythons, a
		// PHP package), the release's line: the newest patch of it.
		if dated, err = p.releaseLine(lang, picked); err != nil {
			return "", "", err
		}
	}
	return dated, fmt.Sprintf("as-of:%s (%s%s)", when, source, note), nil
}

func (p *Project) builtinValue(lang, key string) (any, error) {
	builtin, err := p.layer(lang, true)
	if err != nil {
		return nil, err
	}
	value, _ := builtin.Get(key)
	return value, nil
}

func (p *Project) firstComponent(lang string) (string, error) {
	components, err := p.versionComponents(lang)
	if err != nil {
		return "", err
	}
	return components[0], nil
}

// versionComponents are the components a language's version queries cover:
// those implemented in it, else the first.
func (p *Project) versionComponents(lang string) ([]string, error) {
	dirs, err := p.ImplDirs(lang)
	if err != nil {
		return nil, err
	}
	var components []string
	for _, dir := range dirs {
		components = append(components, dir.Component)
	}
	if len(components) == 0 {
		components = []string{p.Components[0]}
	}
	return components, nil
}

// SpecSources is one distinct spec the components resolve to and where it
// came from.
type SpecSources struct {
	Spec    string
	Sources []string
}

// Versions are the distinct specs a language's components resolve to, in
// component order, with their sources (project.py's versions).
func (p *Project) Versions(lang string) ([]SpecSources, error) {
	components, err := p.versionComponents(lang)
	if err != nil {
		return nil, err
	}
	var result []SpecSources
	for _, component := range components {
		resolved, err := p.ResolveVersion(lang, component)
		if err != nil {
			return nil, err
		}
		i := 0
		for i < len(result) && result[i].Spec != resolved.Spec {
			i++
		}
		if i == len(result) {
			result = append(result, SpecSources{Spec: resolved.Spec})
		}
		if resolved.Source != "" && !contains(result[i].Sources, resolved.Source) {
			result[i].Sources = append(result[i].Sources, resolved.Source)
		}
	}
	return result, nil
}

func contains(list []string, item string) bool {
	for _, entry := range list {
		if entry == item {
			return true
		}
	}
	return false
}

// Conflict is a pin file and the project's "version" both applying to a
// component and disagreeing.
type Conflict struct {
	Lang, Component, Source string
	PinSpec                 string // "" for a pin file naming no version
	ProjectSpec             string
}

// VersionConflicts are project.py's version_conflicts, for hecatron check.
func (p *Project) VersionConflicts() ([]Conflict, error) {
	var conflicts []Conflict
	for _, lang := range p.Languages {
		wanted, set, err := p.projectVersion(lang)
		if err != nil {
			return nil, err
		}
		builtin, err := p.layer(lang, true)
		if err != nil {
			return nil, err
		}
		if !set || len(builtin.Keys) == 0 {
			continue
		}
		pins, err := p.fileEntries(lang, "pinFiles")
		if err != nil {
			return nil, err
		}
		dirs, err := p.ImplDirs(lang)
		if err != nil {
			return nil, err
		}
		for _, dir := range dirs {
			value, source, _, err := p.pinEntry(lang, dir.Component, pins, "pin")
			if err != nil {
				return nil, err
			}
			if value.pins && (value.spec == "" || value.spec != wanted) { // None != "" in Python
				conflicts = append(conflicts, Conflict{lang, dir.Component, source, value.spec, wanted})
			}
		}
	}
	return conflicts, nil
}

// environ is the environment project.py would run with: Environ, else this
// process's.
func (p *Project) environ() []string {
	if p.Environ != nil {
		return p.Environ
	}
	return os.Environ()
}

func (p *Project) lookupEnv(name string) (string, bool) {
	env := p.environ()
	for i := len(env) - 1; i >= 0; i-- {
		if value, ok := strings.CutPrefix(env[i], name+"="); ok {
			return value, true
		}
	}
	return "", false
}
