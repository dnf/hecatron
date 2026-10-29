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
	if packages := p.missingPackages(p.Languages); len(packages) > 0 {
		var names []string
		for _, lp := range p.Languages {
			if len(p.missingPackages([]*LanguagePlan{lp})) > 0 {
				names = append(names, lp.Language.Name)
			}
		}
		_, _ = fmt.Fprintf(w, "==> Installing %s for %s\n", strings.Join(packages, " "), strings.Join(names, ", "))
		if err := p.install(w, packages); err != nil && len(names) > 1 {
			_, _ = fmt.Fprintf(w, "==> That failed (%v); installing each language's packages alone\n", err)
			for _, lp := range p.Languages {
				if alone := p.missingPackages([]*LanguagePlan{lp}); len(alone) > 0 {
					if err := p.install(w, alone); err != nil {
						failed[lp.Language.Name] = fmt.Errorf("installing %s: %w", strings.Join(alone, " "), err)
					}
				}
			}
		} else if err != nil {
			failed[names[0]] = fmt.Errorf("installing %s: %w", strings.Join(packages, " "), err)
		}
	}
	installedNow := p.Manager.Installed(p.missingPackages(p.Languages))
	for _, lp := range p.Languages {
		lang := lp.Language.Name
		var did installed
		for _, name := range p.missingPackages([]*LanguagePlan{lp}) {
			if installedNow[name] {
				did.packages = append(did.packages, name)
			}
		}
		if failed[lang] == nil {
			failed[lang] = p.unpackAndLink(w, lp, &did)
		}
		if failed[lang] == nil {
			did.steps, failed[lang] = p.runSteps(w, lp)
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

func (p *Plan) install(w io.Writer, packages []string) error {
	for _, command := range p.Manager.InstallCommands(packages) {
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
		if !action.Check.Holds(action.context(ctx)) {
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
// after it runs.
func (p *Plan) runSteps(w io.Writer, lp *LanguagePlan) ([]string, error) {
	ctx := lp.Language.Ctx
	var ran []string
	for _, action := range lp.Actions {
		if action.Kind != KindStep {
			continue
		}
		action.evaluate(ctx, p.Manager)
		if !action.Applies || action.Done {
			continue
		}
		_, _ = fmt.Fprintf(w, "==> %s: %s\n", lp.Language.Name, strings.Join(action.Run, " "))
		if err := ctx.System.Run(action.Run, ctx.Root, ctx.Env); err != nil {
			return ran, fmt.Errorf("%s: %w", strings.Join(action.Run, " "), err)
		}
		ran = append(ran, action.Label)
		if !action.Check.Holds(ctx) {
			return ran, fmt.Errorf("%s ran, but its check still fails: %s", action.Label, action.Problem)
		}
	}
	return ran, nil
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
