package install

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const swiPPA = "E8B739E3753FF4A12360BA6A4AB3A5F60EA9AEB3"

// The PPA's key as keyserver.ubuntu.com serves it, whose fingerprint gpg
// prints as swiPPA.
func TestKeyFingerprint(t *testing.T) {
	key, err := os.ReadFile(filepath.Join("testdata", "swi-prolog-ppa.asc"))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := keyFingerprint(key); err != nil || got != swiPPA {
		t.Errorf("fingerprint %q (%v), want %s", got, err, swiPPA)
	}
	binary, _ := dearmor(key)
	if got, err := keyFingerprint(append(append([]byte(nil), binary...), binary...)); err == nil {
		t.Errorf("two keys: %q, want an error", got)
	}
	for _, bad := range []string{"", "hello", "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\n!!!\n-----END PGP PUBLIC KEY BLOCK-----\n"} {
		if got, err := keyFingerprint([]byte(bad)); err == nil {
			t.Errorf("%q: fingerprint %q, want an error", bad, got)
		}
	}
}

const prologJSON = `{"schema": 1, "toolchain": {"versions": "system", "packages": {"apt": ["swi-prolog"]}},
	"aptSources": [{"name": "swi-prolog", "distro": "ubuntu", "uri": "https://ppa.example/swi/ubuntu",
	  "components": ["main"], "key": "https://keys.example/swi", "fingerprint": "` + swiPPA + `",
	  "packages": ["swi-prolog"], "replaces": ["swi-prolog-core", "swi-prolog-gone"]}]}`

// fakeDistro points the apt source paths at a temporary /etc.
func fakeDistro(t *testing.T, osRelease string) string {
	etc := t.TempDir()
	oldApt, oldRelease := aptDir, osReleaseFile
	aptDir, osReleaseFile = filepath.Join(etc, "apt"), filepath.Join(etc, "os-release")
	t.Cleanup(func() { aptDir, osReleaseFile = oldApt, oldRelease })
	if err := os.WriteFile(osReleaseFile, []byte(osRelease), 0o644); err != nil {
		t.Fatal(err)
	}
	return etc
}

func TestAptSourceIsAddedBeforeItsPackagesAndUpgradesThem(t *testing.T) {
	fakeDistro(t, "ID=ubuntu\nVERSION_CODENAME=noble\n")
	f := newFake(t)
	key, _ := os.ReadFile(filepath.Join("testdata", "swi-prolog-ppa.asc"))
	f.served["https://keys.example/swi"] = key
	f.served["https://ppa.example/swi/ubuntu/dists/noble/InRelease"] = []byte("signed")
	f.install("swi-prolog") // the distribution's, too old
	f.install("swi-prolog-core")
	receipts := t.TempDir()
	language := f.language("prolog", prologJSON)
	if problems := Problems(language, f.manager()); len(problems) != 1 || !strings.Contains(problems[0], "apt source swi-prolog not added") {
		t.Errorf("problems %q", problems)
	}
	plan := NewPlan(f.manager(), []*Language{language})
	commands := plan.Commands()
	if len(commands) != 5 || !strings.HasPrefix(commands[0], "sudo install -D -m 644 /dev/stdin "+aptDir+"/keyrings/hecatron-swi-prolog.asc") ||
		commands[2] != "sudo apt-get remove -y swi-prolog-core" ||
		commands[3] != "sudo apt-get -o Acquire::Retries=3 update -qq" ||
		commands[4] != "sudo apt-get -o Acquire::Retries=3 install -y swi-prolog" {
		t.Errorf("commands %q", commands)
	}
	var out bytes.Buffer
	if errs := plan.Apply(&out, receipts); len(errs) > 0 {
		t.Fatal(errs, out.String())
	}
	sources, _ := os.ReadFile(filepath.Join(aptDir, "sources.list.d", "hecatron-swi-prolog.sources"))
	want := "Types: deb\nURIs: https://ppa.example/swi/ubuntu\nSuites: noble\nComponents: main\nSigned-By: " +
		filepath.Join(aptDir, "keyrings", "hecatron-swi-prolog.asc") + "\n"
	if string(sources) != want {
		t.Errorf("sources %q, want %q", sources, want)
	}
	if ran := f.ran[len(f.ran)-3:]; ran[0] != "sudo apt-get remove -y swi-prolog-core" ||
		ran[1] != "sudo apt-get -o Acquire::Retries=3 update -qq" ||
		ran[2] != "sudo apt-get -o Acquire::Retries=3 install -y swi-prolog" {
		t.Errorf("ran %q", f.ran)
	}
	receipt, _ := ReadReceipt(receipts, "prolog")
	if len(receipt.Packages["apt"]) != 0 || len(receipt.Files) != 2 {
		t.Errorf("receipt %+v: want the source's files, not the upgraded swi-prolog", receipt)
	}
	f.ran = nil
	if errs := NewPlan(f.manager(), []*Language{language}).Apply(&out, receipts); len(errs) > 0 || f.ran != nil {
		t.Errorf("a rerun ran %q (%v)", f.ran, errs)
	}
	if problems := Problems(language, f.manager()); len(problems) > 0 {
		t.Errorf("problems after the install: %q", problems)
	}
	removal, err := PlanUninstall("prolog", language.Def, f.manager(), receipts, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := removal.Apply(&out, f.manager(), receipts); err != nil {
		t.Fatal(err)
	}
	for _, file := range receipt.Files {
		if _, err := os.Stat(file); err == nil {
			t.Errorf("uninstall left %s", file)
		}
	}
	if !f.installed["swi-prolog"] {
		t.Errorf("uninstall removed the swi-prolog that was there before")
	}
}

func TestAptSourceElsewhere(t *testing.T) {
	for name, release := range map[string]string{"debian": "ID=debian\nVERSION_CODENAME=trixie\n", "none": ""} {
		fakeDistro(t, release)
		f := newFake(t)
		plan := NewPlan(f.manager(), []*Language{f.language("prolog", prologJSON)})
		var out bytes.Buffer
		plan.Print(&out)
		if !strings.Contains(out.String(), "skip     source swi-prolog") {
			t.Errorf("%s: plan\n%s", name, out.String())
		}
		if errs := plan.Apply(&out, t.TempDir()); len(errs) > 0 || len(f.fetched) > 0 {
			t.Errorf("%s: fetched %q (%v)", name, f.fetched, errs)
		}
	}
}

func TestAptSourceWithoutThisReleaseUsesTheDistributions(t *testing.T) {
	fakeDistro(t, "ID=ubuntu\nVERSION_CODENAME=future\n")
	f := newFake(t)
	var out bytes.Buffer
	if errs := NewPlan(f.manager(), []*Language{f.language("prolog", prologJSON)}).Apply(&out, t.TempDir()); len(errs) > 0 {
		t.Fatal(errs)
	}
	if !strings.Contains(out.String(), "has no future") || !f.installed["swi-prolog"] {
		t.Errorf("said %q, ran %q", out.String(), f.ran)
	}
}

func TestAptSourceWithAnotherKeyFails(t *testing.T) {
	fakeDistro(t, "ID=ubuntu\nVERSION_CODENAME=noble\n")
	f := newFake(t)
	key, _ := os.ReadFile(filepath.Join("testdata", "swi-prolog-ppa.asc"))
	f.served["https://keys.example/swi"] = key
	f.served["https://ppa.example/swi/ubuntu/dists/noble/InRelease"] = nil
	def := strings.Replace(prologJSON, swiPPA, strings.Repeat("A", 40), 1)
	var out bytes.Buffer
	errs := NewPlan(f.manager(), []*Language{f.language("prolog", def)}).Apply(&out, t.TempDir())
	if err := errs["prolog"]; err == nil || !strings.Contains(err.Error(), "want AAAA") || f.ran != nil {
		t.Errorf("errors %v, ran %q", errs, f.ran)
	}
}

func TestAStepsDownloadAndTheDirectoryItCreates(t *testing.T) {
	f := newFake(t)
	script := []byte("(install)")
	sum := sha256.Sum256(script)
	f.served["https://example/ql.lisp"] = script
	def := `{"schema": 1, "toolchain": {"versions": "system", "packages": {"apt": ["sbcl"]}},
		"steps": [{"name": "Quicklisp", "download": {"url": "https://example/ql.lisp", "sha256": "` + hex.EncodeToString(sum[:]) + `"},
		  "run": ["sbcl", "--load", "{file}", "--eval", "(install :path \"{data}/quicklisp/\")"],
		  "check": {"path": "{data}/quicklisp/setup.lisp"}, "problem": "Quicklisp not installed",
		  "creates": "{data}/quicklisp"}]}`
	language := f.language("lisp", def)
	data := dataDir(language.Ctx.Env)
	leftover := filepath.Join(data, "quicklisp", "half-done")
	if err := os.MkdirAll(leftover, 0o755); err != nil {
		t.Fatal(err)
	}
	f.onRun = func(argv []string) {
		if argv[0] == "sbcl" {
			if _, err := os.Stat(leftover); err == nil {
				t.Errorf("the step ran over an earlier run's leftovers")
			}
			_ = os.MkdirAll(filepath.Join(data, "quicklisp"), 0o755)
			_ = os.WriteFile(filepath.Join(data, "quicklisp", "setup.lisp"), nil, 0o644)
		}
	}
	receipts := filepath.Join(data, "receipts")
	var out bytes.Buffer
	if errs := NewPlan(f.manager(), []*Language{language}).Apply(&out, receipts); len(errs) > 0 {
		t.Fatal(errs)
	}
	cached := filepath.Join(DownloadsDir(language.Ctx.Env), hex.EncodeToString(sum[:]))
	want := "sbcl --load " + cached + " --eval (install :path \"" + data + "/quicklisp/\")"
	if f.ran[len(f.ran)-1] != want {
		t.Errorf("ran %q, want %q", f.ran, want)
	}
	receipt, _ := ReadReceipt(receipts, "lisp")
	if !reflect.DeepEqual(receipt.Paths, []string{filepath.Join(data, "quicklisp")}) {
		t.Errorf("receipt %+v", receipt)
	}
	removal, _ := PlanUninstall("lisp", language.Def, f.manager(), receipts, nil)
	if err := removal.Apply(&out, f.manager(), receipts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(data, "quicklisp")); err == nil {
		t.Errorf("uninstall left Quicklisp")
	}
	for _, bad := range []string{`"creates": "/opt/x"`, `"download": {"url": "u"}`} {
		text := `{"schema": 1, "steps": [{"run": ["x"], "check": {"path": "x"}, "problem": "p", ` + bad + `}]}`
		if _, err := Parse([]byte(text)); err == nil {
			t.Errorf("%s: no error", bad)
		}
	}
}
