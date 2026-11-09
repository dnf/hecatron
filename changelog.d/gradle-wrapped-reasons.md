- **Android's AAR metadata failures in the excerpt:** when a dependency
  needs a newer compileSdk, a failed Gradle build's excerpt names the
  dependency, the Android API level it needs, the one the module compiles
  against and the recommended fix, instead of only "Execution failed for
  task ':core-data:checkDebugAarMetadata'".
