package xcode

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Project is an .xcodeproj: its build configurations, targets and shared
// schemes.
type Project struct {
	Path                 string // the .xcodeproj directory
	Dir                  string // the directory it's in: Xcode's SOURCE_ROOT
	ObjectVersion        string
	Configurations       []Configuration // the project's own, which its targets' settings fall back to
	DefaultConfiguration string
	Targets              []Target
	Schemes              []Scheme // xcshareddata/xcschemes; each user's own aren't shared
}

// Target is a native, aggregate or legacy target.
type Target struct {
	ID                   string
	Name                 string
	ProductType          string   // com.apple.product-type.application...; "" for an aggregate target
	Dependencies         []string // the IDs of the targets it depends on (an app on the extensions and helper apps it embeds)
	Configurations       []Configuration
	DefaultConfiguration string
}

// Configuration is a build configuration (Debug, Release...) of a target or
// of the project.
type Configuration struct {
	Name string
	// BaseConfig is the .xcconfig file the configuration is based on, an
	// absolute path ("" when there is none, or when its path is relative to
	// a build directory or SDK, which only Xcode knows).
	BaseConfig string
	Settings   map[string]string // array settings joined with spaces
}

// IsApp is whether the target builds an app (an app clip and a watchOS app
// are apps too).
func (t Target) IsApp() bool {
	switch t.ProductType {
	case "com.apple.product-type.application",
		"com.apple.product-type.application.on-demand-install-capable",
		"com.apple.product-type.application.watchapp2",
		"com.apple.product-type.application.watchapp2-container":
		return true
	}
	return false
}

// Embedded is whether another app target depends on the target: a helper
// app (a Mac login item, a watchOS app) the main app carries.
func (p *Project) Embedded(t *Target) bool {
	for _, other := range p.Targets {
		if other.ID != t.ID && other.IsApp() {
			for _, id := range other.Dependencies {
				if id == t.ID {
					return true
				}
			}
		}
	}
	return false
}

// Name is the project's name: its .xcodeproj's, without the extension.
func (p *Project) Name() string { return strings.TrimSuffix(filepath.Base(p.Path), ".xcodeproj") }

// Target is the target with an ID, or nil.
func (p *Project) Target(id string) *Target {
	for i := range p.Targets {
		if p.Targets[i].ID == id {
			return &p.Targets[i]
		}
	}
	return nil
}

// LoadProject reads an .xcodeproj: project.pbxproj and its shared schemes.
func LoadProject(path string) (*Project, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	text, err := os.ReadFile(filepath.Join(path, "project.pbxproj"))
	if err != nil {
		return nil, err
	}
	p, err := parseProject(string(text), path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Join(path, "project.pbxproj"), err)
	}
	p.Schemes, err = loadSchemes(filepath.Join(path, "xcshareddata", "xcschemes"), p.Dir)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// pbxproj is a parsed project.pbxproj: its objects by ID.
type pbxproj struct {
	objects map[string]map[string]any
	parents map[string]string // a file or group's ID → its group's
	dir     string
}

func parseProject(text, path string) (*Project, error) {
	root, err := parsePlist(text)
	if err != nil {
		return nil, err
	}
	top, ok := root.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("not a project: the top level isn't a dictionary")
	}
	x := &pbxproj{objects: map[string]map[string]any{}, parents: map[string]string{}, dir: filepath.Dir(path)}
	if objects, ok := top["objects"].(map[string]any); ok {
		for id, object := range objects {
			if dict, ok := object.(map[string]any); ok {
				x.objects[id] = dict
			}
		}
	}
	rootObject := x.objects[str(top["rootObject"])]
	if rootObject == nil || str(rootObject["isa"]) != "PBXProject" {
		return nil, fmt.Errorf("not a project: no PBXProject root object")
	}
	x.linkParents(str(rootObject["mainGroup"]), map[string]bool{})
	p := &Project{Path: path, Dir: x.dir, ObjectVersion: str(top["objectVersion"])}
	p.Configurations, p.DefaultConfiguration = x.configurations(str(rootObject["buildConfigurationList"]))
	for _, id := range strs(rootObject["targets"]) {
		object := x.objects[id]
		if object == nil {
			continue
		}
		t := Target{ID: id, Name: str(object["name"]), ProductType: str(object["productType"])}
		for _, dependency := range strs(object["dependencies"]) {
			if target := str(x.objects[dependency]["target"]); target != "" {
				t.Dependencies = append(t.Dependencies, target)
			}
		}
		t.Configurations, t.DefaultConfiguration = x.configurations(str(object["buildConfigurationList"]))
		p.Targets = append(p.Targets, t)
	}
	return p, nil
}

// linkParents records each group's children's parent, from the main group
// down (seen guards against a malformed file's cycles).
func (x *pbxproj) linkParents(id string, seen map[string]bool) {
	if seen[id] {
		return
	}
	seen[id] = true
	for _, child := range strs(x.objects[id]["children"]) {
		x.parents[child] = id
		x.linkParents(child, seen)
	}
}

// path is where a file or group is, absolute, from its sourceTree: relative
// to its group (<group>), to the project's directory (SOURCE_ROOT) or
// absolute. "" for a tree only Xcode can place (BUILT_PRODUCTS_DIR,
// SDKROOT, DEVELOPER_DIR...).
func (x *pbxproj) path(id string, depth int) string {
	object := x.objects[id]
	if object == nil || depth > 64 {
		return ""
	}
	own := str(object["path"])
	switch str(object["sourceTree"]) {
	case "<group>":
		parent, ok := x.parents[id]
		if !ok {
			return filepath.Join(x.dir, own) // the main group, or a file no group lists
		}
		base := x.path(parent, depth+1)
		if base == "" {
			return ""
		}
		return filepath.Join(base, own)
	case "SOURCE_ROOT":
		return filepath.Join(x.dir, own)
	case "<absolute>":
		return own
	}
	return ""
}

// configurations are a configuration list's, and its default's name.
func (x *pbxproj) configurations(listID string) ([]Configuration, string) {
	list := x.objects[listID]
	if list == nil {
		return nil, ""
	}
	var configurations []Configuration
	for _, id := range strs(list["buildConfigurations"]) {
		object := x.objects[id]
		if object == nil {
			continue
		}
		c := Configuration{Name: str(object["name"]), Settings: map[string]string{}}
		if settings, ok := object["buildSettings"].(map[string]any); ok {
			for key, value := range settings {
				c.Settings[key] = settingText(value)
			}
		}
		if ref := str(object["baseConfigurationReference"]); ref != "" {
			c.BaseConfig = x.path(ref, 0)
		} else if anchor := str(object["baseConfigurationReferenceAnchor"]); anchor != "" {
			// Xcode 16's folders synchronized with the disk: the file is
			// named relative to the folder, which lists no files.
			if base := x.path(anchor, 0); base != "" {
				c.BaseConfig = filepath.Join(base, str(object["baseConfigurationReferenceRelativePath"]))
			}
		}
		configurations = append(configurations, c)
	}
	return configurations, str(list["defaultConfigurationName"])
}

// Setting is a build setting's value for a target in a configuration, as
// far as the files say: the target's, its .xcconfig's, the project's, the
// project's .xcconfig's, each $(inherited) taking the next one's. ""
// where none sets it (Xcode's default applies, which this doesn't know).
// Conditional settings (SDKROOT[sdk=...]) aren't read.
func (p *Project) Setting(t *Target, configuration, key string) string {
	var levels []map[string]string
	add := func(configurations []Configuration) {
		for _, c := range configurations {
			if c.Name == configuration {
				levels = append(levels, c.Settings)
				if c.BaseConfig != "" {
					levels = append(levels, readXcconfig(c.BaseConfig))
				}
			}
		}
	}
	if t != nil {
		add(t.Configurations)
	}
	add(p.Configurations)
	value := ""
	for i := len(levels) - 1; i >= 0; i-- {
		if v, ok := levels[i][key]; ok {
			value = strings.TrimSpace(strings.ReplaceAll(v, "$(inherited)", value))
		}
	}
	return value
}

// BaseConfigs are the .xcconfig files the project's and the targets'
// configurations are based on, sorted, each once.
func (p *Project) BaseConfigs(targets []*Target) []string {
	seen := map[string]bool{}
	var paths []string
	add := func(configurations []Configuration) {
		for _, c := range configurations {
			if c.BaseConfig != "" && !seen[c.BaseConfig] {
				seen[c.BaseConfig] = true
				paths = append(paths, c.BaseConfig)
			}
		}
	}
	add(p.Configurations)
	for _, t := range targets {
		add(t.Configurations)
	}
	sort.Strings(paths)
	return paths
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func strs(v any) []string {
	var result []string
	if array, ok := v.([]any); ok {
		for _, item := range array {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
	}
	return result
}

// settingText is a build setting's value as a string: an array's items
// joined with spaces, as Xcode passes them.
func settingText(v any) string {
	if array, ok := v.([]any); ok {
		parts := make([]string, 0, len(array))
		for _, item := range array {
			parts = append(parts, str(item))
		}
		return strings.Join(parts, " ")
	}
	return str(v)
}
