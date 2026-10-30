package xcode

import (
	"reflect"
	"strings"
	"testing"
)

func TestParsePlist(t *testing.T) {
	text := `// !$*UTF8*$!
{
	archiveVersion = 1;
	/* a comment */ objects = {
		ABC123 /* Foo.swift */ = {isa = PBXFileReference; path = "Foo Bar.swift"; sourceTree = "<group>"; };
	};
	list = (a, "b c", );
	escaped = "line\nnext \"quoted\" back\\slash \U00e9 \101";
	empty = ( );
	data = <0fbd7891>;
}`
	got, err := parsePlist(text)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"archiveVersion": "1",
		"objects": map[string]any{
			"ABC123": map[string]any{"isa": "PBXFileReference", "path": "Foo Bar.swift", "sourceTree": "<group>"},
		},
		"list":    []any{"a", "b c"},
		"escaped": "line\nnext \"quoted\" back\\slash é A",
		"empty":   []any{},
		"data":    "<0fbd7891>",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
}

func TestParsePlistErrors(t *testing.T) {
	for text, want := range map[string]string{
		`{ a = b }`:       "line 1: expected ; after the value of \"a\"",
		"{\n a = \"b;\n}": "line 2: unterminated string",
		`{ a = b; } x`:    "after the top-level value",
		`( a b )`:         "expected , or )",
		``:                "unexpected end of file",
	} {
		if _, err := parsePlist(text); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %v, want an error containing %q", text, err, want)
		}
	}
}
