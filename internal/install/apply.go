package install

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Apply carries out the plan: every missing system package of every
// language in one package-manager call (each language's alone, if that
// fails, so one bad package fails only its language), then each language's
// steps whose checks fail, then every check again. A rerun finds every
// check passing and does nothing. What it installed is added to each
// language's receipt in receiptsDir. The result maps each language that
// failed to why.
func (p *Plan) Apply(w io.Writer, receiptsDir string) map[string]error {
	failed := map[string]error{}
	files := map[string][]string{}
	refresh := false
	var added []*Action
	for _, lp := range p.Languages {
		for _, action := range pendingSources([]*LanguagePlan{lp}) {
			written, err := action.Source.add(w, lp.Language.Name, p.Manager)
			files[lp.Language.Name] = append(files[lp.Language.Name], written...)
			if err != nil {
				failed[lp.Language.Name] = err
				break
			}
			if action.Source.unavailable {
				action.evaluate(lp.Language.Ctx, p.Manager) // no longer applies, upgrades nor replaces
				continue
			}
			added = append(added, action)
			refresh = true
		}
	}
	if replaced := replacedPackages(added); len(replaced) > 0 {
		command := p.Manager.RemoveCommand(replaced)
		_, _ = fmt.Fprintf(w, "==> %s\n", strings.Join(command, " "))
		if err := p.Manager.system.Run(command, "", p.Manager.env); err != nil {
			for _, lp := range p.Languages {
				if len(replacedPackages(pendingSources([]*LanguagePlan{lp}))) > 0 && failed[lp.Language.Name] == nil {
					failed[lp.Language.Name] = fmt.Errorf("removing %s: %w", strings.Join(replaced, " "), err)
				}
			}
		}
	}
	var active []*LanguagePlan // those whose sources were added
	for _, lp := range p.Languages {
		if failed[lp.Language.Name] == nil {
			active = append(active, lp)
		}
	}
	if packages := p.missingPackages(active); len(packages) > 0 {
		var names []string
		for _, lp := range active {
			if len(p.missingPackages([]*LanguagePlan{lp})) > 0 {
				names = append(names, lp.Language.Name)
			}
		}
		_, _ = fmt.Fprintf(w, "==> Installing %s for %s\n", strings.Join(packages, " "), strings.Join(names, ", "))
		if err := p.install(w, packages, refresh); err != nil && len(names) > 1 {
			_, _ = fmt.Fprintf(w, "==> That failed (%v); installing each language's packages alone\n", err)
			for _, lp := range active {
				if alone := p.missingPackages([]*LanguagePlan{lp}); len(alone) > 0 {
					if err := p.install(w, alone, false); err != nil {
						failed[lp.Language.Name] = fmt.Errorf("installing %s: %w", strings.Join(alone, " "), err)
					}
				}
			}
		} else if err != nil {
			failed[names[0]] = fmt.Errorf("installing %s: %w", strings.Join(packages, " "), err)
		}
	}
	installedNow := p.Manager.Installed(p.missingPackages(active))
	for _, lp := range p.Languages {
		lang := lp.Language.Name
		did := installed{files: files[lang]}
		for _, name := range p.newPackages(lp) {
			if installedNow[name] {
				did.packages = append(did.packages, name)
			}
		}
		if failed[lang] == nil {
			failed[lang] = p.unpackAndLink(w, lp, &did)
		}
		if failed[lang] == nil {
			failed[lang] = p.runSteps(w, lp, &did)
		}
		if failed[lang] == nil {
			failed[lang] = p.verify(lp)
		}
		if err := record(receiptsDir, lang, p.Manager.Name, did); err != nil && failed[lang] == nil {
			failed[lang] = fmt.Errorf("writing its receipt: %w", err)
		}
		if failed[lang] == nil {
			delete(failed, lang)
		}
	}
	return failed
}

// newPackages are the packages the plan installs for a language that
// weren't installed at all (not those an apt source upgrades), which its
// receipt lists.
func (p *Plan) newPackages(lp *LanguagePlan) []string {
	var packages []string
	for _, action := range lp.Actions {
		if action.Applies && !action.Done && action.Kind != KindStep {
			packages = append(packages, action.Missing...)
		}
	}
	return packages
}

func (p *Plan) install(w io.Writer, packages []string, refresh bool) error {
	for _, command := range p.Manager.InstallCommands(packages, refresh) {
		_, _ = fmt.Fprintf(w, "==> %s\n", strings.Join(command, " "))
		if err := p.Manager.system.Run(command, "", p.Manager.env); err != nil {
			return err
		}
	}
	return nil
}

// unpackAndLink installs a language's release download whose check fails
// (fetched into the downloads cache, verified, unpacked beside its home and
// renamed into place) and makes its links.
func (p *Plan) unpackAndLink(w io.Writer, lp *LanguagePlan, did *installed) error {
	ctx := lp.Language.Ctx
	for _, action := range lp.Actions {
		if action.Kind != KindLink && action.Download == nil {
			continue
		}
		action.evaluate(ctx, p.Manager)
		if action.Done {
			continue
		}
		if action.Kind == KindLink {
			if err := makeLink(action.Link); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(w, "==> %s: linked %s\n", lp.Language.Name, action.Link[0])
			did.links = append(did.links, action.Link[0])
			continue
		}
		_, _ = fmt.Fprintf(w, "==> %s: downloading %s\n", lp.Language.Name, action.Download.describe())
		file, err := action.Download.fetchVerified(ctx.System, DownloadsDir(ctx.Env))
		if err != nil {
			return err
		}
		if err := unpackInto(file, action.Download.Format, action.Download.Strip, action.Home); err != nil {
			return fmt.Errorf("unpacking %s: %w", action.Download.URL, err)
		}
		did.homes = append(did.homes, action.Home)
		if !action.Check.Holds(ctx) {
			return fmt.Errorf("%s was unpacked into %s, but its check still fails (%s)", action.Label, action.Home, action.Check)
		}
	}
	return nil
}

// makeLink points a link at its target, replacing a link but never a file
// that isn't one (a command the user put there).
func makeLink(link [2]string) error {
	if info, err := os.Lstat(link[0]); err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("%s is there and isn't a link; not replacing it", link[0])
		}
		if err := os.Remove(link[0]); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(link[0]), 0o755); err != nil {
		return err
	}
	return os.Symlink(link[1], link[0])
}

// runSteps runs a language's steps whose checks fail, each checked again
// after it runs: its download fetched and verified first, and what an
// earlier run of it left in the directory it creates removed.
func (p *Plan) runSteps(w io.Writer, lp *LanguagePlan, did *installed) error {
	ctx := lp.Language.Ctx
	for _, action := range lp.Actions {
		if action.Kind != KindStep {
			continue
		}
		action.evaluate(ctx, p.Manager)
		if !action.Applies || action.Done {
			continue
		}
		vars := map[string]string{}
		for name, value := range ctx.Vars {
			vars[name] = value
		}
		if action.Fetch != nil {
			_, _ = fmt.Fprintf(w, "==> %s: downloading %s\n", lp.Language.Name, action.Fetch.describe())
			file, err := action.Fetch.fetchVerified(ctx.System, DownloadsDir(ctx.Env))
			if err != nil {
				return err
			}
			vars["file"] = file
		}
		if action.Creates != "" {
			if err := removeCreated(action.Creates, dataDir(ctx.Env)); err != nil {
				return err
			}
			did.paths = append(did.paths, action.Creates)
		}
		argv := fill(action.Run, vars)
		_, _ = fmt.Fprintf(w, "==> %s: %s\n", lp.Language.Name, strings.Join(argv, " "))
		if err := ctx.System.Run(argv, ctx.Root, ctx.Env); err != nil {
			return fmt.Errorf("%s: %w", strings.Join(argv, " "), err)
		}
		did.steps = append(did.steps, action.Label)
		if !action.Check.Holds(ctx) {
			return fmt.Errorf("%s ran, but its check still fails: %s", action.Label, action.Problem)
		}
	}
	return nil
}

// removeCreated removes a directory a step creates, only ever inside
// hecatron's data directory.
func removeCreated(path, data string) error {
	if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(data)+string(filepath.Separator)) {
		return fmt.Errorf("%s isn't inside %s; not removing it", path, data)
	}
	return os.RemoveAll(path)
}

// verify checks everything the language's install does again.
func (p *Plan) verify(lp *LanguagePlan) error {
	for _, action := range lp.Actions {
		action.evaluate(lp.Language.Ctx, p.Manager)
		if action.Applies && !action.Done {
			what := action.Kind + " " + action.Label
			if action.Kind == KindTool || action.Problem != "" && action.Kind != KindStep {
				what = action.Problem
			}
			return fmt.Errorf("still missing after the install: %s", what)
		}
	}
	return nil
}
