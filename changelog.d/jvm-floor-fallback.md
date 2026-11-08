- **A JVM build gets the newest JDK when none is new enough:** a build
  whose pinned JDK can't be installed yet (a CI pin to an unreleased JDK)
  runs on the newest installed JDK, with a warning, instead of staying on
  an older one.
