package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/dnf/hecatron/internal/xcode"
)

const xcodeUsage = `usage: hecatron _xcode plan [--json] [dir] [name...]
  How to build the Xcode project in dir (default: here), from its files: one
  "field<TAB>value" line each for kind, container, scheme, destination,
  setup and unsure, and a missing line per absent file the build needs and
  a nested line per project below; --json prints the plan as JSON. names are
  what a container may be named like when there are several.
`

// xcodeTool is `hecatron _xcode`, for the Swift and Objective-C hooks
// (xcode_hook in languages/_hooks.sh): the Xcode build plan internal/xcode
// reads from the project's files, which needs no Xcode.
func (c *cli) xcodeTool() int {
	args := c.args
	if len(args) == 0 || args[0] != "plan" {
		fmt.Fprint(os.Stderr, xcodeUsage)
		return 2
	}
	args = args[1:]
	asJSON := len(args) > 0 && args[0] == "--json"
	if asJSON {
		args = args[1:]
	}
	dir := "."
	if len(args) > 0 {
		dir, args = args[0], args[1:]
	}
	plan, err := xcode.MakePlan(dir, args...)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron _xcode:", err)
		return 1
	}
	if asJSON {
		encoded, _ := json.MarshalIndent(plan, "", "  ")
		fmt.Println(string(encoded))
		return 0
	}
	for _, field := range [][2]string{
		{"kind", plan.Kind}, {"container", plan.Container}, {"scheme", plan.Scheme},
		{"destination", plan.Destination}, {"setup", plan.Setup}, {"unsure", plan.Unsure},
	} {
		if field[1] != "" {
			fmt.Printf("%s\t%s\n", field[0], field[1])
		}
	}
	for _, path := range plan.Missing {
		fmt.Printf("missing\t%s\n", path)
	}
	for _, path := range plan.Nested {
		fmt.Printf("nested\t%s\n", path)
	}
	return 0
}
