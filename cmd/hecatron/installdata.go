package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dnf/hecatron/internal/install"
	"github.com/dnf/hecatron/internal/project"
)

// Installs from data (docs/contributing/install-engine.md): a language
// whose install.json this binary can carry out is installed, repaired,
// checked and uninstalled from it; any other runs its scripts.

// installerChoice is how a language installs.
type installerChoice struct {
	lang   string
	def    *install.Definition // nil: its script
	path   string              // the install.json, or ""
	reason string              // why its script runs although it has an install.json
}

// declinedStatus is `hecatron _install`'s exit status when the language
// isn't installed from data here, so the install.sh that asked carries on.
const declinedStatus = 125

// platform is this machine's, as hecatron names it.
func platform() install.Platform {
	arch := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[runtime.GOARCH]
	osName := map[string]string{"darwin": "macos"}[runtime.GOOS]
	if osName == "" {
		osName = runtime.GOOS
	}
	return install.Platform{OS: osName, Arch: arch}
}

// packageManager is the package manager installs from data use here, or
// nil.
func packageManager(env []string) *install.Manager {
	return install.ManagerFor(platform(), install.OS{Stdout: os.Stdout, Stderr: os.Stderr}, env)
}

// chooseInstaller decides between a language's install.json and its
// script (script: "install.sh" or "uninstall.sh"), in order:
//  1. the project's own script always wins (a project override);
//  2. HECATRON_INSTALLER=script forces the scripts;
//  3. an install.json (the project's, which replaces the built-in one
//     whole, else the built-in one) that this binary can carry out on this
//     platform;
//  4. otherwise the script.
func chooseInstaller(p *project.Project, lang, script string, manager *install.Manager) installerChoice {
	choice := installerChoice{lang: lang}
	path := p.Resolve(lang, install.FileName)
	switch {
	case path == "":
		return choice
	case isProjectFile(p, lang, script):
		choice.reason = "the project's own " + script
		return choice
	case os.Getenv("HECATRON_INSTALLER") == "script":
		choice.reason = "HECATRON_INSTALLER=script"
		return choice
	}
	def, err := install.LoadFile(path)
	if err != nil {
		choice.reason = err.Error()
		return choice
	}
	if why := def.Unsupported(manager.Name, platform()); why != "" {
		choice.reason = install.FileName + ": " + why
		return choice
	}
	choice.def, choice.path = def, path
	return choice
}

func isProjectFile(p *project.Project, lang, name string) bool {
	info, err := os.Stat(filepath.Join(p.Dir, "languages", lang, name))
	return err == nil && !info.IsDir()
}

// installLanguage is a language as the executor sees it: its definition,
// its components' directories and its environment.
func installLanguage(p *project.Project, choice installerChoice, env []string) (*install.Language, error) {
	dirs, err := p.ImplDirs(choice.lang)
	if err != nil {
		return nil, err
	}
	language := &install.Language{Name: choice.lang, Def: choice.def, Path: choice.path}
	if version, err := p.DefaultVersion(choice.lang); err == nil {
		language.Version = strings.TrimPrefix(version, "=")
	}
	if choice.def.Toolchain.Versions == "managed" {
		if language.Versions, err = missingVersions(p, choice.lang); err != nil {
			return nil, err
		}
	}
	languageEnv, err := p.LanguageEnv(choice.lang, "")
	if err != nil {
		return nil, err
	}
	language.Ctx = &install.Context{Root: p.Root, Env: languageEnv.Apply(env), Platform: platform(),
		System: install.OS{Stdout: os.Stdout, Stderr: os.Stderr}}
	for _, dir := range dirs {
		language.Ctx.Dirs = append(language.Ctx.Dirs, dir.Dir)
	}
	for _, get := range []func(string) (string, error){p.Dependency, p.LintDependency} {
		if command, _ := get(choice.lang); command != "" {
			language.Reported = append(language.Reported, command)
		}
	}
	return language, nil
}

// missingVersions are the exact versions a "managed" toolchain installs
// besides its default: each one the components resolve to that isn't
// installed yet (docs/guide/versions.md), a prefix as the newest release in the definition's
// releases.json it matches, so planning stays offline.
func missingVersions(p *project.Project, lang string) ([]string, error) {
	missing, err := p.MissingVersions(lang)
	if err != nil {
		return nil, err
	}
	var versions []string
	for _, m := range missing {
		version := strings.TrimPrefix(m.Spec, "=")
		if version == m.Spec {
			if version, err = p.NewestRelease(lang, m.Spec); err != nil {
				return nil, err
			}
		}
		if version == "" {
			return nil, &project.Error{Message: fmt.Sprintf("%s: no %s release matches %s (from %s) in its releases.json",
				m.Component, lang, m.Spec, m.Source)}
		}
		if !contains(versions, version) {
			versions = append(versions, version)
		}
	}
	return versions, nil
}

// applyData installs languages from their definitions in one plan (one
// package-manager call for all of them), reporting each failure. It
// returns the languages that failed, in order.
func applyData(p *project.Project, choices []installerChoice, env []string, manager *install.Manager) []string {
	if len(choices) == 0 {
		return nil
	}
	plan, failed := planData(p, choices, env, manager)
	errs := plan.Apply(os.Stdout, install.ReceiptsDir(env))
	for _, choice := range choices {
		if err := errs[choice.lang]; err != nil {
			fmt.Fprintf(os.Stderr, "hecatron: %s: %v\n", choice.lang, err)
			failed = append(failed, choice.lang)
		}
	}
	return failed
}

func planData(p *project.Project, choices []installerChoice, env []string, manager *install.Manager) (*install.Plan, []string) {
	var languages []*install.Language
	var failed []string
	for _, choice := range choices {
		language, err := installLanguage(p, choice, env)
		if err != nil {
			fmt.Fprintf(os.Stderr, "hecatron: %s: %v\n", choice.lang, err)
			failed = append(failed, choice.lang)
			continue
		}
		languages = append(languages, language)
	}
	return install.NewPlan(manager, languages), failed
}

// printInstallPlan is `hecatron install --plan`: how each language would
// install and what that would run, changing nothing.
func printInstallPlan(w io.Writer, p *project.Project, choices []installerChoice, env []string, manager *install.Manager) {
	var data []installerChoice
	for _, choice := range choices {
		if choice.def != nil {
			data = append(data, choice)
		}
	}
	var commands []string
	if len(data) > 0 {
		plan, _ := planData(p, data, env, manager)
		plan.Print(w)
		commands = plan.Commands()
	}
	for _, choice := range choices {
		if choice.def != nil {
			continue
		}
		script := p.Resolve(choice.lang, "install.sh")
		why := ""
		if choice.reason != "" {
			why = " (" + choice.reason + ")"
		}
		_, _ = fmt.Fprintf(w, "%s: %s%s\n", choice.lang, displayScript(script), why)
		if script != "" {
			commands = append(commands, "bash "+script)
		}
	}
	_, _ = fmt.Fprintln(w)
	if len(commands) == 0 {
		_, _ = fmt.Fprintln(w, "Nothing to do: everything is installed.")
		return
	}
	_, _ = fmt.Fprintln(w, "Would run:")
	for _, command := range commands {
		_, _ = fmt.Fprintln(w, "  "+command)
	}
}

func displayScript(script string) string {
	if script == "" {
		return "no install.sh"
	}
	return script
}

// uninstallData is a language's uninstall from its receipt.
func uninstallData(p *project.Project, choice installerChoice, env []string, manager *install.Manager) int {
	others := map[string]*install.Definition{}
	if defs, err := p.Definitions(); err == nil {
		for _, lang := range defs.Names {
			if path := p.Resolve(lang, install.FileName); path != "" && lang != choice.lang {
				if def, err := install.LoadFile(path); err == nil {
					others[lang] = def
				}
			}
		}
	}
	dir := install.ReceiptsDir(env)
	removal, err := install.PlanUninstall(choice.lang, choice.def, manager, dir, others)
	if err == nil {
		fmt.Printf("==> Uninstalling %s (%s)\n", choice.lang, choice.path)
		err = removal.Apply(os.Stdout, manager, dir)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "hecatron: %s: %v\n", choice.lang, err)
		return 1
	}
	return 0
}

// installHidden is `hecatron _install LANG`, which a converted language's
// install.sh runs (engine_install in _common.sh) when another installer
// calls it (Elixir's and Gleam's run Erlang's): the language from its
// install.json, as `hecatron install` would, or declinedStatus when it
// isn't installed from data here, so the script carries on itself. It
// never runs a script, so the two can't call each other in a loop.
func (c *cli) installHidden() int {
	if len(c.args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: hecatron _install <lang>")
		return 2
	}
	if !c.locateProject(false) && !c.detectProject() || c.project == nil {
		return declinedStatus
	}
	env := c.environment(nil)
	c.project.Environ = env
	manager := packageManager(env)
	choice := chooseInstaller(c.project, c.args[0], "install.sh", manager)
	if choice.def == nil {
		return declinedStatus
	}
	if failed := applyData(c.project, []installerChoice{choice}, env, manager); len(failed) > 0 {
		return 1
	}
	return 0
}

// dataProblems are the doctor's problems for a language installed from
// data: each tool and step whose check fails.
func dataProblems(p *project.Project, choice installerChoice, env []string, manager *install.Manager) []string {
	language, err := installLanguage(p, choice, env)
	if err != nil {
		var e *project.Error
		if errors.As(err, &e) {
			return []string{e.Message}
		}
		return []string{err.Error()}
	}
	return install.Problems(language, manager)
}
