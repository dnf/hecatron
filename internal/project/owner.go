package project

import "strings"

// Owners maps paths to the language and component whose implementation
// directory holds them: the deepest such directory, so a component nested
// in another's tree is its own.
type Owners struct {
	dirs []ownedDir
}

type ownedDir struct {
	dir, lang, component string
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
			o.dirs = append(o.dirs, ownedDir{strings.TrimSuffix(d.Dir, "/"), lang, d.Component})
		}
	}
	return o, nil
}

// Owner is the language and component an absolute path belongs to; ok is
// false for a path in no language's directory.
func (o *Owners) Owner(path string) (lang, component string, ok bool) {
	best := -1
	for i, d := range o.dirs {
		if (path == d.dir || strings.HasPrefix(path, d.dir+"/") || d.dir == "/") && (best < 0 || len(d.dir) > len(o.dirs[best].dir)) {
			best = i
		}
	}
	if best < 0 {
		return "", "", false
	}
	return o.dirs[best].lang, o.dirs[best].component, true
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
