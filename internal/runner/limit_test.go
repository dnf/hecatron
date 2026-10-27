package runner

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestHistoryLimit(t *testing.T) {
	h := History{lines: []string{
		"go=99999:0", // older than the last ten passing runs
		"go=2000:0 rust=50:0",
		"go=2400:0", "go=2200:0", "go=1800:0", "go=2000:0", "go=2600:0", "go=2000:0", "go=2200:0",
		"go=2000:0 rust=40:0",
		"go=10:1 rust=6000:124", // a failure stopped early; a timeout ran to its limit
		"go=2400:0 slow=90000:0",
	}}
	cases := []struct {
		label, describe string
		want            time.Duration
	}{
		// go's last ten passing runs: median 220s, times 5
		{"go", "5× its median 3m40s over 10 passing runs", 1100 * time.Second},
		{"rust", "the 10m minimum (5× its median 5s over 2 passing runs is less)", LimitFloor},
		{"slow", "5× its median 2h30m over 1 passing run", 750 * time.Minute},
		{"new", "none", 0},
	}
	for _, c := range cases {
		got := h.Limit(c.label, Override{})
		if got.Duration != c.want || got.Describe() != c.describe {
			t.Errorf("%s: %v, %q; want %v, %q", c.label, got.Duration, got.Describe(), c.want, c.describe)
		}
	}
	if l := (History{lines: []string{"x=5:124", "x=7:1"}}).Limit("x", Override{}); l.Duration != 0 {
		t.Errorf("no passing run: %+v, want none", l)
	}
	if l := h.Limit("go", Override{Set: true, Duration: 90 * time.Second, Source: LimitFlag}); l.Duration != 90*time.Second || l.Describe() != "--timeout" {
		t.Errorf("--timeout 90s: %+v", l)
	}
	if l := h.Limit("go", Override{Set: true, Source: LimitEnv}); l.Duration != 0 {
		t.Errorf("HECATRON_TIMEOUT=0: %+v, want none", l)
	}
}

func TestSelectTimeout(t *testing.T) {
	env := func(value string) func(string) string {
		return func(name string) string {
			if name == "HECATRON_TIMEOUT" {
				return value
			}
			return ""
		}
	}
	cases := []struct {
		flag  string
		given bool
		env   string
		want  Override
		err   bool
	}{
		{"", false, "", Override{}, false},
		{"", false, "90s", Override{true, 90 * time.Second, LimitEnv}, false},
		{"30m", true, "90s", Override{true, 30 * time.Minute, LimitFlag}, false},
		{"0", true, "90s", Override{true, 0, LimitFlag}, false},
		{"45", true, "", Override{true, 45 * time.Second, LimitFlag}, false},
		{"1h30m", true, "", Override{Set: true, Duration: 90 * time.Minute, Source: LimitFlag}, false},
		{"soon", true, "", Override{}, true},
		{"-5m", true, "", Override{}, true},
		{"", false, "x", Override{}, true},
	}
	for _, c := range cases {
		got, err := SelectTimeout(c.flag, c.given, env(c.env))
		if got != c.want || (err != nil) != c.err {
			t.Errorf("flag %q env %q: %+v, %v; want %+v", c.flag, c.env, got, err, c.want)
		}
	}
	if _, err := SelectTimeout("", false, env("x")); err == nil || !strings.HasPrefix(err.Error(), "HECATRON_TIMEOUT must be a duration") {
		t.Errorf("error: %v", err)
	}
}

func TestFormatLimit(t *testing.T) {
	cases := map[time.Duration]string{10 * time.Minute: "10m", 90 * time.Second: "1m30s", 90 * time.Minute: "1h30m",
		2 * time.Hour: "2h", 2500 * time.Millisecond: "2.5s", 300 * time.Millisecond: "300ms"}
	for d, want := range cases {
		if got := FormatLimit(d); got != want {
			t.Errorf("%v: %q, want %q", d, got, want)
		}
	}
}

// A job within its limit, or with none, runs as before.
func TestWithinTheLimit(t *testing.T) {
	quick := func(code int) Job {
		return Job{Label: "q", Limit: Limit{Duration: time.Minute, Source: LimitFlag},
			Exec: func(ctx context.Context, out io.Writer) Outcome { return Outcome{ExitCode: code} }}
	}
	if _, status := Execute(context.Background(), quick(0), io.Discard); status != Passed {
		t.Errorf("passing: %s", status)
	}
	if _, status := Execute(context.Background(), quick(2), io.Discard); status != Failed {
		t.Errorf("failing: %s", status)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	interrupted := Job{Label: "i", Limit: Limit{Duration: time.Minute}, Exec: func(ctx context.Context, out io.Writer) Outcome {
		return Outcome{ExitCode: 1}
	}}
	if _, status := Execute(ctx, interrupted, io.Discard); status != Failed {
		t.Errorf("interrupted, not timed out: %s", status)
	}
}
