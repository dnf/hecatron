package project

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The formats pin and support files are read in (docs/design.md §18), as
// libexec/lib/versions.py reads them: each turns a file's text into a spec,
// into "names no version" (a channel like "stable", which only the
// ecosystem's own tool resolves), or into "doesn't pin this language" (a
// go.mod without a toolchain line), when resolution keeps looking.

// pinned is what a file says about a language's version.
type pinned struct {
	pins bool   // false: the file doesn't concern this language (NOT_PINNED)
	spec string // "" when it names no version (None)
}

var (
	notPinned = pinned{}
	noVersion = pinned{pins: true}
)

func pinnedSpec(spec string) pinned { return pinned{pins: true, spec: spec} }

// pinKey is an entry's "key", which may be absent (Python's None).
type pinKey struct {
	name string
	set  bool
}

type pinFormat func(text string, key pinKey) (pinned, error)

var pinFormats map[string]pinFormat

func init() {
	pinFormats = map[string]pinFormat{
		"python-version":      firstLineFormat,
		"nvmrc":               firstLineFormat,
		"node-version":        firstLineFormat,
		"ruby-version":        rubyVersion,
		"php-version":         firstLineFormat,
		"composer-json":       composerJSON,
		"swift-version":       firstLineFormat,
		"java-version":        javaVersion,
		"rust-toolchain":      rustToolchain,
		"rust-toolchain-toml": rustToolchainTOML,
		"global-json":         globalJSON,
		"go-mod-toolchain":    goModToolchain,
		"build-zig-zon":       buildZigZon,
		"gradle-daemon-jvm":   gradleDaemonJVM,
		"tool-versions":       toolVersions,
		"mise":                mise,
		"sdkmanrc":            sdkmanrc,
		"python-classifiers":  pythonClassifiers,
		"github-setup":        githubSetup,
		"maven-source-level":  mavenSourceLevel,
		"target-framework":    targetFramework,
	}
}

// readPin is versions.py's read_pin.
func readPin(format, text string, key pinKey) (pinned, error) {
	read, ok := pinFormats[format]
	if !ok {
		names := make([]string, 0, len(pinFormats))
		for name := range pinFormats {
			names = append(names, name)
		}
		sort.Strings(names)
		return notPinned, &Error{fmt.Sprintf("unknown pin file format %s (known: %s)", pyStringRepr(format), strings.Join(names, ", "))}
	}
	if err := checkDigits(text); err != nil {
		return notPinned, err
	}
	return read(text, key)
}

// plain is versions.py's _plain: an ecosystem's version as a spec, or
// noVersion for a channel name.
func plain(value string, exact bool) pinned {
	value = strings.Trim(pyStrip(value), `"'`)
	if !isVersion(value) {
		return noVersion
	}
	value = strings.TrimLeft(value, "v")
	if exact {
		value = "=" + value
	}
	return pinnedSpec(value)
}

// stripPin is versions.py's strip_pin: an entry's "strip" removed from the
// version a pin file gives (asdf's Elixir "1.17.3-otp-27" is 1.17.3), and
// noVersion when what's left isn't one.
func stripPin(value pinned, strip *regexp.Regexp) pinned {
	if strip == nil || !value.pins || value.spec == "" {
		return value
	}
	exact := strings.HasPrefix(value.spec, "=")
	rest := strings.TrimPrefix(value.spec, "=")
	rest = strip.ReplaceAllString(rest, "")
	if !isVersion(rest) {
		return noVersion
	}
	rest = strings.TrimLeft(rest, "v")
	if exact {
		rest = "=" + rest
	}
	return pinnedSpec(rest)
}

// firstLine is versions.py's _first_line: the first line with something
// before any comment; ok is false for none.
func firstLine(text string) (string, bool) {
	for _, line := range pySplitLines(text) {
		line, _, _ = strings.Cut(line, "#")
		if line = pyStrip(line); line != "" {
			return line, true
		}
	}
	return "", false
}

func firstLineFormat(text string, _ pinKey) (pinned, error) {
	line, ok := firstLine(text)
	if !ok {
		return noVersion, nil
	}
	return plain(line, false), nil
}

func rubyVersion(text string, _ pinKey) (pinned, error) {
	line, ok := firstLine(text)
	if !ok {
		return noVersion, nil
	}
	return plain(strings.TrimPrefix(line, "ruby-"), false), nil
}

var trailingVersion = regexp.MustCompile(`(\p{Nd}+(?:\.\p{Nd}+)*)$`)

// javaVersion reads jenv's names: "17", "17.0.2", "temurin64-21.0.2".
func javaVersion(text string, _ pinKey) (pinned, error) {
	line, _ := firstLine(text)
	match := trailingVersion.FindStringSubmatch(line)
	if match == nil {
		return noVersion, nil
	}
	return plain(match[1], false), nil
}

// tomlString is what _toml_value finds: a value, none (None), or the key
// missing from the section (NOT_PINNED).
type tomlString struct {
	value      string
	found, set bool
}

var (
	quotedItem   = regexp.MustCompile(`["']([^"']*)["']`)
	tableVersion = pyRegexp(`version\s*=\s*["']([^"']*)["']`)
)

// tomlValue is versions.py's _toml_value: a string (or the first of a list,
// or a table's "version") under [section] in a small TOML file.
func tomlValue(text, section string, key pinKey) tomlString {
	current, inSection := "", false
	for _, raw := range pySplitLines(text) {
		line, _, _ := strings.Cut(raw, "#")
		line = pyStrip(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			current, inSection = pyStrip(strings.Trim(line, "[]")), true
			continue
		}
		if !inSection || current != section || !strings.Contains(line, "=") {
			continue
		}
		name, value, _ := strings.Cut(line, "=")
		name, value = pyStrip(name), pyStrip(value)
		if !key.set || strings.Trim(name, `"'`) != key.name {
			continue
		}
		switch {
		case strings.HasPrefix(value, "["):
			if items := quotedItem.FindStringSubmatch(value); items != nil {
				return tomlString{value: items[1], found: true, set: true}
			}
			return tomlString{found: true}
		case strings.HasPrefix(value, "{"):
			if match := tableVersion.FindStringSubmatch(value); match != nil {
				return tomlString{value: match[1], found: true, set: true}
			}
			return tomlString{found: true}
		}
		return tomlString{value: strings.Trim(value, `"'`), found: true, set: true}
	}
	return tomlString{}
}

func rustToolchainTOML(text string, _ pinKey) (pinned, error) {
	channel := tomlValue(text, "toolchain", pinKey{"channel", true})
	if !channel.set {
		return noVersion, nil
	}
	return plain(channel.value, true), nil
}

// rustToolchain reads the legacy file: a bare channel, or the same TOML as
// rust-toolchain.toml.
func rustToolchain(text string, key pinKey) (pinned, error) {
	if strings.Contains(text, "[toolchain]") {
		return rustToolchainTOML(text, key)
	}
	line, ok := firstLine(text)
	if !ok {
		return noVersion, nil
	}
	return plain(line, true), nil
}

// rollForwardParts is how many leading parts of global.json's sdk.version
// each rollForward policy keeps (versions.py's _ROLL_FORWARD_PARTS; disable
// is exact).
var rollForwardParts = map[string]int{
	"patch": 3, "latestPatch": 3, "feature": 2, "latestFeature": 2,
	"minor": 1, "latestMinor": 1, "major": 1, "latestMajor": 1,
}

// pyJSONLoads is Python's json.loads: ok is false where it raises
// JSONDecodeError. A document Go's decoder rejects but Python might read
// (NaN, Infinity) is declined.
func pyJSONLoads(text string) (value any, ok bool, err error) {
	value, err = decodeJSON([]byte(text))
	if err == nil {
		if jsonDepth(text) > 100 {
			return nil, false, unsupported("JSON nested too deep") // Python's recursion limit
		}
		return value, true, nil
	}
	if strings.Contains(text, "NaN") || strings.Contains(text, "Infinity") {
		return nil, false, err
	}
	return nil, false, nil
}

func jsonDepth(text string) int {
	depth, deepest := 0, 0
	for _, c := range text {
		switch c {
		case '[', '{':
			if depth++; depth > deepest {
				deepest = depth
			}
		case ']', '}':
			depth--
		}
	}
	return deepest
}

func globalJSON(text string, _ pinKey) (pinned, error) {
	document, ok, err := pyJSONLoads(text)
	if err != nil {
		return notPinned, err
	}
	top, isObject := document.(*Object)
	if !ok || !isObject {
		return noVersion, nil // JSONDecodeError or AttributeError
	}
	sdkValue, _ := top.Get("sdk")
	sdk := &Object{}
	if truthy(sdkValue) {
		if sdk, isObject = sdkValue.(*Object); !isObject {
			// Python's `"version" in sdk` looks in a string or list; finding it,
			// it then fails calling sdk.get.
			if !pyContains(sdkValue, "version") {
				return notPinned, nil
			}
			return notPinned, unsupported("global.json's sdk is not an object")
		}
	}
	versionValue, present := sdk.Get("version")
	if !present {
		return notPinned, nil
	}
	policyValue, hasPolicy := sdk.Get("rollForward")
	switch policyValue.(type) {
	case []any, *Object:
		return notPinned, unsupported("global.json's rollForward is not a string") // unhashable in Python
	}
	versionText, isString := versionValue.(string)
	if versionValue != nil && !isString {
		return notPinned, unsupported("global.json's sdk.version is not a string")
	}
	if versionValue == nil {
		return noVersion, nil
	}
	policy, policyIsString := policyValue.(string)
	if !hasPolicy {
		policy, policyIsString = "patch", true
	}
	parts, known := rollForwardParts[policy]
	if !policyIsString || !known {
		return plain(versionText, policyIsString && policy == "disable"), nil
	}
	result := plain(versionText, false)
	if result.spec == "" || strings.Contains(result.spec, "-") {
		return result, nil
	}
	if policy == "latestMajor" {
		return pinnedSpec(floorPrefix + result.spec), nil
	}
	return pinnedSpec(strings.Join(prefixStrings(strings.Split(result.spec, "."), parts), ".")), nil
}

// pyContains is Python's `item in value` for a string or list; anything
// else (a number) raises, which is declined.
func pyContains(value any, item string) bool {
	switch v := value.(type) {
	case string:
		return strings.Contains(v, item)
	case []any:
		for _, element := range v {
			if element == item {
				return true
			}
		}
		return false
	}
	return true
}

func prefixStrings(list []string, n int) []string {
	if n < len(list) {
		return list[:n]
	}
	return list
}

// composerJSON reads config.platform.php, the PHP Composer resolves
// dependencies for: the newest release on its line.
func composerJSON(text string, _ pinKey) (pinned, error) {
	document, ok, err := pyJSONLoads(text)
	if err != nil || !ok {
		return notPinned, err
	}
	get := func(value any, key string) (any, bool) {
		object, isObject := value.(*Object)
		if !isObject {
			return nil, false // AttributeError: not a dict
		}
		found, _ := object.Get(key)
		return found, true
	}
	orEmpty := func(value any) any {
		if !truthy(value) {
			return &Object{}
		}
		return value
	}
	config, ok := get(document, "config")
	if !ok {
		return notPinned, nil
	}
	platform, ok := get(orEmpty(config), "platform")
	if !ok {
		return notPinned, nil
	}
	php, ok := get(orEmpty(platform), "php")
	if !ok || php == nil {
		return notPinned, nil
	}
	var text2 string
	switch v := php.(type) {
	case []any, *Object:
		return noVersion, nil // str() of either is never a version
	default:
		if text2, err = pyStr(v); err != nil {
			return notPinned, err
		}
	}
	if err := checkDigits(text2); err != nil {
		return notPinned, err
	}
	result := plain(text2, false)
	if result.spec == "" {
		return noVersion, nil
	}
	return pinnedSpec(strings.Join(prefixStrings(strings.Split(result.spec, "."), 2), ".")), nil
}

var (
	goModToolchainLine  = pyRegexp(`(?m)^\s*toolchain\s+go(\S+)\s*$`)
	gradleToolchainLine = pyRegexp(`(?m)^\s*toolchainVersion\s*=\s*(\S+)\s*$`)
	minimumZigVersion   = pyRegexp(`(?m)^[^/\n]*?\.minimum_zig_version\s*=\s*"([^"]*)"`)
	preOrBuild          = regexp.MustCompile(`[-+]`)
)

// buildZigZon reads minimum_zig_version as its release line: Zig breaks
// compatibility between minor releases (0.14 code doesn't build with
// 0.15), so it names the line whose newest release the build works with,
// not a floor the default meets.
func buildZigZon(text string, _ pinKey) (pinned, error) {
	match := minimumZigVersion.FindStringSubmatch(text)
	if match == nil {
		return notPinned, nil
	}
	result := plain(match[1], false)
	if result.spec == "" {
		return noVersion, nil
	}
	release := preOrBuild.Split(result.spec, 2)[0]
	return pinnedSpec(strings.Join(prefixStrings(strings.Split(release, "."), 2), ".")), nil
}

func goModToolchain(text string, _ pinKey) (pinned, error) {
	if match := goModToolchainLine.FindStringSubmatch(text); match != nil {
		return plain(match[1], true), nil
	}
	return notPinned, nil
}

func gradleDaemonJVM(text string, _ pinKey) (pinned, error) {
	if match := gradleToolchainLine.FindStringSubmatch(text); match != nil {
		return plain(match[1], false), nil
	}
	return notPinned, nil
}

func toolVersions(text string, key pinKey) (pinned, error) {
	for _, line := range pySplitLines(text) {
		before, _, _ := strings.Cut(line, "#")
		fields := pyFields(before)
		if len(fields) >= 2 && key.set && fields[0] == key.name {
			return plain(fields[1], false), nil
		}
	}
	return notPinned, nil
}

func mise(text string, key pinKey) (pinned, error) {
	value := tomlValue(text, "tools", key)
	switch {
	case !value.found:
		return notPinned, nil
	case !value.set:
		return noVersion, nil
	}
	return plain(value.value, false), nil
}

// sdkmanVendor is the vendor SDKMAN! identifiers add: 21.0.2-tem.
var sdkmanVendor = regexp.MustCompile(`-[A-Za-z][\p{L}\p{N}_.]*$`)

func sdkmanrc(text string, key pinKey) (pinned, error) {
	wanted := "java"
	if key.set && key.name != "" {
		wanted = key.name
	}
	for _, line := range pySplitLines(text) {
		line, _, _ = strings.Cut(line, "#")
		line = pyStrip(line)
		name, value, found := strings.Cut(line, "=")
		if found && pyStrip(name) == wanted {
			return plain(sdkmanVendor.ReplaceAllString(pyStrip(value), ""), false), nil
		}
	}
	return notPinned, nil
}

var troveClassifier = pyRegexp(`Programming Language :: Python :: (\d+\.\d+)`)

// pythonClassifiers reads the trove classifiers a pyproject.toml, setup.cfg
// or setup.py lists: the newest Python the project says it supports. The
// pattern's \b after the version is checked here, as Python's \b knows
// Unicode word characters and RE2's doesn't.
func pythonClassifiers(text string, _ pinKey) (pinned, error) {
	var found []string
	for _, match := range troveClassifier.FindAllStringSubmatchIndex(text, -1) {
		if next, size := utf8.DecodeRuneInString(text[match[1]:]); size > 0 && pyIsWord(next) {
			continue
		}
		found = append(found, text[match[2]:match[3]])
	}
	if best, ok := newest(found); ok {
		return plain(best, false), nil
	}
	return notPinned, nil
}

// setupParts are the parts a setup step's version input needs to say
// anything: "python-version: 3.x" doesn't.
var setupParts = map[string]int{"python-version": 2, "go-version": 2, "ruby-version": 2, "php-version": 2}

var (
	yamlComment  = pyRegexp(`\s+#.*$`)
	yamlListItem = pyRegexp(`^\s*-\s+(\S+)\s*$`)
	matrixInput  = pyRegexp(`^\$\{\{\s*matrix\.([\p{L}\p{N}_-]+)\s*\}\}$`)
	setupVersion = regexp.MustCompile(`^\p{Nd}+(\.\p{Nd}+)*$`)
)

// yamlValues is versions.py's _yaml_values: every value a key is given in a
// workflow, inline, as a [list], or as the "- item" lines under it.
func yamlValues(text, key string) []string {
	pattern := pyRegexp(`^(\s*)(?:-\s+)?` + regexp.QuoteMeta(key) + `\s*:\s*(.*?)\s*$`)
	lines := pySplitLines(text)
	var values []string
	for i, line := range lines {
		match := pattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		value := yamlComment.ReplaceAllString(match[2], "")
		switch {
		case strings.HasPrefix(value, "["):
			for _, item := range strings.Split(strings.Trim(value, "[]"), ",") {
				values = append(values, pyStrip(item))
			}
		case value != "":
			values = append(values, value)
		default:
			for _, item := range lines[i+1:] {
				listed := yamlListItem.FindStringSubmatch(item)
				if listed == nil {
					break
				}
				values = append(values, listed[1])
			}
		}
	}
	return values
}

// setupStepVersion is versions.py's _setup_version: a setup input's value
// as a version line, or false.
func setupStepVersion(value, key string) (string, bool) {
	value = strings.Trim(pyStrip(value), `"'`)
	for strings.HasSuffix(value, ".x") {
		value = value[:len(value)-2]
	}
	if key == "java-version" && strings.HasPrefix(value, "1.") {
		value = value[2:] // 1.8 is Java 8
	}
	needed, ok := setupParts[key]
	if !ok {
		needed = 1
	}
	if !setupVersion.MatchString(value) || len(strings.Split(value, ".")) < needed {
		return "", false
	}
	return value, true
}

// githubSetup reads a GitHub Actions setup step's version input: the newest
// version the project's CI runs the language on.
func githubSetup(text string, key pinKey) (pinned, error) {
	if !key.set {
		return notPinned, unsupported("github-setup without a key")
	}
	var found []string
	for _, value := range yamlValues(text, key.name) {
		candidates := []string{value}
		if matrix := matrixInput.FindStringSubmatch(strings.Trim(value, `"'`)); matrix != nil {
			candidates = nil
			for _, v := range yamlValues(text, matrix[1]) {
				if !strings.Contains(v, "${{") {
					candidates = append(candidates, v)
				}
			}
		}
		for _, candidate := range candidates {
			if v, ok := setupStepVersion(candidate, key.name); ok {
				found = append(found, v)
			}
		}
	}
	if best, ok := newest(found); ok {
		return plain(best, false), nil
	}
	return notPinned, nil
}

var mavenLevel = pyRegexp(`<(?:maven\.compiler\.)?(?:source|target|release)>\s*([\p{Nd}.]+)\s*</`)

// newestJDKForLevel is javac's oldest --source/--target levels: JDK 12
// dropped 6 and JDK 20 dropped 7.
var newestJDKForLevel = []struct {
	level string
	jdk   string
}{{"6", "11"}, {"7", "19"}}

func mavenSourceLevel(text string, _ pinKey) (pinned, error) {
	var levels []string
	for _, match := range mavenLevel.FindAllStringSubmatch(text, -1) {
		level := strings.TrimPrefix(match[1], "1.")
		if level != "" && strings.Trim(level, "0123456789") == "" {
			levels = append(levels, pyInt(level))
		}
	}
	if len(levels) == 0 {
		return notPinned, nil
	}
	lowest := levels[0]
	for _, level := range levels {
		if compareInts(level, lowest) < 0 {
			lowest = level
		}
	}
	for _, rule := range newestJDKForLevel {
		if compareInts(lowest, rule.level) <= 0 {
			return pinnedSpec(rule.jdk), nil
		}
	}
	return notPinned, nil
}

// floorPrefix marks a spec that is a floor, not a pin (versions.py's
// FLOOR): any version at least this one will do.
const floorPrefix = ">="

var (
	targetFrameworks   = regexp.MustCompile(`<TargetFrameworks?>([^<]*)<`)
	targetFrameworkNet = regexp.MustCompile(`(?:^|[^A-Za-z0-9_.])net(\d+)\.\d+`)
)

// targetFramework is versions.py's _target_framework: the newest .NET an
// MSBuild project builds for, as a floor on the SDK's major.
func targetFramework(text string, _ pinKey) (pinned, error) {
	newest := -1
	for _, value := range targetFrameworks.FindAllStringSubmatch(text, -1) {
		for _, match := range targetFrameworkNet.FindAllStringSubmatch(value[1], -1) {
			if major, err := strconv.Atoi(match[1]); err == nil && major >= 5 && major > newest {
				newest = major
			}
		}
	}
	if newest < 0 {
		return notPinned, nil
	}
	return pinnedSpec(floorPrefix + strconv.Itoa(newest)), nil
}
