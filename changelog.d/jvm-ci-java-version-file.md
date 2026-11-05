- **The JDK file CI reads is a floor:** a JVM build runs on a JDK at least
  as new as the one in the file a workflow's setup-java names
  (`java-version-file: .github/.java-version`), as Mihon's Gradle plugins
  need JDK 21 to run.
