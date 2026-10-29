package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/dnf/hecatron/internal/install"
	"github.com/dnf/hecatron/internal/reports"
	"github.com/dnf/hecatron/internal/runner"
	"github.com/dnf/hecatron/internal/secrets"
)

// hct audit's secret scan (docs/guide/audit.md, "Secrets"): gitleaks, a job
// beside the languages' in `hct audit` (as lint's shell is beside lint's),
// and the target `secrets` alone; `hct install secrets` installs it.

const secretsTarget = "secrets"

// secretsJob is audit's secrets job, ok false when there's none: without
// gitleaks, a project that sets audit.secrets shows it not installed, and
// any other leaves it out (as run.sh, which has no secret scan, does).
func (r *invocation) secretsJob() (job runner.Job, ok bool) {
	job = runner.Job{Label: secretsTarget, Kind: runner.NotInstalled}
	if _, found := lookPath("gitleaks", os.Getenv("PATH")); found {
		job.Kind, job.Exec = runner.Runnable, r.secretsExec()
		return job, true
	}
	return job, r.project.SecretsConfigured()
}

// secretsExec scans the project, prints the findings (never a secret) and
// records them for the SARIF report and job summary; it fails on any.
func (r *invocation) secretsExec() func(context.Context, io.Writer) runner.Outcome {
	return func(ctx context.Context, out io.Writer) runner.Outcome {
		program, found := lookPath("gitleaks", os.Getenv("PATH"))
		if !found {
			_, _ = fmt.Fprintln(out, "error:", secrets.ErrNotInstalled)
			return runner.Outcome{ExitCode: 1}
		}
		findings, err := secrets.Scan(ctx, secrets.Options{Program: program, Root: r.root, History: r.args.history,
			Ignore: r.project.SecretsIgnore(), Env: r.environment()})
		if err != nil {
			_, _ = fmt.Fprintln(out, "error: secrets:", err)
			return runner.Outcome{ExitCode: 1}
		}
		secrets.Write(out, findings, r.args.history)
		r.recordSecrets(findings)
		if len(findings) > 0 {
			return runner.Outcome{ExitCode: 1}
		}
		return runner.Outcome{ExitCode: 0}
	}
}

// recordSecrets saves the findings as audit findings, for the reports.
func (r *invocation) recordSecrets(findings []secrets.Finding) {
	s := r.reports
	if s == nil || s.scratch == "" || !s.auditFindings() {
		return
	}
	records := make([]reports.AuditFinding, 0, len(findings))
	for _, f := range findings {
		records = append(records, reports.AuditFinding{ID: f.Rule, Severity: "critical", Title: f.Description,
			Format: reports.SecretsFormat, File: filepath.Join(r.root, filepath.FromSlash(f.File)), Line: f.Line,
			Commit: f.Commit, Fingerprint: f.Fingerprint})
	}
	if err := reports.WriteAuditRecord(s.scratch, records); err != nil {
		fmt.Fprintln(os.Stderr, "hecatron: secrets:", err)
	}
}

// withoutSecretsTarget is install's arguments without `secrets` (unless a
// language goes by that name), and whether it was there.
func (c *cli) withoutSecretsTarget(names []string) ([]string, bool) {
	if lang, err := c.project.ResolveName(secretsTarget); err != nil || lang != "" {
		return names, false
	}
	var rest []string
	found := false
	for _, name := range names {
		if name == secretsTarget {
			found = true
		} else {
			rest = append(rest, name)
		}
	}
	return rest, found
}

// printSecretsPlan is install --plan's line for gitleaks.
func printSecretsPlan(env []string) {
	release, ok := secrets.Release(runtime.GOOS, runtime.GOARCH)
	target := filepath.Join(install.LinksDir(env), "gitleaks")
	switch {
	case !ok:
		fmt.Printf("secrets: no gitleaks release for %s/%s\n", runtime.GOOS, runtime.GOARCH)
	case secrets.InstalledVersion(target) == secrets.Version:
		fmt.Printf("secrets: gitleaks %s is installed (%s)\n", secrets.Version, target)
	default:
		fmt.Printf("secrets: gitleaks %s from %s (sha256 %s…) into %s\n", secrets.Version, release.URL, release.SHA256[:12], target)
	}
}

// installSecrets is `hct install secrets`, and the end of `hct install`:
// gitleaks' release, pinned and checked against its checksum, in
// hecatron's bin directory (no root). One that's there at the version is
// kept.
func installSecrets(env []string) int {
	release, ok := secrets.Release(runtime.GOOS, runtime.GOARCH)
	if !ok {
		fmt.Fprintf(os.Stderr, "WARNING: no gitleaks release for %s/%s; hct audit can't scan for secrets\n", runtime.GOOS, runtime.GOARCH)
		return 0
	}
	target := filepath.Join(install.LinksDir(env), "gitleaks")
	if secrets.InstalledVersion(target) == secrets.Version {
		return 0
	}
	fmt.Printf("==> Installing gitleaks %s (for hct audit's secret scan)...\n", secrets.Version)
	if _, err := install.InstallBinary(install.OS{Stdout: os.Stdout, Stderr: os.Stderr}, release, env); err != nil {
		fmt.Fprintln(os.Stderr, "error: gitleaks:", err)
		return 1
	}
	return 0
}
