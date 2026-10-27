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
