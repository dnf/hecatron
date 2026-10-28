- **.NET's `rollForward: latestMajor` is a floor:** a `global.json` asking
  for SDK 8.0.100 or later now gets hecatron's default SDK (10), or the
  newest of its own major when that is newer than the default, raised to
  the newest major the component's projects target (`net11.0` needs SDK
  11). Before, it read as SDK 8, which can't build a project targeting
  `net10.0`. Definitions name such files in the new `floorFiles` key.
