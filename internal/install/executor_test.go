package install

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// fakeSystem is a machine with apt: packages are installed by the commands
// the executor runs, each package putting its commands in bin.
type fakeSystem struct {
	t         *testing.T
	bin       string
	installed map[string]bool
	provides  map[string][]string // package → commands
	broken    map[string]bool     // packages apt can't install
	passing   map[string]bool     // run checks that pass, by argv
	effects   map[string]string   // a step's argv → the run check it makes pass
	served    map[string][]byte   // URLs Fetch answers
	fetched   []string
	ran       []string
}

func (f *fakeSystem) Fetch(url string) (io.ReadCloser, error) {
	f.fetched = append(f.fetched, url)
	body, ok := f.served[url]
	if !ok {
		return nil, errors.New(url + ": 404 Not Found")
	}
	return io.NopCloser(bytes.NewReader(body)), nil
}

func newFake(t *testing.T) *fakeSystem {
	return &fakeSystem{t: t, bin: t.TempDir(), installed: map[string]bool{}, provides: map[string][]string{},
		broken: map[string]bool{}, passing: map[string]bool{}, effects: map[string]string{}, served: map[string][]byte{}}
}

func (f *fakeSystem) env() []string { return []string{"PATH=" + f.bin, "HOME=" + f.t.TempDir()} }

func (f *fakeSystem) install(name string) {
	f.installed[name] = true
	for _, command := range f.provides[name] {
		if err := os.WriteFile(filepath.Join(f.bin, command), []byte("#!/bin/sh\n"), 0o755); err != nil {
			f.t.Fatal(err)
		}
	}
}

func (f *fakeSystem) Succeeds(argv []string, _ string, _ []string) bool {
	return f.passing[strings.Join(argv, " ")]
}

func (f *fakeSystem) Run(argv []string, _ string, _ []string) error {
	line := strings.Join(argv, " ")
	f.ran = append(f.ran, line)
	if argv[0] == "sudo" {
		argv = argv[1:]
	}
	if len(argv) > 4 && argv[0] == "apt-get" && argv[3] == "install" {
		for _, name := range argv[5:] {
			if f.broken[name] {
				return errors.New("exit status 100")
			}
		}
		for _, name := range argv[5:] {
			f.install(name)
		}
	}
	if len(argv) > 3 && argv[0] == "apt-get" && argv[1] == "remove" {
		for _, name := range argv[3:] {
			delete(f.installed, name)
		}
	}
	if check, ok := f.effects[line]; ok {
		f.passing[check] = true
	}
	return nil
}

func (f *fakeSystem) Output(argv []string, _ []string) (string, error) {
	var out strings.Builder
	for _, name := range argv[3:] {
		if f.installed[name] {
			out.WriteString(name + "\tinstall ok installed\n")
		} else {
			out.WriteString(name + "\tdeinstall ok config-files\n")
		}
	}
	return out.String(), nil
}

func (f *fakeSystem) IsRoot() bool { return false }

func (f *fakeSystem) AptListsAge() (time.Duration, bool) { return 2 * time.Hour, true }

func (f *fakeSystem) manager() *Manager { return &Manager{"apt", f, f.env()} }

func mustParse(t *testing.T, text string) *Definition {
	t.Helper()
	def, err := Parse([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return def
}

const rakuJSON = `{"schema": 1, "toolchain": {"versions": "system", "packages": {"apt": ["rakudo"], "brew": ["rakudo"]}},
	"tools": [{"name": "zef", "when": {"glob": "META6.json", "because": "a component has a META6.json"},
	           "packages": {"apt": ["raku-zef"]}, "check": {"command": "zef"}}]}`

const dJSON = `{"schema": 1, "toolchain": {"versions": "system", "packages": {"apt": ["ldc", "dub"]}},
	"packages": [{"apt": ["build-essential"], "unless": {"command": "cc"}}],
	"uninstall": {"keep": ["build-essential"], "because": "shared"}}`

const racketJSON = `{"schema": 1, "toolchain": {"versions": "system", "packages": {"apt": ["racket"]}},
	"steps": [{"run": ["raco", "pkg", "install", "compiler-lib"], "check": {"run": ["raco", "test", "--help"]},
	           "problem": "raco test is missing (package compiler-lib)"}]}`

func (f *fakeSystem) language(name, def string, dirs ...string) *Language {
	return &Language{Name: name, Def: mustParse(f.t, def), Path: name + "/install.json",
		Ctx: &Context{Dirs: dirs, Root: f.t.TempDir(), Env: f.env(), Platform: Platform{"linux", "x86_64"}, System: f}}
}

func TestApplyBatchesPackagesAndRerunsAsNoOp(t *testing.T) {
	f := newFake(t)
	f.provides = map[string][]string{"raku-zef": {"zef"}, "build-essential": {"cc"}}
	component := t.TempDir()
	if err := os.WriteFile(filepath.Join(component, "META6.json"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	f.install("dub") // there before: never in a receipt
	receipts := t.TempDir()
	languages := []*Language{f.language("raku", rakuJSON, component), f.language("d", dJSON)}
	var out bytes.Buffer
	if errs := NewPlan(f.manager(), languages).Apply(&out, receipts); len(errs) > 0 {
		t.Fatal(errs)
	}
	want := []string{"sudo apt-get -o Acquire::Retries=3 update -qq",
		"sudo apt-get -o Acquire::Retries=3 install -y rakudo raku-zef ldc build-essential"}
	if !reflect.DeepEqual(f.ran, want) {
		t.Errorf("ran %q, want %q", f.ran, want)
	}
	for lang, packages := range map[string][]string{"raku": {"raku-zef", "rakudo"}, "d": {"build-essential", "ldc"}} {
		receipt, err := ReadReceipt(receipts, lang)
		if err != nil || receipt == nil || !reflect.DeepEqual(receipt.Packages["apt"], packages) {
			t.Errorf("%s's receipt: %+v %v, want %q", lang, receipt, err, packages)
		}
	}
	f.ran = nil
	if errs := NewPlan(f.manager(), languages).Apply(&out, receipts); len(errs) > 0 || f.ran != nil {
		t.Errorf("a rerun ran %q (%v)", f.ran, errs)
	}
}

func TestApplySkipsWhatDoesNotApply(t *testing.T) {
	f := newFake(t)
	f.provides = map[string][]string{"build-essential": {"cc"}}
	f.install("build-essential")
	plan := NewPlan(f.manager(), []*Language{f.language("raku", rakuJSON, t.TempDir()), f.language("d", dJSON)})
	var out bytes.Buffer
	plan.Print(&out)
	for _, line := range []string{"install  toolchain (apt: rakudo): missing rakudo",
		"skip     tool zef (apt: raku-zef): not needed: not (a component has a META6.json)",
		"ok       packages (apt: build-essential)"} {
		if !strings.Contains(out.String(), line) {
			t.Errorf("the plan lacks %q:\n%s", line, out.String())
		}
	}
	commands := plan.Commands()
	if len(commands) != 2 || !strings.HasSuffix(commands[1], "install -y rakudo ldc dub") {
		t.Errorf("commands %q", commands)
	}
}

func TestStepsRunOnlyWhenTheirCheckFails(t *testing.T) {
	f := newFake(t)
	f.effects["raco pkg install compiler-lib"] = "raco test --help"
	receipts := t.TempDir()
	language := f.language("racket", racketJSON)
	if problems := Problems(language, f.manager()); !reflect.DeepEqual(problems, []string{"raco test is missing (package compiler-lib)"}) {
		t.Errorf("problems %q", problems)
	}
	var out bytes.Buffer
	if errs := NewPlan(f.manager(), []*Language{language}).Apply(&out, receipts); len(errs) > 0 {
		t.Fatal(errs)
	}
	if f.ran[len(f.ran)-1] != "raco pkg install compiler-lib" {
		t.Errorf("ran %q", f.ran)
	}
	if problems := Problems(language, f.manager()); len(problems) > 0 {
		t.Errorf("problems after the install: %q", problems)
	}
	f.ran = nil
	NewPlan(f.manager(), []*Language{language}).Apply(&out, receipts)
	if f.ran != nil {
		t.Errorf("a rerun ran %q", f.ran)
	}
	receipt, _ := ReadReceipt(receipts, "racket")
	if !reflect.DeepEqual(receipt.Steps, []string{"raco pkg install compiler-lib"}) {
		t.Errorf("steps recorded: %q", receipt.Steps)
	}
}

func TestAStepWhoseCheckStillFailsFailsItsLanguage(t *testing.T) {
	f := newFake(t)
	var out bytes.Buffer
	errs := NewPlan(f.manager(), []*Language{f.language("racket", racketJSON)}).Apply(&out, t.TempDir())
	if err := errs["racket"]; err == nil || !strings.Contains(err.Error(), "raco test is missing") {
		t.Errorf("errors %v", errs)
	}
}

func TestABatchThatFailsIsRetriedPerLanguage(t *testing.T) {
	f := newFake(t)
	f.broken["ldc"] = true
	f.provides = map[string][]string{"build-essential": {"cc"}}
	var out bytes.Buffer
	errs := NewPlan(f.manager(), []*Language{f.language("raku", rakuJSON), f.language("d", dJSON)}).Apply(&out, t.TempDir())
	if len(errs) != 1 || errs["d"] == nil {
		t.Errorf("errors %v, want d's only", errs)
	}
	if !f.installed["rakudo"] {
		t.Errorf("raku wasn't installed alone: %q", f.ran)
	}
}

func TestUninstallRemovesOnlyWhatNoOneElseNeeds(t *testing.T) {
	f := newFake(t)
	receipts := t.TempDir()
	for _, name := range []string{"ldc", "dub", "build-essential", "shared-lib", "named-lib"} {
		f.install(name)
	}
	if err := record(receipts, "d", "apt", installed{packages: []string{"ldc", "dub", "build-essential", "shared-lib", "named-lib", "gone"}}); err != nil {
		t.Fatal(err)
	}
	if err := record(receipts, "other", "apt", installed{packages: []string{"shared-lib"}}); err != nil {
		t.Fatal(err)
	}
	others := map[string]*Definition{"named": mustParse(t, `{"schema": 1, "toolchain": {"versions": "system",
		"packages": {"apt": ["named-lib"]}}}`)}
	removal, err := PlanUninstall("d", mustParse(t, dJSON), f.manager(), receipts, others)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(removal.Remove)
	if !reflect.DeepEqual(removal.Remove, []string{"dub", "ldc"}) {
		t.Errorf("removes %q", removal.Remove)
	}
	wantKept := []string{"build-essential (shared)", "named-lib (also needed by named)", "shared-lib (also needed by other)"}
	sort.Strings(removal.Kept)
	if !reflect.DeepEqual(removal.Kept, wantKept) {
		t.Errorf("keeps %q, want %q", removal.Kept, wantKept)
	}
	var out bytes.Buffer
	if err := removal.Apply(&out, f.manager(), receipts); err != nil {
		t.Fatal(err)
	}
	if f.ran[0] != "sudo apt-get remove -y dub ldc" && f.ran[0] != "sudo apt-get remove -y ldc dub" {
		t.Errorf("ran %q", f.ran)
	}
	if receipt, _ := ReadReceipt(receipts, "d"); receipt != nil {
		t.Errorf("the receipt is still there")
	}
	removal, _ = PlanUninstall("d", mustParse(t, dJSON), f.manager(), receipts, nil)
	out.Reset()
	f.ran = nil
	if err := removal.Apply(&out, f.manager(), receipts); err != nil || f.ran != nil || !strings.Contains(out.String(), "no receipt") {
		t.Errorf("without a receipt: ran %q, said %q", f.ran, out.String())
	}
}

func TestUnsupported(t *testing.T) {
	cases := map[string]string{
		rakuJSON:                         "",
		`{"schema": 2, "toolchain": {}}`: "schema 2 is newer than this binary reads",
		`{"schema": 1, "installed": {}, "toolchain": {"versions": "system", "packages": {"apt": ["x"]}}}`:                                                                                                   `it uses "installed", which this binary can't install yet`,
		`{"schema": 1, "toolchain": {"versions": "system", "packages": {"apt": ["x"]}, "releases": {}}}`:                                                                                                    `it uses "toolchain.releases", which this binary can't install yet`,
		`{"schema": 1, "toolchain": {"versions": "system", "archive": {"url": "u"}}}`:                                                                                                                       `a release download has "versions": "single" or "managed"`,
		`{"schema": 1, "toolchain": {"versions": "single", "archive": {"url": "u.tar.gz", "os": {"linux": "l"}, "arch": {"x86_64": "x"}, "check": {"path": "{bin}/x"}}}}`:                                   "its download for linux-x86_64 has no checksum",
		`{"schema": 1, "toolchain": {"versions": "single", "archive": {"url": "u.tar.gz", "os": {"macos": "m"}, "arch": {"x86_64": "x"}, "sha256": {"macos-x86_64": "a"}, "check": {"path": "x"}}}}`:        "it has no download for linux-x86_64",
		`{"schema": 1, "toolchain": {"versions": "single", "archive": {"url": "u.7z", "os": {"linux": "l"}, "arch": {"x86_64": "x"}, "checksum": "{url}.sha256", "check": {"path": "x"}}}}`:                 `its download's format "" isn't one this binary unpacks`,
		`{"schema": 1, "toolchain": {"versions": "single", "archive": {"url": "u.zip", "os": {"linux": "l"}, "arch": {"x86_64": "x"}, "checksum": "{url}.sha256"}}}`:                                        `its download has no "check" saying the toolchain works`,
		`{"schema": 1, "toolchain": {"versions": "single", "archive": {"url": "u.zip", "os": {"linux": "l"}, "arch": {"x86_64": "x"}, "checksum": "{url}.sha256", "check": {"path": "x"}, "mirror": "m"}}}`: `it uses "toolchain.archive.mirror", which this binary can't install yet`,
		`{"schema": 1, "toolchain": {"versions": "system", "packages": {"brew": ["x"]}}}`:                                                                                                                   "it has no apt packages",
		`{"schema": 1, "toolchain": {"versions": "system", "packages": {"apt": ["x"]}}, "tools": [{"use": "osv-scanner"}]}`:                                                                                 "tool osv-scanner isn't system packages, which this binary can't install yet",
	}
	for text, want := range cases {
		if got := mustParse(t, text).Unsupported("apt", linux); got != want {
			t.Errorf("%s:\n  %q, want %q", text, got, want)
		}
	}
	for _, text := range []string{`{"toolchain": {}}`, `{"schema": 1, "steps": [{"run": ["x"]}]}`,
		`{"schema": 1, "tools": [{"name": "x"}]}`, `{"schema": 1, "packages": [{"apt": ["x"], "when": {}}]}`,
		`{"schema": 1, "packages": [{"aptitude": ["x"]}]}`} {
		if _, err := Parse([]byte(text)); err == nil {
			t.Errorf("%s: no error", text)
		}
	}
}

// The built-in definitions: the converted ones run on both package
// managers, the prototype's archives and installers are declined, and
// every install.json reads.
func TestBuiltInDefinitions(t *testing.T) {
	converted := []string{"bash", "cobol", "d", "erlang", "gleam", "lisp", "octave", "pascal", "prolog", "racket", "raku", "tcl", "zig"}
	paths, _ := filepath.Glob(filepath.Join("..", "..", "languages", "*", FileName))
	if len(paths) == 0 {
		t.Fatal("no install.json")
	}
	for _, path := range paths {
		lang := filepath.Base(filepath.Dir(path))
		def, err := LoadFile(path)
		if err != nil {
			t.Errorf("%s: %v", lang, err)
			continue
		}
		for _, manager := range []string{"apt", "brew"} {
			why := def.Unsupported(manager, Platform{map[string]string{"apt": "linux", "brew": "macos"}[manager], "x86_64"})
			if contains(converted, lang) != (why == "") {
				t.Errorf("%s on %s: %q", lang, manager, why)
			}
		}
	}
}

func TestConditions(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "test", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "test", "deep", "a.bats"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := &Context{Dirs: []string{t.TempDir(), dir}, Platform: Platform{"macos", "aarch64"}}
	cases := map[string]bool{
		`{"glob": "*.bats"}`: true, `{"glob": "*.sh"}`: false, `{"file": "test"}`: true,
		`{"missing": "test"}`: true, `{"platform": "macos"}`: true, `{"platform": "macos-x86_64"}`: false,
		`{"all": [{"glob": "*.bats"}, {"not": {"platform": "linux"}}]}`: true, `{"any": [{"file": "x"}, {"file": "y"}]}`: false,
	}
	for text, want := range cases {
		var condition Condition
		def := mustParse(t, `{"schema": 1, "packages": [{"apt": ["x"], "when": `+text+`}]}`)
		condition = *def.Packages[0].When
		if got := condition.Holds(ctx); got != want {
			t.Errorf("%s holds: %v, want %v", text, got, want)
		}
	}
}

var linux = Platform{"linux", "x86_64"}
