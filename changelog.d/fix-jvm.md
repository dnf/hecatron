- **JVM and .NET builds in real projects:** a Java, Kotlin, Scala or
  Groovy directory inside a Gradle build that isn't one of its projects
  (`moshi/src/main/java`) runs Gradle from the project it belongs to,
  instead of failing as "not part of the build"; a Kotlin Multiplatform
  build without a `test` task runs `jvmTest`, else `allTests`; a
  multi-module Maven build tests through the package phase; Mill builds (`build.mill`) are tested;
  the JVM gets a UTF-8 locale when none is set; Maven, Gradle and sbt are
  installed for builds below the language's directory too. .NET tests roll
  forward to the installed runtime (a test project targeting net8.0 runs on
  .NET 10), and a directory with several projects and no solution tests
  each test project. A Clojure project with `deps.edn` but no `:test`
  alias tests with Leiningen when it has `project.clj`. A Gradle build's
  toolchain JDK is found from its root and its version catalog
  (`libs.versions.jdk`), so a source directory's build gets it installed,
  and a build whose `java` is a runtime without `javac` (a JRE another
  package pulled in) runs on the newest installed JDK.
