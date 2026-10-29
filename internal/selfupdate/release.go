// Package selfupdate is `hct self update` and the update notice: finding
// hecatron's releases on GitHub, verifying a release's binary against its
// SHA256SUMS, replacing an installation install.sh made, and the cached
// result of the last check for a newer release (docs/guide/installing-hecatron.md).
// Hecatron never updates itself unasked: it generates CI workflows and a
// Dockerfile, and a silent version change would change them.
package selfupdate

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// DefaultURL is the GitHub API's address for hecatron's repository;
// HECATRON_UPDATE_URL replaces it (for tests, a mirror or a fork).
const DefaultURL = "https://api.github.com/repos/dnf/hecatron"

// Client asks GitHub's releases API about hecatron's releases.
type Client struct {
	URL       string // the repository's API address (DefaultURL)
	Token     string // GITHUB_TOKEN, sent to the API when set
	UserAgent string
	HTTP      *http.Client
}

// NewClient is a client for the environment's URL and token.
func NewClient(getenv func(string) string, version string, httpClient *http.Client) *Client {
	url := strings.TrimRight(getenv("HECATRON_UPDATE_URL"), "/")
	if url == "" {
		url = DefaultURL
	}
	return &Client{URL: url, Token: getenv("GITHUB_TOKEN"), UserAgent: "hecatron/" + version, HTTP: httpClient}
}

// Release is a published release: its version (X.Y.Z), tag and assets
// (name: download URL).
type Release struct {
	Version string
	Tag     string
	Assets  map[string]string
}

// Latest is the newest published release.
func (c *Client) Latest(ctx context.Context) (*Release, error) {
	return c.release(ctx, c.URL+"/releases/latest")
}

// Get is the release of a version (X.Y.Z or vX.Y.Z).
func (c *Client) Get(ctx context.Context, version string) (*Release, error) {
	return c.release(ctx, c.URL+"/releases/tags/v"+strings.TrimPrefix(version, "v"))
}

func (c *Client) release(ctx context.Context, url string) (*Release, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", c.UserAgent)
	if c.Token != "" {
		request.Header.Set("Authorization", "Bearer "+c.Token)
	}
	response, err := c.HTTP.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	switch {
	case response.StatusCode == http.StatusNotFound:
		return nil, errNoRelease
	case response.StatusCode == http.StatusForbidden || response.StatusCode == http.StatusTooManyRequests:
		return nil, fmt.Errorf("GitHub refused the request (%s); setting GITHUB_TOKEN raises the rate limit", response.Status)
	case response.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("GitHub answered %s for %s", response.Status, url)
	}
	var data struct {
		TagName string `json:"tag_name"`
		Draft   bool   `json:"draft"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("reading the release from %s: %w", url, err)
	}
	version := strings.TrimPrefix(data.TagName, "v")
	if _, ok := parseVersion(version); !ok || data.Draft {
		return nil, fmt.Errorf("%s isn't a hecatron release (tag %q)", url, data.TagName)
	}
	release := &Release{Version: version, Tag: data.TagName, Assets: map[string]string{}}
	for _, asset := range data.Assets {
		release.Assets[asset.Name] = asset.URL
	}
	return release, nil
}

var errNoRelease = errors.New("no such release")

// IsNoRelease is whether err says the release doesn't exist.
func IsNoRelease(err error) bool { return errors.Is(err, errNoRelease) }

// AssetName is the binary's name in a release for this machine, as
// release.yml names it and install.sh downloads it.
func AssetName() string {
	name := "hecatron-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// The largest file a download accepts: well above a binary's size.
const maxDownload = 512 << 20

// download fetches url, failing on an error status, a body shorter than
// its Content-Length (a partial download) or one over the limit.
func (c *Client) download(ctx context.Context, url string, limit int64) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", c.UserAgent)
	response, err := c.HTTP.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading %s: %s", url, response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", url, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("downloading %s: larger than %d bytes", url, limit)
	}
	if response.ContentLength >= 0 && int64(len(data)) != response.ContentLength {
		return nil, fmt.Errorf("downloading %s: got %d of %d bytes", url, len(data), response.ContentLength)
	}
	return data, nil
}

// Binary downloads the release's binary for this machine and its
// SHA256SUMS, and returns the binary once its checksum matches.
func (c *Client) Binary(ctx context.Context, release *Release) ([]byte, error) {
	name := AssetName()
	binaryURL, sumsURL := release.Assets[name], release.Assets["SHA256SUMS"]
	if binaryURL == "" {
		return nil, fmt.Errorf("hecatron %s has no binary for %s-%s", release.Version, runtime.GOOS, runtime.GOARCH)
	}
	if sumsURL == "" {
		return nil, fmt.Errorf("hecatron %s has no SHA256SUMS to verify its binary with", release.Version)
	}
	sums, err := c.download(ctx, sumsURL, 1<<20)
	if err != nil {
		return nil, err
	}
	want, ok := checksum(sums, name)
	if !ok {
		return nil, fmt.Errorf("hecatron %s's SHA256SUMS has no line for %s", release.Version, name)
	}
	binary, err := c.download(ctx, binaryURL, maxDownload)
	if err != nil {
		return nil, err
	}
	got := sha256.Sum256(binary)
	if hex.EncodeToString(got[:]) != want {
		return nil, fmt.Errorf("%s's checksum doesn't match hecatron %s's SHA256SUMS; nothing was changed", name, release.Version)
	}
	return binary, nil
}

// checksum is a file's SHA-256 in sha256sum's output (`<hex>  <name>`,
// or `<hex> *<name>` for binary mode).
func checksum(sums []byte, name string) (string, bool) {
	scanner := bufio.NewScanner(bytes.NewReader(sums))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name && len(fields[0]) == 64 {
			return strings.ToLower(fields[0]), true
		}
	}
	return "", false
}

// parseVersion reads X.Y.Z.
func parseVersion(version string) ([3]int, bool) {
	var parts [3]int
	fields := strings.Split(version, ".")
	if len(fields) != 3 {
		return parts, false
	}
	for i, field := range fields {
		n, err := strconv.Atoi(field)
		if err != nil || n < 0 || field == "" || field[0] == '+' {
			return parts, false
		}
		parts[i] = n
	}
	return parts, true
}

// ValidVersion is whether version is X.Y.Z.
func ValidVersion(version string) bool {
	_, ok := parseVersion(version)
	return ok
}

// Newer is whether version a (X.Y.Z) is newer than b; false when either
// isn't a version.
func Newer(a, b string) bool {
	x, okA := parseVersion(a)
	y, okB := parseVersion(b)
	if !okA || !okB {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return x[i] > y[i]
		}
	}
	return false
}

// readFile is a file's contents without surrounding whitespace, "" when it
// can't be read.
func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
