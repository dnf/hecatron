package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/dnf/hecatron/internal/advisor"
	"github.com/dnf/hecatron/internal/generate"
	"github.com/dnf/hecatron/internal/project"
)

// The project commands manage.py runs: add, remove, enable, disable, check,
// dockerfile, workflows and init (init.go), printing and writing what it
// does. Arguments are read as its argparse reads them for the usual forms;
// any other form (an abbreviated option, -h, an option between languages)
// is left to argparse, whose messages and help depend on the Python
// version. Each command decides everything it will print and write before
// doing any of it, so a project the Go model declines is still handed over
// untouched.

// parsedArgs are a manage.py subcommand's arguments.
type parsedArgs struct {
	positionals []string
	flags       map[string]bool
	values      map[string]string
}

// parseManageArgs reads args with the given boolean flags and valued
// options; positionals must be one run (argparse's nargs="*" takes no
// more after an option) and at most maxPositionals (-1 for any). ok is
// false for anything else.
func parseManageArgs(args []string, flags, valued []string, maxPositionals int) (parsedArgs, bool) {
	a := parsedArgs{flags: map[string]bool{}, values: map[string]string{}}
	if len(args) > 0 && (args[0] == "help" || args[0] == "--help") {
		return a, false
	}
	runs, inRun := 0, false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			if !inRun {
				runs++
			}
			inRun = true
			a.positionals = append(a.positionals, arg)
			continue
		}
		inRun = false
		name, value, hasValue := strings.Cut(arg, "=")
		switch {
		case contains(flags, name) && !hasValue:
			a.flags[name] = true
		case contains(valued, name) && hasValue:
			a.values[name] = value
		case contains(valued, name) && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-"):
			i++
			a.values[name] = args[i]
		default:
			return a, false
		}
	}
	if runs > 1 || maxPositionals >= 0 && len(a.positionals) > maxPositionals {
		return a, false
	}
	return a, true
}

// manageFailure is manage.py's die: a message on stderr and a status.
type manageFailure struct {
	message string
	status  int
}

func (f *manageFailure) report() int {
	fmt.Fprintln(os.Stderr, "hecatron:", f.message)
	return f.status
}

// resolveAll is manage.py's resolve_all: each name as a language, failing
// on the first unknown one (or, with addedOnly, one not added). A name
// that isn't ASCII is declined: Python's str.lower and Go's differ there.
func (c *cli) resolveAll(names []string, addedOnly bool) ([]string, *manageFailure, bool) {
	var resolved []string
	for _, name := range names {
		for _, r := range name {
			if r > 0x7f {
				return nil, nil, false
			}
		}
		lang, err := c.project.ResolveName(name)
		if err != nil {
			return nil, nil, false
		}
		if lang == "" {
			return nil, &manageFailure{fmt.Sprintf("unknown language: %s (try 'hecatron list --all')", name), 1}, true
		}
		if addedOnly && !contains(c.project.Languages, lang) {
			return nil, &manageFailure{fmt.Sprintf("%s is not added; run 'hecatron add %s' first", lang, lang), 1}, true
		}
		resolved = append(resolved, lang)
	}
	return resolved, nil, true
}

// generatedFile is a file regenerate will write (or check).
type generatedFile struct {
	path, label, content string
}

// generatedFiles are what manage.py's write_workflows and write_dockerfile
// would write: each language's workflow (when the project generates
// workflows), then the Dockerfile (when it generates one). A failure the
// Dockerfile's generation reports (dockerfile.py's ValueError) comes back
// as failure, after the workflows; a declined one as an error.
func (c *cli) generatedFiles(workflows, dockerfile bool) ([]generatedFile, *manageFailure, error) {
	p := c.project
	var files []generatedFile
	if workflows {
		ci, err := c.ciFiles()
		if err != nil {
			return nil, nil, err
		}
		files = append(files, ci...)
	}
	if dockerfile {
		relative, err := p.Generated("dockerfile")
		if err != nil {
			return nil, nil, err
		}
		content, err := generate.Dockerfile(p)
		if e, ok := err.(*project.Error); ok {
			return files, &manageFailure{e.Message, 1}, nil
		}
		if err != nil {
			return nil, nil, err
		}
		files = append(files, generatedFile{project.PyJoin(p.Root, relative), relative, content})
	}
	return files, nil, nil
}

// ciSettings are the project's CI settings, with the CI system workflows
// --ci names in place of generated.ci.
func (c *cli) ciSettings() project.CISettings {
	settings, _ := c.project.CISettings()
	if c.ci != "" {
		settings.Systems = []string{c.ci}
	}
	return settings
}

// generatesGitHub reports whether the project generates GitHub workflows:
// it names where they go and generated.ci includes github.
func (c *cli) generatesGitHub() (bool, error) {
	template, err := c.project.Generated("workflows")
	return template != "" && contains(c.ciSettings().Systems, project.CIGitHub), err
}

// ciFiles are the generated CI files: each language's GitHub workflow (with
// cache steps when ci.cache is on), then the GitLab pipeline.
func (c *cli) ciFiles() ([]generatedFile, error) {
	p := c.project
	settings := c.ciSettings()
	var files []generatedFile
	github, err := c.generatesGitHub()
	if err != nil {
		return nil, err
	}
	for _, lang := range p.Languages {
		if !github {
			break
		}
		content, ok, err := generate.Workflow(p, lang)
		if err == nil && ok && settings.Cache {
			content, ok, err = generate.CachedWorkflow(p, lang, generate.HecatronVersion(p))
		}
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		path, err := generate.WorkflowPath(p, lang)
		if err != nil {
			return nil, err
		}
		files = append(files, generatedFile{path, project.PyRelpath(path, p.Root), content})
	}
	if contains(settings.Systems, project.CIGitLab) {
		content, err := generate.GitLabCI(p, settings, generate.HecatronVersion(p))
		if err != nil {
			return nil, err
		}
		files = append(files, generatedFile{project.PyJoin(p.Root, generate.GitLabPath), generate.GitLabPath, content})
	}
	updates, _ := p.UpdateSettings()
	githubActions := generate.HasGitHubWorkflows(p, github)
	for _, f := range []struct {
		path     string
		generate func(*project.Project, project.UpdateSettings, bool) (string, error)
	}{{updates.Dependabot, generate.Dependabot}, {updates.Renovate, generate.Renovate}} {
		if f.path == "" {
			continue
		}
		content, err := f.generate(p, updates, githubActions)
		if err != nil {
			return nil, err
		}
		files = append(files, generatedFile{project.PyJoin(p.Root, f.path), f.path, content})
	}
	return files, nil
}

// generatesUpdates reports whether the project generates dependency update
// configuration (generated.dependabot, generated.renovate).
func (c *cli) generatesUpdates() bool {
	updates, _ := c.project.UpdateSettings()
	return updates.Dependabot != "" || updates.Renovate != ""
}

// generatedProblems are what's wrong with the settings for the files only
// the Go core generates.
func (c *cli) generatedProblems() []string {
	_, problems := c.project.CISettings()
	_, updateProblems := c.project.UpdateSettings()
	return append(problems, updateProblems...)
}

// generates reports which files the project generates: workflows is any
// CI file.
func (c *cli) generates() (workflows, dockerfile bool, err error) {
	github, err := c.generatesGitHub()
	if err != nil {
		return false, false, err
	}
	relative, err := c.project.Generated("dockerfile")
	return github || contains(c.ciSettings().Systems, project.CIGitLab) || c.generatesUpdates(), relative != "", err
}

// writeGenerated writes (or checks) files as write_if_changed does, the
// messages to out; ok is whether every file is current.
func writeGenerated(files []generatedFile, check bool, out *strings.Builder) (bool, error) {
	ok := true
	for _, f := range files {
		current, err := generate.Write(f.path, f.content, f.label, check, out)
		if err != nil {
			return false, err
		}
		ok = ok && current
	}
	return ok, nil
}

// add is `hecatron add LANG...`.
func (c *cli) add() (int, bool) { return c.changeLanguages(true) }

// remove is `hecatron remove LANG...`.
func (c *cli) remove() (int, bool) { return c.changeLanguages(false) }

func (c *cli) changeLanguages(adding bool) (int, bool) {
	args, ok := parseManageArgs(c.args, nil, nil, -1)
	if !ok {
		return 0, false
	}
	if len(args.positionals) == 0 {
		return (&manageFailure{fmt.Sprintf("usage: hecatron %s <lang>...", c.command), 2}).report(), true
	}
	langs, failure, ok := c.resolveAll(args.positionals, false)
	if !ok {
		return 0, false
	}
	if failure != nil {
		return failure.report(), true
	}
	p := c.project
	before := append([]string(nil), p.Languages...)
	var out strings.Builder
	var changed, after []string
	for _, lang := range langs {
		if contains(before, lang) != adding {
			changed = append(changed, lang)
		}
	}
	for _, lang := range langs {
		if !contains(changed, lang) {
			if adding {
				fmt.Fprintf(&out, "Already added: %s\n", lang)
			} else {
				fmt.Fprintf(&out, "Not added: %s\n", lang)
			}
		}
	}
	if len(changed) == 0 {
		fmt.Print(out.String())
		return 0, true
	}
	if adding {
		after = append(append(after, before...), changed...)
	} else {
		for _, lang := range before {
			if !contains(changed, lang) {
				after = append(after, lang)
			}
		}
	}
	// What regenerating will write, worked out with the new languages before
	// anything is written.
	p.Languages = sortedUnique(after)
	workflows, dockerfile, err := c.generates()
	var files []generatedFile
	var failure2 *manageFailure
	if err == nil {
		files, failure2, err = c.generatedFiles(workflows, dockerfile)
	}
	var removals []string
	github, githubErr := c.generatesGitHub()
	if err == nil {
		err = githubErr
	}
	if err == nil && !adding && github {
		for _, lang := range changed {
			path, pathErr := generate.WorkflowPath(p, lang)
			if pathErr != nil {
				err = pathErr
				break
			}
			removals = append(removals, path)
		}
	}
	if err != nil {
		p.Languages = before
		return 0, false
	}
	if err := p.SaveLanguages(after); err != nil {
		if project.IsUnsupported(err) {
			p.Languages = before
			return 0, false
		}
		return c.writeFailed(&out, err), true
	}
	for i, lang := range changed {
		if adding {
			fmt.Fprintf(&out, "Added: %s\n", lang)
			continue
		}
		fmt.Fprintf(&out, "Removed: %s\n", lang)
		if len(removals) == 0 {
			continue
		}
		if info, statErr := os.Stat(removals[i]); statErr == nil && info.Mode().IsRegular() {
			if err := os.Remove(removals[i]); err != nil {
				return c.writeFailed(&out, err), true
			}
		}
	}
	if _, err := writeGenerated(files, false, &out); err != nil {
		return c.writeFailed(&out, err), true
	}
	fmt.Print(out.String())
	if failure2 != nil {
		return failure2.report(), true
	}
	return 0, true
}

// writeFailed reports a failure to write after deciding what to write,
// which manage.py would report with a traceback.
func (c *cli) writeFailed(out *strings.Builder, err error) int {
	fmt.Print(out.String())
	fmt.Fprintln(os.Stderr, "hecatron:", err)
	return 1
}

func sortedUnique(list []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, item := range list {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	sort.Strings(result)
	return result
}

// enable is `hecatron enable [LANG...|--all]`.
func (c *cli) enable() (int, bool) {
	args, ok := parseManageArgs(c.args, []string{"--all"}, nil, -1)
	if !ok {
		return 0, false
	}
	disabled := c.project.Disabled
	if len(args.positionals) == 0 && !args.flags["--all"] {
		if len(disabled) > 0 {
			fmt.Println("Disabled for you: " + strings.Join(disabled, ", "))
			fmt.Println("Use 'hecatron enable <lang>' or 'hecatron enable --all' to re-enable.")
		} else {
			fmt.Println("No languages are disabled.")
		}
		return 0, true
	}
	remaining := []string{}
	if !args.flags["--all"] {
		names, failure, ok := c.resolveAll(args.positionals, false)
		if !ok {
			return 0, false
		}
		if failure != nil {
			return failure.report(), true
		}
		for _, lang := range disabled {
			if !contains(names, lang) {
				remaining = append(remaining, lang)
			}
		}
	}
	if err := c.project.SaveDisabled(remaining); err != nil {
		if project.IsUnsupported(err) {
			return 0, false
		}
		return c.writeFailed(&strings.Builder{}, err), true
	}
	if len(remaining) > 0 {
		fmt.Println("Still disabled: " + strings.Join(remaining, ", "))
	} else {
		fmt.Println("All languages enabled.")
	}
	return 0, true
}

// disable is `hecatron disable [LANG...]`.
func (c *cli) disable() (int, bool) {
	args, ok := parseManageArgs(c.args, nil, nil, -1)
	if !ok {
		return 0, false
	}
	if len(args.positionals) == 0 {
		disabled := strings.Join(c.project.Disabled, ", ")
		if disabled == "" {
			disabled = "none"
		}
		fmt.Println("Disabled for you: " + disabled)
		return 0, true
	}
	names, failure, ok := c.resolveAll(args.positionals, true)
	if !ok {
		return 0, false
	}
	if failure != nil {
		return failure.report(), true
	}
	disabled := sortedUnique(append(append([]string(nil), c.project.Disabled...), names...))
	if err := c.project.SaveDisabled(disabled); err != nil {
		if project.IsUnsupported(err) {
			return 0, false
		}
		return c.writeFailed(&strings.Builder{}, err), true
	}
	fmt.Println("Disabled (for you, in parallel runs): " + strings.Join(disabled, ", "))
	return 0, true
}

// dockerfile is `hecatron dockerfile [--check]`.
func (c *cli) dockerfile() (int, bool) {
	return c.generateCommand("dockerfile", "no Dockerfile configured (config.json generated.dockerfile)", "Dockerfile is up to date.")
}

// workflows is `hecatron workflows [--check]`.
func (c *cli) workflows() (int, bool) {
	return c.generateCommand("workflows", "no workflow path configured (config.json generated.workflows)", "Workflows are up to date.")
}

func (c *cli) generateCommand(which, unconfigured, current string) (int, bool) {
	var valued []string
	if which == "workflows" {
		valued = []string{"--ci"}
	}
	args, ok := parseManageArgs(c.args, []string{"--check"}, valued, 0)
	if !ok {
		return 0, false
	}
	check := args.flags["--check"]
	if ci, given := args.values["--ci"]; given {
		if !contains(project.CISystems, ci) {
			return (&manageFailure{"--ci must be " + strings.Join(project.CISystems, " or "), 2}).report(), true
		}
		c.ci = ci
	}
	if which == "workflows" {
		if problems := c.generatedProblems(); len(problems) > 0 {
			return (&manageFailure{".hecatron/config.json: " + strings.Join(problems, "; "), 1}).report(), true
		}
	}
	workflows, dockerfile, err := c.generates()
	if err != nil {
		return 0, false
	}
	configured := map[string]bool{"workflows": workflows, "dockerfile": dockerfile}[which]
	if !configured {
		if !check {
			return (&manageFailure{unconfigured, 1}).report(), true
		}
		fmt.Println(current)
		return 0, true
	}
	files, failure, err := c.generatedFiles(which == "workflows", which == "dockerfile")
	if err != nil {
		return 0, false
	}
	if failure != nil {
		return failure.report(), true
	}
	var out strings.Builder
	upToDate, err := writeGenerated(files, check, &out)
	if err != nil {
		return c.writeFailed(&out, err), true
	}
	if check && upToDate {
		out.WriteString(current + "\n")
	}
	fmt.Print(out.String())
	if !upToDate {
		return 1, true
	}
	return 0, true
}

// check is `hecatron check`: generated files current, every workflow
// covering every component, no pin file disagreeing with the project's
// version, and config.json's audit and lint sections valid.
func (c *cli) check() (int, bool) {
	if _, ok := parseManageArgs(c.args, nil, nil, 0); !ok {
		return 0, false
	}
	p := c.project
	workflows, dockerfile, err := c.generates()
	if err != nil {
		return 0, false
	}
	var dockerfiles, workflowFiles []generatedFile
	var failure *manageFailure
	if dockerfile {
		if dockerfiles, failure, err = c.generatedFiles(false, true); err != nil {
			return 0, false
		}
		if failure != nil {
			return failure.report(), true
		}
	}
	if workflows {
		if workflowFiles, _, err = c.generatedFiles(true, false); err != nil {
			return 0, false
		}
	}
	coverage, err := generate.CoverageProblems(p)
	if err != nil {
		return 0, false
	}
	conflicts, err := p.VersionConflicts()
	if err != nil {
		return 0, false // manage.py doesn't catch these: a traceback
	}
	problems, err := p.ConfigProblems()
	if err != nil {
		return 0, false
	}
	advice, err := advisor.ConfigProblems(p) // Go only: the advisor has no script
	if err != nil {
		return 0, false
	}
	problems = append(problems, advice...)
	problems = append(problems, p.LicensesProblems()...)
	var out strings.Builder
	ok, err := writeGenerated(dockerfiles, true, &out)
	if err != nil {
		return 0, false
	}
	workflowsOK, err := writeGenerated(workflowFiles, true, &out)
	if err != nil {
		return 0, false
	}
	ok = ok && workflowsOK
	for _, problem := range coverage {
		out.WriteString(problem + "\n")
		ok = false
	}
	for _, conflict := range conflicts {
		pinned := conflict.PinSpec
		if pinned == "" {
			pinned = "no version"
		}
		fmt.Fprintf(&out, "%s in %s: %s pins %s, but the project's version is %s (the pin file wins; make them agree)\n",
			conflict.Lang, conflict.Component, strings.TrimPrefix(conflict.Source, "pin:"), pinned, conflict.ProjectSpec)
		ok = false
	}
	if ok {
		out.WriteString("Generated files are up to date and every workflow covers every component.\n")
	}
	problems = append(problems, c.generatedProblems()...)
	problems = append(problems, c.pinProblems()...)
	for _, problem := range problems {
		fmt.Fprintf(&out, ".hecatron/config.json: %s\n", problem)
	}
	fmt.Print(out.String())
	if !ok {
		fmt.Fprintln(os.Stderr, "Run 'hecatron dockerfile' / 'hecatron workflows' to regenerate.")
	}
	if ok && len(problems) == 0 {
		return 0, true
	}
	return 1, true
}

// relativeToWorkingDir is Python's os.path.relpath(path) against the
// working directory.
func relativeToWorkingDir(path string) string {
	cwd, err := workingDir()
	if err != nil {
		return path
	}
	return project.PyRelpath(path, cwd)
}
