package install

import (
	"bufio"
	"bytes"
	"crypto/sha1" //nolint:gosec // OpenPGP v4 fingerprints are SHA-1 (RFC 4880 §12.2)
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// AptSource is a signed apt repository a language's packages come from on
// one distribution, whose own packages are too old (SWI-Prolog's PPA on
// Ubuntu): its signing key, fetched and checked against its fingerprint,
// goes in /etc/apt/keyrings/hecatron-<name>.asc and the repository in
// /etc/apt/sources.list.d/hecatron-<name>.sources, signed by that key
// alone. Other distributions keep their own packages.
type AptSource struct {
	Name   string `json:"name"`
	Distro string `json:"distro"` // /etc/os-release's ID ("ubuntu")
	URI    string `json:"uri"`
	// Suite is the distribution's release in the repository; {codename}
	// is /etc/os-release's VERSION_CODENAME (the default).
	Suite       string   `json:"suite,omitempty"`
	Components  []string `json:"components"`
	Key         string   `json:"key"`         // the armored public key's URL
	Fingerprint string   `json:"fingerprint"` // its primary key's
	// Packages are those the repository provides newer: when the source
	// is added, ones apt installed from the distribution are upgraded.
	Packages []string `json:"packages,omitempty"`
	// Replaces are the distribution's packages whose files the
	// repository's packages take over without declaring it (Ubuntu's
	// swi-prolog-core): when the source is added, those installed are
	// removed before its packages are installed, or dpkg refuses to
	// overwrite their files.
	Replaces []string `json:"replaces,omitempty"`
}

var (
	sourceName  = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*$`)
	fingerprint = regexp.MustCompile(`^([0-9A-F]{40}|[0-9A-F]{64})$`)
)

func (s *AptSource) validate() error {
	switch {
	case !sourceName.MatchString(s.Name):
		return fmt.Errorf("name %q isn't lowercase letters, digits, dots and dashes", s.Name)
	case s.Distro == "":
		return errors.New(`no "distro"`)
	case !strings.HasPrefix(s.URI, "https://") || !strings.HasPrefix(s.Key, "https://"):
		return errors.New(`"uri" and "key" must be https URLs`)
	case len(s.Components) == 0:
		return errors.New(`no "components"`)
	case !fingerprint.MatchString(s.Fingerprint):
		return errors.New(`"fingerprint" must be the key's 40 (or 64) uppercase hex digits`)
	}
	return nil
}

// aptDir is apt's configuration directory; tests replace it, and
// osReleaseFile.
var (
	aptDir        = "/etc/apt"
	osReleaseFile = "/etc/os-release"
)

func (s *AptSource) keyring() string {
	return filepath.Join(aptDir, "keyrings", "hecatron-"+s.Name+".asc")
}

func (s *AptSource) sourcesFile() string {
	return filepath.Join(aptDir, "sources.list.d", "hecatron-"+s.Name+".sources")
}

// suite is the repository's suite for this machine's release.
func (s *AptSource) suite(release map[string]string) string {
	return strings.ReplaceAll(s.suiteTemplate(), "{codename}", release["VERSION_CODENAME"])
}

func (s *AptSource) suiteTemplate() string {
	if s.Suite == "" {
		return "{codename}"
	}
	return s.Suite
}

// sources is the .sources file (deb822), the same as install.sh writes.
func (s *AptSource) sources(release map[string]string) string {
	return fmt.Sprintf("Types: deb\nURIs: %s\nSuites: %s\nComponents: %s\nSigned-By: %s\n",
		s.URI, s.suite(release), strings.Join(s.Components, " "), s.keyring())
}

// isAptSourceFile says whether a path is one AptSource writes, the only
// files under /etc that uninstall removes.
func isAptSourceFile(path string) bool {
	dir, base := filepath.Dir(path), filepath.Base(path)
	return strings.HasPrefix(base, "hecatron-") &&
		(dir == filepath.Join(aptDir, "keyrings") && strings.HasSuffix(base, ".asc") ||
			dir == filepath.Join(aptDir, "sources.list.d") && strings.HasSuffix(base, ".sources"))
}

// osRelease is /etc/os-release's variables; empty when it can't be read.
func osRelease() map[string]string {
	values := map[string]string{}
	data, err := os.ReadFile(osReleaseFile)
	if err != nil {
		return values
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		key, value, ok := strings.Cut(strings.TrimSpace(scanner.Text()), "=")
		if ok && !strings.HasPrefix(key, "#") {
			values[key] = strings.Trim(value, `"'`)
		}
	}
	return values
}

// sourceAction is an apt source's part of an install.
type sourceAction struct {
	source  *AptSource
	release map[string]string
	// unavailable: the repository has no suite for this release (a new
	// Ubuntu the PPA hasn't built for yet), so the distribution's
	// packages are used.
	unavailable bool
}

func (s *sourceAction) describe() string {
	return s.source.Name + " (" + s.source.URI + " " + s.source.suite(s.release) + ")"
}

// added says whether the source's key and sources file are in place, the
// sources file as this release needs it.
func (s *sourceAction) added() bool {
	if _, err := os.Stat(s.source.keyring()); err != nil {
		return false
	}
	data, err := os.ReadFile(s.source.sourcesFile())
	return err == nil && string(data) == s.source.sources(s.release)
}

// commands are what adding the source runs, as text (`hct install --plan`).
func (s *sourceAction) commands(manager *Manager) []string {
	install := strings.Join(manager.sudo("install", "-D", "-m", "644"), " ")
	return []string{
		fmt.Sprintf("%s %s %s   (key %s, fingerprint %s)", install, "/dev/stdin", s.source.keyring(), s.source.Key, s.source.Fingerprint),
		fmt.Sprintf("%s %s %s   (%s)", install, "/dev/stdin", s.source.sourcesFile(), s.describe()),
	}
}

// add fetches the key, checks its fingerprint and writes the keyring and
// the sources file, returning the files written. A repository without this
// release's suite is left out (unavailable), not a failure.
func (s *sourceAction) add(w io.Writer, lang string, manager *Manager) ([]string, error) {
	system := manager.system
	release, err := system.Fetch(strings.TrimSuffix(s.source.URI, "/") + "/dists/" + s.source.suite(s.release) + "/InRelease")
	if err != nil {
		_, _ = fmt.Fprintf(w, "==> %s: %s has no %s (%v); using the distribution's packages\n", lang, s.source.URI,
			s.source.suite(s.release), err)
		s.unavailable = true
		return nil, nil
	}
	_ = release.Close()
	body, err := system.Fetch(s.source.Key)
	if err != nil {
		return nil, fmt.Errorf("fetching the key of apt source %s: %w", s.source.Name, err)
	}
	key, err := io.ReadAll(io.LimitReader(body, 1<<20))
	_ = body.Close()
	if err != nil {
		return nil, fmt.Errorf("fetching the key of apt source %s: %w", s.source.Name, err)
	}
	got, err := keyFingerprint(key)
	if err != nil {
		return nil, fmt.Errorf("the key of apt source %s (%s): %w", s.source.Name, s.source.Key, err)
	}
	if got != s.source.Fingerprint {
		return nil, fmt.Errorf("the key of apt source %s (%s) is %s, want %s", s.source.Name, s.source.Key, got, s.source.Fingerprint)
	}
	_, _ = fmt.Fprintf(w, "==> %s: adding apt source %s\n", lang, s.describe())
	var written []string
	for _, file := range []struct{ path, content string }{
		{s.source.keyring(), string(key)},
		{s.source.sourcesFile(), s.source.sources(s.release)},
	} {
		if err := installFile(manager, file.path, file.content); err != nil {
			return written, err
		}
		written = append(written, file.path)
	}
	return written, nil
}

// installFile writes a file root owns, through a temporary file and
// `install` (with sudo unless running as root).
func installFile(manager *Manager, path, content string) error {
	tmp, err := os.CreateTemp("", "hecatron-apt-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	_, err = tmp.WriteString(content)
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	command := manager.sudo("install", "-D", "-m", "644", tmp.Name(), path)
	if err := manager.system.Run(command, "", manager.env); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// keyFingerprint is the fingerprint of the one primary key in an armored
// (or binary) OpenPGP public key block, as gpg prints it: uppercase hex.
// A block holding more than one primary key is refused, since apt would
// trust them all.
func keyFingerprint(block []byte) (string, error) {
	data, err := dearmor(block)
	if err != nil {
		return "", err
	}
	found := ""
	for len(data) > 0 {
		tag, body, rest, err := nextPacket(data)
		if err != nil {
			return "", err
		}
		data = rest
		if tag != 6 { // a public key; subkeys (14), user IDs and signatures are left as they are
			continue
		}
		if found != "" {
			return "", errors.New("it holds more than one key")
		}
		if found, err = v4v6Fingerprint(body); err != nil {
			return "", err
		}
	}
	if found == "" {
		return "", errors.New("it holds no public key")
	}
	return found, nil
}

func v4v6Fingerprint(body []byte) (string, error) {
	if len(body) == 0 {
		return "", errors.New("an empty key packet")
	}
	switch body[0] {
	case 4:
		hash := sha1.New() //nolint:gosec // the v4 fingerprint's definition
		hash.Write([]byte{0x99, byte(len(body) >> 8), byte(len(body))})
		hash.Write(body)
		return strings.ToUpper(hex.EncodeToString(hash.Sum(nil))), nil
	case 6:
		hash := sha256.New()
		var length [4]byte
		binary.BigEndian.PutUint32(length[:], uint32(len(body)))
		hash.Write(append([]byte{0x9b}, length[:]...))
		hash.Write(body)
		return strings.ToUpper(hex.EncodeToString(hash.Sum(nil))), nil
	}
	return "", fmt.Errorf("a version %d key, which hecatron doesn't read", body[0])
}

// nextPacket splits the first OpenPGP packet (RFC 4880 §4.2) off data.
func nextPacket(data []byte) (tag int, body, rest []byte, err error) {
	truncated := errors.New("a truncated packet")
	if len(data) < 2 || data[0]&0x80 == 0 {
		return 0, nil, nil, errors.New("not OpenPGP data")
	}
	var length, header int
	if data[0]&0x40 != 0 { // new format
		tag = int(data[0] & 0x3f)
		switch first := int(data[1]); {
		case first < 192:
			length, header = first, 2
		case first < 224:
			if len(data) < 3 {
				return 0, nil, nil, truncated
			}
			length, header = (first-192)<<8+int(data[2])+192, 3
		case first == 255:
			if len(data) < 6 {
				return 0, nil, nil, truncated
			}
			length, header = int(binary.BigEndian.Uint32(data[2:6])), 6
		default:
			return 0, nil, nil, errors.New("a partial-length packet, which a key never has")
		}
	} else { // old format
		tag = int(data[0]>>2) & 0x0f
		switch data[0] & 3 {
		case 0:
			length, header = int(data[1]), 2
		case 1:
			if len(data) < 3 {
				return 0, nil, nil, truncated
			}
			length, header = int(binary.BigEndian.Uint16(data[1:3])), 3
		case 2:
			if len(data) < 5 {
				return 0, nil, nil, truncated
			}
			length, header = int(binary.BigEndian.Uint32(data[1:5])), 5
		default:
			return 0, nil, nil, errors.New("a packet of indeterminate length")
		}
	}
	if length < 0 || header+length > len(data) {
		return 0, nil, nil, truncated
	}
	return tag, data[header : header+length], data[header+length:], nil
}

// dearmor is an ASCII-armored block's data (binary data as it is): the
// base64 between the headers' blank line and the checksum or end line.
func dearmor(block []byte) ([]byte, error) {
	text := string(block)
	begin := strings.Index(text, "-----BEGIN PGP PUBLIC KEY BLOCK-----")
	if begin < 0 {
		if len(block) > 0 && block[0]&0x80 != 0 {
			return block, nil
		}
		return nil, errors.New("not an OpenPGP public key")
	}
	lines := strings.Split(strings.ReplaceAll(text[begin:], "\r\n", "\n"), "\n")[1:]
	for len(lines) > 0 && strings.Contains(lines[0], ":") { // armor headers (Comment:, Version:)
		lines = lines[1:]
	}
	var encoded strings.Builder
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "=") || strings.HasPrefix(line, "-----END") {
			break
		}
		encoded.WriteString(line)
	}
	data, err := base64.StdEncoding.DecodeString(encoded.String())
	if err != nil {
		return nil, fmt.Errorf("its armor isn't base64: %w", err)
	}
	return data, nil
}
