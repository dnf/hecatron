package project

import (
	"encoding/json"
	"os"
	"os/exec"
	"sort"
	"strings"
	"unicode/utf8"
)

// Detection (docs/guide/init.md): which languages a repository uses and
// where, from its tracked files, as manage.py's detect_languages,
// infer_layout and detected_config decide it for hecatron init and for a
// repository read without a config of its own.

// skippedDirs hold code that isn't the project's own implementation:
// vendored dependencies, samples, tests and test data, docs, tooling (hooks/:
// git hooks, build hooks) and a generator's templates, whatever their case
// (Examples/), as do hidden directories (.jenkins/, .github/). A language
// found only there isn't added, and no language's directory is placed there.
var skippedDirs = map[string]bool{"android": true, "benchmarks": true, "buildsrc": true, "carthage": true, "demo": true,
	"demos": true, "doc": true, "docs": true, "example": true, "examples": true, "external": true, "fixtures": true,
	"hooks": true, "node_modules": true, "playground": true, "pods": true, "sample": true, "samples": true, "templates": true,
	"test": true, "test-data": true, "test_data": true, "testdata": true, "tests": true, "tests_data": true,
	"third_party": true, "tools": true, "tutorial": true, "tutorials": true, "vendor": true, "vendored": true}

// preferredParents are where projects keep per-language code; when layouts
// disagree, one under these (or at the root) beats one covering more
// languages (Beam's sdks/{lang} over learning/katas/{lang}).
var preferredParents = map[string]bool{"bindings": true, "ffi": true, "lang": true, "lib": true, "packages": true,
	"sdk": true, "sdks": true}

// detection is what one built-in definition says about finding it.
type detection struct {
	lang                                          string
	detect, sources, unless, shared, names, tests []string
	keys                                          []string
	withSources                                   *withSourcesRule
	yields                                        *yieldsRule
	name                                          string // "name", else lang
}

// detectionDefinitions are the built-in definitions' detection keys, in
// name order (manage.py's _detection_definitions: the project's own
// definitions don't take part).
func detectionDefinitions(home Home) ([]detection, error) {
	entries, err := os.ReadDir(home.Languages)
	if err != nil {
		return nil, unsupported("%v", err)
	}
	var defs []detection
	for _, entry := range entries {
		lang := entry.Name()
		path := pyJoin(home.Languages, lang, "config.json")
		if !isFile(path) {
			continue
		}
		value, err := readJSON(path)
		if err != nil {
			return nil, err
		}
		object, ok := value.(*Object)
		if !ok {
			return nil, unsupported("%s is not an object", path)
		}
		d := detection{lang: lang}
		for key, target := range map[string]*[]string{"detect": &d.detect, "detectSources": &d.sources,
			"detectUnless": &d.unless, "sharedBuildFiles": &d.shared, "dirNames": &d.names, "detectTests": &d.tests} {
			if raw, present := object.Get(key); present {
				if *target, err = toStrings(raw, lang+" "+key); err != nil {
					return nil, err
				}
			}
		}
		if raw, present := object.Get("detectKeys"); present && truthy(raw) {
			if d.keys, err = toStrings(raw, lang+" detectKeys"); err != nil {
				return nil, err
			}
		}
		raw, present := object.Get("detectWithSources")
		if d.withSources, err = parseWithSources(raw, present, lang); err != nil {
			return nil, err
		}
		raw, present = object.Get("detectYields")
		if d.yields, err = parseYields(raw, present, lang); err != nil {
			return nil, err
		}
		d.name = lang
		if raw, present := object.Get("name"); present {
			if d.name, ok = raw.(string); !ok {
				return nil, unsupported("%s name is not a string", lang)
			}
		}
		names := []string{lang}
		for _, name := range d.names {
			if name != lang {
				names = append(names, name)
			}
		}
		d.names = names
		defs = append(defs, d)
	}
	return defs, nil
}

// TrackedFiles are git ls-files' lines in root (paths git quotes stay
// quoted, as manage.py reads them); none outside a repository.
func TrackedFiles(root string) ([]string, error) {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, nil
	}
	if !utf8.Valid(out) {
		return nil, unsupported("git ls-files printed text that isn't UTF-8")
	}
	return pySplitLines(pyText(out)), nil
}

func isSkipped(directory string) bool {
	for _, part := range strings.Split(directory, "/") {
		if skippedDirs[strings.ToLower(part)] || strings.HasPrefix(part, ".") && part != "." {
			return true
		}
	}
	return false
}

func under(directory, parent string) bool {
	return directory == parent || strings.HasPrefix(directory, parent+"/")
}

// dirSet is a set of directories.
type dirSet map[string]bool

func (s dirSet) sorted() []string {
	list := make([]string, 0, len(s))
	for d := range s {
		list = append(list, d)
	}
	sort.Strings(list)
	return list
}

// namesByDir are the file names in each directory.
type namesByDir map[string]map[string]bool

// splitByDir are the file names in each directory: outside skippedDirs,
// and inside.
func splitByDir(files []string) (byDir, skipped namesByDir) {
	byDir, skipped = namesByDir{}, namesByDir{}
	for _, path := range files {
		directory := pyDirname(path)
		if directory == "" {
			directory = "."
		}
		tree := byDir
		if isSkipped(directory) {
			tree = skipped
		}
		if tree[directory] == nil {
			tree[directory] = map[string]bool{}
		}
		_, name := pySplit(path)
		tree[directory][name] = true
	}
	return byDir, skipped
}

// holdsLanguage is whether a directory holds a language's code: one of
// its build or source files (detect, detectSources) in it or under it, or,
// for a language without detectSources, which can't tell its sources, any
// file when the directory sits at the root or under a preferredParents
// directory (Thrift's lib/go, not a languages/go/ holding Go's definition).
// A build file deeper than the hooks look for one (hookSearchDepth levels
// below) doesn't count, and with only such build files the directory
// doesn't hold the language: its hooks would find no build (antlr4's
// runtime/Go/antlr/v4/go.mod under the root).
func holdsLanguage(byDir namesByDir, defs []detection) func(lang, directory string) (bool, error) {
	byLang := map[string]detection{}
	for _, d := range defs {
		byLang[d.lang] = d
	}
	return func(lang, directory string) (bool, error) {
		d := byLang[lang]
		inside, nearBuild, deepBuild := false, false, false
		for sub, names := range byDir {
			if under(sub, directory) {
				inside = true
				if matched, err := anyMatch(names, d.sources); err != nil || matched {
					return matched, err
				}
			}
			if directory != "." && !under(sub, directory) {
				continue
			}
			matched, err := anyMatch(names, d.detect)
			if err != nil {
				return false, err
			}
			if matched && depthBelow(sub, directory) <= hookSearchDepth {
				nearBuild = true
			}
			deepBuild = deepBuild || matched
		}
		if nearBuild {
			return true, nil
		}
		return !deepBuild && len(d.sources) == 0 && inside && preference(directory)[0] == 0, nil
	}
}

// hookSearchDepth is how many levels below a directory the default hooks
// look for a build file (dirs_below_with in languages/_hooks.sh).
const hookSearchDepth = 3

// allDeeper is whether every directory is deeper below the root than the
// hooks look.
func allDeeper(dirs dirSet) bool {
	for directory := range dirs {
		if depthBelow(directory, ".") <= hookSearchDepth {
			return false
		}
	}
	return true
}

func depthBelow(sub, directory string) int {
	depth := 0
	if sub != "." {
		depth = len(strings.Split(sub, "/"))
	}
	if directory != "." {
		depth -= len(strings.Split(directory, "/"))
	}
	return depth
}

func anyMatch(names map[string]bool, patterns []string) (bool, error) {
	for name := range names {
		for _, pattern := range patterns {
			matched, err := fnmatch(name, pattern)
			if err != nil || matched {
				return matched, err
			}
		}
	}
	return false, nil
}

// hits are the directories with a name matching patterns and none matching
// unless.
func hits(dirs namesByDir, patterns, unless []string) (dirSet, error) {
	return hitsAmong(dirs, patterns, unless, false)
}

// sourceHits are hits for source files, leaving out dotfiles: a .nvim.lua
// or .eslintrc.js configures an editor or a tool, it isn't the project's
// code.
func sourceHits(dirs namesByDir, sources, unless []string) (dirSet, error) {
	return hitsAmong(dirs, sources, unless, true)
}

func hitsAmong(dirs namesByDir, patterns, unless []string, noDotfiles bool) (dirSet, error) {
	found := dirSet{}
	for directory, names := range dirs {
		candidates := names
		if noDotfiles {
			candidates = map[string]bool{}
			for name := range names {
				if !strings.HasPrefix(name, ".") {
					candidates[name] = true
				}
			}
		}
		matched, err := anyMatch(candidates, patterns)
		if err != nil {
			return nil, err
		}
		if !matched {
			continue
		}
		excluded, err := anyMatch(names, unless)
		if err != nil {
			return nil, err
		}
		if !excluded {
			found[directory] = true
		}
	}
	return found, nil
}

func countMatching(names map[string]bool, patterns []string) (int, error) {
	n := 0
	for name := range names {
		for _, pattern := range patterns {
			matched, err := fnmatch(name, pattern)
			if err != nil {
				return 0, err
			}
			if matched {
				n++
				break
			}
		}
	}
	return n, nil
}

// Detected is what detection found: each language's directories, and the
// languages left out with why.
type Detected struct {
	Found    map[string]dirSet
	NotAdded map[string]string
}

// DetectLanguages is manage.py's detect_languages over the tracked files.
func DetectLanguages(root string, defs []detection, files []string) (*Detected, error) {
	byDir, skipped := splitByDir(files)

	// A shared build file goes to the language with the most sources under
	// it (libphonenumber's cpp/CMakeLists.txt is C++'s, not its few C
	// files').
	sharedCounts := map[string]map[string]int{}
	for _, d := range defs {
		directories, err := hits(byDir, d.shared, nil)
		if err != nil {
			return nil, err
		}
		for directory := range directories {
			n, err := sharedSources(byDir, directory, d.sources)
			if err != nil {
				return nil, err
			}
			if n > 0 {
				if sharedCounts[directory] == nil {
					sharedCounts[directory] = map[string]int{}
				}
				sharedCounts[directory][d.lang] = n
			}
		}
	}
	sharedHits := func(lang string) dirSet {
		result := dirSet{}
		for directory, counts := range sharedCounts {
			most := 0
			for _, n := range counts {
				most = max(most, n)
			}
			if n, ok := counts[lang]; ok && n == most {
				result[directory] = true
			}
		}
		return result
	}

	// A hit a language's "detectYields" rule holds for is the other
	// language's, whose "detectUnless" doesn't keep it out there.
	yielded, granted := map[string]dirSet{}, map[string]dirSet{}
	for _, d := range defs {
		if d.yields == nil {
			continue
		}
		directories, err := hits(byDir, d.detect, d.unless)
		if err != nil {
			return nil, err
		}
		for directory := range directories {
			var paths []string
			for sub, inside := range byDir {
				rel, ok := relativeTo(sub, directory)
				if !ok {
					continue
				}
				for name := range inside {
					if rel == "" {
						paths = append(paths, name)
					} else {
						paths = append(paths, rel+"/"+name)
					}
				}
			}
			var listing []string
			for name := range byDir[directory] {
				listing = append(listing, name)
			}
			buildFile, err := buildFileIn(pyJoin(root, directory), listing, d.detect)
			if err != nil {
				return nil, err
			}
			holds, err := d.yields.holds(paths, buildFile)
			if err != nil {
				return nil, err
			}
			if holds {
				if yielded[d.lang] == nil {
					yielded[d.lang] = dirSet{}
				}
				if granted[d.yields.To] == nil {
					granted[d.yields.To] = dirSet{}
				}
				yielded[d.lang][directory] = true
				granted[d.yields.To][directory] = true
			}
		}
	}
	nameOf := map[string]string{}
	for _, d := range defs {
		nameOf[d.lang] = d.name
	}

	result := &Detected{Found: map[string]dirSet{}, NotAdded: map[string]string{}}
	fromSources := map[string]bool{}
	for _, d := range defs {
		directories, err := hits(byDir, d.detect, d.unless)
		if err != nil {
			return nil, err
		}
		for directory := range yielded[d.lang] {
			delete(directories, directory)
		}
		if len(granted[d.lang]) > 0 {
			beside, err := hits(byDir, d.detect, nil)
			if err != nil {
				return nil, err
			}
			for directory := range granted[d.lang] {
				if beside[directory] {
					directories[directory] = true
				}
			}
		}
		for directory := range sharedHits(d.lang) {
			directories[directory] = true
		}
		tooling := ""
		if len(d.keys) > 0 && len(directories) > 0 {
			for directory := range directories {
				has, err := hasDetectKey(root, directory, byDir[directory], d.detect, d.keys)
				if err != nil {
					return nil, err
				}
				if !has {
					delete(directories, directory)
				}
			}
			if len(directories) == 0 {
				tooling = "its " + strings.Join(d.detect, ", ") + " has none of " + strings.Join(d.keys, ", ")
			}
		}
		withSources, err := d.withSources.hits(byDir, d.unless)
		if err != nil {
			return nil, err
		}
		for directory := range withSources {
			directories[directory] = true
		}
		if len(directories) == 0 && tooling != "" {
			result.NotAdded[d.lang] = tooling
			continue
		}
		if len(directories) == 0 && len(d.sources) > 0 {
			if directories, err = sourceHits(byDir, d.sources, d.unless); err != nil {
				return nil, err
			}
			fromSources[d.lang] = true
		}
		if len(directories) == 0 && len(d.tests) > 0 {
			inTests, err := hits(skipped, d.tests, nil)
			if err != nil {
				return nil, err
			}
			for directory := range inTests {
				if tested, ok := testedDirectory(directory); ok {
					directories[tested] = true
				}
			}
		}
		if len(directories) > 0 {
			result.Found[d.lang] = directories
			continue
		}
		if len(yielded[d.lang]) > 0 {
			other, ok := nameOf[d.yields.To]
			if !ok {
				other = d.yields.To
			}
			result.NotAdded[d.lang] = "its " + strings.Join(d.detect, ", ") + " only sits beside mostly " + other
			continue
		}
		inSkipped, err := hits(skipped, append(append([]string(nil), d.detect...), d.sources...), d.unless)
		if err != nil {
			return nil, err
		}
		if len(inSkipped) > 0 {
			result.NotAdded[d.lang] = "found only in examples, tests or vendored code"
		}
	}

	// A language found only by its sources keeps the directories outside
	// every other language's, and isn't added when there are none.
	names := map[string][]string{}
	for _, d := range defs {
		names[d.lang] = d.names
	}
	claims := map[string]dirSet{}
	for lang, dirs := range result.Found {
		claimed := dirSet{}
		for c := range languageRoots(dirs, names[lang]) {
			claimed[c] = true
		}
		if !fromSources[lang] {
			for c := range dirs {
				claimed[c] = true
			}
		}
		delete(claimed, ".")
		claims[lang] = claimed
	}
	var langs []string
	for lang := range fromSources {
		if result.Found[lang] != nil {
			langs = append(langs, lang)
		}
	}
	sort.Strings(langs)
	for _, lang := range langs {
		own, owners := dirSet{}, dirSet{}
		for directory := range result.Found[lang] {
			ownedBy := false
			for other, claimed := range claims {
				if other == lang {
					continue
				}
				for c := range claimed {
					if under(directory, c) {
						owners[other] = true
						ownedBy = true
						break
					}
				}
			}
			if !ownedBy {
				own[directory] = true
			}
		}
		if len(own) > 0 {
			result.Found[lang] = own
		} else {
			result.NotAdded[lang] = "its files are all inside " + strings.Join(owners.sorted(), ", ") + "'s directories"
			delete(result.Found, lang)
		}
	}
	return result, nil
}

// testedDirectory is the directory a test directory (test/ or tests/, in
// any case) tests: its parent. Not ok when the test directory is itself in
// skipped code (vendor/x/test/) or the directory isn't in one.
func testedDirectory(directory string) (string, bool) {
	parts := strings.Split(directory, "/")
	for i, part := range parts {
		lower := strings.ToLower(part)
		if lower == "test" || lower == "tests" {
			if i == 0 {
				return ".", true
			}
			return strings.Join(parts[:i], "/"), true
		}
		if skippedDirs[lower] {
			return "", false
		}
	}
	return "", false
}

// sharedSources counts the language's source files in a directory and
// under its src/ and include/.
func sharedSources(byDir namesByDir, directory string, sources []string) (int, error) {
	n, err := countMatching(byDir[directory], sources)
	if err != nil {
		return 0, err
	}
	var nested []string
	for _, sub := range []string{"src", "include"} {
		nested = append(nested, cleanSlashPath(pyJoin(directory, sub)))
	}
	for d, names := range byDir {
		if under(d, nested[0]) || under(d, nested[1]) {
			more, err := countMatching(names, sources)
			if err != nil {
				return 0, err
			}
			n += more
		}
	}
	return n, nil
}

// hasDetectKey is whether the directory's build file (JSON: the first of
// its names a detect pattern matches) has one of the keys, non-empty; a
// dotted key is a path (scripts.test). One that can't be read counts as
// having one.
func hasDetectKey(root, directory string, names map[string]bool, detect, keys []string) (bool, error) {
	listing := make([]string, 0, len(names))
	for name := range names {
		listing = append(listing, name)
	}
	buildFile, err := buildFileIn(pyJoin(root, directory), listing, detect)
	if err != nil || buildFile == "" {
		return true, err
	}
	data, err := os.ReadFile(buildFile)
	if err != nil {
		return true, nil
	}
	if !utf8.Valid(data) {
		return true, nil // UnicodeDecodeError is a ValueError
	}
	document, ok, err := pyJSONLoads(pyText(data)) // json.load reads in text mode
	if err != nil {
		return false, err
	}
	object, isObject := document.(*Object)
	if !ok || !isObject {
		return true, nil
	}
	for _, key := range keys {
		var value any = object
		for _, part := range strings.Split(key, ".") {
			if inner, isObject := value.(*Object); isObject {
				value, _ = inner.Get(part)
			} else {
				value = nil
			}
		}
		if truthy(value) {
			return true, nil
		}
	}
	return false, nil
}

// withSourcesRule is a definition's "detectWithSources": build files
// (files) and directories (dirs, such as an Xcode project's *.xcodeproj)
// that make a hit only with the language's sources beside or below them.
type withSourcesRule struct {
	files, dirs, sources []string
}

func parseWithSources(value any, present bool, lang string) (*withSourcesRule, error) {
	if !present || !truthy(value) {
		return nil, nil
	}
	object, ok := value.(*Object)
	if !ok {
		return nil, unsupported("%s detectWithSources is not an object", lang)
	}
	rule := &withSourcesRule{}
	for key, target := range map[string]*[]string{"files": &rule.files, "dirs": &rule.dirs, "sources": &rule.sources} {
		if raw, present := object.Get(key); present {
			var err error
			if *target, err = toStrings(raw, lang+" detectWithSources "+key); err != nil {
				return nil, err
			}
		}
	}
	return rule, nil
}

// hits are the directories holding one of the rule's files, or a directory
// named like its dirs (none of unless beside it), with one of its sources
// in them or below: a requirements.txt beside Python code, not one for a
// docs build (manage.py's with_sources_hits).
func (rule *withSourcesRule) hits(byDir namesByDir, unless []string) (dirSet, error) {
	if rule == nil {
		return dirSet{}, nil
	}
	candidates, err := hits(byDir, rule.files, unless)
	if err != nil {
		return nil, err
	}
	for sub := range byDir {
		parts := strings.Split(sub, "/")
		for i, part := range parts {
			parent := strings.Join(parts[:i], "/")
			if parent == "" {
				parent = "."
			}
			named, err := anyMatch(map[string]bool{part: true}, rule.dirs)
			if err != nil {
				return nil, err
			}
			if !named {
				continue
			}
			excluded, err := anyMatch(byDir[parent], unless)
			if err != nil {
				return nil, err
			}
			if !excluded {
				candidates[parent] = true
			}
		}
	}
	withSources, err := sourceHits(byDir, rule.sources, nil)
	if err != nil {
		return nil, err
	}
	// Sources count for the innermost candidate holding them: a nested
	// Xcode project's Objective-C isn't the Swift app's above it.
	found := dirSet{}
	for sub := range withSources {
		innermost, depth := "", -1
		for c := range candidates {
			if (c == "." || under(sub, c)) && depthBelow(c, ".") > depth {
				innermost, depth = c, depthBelow(c, ".")
			}
		}
		if depth >= 0 {
			found[innermost] = true
		}
	}
	return found, nil
}

// languageRoots cut each directory at the first part naming the language
// (age/impl/c, not age/impl/c/src).
func languageRoots(directories dirSet, names []string) dirSet {
	roots := dirSet{}
	for directory := range directories {
		parts := strings.Split(directory, "/")
		for i, part := range parts {
			if contains(names, part) {
				roots[strings.Join(parts[:i+1], "/")] = true
				break
			}
		}
	}
	return roots
}

func asTemplate(root string) string {
	parts := strings.Split(root, "/")
	return strings.Join(append(parts[:len(parts)-1], "{lang}"), "/")
}

// preference orders layouts, lower first: under a preferred parent or at
// the root, then shallower.
func preference(path string) [2]int {
	parts := strings.Split(path, "/")
	preferred := len(parts) == 1
	for _, part := range parts[:len(parts)-1] {
		preferred = preferred || preferredParents[part]
	}
	if preferred {
		return [2]int{0, len(parts)}
	}
	return [2]int{1, len(parts)}
}

func lessPreference(a, b [2]int) bool {
	return a[0] < b[0] || a[0] == b[0] && a[1] < b[1]
}

// ownDirectory is a language's directory when none is named after it: the
// shallowest directory holding its files, or the common parent of several
// equally shallow ones.
func ownDirectory(directories dirSet) string {
	depth := -1
	for d := range directories {
		if n := len(strings.Split(d, "/")); depth < 0 || n < depth {
			depth = n
		}
	}
	var shallowest []string
	for _, d := range directories.sorted() {
		if len(strings.Split(d, "/")) == depth {
			shallowest = append(shallowest, d)
		}
	}
	common := shallowest[0]
	if len(shallowest) > 1 {
		common = commonPath(shallowest)
	}
	if common == "" {
		return "."
	}
	return common
}

// commonPath is Python's os.path.commonpath of relative paths.
func commonPath(paths []string) string {
	var split [][]string
	for _, path := range paths {
		var parts []string
		for _, part := range strings.Split(path, "/") {
			if part != "" && part != "." {
				parts = append(parts, part)
			}
		}
		split = append(split, parts)
	}
	common := split[0]
	for _, parts := range split[1:] {
		n := 0
		for n < len(common) && n < len(parts) && common[n] == parts[n] {
			n++
		}
		common = common[:n]
	}
	return strings.Join(common, "/")
}

// Layout is where a project's languages live: its components, implPath and
// per-language implPaths.
type Layout struct {
	Components []string
	ImplPath   string
	ImplPaths  map[string]string
}

// InferLayout is manage.py's infer_layout: `age/impl/go` and `date/impl/go`
// give components [age, date] at `{component}/impl/{lang}`; `go/` and
// `rust/` one component at `{lang}`; a language whose directory doesn't fit
// gets its own implPaths entry, unless the shared template names a
// directory holding the language's build or source files (holds, which may
// be nil) that no other language was found in.
func InferLayout(found map[string]dirSet, names map[string][]string,
	holds func(lang, directory string) (bool, error)) (Layout, error) {
	namesOf := func(lang string) []string {
		if n, ok := names[lang]; ok {
			return n
		}
		return []string{lang}
	}
	roots := map[string]dirSet{}
	templates := map[string]dirSet{}
	for lang, dirs := range found {
		roots[lang] = languageRoots(dirs, namesOf(lang))
		for root := range roots[lang] {
			template := asTemplate(root)
			if templates[template] == nil {
				templates[template] = dirSet{}
			}
			templates[template][lang] = true
		}
	}
	if len(templates) == 0 {
		// Every language at the root, unless its hooks would find no build
		// from there: one whose files are all deeper than they look.
		overrides := map[string]string{}
		for lang, dirs := range found {
			if holds == nil || !allDeeper(dirs) {
				continue
			}
			held, err := holds(lang, ".")
			if err != nil {
				return Layout{}, err
			}
			if !held {
				overrides[lang] = ownDirectory(dirs)
			}
		}
		return Layout{Components: []string{"."}, ImplPath: ".", ImplPaths: overrides}, nil
	}
	var split [][]string
	lengths := map[int]bool{}
	for template := range templates {
		parts := strings.Split(template, "/")
		split = append(split, parts)
		lengths[len(parts)] = true
	}
	if len(templates) > 1 && len(lengths) == 1 && everyLanguageHasRoots(roots) {
		var varying []int
		for i := range split[0] {
			values := map[string]bool{}
			for _, parts := range split {
				values[parts[i]] = true
			}
			if len(values) > 1 {
				varying = append(varying, i)
			}
		}
		if len(varying) == 1 {
			i := varying[0]
			components := dirSet{}
			for _, parts := range split {
				components[parts[i]] = true
			}
			pattern := append(append(append([]string(nil), split[0][:i]...), "{component}"), split[0][i+1:]...)
			return Layout{Components: components.sorted(), ImplPath: strings.Join(pattern, "/"), ImplPaths: map[string]string{}}, nil
		}
	}
	template := ""
	for candidate := range templates {
		if template == "" || betterTemplate(candidate, template, templates) {
			template = candidate
		}
	}
	overrides := map[string]string{}
	for lang, dirs := range found {
		others := dirSet{}
		for other, d := range found {
			if other != lang {
				for directory := range d {
					others[directory] = true
				}
			}
		}
		fits := false
		for _, name := range namesOf(lang) {
			candidate := strings.ReplaceAll(template, "{lang}", name)
			if roots[lang][candidate] {
				fits = true
			} else if holds != nil && !others[candidate] {
				held, err := holds(lang, candidate)
				if err != nil {
					return Layout{}, err
				}
				fits = fits || held
			}
		}
		if fits {
			continue
		}
		if len(roots[lang]) > 0 {
			best := ""
			for _, root := range roots[lang].sorted() {
				if best == "" || lessPreference(preference(root), preference(best)) {
					best = root
				}
			}
			overrides[lang] = best
		} else {
			overrides[lang] = ownDirectory(dirs)
		}
	}
	return Layout{Components: []string{"."}, ImplPath: template, ImplPaths: overrides}, nil
}

// everyLanguageHasRoots is whether each language has a directory named
// after it: components inferred from the others would leave it none.
func everyLanguageHasRoots(roots map[string]dirSet) bool {
	for _, r := range roots {
		if len(r) == 0 {
			return false
		}
	}
	return true
}

// betterTemplate orders templates as min(key=(preference, -languages, text)).
func betterTemplate(a, b string, templates map[string]dirSet) bool {
	pa, pb := preference(a), preference(b)
	if pa != pb {
		return lessPreference(pa, pb)
	}
	if len(templates[a]) != len(templates[b]) {
		return len(templates[a]) > len(templates[b])
	}
	return a < b
}

// DetectedConfig is manage.py's detected_config: the config.json hecatron
// init writes for a repository, its languages and layout as detected or as
// given (comma-separated; "" for detected), and the languages found but
// not added, with why.
func DetectedConfig(root string, home Home, components, implPath, languages string) (*Object, map[string]string, error) {
	defs, err := detectionDefinitions(home)
	if err != nil {
		return nil, nil, err
	}
	files, err := TrackedFiles(root)
	if err != nil {
		return nil, nil, err
	}
	detected, err := DetectLanguages(root, defs, files)
	if err != nil {
		return nil, nil, err
	}
	names := map[string][]string{}
	for _, d := range defs {
		names[d.lang] = d.names
	}
	byDir, _ := splitByDir(files)
	layout, err := InferLayout(detected.Found, names, holdsLanguage(byDir, defs))
	if err != nil {
		return nil, nil, err
	}
	chosenComponents := layout.Components
	if components != "" {
		chosenComponents = nil
		for _, c := range strings.Split(components, ",") {
			if c != "" {
				chosenComponents = append(chosenComponents, c)
			}
		}
	}
	if implPath != "" {
		layout.ImplPath, layout.ImplPaths = implPath, map[string]string{}
	}
	var chosen []string
	if languages != "" {
		chosen = strings.Split(languages, ",")
	} else {
		for lang := range detected.Found {
			chosen = append(chosen, lang)
		}
	}
	sort.Strings(chosen)
	config := &Object{Values: map[string]any{}}
	config.set("schema", json.Number("1"))
	config.set("hecatron", ">="+home.Version())
	config.set("project", pyBasename(root))
	if len(chosenComponents) != 1 || chosenComponents[0] != "." {
		config.set("components", stringsToAny(chosenComponents))
	}
	config.set("implPath", layout.ImplPath)
	config.set("languages", stringsToAny(chosen))
	config.set("tools", &Object{Values: map[string]any{}})
	config.set("generated", &Object{Values: map[string]any{}})
	config.set("logs", ".logs")
	implPaths := &Object{Values: map[string]any{}}
	var withPaths []string
	for lang := range layout.ImplPaths {
		withPaths = append(withPaths, lang)
	}
	sort.Strings(withPaths)
	for _, lang := range withPaths {
		if contains(chosen, lang) {
			implPaths.set(lang, layout.ImplPaths[lang])
		}
	}
	if len(implPaths.Keys) > 0 {
		config.set("implPaths", implPaths)
	}
	return config, detected.NotAdded, nil
}
