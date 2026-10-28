- **A Kotlin JVM target picks the JDK:** a Gradle build whose Kotlin sets
  `jvmTarget` (11 or later) and that sets no Java target or toolchain runs on
  that JDK, which `hct install` installs, instead of failing Kotlin's check
  that Java and Kotlin target the same JVM.
