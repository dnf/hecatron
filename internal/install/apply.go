package install

import (
	"fmt"
	"io"
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
		var installed, ran []string
		for _, name := range p.missingPackages([]*LanguagePlan{lp}) {
			if installedNow[name] {
				installed = append(installed, name)
			}
		}
		if failed[lang] == nil {
			ran, failed[lang] = p.runSteps(w, lp)
		}
		if failed[lang] == nil {
			failed[lang] = p.verify(lp)
		}
		if err := record(receiptsDir, lang, p.Manager.Name, installed, ran); err != nil && failed[lang] == nil {
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
			if action.Kind == KindTool {
				what = action.Problem
			}
			return fmt.Errorf("still missing after the install: %s", what)
		}
	}
	return nil
}
