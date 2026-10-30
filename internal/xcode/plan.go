package xcode

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Plan is how to build the Xcode project in a directory, as far as its
// files tell: the container (a workspace before a project: CocoaPods' app
// doesn't link without the workspace's Pods), the scheme that builds its
// app, the generic destination for the app's platform, and the files the
// build needs that aren't there yet. Paths are relative to the directory.
type Plan struct {
	Kind        string   `json:"kind,omitempty"` // "-workspace" or "-project"
	Container   string   `json:"container,omitempty"`
	Scheme      string   `json:"scheme,omitempty"`
	Destination string   `json:"destination,omitempty"`
	Missing     []string `json:"missing,omitempty"` // .xcconfig files and projects named but absent
	Setup       string   `json:"setup,omitempty"`   // what makes the missing files
	// Unsure says why Container, Scheme or Destination is empty: then
	// xcodebuild (or the project's own hook) has to decide.
	Unsure string `json:"unsure,omitempty"`
	// Nested are the Xcode projects and workspaces below the directory,
	// when there are none in it.
	Nested []string `json:"nested,omitempty"`
}

// Destinations for each SDK: a generic device of the platform, a simulator
// for those that have one (a simulator build needs no signing or
// provisioning).
var destinations = map[string]string{
	"macosx":           "generic/platform=macOS",
	"iphoneos":         "generic/platform=iOS Simulator",
	"iphonesimulator":  "generic/platform=iOS Simulator",
	"appletvos":        "generic/platform=tvOS Simulator",
	"appletvsimulator": "generic/platform=tvOS Simulator",
	"watchos":          "generic/platform=watchOS Simulator",
	"watchsimulator":   "generic/platform=watchOS Simulator",
	"xros":             "generic/platform=visionOS Simulator",
	"xrsimulator":      "generic/platform=visionOS Simulator",
}

// MakePlan plans the build of the Xcode project in dir. names are what a
// container may be named like when there are several (the directory, the
// project, the repository), in order.
func MakePlan(dir string, names ...string) (*Plan, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	workspaces, projects := containers(dir)
	plan := &Plan{}
	if len(workspaces)+len(projects) == 0 {
		for _, path := range Below(dir) {
			plan.Nested = append(plan.Nested, relative(dir, path))
		}
		plan.Unsure = "no Xcode project or workspace here"
		return plan, nil
	}
	plan.Kind = "-workspace"
	container, unsure := pick(workspaces, ".xcworkspace", names)
	if len(workspaces) == 0 {
		plan.Kind = "-project"
		container, unsure = pick(projects, ".xcodeproj", names)
	}
	if container == "" {
		plan.Kind, plan.Unsure = "", unsure
		return plan, nil
	}
	plan.Container = relative(dir, container)
	set, err := loadSet(container)
	if err != nil {
		return nil, err
	}
	for _, missing := range set.missingProjects {
		plan.Missing = append(plan.Missing, relative(dir, missing))
	}
	scheme, unsure := set.appScheme(strings.TrimSuffix(filepath.Base(container), filepath.Ext(container)), names)
	if scheme == nil {
		plan.Unsure = unsure
	} else {
		plan.Scheme = scheme.Name
		plan.Destination, plan.Unsure = set.destination(scheme)
	}
	for _, path := range set.baseConfigs(scheme) {
		if _, err := os.Stat(path); err != nil {
			plan.Missing = append(plan.Missing, relative(dir, path))
		}
	}
	sort.Strings(plan.Missing)
	plan.Setup = setup(plan.Missing)
	return plan, nil
}

// containers are the workspaces and projects directly in dir. A project's
// own workspace (Foo.xcodeproj/project.xcworkspace) is inside the project,
// so it isn't one.
func containers(dir string) (workspaces, projects []string) {
	workspaces, _ = filepath.Glob(filepath.Join(dir, "*.xcworkspace"))
	projects, _ = filepath.Glob(filepath.Join(dir, "*.xcodeproj"))
	return workspaces, projects
}

// skippedBelow are directories Below doesn't look in: dependencies,
// vendored code (firefox-ios/ThirdParty/Deferred), build output and
// version control.
var skippedBelow = map[string]bool{
	"Pods": true, "Carthage": true, "build": true, "DerivedData": true, "node_modules": true,
	"vendor": true, "Vendor": true, "ThirdParty": true, "third_party": true, "third-party": true,
	"External": true, "external": true, ".build": true, ".git": true, ".swiftpm": true,
}

// Below are the Xcode projects and workspaces below dir (not in it), the
// outermost of each tree, outside dependencies and build output: an app
// kept in a subdirectory (GoMap's src/), or apps beside a repository's
// Swift package (firefox-ios).
func Below(dir string) []string {
	var found []string
	var walk func(string, int)
	walk = func(path string, depth int) {
		entries, err := os.ReadDir(path)
		if err != nil || depth > 4 {
			return
		}
		var here []string
		for _, e := range entries {
			name := e.Name()
			if !e.IsDir() {
				continue
			}
			if strings.HasSuffix(name, ".xcodeproj") || strings.HasSuffix(name, ".xcworkspace") {
				here = append(here, filepath.Join(path, name))
			}
		}
		if depth > 0 && len(here) > 0 {
			found = append(found, here...)
			return
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() && !skippedBelow[name] && !strings.HasPrefix(name, ".") &&
				!strings.HasSuffix(name, ".xcodeproj") && !strings.HasSuffix(name, ".xcworkspace") {
				walk(filepath.Join(path, name), depth+1)
			}
		}
	}
	walk(dir, 0)
	sort.Strings(found)
	return found
}

// pick is the container to build among those of one kind: the only one,
// else the one named like the first of names that one is named like.
func pick(paths []string, extension string, names []string) (string, string) {
	switch len(paths) {
	case 0:
		return "", ""
	case 1:
		return paths[0], ""
	}
	for _, name := range names {
		for _, path := range paths {
			if name != "" && filepath.Base(path) == name+extension {
				return path, ""
			}
		}
	}
	bases := make([]string, len(paths))
	for i, path := range paths {
		bases[i] = filepath.Base(path)
	}
	kind := "project"
	if extension == ".xcworkspace" {
		kind = "workspace"
	}
	return "", fmt.Sprintf("several Xcode %ss here (%s), none named like the directory", kind, strings.Join(bases, ", "))
}

// set is what a container builds from: its projects (a workspace's that
// exist, or the one project) and the schemes they and the workspace share.
type set struct {
	projects        []*Project
	schemes         []Scheme
	missingProjects []string
}

func loadSet(container string) (*set, error) {
	s := &set{}
	if strings.HasSuffix(container, ".xcworkspace") {
		w, err := LoadWorkspace(container)
		if err != nil {
			return nil, err
		}
		s.schemes = append(s.schemes, w.Schemes...)
		for _, path := range w.Projects {
			if _, err := os.Stat(filepath.Join(path, "project.pbxproj")); err != nil {
				s.missingProjects = append(s.missingProjects, path)
				continue
			}
			p, err := LoadProject(path)
			if err != nil {
				return nil, err
			}
			s.projects = append(s.projects, p)
		}
	} else {
		p, err := LoadProject(container)
		if err != nil {
			return nil, err
		}
		s.projects = append(s.projects, p)
	}
	for _, p := range s.projects {
		schemes := p.Schemes
		if len(schemes) == 0 {
			schemes = autoSchemes(p)
		}
		s.schemes = append(s.schemes, schemes...)
	}
	return s, nil
}

// autoSchemes are the schemes Xcode makes for a project that shares none:
// one per target, named after it.
func autoSchemes(p *Project) []Scheme {
	schemes := make([]Scheme, 0, len(p.Targets))
	for _, t := range p.Targets {
		ref := BuildableRef{TargetID: t.ID, TargetName: t.Name, Project: p.Path}
		s := Scheme{Name: t.Name, Build: []BuildableRef{ref}, RunConfiguration: "Debug"}
		if t.IsApp() {
			s.Run = &ref
		}
		schemes = append(schemes, s)
	}
	return schemes
}

// target is the project and target a reference names, or nils.
func (s *set) target(ref BuildableRef) (*Project, *Target) {
	for _, p := range s.projects {
		if p.Path == ref.Project {
			if t := p.Target(ref.TargetID); t != nil {
				return p, t
			}
		}
	}
	return nil, nil
}

// appTarget is the app a scheme is for: its first build entry (else its
// runnable), when that's an app. An extension's scheme builds its host app
// too, and runs it, but lists the extension first.
func (s *set) appTarget(scheme *Scheme) (*Project, *Target) {
	var first BuildableRef
	switch {
	case len(scheme.Build) > 0:
		first = scheme.Build[0]
	case scheme.Run != nil:
		first = *scheme.Run
	default:
		return nil, nil
	}
	if p, t := s.target(first); t != nil && t.IsApp() {
		return p, t
	}
	return nil, nil
}

// appScheme is the scheme that builds the container's app. Not "the only
// scheme": a Swift package the project uses adds one (NetNewsWire lists
// 27), and CocoaPods' workspace one per pod. Of the schemes that build an
// app: the one named like the container (or like names), else the only
// one, else the only one for an app no other app embeds (not
// home-assistant's watchOS app or Mac launcher), else the only one that
// runs Debug (home-assistant's App-Debug, not
// App-Release), else the only one named after its app (App, App-Debug). Without one that
// builds an app (a framework's project), the scheme named like the
// container.
func (s *set) appScheme(container string, names []string) (*Scheme, string) {
	var apps []*Scheme
	for i := range s.schemes {
		if _, t := s.appTarget(&s.schemes[i]); t != nil {
			apps = append(apps, &s.schemes[i])
		}
	}
	named := func(candidates []*Scheme) *Scheme {
		for _, name := range append([]string{container}, names...) {
			for _, scheme := range candidates {
				if name != "" && scheme.Name == name {
					return scheme
				}
			}
		}
		return nil
	}
	if scheme := named(apps); scheme != nil {
		return scheme, ""
	}
	if len(apps) == 1 {
		return apps[0], ""
	}
	if len(apps) == 0 {
		all := make([]*Scheme, len(s.schemes))
		for i := range s.schemes {
			all[i] = &s.schemes[i]
		}
		if scheme := named(all); scheme != nil {
			return scheme, ""
		}
		return nil, "no scheme builds an app, and none is named like " + container
	}
	// Each rule narrows what the one before left, until one scheme is left.
	for _, keep := range []func(*Scheme) bool{
		func(scheme *Scheme) bool {
			p, t := s.appTarget(scheme)
			return !p.Embedded(t)
		},
		func(scheme *Scheme) bool { return scheme.RunConfiguration == "Debug" },
		func(scheme *Scheme) bool {
			_, t := s.appTarget(scheme)
			return scheme.Name == t.Name || strings.HasPrefix(scheme.Name, t.Name+"-") || strings.HasPrefix(scheme.Name, t.Name+" ")
		},
	} {
		switch kept := filterSchemes(apps, keep); len(kept) {
		case 1:
			return kept[0], ""
		case 0:
		default:
			apps = kept
		}
	}
	candidates := make([]string, len(apps))
	for i, scheme := range apps {
		candidates[i] = scheme.Name
	}
	return nil, "several schemes build an app (" + strings.Join(candidates, ", ") + ")"
}

func filterSchemes(schemes []*Scheme, keep func(*Scheme) bool) []*Scheme {
	var kept []*Scheme
	for _, scheme := range schemes {
		if keep(scheme) {
			kept = append(kept, scheme)
		}
	}
	return kept
}

// destination is the generic destination for the platform the scheme's
// app (else its first target) is for: its SDKROOT, a Mac app's macOS, an
// iOS app's simulator even where it also builds for the Mac with Catalyst
// (a separate build, which its dependencies may not support). For a
// multiplatform target (SDKROOT auto), its SUPPORTED_PLATFORMS, the Mac
// first. Empty, saying why, when the files don't tell.
func (s *set) destination(scheme *Scheme) (string, string) {
	p, t := s.appTarget(scheme)
	if t == nil && len(scheme.Build) > 0 {
		p, t = s.target(scheme.Build[0])
	}
	if t == nil {
		return "", "scheme " + scheme.Name + " names no target in these projects"
	}
	configuration := scheme.RunConfiguration
	if configuration == "" {
		configuration = t.DefaultConfiguration
	}
	sdk := p.Setting(t, configuration, "SDKROOT")
	if destination, ok := destinations[sdk]; ok {
		return destination, ""
	}
	if sdk == "auto" || sdk == "" {
		platforms := " " + p.Setting(t, configuration, "SUPPORTED_PLATFORMS") + " "
		for _, platform := range []string{"macosx", "iphonesimulator", "iphoneos", "xrsimulator", "appletvsimulator", "watchsimulator"} {
			if strings.Contains(platforms, " "+platform+" ") {
				return destinations[platform], ""
			}
		}
	}
	if sdk == "" {
		return "", "target " + t.Name + " doesn't set SDKROOT in the files hecatron reads"
	}
	return "", "target " + t.Name + "'s SDKROOT is " + sdk
}

// baseConfigs are the .xcconfig files a build of the scheme reads: those of
// the projects' configurations and of the targets it builds (every
// target's, without a scheme).
func (s *set) baseConfigs(scheme *Scheme) []string {
	seen := map[string]bool{}
	var paths []string
	for _, p := range s.projects {
		var targets []*Target
		for i := range p.Targets {
			t := &p.Targets[i]
			if scheme == nil || buildsTarget(scheme, p, t) {
				targets = append(targets, t)
			}
		}
		for _, path := range p.BaseConfigs(targets) {
			if !seen[path] {
				seen[path] = true
				paths = append(paths, path)
			}
		}
	}
	return paths
}

func buildsTarget(scheme *Scheme, p *Project, t *Target) bool {
	refs := scheme.Build
	if scheme.Run != nil {
		refs = append(refs, *scheme.Run)
	}
	for _, ref := range refs {
		if ref.Project == p.Path && ref.TargetID == t.ID {
			return true
		}
	}
	return false
}

// setup is what makes the missing files: CocoaPods' and Carthage's
// installs, else a step only the project's README knows.
func setup(missing []string) string {
	if len(missing) == 0 {
		return ""
	}
	for _, path := range missing {
		if strings.HasPrefix(path, "Pods/") {
			return "pod install"
		}
	}
	for _, path := range missing {
		if strings.HasPrefix(path, "Carthage/") {
			return "carthage bootstrap"
		}
	}
	return "the project's own setup (see its README)"
}

func relative(dir, path string) string {
	if rel, err := filepath.Rel(dir, path); err == nil {
		return rel
	}
	return path
}
