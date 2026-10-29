package sbom

import (
	"strings"
)

// cyclonedx is the bill of materials as CycloneDX 1.6 JSON: the project
// as the metadata's component, its components (bom-ref component:NAME)
// and their packages (bom-ref the purl) as components, a dependency graph
// with each component's direct dependencies, and dev and test ones scoped
// optional. Which package a transitive one comes through isn't known, so
// the graph is declared incomplete.
func (m *model) cyclonedx() []byte {
	root := obj{"type": "application", "bom-ref": "project", "name": m.in.Project}
	var rootProperties []obj
	if m.in.Commit != "" {
		rootProperties = append(rootProperties, property("hecatron:commit", m.in.Commit))
	}
	if len(rootProperties) > 0 {
		root["properties"] = rootProperties
	}
	components := []obj{}
	dependencies := []obj{}
	var componentRefs []string
	for _, c := range m.in.Components {
		ref := "component:" + c.Name
		componentRefs = append(componentRefs, ref)
		components = append(components, obj{"type": "application", "bom-ref": ref, "name": c.Name,
			"properties": []obj{property("hecatron:languages", strings.Join(c.Languages, ", "))}})
	}
	for _, p := range m.packages {
		components = append(components, cyclonedxPackage(p))
	}
	dependencies = append(dependencies, obj{"ref": "project", "dependsOn": orEmpty(componentRefs)})
	for _, c := range m.in.Components {
		var direct []string
		for _, u := range m.uses[c.Name] {
			if u.dependency == "direct" && (len(direct) == 0 || direct[len(direct)-1] != u.purl) {
				direct = append(direct, u.purl)
			}
		}
		dependencies = append(dependencies, obj{"ref": "component:" + c.Name, "dependsOn": orEmpty(direct)})
	}
	document := obj{
		"$schema":     "http://cyclonedx.org/schema/bom-1.6.schema.json",
		"bomFormat":   "CycloneDX",
		"specVersion": "1.6",
		"version":     1,
		"metadata": obj{
			"timestamp":  m.timestamp(),
			"lifecycles": []obj{{"phase": "pre-build"}},
			"tools": obj{"components": []obj{{"type": "application", "name": "hecatron", "version": m.in.ToolVersion,
				"externalReferences": []obj{{"type": "website", "url": "https://github.com/dnf/hecatron"}}}}},
			"component": root,
		},
		"components":   components,
		"dependencies": dependencies,
	}
	if len(componentRefs) > 0 {
		document["compositions"] = []obj{{"aggregate": "incomplete", "dependencies": componentRefs}}
	}
	document["serialNumber"] = "urn:uuid:" + uuid(marshal(document))
	return marshal(document)
}

func cyclonedxPackage(p *pkg) obj {
	component := obj{"type": "library", "bom-ref": p.purl, "name": p.name, "purl": p.purl, "scope": "required"}
	if p.version != "" {
		component["version"] = p.version
	}
	if p.dev {
		component["scope"] = "optional"
	}
	if p.license != "" {
		component["licenses"] = []obj{{"expression": p.license}}
	}
	properties := []obj{}
	if p.dependency != "" {
		properties = append(properties, property("hecatron:dependency", p.dependency))
	}
	if p.dev {
		properties = append(properties, property("hecatron:dev", "true"))
	}
	for _, declared := range p.declared {
		properties = append(properties, property("hecatron:declaredLicense", declared))
	}
	for _, c := range p.components {
		properties = append(properties, property("hecatron:component", c))
	}
	for _, l := range p.languages {
		properties = append(properties, property("hecatron:language", l))
	}
	for _, s := range p.sources {
		properties = append(properties, property("hecatron:source", s))
	}
	component["properties"] = properties
	return component
}

type obj = map[string]any

func property(name, value string) obj { return obj{"name": name, "value": value} }

func orEmpty(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}
