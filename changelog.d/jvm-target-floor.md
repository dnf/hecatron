- **A JVM build runs on a JDK new enough for the Java it compiles for.**
  A build that targets Java 21 (`sourceCompatibility`,
  `JavaVersion.VERSION_21`, Kotlin's `jvmTarget`, `options.release`,
  `maven.compiler.release`, also from a convention plugin or the version
  catalog) now runs on an installed JDK 21 or later instead of an older
  default ("invalid source release: 21"), and `hct install` adds one when
  none is installed. Found by the real-apps workflow (nextcloud, Pocket
  Casts and Lawnchair's Android apps).
