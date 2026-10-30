package xcode

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Scheme is a shared .xcscheme: what it builds, runs and tests.
type Scheme struct {
	Name             string
	Path             string
	Build            []BuildableRef // the build action's entries that build for running
	Run              *BuildableRef  // the launch action's runnable
	RunConfiguration string         // the launch action's build configuration
	Tests            []BuildableRef // the test action's testables, less skipped ones
	TestPlans        []string       // the test action's test plans (absolute paths), the default first
}

// BuildableRef is a target a scheme names, in a project.
type BuildableRef struct {
	TargetID   string // BlueprintIdentifier
	TargetName string // BlueprintName
	Product    string // BuildableName: Foo.app, Foo.framework...
	Project    string // the .xcodeproj it's in, absolute
}

type xmlBuildableRef struct {
	BlueprintIdentifier string `xml:"BlueprintIdentifier,attr"`
	BuildableName       string `xml:"BuildableName,attr"`
	BlueprintName       string `xml:"BlueprintName,attr"`
	ReferencedContainer string `xml:"ReferencedContainer,attr"`
}

type xmlScheme struct {
	BuildAction struct {
		Entries []struct {
			BuildForRunning string          `xml:"buildForRunning,attr"`
			Ref             xmlBuildableRef `xml:"BuildableReference"`
		} `xml:"BuildActionEntries>BuildActionEntry"`
	} `xml:"BuildAction"`
	TestAction struct {
		Testables []struct {
			Skipped string          `xml:"skipped,attr"`
			Ref     xmlBuildableRef `xml:"BuildableReference"`
		} `xml:"Testables>TestableReference"`
		Plans []struct {
			Reference string `xml:"reference,attr"`
			Default   string `xml:"default,attr"`
		} `xml:"TestPlans>TestPlanReference"`
	} `xml:"TestAction"`
	LaunchAction struct {
		BuildConfiguration string           `xml:"buildConfiguration,attr"`
		Runnable           *xmlBuildableRef `xml:"BuildableProductRunnable>BuildableReference"`
	} `xml:"LaunchAction"`
}

// loadSchemes reads the .xcscheme files in a directory (none when it's
// missing); their containers are relative to base, the directory the
// project or workspace is in.
func loadSchemes(dir, base string) ([]Scheme, error) {
	paths, _ := filepath.Glob(filepath.Join(dir, "*.xcscheme"))
	sort.Strings(paths)
	var schemes []Scheme
	for _, path := range paths {
		text, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var x xmlScheme
		if err := xml.Unmarshal(text, &x); err != nil {
			continue // a scheme Xcode itself couldn't read
		}
		s := Scheme{
			Name:             strings.TrimSuffix(filepath.Base(path), ".xcscheme"),
			Path:             path,
			RunConfiguration: x.LaunchAction.BuildConfiguration,
		}
		for _, e := range x.BuildAction.Entries {
			if e.BuildForRunning != "NO" {
				s.Build = append(s.Build, buildableRef(e.Ref, base))
			}
		}
		if x.LaunchAction.Runnable != nil {
			run := buildableRef(*x.LaunchAction.Runnable, base)
			s.Run = &run
		}
		for _, t := range x.TestAction.Testables {
			if t.Skipped != "YES" {
				s.Tests = append(s.Tests, buildableRef(t.Ref, base))
			}
		}
		for _, plan := range x.TestAction.Plans {
			path := containerPath(plan.Reference, base)
			if plan.Default == "YES" {
				s.TestPlans = append([]string{path}, s.TestPlans...)
			} else {
				s.TestPlans = append(s.TestPlans, path)
			}
		}
		schemes = append(schemes, s)
	}
	return schemes, nil
}

func buildableRef(x xmlBuildableRef, base string) BuildableRef {
	return BuildableRef{
		TargetID:   x.BlueprintIdentifier,
		TargetName: x.BlueprintName,
		Product:    x.BuildableName,
		Project:    containerPath(x.ReferencedContainer, base),
	}
}

// containerPath is a scheme's "container:Foo.xcodeproj" as an absolute
// path, relative to base.
func containerPath(reference, base string) string {
	kind, path, ok := strings.Cut(reference, ":")
	if !ok {
		path, kind = reference, "container"
	}
	if kind == "absolute" || filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(base, path)
}

// Workspace is an .xcworkspace: the projects it opens together (CocoaPods'
// puts the app's project beside Pods/Pods.xcodeproj) and its shared
// schemes.
type Workspace struct {
	Path     string
	Dir      string
	Projects []string // the .xcodeproj files it names, absolute, whether or not they exist
	Schemes  []Scheme
}

// Name is the workspace's name: its .xcworkspace's, without the extension.
func (w *Workspace) Name() string { return strings.TrimSuffix(filepath.Base(w.Path), ".xcworkspace") }

type xmlWorkspaceItem struct {
	XMLName  xml.Name
	Location string             `xml:"location,attr"`
	Items    []xmlWorkspaceItem `xml:",any"`
}

// LoadWorkspace reads an .xcworkspace's contents.xcworkspacedata and shared
// schemes.
func LoadWorkspace(path string) (*Workspace, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	text, err := os.ReadFile(filepath.Join(path, "contents.xcworkspacedata"))
	if err != nil {
		return nil, err
	}
	var root xmlWorkspaceItem
	if err := xml.Unmarshal(text, &root); err != nil {
		return nil, err
	}
	w := &Workspace{Path: path, Dir: filepath.Dir(path)}
	w.collect(root.Items, w.Dir)
	w.Schemes, err = loadSchemes(filepath.Join(path, "xcshareddata", "xcschemes"), w.Dir)
	if err != nil {
		return nil, err
	}
	return w, nil
}

// collect records the projects among a group's items. A location is
// "group:" (relative to the enclosing group), "container:" (to the
// workspace's directory), "absolute:", or "self:" (a project's own
// workspace, which names nothing else).
func (w *Workspace) collect(items []xmlWorkspaceItem, group string) {
	for _, item := range items {
		kind, path, _ := strings.Cut(item.Location, ":")
		var where string
		switch kind {
		case "group":
			where = filepath.Join(group, path)
		case "container":
			where = filepath.Join(w.Dir, path)
		case "absolute":
			where = filepath.Clean(path)
		default:
			where = group
		}
		switch item.XMLName.Local {
		case "Group":
			w.collect(item.Items, where)
		case "FileRef":
			if strings.HasSuffix(where, ".xcodeproj") {
				w.Projects = append(w.Projects, where)
			}
		}
	}
}
