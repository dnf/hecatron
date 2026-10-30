package xcode

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// app describes the project appProject writes: an app (APP0) with an
// extension (EXT0) and a helper app it embeds (HLP0), the project's
// configurations based on Config/Base.xcconfig, in a group.
type app struct {
	sdk       string // the app target's SDKROOT, "" for none
	catalyst  bool
	platforms string // its SUPPORTED_PLATFORMS
	anchored  bool   // the base config named through a synchronized folder (Xcode 16)
}

func appProject(a app) string {
	settings := ""
	if a.sdk != "" {
		settings += fmt.Sprintf("SDKROOT = %s; ", a.sdk)
	}
	if a.catalyst {
		settings += "SUPPORTS_MACCATALYST = YES; "
	}
	if a.platforms != "" {
		settings += fmt.Sprintf("SUPPORTED_PLATFORMS = \"%s\"; ", a.platforms)
	}
	base := "baseConfigurationReference = CFG0 /* Base.xcconfig */;"
	if a.anchored {
		base = "baseConfigurationReferenceAnchor = SYN0; baseConfigurationReferenceRelativePath = Base.xcconfig;"
	}
	return `// !$*UTF8*$!
{
	archiveVersion = 1;
	objectVersion = 77;
	objects = {
		PRJ0 = {isa = PBXProject; mainGroup = GRP0; buildConfigurationList = PCL0; targets = (APP0, EXT0, HLP0, ); };
		GRP0 = {isa = PBXGroup; children = (GRP1, SYN0, ); sourceTree = "<group>"; };
		GRP1 = {isa = PBXGroup; path = Config; children = (CFG0, ); sourceTree = "<group>"; };
		SYN0 = {isa = PBXFileSystemSynchronizedRootGroup; path = Config; sourceTree = "<group>"; };
		CFG0 = {isa = PBXFileReference; path = Base.xcconfig; sourceTree = "<group>"; };
		PCL0 = {isa = XCConfigurationList; buildConfigurations = (PDB0, PRL0, ); defaultConfigurationName = Release; };
		PDB0 = {isa = XCBuildConfiguration; name = Debug; ` + base + ` buildSettings = {OTHER = ("$(inherited)", b, ); }; };
		PRL0 = {isa = XCBuildConfiguration; name = Release; ` + base + ` buildSettings = {}; };
		APP0 = {isa = PBXNativeTarget; name = App; productType = "com.apple.product-type.application"; buildConfigurationList = ACL0; dependencies = (DEP0, DEP1, ); };
		DEP0 = {isa = PBXTargetDependency; target = EXT0; };
		DEP1 = {isa = PBXTargetDependency; target = HLP0; };
		ACL0 = {isa = XCConfigurationList; buildConfigurations = (ADB0, ); defaultConfigurationName = Debug; };
		ADB0 = {isa = XCBuildConfiguration; name = Debug; buildSettings = {` + settings + `}; };
		EXT0 = {isa = PBXNativeTarget; name = Ext; productType = "com.apple.product-type.app-extension"; buildConfigurationList = ECL0; };
		ECL0 = {isa = XCConfigurationList; buildConfigurations = (EDB0, ); };
		EDB0 = {isa = XCBuildConfiguration; name = Debug; buildSettings = {SDKROOT = iphoneos; }; };
		HLP0 = {isa = PBXNativeTarget; name = Helper; productType = "com.apple.product-type.application"; buildConfigurationList = HCL0; };
		HCL0 = {isa = XCConfigurationList; buildConfigurations = (HDB0, ); };
		HDB0 = {isa = XCBuildConfiguration; name = Debug; buildSettings = {SDKROOT = macosx; }; };
	};
	rootObject = PRJ0;
}
`
}

// ref is a scheme's reference to a target of App.xcodeproj.
type ref struct{ id, name string }

var (
	appRef    = ref{"APP0", "App"}
	extRef    = ref{"EXT0", "Ext"}
	helperRef = ref{"HLP0", "Helper"}
)

func scheme(container string, run *ref, configuration string, build ...ref) string {
	entry := func(r ref) string {
		return fmt.Sprintf(`<BuildableReference BuildableIdentifier="primary" BlueprintIdentifier="%s" BuildableName="%s.app" BlueprintName="%s" ReferencedContainer="container:%s"/>`,
			r.id, r.name, r.name, container)
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n<Scheme>\n<BuildAction><BuildActionEntries>\n")
	for _, r := range build {
		fmt.Fprintf(&b, `<BuildActionEntry buildForTesting="YES" buildForRunning="YES">%s</BuildActionEntry>`+"\n", entry(r))
	}
	b.WriteString("</BuildActionEntries></BuildAction>\n")
	b.WriteString(`<TestAction><Testables><TestableReference skipped="NO">` + entry(ref{"TST0", "AppTests"}) +
		`</TestableReference><TestableReference skipped="YES">` + entry(ref{"TST1", "Slow"}) + `</TestableReference></Testables>` +
		`<TestPlans><TestPlanReference reference="container:Other.xctestplan"/><TestPlanReference reference="container:App.xctestplan" default="YES"/></TestPlans></TestAction>` + "\n")
	fmt.Fprintf(&b, `<LaunchAction buildConfiguration="%s">`, configuration)
	if run != nil {
		b.WriteString("<BuildableProductRunnable>" + entry(*run) + "</BuildableProductRunnable>")
	}
	b.WriteString("</LaunchAction>\n</Scheme>\n")
	return b.String()
}

// project writes App.xcodeproj in dir with schemes (name → text).
func project(t *testing.T, dir string, a app, schemes map[string]string) {
	t.Helper()
	write(t, filepath.Join(dir, "App.xcodeproj", "project.pbxproj"), appProject(a))
	for name, text := range schemes {
		write(t, filepath.Join(dir, "App.xcodeproj", "xcshareddata", "xcschemes", name+".xcscheme"), text)
	}
}

func plan(t *testing.T, dir string, names ...string) *Plan {
	t.Helper()
	p, err := MakePlan(dir, names...)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadProject(t *testing.T) {
	dir := t.TempDir()
	project(t, dir, app{sdk: "iphoneos"}, map[string]string{"App": scheme("App.xcodeproj", &appRef, "Debug", appRef, extRef)})
	write(t, filepath.Join(dir, "Config", "Base.xcconfig"), `#include "Shared.xcconfig"
SDKROOT = macosx // a comment
OTHER = $(inherited) c
FLAG[sdk=iphoneos*] = conditional
`)
	write(t, filepath.Join(dir, "Config", "Shared.xcconfig"), "OTHER = from-shared\nNAME = shared\n#include? \"Missing.xcconfig\"\n")
	p, err := LoadProject(filepath.Join(dir, "App.xcodeproj"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != "App" || p.ObjectVersion != "77" || p.DefaultConfiguration != "Release" {
		t.Errorf("got %q, %q, %q", p.Name(), p.ObjectVersion, p.DefaultConfiguration)
	}
	var names []string
	for _, target := range p.Targets {
		names = append(names, target.Name)
	}
	if !reflect.DeepEqual(names, []string{"App", "Ext", "Helper"}) {
		t.Errorf("targets: %v", names)
	}
	appTarget, helper := &p.Targets[0], &p.Targets[2]
	if !appTarget.IsApp() || p.Targets[1].IsApp() || !reflect.DeepEqual(appTarget.Dependencies, []string{"EXT0", "HLP0"}) {
		t.Errorf("app target: %+v", appTarget)
	}
	if p.Embedded(appTarget) || !p.Embedded(helper) {
		t.Errorf("only the helper app is embedded")
	}
	if got := p.Configurations[0].BaseConfig; got != filepath.Join(dir, "Config", "Base.xcconfig") {
		t.Errorf("base config: %s", got)
	}
	for key, want := range map[string]string{
		"SDKROOT": "iphoneos",        // the target's, over the project's .xcconfig
		"OTHER":   "from-shared c b", // the project's $(inherited) is its .xcconfig's, which includes another
		"NAME":    "shared",
		"FLAG":    "", // conditional
		"MISSING": "",
	} {
		if got := p.Setting(appTarget, "Debug", key); got != want {
			t.Errorf("%s: got %q, want %q", key, got, want)
		}
	}
	if len(p.Schemes) != 1 {
		t.Fatalf("schemes: %+v", p.Schemes)
	}
	s := p.Schemes[0]
	if s.Name != "App" || s.RunConfiguration != "Debug" || s.Run == nil || s.Run.TargetID != "APP0" ||
		s.Run.Project != filepath.Join(dir, "App.xcodeproj") || len(s.Build) != 2 {
		t.Errorf("scheme: %+v", s)
	}
	if len(s.Tests) != 1 || s.Tests[0].TargetName != "AppTests" {
		t.Errorf("tests: %+v", s.Tests)
	}
	if !reflect.DeepEqual(s.TestPlans, []string{filepath.Join(dir, "App.xctestplan"), filepath.Join(dir, "Other.xctestplan")}) {
		t.Errorf("test plans: %v", s.TestPlans)
	}
}

func TestSynchronizedFolderBaseConfig(t *testing.T) {
	dir := t.TempDir()
	project(t, dir, app{anchored: true}, nil)
	p, err := LoadProject(filepath.Join(dir, "App.xcodeproj"))
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Configurations[0].BaseConfig; got != filepath.Join(dir, "Config", "Base.xcconfig") {
		t.Errorf("base config: %s", got)
	}
}

func TestLoadWorkspace(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "App.xcworkspace", "contents.xcworkspacedata"), `<?xml version="1.0" encoding="UTF-8"?>
<Workspace version="1.0">
  <FileRef location="group:App.xcodeproj"></FileRef>
  <Group location="group:Libraries" name="Libraries">
    <FileRef location="group:Lib/Lib.xcodeproj"></FileRef>
    <FileRef location="group:README.md"></FileRef>
  </Group>
  <FileRef location="container:Pods/Pods.xcodeproj"></FileRef>
  <FileRef location="absolute:/opt/Shared.xcodeproj"></FileRef>
</Workspace>`)
	w, err := LoadWorkspace(filepath.Join(dir, "App.xcworkspace"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(dir, "App.xcodeproj"), filepath.Join(dir, "Libraries", "Lib", "Lib.xcodeproj"),
		filepath.Join(dir, "Pods", "Pods.xcodeproj"), "/opt/Shared.xcodeproj"}
	if w.Name() != "App" || !reflect.DeepEqual(w.Projects, want) {
		t.Errorf("got %s %v", w.Name(), w.Projects)
	}
}

func TestPlanChoosesTheAppsScheme(t *testing.T) {
	for name, tc := range map[string]struct {
		schemes map[string]string
		want    string // the scheme, or what Unsure contains
	}{
		"named like the project": {map[string]string{
			"App": scheme("App.xcodeproj", &appRef, "Debug", appRef), "Other": scheme("App.xcodeproj", &appRef, "Debug", appRef),
		}, "App"},
		// An extension's scheme builds and runs its host app, but lists the
		// extension first.
		"not an extension's": {map[string]string{
			"Main": scheme("App.xcodeproj", &appRef, "Debug", appRef), "Ext": scheme("App.xcodeproj", &appRef, "Debug", extRef, appRef),
		}, "Main"},
		"not an embedded helper app's": {map[string]string{
			"Main": scheme("App.xcodeproj", &appRef, "Debug", appRef), "Helper": scheme("App.xcodeproj", &helperRef, "Debug", helperRef),
		}, "Main"},
		"the one that runs Debug": {map[string]string{
			"Dev": scheme("App.xcodeproj", &appRef, "Debug", appRef), "Prod": scheme("App.xcodeproj", &appRef, "Release", appRef),
		}, "Dev"},
		"named after its app": {map[string]string{
			"App-Dev": scheme("App.xcodeproj", &appRef, "Debug", appRef), "Tests": scheme("App.xcodeproj", &appRef, "Debug", appRef),
		}, "App-Dev"},
		"several": {map[string]string{
			"Brand A": scheme("App.xcodeproj", &appRef, "Debug", appRef), "Brand B": scheme("App.xcodeproj", &appRef, "Debug", appRef),
		}, "several schemes build an app (Brand A, Brand B)"},
		// Xcode makes a scheme per target where the project shares none.
		"none shared": {nil, "App"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "Repo")
			project(t, dir, app{sdk: "iphoneos"}, tc.schemes)
			p := plan(t, dir)
			if p.Kind != "-project" || p.Container != "App.xcodeproj" {
				t.Errorf("container: %s %s", p.Kind, p.Container)
			}
			if p.Scheme != tc.want && !strings.Contains(p.Unsure, tc.want) {
				t.Errorf("got scheme %q (%s), want %q", p.Scheme, p.Unsure, tc.want)
			}
		})
	}
}

func TestPlanDestination(t *testing.T) {
	for name, tc := range map[string]struct {
		app  app
		want string
	}{
		"a Mac app":       {app{sdk: "macosx"}, "generic/platform=macOS"},
		"an iOS app":      {app{sdk: "iphoneos"}, "generic/platform=iOS Simulator"},
		"Catalyst":        {app{sdk: "iphoneos", catalyst: true}, "generic/platform=iOS Simulator"},
		"visionOS":        {app{sdk: "xros"}, "generic/platform=visionOS Simulator"},
		"multiplatform":   {app{sdk: "auto", platforms: "iphoneos iphonesimulator macosx"}, "generic/platform=macOS"},
		"auto, no Mac":    {app{sdk: "auto", platforms: "iphoneos iphonesimulator xros xrsimulator"}, "generic/platform=iOS Simulator"},
		"from .xcconfig":  {app{}, "generic/platform=macOS"}, // Config/Base.xcconfig sets SDKROOT
		"an unknown SDKs": {app{sdk: "driverkit"}, "SDKROOT is driverkit"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			project(t, dir, tc.app, map[string]string{"App": scheme("App.xcodeproj", &appRef, "Debug", appRef)})
			write(t, filepath.Join(dir, "Config", "Base.xcconfig"), "SDKROOT = macosx\n")
			p := plan(t, dir)
			if p.Destination != tc.want && !strings.Contains(p.Unsure, tc.want) {
				t.Errorf("got %q (%s), want %q", p.Destination, p.Unsure, tc.want)
			}
		})
	}
}

// CocoaPods' workspace names Pods/Pods.xcodeproj and the app's
// configurations are based on the Pods' .xcconfig files: without
// `pod install`, they're missing.
func TestPlanSaysWhatsMissing(t *testing.T) {
	dir := t.TempDir()
	project(t, dir, app{sdk: "iphoneos"}, map[string]string{"App": scheme("App.xcodeproj", &appRef, "Debug", appRef)})
	write(t, filepath.Join(dir, "App.xcworkspace", "contents.xcworkspacedata"),
		`<Workspace version="1.0"><FileRef location="group:App.xcodeproj"/><FileRef location="group:Pods/Pods.xcodeproj"/></Workspace>`)
	p := plan(t, dir)
	want := &Plan{Kind: "-workspace", Container: "App.xcworkspace", Scheme: "App", Destination: "generic/platform=iOS Simulator",
		Missing: []string{filepath.Join("Config", "Base.xcconfig"), filepath.Join("Pods", "Pods.xcodeproj")}, Setup: "pod install"}
	if !reflect.DeepEqual(p, want) {
		t.Errorf("got %+v\nwant %+v", p, want)
	}
	write(t, filepath.Join(dir, "Pods", "Pods.xcodeproj", "project.pbxproj"),
		"{ objects = { P = {isa = PBXProject; mainGroup = G; targets = (); }; G = {isa = PBXGroup; children = (); }; }; rootObject = P; }")
	if p := plan(t, dir); !reflect.DeepEqual(p.Missing, []string{filepath.Join("Config", "Base.xcconfig")}) ||
		p.Setup != "the project's own setup (see its README)" {
		t.Errorf("got %v %q", p.Missing, p.Setup)
	}
}

func TestPlanPicksAmongSeveralProjects(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Repo")
	project(t, dir, app{sdk: "iphoneos"}, nil)
	write(t, filepath.Join(dir, "Repo.xcodeproj", "project.pbxproj"), appProject(app{sdk: "macosx"}))
	if p := plan(t, dir, "Repo"); p.Container != "Repo.xcodeproj" || p.Destination != "generic/platform=macOS" {
		t.Errorf("got %+v", p)
	}
	if p := plan(t, dir, "Other"); p.Container != "" || !strings.Contains(p.Unsure, "several Xcode projects here (App.xcodeproj, Repo.xcodeproj)") {
		t.Errorf("got %+v", p)
	}
}

func TestPlanFindsProjectsBelow(t *testing.T) {
	dir := t.TempDir()
	for _, path := range []string{"src/GoMap.xcodeproj", "apps/a/A.xcodeproj", "apps/a/A.xcworkspace", "apps/a/nested/N.xcodeproj",
		"Pods/Pods.xcodeproj", ".build/x/X.xcodeproj", "src/GoMap.xcodeproj/project.xcworkspace"} {
		if err := os.MkdirAll(filepath.Join(dir, path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	p := plan(t, dir)
	want := []string{filepath.Join("apps", "a", "A.xcodeproj"), filepath.Join("apps", "a", "A.xcworkspace"), filepath.Join("src", "GoMap.xcodeproj")}
	if p.Container != "" || !reflect.DeepEqual(p.Nested, want) || p.Unsure != "no Xcode project or workspace here" {
		t.Errorf("got %+v", p)
	}
}
