- **A pinned JDK is a floor:** a JVM build runs on a JDK at least as new as
  the one its `.java-version`, `.sdkmanrc` or `.tool-versions` pins, and
  `hct install` adds one when none is installed (a dependency compiled for
  Java 21 fails in javac on JDK 17).
- **The JDK a build runs on is `JAVA_HOME`'s:** hct checked the `java` on
  `PATH`, so a build whose `JAVA_HOME` named an older JDK ran on it.
- **Android builds in AOSP's layout** (an `AndroidManifest.xml` beside its
  `build.gradle`, as Lawnchair has) build and test their debug variants.
