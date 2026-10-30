package project

import (
	pathpkg "path"
	"strings"
)

// Owners maps paths to the language and component whose implementation
// directory holds them: the deepest such directory, so a component nested
// in another's tree is its own. A directory holding another component's
// (a root Cargo workspace beside clients/go) claims only the files that are
// its language's (see claimed); any other file in it falls to a directory
// above that claims it, if any. A directory holding no other component
// claims everything in it, at the root too: a single package's fixtures,
// templates and assets are its changes.
type Owners struct {
	dirs []ownedDir
}

type ownedDir struct {
	dir, lang, component string
	claims               []string // the patterns it's limited to; nil: everything in it
}

// Owners reads every language's implementation directories.
func (p *Project) Owners() (*Owners, error) {
	o := &Owners{}
	for _, lang := range p.Languages {
		dirs, err := p.ImplDirs(lang)
		if err != nil {
			return nil, err
		}
		for _, d := range dirs {
			o.dirs = append(o.dirs, ownedDir{dir: strings.TrimSuffix(d.Dir, "/"), lang: lang, component: d.Component})
		}
	}
	for i, d := range o.dirs {
		if !o.holdsAnother(i) {
			continue
		}
		claims, err := p.LanguageFiles(d.lang)
		if err != nil {
			return nil, err
		}
		o.dirs[i].claims = claims
	}
	return o, nil
}

// holdsAnother is whether another component's directory, in any language,
// sits below the i-th one.
func (o *Owners) holdsAnother(i int) bool {
	for _, d := range o.dirs {
		if d.dir != o.dirs[i].dir && within(d.dir, o.dirs[i].dir) {
			return true
		}
	}
	return false
}

// within reports whether an absolute path is dir or below it.
func within(path, dir string) bool {
	return path == dir || strings.HasPrefix(path, dir+"/") || dir == "/"
}

// Owner is the language and component an absolute path belongs to; ok is
// false for a path in no language's directory, or only in ones that don't
// claim it.
func (o *Owners) Owner(path string) (lang, component string, ok bool) {
	best := -1
	for i, d := range o.dirs {
		if within(path, d.dir) && d.claimed(path) && (best < 0 || len(d.dir) > len(o.dirs[best].dir)) {
			best = i
		}
	}
	if best < 0 {
		return "", "", false
	}
	return o.dirs[best].lang, o.dirs[best].component, true
}

// claimed is whether a path in the directory is its language's: anything
// in a directory of its own; in one at the root or holding other
// components, only what its language's file patterns match. Otherwise a
// Cargo workspace at the root would own a crash dump or an editor's files
// beside it, and hct test --changed would run Rust for them.
func (d ownedDir) claimed(path string) bool {
	if d.claims == nil {
		return true
	}
	rel := strings.TrimPrefix(strings.TrimPrefix(path, d.dir), "/")
	return MatchesLanguageFile(rel, d.claims)
}

// LanguageFiles are the file patterns that are a language's own, for
// attributing a change in a directory it shares with others (the root):
// its definition's detect, detectSources and sourceFiles, and the names of
// its pin and support files. nil, claiming everything, for a definition
// without "sourceFiles", whose files can't be told apart.
func (p *Project) LanguageFiles(lang string) ([]string, error) {
	sources, err := p.definitionStrings(lang, "sourceFiles")
	if err != nil || len(sources) == 0 {
		return nil, err
	}
	var patterns []string
	for _, key := range []string{"detect", "detectSources"} {
		list, err := p.definitionStrings(lang, key)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, list...)
	}
	versionFiles, err := p.VersionFiles(lang)
	if err != nil {
		return nil, err
	}
	for _, file := range versionFiles {
		if !strings.Contains(file, "/") { // .github/workflows/*.yml is read for a version, not the language's code
			patterns = append(patterns, file)
		}
	}
	return append(patterns, sources...), nil
}

// MatchesLanguageFile reports whether a relative path matches one of a
// language's file patterns: a glob without a slash against its name, a
// directory name with a trailing slash (gradle/, testdata/) against every
// directory it's in, any other glob with a slash against the whole path.
func MatchesLanguageFile(rel string, patterns []string) bool {
	for _, pattern := range patterns {
		if dir, ok := strings.CutSuffix(pattern, "/"); ok && !strings.Contains(dir, "/") {
			if strings.Contains("/"+rel, "/"+dir+"/") {
				return true
			}
			continue
		}
		name := rel
		if !strings.Contains(pattern, "/") {
			name = pathpkg.Base(rel)
		}
		if matched, _ := pathpkg.Match(pattern, name); matched {
			return true
		}
	}
	return false
}

// BuildFiles are the file name patterns that say how a language's code is
// built or which toolchain it uses: its definition's detect and
// sharedBuildFiles, and the files its versions are read from (pinFiles,
// supportFiles). A pattern with a slash is relative to the root.
func (p *Project) BuildFiles(lang string) ([]string, error) {
	var patterns []string
	for _, key := range []string{"detect", "sharedBuildFiles"} {
		list, err := p.definitionStrings(lang, key)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, list...)
	}
	versionFiles, err := p.VersionFiles(lang)
	if err != nil {
		return nil, err
	}
	return append(patterns, versionFiles...), nil
}

// VersionFiles are the file name patterns a language's versions are read
// from: its definition's pinFiles and supportFiles, each looked for from a
// component's directory up to the root (a pattern with a slash, such as
// .github/workflows/*.yml, relative to the directory it's looked for in).
func (p *Project) VersionFiles(lang string) ([]string, error) {
	var patterns []string
	for _, key := range []string{"pinFiles", "supportFiles"} {
		entries, err := p.fileEntries(lang, key)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			patterns = append(patterns, entry.file)
		}
	}
	return patterns, nil
}

// TaskPaths are config.json's "taskPaths": for a project task, by name, the
// paths whose changes it depends on (hct test --changed); ok is false for
// a task that declares none.
func (p *Project) TaskPaths(task string) (paths []string, ok bool, err error) {
	value, present := p.config.Get("taskPaths")
	if !present || value == nil {
		return nil, false, nil
	}
	object, isObject := value.(*Object)
	if !isObject {
		return nil, false, &Error{`"taskPaths" must be an object of task names to lists of paths`}
	}
	list, present := object.Get(task)
	if !present {
		return nil, false, nil
	}
	paths, err = toStrings(list, "taskPaths."+task)
	if err != nil {
		return nil, false, &Error{`taskPaths.` + task + ` must be a list of paths`}
	}
	return paths, true, nil
}
