package actions

import (
	"os"
	"path/filepath"
)

func init() { families["jvm"] = jvm }

var jvmBuilds = []string{"pom.xml", "build.gradle", "build.gradle.kts", "build.sbt"}

// jvm plans Maven, Gradle and sbt builds (Java, Kotlin, Scala, Groovy),
// through the build's wrapper when it ships one. Without a build file here,
// a Gradle settings file above makes this directory one of that build's
// projects; else the builds below, each.
//
// Not yet ported from jvm_hook: a Makefile above with a target for this
// directory, and choosing a JDK the build can run on (jvm_select_jdk).
func jvm(def *Definition, ctx Context) ([]Op, error) {
	var ops []Op
	gradleAbove := ""
	if !existsAny(ctx.Dir, jvmBuilds) {
		if above, ok := DirAbove(ctx.Dir, ctx.Root, []string{"settings.gradle", "settings.gradle.kts"}); ok {
			gradleAbove = above
			ops = append(ops, Op{Kind: Note, Dir: ".", Message: def.Lang + ": " + ctx.Action + ": " + ctx.Dir + " is a project of the Gradle build in " + above})
		} else if below := DirsBelow(ctx.Dir, jvmBuilds); len(below) > 0 {
			return inEach(below, ctx, func(sub Context) ([]Op, error) { return jvm(def, sub) })
		} else {
			return []Op{{Kind: Unknown, Dir: ".", Message: "no pom.xml, build.gradle(.kts) or build.sbt here or below, no Gradle settings above, and no Makefile above with a target for " + filepath.Base(ctx.Dir)}}, nil
		}
	}
	tasks := map[string]map[string][]string{
		"maven":  {"test": {"test"}, "build": {"package", "-DskipTests"}, "clean": {"clean"}},
		"gradle": {"test": {"test"}, "build": {"assemble"}, "clean": {"clean"}, "lint": {"check", "-x", "test"}},
		"sbt":    {"test": {"test"}, "build": {"compile"}, "clean": {"clean"}},
	}
	noLint := map[string]string{"maven": "Maven has no standard lint goal", "sbt": "sbt has no standard lint task"}
	var tool string
	var command []string
	switch {
	case exists(ctx.Dir, "pom.xml"):
		tool, command = "maven", wrapperOr(ctx.Dir, "mvnw", "mvn", "-B")
	case exists(ctx.Dir, "build.gradle") || exists(ctx.Dir, "build.gradle.kts") || gradleAbove != "":
		tool, command = "gradle", gradleCommand(ctx)
	default:
		tool, command = "sbt", []string{"sbt"}
	}
	args, ok := tasks[tool][ctx.Action]
	if !ok {
		if reason, known := noLint[tool]; known && ctx.Action == "lint" {
			return append(ops, Op{Kind: Skip, Dir: ".", Message: reason}), nil
		}
		return append(ops, Op{Kind: Unknown, Dir: ".", Message: "no " + ctx.Action + " for " + tool}), nil
	}
	argv := append(append([]string{}, command...), args...)
	return append(ops, Op{Kind: Run, Dir: ".", Argv: argv}), nil
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
