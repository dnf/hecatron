package main

import (
	"fmt"
	"strings"
)

// env is `hecatron env` without a language: shell code a project's own
// recipes eval to get HECATRON_HOME, HECATRON_ROOT and the helpers. Paths
// bash's printf %q would leave alone are printed here; any other is quoted
// by bash itself (envTarget), since %q's escaping is bash's own.
func (c *cli) env() (int, bool) {
	helpers := c.homePath("libexec/lib/helpers.sh")
	if len(c.args) > 0 || !quoteFree(c.home.Dir) || !quoteFree(c.root) || !quoteFree(helpers) {
		return 0, false
	}
	fmt.Printf("export HECATRON_HOME=%s\nexport HECATRON_ROOT=%s\nsource %s\n", c.home.Dir, c.root, helpers)
	return 0, true
}

// quoteFree is text printf %q prints unchanged.
func quoteFree(s string) bool {
	return s != "" && strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_./-") == "" &&
		!strings.HasPrefix(s, "-")
}
