package install

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Platform is an OS ("linux", "macos", "windows") and CPU ("x86_64",
// "aarch64"), named as hecatron names them.
type Platform struct {
	OS   string
	Arch string
}

// Platforms is every platform hecatron releases for.
var Platforms = []Platform{
	{"linux", "x86_64"}, {"linux", "aarch64"},
	{"macos", "x86_64"}, {"macos", "aarch64"},
	{"windows", "x86_64"}, {"windows", "aarch64"},
}

// Package managers by OS, preferred first.
var managers = map[string][]string{
	"linux":   {"apt"},
	"macos":   {"brew"},
	"windows": {"winget", "scoop"},
}

// Step is one thing an install does.
type Step struct {
	Tool     string            // the tool this step installs; "" for the language
	Download string            // a URL to download
	Checksum string            // the SHA-256 (hex), or a URL that serves it
	Unpack   bool              // unpack the download (an archive), else run it
	Strip    int               //
	Run      []string          // a command; {file} is the download
	Env      map[string]string //
	Bin      string            // the directory, inside the install, to put on PATH
	Manager  string            // a package manager, installing Packages
	Packages []string          //
	Requires []string          // languages installed first
}

// Resolve says how to install version on platform. Resolvers that need the
// network (a release's checksum from an API) run only when online is set;
// otherwise their steps name the resolver instead of a URL.
func Resolve(spec *Spec, version string, platform Platform, online bool) ([]Step, error) {
	version = strings.TrimPrefix(version, "=")
	if override, ok := spec.Platforms[platform.OS+"-"+platform.Arch]; ok {
		return Resolve(&override, version, platform, online)
	}
	var steps []Step
	if len(spec.Requires) > 0 {
		steps = append(steps, Step{Requires: spec.Requires})
	}
	if spec.Via != nil {
		tool, ok := spec.Tools[spec.Via.Tool]
		if !ok {
			return nil, fmt.Errorf("via %s: no such tool", spec.Via.Tool)
		}
		toolSteps, err := Resolve(&tool, tool.Version, platform, online)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", spec.Via.Tool, err)
		}
		for _, step := range toolSteps {
			step.Tool = spec.Via.Tool
			steps = append(steps, step)
		}
		return append(steps, Step{Run: fill(spec.Via.Run, map[string]string{"version": version})}), nil
	}
	switch {
	case spec.Archive != nil:
		step, err := spec.Archive.resolve(version, platform, online)
		if err != nil {
			return nil, err
		}
		return append(steps, step), nil
	case spec.Vendor != nil:
		step, err := spec.Vendor.resolve(version, platform)
		if err != nil {
			return nil, err
		}
		return append(steps, step), nil
	case spec.Packages != nil:
		for _, manager := range managers[platform.OS] {
			if packages, ok := spec.Packages[manager]; ok {
				return append(steps, Step{Manager: manager, Packages: packages}), nil
			}
		}
		return nil, fmt.Errorf("no package for %s", platform.OS)
	}
	return nil, fmt.Errorf("nothing to install")
}

func (a *Archive) resolve(version string, platform Platform, online bool) (Step, error) {
	step := Step{Unpack: true, Strip: a.Strip, Bin: byOS(a.Bin, platform.OS)}
	if a.Resolver != "" {
		resolver, ok := resolvers[a.Resolver]
		if !ok {
			return step, fmt.Errorf("no resolver %q", a.Resolver)
		}
		if !online {
			step.Download = "(" + a.Resolver + ")"
			return step, nil
		}
		url, checksum, err := resolver(version, platform)
		step.Download, step.Checksum = url, checksum
		return step, err
	}
	osName, okOS := a.OS[platform.OS]
	arch, okArch := a.Arch[platform.Arch]
	if !okOS || !okArch {
		return step, fmt.Errorf("no archive for %s %s", platform.OS, platform.Arch)
	}
	values := map[string]string{"version": version, "os": osName, "arch": arch, "ext": byOS(a.Ext, platform.OS)}
	step.Download = fillOne(a.URL, values)
	step.Checksum = fillOne(a.Checksum, values)
	return step, nil
}

func (v *Vendor) resolve(version string, platform Platform) (Step, error) {
	family := "unix"
	if platform.OS == "windows" {
		family = "windows"
	}
	values := map[string]string{"version": version, "arch": v.Arch[platform.Arch]}
	url, ok := v.URL[family]
	if !ok {
		return Step{}, fmt.Errorf("no installer for %s", platform.OS)
	}
	return Step{Download: fillOne(url, values), Run: fill(v.Run[family], values), Env: v.Env, Bin: v.Bin}, nil
}

func byOS(values map[string]string, osName string) string {
	if value, ok := values[osName]; ok {
		return value
	}
	return values["default"]
}

// fillOne fills a template's {name}s; {file} and other names it has no value
// for stay, for the installer to fill.
func fillOne(template string, values map[string]string) string {
	for name, value := range values {
		template = strings.ReplaceAll(template, "{"+name+"}", value)
	}
	return template
}

func fill(templates []string, values map[string]string) []string {
	filled := make([]string, len(templates))
	for i, template := range templates {
		filled[i] = fillOne(template, values)
	}
	return filled
}

// A resolver finds an archive's URL and SHA-256 from its publisher's API.
type resolver func(version string, platform Platform) (url, checksum string, err error)

var resolvers = map[string]resolver{"adoptium": adoptium}

var client = &http.Client{Timeout: 60 * time.Second}

// adoptium finds a Temurin JDK release: the API names releases with their
// build number (jdk-25.0.5+8), which a version doesn't have.
func adoptium(version string, platform Platform) (string, string, error) {
	osName := map[string]string{"linux": "linux", "macos": "mac", "windows": "windows"}[platform.OS]
	arch := map[string]string{"x86_64": "x64", "aarch64": "aarch64"}[platform.Arch]
	feature := strings.SplitN(version, ".", 2)[0]
	for page := 0; page < 20; page++ {
		url := fmt.Sprintf("https://api.adoptium.net/v3/assets/feature_releases/%s/ga?os=%s&architecture=%s&image_type=jdk&jvm_impl=hotspot&page_size=20&page=%d",
			feature, osName, arch, page)
		response, err := client.Get(url)
		if err != nil {
			return "", "", err
		}
		body, err := io.ReadAll(response.Body)
		_ = response.Body.Close() // read in full
		if err != nil {
			return "", "", err
		}
		if response.StatusCode == http.StatusNotFound {
			break
		}
		var releases []struct {
			ReleaseName string `json:"release_name"`
			Binaries    []struct {
				Package struct {
					Link     string `json:"link"`
					Checksum string `json:"checksum"`
				} `json:"package"`
			} `json:"binaries"`
		}
		if err := json.Unmarshal(body, &releases); err != nil {
			return "", "", fmt.Errorf("adoptium: %w", err)
		}
		for _, release := range releases {
			name := strings.SplitN(strings.TrimPrefix(release.ReleaseName, "jdk-"), "+", 2)[0]
			if name == version && len(release.Binaries) > 0 {
				return release.Binaries[0].Package.Link, release.Binaries[0].Package.Checksum, nil
			}
		}
		if len(releases) < 20 {
			break
		}
	}
	return "", "", fmt.Errorf("adoptium: no JDK %s for %s %s", version, platform.OS, platform.Arch)
}
