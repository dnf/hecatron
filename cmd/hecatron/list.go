package main

import (
	"fmt"
	"strings"
)

// list is `hecatron list [languages] [--all]`, as manage.py prints it. Any
// other argument (an abbreviated --al, -h, a typo) is left to manage.py's
// argparse, whose messages and help depend on the Python version.
func (c *cli) list() (int, bool) {
	all, what := false, 0
	for _, arg := range c.args {
		switch arg {
		case "--all":
			all = true
		case "languages":
			what++
		default:
			return 0, false
		}
	}
	if what > 1 {
		return 0, false
	}
	var out strings.Builder
	p := c.project
	if all {
		defs, err := p.Definitions()
		if err != nil {
			return 0, false
		}
		added := map[string]bool{}
		for _, lang := range p.Languages {
			added[lang] = true
		}
		addedCount := len(added)
		for _, lang := range defs.Names {
			origin := "built-in"
			if p.HasProjectDefinition(lang) {
				origin = "project"
			}
			status := ""
			if !added[lang] {
				status = "  (not added)"
			}
			fmt.Fprintf(&out, "  %-12s %s%s\n", lang, origin, status)
		}
		fmt.Fprintf(&out, "\n%d languages known, %d added\n", len(defs.Names), addedCount)
	} else {
		disabled := map[string]bool{}
		for _, lang := range p.Disabled {
			disabled[lang] = true
		}
		for _, lang := range p.Languages {
			status := ""
			if disabled[lang] {
				status = "  (disabled)"
			}
			fmt.Fprintf(&out, "  %s%s\n", lang, status)
		}
		fmt.Fprintf(&out, "\n%d languages\n", len(p.Languages))
	}
	fmt.Print(out.String())
	return 0, true
}
