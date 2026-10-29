package install

import (
	"fmt"
	"os"
	"path/filepath"
)

// ReleaseBinary is one command from a release download: its URL, its
// SHA-256 (required), and the archive's format ("tar.gz", "zip", or ""
// for the bare binary) and path to the command inside it.
type ReleaseBinary struct {
	Name, URL, SHA256, Format, Member string
}

// InstallBinary puts a release's command in hecatron's bin directory
// (LinksDir; no root), as _common.sh's release_binary_install does: the
// download checked against its checksum (and cached by it), unpacked
// beside the bin directory, then renamed into place. It returns where the
// command went.
func InstallBinary(system System, b ReleaseBinary, env []string) (string, error) {
	if !hexSHA256.MatchString(b.SHA256) {
		return "", fmt.Errorf("%s: no SHA-256 to check the download against", b.Name)
	}
	d := &download{URL: b.URL, SHA256: b.SHA256, Format: b.Format}
	file, err := d.fetchVerified(system, DownloadsDir(env))
	if err != nil {
		return "", err
	}
	bin := LinksDir(env)
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return "", err
	}
	staging, err := os.MkdirTemp(bin, "."+b.Name+".partial-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(staging) }()
	member := b.Member
	switch b.Format {
	case "tar.gz":
		err = untarGz(file, 0, staging)
	case "zip":
		err = unzip(file, 0, staging)
	case "":
		member = b.Name
		err = copyFile(file, filepath.Join(staging, member))
	default:
		err = fmt.Errorf("can't unpack %s", b.Format)
	}
	if err != nil {
		return "", err
	}
	unpacked := filepath.Join(staging, filepath.FromSlash(member))
	if info, statErr := os.Stat(unpacked); statErr != nil || info.IsDir() {
		return "", fmt.Errorf("%s: no %s in %s", b.Name, member, b.URL)
	}
	if err := os.Chmod(unpacked, 0o755); err != nil {
		return "", err
	}
	target := filepath.Join(bin, b.Name)
	return target, os.Rename(unpacked, target)
}

func copyFile(from, to string) error {
	f, err := os.Open(from)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return writeFile(to, f, 0o755)
}
