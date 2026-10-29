package sbom

import (
	"strconv"
	"strings"
)

// spdx is the bill of materials as SPDX 2.3 JSON: the project a package
// the document describes, CONTAINS each component, each component a
// package that DEPENDS_ON every package it uses (a dev or test one is its
// DEV_DEPENDENCY_OF), the relationship's comment saying direct or
// transitive where the lock file does.
func (m *model) spdx() []byte {
	ids := map[string]bool{"SPDXRef-DOCUMENT": true}
	rootID := spdxID(ids, "SPDXRef-Project-"+m.in.Project)
	root := obj{"SPDXID": rootID, "name": m.in.Project, "downloadLocation": "NOASSERTION", "filesAnalyzed": false,
		"primaryPackagePurpose": "APPLICATION"}
	if m.in.Commit != "" {
		root["sourceInfo"] = "git commit " + m.in.Commit
	}
	packages := []obj{root}
	relationships := []obj{relationship("SPDXRef-DOCUMENT", "DESCRIBES", rootID, "")}
	componentIDs := map[string]string{}
	for _, c := range m.in.Components {
		id := spdxID(ids, "SPDXRef-Component-"+c.Name)
		componentIDs[c.Name] = id
		packages = append(packages, obj{"SPDXID": id, "name": c.Name, "downloadLocation": "NOASSERTION",
			"filesAnalyzed": false, "comment": "A component of " + m.in.Project + ": " + strings.Join(c.Languages, ", ")})
		relationships = append(relationships, relationship(rootID, "CONTAINS", id, ""))
	}
	packageIDs := map[string]string{}
	for _, p := range m.packages {
		id := spdxID(ids, "SPDXRef-Package-"+p.ecosystem+"-"+p.name+"-"+p.version)
		packageIDs[p.purl] = id
		packages = append(packages, spdxPackage(p, id))
	}
	for _, c := range m.in.Components {
		seen := map[use]bool{}
		for _, u := range m.uses[c.Name] {
			if seen[u] {
				continue
			}
			seen[u] = true
			if u.dev {
				relationships = append(relationships, relationship(packageIDs[u.purl], "DEV_DEPENDENCY_OF", componentIDs[c.Name], u.dependency))
			} else {
				relationships = append(relationships, relationship(componentIDs[c.Name], "DEPENDS_ON", packageIDs[u.purl], u.dependency))
			}
		}
	}
	creation := obj{"created": m.timestamp(), "creators": []string{"Tool: hecatron-" + m.in.ToolVersion}}
	document := obj{
		"spdxVersion":   "SPDX-2.3",
		"dataLicense":   "CC0-1.0",
		"SPDXID":        "SPDXRef-DOCUMENT",
		"name":          m.in.Project,
		"creationInfo":  creation,
		"packages":      packages,
		"relationships": relationships,
	}
	document["documentNamespace"] = "https://spdx.org/spdxdocs/" + idText(m.in.Project) + "-" + uuid(marshal(document))
	return marshal(document)
}

func spdxPackage(p *pkg, id string) obj {
	declared := "NOASSERTION"
	var comments []string
	switch {
	case p.license != "" && !strings.Contains(p.license, "LicenseRef-") && !strings.Contains(p.license, "DocumentRef-"):
		declared = p.license
	case p.license != "":
		// A LicenseRef needs its text in the document, which hecatron doesn't have.
		comments = append(comments, "declared: "+p.license)
	case len(p.declared) > 0:
		comments = append(comments, "declared, not an SPDX expression: "+strings.Join(p.declared, ", "))
	}
	out := obj{"SPDXID": id, "name": p.name, "downloadLocation": "NOASSERTION", "filesAnalyzed": false,
		"licenseConcluded": "NOASSERTION", "licenseDeclared": declared, "primaryPackagePurpose": "LIBRARY",
		"externalRefs": []obj{{"referenceCategory": "PACKAGE-MANAGER", "referenceType": "purl", "referenceLocator": p.purl}}}
	if p.version != "" {
		out["versionInfo"] = p.version
	}
	if len(comments) > 0 {
		out["licenseComments"] = strings.Join(comments, "; ")
	}
	out["comment"] = "Used by " + strings.Join(p.components, ", ") + " (" + strings.Join(p.languages, ", ") +
		"), from " + strings.Join(p.sources, ", ")
	return out
}

func relationship(from, kind, to, comment string) obj {
	r := obj{"spdxElementId": from, "relationshipType": kind, "relatedSpdxElement": to}
	if comment != "" {
		r["comment"] = comment
	}
	return r
}

// spdxID is an SPDX identifier made of text (letters, digits, . and -),
// unique among ids.
func spdxID(ids map[string]bool, text string) string {
	id := idText(text)
	for n := 2; ids[id]; n++ {
		id = idText(text) + "-" + strconv.Itoa(n)
	}
	ids[id] = true
	return id
}

func idText(text string) string {
	var out strings.Builder
	for _, r := range text {
		switch {
		case 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z', '0' <= r && r <= '9', r == '.', r == '-':
			out.WriteRune(r)
		default:
			out.WriteByte('-')
		}
	}
	return out.String()
}
