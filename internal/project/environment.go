package project

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Installed is one installed version of a language's toolchain, as its
// installed.sh prints it.
type Installed struct {
	Version, Home string
}

// installedTimeout is how long installed.sh may take: a cold first run of
// several runtimes on macOS, which scans new binaries, can take seconds.
const installedTimeout = 30 * time.Second

// InstalledVersions are the versions a language's installed.sh prints
// ("version<TAB>home" lines), run with the language's static "path"
// entries first on PATH and stdin closed; none without one, or when it
// fails or takes too long. Read once per Project, as project.py does: the
// machine's toolchains don't change while a command runs.
func (p *Project) InstalledVersions(lang string) ([]Installed, error) {
	p.cacheMu.Lock()
	cached, ok := p.cache.installed[lang]
	p.cacheMu.Unlock()
	if ok {
		return cached, nil
	}
	found, err := p.runInstalled(lang)
	if err != nil {
		return nil, err
	}
	p.cacheMu.Lock()
	if p.cache.installed == nil {
		p.cache.installed = map[string][]Installed{}
	}
	p.cache.installed[lang] = found
	p.cacheMu.Unlock()
	return found, nil
}

func (p *Project) runInstalled(lang string) ([]Installed, error) {
	script := p.Resolve(lang, "installed.sh")
	if script == "" {
		return nil, nil
	}
	static, err := p.StaticPath(lang)
	if err != nil {
		return nil, err
	}
	path, _ := p.lookupEnv("PATH")
	path = strings.Join(append(static, path), ":")
	env := setEnviron(p.environ(), "PATH", path)
	bash, found := findExecutable("bash", path)
	if !found {
		return nil, unsupported("no bash on PATH to run %s", script)
	}
	ctx, cancel := context.WithTimeout(context.Background(), installedTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bash, script)
	cmd.Args[0] = "bash"
	cmd.Dir, cmd.Env = p.Root, env
	cmd.WaitDelay = time.Second
	out, err := cmd.Output()
	if err != nil {
		return nil, nil
	}
	if !utf8.Valid(out) {
		return nil, unsupported("%s printed text that isn't UTF-8", script)
	}
	home, haveHome := p.lookupEnv("HOME")
	var installed []Installed
	for _, line := range pySplitLines(pyText(out)) {
		version, home2, _ := strings.Cut(line, "\t")
		if version == "" || home2 == "" {
			continue
		}
		if err := checkDigits(version); err != nil {
			return nil, err
		}
		if !isVersion(version) {
			continue
		}
		expanded, err := pyExpandUser(pyStrip(home2), home, haveHome)
		if err != nil {
			return nil, err
		}
		installed = append(installed, Installed{pyStrip(version), expanded})
	}
	return installed, nil
}

// findExecutable is how Python's subprocess finds a program: on the PATH
// the child gets.
func findExecutable(name, path string) (string, bool) {
	for _, dir := range strings.Split(path, ":") {
		if dir == "" {
			dir = "."
		}
		candidate := pyJoin(dir, name)
		if isExecutableFile(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func setEnviron(env []string, name, value string) []string {
	result := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if !strings.HasPrefix(entry, name+"=") {
			result = append(result, entry)
		}
	}
	return append(result, name+"="+value)
}

// InstalledVersion is the newest installed version matching the component's
// spec (project.py's installed_version), and false for none. A component on
// the default takes any version on the default's release line; one whose
// version doesn't select what runs ("selects": false) runs what a
// component on the default would, else the newest installed.
func (p *Project) InstalledVersion(lang, component string) (Installed, bool, error) {
	resolved, err := p.ResolveVersion(lang, component)
	if err != nil {
		return Installed{}, false, err
	}
	if !resolved.Selects {
		defaultVersion, err := p.DefaultVersion(lang)
		if err != nil {
			return Installed{}, false, err
		}
		match, ok, err := p.installedMatch(lang, defaultVersion, "default")
		if err != nil || ok {
			return match, ok, err
		}
		return p.installedMatch(lang, "", "newest")
	}
	if resolved.Spec == "" {
		return Installed{}, false, nil
	}
	return p.installedMatch(lang, resolved.Spec, resolved.Source)
}

// installedMatch is the newest installed version a spec matches ("" matches
// any); a default matches its release line.
func (p *Project) installedMatch(lang, specText, source string) (Installed, bool, error) {
	if source == "default" {
		if specText == "" {
			return Installed{}, false, nil
		}
		var err error
		if specText, err = p.releaseLine(lang, specText); err != nil {
			return Installed{}, false, err
		}
	}
	var wanted *spec
	if specText != "" {
		parsed, err := parseSpec(specText)
		if err != nil {
			return Installed{}, false, err
		}
		wanted = &parsed
	}
	all, err := p.InstalledVersions(lang)
	if err != nil {
		return Installed{}, false, err
	}
	// A dict from version to home in project.py: a version listed twice keeps
	// its first place and its last home.
	var order []string
	homes := map[string]string{}
	for _, entry := range all {
		if wanted != nil && !wanted.matches(entry.Version) {
			continue
		}
		if _, seen := homes[entry.Version]; !seen {
			order = append(order, entry.Version)
		}
		homes[entry.Version] = entry.Home
	}
	best, ok := newest(order)
	if !ok {
		return Installed{}, false, nil
	}
	return Installed{best, homes[best]}, true, nil
}

// MissingVersion is a component whose version has no installed match.
type MissingVersion struct {
	Component, Spec, Source string
}

// MissingVersions are the components of a language the project uses whose
// version isn't installed; none without installed.sh, nothing to check
// against.
func (p *Project) MissingVersions(lang string) ([]MissingVersion, error) {
	if !contains(p.Languages, lang) || p.Resolve(lang, "installed.sh") == "" {
		return nil, nil
	}
	components, err := p.versionComponents(lang)
	if err != nil {
		return nil, err
	}
	var missing []MissingVersion
	for _, component := range components {
		resolved, err := p.ResolveVersion(lang, component)
		if err != nil {
			return nil, err
		}
		if resolved.Spec == "" {
			continue
		}
		if _, ok, err := p.installedMatch(lang, resolved.Spec, resolved.Source); err != nil {
			return nil, err
		} else if !ok {
			missing = append(missing, MissingVersion{component, resolved.Spec, resolved.Source})
		}
	}
	return missing, nil
}

// VersionNote is what the doctor says about a component whose version
// neither a pin nor the default chose.
type VersionNote struct {
	Component, Note string
}

// VersionNotes are project.py's version_notes: what each such component
// supports, or its commit's date.
func (p *Project) VersionNotes(lang string) ([]VersionNote, error) {
	components, err := p.versionComponents(lang)
	if err != nil {
		return nil, err
	}
	var notes []VersionNote
	for _, component := range components {
		resolved, err := p.ResolveVersion(lang, component)
		if err != nil {
			return nil, err
		}
		source, specText := resolved.Source, strings.TrimLeft(resolved.Spec, "=")
		switch {
		case !resolved.Selects:
			note, err := p.unselectedNote(lang, component, source, specText)
			if err != nil {
				return nil, err
			}
			notes = append(notes, VersionNote{component, note})
		case strings.HasPrefix(source, "supports:"):
			notes = append(notes, VersionNote{component, fmt.Sprintf("%s, the newest %s supports", specText, strings.TrimPrefix(source, "supports:"))})
		case strings.HasPrefix(source, "as-of:"):
			when, reason, _ := strings.Cut(strings.TrimPrefix(source, "as-of:"), " (")
			reason = reason[:max(len(reason)-1, 0)]
			because := ""
			switch {
			case strings.HasPrefix(reason, "supports:"):
				because = fmt.Sprintf(" (the line from %s)", strings.TrimPrefix(reason, "supports:"))
			case reason != "default" && reason != "project":
				because = fmt.Sprintf(" (%s)", reason)
			}
			notes = append(notes, VersionNote{component, fmt.Sprintf("%s, current on %s, this commit's date%s", specText, when, because)})
		}
	}
	return notes, nil
}

// unselectedNote says what a component whose version only says what to
// install gets installed, and what runs.
func (p *Project) unselectedNote(lang, component, source, specText string) (string, error) {
	where := source
	if i := strings.LastIndex(source, "supports:"); i >= 0 {
		where = source[i+len("supports:"):]
	}
	where = strings.TrimRight(where, ")")
	runs := "once installed"
	match, ok, err := p.InstalledVersion(lang, component)
	if err != nil {
		return "", err
	}
	if ok {
		runs = match.Version
	}
	pins, err := p.fileEntries(lang, "pinFiles")
	if err != nil {
		return "", err
	}
	var files []string
	for _, entry := range pins {
		files = append(files, "a "+entry.file)
	}
	note := fmt.Sprintf("%s installed for %s; runs the newest, %s", specText, where, runs)
	if len(files) > 0 {
		note += ", unless " + strings.Join(files, " or ") + " picks one"
	}
	return note, nil
}

// LanguageEnv is a language's environment for a component: directories to
// put first on PATH, in order, and variables, in the definition's order.
type LanguageEnv struct {
	Path []string
	Vars [][2]string
}

// LanguageEnv is project.py's language_env. "path" entries and "env"
// variables naming {home}, {version}, {major} or {minor} need an installed
// version matching the component's spec; without one they're left out (the
// doctor reports it).
func (p *Project) LanguageEnv(lang, component string) (LanguageEnv, error) {
	defs, err := p.Definitions()
	if err != nil {
		return LanguageEnv{}, err
	}
	definition := defs.Get(lang)
	patterns, err := p.definitionStrings(lang, "path")
	if err != nil {
		return LanguageEnv{}, err
	}
	match, ok, err := p.InstalledVersion(lang, component)
	if err != nil {
		return LanguageEnv{}, err
	}
	var fields map[string]string
	if ok {
		numbers := strings.Split(strings.TrimLeft(match.Version, "v"), ".")
		minor := "0"
		if len(numbers) > 1 {
			minor = numbers[1]
		}
		fields = map[string]string{"home": match.Home, "version": match.Version, "major": numbers[0], "minor": minor}
	}
	home, haveHome := p.lookupEnv("HOME")
	var env LanguageEnv
	for _, pattern := range patterns {
		if strings.Contains(pattern, "{") && fields == nil {
			continue
		}
		expanded, err := pyFormat(pattern, fields)
		if err != nil {
			return LanguageEnv{}, err
		}
		if expanded, err = pyExpandUser(expanded, home, haveHome); err != nil {
			return LanguageEnv{}, err
		}
		matches, err := pyGlob(expanded)
		if err != nil {
			return LanguageEnv{}, err
		}
		env.Path = append(env.Path, sortedDirs(matches)...)
	}
	variables, present := definition.Get("env")
	if fields != nil && present {
		object, isObject := variables.(*Object)
		if !isObject {
			return LanguageEnv{}, unsupported("%s's env is not an object", lang)
		}
		for _, name := range object.Keys {
			value, ok := object.Values[name].(string)
			if !ok {
				return LanguageEnv{}, unsupported("%s's env.%s is not a string", lang, name)
			}
			expanded, err := pyFormat(value, fields)
			if err != nil {
				return LanguageEnv{}, err
			}
			env.Vars = append(env.Vars, [2]string{name, expanded})
		}
	}
	if wanted, _ := definition.Get("sdkrootFromXcrun"); truthy(wanted) && hostOS == "darwin" {
		if _, set := p.lookupEnv("SDKROOT"); !set {
			if sdk := xcrunSDKPath(); sdk != "" {
				env.Vars = append(env.Vars, [2]string{"SDKROOT", sdk})
			}
		}
	}
	return env, nil
}

// hostOS is runtime.GOOS, which the parity tests set to take macOS's path
// elsewhere.
var hostOS = runtime.GOOS

// xcrunSDK caches xcrunSDKPath's answer for the process.
type xcrunCache struct {
	once sync.Once
	path string
}

var xcrunSDK = new(xcrunCache)

// xcrunSDKPath is project.py's xcrun_sdk_path: the selected Xcode's (or
// Command Line Tools') SDK, from `xcrun --show-sdk-path`, asked once per
// process, for a definition's "sdkrootFromXcrun" (GNAT's gcc links against
// the Command Line Tools' SDK, which can be newer than the selected Xcode's
// ld knows; clang asks xcrun). "" when xcrun fails.
func xcrunSDKPath() string {
	cache := xcrunSDK
	cache.once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), installedTimeout)
		defer cancel()
		out, err := exec.CommandContext(ctx, "xcrun", "--show-sdk-path").Output()
		if err == nil {
			cache.path = strings.TrimSpace(string(out))
		}
	})
	return cache.path
}

// pyFormat is Python's str.format(**fields) for the forms a definition
// writes: "{name}" for a field, "{{" and "}}" for braces. Anything else
// (a format spec, an attribute, an unknown name) is declined.
func pyFormat(text string, fields map[string]string) (string, error) {
	var out strings.Builder
	for i := 0; i < len(text); i++ {
		switch c := text[i]; {
		case strings.HasPrefix(text[i:], "{{"), strings.HasPrefix(text[i:], "}}"):
			out.WriteByte(c)
			i++
		case c == '{':
			end := strings.IndexByte(text[i:], '}')
			if end < 0 {
				return "", unsupported("format string %q", text)
			}
			value, ok := fields[text[i+1:i+end]]
			if !ok {
				return "", unsupported("format field in %q", text)
			}
			out.WriteString(value)
			i += end
		case c == '}':
			return "", unsupported("format string %q", text)
		default:
			out.WriteByte(c)
		}
	}
	return out.String(), nil
}

// Exports is the environment as shell exports, as project.py's env_exports
// writes them for `hecatron env LANG`.
func (e LanguageEnv) Exports() string {
	var out bytes.Buffer
	for _, variable := range e.Vars {
		fmt.Fprintf(&out, "export %s=%s\n", variable[0], shellQuote(variable[1]))
	}
	if len(e.Path) > 0 {
		fmt.Fprintf(&out, "export PATH=%s:\"${PATH}\"\n", shellQuote(strings.Join(e.Path, ":")))
	}
	return out.String()
}

// Apply is env with the environment applied, as a shell evaluating
// Exports would: the variables, then the directories before PATH.
func (e LanguageEnv) Apply(env []string) []string {
	env = append([]string(nil), env...)
	for _, variable := range e.Vars {
		env = setEnviron(env, variable[0], variable[1])
	}
	if len(e.Path) > 0 {
		path := ""
		for i := len(env) - 1; i >= 0; i-- {
			if value, ok := strings.CutPrefix(env[i], "PATH="); ok {
				path = value
				break
			}
		}
		env = setEnviron(env, "PATH", strings.Join(e.Path, ":")+":"+path)
	}
	return env
}
