package install

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Receipt records what hecatron installed for a language from its
// definition: only packages that weren't installed before, so uninstall
// never removes what was there first (python3, a system Bash).
type Receipt struct {
	Schema   int                 `json:"schema"`
	Language string              `json:"language"`
	Packages map[string][]string `json:"packages,omitempty"` // package manager → packages
	Steps    []string            `json:"steps,omitempty"`    // steps it ran, for the record
	Updated  string              `json:"updated"`
}

// ReceiptsDir is where receipts live: $XDG_DATA_HOME/hecatron/receipts,
// else ~/.local/share/hecatron/receipts.
func ReceiptsDir(env []string) string {
	if data := getenv(env, "XDG_DATA_HOME"); data != "" {
		return filepath.Join(data, "hecatron", "receipts")
	}
	return filepath.Join(getenv(env, "HOME"), ".local", "share", "hecatron", "receipts")
}

// ReadReceipt is a language's receipt, nil when there is none.
func ReadReceipt(dir, lang string) (*Receipt, error) {
	data, err := os.ReadFile(filepath.Join(dir, lang+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	receipt := &Receipt{}
	if err := json.Unmarshal(data, receipt); err != nil {
		return nil, err
	}
	return receipt, nil
}

// ReadReceipts are every language's receipt but except's.
func ReadReceipts(dir, except string) (map[string]*Receipt, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	receipts := map[string]*Receipt{}
	for _, entry := range entries {
		lang, ok := strings.CutSuffix(entry.Name(), ".json")
		if !ok || lang == except {
			continue
		}
		receipt, err := ReadReceipt(dir, lang)
		if err != nil {
			return nil, err
		}
		receipts[lang] = receipt
	}
	return receipts, nil
}

// record adds what an install did to the language's receipt.
func record(dir, lang, manager string, packages, steps []string) error {
	if len(packages) == 0 && len(steps) == 0 {
		return nil
	}
	receipt, err := ReadReceipt(dir, lang)
	if err != nil {
		return err
	}
	if receipt == nil {
		receipt = &Receipt{Schema: 1, Language: lang}
	}
	if receipt.Packages == nil {
		receipt.Packages = map[string][]string{}
	}
	receipt.Packages[manager] = union(receipt.Packages[manager], packages)
	if len(receipt.Packages[manager]) == 0 {
		delete(receipt.Packages, manager)
	}
	receipt.Steps = union(receipt.Steps, steps)
	receipt.Updated = time.Now().UTC().Format(time.RFC3339)
	return writeReceipt(dir, receipt)
}

func writeReceipt(dir string, receipt *Receipt) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, receipt.Language+".json")
	if err := os.WriteFile(path+".new", append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(path+".new", path)
}

func union(list, more []string) []string {
	result := append([]string(nil), list...)
	for _, item := range more {
		if !contains(result, item) {
			result = append(result, item)
		}
	}
	sort.Strings(result)
	return result
}

func removeReceipt(dir, lang string) error {
	err := os.Remove(filepath.Join(dir, lang+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
