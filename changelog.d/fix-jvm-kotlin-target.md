- **A Kotlin JVM target picks the JDK:** a Gradle build whose Kotlin sets
  `jvmTarget` (11 or later) and that sets no Java target or toolchain runs on
  that JDK, which `hct install` installs, instead of failing Kotlin's check
  that Java and Kotlin target the same JVM.

- **JDK 8 installs:** a Gradle or Maven toolchain asking for JDK 8 gets
  Temurin 8, whose releases are named `jdk8u412-b08`. Before, `hct install`
  failed with "No Temurin JDK release matches 8" and installed no other JDK
  the build needed either.
