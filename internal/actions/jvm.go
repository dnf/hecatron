package actions

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func init() { families["jvm"] = jvm }

var jvmBuilds = []string{"pom.xml", "build.gradle", "build.gradle.kts", "build.sbt", "build.mill", "build.sc"}

// jvm plans Maven, Gradle, sbt and Mill builds (Java, Kotlin, Scala,
// Groovy), through the build's wrapper when it ships one. Without a build
// file here, a Gradle settings file above makes this directory part of that
// build, run from the project gradleProjectDir says it belongs to; else the
// builds below, each.
//
// Not yet ported from jvm_hook: a Makefile above with a target for this
// directory, choosing a JDK the build can run on (jvm_select_jdk), a UTF-8
// locale (jvm_utf8_locale), and running allTests when a Kotlin
// Multiplatform build has no test task (gradle_test).
func jvm(def *Definition, ctx Context) ([]Op, error) {
	var ops []Op
	gradleAbove := ""
	runDir := "."
	if !existsAny(ctx.Dir, jvmBuilds) {
		if above, ok := DirAbove(ctx.Dir, ctx.Root, []string{"settings.gradle", "settings.gradle.kts"}); ok {
			gradleAbove = above
			if project := gradleProjectDir(ctx.Dir, above); project != ctx.Dir {
				ops = append(ops, Op{Kind: Note, Dir: ".", Message: def.Lang + ": " + ctx.Action + ": " + ctx.Dir + " is in the Gradle project " + project})
				runDir, _ = filepath.Rel(ctx.Dir, project)
				ctx.Dir = project
			} else {
				ops = append(ops, Op{Kind: Note, Dir: ".", Message: def.Lang + ": " + ctx.Action + ": " + ctx.Dir + " is a project of the Gradle build in " + above})
			}
		} else if below := DirsBelow(ctx.Dir, jvmBuilds); len(below) > 0 {
			return inEach(below, ctx, func(sub Context) ([]Op, error) { return jvm(def, sub) })
		} else {
			return []Op{{Kind: Unknown, Dir: ".", Message: "no pom.xml, build.gradle(.kts), build.sbt or build.mill here or below, no Gradle settings above, and no Makefile above with a target for " + filepath.Base(ctx.Dir)}}, nil
		}
	}
	tasks := map[string]map[string][]string{
		"maven":  {"test": {"test"}, "build": {"package", "-DskipTests"}, "clean": {"clean"}},
		"gradle": {"test": {"test"}, "build": {"assemble"}, "clean": {"clean"}, "lint": {"check", "-x", "test"}},
		"sbt":    {"test": {"test"}, "build": {"compile"}, "clean": {"clean"}},
		"mill":   {"test": {"-i", "-k", "__.test"}, "build": {"-i", "__.compile"}, "clean": {"clean"}},
	}
	noLint := map[string]string{"maven": "Maven has no standard lint goal", "sbt": "sbt has no standard lint task",
		"mill": "Mill has no standard lint task"}
	var tool string
	var command []string
	switch {
	case exists(ctx.Dir, "pom.xml"):
		tool, command = "maven", wrapperOr(ctx.Dir, "mvnw", "mvn", "-B")
		if ctx.Action == "test" && multiModule(ctx.Dir) {
			ops = append(ops, Op{Kind: Note, Dir: runDir, Message: def.Lang + ": test: a multi-module build: running its tests through the package phase"})
			tasks["maven"]["test"] = []string{"package"}
		}
	case exists(ctx.Dir, "build.gradle") || exists(ctx.Dir, "build.gradle.kts") || gradleAbove != "":
		tool, command = "gradle", gradleCommand(ctx)
	case exists(ctx.Dir, "build.sbt"):
		tool, command = "sbt", []string{"sbt"}
	default:
		tool, command = "mill", wrapperOr(ctx.Dir, "mill", "mill")
	}
	args, ok := tasks[tool][ctx.Action]
	if !ok {
		if reason, known := noLint[tool]; known && ctx.Action == "lint" {
			return append(ops, Op{Kind: Skip, Dir: runDir, Message: reason}), nil
		}
		return append(ops, Op{Kind: Unknown, Dir: runDir, Message: "no " + ctx.Action + " for " + tool}), nil
	}
	argv := append(append([]string{}, command...), args...)
	return append(ops, Op{Kind: Run, Dir: runDir, Argv: argv}), nil
}

// gradleProjectDir is the Gradle project a directory without a build file
// of its own belongs to, given the build's root (gradle_project_dir in
// languages/_hooks.sh): the one whose sources it holds (the directory above
// its nearest src), the root of a build of one project, else the directory.
func gradleProjectDir(dir, root string) string {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return dir
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] == "src" {
			return filepath.Join(append([]string{root}, parts[:i]...)...)
		}
	}
	for _, name := range []string{"settings.gradle", "settings.gradle.kts"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err == nil && settingsInclude.Match(data) {
			return dir
		}
	}
	return root
}

var settingsInclude = regexp.MustCompile(`(?m)^[ \t]*include[ \t(]`)

// multiModule is whether the pom.xml here lists modules.
func multiModule(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "pom.xml"))
	return err == nil && strings.Contains(string(data), "<modules>")
}

func wrapperOr(dir, wrapper, tool string, flags ...string) []string {
	if executable(filepath.Join(dir, wrapper)) {
		return append([]string{"./" + wrapper}, flags...)
	}
	return append([]string{tool}, flags...)
}

// A Gradle subproject's wrapper is usually in the build's root, above it.
func gradleCommand(ctx Context) []string {
	if executable(filepath.Join(ctx.Dir, "gradlew")) {
		return []string{"./gradlew"}
	}
	if above, ok := DirAbove(ctx.Dir, ctx.Root, []string{"gradlew"}); ok && executable(filepath.Join(above, "gradlew")) {
		return []string{filepath.Join(above, "gradlew")}
	}
	return []string{"gradle"}
}

func executable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}
