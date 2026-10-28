package install

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// Removal is what uninstalling a language does: remove the packages its
// receipt lists, except those another language still needs.
type Removal struct {
	Language string
	Receipt  *Receipt // nil: hecatron installed nothing for it from data
	Remove   []string
	Kept     []string // "cmake (also needed by c)"
}

// PlanUninstall works out a language's removal on manager from its
// receipt. A package stays when the definition keeps it, another
// language's receipt lists it, or another definition names it (others:
// every other language's install.json), and when it's no longer installed
// there is nothing to remove.
func PlanUninstall(lang string, def *Definition, manager *Manager, receiptsDir string, others map[string]*Definition) (*Removal, error) {
	removal := &Removal{Language: lang}
	receipt, err := ReadReceipt(receiptsDir, lang)
	if err != nil || receipt == nil {
		return removal, err
	}
	removal.Receipt = receipt
	receipts, err := ReadReceipts(receiptsDir, lang)
	if err != nil {
		return nil, err
	}
	installed := manager.Installed(receipt.Packages[manager.Name])
	for _, name := range receipt.Packages[manager.Name] {
		var users []string
		for other, r := range receipts {
			if contains(r.Packages[manager.Name], name) {
				users = append(users, other)
			}
		}
		for other, d := range others {
			if other != lang && !contains(users, other) && contains(d.PackageNames(manager.Name), name) {
				users = append(users, other)
			}
		}
		sort.Strings(users)
		switch {
		case !installed[name]:
		case contains(def.Uninstall.Keep, name):
			kept := name
			if def.Uninstall.Because != "" {
				kept += " (" + def.Uninstall.Because + ")"
			}
			removal.Kept = append(removal.Kept, kept)
		case len(users) > 0:
			removal.Kept = append(removal.Kept, name+" (also needed by "+strings.Join(users, ", ")+")")
		default:
			removal.Remove = append(removal.Remove, name)
		}
	}
	return removal, nil
}

// PackageNames are every package a definition names for manager.
func (d *Definition) PackageNames(manager string) []string {
	names := append([]string(nil), d.Toolchain.Packages[manager]...)
	for _, entry := range d.Packages {
		names = append(names, entry.Managers[manager]...)
	}
	for _, tool := range d.Tools {
		names = append(names, tool.Packages[manager]...)
	}
	return names
}

// Apply removes the packages and the receipt.
func (r *Removal) Apply(w io.Writer, manager *Manager, receiptsDir string) error {
	if r.Receipt == nil {
		_, _ = fmt.Fprintf(w, "==> %s: hecatron has no receipt for it, so it installed nothing it could remove (installed by install.sh, or before hecatron); HECATRON_INSTALLER=script hct uninstall %[1]s runs uninstall.sh\n", r.Language)
		return nil
	}
	for _, kept := range r.Kept {
		_, _ = fmt.Fprintf(w, "==> %s: keeping %s\n", r.Language, kept)
	}
	if len(r.Remove) > 0 {
		command := manager.RemoveCommand(r.Remove)
		_, _ = fmt.Fprintf(w, "==> %s\n", strings.Join(command, " "))
		if err := manager.system.Run(command, "", manager.env); err != nil {
			return err
		}
	}
	return removeReceipt(receiptsDir, r.Language)
}
