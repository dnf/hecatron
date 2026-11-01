- **One Gradle run at a time in a build:** a Gradle build with Java and
  Kotlin sources runs as two jobs, which ran Gradle at once in one project,
  overwrote each other's generated sources and doubled the memory; the
  second now waits for the first.
