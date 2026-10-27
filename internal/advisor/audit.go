package advisor

import (
	"fmt"
	"strings"

	"github.com/dnf/hecatron/internal/project"
	"github.com/dnf/hecatron/internal/status"
)

// auditLink is hecatron's own: audit acceptances are hecatron's config.
const auditLink = "https://github.com/dnf/hecatron/blob/main/docs/agents.md#project-configuration"

// auditFailed reports a language whose last `hct audit` failed: known
// vulnerabilities not yet fixed or accepted. The advisor runs no auditor;
// it reads the run's log, as hct status does.
func (r *run) auditFailed(lang string) {
	if r.p.Resolve(lang, "audit.sh") == "" {
		return
	}
	last, ok := status.LastRun(r.p, "audit", lang, r.opts.Now)
	if !ok || last.Status != "failed" {
		return
	}
	r.add(lang, "", Finding{ID: idAuditFailed, Level: Recommended, Category: "security", Dir: ".",
		Message: fmt.Sprintf("Fix or accept the known vulnerabilities hct audit %s reported on %s (log %s)",
			lang, last.Finished.Format("2006-01-02"), project.PyRelpath(last.Log, r.p.Root)),
		Why:  "A dependency with a known vulnerability is the easiest way in; accepting one with a reason and a date keeps the decision reviewed.",
		Link: auditLink})
}

// auditExpired reports accepted audit findings (config.json
// audit.ignore) whose "until" date has passed.
func (r *run) auditExpired() {
	audit, _ := r.p.ConfigValue("audit")
	object, ok := audit.(*project.Object)
	if !ok {
		return
	}
	ignore, _ := object.Get("ignore")
	entries, _ := ignore.([]any)
	today := r.opts.Now.Format("2006-01-02")
	var expired []string
	for _, raw := range entries {
		entry, ok := raw.(*project.Object)
		if !ok {
			continue
		}
		id, _ := entry.Get("id")
		until, _ := entry.Get("until")
		idText, idOK := id.(string)
		untilText, untilOK := until.(string)
		if idOK && untilOK && len(untilText) == 10 && untilText < today {
			expired = append(expired, fmt.Sprintf("%s (until %s)", idText, untilText))
		}
	}
	if len(expired) == 0 {
		return
	}
	r.add("", "", Finding{ID: idAuditExpired, Level: Recommended, Category: "security", Dir: ".",
		Message: "Review the accepted vulnerabilities past their date in .hecatron/config.json audit.ignore: " + strings.Join(expired, ", "),
		Why:     "An acceptance's date is when it was to be looked at again; past it, the risk is accepted by no one.",
		Link:    auditLink})
}
