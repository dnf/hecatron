package install

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// droppingSystem serves a body that breaks partway for the first drops
// fetches, as proxy.golang.org's "stream error" did.
type droppingSystem struct {
	fakeSystem
	body    []byte
	drops   int
	fetches int
}

type brokenReader struct{ rest io.Reader }

func (b *brokenReader) Read(p []byte) (int, error) {
	n, err := b.rest.Read(p)
	if err == io.EOF {
		return n, errors.New("stream error: stream ID 1; INTERNAL_ERROR")
	}
	return n, err
}

func (s *droppingSystem) Fetch(string) (io.ReadCloser, error) {
	s.fetches++
	if s.fetches <= s.drops {
		return io.NopCloser(&brokenReader{bytes.NewReader(s.body[:len(s.body)/2])}), nil
	}
	return io.NopCloser(bytes.NewReader(s.body)), nil
}

func TestADroppedTransferIsTriedAgain(t *testing.T) {
	retryDelay = time.Millisecond
	t.Cleanup(func() { retryDelay = 3 * time.Second })
	body := []byte(strings.Repeat("toolchain ", 1000))
	digest := sha256.Sum256(body)
	sum := hex.EncodeToString(digest[:])
	for _, c := range []struct {
		drops   int
		wantErr bool
	}{{0, false}, {2, false}, {downloadTries, true}} {
		system := &droppingSystem{body: body, drops: c.drops}
		cache := t.TempDir()
		d := &download{URL: "https://example.com/go.zip", SHA256: sum}
		path, err := d.fetchVerified(system, cache)
		if c.wantErr {
			if err == nil || !strings.Contains(err.Error(), "stream error") {
				t.Errorf("%d drops: err %v, want the stream error", c.drops, err)
			}
			if system.fetches != downloadTries {
				t.Errorf("%d drops: %d fetches, want %d", c.drops, system.fetches, downloadTries)
			}
		} else if err != nil {
			t.Errorf("%d drops: %v", c.drops, err)
		} else if got, _ := os.ReadFile(path); !bytes.Equal(got, body) {
			t.Errorf("%d drops: cached %d bytes, want %d", c.drops, len(got), len(body))
		}
		if leftovers, _ := filepath.Glob(filepath.Join(cache, ".partial-*")); len(leftovers) > 0 {
			t.Errorf("%d drops: partial files left: %v", c.drops, leftovers)
		}
	}
}
