package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dnf/hecatron/internal/actions"
	"github.com/dnf/hecatron/internal/project"
)

// plan prints what a language's default hook would run in a directory,
// without running it (the declarative definitions' prototype, #83):
//
//	hecatron plan <lang> <action> [dir]
//
// HECATRON_ROOT is the project root (default: dir).
func plan(home project.Home, args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: hecatron plan <lang> <action> [dir]")
		return 2
	}
	lang, action, dir := args[0], args[1], "."
	if len(args) > 2 {
		dir = args[2]
	}
	if err := printPlan(home, lang, action, dir); err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return 1
	}
	return 0
}

func printPlan(home project.Home, lang, action, dir string) error {
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
	def, err := actions.Load(home.Languages, lang)
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
