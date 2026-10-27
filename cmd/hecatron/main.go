// Command hecatron is the Go core (docs/design.md §19). This prototype has
// one command, plan, which prints what a language's default hook would run
// in a directory without running it:
//
//	hecatron plan <lang> <action> [dir]
//
// HECATRON_HOME is hecatron's checkout (default: two directories above this
// binary's source); HECATRON_ROOT is the project root (default: dir).
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dnf/hecatron/internal/actions"
)

func main() {
	if len(os.Args) < 4 || os.Args[1] != "plan" {
		fmt.Fprintln(os.Stderr, "usage: hecatron plan <lang> <action> [dir]")
		os.Exit(2)
	}
	lang, action, dir := os.Args[2], os.Args[3], "."
	if len(os.Args) > 4 {
		dir = os.Args[4]
	}
	if err := plan(lang, action, dir); err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		os.Exit(1)
	}
}

func plan(lang, action, dir string) error {
	home := os.Getenv("HECATRON_HOME")
	if home == "" {
		home = "."
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	root := os.Getenv("HECATRON_ROOT")
	if root == "" {
		root = abs
	}
	if root, err = filepath.Abs(root); err != nil {
		return err
	}
	def, err := actions.Load(filepath.Join(home, "languages"), lang)
	if err != nil {
		return err
	}
	ctx := actions.Context{Action: action, Dir: abs, Root: root}
	ops, err := actions.Plan(def, ctx)
	if err != nil {
		return err
	}
	for _, line := range actions.Lines(def, ctx, ops) {
		fmt.Println(line)
	}
	return nil
}
