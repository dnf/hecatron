package install

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// download is one release download: where from, how it's verified, how it
// unpacks.
type download struct {
	URL     string
	SHA256  string         // inline, else
	Sidecar string         // a URL serving it, else
	Index   *ChecksumIndex // filled in
	Format  string
	Strip   int

	indexBody any // the index, once fetched
}

// resolveDownload fills an archive's templates for a version on platform.
func (a *Archive) resolveDownload(version string, platform Platform) (*download, error) {
	format, err := a.format(platform)
	if err != nil {
		return nil, err
	}
	values := map[string]string{"version": version, "os": a.OS[platform.OS], "arch": a.Arch[platform.Arch],
		"ext": byOS(a.Ext, platform.OS)}
	d := &download{URL: fillOne(a.URL, values), SHA256: a.SHA256[platform.Key()], Format: format, Strip: a.Strip}
	values["url"] = d.URL
	if d.SHA256 == "" && a.Checksum != "" {
		d.Sidecar = fillOne(a.Checksum, values)
	}
	if d.SHA256 == "" && d.Sidecar == "" && a.ChecksumIndex != nil {
		d.Index = &ChecksumIndex{URL: fillOne(a.ChecksumIndex.URL, values), Path: fill(a.ChecksumIndex.Path, values)}
		if len(a.ChecksumIndex.URLPath) > 0 {
			d.Index.URLPath = fill(a.ChecksumIndex.URLPath, values)
		}
	}
	return d, nil
}

// describe is the download for a plan.
func (d *download) describe() string {
	switch {
	case d.SHA256 != "":
		return d.URL + " (sha256 " + d.SHA256[:min(12, len(d.SHA256))] + "…)"
	case d.Sidecar != "":
		return d.URL + " (sha256 from " + d.Sidecar + ")"
	case len(d.Index.URLPath) > 0:
		return "the URL at " + strings.Join(d.Index.URLPath, " → ") + " in " + d.Index.URL + " (sha256 from it too)"
	}
	return d.URL + " (sha256 from " + d.Index.URL + ")"
}

// indexString is the string at path in the download's index, fetched once.
func (d *download) indexString(system System, path []string) (string, error) {
	if d.indexBody == nil {
		body, err := fetchText(system, d.Index.URL)
		if err != nil {
			return "", err
		}
		if err := json.Unmarshal([]byte(body), &d.indexBody); err != nil {
			return "", fmt.Errorf("%s: %w", d.Index.URL, err)
		}
	}
	value := d.indexBody
	for _, key := range path {
		object, ok := value.(map[string]any)
		if !ok {
			return "", fmt.Errorf("%s: no %s in it", d.Index.URL, strings.Join(path, " → "))
		}
		value = object[key]
	}
	text, _ := value.(string)
	return text, nil
}

// resolveURL replaces the download's URL with the one its index lists,
// when the archive says to read it there.
func (d *download) resolveURL(system System) error {
	if d.Index == nil || len(d.Index.URLPath) == 0 {
		return nil
	}
	url, err := d.indexString(system, d.Index.URLPath)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("%s: no https URL at %s", d.Index.URL, strings.Join(d.Index.URLPath, " → "))
	}
	d.URL = url
	return nil
}

var hexSHA256 = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

// checksum is the download's SHA-256: inline, or fetched from its sidecar
// (the first word: bare hex and sha256sum's format both) or its index.
func (d *download) checksum(system System) (string, error) {
	if d.SHA256 != "" {
		return strings.ToLower(d.SHA256), nil
	}
	if d.Sidecar != "" {
		body, err := fetchText(system, d.Sidecar)
		if err != nil {
			return "", err
		}
		fields := strings.Fields(body)
		if len(fields) == 0 || !hexSHA256.MatchString(fields[0]) {
			return "", fmt.Errorf("%s: no SHA-256 in it", d.Sidecar)
		}
		return strings.ToLower(fields[0]), nil
	}
	sum, err := d.indexString(system, d.Index.Path)
	if err != nil {
		return "", err
	}
	if !hexSHA256.MatchString(sum) {
		return "", fmt.Errorf("%s: no SHA-256 at %s", d.Index.URL, strings.Join(d.Index.Path, " → "))
	}
	return strings.ToLower(sum), nil
}

func fetchText(system System, url string) (string, error) {
	body, err := system.Fetch(url)
	if err != nil {
		return "", err
	}
	defer func() { _ = body.Close() }()
	data, err := io.ReadAll(io.LimitReader(body, 16<<20))
	return string(data), err
}

// fetchVerified is the download in the cache (cacheDir, by its SHA-256),
// fetched when it isn't there; a file that doesn't match is never kept.
func (d *download) fetchVerified(system System, cacheDir string) (string, error) {
	sum, err := d.checksum(system)
	if err != nil {
		return "", err
	}
	path := filepath.Join(cacheDir, sum)
	if fileSHA256(path) == sum {
		return path, nil
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}
	if err := d.resolveURL(system); err != nil {
		return "", err
	}
	body, err := system.Fetch(d.URL)
	if err != nil {
		return "", err
	}
	defer func() { _ = body.Close() }()
	partial, err := os.CreateTemp(cacheDir, ".partial-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(partial.Name()) }()
	hash := sha256.New()
	_, err = io.Copy(io.MultiWriter(partial, hash), body)
	if closeErr := partial.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", fmt.Errorf("%s: %w", d.URL, err)
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != sum {
		return "", fmt.Errorf("%s doesn't match its checksum: sha256 %s, want %s", d.URL, got, sum)
	}
	return path, os.Rename(partial.Name(), path)
}

func fileSHA256(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return ""
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// unpackInto unpacks an archive into home: into a new directory beside it
// first, then renamed into place, so an interrupted install never leaves a
// half-unpacked home (a broken one is replaced).
func unpackInto(file, format string, strip int, home string) error {
	if err := os.MkdirAll(filepath.Dir(home), 0o755); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(filepath.Dir(home), "."+filepath.Base(home)+".partial-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(staging) }()
	switch format {
	case "tar.gz":
		err = untarGz(file, strip, staging)
	case "zip":
		err = unzip(file, strip, staging)
	case "tar.xz":
		// Go reads no xz; every Unix tar does (GNU tar, bsdtar on macOS).
		out, runErr := exec.Command("tar", "-xJf", file, "-C", staging, "--strip-components", strconv.Itoa(strip)).CombinedOutput()
		if runErr != nil {
			err = fmt.Errorf("tar: %v: %s", runErr, strings.TrimSpace(string(out)))
		}
	default:
		err = fmt.Errorf("can't unpack %s", format)
	}
	if err != nil {
		return err
	}
	if _, statErr := os.Lstat(home); statErr == nil {
		old := staging + ".old"
		if err := os.Rename(home, old); err != nil {
			return err
		}
		defer func() { _ = os.RemoveAll(old) }()
	}
	return os.Rename(staging, home)
}

// stripped is an archive entry's path with its first strip components
// dropped, "" when nothing is left; an entry escaping dest is an error.
func stripped(name string, strip int) (string, error) {
	parts := strings.Split(strings.Trim(filepath.ToSlash(name), "/"), "/")
	var kept []string
	for _, part := range parts {
		if part != "" && part != "." {
			kept = append(kept, part)
		}
	}
	if len(kept) <= strip {
		return "", nil
	}
	relative := filepath.FromSlash(strings.Join(kept[strip:], "/"))
	if !filepath.IsLocal(relative) {
		return "", fmt.Errorf("archive entry %q leaves the directory", name)
	}
	return relative, nil
}

func untarGz(file string, strip int, dest string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	reader := tar.NewReader(gz)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		relative, err := stripped(header.Name, strip)
		if err != nil || relative == "" {
			if err != nil {
				return err
			}
			continue
		}
		target := filepath.Join(dest, relative)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(target, 0o755)
		case tar.TypeReg:
			err = writeFile(target, reader, os.FileMode(header.Mode)&0o777)
		case tar.TypeSymlink:
			if filepath.IsAbs(header.Linkname) || !filepath.IsLocal(filepath.Join(filepath.Dir(relative), header.Linkname)) {
				return fmt.Errorf("archive link %q leaves the directory", header.Name)
			}
			err = os.Symlink(header.Linkname, target)
		case tar.TypeLink:
			var source string
			if source, err = stripped(header.Linkname, strip); err == nil {
				err = os.Link(filepath.Join(dest, source), target)
			}
		}
		if err != nil {
			return err
		}
	}
}

func unzip(file string, strip int, dest string) error {
	reader, err := zip.OpenReader(file)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()
	for _, entry := range reader.File {
		relative, err := stripped(entry.Name, strip)
		if err != nil {
			return err
		}
		if relative == "" {
			continue
		}
		target := filepath.Join(dest, relative)
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		content, err := entry.Open()
		if err != nil {
			return err
		}
		mode := entry.Mode() & 0o777
		if mode == 0 {
			mode = 0o644
		}
		err = writeFile(target, content, mode)
		_ = content.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func writeFile(path string, content io.Reader, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode|0o200)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, content)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

// The download client: _common.sh's curl wrapper's limits. Five tries,
// 30 s to connect, and a transfer that stalls for a minute fails (and is
// tried again); a 4xx answer isn't retried.
var downloadClient = &http.Client{Transport: &http.Transport{
	Proxy:                 http.ProxyFromEnvironment,
	DialContext:           (&net.Dialer{Timeout: 30 * time.Second}).DialContext,
	TLSHandshakeTimeout:   30 * time.Second,
	ResponseHeaderTimeout: 60 * time.Second,
}}

const downloadTries = 5

// Fetch implements System: the URL's body, after up to five tries.
func (OS) Fetch(url string) (io.ReadCloser, error) {
	var lastErr error
	for try := 0; try < downloadTries; try++ {
		if try > 0 {
			time.Sleep(3 * time.Second)
		}
		response, err := downloadClient.Get(url)
		if err != nil {
			lastErr = err
			continue
		}
		if response.StatusCode == http.StatusOK {
			return &stallReader{body: response.Body}, nil
		}
		_ = response.Body.Close()
		lastErr = fmt.Errorf("%s: %s", url, response.Status)
		if response.StatusCode < 500 && response.StatusCode != http.StatusTooManyRequests {
			break
		}
	}
	return nil, lastErr
}

// stallReader fails a read that takes over a minute.
type stallReader struct{ body io.ReadCloser }

func (s *stallReader) Read(p []byte) (int, error) {
	type result struct {
		n   int
		err error
	}
	done := make(chan result, 1)
	go func() {
		n, err := s.body.Read(p)
		done <- result{n, err}
	}()
	select {
	case r := <-done:
		return r.n, r.err
	case <-time.After(time.Minute):
		_ = s.body.Close()
		return 0, errors.New("the download stalled for a minute")
	}
}

func (s *stallReader) Close() error { return s.body.Close() }
