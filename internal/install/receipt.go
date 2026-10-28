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
	Homes    []string            `json:"homes,omitempty"`    // toolchain directories it unpacked
	Links    []string            `json:"links,omitempty"`    // links it made in LinksDir
	Updated  string              `json:"updated"`
}

// installed is what one install did, for the receipt.
type installed struct {
	packages, steps, homes, links []string
}

func (i installed) empty() bool {
	return len(i.packages)+len(i.steps)+len(i.homes)+len(i.links) == 0
}

// dataDir is hecatron's own data directory: $XDG_DATA_HOME/hecatron, else
// ~/.local/share/hecatron.
func dataDir(env []string) string {
	if data := getenv(env, "XDG_DATA_HOME"); data != "" {
		return filepath.Join(data, "hecatron")
	}
	return filepath.Join(getenv(env, "HOME"), ".local", "share", "hecatron")
}

// ReceiptsDir is where receipts live (receipts/ in the data directory).
func ReceiptsDir(env []string) string { return filepath.Join(dataDir(env), "receipts") }

// ToolchainsDir is where downloaded toolchains are unpacked, one
// <lang>/<version> directory each (ToolchainsHome, or under
// $XDG_DATA_HOME).
func ToolchainsDir(env []string) string { return filepath.Join(dataDir(env), "toolchains") }

// LinksDir holds links to each language's default version's commands,
// which ~/.hecatron/env puts on PATH for interactive shells.
func LinksDir(env []string) string { return filepath.Join(dataDir(env), "bin") }

// DownloadsDir caches downloads by their SHA-256: $XDG_CACHE_HOME/hecatron/
// downloads, else ~/.cache/hecatron/downloads.
func DownloadsDir(env []string) string {
	if cache := getenv(env, "XDG_CACHE_HOME"); cache != "" {
		return filepath.Join(cache, "hecatron", "downloads")
	}
	return filepath.Join(getenv(env, "HOME"), ".cache", "hecatron", "downloads")
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
func record(dir, lang, manager string, did installed) error {
	if did.empty() {
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
	receipt.Packages[manager] = union(receipt.Packages[manager], did.packages)
	if len(receipt.Packages[manager]) == 0 {
		delete(receipt.Packages, manager)
	}
	receipt.Steps = union(receipt.Steps, did.steps)
	receipt.Homes = union(receipt.Homes, did.homes)
	receipt.Links = union(receipt.Links, did.links)
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
