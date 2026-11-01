- **Android apps build and test their debug variants.** In a Gradle build
  with an Android module, `hct build` runs `assembleDebug` and `hct test`
  `testDebugUnitTest`, as a newcomer would, instead of `assemble` and
  `test`, which also build the release variants: those need a signing
  config and services files only the app's maintainers have. Found by the
  real-apps workflow (AntennaPod, Pokedex, bitwarden).
