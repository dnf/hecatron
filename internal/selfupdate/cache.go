package selfupdate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// CacheFile is where the last check's result is kept, in hecatron's
// installation directory (~/.hecatron, or HECATRON_INSTALL_DIR).
const CacheFile = ".update-check.json"

// CheckInterval is how often hecatron looks for a newer release, at most.
const CheckInterval = 24 * time.Hour

// Cache is the last check for a newer release: when it was made and the
// latest version it found ("" when none is known yet).
type Cache struct {
	Checked time.Time `json:"checked"`
	Latest  string    `json:"latest,omitempty"`
}

// ReadCache is home's cache, the zero Cache when there is none.
func ReadCache(home string) Cache {
	var cache Cache
	data, err := os.ReadFile(filepath.Join(home, CacheFile))
	if err == nil && json.Unmarshal(data, &cache) == nil && (cache.Latest == "" || ValidVersion(cache.Latest)) {
		return cache
	}
	return Cache{}
}

// WriteCache replaces home's cache, through a rename so a reader never
// sees half of it.
func WriteCache(home string, cache Cache) error {
	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(home, CacheFile+".*")
	if err != nil {
		return err
	}
	_, err = temp.Write(append(data, '\n'))
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(temp.Name(), filepath.Join(home, CacheFile))
	}
	if err != nil {
		_ = os.Remove(temp.Name())
	}
	return err
}

// Due is whether the cache is old enough for another check: never
// checked, checked a day or more ago, or (a clock set back) in the future.
func (c Cache) Due(now time.Time) bool {
	return c.Checked.IsZero() || now.Sub(c.Checked) >= CheckInterval || c.Checked.After(now.Add(time.Minute))
}

// Newer is the newer release the cache knows of than current, else "".
func (c Cache) Newer(current string) string {
	if Newer(c.Latest, current) {
		return c.Latest
	}
	return ""
}
