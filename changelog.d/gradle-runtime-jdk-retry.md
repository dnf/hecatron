- **Gradle reruns on the JDK a plugin needs:** when Gradle fails because a
  plugin needs a newer JVM ("Dependency requires at least JVM runtime
  version 21"), hct runs it again on an installed JDK that new.
