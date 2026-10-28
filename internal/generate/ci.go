package generate

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/dnf/hecatron/internal/install"
	"github.com/dnf/hecatron/internal/project"
)

// The generated CI beyond each language's GitHub workflow template, which
// only the Go core writes (docs/design.md §11, §20): a GitLab pipeline with
// one job per language, and caching of toolchains and dependencies, added
// to the GitHub workflows and the GitLab jobs when config.json says
// "ci": {"cache": true}.

// GitLabPath is where the GitLab pipeline goes, relative to the root.
const GitLabPath = ".gitlab-ci.yml"

// gitlabKeyFiles is the most files GitLab keys one cache on
// (cache:key:files).
const gitlabKeyFiles = 10

// cacheRoot is where a GitLab job keeps what it caches: GitLab caches
// only paths inside the project's directory, so each home directory a
// definition caches is linked to one under it.
const cacheRoot = ".hecatron-cache"

// Cache kinds, in the order a job restores them.
const (
	toolchainCache  = "toolchain"
	dependencyCache = "dependencies"
)

// languageCache is what CI caches for one language, worked out for the
// project: each cache's directories and the files it's keyed on, relative
// to the root.
type languageCache struct {
	lang   string
	caches []cacheEntry
}

type cacheEntry struct {
	kind     string
	dirs     []string // ~/... or relative to the root
	keyFiles []string // relative to the root
	// data is what the executor would install (install.CacheKey), for a
	// toolchain hecatron installs from its install.json; "" otherwise.
	data string
}

// projectCache works out a language's caches: the toolchain keyed on the
// project's config, the language's own definition in the project and its
// pin and support files; the dependencies on its lock files. Key files are
// matched anywhere in the repository (**/), so adding a component doesn't
// change the generated files.
func projectCache(p *project.Project, lang string) (languageCache, error) {
	spec, err := p.Cache(lang)
	if err != nil {
		return languageCache{}, err
	}
	result := languageCache{lang: lang}
	engine, data, err := engineToolchain(p, lang)
	if err != nil {
		return languageCache{}, err
	}
	if engine != "" {
		spec.Toolchain = []string{engine}
	}
	if spec.Empty() {
		return result, nil
	}
	dirs, err := p.ImplDirs(lang)
	if err != nil {
		return languageCache{}, err
	}
	anywhere := func(names []string) []string {
		files := make([]string, len(names))
		for i, name := range names {
			files[i] = "**/" + name
		}
		return files
	}
	cacheDirs := func(listed []string) []string {
		var result []string
		for _, dir := range listed {
			if strings.HasPrefix(dir, "~/") {
				result = append(result, dir)
				continue
			}
			for _, component := range dirs {
				if relative := path.Join(project.PyRelpath(component.Dir, p.Root), dir); !contains(result, relative) {
					result = append(result, relative)
				}
			}
		}
		return result
	}
	if len(spec.Toolchain) > 0 {
		keys := []string{".hecatron/config.json", ".hecatron/languages/" + lang + "/config.json"}
		if engine != "" {
			keys = append(keys, ".hecatron/languages/"+lang+"/"+install.FileName)
		}
		result.caches = append(result.caches, cacheEntry{toolchainCache, cacheDirs(spec.Toolchain),
			append(keys, anywhere(spec.KeyFiles)...), data})
	}
	if deps := cacheDirs(spec.Dependencies); len(deps) > 0 {
		result.caches = append(result.caches, cacheEntry{dependencyCache, deps, anywhere(spec.LockFiles), ""})
	}
	return result, nil
}

// engineInstalls says whether the executor carries out a definition on a
// Linux runner (apt); tests replace it.
var engineInstalls = func(def *install.Definition) bool { return def.Unsupported("apt") == "" }

// engineToolchain is the directory CI caches for a language whose
// toolchain hecatron's executor installs from its install.json on a Linux
// runner into its own toolchains directory (no root, so a restore works),
// and the key of what it would install there; "" for any other language,
// whose definition's "cache" applies. A restored toolchain passes the
// executor's checks, so the install does nothing; a partial one fails
// them and is repaired.
func engineToolchain(p *project.Project, lang string) (dir, data string, err error) {
	path := p.Resolve(lang, install.FileName)
	if path == "" {
		return "", "", nil
	}
	def, err := install.LoadFile(path)
	if err != nil {
		return "", "", &project.Error{Message: err.Error()}
	}
	if !engineInstalls(def) || !def.InstallsIntoToolchainsHome() {
		return "", "", nil
	}
	version, err := p.DefaultVersion(lang)
	if err != nil {
		return "", "", err
	}
	return install.ToolchainsHome + "/" + lang, def.CacheKey(version), nil
}

// githubCacheKey is an actions/cache key, and the prefix restore-keys
// falls back to: the toolchain's carries hecatron's version, since a new
// hecatron can install differently.
func githubCacheKey(lang string, entry cacheEntry, version string) (key, restore string) {
	restore = fmt.Sprintf("hecatron-%s-%s-${{ runner.os }}-", lang, entry.kind)
	if entry.kind == toolchainCache {
		restore += version + "-"
	}
	if entry.data != "" {
		restore += "${{ runner.arch }}-" + entry.data + "-"
	}
	if len(entry.keyFiles) == 0 {
		return restore + "none", restore
	}
	quoted := make([]string, len(entry.keyFiles))
	for i, file := range entry.keyFiles {
		quoted[i] = "'" + file + "'"
	}
	return restore + "${{ hashFiles(" + strings.Join(quoted, ", ") + ") }}", restore
}

// githubCacheSteps are the actions/cache steps for a language, indented
// to sit in a job's steps at the given column.
func githubCacheSteps(c languageCache, version, indent string) []string {
	var lines []string
	for _, entry := range c.caches {
		key, restore := githubCacheKey(c.lang, entry, version)
		lines = append(lines,
			indent+"- name: "+yamlScalar(fmt.Sprintf("Cache %s %s", c.lang, entry.kind)),
			indent+"  uses: actions/cache@v4",
			indent+"  with:",
			indent+"    path: |")
		for _, dir := range entry.dirs {
			lines = append(lines, indent+"      "+dir)
		}
		lines = append(lines,
			indent+"    key: "+yamlScalar(key),
			indent+"    restore-keys: "+yamlScalar(restore))
	}
	return lines
}

var checkoutStep = regexp.MustCompile(`^(\s*)(- )?(\s*)uses:\s*actions/checkout@`)

// withGitHubCache adds a language's cache steps to a workflow, after each
// job's actions/checkout step (the cache keys hash the checked-out
// files); a workflow without one is left as it is.
func withGitHubCache(body string, c languageCache, version string) string {
	if len(c.caches) == 0 {
		return body
	}
	lines := strings.Split(body, "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		out = append(out, lines[i])
		match := checkoutStep.FindStringSubmatch(lines[i])
		if match == nil {
			continue
		}
		dash := stepIndent(lines, i, match)
		if dash < 0 {
			continue
		}
		end := i + 1
		for end < len(lines) && (strings.TrimSpace(lines[end]) == "" || indentOf(lines[end]) > dash) {
			end++
		}
		for end > i+1 && strings.TrimSpace(lines[end-1]) == "" {
			end--
		}
		out = append(out, lines[i+1:end]...)
		out = append(out, githubCacheSteps(c, version, strings.Repeat(" ", dash))...)
		i = end - 1
	}
	return strings.Join(out, "\n")
}

// stepIndent is the column of the "- " starting the step a checkout's
// uses: line is in, or -1 when it isn't in a list.
func stepIndent(lines []string, at int, match []string) int {
	if match[2] != "" {
		return len(match[1])
	}
	own := indentOf(lines[at])
	for j := at - 1; j >= 0; j-- {
		line := lines[j]
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := indentOf(line)
		if indent < own && strings.HasPrefix(strings.TrimLeft(line, " "), "- ") {
			if indent+2 == own {
				return indent
			}
			return -1
		}
		if indent < own {
			return -1
		}
	}
	return -1
}

func indentOf(line string) int { return len(line) - len(strings.TrimLeft(line, " ")) }

// gitlabReportDir is where the pipeline's hct commands write their reports
// for GitLab, relative to the project directory jobs start in.
const gitlabReportDir = "hecatron-reports"

// GitLabCI is the project's .gitlab-ci.yml: a job per language that
// installs hecatron (at the version that generated it) and runs hct
// install, test and lint for the language, which cover every component.
func GitLabCI(p *project.Project, settings project.CISettings, version string) (string, error) {
	tag := "v" + strings.TrimPrefix(version, "v")
	banner := Banner([]string{
		"Generated by: hecatron workflows (generated.ci: gitlab)",
		"from the project's languages and hecatron " + tag,
		"",
		"To update: hecatron workflows",
		"(hecatron add and remove regenerate it too)",
	}, "# ", "")
	lines := []string{strings.TrimRight(banner, "\n"), "",
		"variables:",
		"  HECATRON_VERSION: " + yamlScalar(tag),
		"  HECATRON_REPORT_DIR: " + yamlScalar(gitlabReportDir),
		"",
		"default:",
		"  image: " + yamlScalar(settings.Image),
		"  before_script:",
		"    - " + yamlScalar("apt-get update -qq && apt-get install -y -qq --no-install-recommends ca-certificates curl > /dev/null"),
		"    - " + yamlScalar(`curl -fsSL "https://raw.githubusercontent.com/dnf/hecatron/${HECATRON_VERSION}/install.sh" | HECATRON_NO_MODIFY_PATH=1 sh`),
		"    - " + yamlScalar(`. "${HOME}/.hecatron/env"`),
	}
	for _, lang := range p.Languages {
		lines = append(lines, "", lang+":", "  timeout: "+p.CITimeoutMinutes(lang)+"m")
		var script []string
		if settings.Cache {
			c, err := projectCache(p, lang)
			if err != nil {
				return "", err
			}
			cache, links := gitlabCache(c, tag)
			lines = append(lines, cache...)
			script = links
		}
		script = append(script, "hct install "+lang, "hct test "+lang, "hct lint "+lang)
		lines = append(lines, "  script:")
		for _, command := range script {
			lines = append(lines, "    - "+yamlScalar(command))
		}
		// HECATRON_REPORT_DIR's files (docs/runner.md, "Reports"): the
		// tests for the merge request's test widget, lint's findings for
		// its code quality widget; kept when the job fails too.
		lines = append(lines, "  artifacts:", "    when: always", "    reports:",
			"      junit: "+yamlScalar(gitlabReportDir+"/test-"+lang+".junit.xml"),
			"      codequality: "+yamlScalar(gitlabReportDir+"/lint-"+lang+".codequality.json"))
	}
	return strings.Join(lines, "\n") + "\n", nil
}

// gitlabCache is a job's cache: entries, and the commands linking each
// cached home directory to its place under the project, run before
// anything is installed.
func gitlabCache(c languageCache, tag string) (yaml, links []string) {
	if len(c.caches) == 0 {
		return nil, nil
	}
	yaml = append(yaml, "  cache:")
	for _, entry := range c.caches {
		prefix := c.lang + "-" + entry.kind
		if entry.kind == toolchainCache {
			prefix += "-" + tag
		}
		if entry.data != "" {
			prefix += "-" + entry.data
		}
		yaml = append(yaml, "    - key:")
		if files := gitlabKeyFileList(entry.keyFiles); len(files) > 0 {
			yaml = append(yaml, "        files:")
			for _, file := range files {
				yaml = append(yaml, "          - "+yamlScalar(file))
			}
			yaml = append(yaml, "        prefix: "+yamlScalar(prefix))
		} else {
			yaml[len(yaml)-1] = "    - key: " + yamlScalar(prefix)
		}
		yaml = append(yaml, "      paths:")
		for _, dir := range entry.dirs {
			if strings.HasPrefix(dir, "~/") {
				cached := cacheRoot + "/" + c.lang + "/" + cachedName(dir)
				home := `"${HOME}/` + strings.TrimPrefix(dir, "~/") + `"`
				links = append(links, fmt.Sprintf(`mkdir -p "%s" "$(dirname %s)" && ln -sfn "${CI_PROJECT_DIR}/%s" %s`,
					cached, home, cached, home))
				dir = cached
			}
			yaml = append(yaml, "        - "+yamlScalar(dir))
		}
	}
	return yaml, links
}

// gitlabKeyFileList fits a cache's key files in GitLab's limit; the
// definitions' tests keep every built-in one within it.
func gitlabKeyFileList(files []string) []string {
	if len(files) > gitlabKeyFiles {
		return files[:gitlabKeyFiles]
	}
	return files
}

// cachedName is a home directory's name under the cache root:
// ~/.cargo/registry is cargo-registry.
func cachedName(dir string) string {
	parts := strings.Split(strings.TrimPrefix(dir, "~/"), "/")
	for i, part := range parts {
		parts[i] = strings.TrimLeft(part, ".")
	}
	return strings.Join(parts, "-")
}

var plainScalar = regexp.MustCompile(`^[A-Za-z0-9_./~][A-Za-z0-9_./~ =+-]*$`)

var yamlReserved = regexp.MustCompile(`^(?i:true|false|yes|no|on|off|null|~|y|n|\.inf|\.nan)$|^[-+]?\.?[0-9]`)

// yamlScalar is a string as a YAML scalar: plain when that reads back as
// the same string, else single-quoted.
func yamlScalar(s string) string {
	if plainScalar.MatchString(s) && !yamlReserved.MatchString(s) && !strings.HasSuffix(s, " ") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// CachedWorkflow is a language's workflow (Workflow) with its cache steps
// added after each job's checkout.
func CachedWorkflow(p *project.Project, lang, version string) (content string, ok bool, err error) {
	content, ok, err = Workflow(p, lang)
	if err != nil || !ok {
		return content, ok, err
	}
	c, err := projectCache(p, lang)
	if err != nil {
		return "", false, err
	}
	return withGitHubCache(content, c, "v"+strings.TrimPrefix(version, "v")), true, nil
}
