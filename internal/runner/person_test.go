package runner

import (
	"strings"
	"testing"
)

func TestForAPerson(t *testing.T) {
	env := func(vars ...string) func(string) string {
		return func(name string) string {
			for _, v := range vars {
				if k, value, _ := strings.Cut(v, "="); k == name {
					return value
				}
			}
			return ""
		}
	}
	for _, c := range []struct {
		name   string
		getenv func(string) string
		args   []string
		stdout bool
		stderr bool
		want   bool
	}{
		{"human on a terminal", env(), nil, true, true, true},
		{"stderr not a terminal", env(), nil, true, false, false},
		{"stdout piped, stderr a terminal", env(), nil, false, true, true},
		{"--output agent", env(), []string{"--output", "agent"}, true, true, false},
		{"--output=json", env(), []string{"--output=json"}, true, true, false},
		{"--json", env(), []string{"ts", "--json"}, true, true, false},
		{"--output human", env("HECATRON_OUTPUT=agent"), []string{"--output", "human"}, true, true, true},
		{"HECATRON_OUTPUT=json", env("HECATRON_OUTPUT=json"), nil, true, true, false},
		{"an agent", env("CLAUDECODE=1"), nil, false, true, false},
		{"GitHub Actions", env("GITHUB_ACTIONS=true"), nil, true, true, false},
		{"after --", env(), []string{"x", "--", "--output", "json"}, true, true, true},
	} {
		if got := ForAPerson(c.getenv, c.args, c.stdout, c.stderr); got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}
}
