package spdx

import (
	_ "embed"
	"strings"
)

// The SPDX list doesn't say which licenses are copyleft. Blue Oak Council
// keeps that list (and one of permissive licenses), as SPDX identifiers;
// copyleft.txt is both, vendored (its header says which versions).

//go:embed copyleft.txt
var classList string

// Class is how far a license's copyleft reaches, as Blue Oak Council
// classifies it, from none (permissive) to maximal.
type Class string

const (
	None    Class = "none"    // permissive
	Weak    Class = "weak"    // the licensed files (LGPL, MPL, EPL, CDDL)
	Strong  Class = "strong"  // the whole work (GPL)
	Network Class = "network" // the whole work, offered over a network too (AGPL, SSPL, EUPL, OSL)
	Maximal Class = "maximal" // beyond the work (RPL, CAL, Parity)
	// Unclassified is a license on neither of Blue Oak's lists (CC-BY-4.0,
	// a LicenseRef-).
	Unclassified Class = "unclassified"
)

var classRank = map[Class]int{None: 0, Weak: 1, Strong: 2, Network: 3, Maximal: 4}

var classes = map[string]Class{}

func init() {
	for _, line := range strings.Split(classList, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		class, id, _ := strings.Cut(line, " ")
		classes[id] = Class(class)
	}
}

// ClassOf is a license identifier's class (canonical, as Canonical spells
// it); "Apache-2.0+" is Apache-2.0's.
func ClassOf(id string) Class {
	if class, ok := classes[id]; ok {
		return class
	}
	if class, ok := classes[strings.TrimSuffix(id, "+")]; ok {
		return class
	}
	return Unclassified
}

// Copyleft is the expression's class as a whole: an OR is a choice, so the
// least restrictive option's; an AND applies every part, so the most
// restrictive's. An unclassified part makes it unclassified unless the
// rest decides it anyway (an OR with a permissive option, an AND with a
// maximal part). A license WITH an exception is the license's class, as a
// policy judges it.
func (e *Expression) Copyleft() Class {
	if e.Op == "" {
		return ClassOf(e.ID)
	}
	found, unclassified := Class(""), false
	for _, term := range e.Terms {
		class := term.Copyleft()
		switch {
		case class == Unclassified:
			unclassified = true
		case found == "",
			e.Op == "OR" && classRank[class] < classRank[found],
			e.Op == "AND" && classRank[class] > classRank[found]:
			found = class
		}
	}
	decided := e.Op == "OR" && found == None || e.Op == "AND" && found == Maximal
	if unclassified && !decided {
		return Unclassified
	}
	return found
}
