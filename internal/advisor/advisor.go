package advisor

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dnf/hecatron/internal/project"
)

// Ids of the rules written in Go: per set ("<set>/toolchain-pinned"), and
// for the whole repository.
const (
	idPinned          = "toolchain-pinned"
	idEndOfLife       = "toolchain-end-of-life"
	idBehind          = "toolchain-behind"
	idLockFresh       = "lock-matches-manifest"
	idCICovers        = "ci-covers"
	idCIConfigured    = "ci/configured"
	idWorkflowCurrent = "ci/workflow-current"
	idAuditFailed     = "audit/failed"
	idAuditExpired    = "audit/acceptance-expired"
)

var builtinIDs = []string{idCIConfigured, idWorkflowCurrent, idAuditFailed, idAuditExpired}

// Options are a run's settings.
type Options struct {
	Languages []string  // only these (canonical names); none: all the project's
	Now       time.Time // what end of life and expiry are judged against
	Fix       bool      // apply the safe fixes
}

// Finding is one piece of advice.
type Finding struct {
	ID         string   `json:"id"`
	Level      string   `json:"level"`
	Category   string   `json:"category"`
	Languages  []string `json:"languages"`
	Components []string `json:"components"`
	Dir        string   `json:"dir"` // relative to the root; "." for the root
	Message    string   `json:"message"`
	Why        string   `json:"why"`
	Link       string   `json:"link"`
	Fix        *Fix     `json:"fix,omitempty"`
	Reason     string   `json:"reason,omitempty"` // why the project ignores it, for an ignored finding
}

// Fix is the safe change --fix makes: a new file.
type Fix struct {
	Create  string `json:"create"` // relative to the root
	Content string `json:"content"`
	Applied bool   `json:"applied"`
}

// Report is a run's result.
type Report struct {
	Root      string    `json:"root"`
	Config    string    `json:"config"` // "saved" or "detected"
	Languages []string  `json:"languages"`
	NoAdvice  []string  `json:"noAdvice"` // languages with no rules yet
	Findings  []Finding `json:"findings"`
	Ignored   []Finding `json:"ignored"`
	Fixed     []string  `json:"fixed"` // files --fix created, relative to the root
}

// Count is how many findings there are at a level.
func (r *Report) Count(level string) int {
	n := 0
	for _, f := range r.Findings {
		if f.Level == level {
			n++
		}
	}
	return n
}

// run is one advisor run's state.
type run struct {
	p        *project.Project
	lib      *Library
	files    *files
	opts     Options
	found    map[string]*Finding // by id and directory
	order    []string
	defNames map[string]string
}

// Advise reads the project and gives its findings, applying the fixes when
// asked to.
func Advise(p *project.Project, opts Options) (*Report, error) {
	languages := opts.Languages
	if len(languages) == 0 {
		languages = p.Languages
	}
	lib, err := Load(p, languages)
	if err != nil {
		return nil, err
	}
	r := &run{p: p, lib: lib, opts: opts, found: map[string]*Finding{}, files: newFiles(p.Root, gitTracked(p.Root)), defNames: map[string]string{}}
	report := &Report{Root: p.Root, Config: "saved", Languages: append([]string{}, languages...),
		NoAdvice: []string{}, Findings: []Finding{}, Ignored: []Finding{}, Fixed: []string{}}
	if p.Detected {
		report.Config = "detected"
	}
	for _, lang := range languages {
		if lib.Languages[lang] == nil {
			report.NoAdvice = append(report.NoAdvice, lang)
			continue
		}
		if err := r.language(lang); err != nil {
			return nil, err
		}
	}
	if len(opts.Languages) == 0 {
		if err := r.repository(); err != nil {
			return nil, err
		}
	}
	for _, lang := range languages {
		r.auditFailed(lang)
	}
	ignores := Ignores(p)
	for _, key := range r.order {
		f := *r.found[key]
		if reason, ok := ignores.match(f); ok {
			f.Reason = reason
			report.Ignored = append(report.Ignored, f)
			continue
		}
		report.Findings = append(report.Findings, f)
	}
	sortFindings(report.Findings)
	sortFindings(report.Ignored)
	if opts.Fix {
		fixed, err := applyFixes(p.Root, report.Findings)
		if err != nil {
			return nil, err
		}
		report.Fixed = fixed
	}
	return report, nil
}

// language evaluates every set a language has, in each of its components.
func (r *run) language(lang string) error {
	dirs, err := r.p.ImplDirs(lang)
	if err != nil {
		return err
	}
	var existing []project.ComponentDir
	for _, d := range dirs {
		if info, err := os.Stat(d.Dir); err == nil && info.IsDir() {
			existing = append(existing, d)
		}
	}
	for _, set := range r.lib.sets(lang) {
		for i := range set.Rules {
			rule := &set.Rules[i]
			if rule.Scope == "root" {
				if r.files.eval(rule.When, r.p.Root) && !r.files.eval(rule.Expect, r.p.Root) {
					r.add(lang, ".", rule.finding(set.name))
				}
				continue
			}
			for _, d := range existing {
				if r.files.eval(rule.When, d.Dir) && !r.files.eval(rule.Expect, d.Dir) {
					r.add(lang, d.Component, r.at(rule.finding(set.name), d.Dir))
				}
			}
		}
		for _, d := range existing {
			if err := r.toolchain(set, lang, d); err != nil {
				return err
			}
			r.lockFresh(set, lang, d)
		}
		if set.CI != nil {
			if err := r.ciCovers(set, lang, existing); err != nil {
				return err
			}
		}
	}
	return nil
}

func (rule *Rule) finding(namespace string) Finding {
	return Finding{ID: namespace + "/" + rule.ID, Level: rule.Level, Category: rule.Category,
		Message: rule.Message, Why: rule.Why, Link: rule.Link, Dir: "."}
}

// at places a finding in a directory.
func (r *run) at(f Finding, dir string) Finding {
	f.Dir = filepath.ToSlash(project.PyRelpath(dir, r.p.Root))
	return f
}

// add records a finding for a language's component; the same finding in
// the same directory from another language (JavaScript and TypeScript
// share Node.js's rules) or component is merged into one.
func (r *run) add(lang, component string, f Finding) {
	key := f.ID + "\x00" + f.Dir
	existing, ok := r.found[key]
	if !ok {
		f.Languages, f.Components = []string{}, []string{}
		existing = &f
		r.found[key] = existing
		r.order = append(r.order, key)
	}
	if lang != "" && !contains(existing.Languages, lang) {
		existing.Languages = append(existing.Languages, lang)
	}
	if component != "" && !contains(existing.Components, component) {
		existing.Components = append(existing.Components, component)
	}
}

// name is a language's display name from its definition.
func (r *run) name(lang string) string {
	if name, ok := r.defNames[lang]; ok {
		return name
	}
	name := lang
	if defs, err := r.p.Definitions(); err == nil {
		if value, _ := defs.Get(lang).Get("name"); value != nil {
			if s, ok := value.(string); ok {
				name = s
			}
		}
	}
	r.defNames[lang] = name
	return name
}

// gitTracked is the files git tracks under root, nil outside a git
// repository (or without git).
func gitTracked(root string) map[string]bool {
	cmd := exec.Command("git", "ls-files", "-z", "--full-name", "--", ".")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	top := exec.Command("git", "rev-parse", "--show-prefix")
	top.Dir = root
	prefixOut, err := top.Output()
	if err != nil {
		return nil
	}
	prefix := strings.TrimRight(string(prefixOut), "\n")
	var paths []string
	for _, path := range strings.Split(string(out), "\x00") {
		if path == "" {
			continue
		}
		if prefix != "" {
			var ok bool
			if path, ok = strings.CutPrefix(path, prefix); !ok {
				continue
			}
		}
		paths = append(paths, path)
	}
	return trackedSet(paths)
}

func levelRank(level string) int {
	if level == Recommended {
		return 0
	}
	return 1
}

func categoryRank(category string) int {
	for i, c := range Categories {
		if c == category {
			return i
		}
	}
	return len(Categories)
}

// sortFindings orders findings by directory (the root first), then level,
// category and id.
func sortFindings(findings []Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.Dir != b.Dir {
			if a.Dir == "." || b.Dir == "." {
				return a.Dir == "."
			}
			return a.Dir < b.Dir
		}
		if levelRank(a.Level) != levelRank(b.Level) {
			return levelRank(a.Level) < levelRank(b.Level)
		}
		if categoryRank(a.Category) != categoryRank(b.Category) {
			return categoryRank(a.Category) < categoryRank(b.Category)
		}
		return a.ID < b.ID
	})
}

// applyFixes creates each fixable finding's file, never overwriting one,
// and returns those it created.
func applyFixes(root string, findings []Finding) ([]string, error) {
	fixed := []string{}
	for i := range findings {
		fix := findings[i].Fix
		if fix == nil {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(fix.Create))
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return fixed, err
		}
		_, err = file.WriteString(fix.Content)
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return fixed, err
		}
		fix.Applied = true
		fixed = append(fixed, fix.Create)
	}
	return fixed, nil
}
