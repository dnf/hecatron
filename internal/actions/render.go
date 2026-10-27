package actions

import (
	"path/filepath"
	"sort"
	"strings"
)

// Lines renders a plan as the shell hooks' logs show it, so the two can be
// compared while definitions are converted.
func Lines(def *Definition, ctx Context, ops []Op) []string {
	var lines []string
	for _, op := range ops {
		switch op.Kind {
		case Run:
			lines = append(lines, "+ "+strings.Join(append(envPrefix(op.Env), op.Argv...), " "))
		case Remove:
			line := "remove " + strings.Join(op.Paths, " ")
			if len(op.Keep) > 0 {
				line += " (keeping " + strings.Join(op.Keep, " ") + ")"
			}
			lines = append(lines, line)
		case Skip:
			lines = append(lines, def.Lang+": "+ctx.Action+": "+op.Message+" (skipping)")
		case Unknown:
			lines = append(lines, "error: "+def.Lang+": no "+ctx.Action+" command for "+filepath.Join(ctx.Dir, op.Dir)+": "+op.Message)
		case Note:
			lines = append(lines, op.Message)
		case Enter:
			lines = append(lines, "--- "+def.Lang+": "+ctx.Action+" in "+op.Dir)
		}
	}
	return lines
}

func envPrefix(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	prefix := []string{"env"}
	for _, key := range keys {
		prefix = append(prefix, key+"="+env[key])
	}
	return prefix
}
