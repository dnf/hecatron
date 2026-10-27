package project

// ConfigValue is one top-level key of the project's config.json, for a
// setting a command reads itself (the advisor's "advisor", "audit"): nil
// and false when the key is missing or the project was detected.
func (p *Project) ConfigValue(key string) (any, bool) { return p.config.Get(key) }

// Release is one row of a language's releases.json: its version, the date
// it was published (YYYY-MM-DD), and whether it was a long-term support
// release.
type Release struct {
	Version string
	Date    string
	LTS     bool
}

// Releases are a language's release dates (docs/design.md §18), from its
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
