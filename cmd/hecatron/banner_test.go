package main

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// banner prints what manage.py's cmd_banner prints, byte for byte, with the
// same status, for every argument form (those Go leaves to argparse
// included) and in locales where Python's stdout isn't UTF-8.
var bannerArgs = [][]string{
	{"--generator", "tools/gen.py"},
	{"--generator=tools/gen.py", "--source=api/schema.json", "--regenerate=just gen"},
	{"--generator", "gen", "--source", "s", "--regenerate", "r", "--prefix", "// ", "--suffix", ""},
	{"--generator", "gen", "--prefix", "<!-- ", "--suffix", " -->"},
	{"--generator", "gen", "--prefix=", "--suffix="},
	{"--generator", "gen", "--prefix", ""},
	{"--generator", "", "--source", "", "--regenerate", ""},
	{"--generator", "a", "--generator", "b"},
	{"--generator", "générateur/日本語.py", "--source", "ünïcödé"},
	{"--generator", strings.Repeat("x", 70), "--source", strings.Repeat("y", 61)},
	{"--generator", "gen", "--regenerate", "line\nbreak\ttab"},
	{"--generator", "gen", "--prefix", "-- "},
	{"--generator", "gen", "--prefix", "-x y"},
	{"--generator", "-1"},
	{"--gen", "gen"},
	{"--generator"},
	{"--source", "s"},
	{},
	{"--generator", "gen", "extra"},
	{"help"}, {"--help"}, {"-h"}, {"--generator", "gen", "-h"},
	{"--generator", "gen", "--", "x"},
	{"--generator", "bad\xffbyte"},
}

func TestBannerMatchesBinHecatron(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bin/hecatron needs bash and Python")
	}
	home := repoHome(t)
	dir := t.TempDir()
	envs := map[string][]string{
		"default":    {},
		"C":          {"LC_ALL=C"},
		"POSIX lang": {"LC_ALL=", "LC_CTYPE=", "LANG=POSIX"},
		"utf-8":      {"LC_ALL=C.UTF-8"},
		"latin-1":    {"LC_ALL=", "LC_CTYPE=en_US.ISO-8859-1"},
		"ascii io":   {"PYTHONIOENCODING=ascii"},
		"no utf8":    {"LC_ALL=C", "PYTHONUTF8=0"},
	}
	compared := 0
	for name, extra := range envs {
		t.Run(name, func(t *testing.T) {
			env := append([]string{"HECATRON_HOME=" + home, "HECATRON_NO_GO=1"}, extra...)
			for _, args := range bannerArgs {
				var results [2]result
				for i, entry := range [][]string{{"bash", filepath.Join(home, "bin", "hecatron")}, {binary}} {
					results[i] = runIn(t, dir, env, append(append(entry, "banner"), args...)...)
				}
				if results[0] != results[1] {
					t.Errorf("%q:\nbin/hecatron: %+v\nGo:           %+v", args, results[0], results[1])
				}
				compared++
			}
		})
	}
	t.Logf("%d banner invocations compared with bin/hecatron", compared)
}
