package advisor

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dnf/hecatron/internal/generate"
	"github.com/dnf/hecatron/internal/project"
)

// ciFiles are where CI services read their configuration, relative to the
// repository's root.
var ciFiles = []string{
	".github/workflows/*.yml",
	".github/workflows/*.yaml",
	".github/actions/*/action.yml",
	".github/actions/*/action.yaml",
	".gitlab-ci.yml",
	".gitlab/ci/*.yml",
	".circleci/config.yml",
	"azure-pipelines.yml",
	".azure-pipelines/*.yml",
	".travis.yml",
	"Jenkinsfile",
	"bitbucket-pipelines.yml",
	".buildkite/*.yml",
	".woodpecker.yml",
	".woodpecker/*.yml",
	"appveyor.yml",
	".appveyor.yml",
	".cirrus.yml",
	".drone.yml",
}

// hecatronRun is a CI step running hecatron's run commands: without
// languages it covers them all, else those named.
var hecatronRun = regexp.MustCompile(`(?m)(?:^|[\s/])(?:hct|hecatron)[ \t]+(?:test|retest|build|rebuild|lint)\b([^\n|&;#]*)`)

// reusableWorkflow is a job calling another repository's workflow, which
// the advisor can't read: what it covers is unknown, so it isn't judged.
var reusableWorkflow = regexp.MustCompile(`(?m)^\s*uses:\s*['"]?[^\s'"./][^\s'"]*/\.github/workflows/`)

// ciText is the CI configuration's text, every file joined; "" when there
// is none.
func (r *run) ciText() string {
	var parts []string
	for _, pattern := range ciFiles {
		for _, path := range r.files.glob(r.p.Root, pattern) {
			parts = append(parts, r.files.read(path))
		}
	}
	return strings.Join(parts, "\n")
}

// coveredByHecatron is whether a CI step runs hecatron for the language.
func (r *run) coveredByHecatron(text, lang string) bool {
	for _, match := range hecatronRun.FindAllStringSubmatch(text, -1) {
		var named []string
		for _, arg := range strings.Fields(match[1]) {
			if !strings.HasPrefix(arg, "-") && !strings.ContainsAny(arg, "$\"'`") {
				named = append(named, arg)
			}
		}
		if len(named) == 0 {
			return true
		}
		for _, name := range named {
			if resolved, _ := r.p.ResolveName(name); resolved == lang {
				return true
			}
		}
	}
	return false
}

// ciCovers is a set's ci-covers rule for a language: some CI configuration
// mentions one of its markers, or runs it through hecatron. Generated
// workflows are judged by ciGenerated instead.
func (r *run) ciCovers(set *Set, lang string, dirs []project.ComponentDir) error {
	if len(dirs) == 0 {
		return nil
	}
	generated, err := r.p.Generated("workflows")
	if err != nil {
		return nil
	}
	if generated != "" {
		if source := r.p.Resolve(lang, "workflow.yml"); source != "" {
			return r.ciGenerated(lang)
		}
	}
	text := r.ciText()
	if text == "" {
		return nil // ci/configured says so once for the repository
	}
	if reusableWorkflow.MatchString(text) || r.coveredByHecatron(text, lang) {
		return nil
	}
	for _, marker := range set.CI.Markers {
		if r.files.compile(marker).MatchString(text) {
			return nil
		}
	}
	name := set.Toolchain
	if name == "" {
		name = r.name(lang)
	}
	f := Finding{ID: set.name + "/" + idCICovers, Level: Suggestion, Category: "ci",
		Message: fmt.Sprintf("Run %s's tests in CI: no CI configuration mentions it", name),
		Why:     set.CI.Why, Link: set.CI.Link, Dir: "."}
	r.add(lang, "", f)
	return nil
}

// ciGenerated checks a language's generated workflow, as hct check does:
// written, current, and covering every component.
func (r *run) ciGenerated(lang string) error {
	path, err := generate.WorkflowPath(r.p, lang)
	if err != nil || path == "" {
		return nil
	}
	content, ok, err := generate.Workflow(r.p, lang)
	if err != nil || !ok {
		return nil
	}
	relative := filepath.ToSlash(project.PyRelpath(path, r.p.Root))
	message := ""
	if current, err := project.ReadPyText(path); err != nil {
		message = fmt.Sprintf("Generate %s's workflow, %s: run hct workflows", r.name(lang), relative)
	} else if current != content {
		message = fmt.Sprintf("Regenerate %s, which is out of date: run hct workflows", relative)
	}
	if message == "" {
		problems, err := generate.CoverageProblems(r.p)
		if err != nil {
			return nil
		}
		for _, problem := range problems {
			if rest, ok := strings.CutPrefix(problem, lang+": "); ok {
				message = fmt.Sprintf("Make %s's workflow test every component: it %s", r.name(lang), strings.TrimPrefix(rest, "workflow "))
				break
			}
		}
	}
	if message != "" {
		r.add(lang, "", Finding{ID: idWorkflowCurrent, Level: Recommended, Category: "ci", Dir: ".",
			Message: message,
			Why:     "A workflow that isn't current or skips a component lets changes merge untested.",
			Link:    "https://docs.github.com/en/actions/get-started/understand-github-actions"})
	}
	return nil
}

// repository runs the rules about the whole repository: the repo set's
// rules, CI configured, and accepted audit findings past their date.
func (r *run) repository() error {
	if set := r.lib.Repo; set != nil {
		for i := range set.Rules {
			rule := &set.Rules[i]
			if r.files.eval(rule.When, r.p.Root) && !r.files.eval(rule.Expect, r.p.Root) {
				r.add("", "", rule.finding(set.name))
			}
		}
	}
	if len(r.p.Languages) > 0 && r.ciText() == "" {
		r.add("", "", Finding{ID: idCIConfigured, Level: Recommended, Category: "ci", Dir: ".",
			Message: "Set up CI that runs the tests on every change (no GitHub Actions, GitLab CI, CircleCI, Azure Pipelines, Travis CI, Jenkins, Buildkite or other CI configuration found)",
			Why:     "Tests that run only on someone's machine stop being run; CI runs them on every push and pull request.",
			Link:    "https://docs.github.com/en/actions/get-started/understand-github-actions"})
	}
	r.auditExpired()
	return nil
}
