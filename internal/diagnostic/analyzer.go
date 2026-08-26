// Package diagnostic derives advisory consistency findings from OpenSpec evidence.
package diagnostic

import (
	"strings"

	"kv/internal/hook"
	"kv/internal/transform"
)

type Artifact struct{ LogicalType, Contents string }

// Analyze operates solely on saved OpenSpec snapshots. Findings are advisory;
// callers must never edit OpenSpec artifacts in response to them.
func Analyze(artifacts []Artifact, summary hook.ProviderSummary) []hook.Diagnostic {
	var proposal, design, specs, tasks []transform.Concept
	for _, artifact := range artifacts {
		concepts := transform.ParseArtifact(artifact.LogicalType, artifact.Contents)
		switch artifact.LogicalType {
		case "proposal":
			proposal = append(proposal, concepts...)
		case "design":
			design = append(design, concepts...)
		case "specs":
			specs = append(specs, concepts...)
		case "tasks":
			tasks = append(tasks, concepts...)
		}
	}
	var findings []hook.Diagnostic
	requirements, scenarios := filter(specs, transform.ConceptRequirement), filter(specs, transform.ConceptScenario)
	if len(proposal) > 0 && len(requirements) == 0 {
		findings = append(findings, finding("scope_spec_drift", "proposal scope has no requirement coverage"))
	}
	for _, decision := range filter(design, transform.ConceptDecision) {
		if !covered(decision, append(requirements, tasks...)) {
			findings = append(findings, finding("decision_without_coverage", "decision has no requirement or task coverage: "+decision.Title))
		}
	}
	if hasConflictingDecisions(filter(design, transform.ConceptDecision)) {
		findings = append(findings, finding("conflicting_current_decisions", "current design decisions contain opposing statements"))
	}
	if len(requirements) > 0 && len(scenarios) == 0 {
		findings = append(findings, finding("requirement_without_scenario", "requirements have no scenarios"))
	}
	for _, task := range tasks {
		if !covered(task, requirements) {
			findings = append(findings, finding("untraceable_task", "task has no traceable requirement: "+task.ID))
		}
	}
	if len(summary.Claims) > 0 {
		normalized := transform.NormalizeProviderSummary(summary, conceptDeltas(append(append([]transform.Concept{}, proposal...), append(append(design, specs...), tasks...)...)))
		if len(normalized.Rejected) > 0 {
			findings = append(findings, finding("provider_summary_divergence", "provider summary includes claims absent from the saved artifacts"))
		}
	}
	return findings
}
func finding(code, message string) hook.Diagnostic {
	return hook.Diagnostic{Code: code, Severity: "warning", Message: message}
}
func filter(concepts []transform.Concept, kind transform.ConceptKind) []transform.Concept {
	result := []transform.Concept{}
	for _, concept := range concepts {
		if concept.Kind == kind {
			result = append(result, concept)
		}
	}
	return result
}
func covered(concept transform.Concept, candidates []transform.Concept) bool {
	tokens := words(concept.Title + " " + concept.Body)
	for _, candidate := range candidates {
		for token := range words(candidate.Title + " " + candidate.Body) {
			if _, ok := tokens[token]; ok && len(token) > 3 {
				return true
			}
		}
	}
	return false
}
func words(value string) map[string]struct{} {
	result := map[string]struct{}{}
	for _, value := range strings.Fields(strings.ToLower(value)) {
		result[strings.Trim(value, ".,:;()[]")] = struct{}{}
	}
	return result
}
func conceptDeltas(concepts []transform.Concept) []transform.Delta {
	result := make([]transform.Delta, 0, len(concepts))
	for index := range concepts {
		result = append(result, transform.Delta{Kind: transform.DeltaAdded, After: &concepts[index]})
	}
	return result
}

func hasConflictingDecisions(decisions []transform.Concept) bool {
	for left := range decisions {
		for right := left + 1; right < len(decisions); right++ {
			leftNegative := strings.Contains(strings.ToLower(decisions[left].Title+" "+decisions[left].Body), "not ")
			rightNegative := strings.Contains(strings.ToLower(decisions[right].Title+" "+decisions[right].Body), "not ")
			if leftNegative != rightNegative && covered(decisions[left], []transform.Concept{decisions[right]}) {
				return true
			}
		}
	}
	return false
}
