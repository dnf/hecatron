package reports

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The audit hooks run their auditors and hand each report to `hecatron
// _audit report`, a process per component; when the run command wants
// findings for its reports it gives the hooks HECATRON_AUDIT_FINDINGS, a
// directory, and _audit writes each report's findings there. The run
// command reads them all when the run ends.

// WriteAuditRecord saves one report's findings in dir.
func WriteAuditRecord(dir string, findings []AuditFinding) error {
	if findings == nil {
		findings = []AuditFinding{}
	}
	data, err := json.Marshal(findings)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "audit-*.json")
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// ReadAuditRecords are every finding saved in dir; files it can't read are
// skipped.
func ReadAuditRecords(dir string) []AuditFinding {
	paths, _ := filepath.Glob(filepath.Join(dir, "audit-*.json"))
	sort.Strings(paths)
	var all []AuditFinding
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var findings []AuditFinding
		if json.Unmarshal(data, &findings) == nil {
			all = append(all, findings...)
		}
	}
	return sortedFindings(all)
}

// OSVSources are, from an OSV-Scanner report, the lock file or manifest
// each package was found in (absolute, or relative to where it ran).
func OSVSources(text string) map[string]string {
	var report struct {
		Results []struct {
			Source struct {
				Path string `json:"path"`
			} `json:"source"`
			Packages []struct {
				Package struct {
					Name string `json:"name"`
				} `json:"package"`
			} `json:"packages"`
		} `json:"results"`
	}
	sources := map[string]string{}
	if json.Unmarshal([]byte(text), &report) != nil {
		return sources
	}
	for _, result := range report.Results {
		for _, p := range result.Packages {
			if _, seen := sources[p.Package.Name]; !seen && result.Source.Path != "" {
				sources[p.Package.Name] = result.Source.Path
			}
		}
	}
	return sources
}

// PlaceAudit makes each finding's file relative to the workspace, with
// the line naming its package. A file outside the workspace (a lock file
// resolved in a scratch directory) is replaced by fallback's (the
// component's own manifest), or dropped.
func PlaceAudit(findings []AuditFinding, workspace string, fallback func(f AuditFinding) string) []AuditFinding {
	placed := make([]AuditFinding, 0, len(findings))
	for _, f := range findings {
		if f.File != "" && !inside(workspace, f.File) {
			f.File = ""
		}
		if f.File == "" && fallback != nil {
			f.File = fallback(f)
		}
		if f.File != "" && inside(workspace, f.File) {
			if !f.IsSecret() {
				f.Line = LineOf(f.File, f.Package)
			}
			rel, _ := filepath.Rel(workspace, f.File)
			f.File = filepath.ToSlash(rel)
		} else {
			f.File, f.Line = "", 0
		}
		placed = append(placed, f)
	}
	return placed
}

func inside(dir, path string) bool {
	if !filepath.IsAbs(path) {
		return false
	}
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
