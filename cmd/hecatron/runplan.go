package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/dnf/hecatron/internal/changed"
	"github.com/dnf/hecatron/internal/project"
	"github.com/dnf/hecatron/internal/runner"
)

// A run command's --plan (#263): what it would run, worked out as the run
// works it out (--changed's selection, the jobs, the skips known before
// anything starts), printed instead of running anything. hct install
// --plan is the same idea for installing.

// runPlan is the plan as JSON output has it.
type runPlan struct {
	Command    string      `json:"command"`
	Summary    string      `json:"summary,omitempty"` // --changed's line
	Base       string      `json:"base,omitempty"`
	BaseSource string      `json:"baseSource,omitempty"`
	MergeBase  string      `json:"mergeBase,omitempty"`
	Note       string      `json:"note,omitempty"`       // no base: everything runs
	Everything string      `json:"everything,omitempty"` // the file that makes everything run
	Files      *[]planFile `json:"files,omitempty"`      // with --changed
	Jobs       []planJob   `json:"jobs"`
	Skipped    []planSkip  `json:"skipped"`
}

// planFile is a changed file and what it selects.
type planFile struct {
	Path      string     `json:"path"`
	Reason    string     `json:"reason"` // changed.Reason*
	Why       string     `json:"why"`
	Language  string     `json:"language,omitempty"`  // the directory it's in
	Component string     `json:"component,omitempty"` // (reason "component")
	Selects   []planPick `json:"selects,omitempty"`
	Tasks     []string   `json:"tasks,omitempty"`
}

type planPick struct {
	Language  string `json:"language"`
	Component string `json:"component"`
}

// planJob is a job the run would start.
type planJob struct {
	Action     string   `json:"action"` // rebuild and retest: clean, then build or test
	Label      string   `json:"label"`
	Language   string   `json:"language,omitempty"`
	Task       bool     `json:"task,omitempty"`
	Components []string `json:"components,omitempty"`
	Dirs       []string `json:"dirs,omitempty"` // relative to the root, ending in a slash
}

// planSkip is a job the run would skip without starting it.
type planSkip struct {
	Action   string `json:"action"`
	Label    string `json:"label"`
	Language string `json:"language,omitempty"`
	Reason   string `json:"reason"`
}

// printPlan prints the plan; the exit status is 0, or 1 when it can't be
// worked out.
func (r *invocation) printPlan() int {
	r.project.Environ = r.environment()
	plan := runPlan{Command: r.command, Jobs: []planJob{}, Skipped: []planSkip{}}
	if s := r.selection; s != nil {
		plan.Summary, plan.Note, plan.Everything = s.Line(), s.Note, s.Everything
		if s.Note == "" {
			plan.Base, plan.BaseSource, plan.MergeBase = s.Base.Ref, s.Base.Source, s.Base.Commit
		}
		files := planFiles(s, r.command)
		plan.Files = &files
	}
	actions := []string{r.command}
	if then, ok := runThen[r.command]; ok {
		actions = []string{"clean", then}
	}
	for _, action := range actions {
		sub := r
		if action != r.command {
			sub = r.sub(action)
		}
		jobs, skipped, err := sub.planJobs()
		if err != nil {
			fmt.Fprintln(os.Stderr, "hecatron:", err)
			return 1
		}
		plan.Jobs, plan.Skipped = append(plan.Jobs, jobs...), append(plan.Skipped, skipped...)
	}
	switch r.format {
	case runner.JSON:
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(plan); err != nil {
			fmt.Fprintln(os.Stderr, "hecatron:", err)
			return 1
		}
	case runner.Agent:
		writeAgentPlan(os.Stdout, &plan, len(actions) > 1)
	default:
		writeHumanPlan(os.Stdout, &plan, len(actions) > 1)
	}
	return 0
}

// planFiles are the changed files, the nested repositories' among them, in
// path order. A shell script selects lint's shell check only.
func planFiles(s *changed.Selection, command string) []planFile {
	files := []planFile{}
	for _, a := range s.Attributions {
		f := planFile{Path: a.File, Reason: a.Reason, Why: a.Why, Language: a.Language, Component: a.Component, Tasks: a.Tasks}
		if f.Reason == changed.ReasonShell && command != "lint" {
			f.Reason, f.Why = changed.ReasonOther, "a shell script, which only hct lint's shell check reads"
		}
		for _, p := range a.Selects {
			f.Selects = append(f.Selects, planPick{p.Language, p.Component})
		}
		files = append(files, f)
	}
	for _, file := range s.Nested {
		files = append(files, planFile{Path: file, Reason: changed.ReasonNested, Why: "inside another git repository below the root"})
	}
	sort.SliceStable(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files
}

// planJobs are the jobs this action would start and those it would skip
// up front: named targets one after another, else the parallel jobs.
func (r *invocation) planJobs() ([]planJob, []planSkip, error) {
	list := r.named
	if len(list) == 0 {
		list = r.jobs()
	}
	var jobs []planJob
	var skipped []planSkip
	for _, job := range list {
		skip := planSkip{Action: r.command, Label: job.Label, Language: job.Lang}
		switch job.Kind {
		case runner.Separator:
			continue
		case runner.Disabled:
			skip.Reason = "disabled"
			skipped = append(skipped, skip)
			continue
		case runner.NotInstalled:
			skip.Reason = r.notInstalled(job)
			skipped = append(skipped, skip)
			continue
		}
		planned := planJob{Action: r.command, Label: job.Label, Language: job.Lang, Task: job.Task}
		if job.Lang != "" {
			if r.project.Resolve(job.Lang, r.command+".sh") == "" {
				skip.Reason, _ = noHookReason(r.command, job.Lang)
				if skip.Reason == "" {
					skip.Reason = fmt.Sprintf("%s has no %s hook: the run would fail", job.Lang, r.command)
				}
				skipped = append(skipped, skip)
				continue
			}
			components, dirs, err := r.planDirs(job.Lang)
			if err != nil {
				return nil, nil, err
			}
			planned.Components, planned.Dirs = components, dirs
		}
		jobs = append(jobs, planned)
	}
	return jobs, skipped, nil
}

// notInstalled is why a job isn't run for want of its tool.
func (r *invocation) notInstalled(job runner.Job) string {
	tool := ""
	switch {
	case job.Lang != "":
		tool, _ = r.project.Dependency(job.Lang)
	case job.Label == "shell":
		tool = "shellcheck"
	case job.Label == secretsTarget:
		tool = "gitleaks"
	}
	if tool == "" {
		return "not installed"
	}
	return "not installed (" + tool + " not found)"
}

// planDirs are the components a language job would run for and their
// directories, as languageExec picks them.
func (r *invocation) planDirs(lang string) (components, dirs []string, err error) {
	wanted, _ := r.wantedComponents(lang)
	all, err := r.project.ImplDirs(lang)
	if err != nil {
		return nil, nil, err
	}
	for _, d := range all {
		if len(wanted) > 0 && !contains(wanted, d.Component) {
			continue
		}
		components = append(components, d.Component)
		dirs = append(dirs, relDir(r.root, d.Dir))
	}
	return components, dirs, nil
}

// relDir is a directory relative to the root, ending in a slash ("./" for
// the root itself), as agent output shows directories.
func relDir(root, dir string) string {
	rel := project.PyRelpath(dir, root)
	if rel == "." {
		return "./"
	}
	return strings.TrimSuffix(rel, "/") + "/"
}

// fileLine is a changed file's attribution in words: what it selects, and
// why.
func fileLine(f planFile) string {
	what := "nothing"
	switch {
	case f.Reason == changed.ReasonConfiguration:
		what = "everything"
	case len(f.Selects) > 0:
		what = picks(f.Selects)
	case f.Reason == changed.ReasonShell:
		what = "shell"
	}
	for _, task := range f.Tasks {
		if what == "nothing" {
			what = "task " + task
		} else {
			what += ", task " + task
		}
	}
	return what + ": " + f.Why
}

// picks are languages and their components: "go [a, b], python [a]".
func picks(list []planPick) string {
	var order []string
	components := map[string][]string{}
	for _, p := range list {
		if components[p.Language] == nil {
			order = append(order, p.Language)
		}
		components[p.Language] = append(components[p.Language], p.Component)
	}
	var parts []string
	for _, lang := range order {
		parts = append(parts, lang+" ["+strings.Join(components[lang], ", ")+"]")
	}
	return strings.Join(parts, ", ")
}

// jobWhat is a job's components and directories, or that it's a task.
func jobWhat(j planJob) string {
	switch {
	case j.Task:
		return "project task"
	case j.Language == "":
		return "tool"
	case len(j.Dirs) == 0:
		return "no component"
	}
	return strings.Join(j.Dirs, " ")
}

func planHead(plan *runPlan) string {
	return fmt.Sprintf("hct %s --plan: %d job%s would run, %d skipped; nothing was run",
		plan.Command, len(plan.Jobs), plural(len(plan.Jobs)), len(plan.Skipped))
}

// writeHumanPlan prints the plan in columns.
func writeHumanPlan(w io.Writer, plan *runPlan, actions bool) {
	_, _ = fmt.Fprintln(w, planHead(plan))
	if plan.Summary != "" {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, plan.Summary)
	}
	if plan.Files != nil && len(*plan.Files) > 0 {
		width := 0
		for _, f := range *plan.Files {
			width = max(width, len(f.Path))
		}
		for _, f := range *plan.Files {
			_, _ = fmt.Fprintf(w, "  %-*s  %s\n", width, f.Path, fileLine(f))
		}
	}
	label := func(action, name string) string {
		if actions {
			return action + " " + name
		}
		return name
	}
	width := 0
	for _, j := range plan.Jobs {
		width = max(width, len(label(j.Action, j.Label)))
	}
	for _, s := range plan.Skipped {
		width = max(width, len(label(s.Action, s.Label)))
	}
	if len(plan.Jobs) > 0 {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, "Would run:")
		for _, j := range plan.Jobs {
			_, _ = fmt.Fprintf(w, "  %-*s  %s\n", width, label(j.Action, j.Label), jobWhat(j))
		}
	}
	if len(plan.Skipped) > 0 {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, "Would skip:")
		for _, s := range plan.Skipped {
			_, _ = fmt.Fprintf(w, "  %-*s  %s\n", width, label(s.Action, s.Label), s.Reason)
		}
	}
}

// writeAgentPlan prints the plan a line per item: FILE, JOB, SKIP.
func writeAgentPlan(w io.Writer, plan *runPlan, actions bool) {
	_, _ = fmt.Fprintln(w, planHead(plan))
	if plan.Summary != "" {
		_, _ = fmt.Fprintln(w, plan.Summary)
	}
	if plan.Files != nil {
		for _, f := range *plan.Files {
			_, _ = fmt.Fprintf(w, "FILE %s: %s\n", f.Path, fileLine(f))
		}
	}
	prefix := func(action string) string {
		if actions {
			return action + " "
		}
		return ""
	}
	for _, j := range plan.Jobs {
		_, _ = fmt.Fprintf(w, "JOB %s%s (%s)\n", prefix(j.Action), j.Label, jobWhat(j))
	}
	for _, s := range plan.Skipped {
		_, _ = fmt.Fprintf(w, "SKIP %s%s: %s\n", prefix(s.Action), s.Label, s.Reason)
	}
}
