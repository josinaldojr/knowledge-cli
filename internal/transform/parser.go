// Package transform parses OpenSpec artifacts into deterministic concepts.
package transform

import (
	"bufio"
	"strings"
)

type ConceptKind string

const (
	ConceptSection     ConceptKind = "section"
	ConceptDecision    ConceptKind = "decision"
	ConceptRequirement ConceptKind = "requirement"
	ConceptScenario    ConceptKind = "scenario"
	ConceptTask        ConceptKind = "task"
)

// Concept is a source-ordered, parser-versioned fact extracted from one
// OpenSpec artifact. It intentionally retains text instead of interpreting it
// semantically so later delta and memory layers can remain evidence-backed.
type Concept struct {
	Kind      ConceptKind `json:"kind"`
	ID        string      `json:"id"`
	Title     string      `json:"title"`
	Body      string      `json:"body,omitempty"`
	Operation string      `json:"operation,omitempty"`
	Completed bool        `json:"completed,omitempty"`
}

// ParseArtifact selects the deterministic parser for an OpenSpec logical
// artifact type. Unknown types produce no concepts rather than guessing.
func ParseArtifact(logicalType, contents string) []Concept {
	switch logicalType {
	case "proposal":
		return ParseProposal(contents)
	case "design":
		return ParseDesign(contents)
	case "specs":
		return ParseSpec(contents)
	case "tasks":
		return ParseTasks(contents)
	default:
		return nil
	}
}

// ParseProposal returns level-two proposal sections in source order.
func ParseProposal(contents string) []Concept { return parseSections(contents, "## ") }

// ParseDesign returns numbered or unnumbered level-three design decisions.
func ParseDesign(contents string) []Concept {
	concepts := parseSections(contents, "### ")
	for index := range concepts {
		concepts[index].Kind = ConceptDecision
	}
	return concepts
}

func parseSections(contents, prefix string) []Concept {
	var concepts []Concept
	var current *Concept
	for _, line := range lines(contents) {
		if strings.HasPrefix(line, prefix) {
			concept := Concept{Kind: ConceptSection, ID: slug(strings.TrimPrefix(line, prefix)), Title: strings.TrimSpace(strings.TrimPrefix(line, prefix))}
			concepts = append(concepts, concept)
			current = &concepts[len(concepts)-1]
			continue
		}
		if current != nil {
			current.Body = appendLine(current.Body, line)
		}
	}
	for index := range concepts {
		concepts[index].Body = strings.TrimSpace(concepts[index].Body)
	}
	return concepts
}

// ParseSpec recognizes OpenSpec delta operation headings, requirements, and
// scenarios. Requirement/scenario bodies remain verbatim evidence.
func ParseSpec(contents string) []Concept {
	var concepts []Concept
	operation := ""
	var current *Concept
	for _, line := range lines(contents) {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			operation = deltaOperation(strings.TrimPrefix(trimmed, "## "))
			current = nil
			continue
		}
		if strings.HasPrefix(trimmed, "### Requirement: ") {
			title := strings.TrimSpace(strings.TrimPrefix(trimmed, "### Requirement: "))
			concepts = append(concepts, Concept{Kind: ConceptRequirement, ID: slug(title), Title: title, Operation: operation})
			current = &concepts[len(concepts)-1]
			continue
		}
		if strings.HasPrefix(trimmed, "#### Scenario: ") {
			title := strings.TrimSpace(strings.TrimPrefix(trimmed, "#### Scenario: "))
			concepts = append(concepts, Concept{Kind: ConceptScenario, ID: slug(title), Title: title, Operation: operation})
			current = &concepts[len(concepts)-1]
			continue
		}
		if current != nil {
			current.Body = appendLine(current.Body, line)
		}
	}
	for index := range concepts {
		concepts[index].Body = strings.TrimSpace(concepts[index].Body)
	}
	return concepts
}

// ParseTasks recognizes OpenSpec markdown checkboxes including their stable
// numeric prefix when present.
func ParseTasks(contents string) []Concept {
	var concepts []Concept
	for _, line := range lines(contents) {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- [") || len(trimmed) < 6 || trimmed[4] != ']' || trimmed[5] != ' ' {
			continue
		}
		state, title := trimmed[3], strings.TrimSpace(trimmed[6:])
		if state != ' ' && state != 'x' && state != 'X' {
			continue
		}
		id := title
		if index := strings.IndexByte(title, ' '); index > 0 {
			id = title[:index]
		}
		concepts = append(concepts, Concept{Kind: ConceptTask, ID: id, Title: title, Completed: state == 'x' || state == 'X'})
	}
	return concepts
}

func deltaOperation(heading string) string {
	for _, operation := range []string{"ADDED", "MODIFIED", "REMOVED", "RENAMED"} {
		if strings.HasPrefix(heading, operation) {
			return operation
		}
	}
	return ""
}

func lines(contents string) []string {
	scanner := bufio.NewScanner(strings.NewReader(contents))
	result := []string{}
	for scanner.Scan() {
		result = append(result, scanner.Text())
	}
	return result
}

func appendLine(body, line string) string {
	if body == "" {
		return line
	}
	return body + "\n" + line
}

func slug(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), "-"))
}
