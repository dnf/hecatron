package install

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tarGz is a release archive: tool-1.0/bin/tool, a link to it, and a
// README, under one top directory as releases have them.
func tarGz(t *testing.T) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, h := range []*tar.Header{
		{Name: "tool-1.0/", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "tool-1.0/bin/tool", Typeflag: tar.TypeReg, Mode: 0o755, Size: 18},
		{Name: "tool-1.0/bin/tl", Typeflag: tar.TypeSymlink, Linkname: "tool"},
		{Name: "tool-1.0/README", Typeflag: tar.TypeReg, Mode: 0o644, Size: 2},
	} {
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		switch h.Name {
		case "tool-1.0/bin/tool":
			_, _ = tw.Write([]byte("#!/bin/sh\necho ok\n"))
		case "tool-1.0/README":
			_, _ = tw.Write([]byte("hi"))
		}
	}
	_ = tw.Close()
	_ = gz.Close()
	return buf.Bytes()
}

func sum(data []byte) string {
	s := sha256.Sum256(data)
	return hex.EncodeToString(s[:])
}

func toolJSON(checksum string) string {
	return `{"schema": 1, "toolchain": {"versions": "single", "archive": {
		"url": "https://example.com/tool-{version}-{os}-{arch}.tar.gz", "os": {"linux": "linux"}, "arch": {"x86_64": "amd64"},
		` + checksum + `, "strip": 1, "bin": {"default": "bin"}, "check": {"path": "{bin}/tool"}}},
		"links": ["tool"]}`
}

const toolURL = "https://example.com/tool-1.0-linux-amd64.tar.gz"

func (f *fakeSystem) archiveLanguage(def string) *Language {
	language := f.language("tool", def)
	language.Version = "1.0"
	return language
}

func TestReleaseDownloadInstallsChecksAndRepairs(t *testing.T) {
	f := newFake(t)
	archive := tarGz(t)
	f.served[toolURL] = archive
	language := f.archiveLanguage(toolJSON(`"sha256": {"linux-x86_64": "` + sum(archive) + `"}`))
	env := language.Ctx.Env
	receipts := ReceiptsDir(env)
	var out bytes.Buffer
	if errs := NewPlan(f.manager(), []*Language{language}).Apply(&out, receipts); len(errs) > 0 {
		t.Fatal(errs, out.String())
	}
	home := filepath.Join(ToolchainsDir(env), "tool", "1.0")
	link := filepath.Join(LinksDir(env), "tool")
	if target, err := os.Readlink(link); err != nil || target != filepath.Join(home, "bin", "tool") {
		t.Errorf("link: %q %v", target, err)
	}
	if target, _ := os.Readlink(filepath.Join(home, "bin", "tl")); target != "tool" {
		t.Errorf("the archive's own link: %q", target)
	}
	if data, _ := os.ReadFile(filepath.Join(home, "README")); string(data) != "hi" {
		t.Errorf("README: %q", data)
	}
	receipt, _ := ReadReceipt(receipts, "tool")
	if receipt == nil || len(receipt.Homes) != 1 || receipt.Homes[0] != home || len(receipt.Links) != 1 {
		t.Errorf("receipt: %+v", receipt)
	}
	if len(f.fetched) != 1 {
		t.Errorf("fetched %q", f.fetched)
	}

	// A rerun, as after restoring a CI cache: nothing to fetch or unpack.
	out.Reset()
	if errs := NewPlan(f.manager(), []*Language{language}).Apply(&out, receipts); len(errs) > 0 || out.Len() > 0 || len(f.fetched) != 1 {
		t.Errorf("a rerun: %v %q %q", errs, out.String(), f.fetched)
	}

	// A partial restore: the doctor says so, and an install repairs it from
	// the downloads cache.
	if err := os.Remove(filepath.Join(home, "bin", "tool")); err != nil {
		t.Fatal(err)
	}
	if problems := Problems(language, f.manager()); len(problems) != 1 || !strings.Contains(problems[0], "doesn't work") {
		t.Errorf("problems: %q", problems)
	}
	if errs := NewPlan(f.manager(), []*Language{language}).Apply(&out, receipts); len(errs) > 0 || len(f.fetched) != 1 {
		t.Errorf("the repair: %v %q", errs, f.fetched)
	}
	if _, err := os.Stat(filepath.Join(home, "bin", "tool")); err != nil {
		t.Error(err)
	}

	// Uninstall removes the home and the link it made.
	removal, err := PlanUninstall("tool", language.Def, f.manager(), receipts, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := removal.Apply(&out, f.manager(), receipts); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{home, link} {
		if _, err := os.Lstat(path); err == nil {
			t.Errorf("%s is still there", path)
		}
	}
}

func TestADownloadThatDoesNotMatchIsNeverUnpacked(t *testing.T) {
	f := newFake(t)
	f.served[toolURL] = tarGz(t)
	language := f.archiveLanguage(toolJSON(`"sha256": {"linux-x86_64": "` + strings.Repeat("0", 64) + `"}`))
	var out bytes.Buffer
	errs := NewPlan(f.manager(), []*Language{language}).Apply(&out, ReceiptsDir(language.Ctx.Env))
	if err := errs["tool"]; err == nil || !strings.Contains(err.Error(), "doesn't match its checksum") {
		t.Errorf("errors %v", errs)
	}
	if _, err := os.Stat(filepath.Join(ToolchainsDir(language.Ctx.Env), "tool")); err == nil {
		t.Error("unpacked anyway")
	}
}

func TestChecksumsFromASidecarOrAnIndex(t *testing.T) {
	archive := tarGz(t)
	for name, form := range map[string]string{
		"sidecar": `"checksum": "{url}.sha256"`,
		"index":   `"checksumIndex": {"url": "https://example.com/index.json", "path": ["{version}", "{arch}-{os}", "shasum"]}`,
	} {
		f := newFake(t)
		f.served[toolURL] = archive
		f.served[toolURL+".sha256"] = []byte(sum(archive) + " *tool-1.0-linux-amd64.tar.gz\n")
		f.served["https://example.com/index.json"] = []byte(`{"1.0": {"amd64-linux": {"shasum": "` + sum(archive) + `"}}}`)
		language := f.archiveLanguage(toolJSON(form))
		var out bytes.Buffer
		if errs := NewPlan(f.manager(), []*Language{language}).Apply(&out, ReceiptsDir(language.Ctx.Env)); len(errs) > 0 {
			t.Errorf("%s: %v", name, errs)
		}
	}
}

func TestPlanShowsTheDownload(t *testing.T) {
	f := newFake(t)
	language := f.archiveLanguage(toolJSON(`"checksum": "{url}.sha256"`))
	plan := NewPlan(f.manager(), []*Language{language})
	var out bytes.Buffer
	plan.Print(&out)
	if !strings.Contains(out.String(), "install  toolchain tool 1.0 ("+toolURL+" (sha256 from "+toolURL+".sha256) into ") ||
		!strings.Contains(out.String(), "install  link ") {
		t.Errorf("plan:\n%s", out.String())
	}
	if commands := plan.Commands(); len(commands) != 2 || !strings.HasPrefix(commands[0], "download "+toolURL) ||
		!strings.HasPrefix(commands[1], "ln -s ") {
		t.Errorf("commands %q", commands)
	}
	if len(f.fetched) > 0 {
		t.Errorf("planning fetched %q", f.fetched)
	}
}

func TestUnzipStripsAndStaysInside(t *testing.T) {
	dir := t.TempDir()
	for name, entries := range map[string][]string{"good.zip": {"top/a/b.txt"}, "bad.zip": {"top/../../evil"}} {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		for _, entry := range entries {
			w, _ := zw.Create(entry)
			_, _ = w.Write([]byte("x"))
		}
		_ = zw.Close()
		if err := os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	home := filepath.Join(dir, "home")
	if err := unpackInto(filepath.Join(dir, "good.zip"), "zip", 1, home); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(home, "a", "b.txt")); string(data) != "x" {
		t.Errorf("unzipped %q", data)
	}
	if err := unpackInto(filepath.Join(dir, "bad.zip"), "zip", 1, filepath.Join(dir, "bad")); err == nil {
		t.Error("an entry leaving the directory was unpacked")
	}
}

func TestManagedInstallsEachVersionSideBySide(t *testing.T) {
	f := newFake(t)
	archive := tarGz(t)
	older := strings.ReplaceAll(toolURL, "1.0", "0.9")
	f.served[toolURL], f.served[older] = archive, archive
	def := strings.Replace(toolJSON(`"sha256": {"linux-x86_64": "`+sum(archive)+`"}`), `"single"`, `"managed"`, 1)
	language := f.archiveLanguage(def)
	language.Versions = []string{"0.9", "1.0"}
	env := language.Ctx.Env
	plan := NewPlan(f.manager(), []*Language{language})
	if actions := plan.Languages[0].Actions; len(actions) != 3 || actions[0].Label != "tool 1.0" || actions[1].Label != "tool 0.9" {
		t.Fatalf("actions: %+v", actions)
	}
	var out bytes.Buffer
	if errs := plan.Apply(&out, ReceiptsDir(env)); len(errs) > 0 {
		t.Fatal(errs, out.String())
	}
	for _, version := range []string{"1.0", "0.9"} {
		if _, err := os.Stat(filepath.Join(ToolchainsDir(env), "tool", version, "bin", "tool")); err != nil {
			t.Error(err)
		}
	}
	link := filepath.Join(LinksDir(env), "tool")
	if target, _ := os.Readlink(link); target != filepath.Join(ToolchainsDir(env), "tool", "1.0", "bin", "tool") {
		t.Errorf("the link points at %q, not the default's", target)
	}
	if receipt, _ := ReadReceipt(ReceiptsDir(env), "tool"); receipt == nil || len(receipt.Homes) != 2 {
		t.Errorf("receipt: %+v", receipt)
	}

	// Each version is checked in its own home: one broken is reported and
	// repaired alone.
	if err := os.Remove(filepath.Join(ToolchainsDir(env), "tool", "0.9", "bin", "tool")); err != nil {
		t.Fatal(err)
	}
	if problems := Problems(language, f.manager()); len(problems) != 1 || !strings.Contains(problems[0], "tool 0.9 in ") {
		t.Errorf("problems: %q", problems)
	}
	if errs := NewPlan(f.manager(), []*Language{language}).Apply(&out, ReceiptsDir(env)); len(errs) > 0 {
		t.Errorf("the repair: %v", errs)
	}

	// "single" installs the default alone, whatever the components want.
	single := f.archiveLanguage(toolJSON(`"sha256": {"linux-x86_64": "` + sum(archive) + `"}`))
	single.Versions = []string{"0.9"}
	if actions := single.Actions(f.manager().Name); len(actions) != 2 || actions[0].Label != "tool 1.0" {
		t.Errorf("single: %+v", actions)
	}
}

func TestADownloadsURLFromItsIndex(t *testing.T) {
	f := newFake(t)
	archive := tarGz(t)
	renamed := "https://example.com/old-name-1.0.tar.gz"
	f.served[renamed] = archive
	f.served["https://example.com/index.json"] = []byte(`{"1.0": {"amd64-linux": {"shasum": "` + sum(archive) +
		`", "tarball": "` + renamed + `"}}}`)
	language := f.archiveLanguage(toolJSON(`"checksumIndex": {"url": "https://example.com/index.json",
		"path": ["{version}", "{arch}-{os}", "shasum"], "urlPath": ["{version}", "{arch}-{os}", "tarball"]}`))
	var out bytes.Buffer
	if errs := NewPlan(f.manager(), []*Language{language}).Apply(&out, ReceiptsDir(language.Ctx.Env)); len(errs) > 0 {
		t.Fatal(errs, out.String())
	}
	if !contains(f.fetched, renamed) || contains(f.fetched, toolURL) {
		t.Errorf("fetched %q", f.fetched)
	}
	if !strings.Contains(out.String(), "downloading the URL at 1.0 → amd64-linux → tarball in https://example.com/index.json (sha256 from it too)") {
		t.Errorf("said %q", out.String())
	}
}
