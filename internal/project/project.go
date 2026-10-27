// Package project is the read-only hecatron project model: finding the
// project, its config.json and local.json, components, language definitions
// and file resolution (docs/design.md §6-§7). It is a port of
// libexec/lib/project.py and must agree with it: tests run both on the same
// projects.
//
// Where project.py's result depends on how Python treats an unusual value
// (a "components" string iterates as its characters, true == 1), the model
// returns an *UnsupportedError instead of guessing, and the caller hands the
// command to the Python implementation. So the Go model may refuse more than
// project.py does, never less, and what it does answer is project.py's
// answer.
package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// Schema is the config.json format this model reads.
const Schema = 1

// Error is a project that can't be found or read. Its message is
// project.py's ProjectError message, word for word.
type Error struct{ Message string }

func (e *Error) Error() string { return e.Message }

// UnsupportedError is a project the Go model doesn't read yet, though
// project.py might.
type UnsupportedError struct{ Reason string }

func (e *UnsupportedError) Error() string { return "unsupported by the Go model: " + e.Reason }

func unsupported(format string, args ...any) error {
	return &UnsupportedError{Reason: fmt.Sprintf(format, args...)}
}

// IsUnsupported reports whether err means "ask project.py".
func IsUnsupported(err error) bool {
	var u *UnsupportedError
	return errors.As(err, &u)
}

// Home is where hecatron is installed: HECATRON_HOME, and the built-in
// language definitions (HECATRON_BUILTIN_LANGUAGES, else <home>/languages).
type Home struct {
	Dir       string
	Languages string
}

// NewHome is the Home for an install directory, honouring
// HECATRON_BUILTIN_LANGUAGES as project.py does (the tests point it at
// fixture definitions).
func NewHome(dir string) Home {
	languages := os.Getenv("HECATRON_BUILTIN_LANGUAGES")
	if languages == "" {
		languages = pyJoin(dir, "languages")
	}
	return Home{Dir: dir, Languages: languages}
}

// Version is hecatron's own version, from <home>/VERSION.
func (h Home) Version() string {
	data, err := os.ReadFile(pyJoin(h.Dir, "VERSION"))
	if err != nil {
		return "0.0.0"
	}
	return pyStrip(string(data))
}

// FindRoot is the nearest directory at or above start holding
// .hecatron/config.json.
func FindRoot(start string) (string, error) {
	directory, err := filepath.Abs(start)
	if err != nil {
		return "", unsupported("%v", err)
	}
	for {
		if isFile(filepath.Join(directory, ".hecatron", "config.json")) {
			return directory, nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", &Error{"not a hecatron project (no .hecatron/config.json here or above); " +
				"run 'hecatron init' to set one up"}
		}
		directory = parent
	}
}

// Project is a project's configuration as project.py reads it.
type Project struct {
	Root string
	Dir  string // <root>/.hecatron
	Home Home

	Name       string
	Components []string
	ImplPath   string
	Languages  []string // sorted
	Disabled   []string // sorted, from local.json
	Logs       string
	Metadata   string // the metadata file's path, or ""

	config      *Object
	implPaths   map[string]string
	definitions *Definitions
}

// Load reads the project at root: config.json, checked as project.py checks
// it, and local.json.
func Load(root string, home Home) (*Project, error) {
	p := &Project{Root: root, Dir: pyJoin(root, ".hecatron"), Home: home}
	configPath := pyJoin(p.Dir, "config.json")
	config, err := readJSON(configPath)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, &Error{configPath + " not found"}
	}
	object, ok := config.(*Object)
	if !ok {
		return nil, unsupported("config.json is not an object")
	}
	p.config = object
	if err := p.checkSchema(configPath); err != nil {
		return nil, err
	}
	if err := p.checkHecatronVersion(); err != nil {
		return nil, err
	}
	if err := p.readConfig(); err != nil {
		return nil, err
	}
	if err := p.readLocal(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Project) checkSchema(configPath string) error {
	schema, _ := p.config.Get("schema")
	if isSchema(schema) {
		return nil
	}
	repr, err := pyRepr(schema)
	if err != nil {
		return err
	}
	return &Error{fmt.Sprintf("%s: schema %s is not supported (this hecatron reads schema %d)",
		configPath, repr, Schema)}
}

// isSchema is Python's schema == 1, which true and 1.0 also satisfy.
func isSchema(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case json.Number:
		f, err := strconv.ParseFloat(string(v), 64)
		return err == nil && f == Schema
	}
	return false
}

// checkHecatronVersion refuses a project that needs a newer hecatron
// ("hecatron": ">=0.3.0").
func (p *Project) checkHecatronVersion() error {
	needed := ""
	if value, ok := p.config.Get("hecatron"); ok {
		text, err := pyStr(value)
		if err != nil {
			return err
		}
		needed = pyStrip(strings.TrimLeft(text, ">="))
	}
	if needed == "" {
		return nil
	}
	installed := p.Home.Version()
	have, err := versionTuple(installed)
	if err != nil {
		return err
	}
	want, err := versionTuple(needed)
	if err != nil {
		return err
	}
	if lessTuple(have, want) {
		return &Error{fmt.Sprintf("this project needs hecatron %s or newer (installed: %s)", needed, installed)}
	}
	return nil
}

// versionTuple is project.py's _version_tuple: the dot-separated parts that
// are all digits.
func versionTuple(version string) ([]int, error) {
	var numbers []int
	for _, part := range strings.Split(version, ".") {
		if part == "" || strings.Trim(part, "0123456789") != "" {
			if strings.IndexFunc(part, func(r rune) bool { return r > 0x7f }) >= 0 {
				return nil, unsupported("non-ASCII version %q", version) // Python's isdigit takes other digits
			}
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return nil, unsupported("version part %q", part)
		}
		numbers = append(numbers, n)
	}
	return numbers, nil
}

func lessTuple(a, b []int) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}

// pyStrip is Python's str.strip(), whose whitespace also takes in the
// ASCII separators \x1c-\x1f.
func pyStrip(s string) string {
	return strings.TrimFunc(s, func(r rune) bool { return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f) })
}

// pyStr is Python's str() of a decoded scalar.
func pyStr(value any) (string, error) {
	if s, ok := value.(string); ok {
		return s, nil
	}
	return pyRepr(value)
}

func (p *Project) readConfig() error {
	var err error
	if p.Name, err = p.optionalString("project"); err != nil {
		return err
	}
	if p.Name == "" {
		p.Name = pyBasename(p.Root)
	}
	if p.Components, err = p.optionalStrings("components"); err != nil {
		return err
	}
	if len(p.Components) == 0 {
		p.Components = []string{"."}
	}
	if p.ImplPath, err = p.optionalString("implPath"); err != nil {
		return err
	}
	if p.ImplPath == "" {
		p.ImplPath = "{component}/{lang}"
		if len(p.Components) == 1 && p.Components[0] == "." {
			p.ImplPath = "{lang}"
		}
	}
	if p.implPaths, err = p.optionalStringMap("implPaths"); err != nil {
		return err
	}
	if p.Languages, err = p.stringList(p.config, "languages", "config.json"); err != nil {
		return err
	}
	sort.Strings(p.Languages)
	logs := ".logs"
	if value, ok := p.config.Get("logs"); ok {
		if logs, ok = value.(string); !ok {
			return unsupported(`"logs" is not a string`)
		}
	}
	p.Logs = pyJoin(p.Root, logs)
	metadata, err := p.optionalString("metadata")
	if err != nil {
		return err
	}
	if metadata != "" {
		p.Metadata = pyJoin(p.Root, metadata)
	}
	return nil
}

func (p *Project) readLocal() error {
	local, err := readJSON(pyJoin(p.Dir, "local.json"))
	if err != nil {
		return err
	}
	if local == nil {
		if isFile(pyJoin(p.Dir, "local.json")) {
			return unsupported("local.json is null")
		}
		return nil
	}
	object, ok := local.(*Object)
	if !ok {
		return unsupported("local.json is not an object")
	}
	if p.Disabled, err = p.stringList(object, "disabled", "local.json"); err != nil {
		return err
	}
	sort.Strings(p.Disabled)
	return nil
}

// optionalString is a key Python reads as `config.get(key) or default`:
// "" when missing or false-like, the string when one.
func (p *Project) optionalString(key string) (string, error) {
	value, _ := p.config.Get(key)
	if !truthy(value) {
		return "", nil
	}
	if s, ok := value.(string); ok {
		return s, nil
	}
	return "", unsupported("%q is not a string", key)
}

// optionalStrings is a list Python reads as `config.get(key) or default`.
func (p *Project) optionalStrings(key string) ([]string, error) {
	value, _ := p.config.Get(key)
	if !truthy(value) {
		return nil, nil
	}
	return toStrings(value, key)
}

func (p *Project) optionalStringMap(key string) (map[string]string, error) {
	value, _ := p.config.Get(key)
	if !truthy(value) {
		return nil, nil
	}
	object, ok := value.(*Object)
	if !ok {
		return nil, unsupported("%q is not an object", key)
	}
	result := map[string]string{}
	for _, name := range object.Keys {
		entry := object.Values[name]
		if !truthy(entry) {
			continue
		}
		s, ok := entry.(string)
		if !ok {
			return nil, unsupported("%q.%s is not a string", key, name)
		}
		result[name] = s
	}
	return result, nil
}

// stringList is a list Python reads as `sorted(data.get(key, []))`: missing
// is empty, anything but a list of strings unsupported.
func (p *Project) stringList(data *Object, key, file string) ([]string, error) {
	value, ok := data.Get(key)
	if !ok {
		return nil, nil
	}
	list, err := toStrings(value, file+" "+key)
	return list, err
}

func toStrings(value any, what string) ([]string, error) {
	list, ok := value.([]any)
	if !ok {
		return nil, unsupported("%s is not a list", what)
	}
	result := make([]string, 0, len(list))
	for _, item := range list {
		s, ok := item.(string)
		if !ok {
			return nil, unsupported("%s holds a non-string", what)
		}
		result = append(result, s)
	}
	return result, nil
}

// ImplTemplate is a language's implPath: its "implPaths" entry, else the
// project's.
func (p *Project) ImplTemplate(lang string) string {
	if template := p.implPaths[lang]; template != "" {
		return template
	}
	return p.ImplPath
}

// Generated is a "generated" entry (dockerfile, workflows), or "".
func (p *Project) Generated(key string) (string, error) {
	generated, _ := p.config.Get("generated")
	if !truthy(generated) {
		return "", nil
	}
	object, ok := generated.(*Object)
	if !ok {
		return "", unsupported(`"generated" is not an object`)
	}
	value, _ := object.Get(key)
	if value == nil {
		return "", nil
	}
	s, ok := value.(string)
	if !ok {
		return "", unsupported("generated.%s is not a string", key)
	}
	return s, nil
}

// Task is a project command run with an action (config.json "tasks").
type Task struct {
	Name    string
	Command string
}

// Tasks are an action's tasks, in the order config.json lists them.
func (p *Project) Tasks(action string) ([]Task, error) {
	tasks, _ := p.config.Get("tasks")
	if !truthy(tasks) {
		return nil, nil
	}
	object, ok := tasks.(*Object)
	if !ok {
		return nil, unsupported(`"tasks" is not an object`)
	}
	forAction, _ := object.Get(action)
	if !truthy(forAction) {
		return nil, nil
	}
	commands, ok := forAction.(*Object)
	if !ok {
		return nil, unsupported("tasks.%s is not an object", action)
	}
	var result []Task
	for _, name := range commands.Keys {
		command, ok := commands.Values[name].(string)
		if !ok {
			return nil, unsupported("tasks.%s.%s is not a string", action, name)
		}
		result = append(result, Task{Name: name, Command: command})
	}
	return result, nil
}

// Tools are the project's non-language dependencies by name
// (config.json "tools": {"shell": {"dependency": "shellcheck"}}).
func (p *Project) Tools() (map[string]string, error) {
	tools, ok := p.config.Get("tools")
	if !ok {
		return map[string]string{}, nil
	}
	object, isObject := tools.(*Object)
	if !isObject {
		return nil, unsupported(`"tools" is not an object`)
	}
	result := map[string]string{}
	for _, name := range object.Keys {
		tool, ok := object.Values[name].(*Object)
		if !ok {
			return nil, unsupported("tools.%s is not an object", name)
		}
		dependency, err := optionalField(tool, "dependency")
		if err != nil {
			return nil, err
		}
		result[name] = dependency
	}
	return result, nil
}

// optionalField is a definition or tool key Python reads as
// `data.get(key)` and uses only when truthy, as a string.
func optionalField(data *Object, key string) (string, error) {
	value, _ := data.Get(key)
	if !truthy(value) {
		return "", nil
	}
	s, ok := value.(string)
	if !ok {
		return "", unsupported("%q is not a string", key)
	}
	return s, nil
}

func readJSON(path string) (any, error) {
	if !isFile(path) {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, unsupported("%v", err)
	}
	value, err := decodeJSON(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return value, nil
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
