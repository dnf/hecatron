package project

// SecretsIgnore is config.json's "audit": {"secrets": {"ignore": [...]}}:
// the paths hct audit's secret scan leaves out (test fixtures), relative to
// the project root. Only the binary reads it, so only its check reports
// what's wrong with it (SecretsProblems).
func (p *Project) SecretsIgnore() []string {
	section := p.secretsSection()
	paths, _ := section.Get("ignore")
	list, _ := paths.([]any)
	var found []string
	for _, path := range list {
		if text, ok := path.(string); ok && text != "" {
			found = append(found, text)
		}
	}
	return found
}

// SecretsConfigured says whether config.json has an audit.secrets section.
func (p *Project) SecretsConfigured() bool { return p.secretsSection() != nil }

func (p *Project) secretsSection() *Object {
	audit, _ := p.config.Values["audit"].(*Object)
	value, _ := audit.Get("secrets")
	section, _ := value.(*Object)
	return section
}

// SecretsProblems is what's wrong with audit.secrets, for hecatron check.
func (p *Project) SecretsProblems() []string {
	audit, _ := p.config.Values["audit"].(*Object)
	value, present := audit.Get("secrets")
	if !present || value == nil {
		return nil
	}
	section, ok := value.(*Object)
	if !ok {
		return []string{"audit.secrets must be an object"}
	}
	var problems []string
	for _, key := range section.Keys {
		if key != "ignore" {
			problems = append(problems, "audit.secrets."+key+" isn't a setting (ignore)")
		}
	}
	if paths, present := section.Get("ignore"); present {
		list, valid := paths.([]any)
		for _, path := range list {
			if text, isText := path.(string); !isText || text == "" {
				valid = false
			}
		}
		if !valid {
			problems = append(problems, `audit.secrets.ignore must be a list of paths relative to the project root (["tests/fixtures/"])`)
		}
	}
	return problems
}
