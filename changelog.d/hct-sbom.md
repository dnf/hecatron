- **A software bill of materials:** `hct sbom` writes CycloneDX 1.6 JSON
  (or SPDX 2.3 with `--format spdx`) for every component in every
  language, from what `hct licenses` reads: each package's purl, its SPDX
  license where known (never guessed), direct or transitive where the lock
  file says, dev and test dependencies marked (`--no-dev` leaves them out),
  and the project's name, commit and a timestamp (`SOURCE_DATE_EPOCH` for
  reproducible output). To stdout or `-o FILE`; positional languages and
  `-c COMPONENT` select as `hct licenses` does. Packages from registries
  deps.dev has no licenses for (pub.dev, Hex...) are listed without
  licenses (#226).
