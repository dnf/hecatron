package install

import (
	"fmt"
	"io"
	"strings"
)

// Language is one language to install from its definition.
type Language struct {
	Name string
	Def  *Definition
	Path string // the install.json it came from
	Ctx  *Context
	// Reported are the commands the doctor already checks (config.json's
	// dependency and lintDependency): a tool whose check is one of them
	// isn't reported twice.
	Reported []string
}

// Action kinds.
const (
	KindToolchain = "toolchain"
	KindPackages  = "packages"
	KindTool      = "tool"
	KindStep      = "step"
)

// Action is one thing a language's install does, with its check.
type Action struct {
	Kind     string
	Label    string
	Packages []string   // what it installs (toolchain, packages, tool)
	Run      []string   // what it runs (step)
	When     *Condition // whether it applies
	Check    *Condition // whether it's done; nil: its packages are installed
	Problem  string     // the doctor's message while it isn't done
	NoReport bool       // the doctor checks it already (dependency, lintDependency)

	// Worked out by the plan.
	Applies bool
	Reason  string   // why it doesn't apply
	Done    bool     // its check passes
	Missing []string // packages not installed
}

// LanguagePlan is one language's actions.
type LanguagePlan struct {
	Language *Language
	Actions  []*Action
}

// Plan is the install of some languages from their definitions on one
// package manager.
type Plan struct {
	Manager   *Manager
	Languages []*LanguagePlan
}

// Actions are a language's actions, in order (toolchain, packages, tools,
// steps), not yet checked.
func (l *Language) Actions(manager string) []*Action {
	def := l.Def
	actions := []*Action{{Kind: KindToolchain, Label: strings.Join(def.Toolchain.Packages[manager], " "),
		Packages: def.Toolchain.Packages[manager], Applies: true}}
	for _, entry := range def.Packages {
		actions = append(actions, &Action{Kind: KindPackages, Label: strings.Join(entry.Managers[manager], " "),
			Packages: entry.Managers[manager], When: entry.When, Check: entry.Unless})
	}
	for _, tool := range def.Tools {
		problem := tool.Name + " not installed"
		if tool.When != nil && tool.When.Because != "" {
			problem += " (" + tool.When.Because + ")"
		}
		action := &Action{Kind: KindTool, Label: tool.Name, Packages: tool.Packages[manager], When: tool.When,
			Check: tool.Check, Problem: problem}
		if tool.Check != nil && tool.Check.Command != "" && contains(l.Reported, tool.Check.Command) {
			action.NoReport = true
		}
		actions = append(actions, action)
	}
	for _, step := range def.Steps {
		label := step.Name
		if label == "" {
			label = strings.Join(step.Run, " ")
		}
		actions = append(actions, &Action{Kind: KindStep, Label: label, Run: step.Run, When: step.When,
			Check: step.Check, Problem: step.Problem})
	}
	return actions
}

// evaluate works out whether an action applies and whether it's done.
func (a *Action) evaluate(ctx *Context, manager *Manager) {
	a.Applies, a.Reason = true, ""
	switch {
	case a.Kind != KindStep && a.Kind != KindToolchain && len(a.Packages) == 0:
		a.Applies, a.Reason = false, "no "+manager.Name+" package"
	case !a.When.Holds(ctx):
		a.Applies, a.Reason = false, "not needed: not ("+a.When.String()+")"
	}
	if !a.Applies {
		return
	}
	if a.Check != nil {
		a.Done = a.Check.Holds(ctx)
		if !a.Done {
			a.Missing = a.Packages
		}
		return
	}
	a.Missing = manager.Missing(a.Packages)
	a.Done = len(a.Missing) == 0
}

// NewPlan checks every action of the languages.
func NewPlan(manager *Manager, languages []*Language) *Plan {
	plan := &Plan{Manager: manager}
	for _, language := range languages {
		lp := &LanguagePlan{Language: language, Actions: language.Actions(manager.Name)}
		for _, action := range lp.Actions {
			action.evaluate(language.Ctx, manager)
		}
		plan.Languages = append(plan.Languages, lp)
	}
	return plan
}

// Problems are what the doctor reports for a language from its
// definition: each tool and step that applies and whose check fails. The
// package manager isn't asked: a toolchain's own command is the doctor's
// dependency check.
func Problems(language *Language, manager *Manager) []string {
	var problems []string
	for _, action := range language.Actions(manager.Name) {
		if action.Kind != KindTool && action.Kind != KindStep || action.NoReport {
			continue
		}
		action.evaluate(language.Ctx, manager)
		if action.Applies && !action.Done {
			problems = append(problems, action.Problem)
		}
	}
	return problems
}

// missingPackages are the packages the plan installs, each once, in order.
func (p *Plan) missingPackages(languages []*LanguagePlan) []string {
	var packages []string
	for _, lp := range languages {
		for _, action := range lp.Actions {
			if action.Applies && !action.Done && action.Kind != KindStep {
				for _, name := range action.Missing {
					if !contains(packages, name) {
						packages = append(packages, name)
					}
				}
			}
		}
	}
	return packages
}

// Print writes the plan: per language, each action and what will happen to
// it, then the commands that would run.
func (p *Plan) Print(w io.Writer) {
	for _, lp := range p.Languages {
		_, _ = fmt.Fprintf(w, "%s: %s\n", lp.Language.Name, lp.Language.Path)
		for _, action := range lp.Actions {
			_, _ = fmt.Fprintf(w, "  %-8s %s\n", action.state(), action.describe(p.Manager.Name))
		}
	}
}

// Commands are what applying the plan would run, as text.
func (p *Plan) Commands() []string {
	var commands []string
	if packages := p.missingPackages(p.Languages); len(packages) > 0 {
		for _, command := range p.Manager.InstallCommands(packages) {
			commands = append(commands, strings.Join(command, " "))
		}
	}
	for _, lp := range p.Languages {
		for _, action := range lp.Actions {
			if action.Kind == KindStep && action.Applies && !action.Done {
				commands = append(commands, strings.Join(action.Run, " ")+"   ("+lp.Language.Name+")")
			}
		}
	}
	return commands
}

func (a *Action) state() string {
	switch {
	case !a.Applies:
		return "skip"
	case a.Done:
		return "ok"
	case a.Kind == KindStep:
		return "run"
	}
	return "install"
}

func (a *Action) describe(manager string) string {
	text := a.Kind
	switch a.Kind {
	case KindStep:
		text += " " + a.Label
	case KindTool:
		text += " " + a.Label
		if len(a.Packages) > 0 {
			text += " (" + manager + ": " + strings.Join(a.Packages, " ") + ")"
		}
	default:
		text += " (" + manager + ": " + a.Label + ")"
	}
	switch {
	case !a.Applies:
		text += ": " + a.Reason
	case !a.Done && a.Kind == KindStep:
		text += ": " + a.Problem
	case !a.Done && len(a.Missing) > 0 && a.Check == nil:
		text += ": missing " + strings.Join(a.Missing, " ")
	}
	return text
}
