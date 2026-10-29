package project

// Release is one row of a language's releases.json: its version, the date
// it was published (YYYY-MM-DD), and whether it was a long-term support
// release.
type Release struct {
	Version string
	Date    string
	LTS     bool
}

// Releases are a language's release dates (docs/guide/versions.md), from its
// definition's releases.json or that of the language its "releasesOf"
// names, oldest first as the table lists them; none when it has no table.
func (p *Project) Releases(lang string) ([]Release, error) {
	rows, err := p.releaseDates(lang)
	if err != nil {
		return nil, err
	}
	releases := make([]Release, 0, len(rows))
	for _, r := range rows {
		releases = append(releases, Release{r.version, r.date, r.lts})
	}
	return releases, nil
}

// NewestRelease is the newest release in a language's releases.json that a
// spec matches, "" for none: a prefix resolved offline, for an installer
// that installs exact versions.
func (p *Project) NewestRelease(lang, specText string) (string, error) {
	wanted, err := parseSpec(specText)
	if err != nil {
		return "", err
	}
	rows, err := p.releaseDates(lang)
	if err != nil {
		return "", err
	}
	var matching []string
	for _, r := range rows {
		if wanted.matches(r.version) {
			matching = append(matching, r.version)
		}
	}
	best, _ := newest(matching)
	return best, nil
}
