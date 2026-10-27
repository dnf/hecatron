package project

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
